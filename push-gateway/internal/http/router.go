package http

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	v1 "push-gateway/internal/http/v1"
)

// InitRouter 初始化总路由
func InitRouter(h *server.Hertz) {
	v1.RegisterPushRoutes(h)
}
