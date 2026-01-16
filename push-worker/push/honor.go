package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"push-worker/global"
	"push-worker/models"

	"github.com/bytedance/sonic"
	"go.uber.org/zap"
)

// HonorPusher 荣耀推送执行器
type HonorPusher struct {
	config     *HonorKeyConfig
	authToken  string
	expiry     time.Time
	mu         sync.RWMutex
	httpClient *http.Client
}

// HonorKeyConfig 荣耀密钥配置
type HonorKeyConfig struct {
	AppId        string `json:"app_id"`        // 对应 API URL 中的 ID
	ClientId     string `json:"client_id"`     // 对应 Auth 中的 Client ID
	ClientSecret string `json:"client_secret"` // 对应 Auth 中的 Client Secret
}

// NewHonorPusher 初始化荣耀推送执行器
func NewHonorPusher(path string) *HonorPusher {
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("读取荣耀证书失败", zap.Error(err))
	}

	var cfg HonorKeyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		global.LOG.Fatal("解析荣耀配置失败", zap.Error(err))
	}

	// 去除隐形空格，确保配置准确
	cfg.AppId = strings.TrimSpace(cfg.AppId)
	cfg.ClientId = strings.TrimSpace(cfg.ClientId)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)

	return &HonorPusher{
		config: &cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// getToken 获取/刷新 AccessToken (由 Redis 缓存改为本地内存缓存以降低 QPS 损耗)
func (p *HonorPusher) getToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	if p.authToken != "" && time.Now().Before(p.expiry) {
		defer p.mu.RUnlock()
		return p.authToken, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查
	if p.authToken != "" && time.Now().Before(p.expiry) {
		return p.authToken, nil
	}

	authURL := "https://iam.developer.honor.com/auth/token"
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", p.config.ClientId)
	formData.Set("client_secret", p.config.ClientSecret)

	req, _ := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       any    `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	respBuf, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBuf, &res)

	if res.AccessToken == "" {
		return "", fmt.Errorf("honor auth failed: %v - %s", res.Error, res.ErrorDesc)
	}

	p.authToken = res.AccessToken
	// 提前 5 分钟失效
	p.expiry = time.Now().Add(time.Duration(res.ExpiresIn)*time.Second - 5*time.Minute)

	return p.authToken, nil
}

// Send 执行发送
func (p *HonorPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	token, err := p.getToken(ctx)
	if err != nil {
		global.LOG.Error("Get Honor Token Failed", zap.Error(err))
		return err
	}

	// 1. 构造 API URL
	apiURL := fmt.Sprintf("https://push-api.cloud.honor.com/api/v1/%s/sendMessage", p.config.AppId)

	// 2. 构造 Payload (整合透传+通知逻辑)
	// 荣耀要求 passThrough 字段为 JSON 字符串
	passThrough := fmt.Sprintf(`{"sn":"%s"}`, data.Sn)
	payload := map[string]any{
		"token": []string{regId},
		"data":  passThrough,
		"android": map[string]any{
			"targetUserType": 0,
			"ttl":            "86400s",
			"data":           passThrough,
			"notification": map[string]any{
				"title":       data.Title,
				"body":        data.Body,
				"importance":  "NORMAL",
				"style":       0,
				"clickAction": map[string]any{"type": 3}, // 3: 直接打开应用首页
				"when":        time.Now().Format(time.RFC3339),
			},
		},
	}

	bodyBytes, _ := sonic.Marshal(payload)

	// 3. 构建请求并设置 Header
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	// 荣耀 V1 要求 timestamp
	req.Header.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}

	// 4. 显式丢弃 Body 以复用连接
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// 5. 处理响应结果
	respBuf, _ := io.ReadAll(resp.Body)
	var res map[string]any
	_ = json.Unmarshal(respBuf, &res)

	// 使用您在 dgiot-worker 中定义的 code 80000000 成功判定逻辑
	if !p.isSuccess(res["code"]) {
		// 自动清除本地过期 Token 触发下次刷新
		if p.isAuthError(res["error"]) {
			p.mu.Lock()
			p.authToken = ""
			p.mu.Unlock()
		}
		global.LOG.Error("Honor Push Failed", zap.String("body", string(respBuf)), zap.String("sn", data.Sn))
		return fmt.Errorf("honor push error: %s", string(respBuf))
	}

	global.LOG.Debug("Honor Push Success", zap.String("sn", data.Sn))
	return nil
}

// isSuccess 荣耀成功的 code 是 80000000
func (p *HonorPusher) isSuccess(code any) bool {
	if num, ok := code.(float64); ok {
		return num == 80000000
	}
	if str, ok := code.(string); ok {
		return str == "80000000"
	}
	return false
}

// isAuthError 判断是否为鉴权错误
func (p *HonorPusher) isAuthError(errCode any) bool {
	if s, ok := errCode.(string); ok {
		return s == "access_denied" || s == "invalid_token" || s == "1101"
	}
	if f, ok := errCode.(float64); ok {
		return f == 1101
	}
	return false
}
