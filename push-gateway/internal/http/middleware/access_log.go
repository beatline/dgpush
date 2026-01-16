package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
	"push-gateway/global"
	"time"
)

func AccessLog() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()

		// 向下执行业务逻辑
		c.Next(ctx)

		// 只有在请求完成后记录一次全量日志
		end := time.Now()
		latency := end.Sub(start)
		statusCode := c.Response.StatusCode()

		// 使用 zap 的强类型字段，避免反射开销
		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", string(c.Method())),
			zap.String("path", string(c.Path())),
			zap.String("ip", c.ClientIP()),
			zap.Duration("latency", latency),
			zap.Time("req_time", start), // 接收请求的时间
		}

		// 只有在错误或告警时，才记录参数信息，减少正常流量下的负担
		if statusCode >= 400 {
			fields = append(fields, zap.ByteString("query", c.Request.QueryString()))
			//// 在高并发下，c.Request.Body() 会导致内存拷贝，仅在必要时开启
			//fields = append(fields, zap.ByteString("body", c.Request.Body()))
		}

		global.LOG.Info("access_log", fields...)
	}
}
