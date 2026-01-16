package service

import (
	"context"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"push-worker/global"
)

// StartPushWorker 启动消息消费循环
func StartPushWorker(ctx context.Context) {
	// 绑定到配置好的持久化消费者 push_processor
	sub, err := global.JS.PullSubscribe(global.CONFIG.NATS.JetStream.Subjects[0], global.CONFIG.NATS.JetStream.DurableName)
	if err != nil {
		global.LOG.Fatal("NATS 订阅失败", zap.Error(err))
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 批量拉取 100 条消息以提高吞吐量
				msgs, err := sub.Fetch(100, nats.Context(ctx))
				if err != nil {
					continue
				}

				for _, msg := range msgs {
					m := msg
					// 提交给 ants 协程池处理具体业务
					_ = global.MsgPool.Submit(func() {
						handleTask(m)
					})
				}
			}
		}
	}()
}
