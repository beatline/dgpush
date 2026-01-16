package main

import (
	"context"
	"os"
	"os/signal"
	"push-worker/global"
	"push-worker/internal/conf"
	"push-worker/internal/infrastructure"
	"push-worker/push"
	"push-worker/service"
	"syscall"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

func main() {
	// 1. 初始化配置
	global.CONFIG = conf.InitConfig("configs/config.yaml")

	// 2. 初始化日志 (开启异步写盘与高并发采样)
	global.LOG = infrastructure.InitLogger(global.CONFIG.Zap)
	defer global.LOG.Sync()

	// 3. 初始化 MySQL
	db, dbCleanup := infrastructure.InitMySQL(global.CONFIG.MySQL)
	global.DB = db
	defer dbCleanup()

	// 4. 初始化 Redis (开启客户端侧缓存 CSC)
	rdb, rdbCleanup := infrastructure.InitRedis(global.CONFIG.Redis)
	global.REDIS = rdb
	defer rdbCleanup()

	// 5. 初始化 NATS (创建 Stream 和 Durable 消费者)
	nc, js, natsCleanup := infrastructure.InitNATS(global.CONFIG.NATS, global.LOG)
	global.NATS = nc
	global.JS = js
	defer natsCleanup()

	// 6. 初始化ants协程池 (容量 10,000 应对 10k QPS)
	pool, err := ants.NewPool(10000, ants.WithPreAlloc(true))
	if err != nil {
		global.LOG.Fatal("ants 协程池初始化失败", zap.Error(err))
	}
	global.MsgPool = pool
	defer pool.Release()

	// 7. 初始化推送组件 (加载所有厂商证书)
	push.InitAllPushers("scripts/v1")

	// 8. 启动消息消费逻辑 (非阻塞)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartPushWorker(ctx)

	global.LOG.Info("Push-Worker 服务器已就绪")

	// 9. 优雅退出信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	global.LOG.Info("正在优雅关闭服务...")
	time.Sleep(2 * time.Second) // 留出时间处理余下的 Ack
}
