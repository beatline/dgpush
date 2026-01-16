package push

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"push-worker/models"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"go.uber.org/zap"
	"push-worker/global"
)

type OppoPusher struct {
	config     *OppoConfig
	authToken  string
	expiry     time.Time
	mu         sync.RWMutex
	httpClient *http.Client
}

type OppoConfig struct {
	AppKey       string `json:"app_key"`
	MasterSecret string `json:"master_secret"`
	ChannelId    string `json:"channel_id"`
}

func NewOppoPusher(path string) *OppoPusher {
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("读取OPPO证书失败", zap.Error(err))
	}
	var cfg OppoConfig
	_ = json.Unmarshal(data, &cfg)

	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &OppoPusher{
		config: &cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

func (o *OppoPusher) getToken(ctx context.Context) (string, error) {
	// 1. 读锁检查缓存是否有效
	o.mu.RLock()
	// 只要当前时间还在 expiry 之前，就直接使用内存中的 jwt
	if o.authToken != "" && time.Now().Before(o.expiry) {
		defer o.mu.RUnlock()
		return o.authToken, nil
	}
	o.mu.RUnlock()

	// 2. 缓存失效，加写锁重新生成
	o.mu.Lock()
	defer o.mu.Unlock()

	// 双重检查，防止并发场景下重复生成
	if o.authToken != "" && time.Now().Before(o.expiry) {
		return o.authToken, nil
	}

	apiURL := "https://api.push.oppomobile.com/server/v1/auth"
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	rawSign := o.config.AppKey + timestamp + o.config.MasterSecret
	hash := sha256.Sum256([]byte(rawSign))
	sign := hex.EncodeToString(hash[:])
	formData := url.Values{}
	formData.Set("app_key", o.config.AppKey)
	formData.Set("timestamp", timestamp)
	formData.Set("sign", sign)

	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Code int `json:"code"`
		Data struct {
			AuthToken string `json:"auth_token"`
		} `json:"data"`
		Message string `json:"message"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := sonic.Unmarshal(body, &res); err != nil {
		return "", err
	}

	if res.Code != 0 {
		return "", fmt.Errorf("oppo鉴权失败: %d (%s)", res.Code, res.Message)
	}
	// 将生成的 Token 放入本地内存，并将过期时间设为与 exp 相同
	o.authToken = res.Data.AuthToken
	o.expiry = time.UnixMilli(time.Now().UnixMilli()).Add(24 * time.Hour).Add(-10 * time.Minute)

	return o.authToken, nil
}

// Send 发起单推请求并打印调试日志
func (o *OppoPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	token, err := o.getToken(ctx)
	if err != nil {
		global.LOG.Fatal("Parse oppo auth token failed", zap.Error(err))
		return err
	}

	notification := map[string]any{
		"title":             data.Title,
		"content":           data.Body,
		"click_action_type": 0,
		"channel_id":        o.config.ChannelId,
	}

	messagePayload := map[string]any{
		"target_type":            2,
		"target_value":           regId,
		"notification":           notification,
		"verify_registration_id": true,
		"off_line":               true,
		"off_line_ttl":           3600 * 24,
	}

	messageJson, _ := sonic.MarshalString(messagePayload)

	formData := url.Values{}
	formData.Set("message", messageJson)

	apiURL := "https://api.push.oppomobile.com/server/v1/message/notification/unicast"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("auth_token", token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBuf, _ := io.ReadAll(resp.Body) // 仅在报错时读取具体内容
		global.LOG.Error("OPPO推送返回异常", zap.Int("status", resp.StatusCode), zap.String("body", string(respBuf)))
		return fmt.Errorf("oppo error: %d", resp.StatusCode)
	}

	return nil
}
