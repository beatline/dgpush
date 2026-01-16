# dgpush

[![Go CI/CD](https://github.com/beatline/dgpush/actions/workflows/go-ci.yml/badge.svg)](https://github.com/beatline/dgpush/actions/workflows/go-ci.yml)

dgpush 是一个基于 Go 语言开发的高性能、高可靠消息推送系统。采用 Gateway + NATS + Worker 的架构设计，旨在解决大规模物联网告警、即时通讯消息推送中的高并发与延迟挑战。

## Architecture

- **push-gateway**: 接收 HTTP 请求并生产消息到 NATS
- **push-worker**: 消费消息并分发到各移动推送服务商 (APNs, HMS, FCM)

## Development

### Prerequisites

- Go 1.20 or higher
- NATS server

### Building

```bash
# Build push-gateway
go build -o bin/push-gateway ./cmd/push-gateway

# Build push-worker
go build -o bin/push-worker ./cmd/push-worker
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
```

### Linting

The project uses [golangci-lint](https://golangci-lint.run/) for code quality checks:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

## CI/CD

This project uses GitHub Actions for continuous integration and deployment. The workflow includes:

- **Linting**: Code quality checks with golangci-lint
- **Building**: Multi-version Go builds (1.20, 1.21, 1.22)
- **Testing**: Unit tests with race detection and coverage reporting

Workflows run automatically on:
- Push to `main`, `master`, or `develop` branches
- Pull requests to `main`, `master`, or `develop` branches
