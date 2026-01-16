# dgpush (High-Performance Push System)

`dgpush` 是一个基于 Go 语言开发的高性能、高可靠消息推送系统。采用 **Gateway + NATS + Worker** 的架构设计，旨在解决大规模物联网告警、即时通讯消息推送中的高并发与延迟挑战。

## 🚀 核心特性

- **高性能架构**：基于 `CloudWeGo/Hertz` 框架，集成 `panjf2000/ants` 协程池，支持万级 QPS 吞吐。
- **削峰填谷**：利用 `NATS JetStream` 实现消息持久化与流量缓冲，确保下游厂商 API 故障时消息不丢失。
- **全平台支持**：统一抽象 `Pusher` 接口，原生支持：
  - **主流厂商**：华为、小米、vivo、oppo、魅族、三星、荣耀。
  - **移动平台**：iOS (APNs)、HarmonyOS (鸿蒙)。
- **极致优化**：
  - 使用 `Sonic` (Bytedance) 进行高速 JSON 序列化。
  - 集成 `Rueidis` 实现高性能 Redis 客户端侧缓存（CSC）。
  - 对象池 (`sync.Pool`) 减少 GC 压力。
- **生产级监控**：完善的 `Zap` 日志体系、配置热加载及健康检查机制。

------

## 🏗️ 系统架构

1. **push-gateway** (接口层)：接收业务方的推送请求，进行参数校验和数据格式化，随后将消息推送到 NATS 集群。
2. **NATS JetStream** (中间件)：作为消息总线，负责消息的持久化存储和分发。
3. **push-worker** (处理层)：订阅 NATS 消息，根据用户设备类型自动分发至对应的厂商 API。

------

## 🛠️ 快速开始

### 1. 环境依赖

- Go 1.22+
- NATS Server (开启 JetStream)
- MySQL 8.0+
- Redis 6.2+

### 2. 配置说明

项目分为两个模块，分别需要配置：

- `push-gateway/configs/config.yaml`
- `push-worker/configs/config.yaml`

> **注意**：请将各厂商的证书文件放置于 `push-worker/scripts/v1/` 目录下，并确保路径与配置文件一致。

### 3. 运行项目

Bash

```
# 1. 克隆项目
git clone https://github.com/your-username/dgpush.git
cd dgpush

# 2. 启动网关 (Gateway)
cd push-gateway
go mod tidy
go run cmd/server/main.go

# 3. 启动工作进程 (Worker)
cd ../push-worker
go mod tidy
go run cmd/server/main.go
```

------

## 📁 目录结构

Plaintext

```
dgpush/
├── push-gateway/          # 接口层：处理 HTTP 请求，生产消息
│   ├── cmd/               # 入口程序
│   ├── internal/          # 内部逻辑 (HTTP/Infra/Conf)
│   └── models/            # 统一数据模型
├── push-worker/           # 处理层：消费消息，对接厂商 API
│   ├── push/              # 厂商 API 具体实现 (华为/小米/iOS等)
│   ├── scripts/v1/        # 推送证书存放地 (不建议上传至 Git)
│   └── service/           # 消息分发逻辑
└── ...
```

------

## 🛡️ 开源协议

[MIT License](https://www.google.com/search?q=LICENSE)