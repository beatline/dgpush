package push

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"push-worker/global"
	"push-worker/models"
	"sync"
	"time"
)

// HarmonyPusher 鸿蒙 Pusher
type HarmonyPusher struct {
	key        *ServiceAccountKey
	private    *rsa.PrivateKey
	jwt        string
	expiry     time.Time
	mu         sync.RWMutex
	httpClient *http.Client
}

// ServiceAccountKey 服务账户密钥
type ServiceAccountKey struct {
	ProjectId  string `json:"project_id"`
	KeyID      string `json:"key_id"`
	SubAccount string `json:"sub_account"`
	PrivateKey string `json:"private_key"`
}

// NewHarmonyPusher 初始化鸿蒙 Pusher
func NewHarmonyPusher(path string) *HarmonyPusher {
	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("Missing harmony key file", zap.Error(err))
	}
	// 解析 JSON 到 ServiceAccountKey
	var saKey ServiceAccountKey
	if err := json.Unmarshal(data, &saKey); err != nil {
		global.LOG.Fatal("Parse harmony key file failed", zap.Error(err))
	}
	// 解析 RSA 私钥 (从 PEM 字符串) 使用 jwt.ParseRSAPrivateKeyFromPEM 直接转换
	priKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(saKey.PrivateKey))
	if err != nil {
		global.LOG.Fatal("Parse harmony private key failed", zap.Error(err))
	}
	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	// 4. 返回初始化好的对象
	return &HarmonyPusher{
		key:     &saKey,
		private: priKey,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// getToken 获取/刷新 JWT Token
func (h *HarmonyPusher) getToken() (string, error) {
	// 1. 读锁检查缓存是否有效
	h.mu.RLock()
	// 只要当前时间还在 expiry 之前，就直接使用内存中的 jwt
	if h.jwt != "" && time.Now().Before(h.expiry) {
		defer h.mu.RUnlock()
		return h.jwt, nil
	}
	h.mu.RUnlock()

	// 2. 缓存失效，加写锁重新生成
	h.mu.Lock()
	defer h.mu.Unlock()

	// 双重检查，防止并发场景下重复生成
	if h.jwt != "" && time.Now().Before(h.expiry) {
		return h.jwt, nil
	}

	now := time.Now().UTC()
	// 生成符合规则的 JWT Claims
	token := jwt.NewWithClaims(jwt.SigningMethodPS256, jwt.MapClaims{
		"iss": h.key.SubAccount,
		"iat": now.Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
		"aud": "https://oauth-login.cloud.huawei.com/oauth2/v3/token",
	})
	token.Header["kid"] = h.key.KeyID

	// 使用私钥签名
	signedToken, err := token.SignedString(h.private)
	if err != nil {
		return "", err
	}

	// 将生成的 Token 放入本地内存，并将过期时间设为与 exp 相同
	h.jwt = signedToken
	h.expiry = now.Add(1 * time.Hour).Add(-5 * time.Minute)

	return signedToken, nil
}

func (p *HarmonyPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	jwtToken, err := p.getToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://push-api.cloud.huawei.com/v3/%s/messages:send", p.key.ProjectId)

	// 优化点 1：使用结构体而非 map，并复用 time.Now() 减少系统调用
	now := time.Now().UTC()
	body := map[string]any{ // 实际生产建议定义具体 struct
		"payload": map[string]any{
			"notification": map[string]any{
				"category": "DEVICE_REMINDER",
				"title":    data.Title,
				"body":     data.Body,
				"image":    data.Image,
				"clickAction": map[string]any{
					"actionType": data.ActionType,
				},
				"notifyId": now.Unix(),
			},
		},
		"target":      map[string]any{"token": []string{regId}},
		"pushOptions": map[string]any{"testMessage": true},
	}

	bodyBytes, err := sonic.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("push-type", "0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}

	//显式丢弃 Body，确保连接 100% 复用
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBuf, _ := io.ReadAll(resp.Body) // 仅在报错时读取具体内容
		global.LOG.Error("鸿蒙推送返回异常", zap.Int("status", resp.StatusCode), zap.String("body", string(respBuf)))
		return fmt.Errorf("harmony error: %d", resp.StatusCode)
	}

	return nil
}
