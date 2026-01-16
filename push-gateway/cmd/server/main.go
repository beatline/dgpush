package main

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"push-gateway/global"
	"push-gateway/internal/conf"
	"push-gateway/internal/http"
	"push-gateway/internal/infrastructure"
)

func main() {
	// 1. 加载配置
	global.CONFIG = conf.InitConfig("configs/config.yaml")

	// 2. 初始化日志
	global.LOG = infrastructure.InitLogger(global.CONFIG.Zap)

	// 3. 初始化 MySQL
	db, dbCleanup := infrastructure.InitMySQL(global.CONFIG.MySQL)
	global.DB = db
	defer dbCleanup()

	// 4. 初始化 Redis
	rdb, rdbCleanup := infrastructure.InitRedis(global.CONFIG.Redis)
	global.REDIS = rdb
	defer rdbCleanup()

	// 5. 初始化 NATS
	nc, js, natsCleanup := infrastructure.InitNATS(global.CONFIG.NATS, global.LOG)
	global.NATS = nc
	global.JS = js
	defer natsCleanup()

	// 初始化 ants 协程池
	pool, err := ants.NewPool(1000)
	if err != nil {
		global.LOG.Fatal("初始化协程池失败", zap.Error(err))
	}
	global.MsgPool = pool
	// 程序退出时释放资源
	defer pool.Release()

	// 6. 启动 Hertz
	h := server.Default(server.WithHostPorts(fmt.Sprintf("%s:%d", global.CONFIG.Server.Host, global.CONFIG.Server.Port)))

	// 7. 注册路由
	http.InitRouter(h)

	global.LOG.Info("Push-Gateway 服务器已就绪")
	h.Spin()
}
