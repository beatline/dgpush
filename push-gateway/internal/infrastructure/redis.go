package infrastructure

import (
	"context"
	"fmt"
	"github.com/redis/rueidis"
	"push-gateway/internal/conf"
	"time"
)

func InitRedis(cfg *conf.RedisConfig) (rueidis.Client, func()) {
	// 构建连接选项
	option := rueidis.ClientOption{
		InitAddress:      []string{fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port)},
		Password:         cfg.Password,
		Username:         cfg.Username, // 检查此用户名是否在阿里云控制台已创建
		DisableCache:     !cfg.ClientCache,
		SelectDB:         0, // 默认使用 DB 0
		ConnWriteTimeout: cfg.WriteTimeout,
	}

	client, err := rueidis.NewClient(option)
	if err != nil {
		// 捕获驱动创建阶段的错误
		panic(fmt.Sprintf("Redis 客户端创建失败: %v", err))
	}

	// 优化：增加 Ping 检查，确保认证通过
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		// 这里会捕获到具体的 WRONGPASS 错误
		panic(fmt.Sprintf("Redis 连接认证失败 (请核对用户名密码): %v", err))
	}

	cleanup := func() {
		client.Close()
	}
	return client, cleanup
}
