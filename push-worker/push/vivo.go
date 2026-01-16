package push

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"push-worker/global"
	"push-worker/models"

	"github.com/bytedance/sonic"
	"go.uber.org/zap"
)

type VivoPusher struct {
	config     *VivoConfig
	authToken  string
	expiry     time.Time
	httpClient *http.Client
	mu         sync.RWMutex
}

type VivoConfig struct {
	AppId     int    `json:"app_id"`
	AppKey    string `json:"app_key"`
	AppSecret string `json:"app_secret"`
}

func NewVivoPusher(path string) *VivoPusher {
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("读取VIVO证书失败", zap.Error(err))
	}
	var cfg VivoConfig
	_ = json.Unmarshal(data, &cfg)

	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &VivoPusher{
		config: &cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

func (v *VivoPusher) getToken(ctx context.Context) (string, error) {
	v.mu.RLock()
	if v.authToken != "" && time.Now().Before(v.expiry) {
		defer v.mu.RUnlock()
		return v.authToken, nil
	}
	v.mu.RUnlock()

	apiURL := "https://api-push.vivo.com.cn/message/auth"
	// 2. 构造鉴权请求
	timestamp := time.Now().UnixMilli()
	// 签名规则: MD5(appId + appKey + timestamp + appSecret)
	signStr := fmt.Sprintf("%d%s%d%s", v.config.AppId, v.config.AppKey, timestamp, v.config.AppSecret)
	signHash := md5.Sum([]byte(signStr))
	sign := hex.EncodeToString(signHash[:])
	// 按照 vivo 官方要求构造鉴权请求体
	payload := map[string]any{
		"appId":     v.config.AppId,
		"appKey":    v.config.AppKey,
		"timestamp": timestamp,
		"sign":      sign,
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Result    int    `json:"result"`
		AuthToken string `json:"authToken"`
		Desc      string `json:"desc"`
	}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &res)

	if res.Result != 0 {
		return "", fmt.Errorf("vivo auth fail: %d", res.Result)
	}

	v.authToken = res.AuthToken
	v.expiry = time.Now().Add(24 * time.Hour).Add(-10 * time.Minute)
	return v.authToken, nil
}

func (v *VivoPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	token, _ := v.getToken(ctx)
	apiURL := "https://api-push.vivo.com.cn/message/send"
	payload := map[string]any{
		"appId":          v.config.AppId, // 必须：从配置中读取 appId
		"regId":          regId,
		"notifyType":     4, // 4: 响铃和振动
		"title":          data.Title,
		"content":        data.Body,
		"skipType":       1,                // 1: 打开APP首页
		"requestId":      time.Now().UTC(), // 必须：请求唯一标识
		"classification": 1,                // 1: 系统类消息（避免进入运营消息盒）
		"pushMode":       0,                // 0: 正式推送
	}

	bodyBytes, _ := sonic.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	req.Header.Set("authToken", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBuf, _ := io.ReadAll(resp.Body) // 仅在报错时读取具体内容
		global.LOG.Error("VIVO推送返回异常", zap.Int("status", resp.StatusCode), zap.String("body", string(respBuf)))
		return fmt.Errorf("vivo error: %d", resp.StatusCode)
	}
	return nil
}
