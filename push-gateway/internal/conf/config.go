package conf

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server *ServerConfig `mapstructure:"server"`
	Zap    *ZapConfig    `mapstructure:"zap"`
	MySQL  *MySQLConfig  `mapstructure:"mysql"`
	Redis  *RedisConfig  `mapstructure:"redis"`
	NATS   *NATSConfig   `mapstructure:"nats"`
}

type ServerConfig struct {
	Name string `mapstructure:"name"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Env  string `mapstructure:"env"`
}

type ZapConfig struct {
	Level        string `mapstructure:"level"`
	Format       string `mapstructure:"format"`
	Director     string `mapstructure:"director"`
	EncodeLevel  string `mapstructure:"encode_level"`
	EncodeTime   string `mapstructure:"encode_time"`
	ShowLine     bool   `mapstructure:"show_line"`
	LogInConsole bool   `mapstructure:"log_in_console"`
	EnableAsync  bool   `mapstructure:"enable_async"`
	MaxSize      int    `mapstructure:"max_size"`
	MaxBackups   int    `mapstructure:"max_backups"`
	MaxAge       int    `mapstructure:"max_age"`
	Compress     bool   `mapstructure:"compress"`
}

type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Config          string `mapstructure:"config"`
	DbName          string `mapstructure:"db_name"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	LogZap          bool   `mapstructure:"log_zap"`
	Prefix          string `mapstructure:"prefix"`
	Singular        bool   `mapstructure:"singular"`
	LogMode         string `mapstructure:"log_mode"`
}

type RedisConfig struct {
	Addr            string        `mapstructure:"addr"`
	Port            int           `mapstructure:"port"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	PoolSize        int           `mapstructure:"pool_size"`
	ConnMaxLifetime int           `mapstructure:"conn_max_lifetime"`
	ClientCache     bool          `mapstructure:"client_cache"`
	ClientCacheSize string        `mapstructure:"client_cache_size"`
	DialTimeout     time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
}

type NATSConfig struct {
	Url           string           `mapstructure:"url"`
	MaxReconnect  int              `mapstructure:"max_reconnect"`
	ReconnectWait time.Duration    `mapstructure:"reconnect_wait"`
	DialTimeout   time.Duration    `mapstructure:"dial_timeout"`
	JetStream     *JetStreamConfig `mapstructure:"jetstream"`
}

type JetStreamConfig struct {
	StreamName  string        `mapstructure:"stream_name"`
	Subjects    []string      `mapstructure:"subjects"`
	MaxMsgs     int64         `mapstructure:"max_msgs"`
	MaxBytes    int64         `mapstructure:"max_bytes"`
	Storage     string        `mapstructure:"storage"`
	DurableName string        `mapstructure:"durable_name"`
	AckWait     time.Duration `mapstructure:"ack_wait"`
	MaxDeliver  int           `mapstructure:"max_deliver"`
}

// GlobalConfig 全局变量
var GlobalConfig = &Config{}

func InitConfig(filePath string) *Config {
	v := viper.New()
	v.SetConfigFile(filePath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	// 监听配置文件变化
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config file changed:", e.Name)
		if err := v.Unmarshal(GlobalConfig); err != nil {
			fmt.Println("unmarshal config error:", err)
		}
	})

	if err := v.Unmarshal(GlobalConfig); err != nil {
		panic(fmt.Errorf("unmarshal config error: %s", err))
	}

	return GlobalConfig
}
