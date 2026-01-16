package infrastructure

import (
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"push-worker/internal/conf"
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

	// 3. 确保 Stream 存在 (同之前逻辑)
	if cfg.JetStream != nil {
		streamCfg := cfg.JetStream
		info, err := js.StreamInfo(streamCfg.StreamName)
		if err != nil || info == nil {
			log.Info("Stream 不存在，准备创建", zap.String("name", streamCfg.StreamName))

			storageType := nats.FileStorage
			if streamCfg.Storage == "memory" {
				storageType = nats.MemoryStorage
			}

			_, err = js.AddStream(&nats.StreamConfig{
				Name:      streamCfg.StreamName,
				Subjects:  streamCfg.Subjects,
				MaxMsgs:   streamCfg.MaxMsgs,
				MaxBytes:  streamCfg.MaxBytes,
				Storage:   storageType,
				Retention: nats.LimitsPolicy, // 消费者通常使用 Limits 配合 Ack
				Discard:   nats.DiscardOld,
			})
			if err != nil {
				log.Fatal("创建 Stream 失败", zap.Error(err))
			}
		}

		// 4. 新增：确保持久化消费者 (Durable Consumer) 存在
		// 这是消费者逻辑的核心：即使 worker 重启，也能从上次 Ack 的地方继续消费
		_, err = js.AddConsumer(streamCfg.StreamName, &nats.ConsumerConfig{
			Durable:       streamCfg.DurableName,  // 持久化名称: push_processor
			AckPolicy:     nats.AckExplicitPolicy, // 手动确认，确保消息真正处理完
			AckWait:       streamCfg.AckWait,      // 确认超时
			MaxDeliver:    streamCfg.MaxDeliver,   // 最大重试次数
			DeliverPolicy: nats.DeliverAllPolicy,  // 启动时从头开始消费未 Ack 的消息
		})
		if err != nil {
			log.Error("创建/更新消费者失败", zap.Error(err))
		} else {
			log.Info("NATS 消费者配置就绪", zap.String("durable", streamCfg.DurableName))
		}
	}

	cleanup := func() {
		_ = nc.Drain() // 优雅关闭：确保发出的 Ack 到达 Server 且不再接收新消息
	}

	return nc, js, cleanup
}
