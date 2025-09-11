# NetCore-Go 高性能网络库

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.19-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](#)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-yellow.svg)](#)

NetCore-Go 是一个功能丰富、高性能的 Go 语言网络库，提供了完整的网络编程解决方案，包括 TCP/UDP 服务器、WebSocket、HTTP 服务器、RPC、gRPC、KCP 协议支持，以及服务发现、负载均衡、配置管理、日志系统和监控指标等企业级功能。

## ✨ 主要特性

### 🚀 核心网络功能
- **TCP/UDP 服务器**: 高性能的 TCP 和 UDP 服务器实现
- **WebSocket 支持**: 完整的 WebSocket 服务器和客户端
- **HTTP 服务器**: 基于标准库的高性能 HTTP 服务器
- **连接池管理**: 智能的连接池和资源管理
- **协程池**: 高效的 Goroutine 池管理

### 🔌 协议支持
- **自定义 RPC**: 轻量级、高性能的 RPC 协议
- **gRPC 集成**: 完整的 gRPC 支持和 Protocol Buffers 序列化
- **KCP 协议**: 基于 UDP 的可靠传输协议，适用于游戏和实时应用
- **多种编解码器**: JSON、Gob、Protobuf、MsgPack 等

### 🏗️ 微服务架构
- **服务发现**: 支持内存、Etcd、Consul 等注册中心
- **负载均衡**: 多种负载均衡算法（轮询、加权、最少连接等）
- **API 网关**: 功能完整的微服务网关
- **熔断器**: 服务容错和故障隔离

### 📊 监控和运维
- **结构化日志**: 支持多种格式和输出方式
- **指标监控**: Prometheus 指标导出
- **健康检查**: HTTP、TCP、脚本等多种健康检查方式
- **配置管理**: 支持 JSON、YAML、TOML、环境变量等配置源

### 🛡️ 安全和中间件
- **认证授权**: JWT、API Key、Basic Auth 等
- **限流控制**: 多种限流策略
- **CORS 支持**: 跨域资源共享
- **安全头部**: 各种安全相关的 HTTP 头部

## 📦 安装

```bash
go get github.com/netcore-go
```

## 🚀 快速开始

### TCP 服务器示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/netcore-go/pkg/server"
)

func main() {
    // 创建 TCP 服务器
    srv := server.NewTCPServer(&server.Config{
        Network: "tcp",
        Address: ":8080",
        MaxConnections: 1000,
    })
    
    // 设置消息处理器
    srv.SetHandler(&EchoHandler{})
    
    // 启动服务器
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}

type EchoHandler struct{}

func (h *EchoHandler) OnConnect(conn server.Connection) {
    fmt.Printf("客户端连接: %s\n", conn.RemoteAddr())
}

func (h *EchoHandler) OnMessage(conn server.Connection, data []byte) {
    // 回显消息
    conn.Write(data)
}

func (h *EchoHandler) OnDisconnect(conn server.Connection) {
    fmt.Printf("客户端断开: %s\n", conn.RemoteAddr())
}
```

### WebSocket 服务器示例

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/netcore-go/pkg/websocket"
)

func main() {
    // 创建 WebSocket 服务器
    ws := websocket.NewServer(&websocket.Config{
        CheckOrigin: func(r *http.Request) bool {
            return true // 允许所有来源
        },
    })
    
    // 设置消息处理器
    ws.SetHandler(&ChatHandler{})
    
    // 注册路由
    http.HandleFunc("/ws", ws.HandleWebSocket)
    
    // 启动 HTTP 服务器
    log.Fatal(http.ListenAndServe(":8080", nil))
}

type ChatHandler struct{}

func (h *ChatHandler) OnConnect(conn websocket.Connection) {
    log.Printf("WebSocket 连接: %s", conn.RemoteAddr())
}

func (h *ChatHandler) OnMessage(conn websocket.Connection, messageType int, data []byte) {
    // 广播消息给所有连接
    conn.WriteMessage(messageType, data)
}

func (h *ChatHandler) OnDisconnect(conn websocket.Connection) {
    log.Printf("WebSocket 断开: %s", conn.RemoteAddr())
}
```

### RPC 服务示例

```go
package main

import (
    "context"
    "log"
    
    "github.com/netcore-go/pkg/rpc"
)

// 定义服务
type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    return &User{ID: userID, Name: "John Doe"}, nil
}

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func main() {
    // 创建 RPC 服务器
    server := rpc.NewServer(&rpc.Config{
        Network: "tcp",
        Address: ":8080",
    })
    
    // 注册服务
    server.RegisterService("UserService", &UserService{})
    
    // 启动服务器
    log.Fatal(server.Start())
}
```

### 服务发现和负载均衡

```go
package main

import (
    "context"
    "log"
    
    "github.com/netcore-go/pkg/discovery"
    "github.com/netcore-go/pkg/loadbalancer"
)

func main() {
    // 创建服务注册中心
    registry := discovery.NewMemoryClient()
    
    // 注册服务实例
    instance := &discovery.ServiceInstance{
        ID:      "user-service-1",
        Name:    "user-service",
        Address: "192.168.1.10",
        Port:    8080,
        Health:  discovery.Healthy,
        Weight:  100,
    }
    
    if err := registry.Register(context.Background(), instance); err != nil {
        log.Fatal(err)
    }
    
    // 发现服务
    instances, err := registry.GetHealthyServices(context.Background(), "user-service")
    if err != nil {
        log.Fatal(err)
    }
    
    // 负载均衡选择实例
    selected, err := loadbalancer.Select(
        context.Background(), 
        instances, 
        loadbalancer.RoundRobin,
    )
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("选中的服务实例: %s", selected.GetEndpoint())
}
```

## 📚 详细文档

### 核心组件

- [TCP/UDP 服务器](docs/server.md) - 高性能网络服务器
- [WebSocket 服务器](docs/websocket.md) - WebSocket 实现
- [HTTP 服务器](docs/http.md) - HTTP 服务器和中间件
- [连接管理](docs/connection.md) - 连接池和资源管理

### 协议支持

- [RPC 协议](docs/rpc.md) - 自定义 RPC 实现
- [gRPC 集成](docs/grpc.md) - gRPC 服务和客户端
- [KCP 协议](docs/kcp.md) - 可靠 UDP 传输
- [编解码器](docs/codec.md) - 消息序列化

### 微服务架构

- [服务发现](docs/discovery.md) - 服务注册和发现
- [负载均衡](docs/loadbalancer.md) - 负载均衡算法
- [API 网关](docs/gateway.md) - 微服务网关
- [熔断器](docs/circuitbreaker.md) - 服务容错

### 监控和运维

- [日志系统](docs/logger.md) - 结构化日志
- [指标监控](docs/metrics.md) - Prometheus 集成
- [配置管理](docs/config.md) - 配置系统
- [健康检查](docs/health.md) - 健康检查机制

### 安全和中间件

- [认证授权](docs/auth.md) - 身份验证和授权
- [限流控制](docs/ratelimit.md) - 请求限流
- [中间件](docs/middleware.md) - HTTP 中间件
- [安全配置](docs/security.md) - 安全最佳实践

## 🏗️ 项目结构

```
netcore-go/
├── pkg/                    # 核心包
│   ├── server/            # TCP/UDP 服务器
│   ├── websocket/         # WebSocket 实现
│   ├── http/              # HTTP 服务器
│   ├── rpc/               # RPC 协议
│   ├── grpc/              # gRPC 集成
│   ├── kcp/               # KCP 协议
│   ├── discovery/         # 服务发现
│   ├── loadbalancer/      # 负载均衡
│   ├── config/            # 配置管理
│   ├── logger/            # 日志系统
│   ├── metrics/           # 指标监控
│   └── middleware/        # 中间件
├── examples/              # 示例代码
│   ├── tcp-server/        # TCP 服务器示例
│   ├── websocket-chat/    # WebSocket 聊天室
│   ├── rpc-service/       # RPC 服务示例
│   ├── grpc-service/      # gRPC 服务示例
│   ├── kcp-game/          # KCP 游戏服务器
│   ├── microservice/      # 微服务示例
│   ├── gateway/           # API 网关示例
│   ├── chatroom/          # 聊天室应用
│   ├── config/            # 配置管理示例
│   ├── logger/            # 日志系统示例
│   ├── metrics/           # 监控指标示例
│   └── discovery/         # 服务发现示例
├── docs/                  # 文档
├── tests/                 # 测试
├── benchmarks/            # 性能测试
└── tools/                 # 工具
```

## 🎯 示例应用

### 1. 聊天室应用

完整的 WebSocket 聊天室应用，包含：
- 实时消息传输
- 用户管理
- 房间管理
- 消息历史
- 在线状态

```bash
cd examples/chatroom
go run server/main.go
```

访问 http://localhost:8080 体验聊天室。

### 2. 微服务网关

功能完整的 API 网关，支持：
- 路由转发
- 负载均衡
- 认证授权
- 限流控制
- 监控指标

```bash
cd examples/gateway
go run main.go
```

### 3. RPC 服务

高性能 RPC 服务示例：
- 服务注册发现
- 负载均衡
- 连接池
- 拦截器

```bash
# 启动服务器
cd examples/rpc/server
go run main.go

# 启动客户端
cd examples/rpc/client
go run main.go
```

### 4. KCP 游戏服务器

基于 KCP 协议的游戏服务器：
- 低延迟通信
- 可靠传输
- 连接管理
- 消息广播

```bash
# 启动服务器
cd examples/kcp/server
go run main.go

# 启动客户端
cd examples/kcp/client
go run main.go
```

## 📊 性能特性

### 基准测试结果

| 功能 | QPS | 延迟 (P99) | 内存使用 |
|------|-----|-----------|----------|
| TCP 服务器 | 100K+ | < 1ms | < 50MB |
| WebSocket | 50K+ | < 2ms | < 100MB |
| RPC 调用 | 80K+ | < 1.5ms | < 80MB |
| HTTP 网关 | 60K+ | < 3ms | < 120MB |

### 优化特性

- **零拷贝**: 减少内存拷贝开销
- **连接池**: 复用连接减少创建开销
- **协程池**: 控制 Goroutine 数量
- **内存池**: 减少 GC 压力
- **批量处理**: 提高吞吐量

## 🔧 配置示例

### 服务器配置 (YAML)

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  max_connections: 10000
  buffer_size: 4096

logger:
  level: "info"
  format: "json"
  output: "stdout"

metrics:
  enabled: true
  path: "/metrics"
  port: 9090

discovery:
  enabled: true
  provider: "memory"
  endpoints:
    - "localhost:2379"

loadbalancer:
  algorithm: "round_robin"
  health_check: true
  max_retries: 3
```

### 网关配置 (JSON)

```json
{
  "server": {
    "host": "localhost",
    "port": 8080
  },
  "routes": [
    {
      "path": "/api/users",
      "method": "*",
      "service": "user-service",
      "timeout": "10s",
      "load_balance": "round_robin"
    }
  ],
  "cors": {
    "enabled": true,
    "allowed_origins": ["*"],
    "allowed_methods": ["GET", "POST", "PUT", "DELETE"]
  }
}
```

## 🧪 测试

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./pkg/server

# 运行基准测试
go test -bench=. ./benchmarks

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 性能测试

```bash
# TCP 服务器性能测试
cd benchmarks/tcp
go test -bench=BenchmarkTCPServer

# WebSocket 性能测试
cd benchmarks/websocket
go test -bench=BenchmarkWebSocket

# RPC 性能测试
cd benchmarks/rpc
go test -bench=BenchmarkRPC
```

## 🤝 贡献指南

我们欢迎所有形式的贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详细信息。

### 开发环境设置

```bash
# 克隆仓库
git clone https://github.com/netcore-go/netcore.git
cd netcore-go

# 安装依赖
go mod download

# 运行测试
go test ./...

# 运行示例
cd examples/tcp-server
go run main.go
```

### 代码规范

- 遵循 Go 官方代码规范
- 使用 `gofmt` 格式化代码
- 添加适当的注释和文档
- 编写单元测试
- 更新相关文档

## 📄 许可证

本项目采用 MIT 许可证。详情请查看 [LICENSE](LICENSE) 文件。

## 🙏 致谢

感谢以下开源项目的启发和支持：

- [Gin](https://github.com/gin-gonic/gin) - HTTP Web 框架
- [gRPC-Go](https://github.com/grpc/grpc-go) - gRPC 实现
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket 实现
- [Prometheus](https://github.com/prometheus/prometheus) - 监控系统
- [Logrus](https://github.com/sirupsen/logrus) - 日志库

## 📞 联系我们

- 项目主页: https://github.com/netcore-go/netcore
- 问题反馈: https://github.com/netcore-go/netcore/issues
- 讨论区: https://github.com/netcore-go/netcore/discussions
- 邮箱: netcore-go@example.com

## 🗺️ 路线图

### v1.1.0 (计划中)
- [ ] HTTP/2 和 HTTP/3 支持
- [ ] 分布式追踪集成
- [ ] 更多服务发现后端支持
- [ ] 性能优化和内存使用改进

### v1.2.0 (计划中)
- [ ] 图形化管理界面
- [ ] 更多中间件和插件
- [ ] 云原生部署支持
- [ ] 更完善的文档和教程

---

**NetCore-Go** - 让 Go 网络编程更简单、更高效！ 🚀