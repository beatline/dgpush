package v1

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"push-gateway/internal/http/middleware"
	"push-gateway/service"
)

func RegisterPushRoutes(r *server.Hertz) {
	v1 := r.Group("/api/v1/push", middleware.AccessLog())
	{
		v1.POST("/", service.PushMessage)
		v1.POST("/alarm", service.PushAlarmMessage)
		v1.GET("/status", service.GetPushStatus)
		v1.POST("/status", service.SetPushStatus)
	}
}
