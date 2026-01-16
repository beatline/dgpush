package models

import "context"

// Pusher 定义推送统一接口
type Pusher interface {
	Send(ctx context.Context, deviceToken string, data CommonMessage) error
}

type IosKeyConfig struct {
	TeamID     string `json:"team_id"`
	KeyID      string `json:"key_id"`
	BundleID   string `json:"bundle_id"`
	PrivateKey string `json:"private_key"` // 存放 PEM 格式的私钥字符串
	Sandbox    bool   `json:"sandbox"`     // 是否为沙盒环境
}
