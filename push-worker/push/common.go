package push

import (
	"context"
	"go.uber.org/zap"
	"push-worker/global"
	"push-worker/models"
)

// NoopPusher 是一个空对象实现，用于处理暂时不支持的厂商
type NoopPusher struct {
	BrandName string
}

func (n *NoopPusher) Send(ctx context.Context, regId string, data models.CommonMessage) error {
	// 仅记录日志，不执行任何实际推送
	global.LOG.Debug("暂不支持该厂商推送，消息已跳过",
		zap.String("brand", n.BrandName),
		zap.Int64("user_id", data.UserID))
	return nil
}
