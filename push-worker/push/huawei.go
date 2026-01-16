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

type HuaweiPusher struct {
	config     *HuaweiConfig
	httpClient *http.Client
	authToken  string
	expiry     time.Time
	mu         sync.RWMutex
}

type HuaweiConfig struct {
	AppId     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

func NewHuaweiPusher(path string) *HuaweiPusher {
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("读取华为证书失败", zap.Error(err))
	}
	var cfg HuaweiConfig
	_ = json.Unmarshal(data, &cfg)
	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &HuaweiPusher{
		config: &cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

func (h *HuaweiPusher) getToken(ctx context.Context) (string, error) {
	h.mu.RLock()
	if h.authToken != "" && time.Now().Before(h.expiry) {
		defer h.mu.RUnlock()
		return h.authToken, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.authToken != "" && time.Now().Before(h.expiry) {
		return h.authToken, nil
	}

	apiURL := "https://oauth-login.cloud.huawei.com/oauth2/v3/token"
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", h.config.AppId)
	data.Set("client_secret", h.config.AppSecret)

	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       int    `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &res)

	h.authToken = res.AccessToken
	h.expiry = time.Now().Add(time.Duration(res.ExpiresIn-60) * time.Second)
	return h.authToken, nil
}

func (h *HuaweiPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	token, err := h.getToken(ctx)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://push-api.cloud.huawei.com/v1/%s/messages:send", h.config.AppId)
	payload := map[string]any{
		"message": map[string]any{
			"notification": map[string]any{
				"title": data.Title,
				"body":  data.Body,
			},
			"android": map[string]any{
				"category": "DEVICE_REMINDER",
				"notification": map[string]any{
					"title": data.Title,
					"body":  data.Body,
					"click_action": map[string]any{
						"type": 3,
					},
				},
			},
			"token": []string{regId},
		},
	}

	bodyBytes, _ := sonic.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBuf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("huawei error: %d, res: %s", resp.StatusCode, string(respBuf))
	}
	return nil
}
