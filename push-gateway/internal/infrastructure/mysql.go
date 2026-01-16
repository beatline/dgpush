package infrastructure

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"push-gateway/internal/conf"
	"time"
)

func InitMySQL(cfg *conf.MySQLConfig) (*sqlx.DB, func()) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.DbName, cfg.Config)

	// 优化：使用 Open 而非 MustConnect，配合 Ping 检查，避免直接 panic 难以排查
	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		panic(fmt.Sprintf("MySQL 驱动初始化失败: %v", err))
	}

	// 检查连接是否可用
	if err := db.Ping(); err != nil {
		panic(fmt.Sprintf("MySQL 连接失败: %v", err))
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	cleanup := func() {
		_ = db.Close()
	}
	return db, cleanup
}
