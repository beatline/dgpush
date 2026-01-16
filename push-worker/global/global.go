package global

import (
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
	"push-worker/internal/conf"
)

var (
	CONFIG  *conf.Config
	LOG     *zap.Logger
	DB      *sqlx.DB
	REDIS   rueidis.Client
	NATS    *nats.Conn
	JS      nats.JetStreamContext
	MsgPool *ants.Pool
)
