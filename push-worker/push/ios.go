package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"push-worker/global"
	"push-worker/models"

	"github.com/bytedance/sonic"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// IosPusher iOS 推送执行器
type IosPusher struct {
	config     *models.IosKeyConfig
	private    *ecdsa.PrivateKey
	jwt        string
	expiry     time.Time
	mu         sync.RWMutex
	httpClient *http.Client
}

// NewIosPusher 初始化 iOS 推送执行器
func NewIosPusher(path string) *IosPusher {
	// 1. 读取并解析配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("读取iOS证书文件失败", zap.String("path", path), zap.Error(err))
	}

	var cfg models.IosKeyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		global.LOG.Fatal("解析iOS配置文件失败", zap.Error(err))
	}

	// 2. 解析 ECDSA 私钥 (APNs 专用)
	priv, err := jwt.ParseECPrivateKeyFromPEM([]byte(cfg.PrivateKey))
	if err != nil {
		global.LOG.Fatal("解析iOS私钥失败", zap.Error(err))
	}

	// 3. 配置高性能 HTTP/2 传输层 (针对 APNs 跨境优化)
	transport := &http.Transport{
		ForceAttemptHTTP2: true, // APNs 必须强制 H2
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 延长握手超时以应对跨境延迟
			KeepAlive: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 40 * time.Second,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
	}

	return &IosPusher{
		config:  &cfg,
		private: priv,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second, // 总请求超时设置为 60s
		},
	}
}

// getToken 获取或刷新本地内存缓存的 JWT Token
func (i *IosPusher) getToken() (string, error) {
	i.mu.RLock()
	if i.jwt != "" && time.Now().Before(i.expiry) {
		defer i.mu.RUnlock()
		return i.jwt, nil
	}
	i.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()

	// 双重检查锁定
	if i.jwt != "" && time.Now().Before(i.expiry) {
		return i.jwt, nil
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": i.config.TeamID,
		"iat": now.Unix(),
	})
	token.Header["kid"] = i.config.KeyID

	signedToken, err := token.SignedString(i.private)
	if err != nil {
		return "", fmt.Errorf("iOS JWT 签名失败: %w", err)
	}

	i.jwt = signedToken
	i.expiry = now.Add(50 * time.Minute) // 提前 10 分钟失效以保证连续性

	return signedToken, nil
}

// Send 执行苹果消息推送
func (i *IosPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	// 1. 获取鉴权 Token
	jwtToken, err := i.getToken()
	if err != nil {
		return err
	}

	// 2. 构造 APNs 请求地址
	host := "https://api.push.apple.com"
	if i.config.Sandbox {
		host = "https://api.sandbox.push.apple.com"
	}
	url := fmt.Sprintf("%s/3/device/%s", host, regId)

	// 3. 构建载荷
	payload := map[string]any{
		"aps": map[string]any{
			"alert": map[string]any{
				"title": data.Title,
				"body":  data.Body,
			},
			"sound": "default",
			"badge": 1,
		},
		"sn": data.Sn,
	}

	bodyBytes, err := sonic.Marshal(payload)
	if err != nil {
		return fmt.Errorf("报文序列化失败: %w", err)
	}

	// 4. 发起 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("apns-topic", i.config.BundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 APNs 网关失败: %w", err)
	}

	// 5. 显式丢弃 Body 并关闭响应，确保 TCP 连接复用
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// 6. 处理响应结果
	if resp.StatusCode != http.StatusOK {
		respBuf, _ := io.ReadAll(resp.Body)
		global.LOG.Error("iOS推送返回异常",
			zap.Int("status", resp.StatusCode),
			zap.String("response", string(respBuf)),
			zap.String("sn", data.Sn))
		return fmt.Errorf("apns api error: status %d", resp.StatusCode)
	}

	return nil
}
