package infrastructure

import (
	"os"
	"path/filepath"
	"push-gateway/internal/conf"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitLogger(cfg *conf.ZapConfig) *zap.Logger {
	// 1. 设置日志轮转 (Lumberjack)
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Director, "server.log"),
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// 2. 配置 Encoder (JSON 格式对机器友好，性能更高)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 3. 开启异步写盘 (关键：防止 I/O 阻塞业务协程)
	// 注意：BufferedWriteSyncer 需要在程序退出时调用 Sync()
	var writer zapcore.WriteSyncer
	syncWriter := zapcore.AddSync(lumberJackLogger)
	if cfg.EnableAsync {
		writer = &zapcore.BufferedWriteSyncer{
			WS:            syncWriter,
			Size:          4096,             // 4KB 缓冲区
			FlushInterval: time.Second * 15, // 每15秒强制刷盘
		}
	} else {
		writer = syncWriter
	}

	// 4. 设置日志级别
	level := zap.InfoLevel
	_ = level.UnmarshalText([]byte(cfg.Level))

	// 5. 构建核心 Core
	core := zapcore.NewCore(encoder, writer, level)

	// 6. 开启日志采样 (关键：防止 10k QPS 时相同日志瞬间打满磁盘)
	// 采样规则：每秒内相同级别的日志，前 100 条正常记录，之后每 100 条记录 1 条
	samplerCore := zapcore.NewSamplerWithOptions(
		core,
		time.Second,
		100, // initial
		100, // thereafter
	)

	// 7. 合并控制台输出 (开发环境)
	var finalCore zapcore.Core = samplerCore
	if cfg.LogInConsole {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		finalCore = zapcore.NewTee(
			samplerCore,
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
		)
	}

	// 8. 创建 Logger
	logger := zap.New(finalCore)
	if cfg.ShowLine {
		logger = logger.WithOptions(zap.AddCaller()) // 仅在需要时开启，获取调用行号有微小性能损耗
	}

	return logger
}
