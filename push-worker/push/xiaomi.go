package push

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"push-worker/global"
	"push-worker/models"

	"go.uber.org/zap"
)

type XiaomiPusher struct {
	config     *XiaomiConfig
	httpClient *http.Client
}

type XiaomiConfig struct {
	AppSecret   string `json:"app_secret"`
	PackageName string `json:"package_name"`
	ChannelID   string `json:"channel_id"`
}

// NewXiaomiPusher 按照项目统一风格，从配置文件初始化
func NewXiaomiPusher(path string) *XiaomiPusher {
	data, err := os.ReadFile(path)
	if err != nil {
		global.LOG.Fatal("读取小米证书失败", zap.String("path", path), zap.Error(err))
	}
	var cfg XiaomiConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		global.LOG.Fatal("解析小米配置失败", zap.Error(err))
	}

	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &XiaomiPusher{
		config: &cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

func (x *XiaomiPusher) Send(ctx context.Context, token string, data models.CommonMessage) error {
	apiURL := "https://api.xmpush.xiaomi.com/v3/message/regid"

	form := url.Values{}
	// 1. 必须关联正确的包名，否则下发会被拦截
	form.Set("restricted_package_name", x.config.PackageName)
	form.Set("registration_id", token)
	form.Set("title", data.Title)
	form.Set("description", data.Body)
	form.Set("payload", data.Extra)

	// 2. 这里的 notify_id 必须是整数的字符串，不能包含空格或特殊字符
	form.Set("notify_id", fmt.Sprintf("%d", time.Now().Unix()%2147483647))

	// 3. 渠道 ID 必须与小米后台申请的一致
	if x.config.ChannelID != "" {
		form.Set("extra.channel_id", x.config.ChannelID)
	}
	form.Set("extra.notify_effect", "1") // 1: 点击后打开通知栏

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	// 4. 关键修正：Authorization 必须以 "key=" 开头
	req.Header.Set("Authorization", x.config.AppSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := x.httpClient.Do(req)
	if err != nil {
		global.LOG.Error("小米推送请求失败", zap.Error(err))
		return err
	}

	// 5. 显式丢弃 Body 确保连接复用
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	respBuf, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		global.LOG.Error("小米服务端返回异常", zap.Int("status", resp.StatusCode), zap.String("body", string(respBuf)))
		return fmt.Errorf("xiaomi error: %d", resp.StatusCode)
	}

	return nil
}
