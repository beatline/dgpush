package infrastructure

import (
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"push-gateway/internal/conf"
)

func InitNATS(cfg *conf.NATSConfig, log *zap.Logger) (*nats.Conn, nats.JetStreamContext, func()) {
	// 1. 建立基础连接
	nc, err := nats.Connect(cfg.Url,
		nats.MaxReconnects(cfg.MaxReconnect),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.ErrorHandler(func(nc *nats.Conn, s *nats.Subscription, err error) {
			log.Error("NATS 异步错误", zap.Error(err), zap.String("subject", s.Subject))
		}),
	)
	if err != nil {
		log.Fatal("NATS 连接失败", zap.Error(err))
	}

	// 2. 初始化 JetStream 上下文
	js, err := nc.JetStream()
	if err != nil {
		log.Fatal("JetStream 初始化失败", zap.Error(err))
	}

	// 3. 检查并自动创建 Stream (新增逻辑)
	if cfg.JetStream != nil {
		streamCfg := cfg.JetStream

		// 检查 Stream 是否已存在
		info, err := js.StreamInfo(streamCfg.StreamName)

		// 如果不存在则创建
		if err != nil || info == nil {
			log.Info("Stream 不存在，准备创建", zap.String("name", streamCfg.StreamName))

			// 设置存储模式：文件(FileStorage) 或 内存(MemoryStorage)
			storageType := nats.FileStorage
			if streamCfg.Storage == "memory" {
				storageType = nats.MemoryStorage
			}

			_, err = js.AddStream(&nats.StreamConfig{
				Name:      streamCfg.StreamName,
				Subjects:  streamCfg.Subjects, // 匹配 config.yaml 中的 push.task.>
				MaxMsgs:   streamCfg.MaxMsgs,  // 最大消息数
				MaxBytes:  streamCfg.MaxBytes, // 最大字节数
				Storage:   storageType,        // 持久化方式
				Retention: nats.LimitsPolicy,  // 使用 Limits 模式，替代之前的 WorkQueue 模式
				Discard:   nats.DiscardOld,    // 队列满时丢弃旧消息
			})

			if err != nil {
				log.Fatal("创建 Stream 失败", zap.Error(err))
			}
			log.Info("Stream 创建成功", zap.String("name", streamCfg.StreamName))
		} else {
			log.Info("Stream 已存在，跳过创建", zap.String("name", streamCfg.StreamName))
		}
	}

	cleanup := func() {
		_ = nc.Drain() // 优雅关闭
	}

	return nc, js, cleanup
}
