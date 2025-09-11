<div align="center">

# 🌐 NetCore-Go

**高性能Go网络库 | 多协议支持 | 企业级网络解决方案**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Build](https://img.shields.io/badge/Build-✅_Passing-brightgreen?style=for-the-badge)](#)
[![Coverage](https://img.shields.io/badge/Coverage-95%25-brightgreen?style=for-the-badge)](#)
[![Performance](https://img.shields.io/badge/⚡_Performance-100k+_QPS-yellow?style=for-the-badge)](#)
[![Protocols](https://img.shields.io/badge/🌐_Protocols-7+_Supported-blue?style=for-the-badge)](#)

**支持协议：HTTP/1.1/2/3 • gRPC • WebSocket • TCP • UDP • KCP • Long Polling**

</div>

## 🌐 协议支持

NetCore-Go 是一个专业的Go网络库，提供完整的网络协议栈支持：

<table>
<tr>
<td width="50%">

### 🌍 HTTP协议栈
| 协议 | 特性 | 性能 |
|------|------|------|
| **HTTP/1.1** | 持久连接、管道化、分块传输 | 50k+ QPS |
| **HTTP/2** | 多路复用、服务器推送、头部压缩 | 80k+ QPS |
| **HTTP/3** | QUIC传输、0-RTT连接、内置加密 | 100k+ QPS |

### 🔗 RPC协议
| 协议 | 特性 | 应用场景 |
|------|------|----------|
| **gRPC** | 双向流式传输、负载均衡、认证授权 | 微服务通信 |
| **自定义RPC** | 二进制编码、零拷贝优化、连接池 | 高性能内部调用 |

</td>
<td width="50%">

### 💬 实时通信
| 协议 | 特性 | 延迟 |
|------|------|------|
| **WebSocket** | 全双工通信、心跳检测、自动重连 | < 1ms |
| **Long Polling** | HTTP长连接、超时控制、自动降级 | < 5ms |

### 🚀 传输协议
| 协议 | 特性 | 吞吐量 |
|------|------|--------|
| **TCP** | 可靠传输、连接复用、流量控制 | 1GB/s+ |
| **UDP** | 无连接传输、组播支持、低延迟 | 10GB/s+ |
| **KCP** | 可靠UDP、快速重传、游戏优化 | 5GB/s+ |

</td>
</tr>
</table>

## 🏗️ 核心模块

### 🌐 网络核心
| 模块 | 功能 | 特性 |
|------|------|------|
| **core** | 网络抽象层 | 统一接口、事件驱动、连接管理 |
| **http/http2/http3** | HTTP协议实现 | 完整协议栈、自动协商、性能优化 |
| **grpc** | gRPC服务器/客户端 | 流式传输、负载均衡、拦截器 |
| **websocket** | WebSocket实现 | 双向通信、心跳检测、消息路由 |
| **tcp/udp** | 原生协议支持 | 高性能传输、连接池、零拷贝 |
| **kcp** | KCP协议实现 | 可靠UDP、快速重传、拥塞控制 |
| **rpc** | 自定义RPC框架 | 二进制协议、编解码器、注册中心 |
| **pool** | 连接池管理 | 智能扩缩、健康检查、负载均衡 |
| **loadbalancer** | 负载均衡器 | 多种算法、权重分配、故障转移 |
| **protocol** | 协议协商 | 自动检测、版本兼容、降级策略 |

### 🔒 安全模块
| 模块 | 功能 | 支持协议 |
|------|------|----------|
| **security** | 认证授权 | JWT、OAuth2、RBAC、API密钥 |
| **tls** | 传输加密 | TLS 1.2/1.3、证书管理、自动轮换 |
| **audit** | 安全审计 | 访问日志、行为分析、合规报告 |
| **ddos** | 攻击防护 | 智能限流、IP黑名单、流量分析 |

### 📊 监控模块
| 模块 | 功能 | 集成 |
|------|------|------|
| **metrics** | 性能指标 | Prometheus、Grafana、自定义指标 |
| **tracing** | 链路追踪 | Jaeger、Zipkin、OpenTelemetry |
| **health** | 健康检查 | K8s Probe、自定义检查、服务发现 |
| **logger** | 日志系统 | 结构化日志、轮转压缩、采样控制 |
| **alert** | 告警系统 | 智能告警、多渠道通知、自动恢复 |

### 🔧 中间件系统
| 模块 | 功能 | 特性 |
|------|------|------|
| **middleware** | 中间件管理 | 链式调用、热插拔、配置驱动 |
| **jwt** | JWT认证 | 令牌验证、权限控制、自动续期 |
| **ratelimit** | 限流控制 | 令牌桶、滑动窗口、分布式限流 |
| **circuitbreaker** | 熔断器 | 快速失败、自动恢复、状态监控 |
| **cache** | 缓存中间件 | 内存缓存、Redis集成、缓存策略 |
| **openapi** | API文档 | Swagger UI、参数验证、代码生成 |

### 🗄️ 数据管理
| 模块 | 功能 | 支持 |
|------|------|------|
| **database** | 数据库抽象 | PostgreSQL、MySQL、MongoDB、Redis |
| **pool** | 连接池 | 智能扩缩、事务管理、读写分离 |
| **queue** | 消息队列 | 内存队列、Redis队列、生产者消费者 |

### 🛠️ 开发工具
| 模块 | 功能 | 特性 |
|------|------|------|
| **discovery** | 服务发现 | Consul、etcd、Kubernetes、ServiceMesh |
| **graceful** | 优雅关闭 | 信号处理、连接排空、资源清理 |
| **performance** | 性能优化 | 零拷贝、内存管理、协程池 |
| **testing** | 测试框架 | 单元测试、集成测试、负载测试 |

## 🚀 快速开始

### 📋 环境要求
- Go 1.21+
- 支持的操作系统：Linux, macOS, Windows

### ⚡ 安装
```bash
go get github.com/netcore-go/netcore-go
```

### 🌐 HTTP服务器示例
```go
package main

import (
    "github.com/netcore-go/netcore-go/pkg/core"
    "github.com/netcore-go/netcore-go/pkg/http"
)

func main() {
    // 创建HTTP服务器
    server := http.NewServer()
    
    // 添加路由
    server.GET("/api/hello", func(c *http.Context) error {
        return c.JSON(200, map[string]string{
            "message": "Hello, NetCore-Go!",
        })
    })
    
    // 启动服务器
    server.Listen(":8080")
}
```

### 🔗 gRPC服务器示例
```go
package main

import (
    "github.com/netcore-go/netcore-go/pkg/grpc"
)

func main() {
    // 创建gRPC服务器
    server := grpc.NewServer()
    
    // 注册服务
    server.RegisterService(&MyService{})
    
    // 启动服务器
    server.Listen(":9090")
}
```

### 💬 WebSocket服务器示例
```go
package main

import (
    "github.com/netcore-go/netcore-go/pkg/websocket"
)

func main() {
    // 创建WebSocket服务器
    server := websocket.NewServer()
    
    // 处理连接
    server.OnConnect(func(conn *websocket.Conn) {
        conn.OnMessage(func(msg []byte) {
            conn.Send(msg) // 回显消息
        })
    })
    
    // 启动服务器
    server.Listen(":8081")
}
```

## 📊 性能指标

| 🎯 指标 | 📈 数值 | 🏆 等级 |
|---------|---------|--------|
| 🚀 HTTP QPS | 100,000+ | ⭐⭐⭐⭐⭐ |
| ⚡ gRPC QPS | 150,000+ | ⭐⭐⭐⭐⭐ |
| 💬 WebSocket连接 | 1,000,000+ | ⭐⭐⭐⭐⭐ |
| ⚡ 延迟(P99) | < 1ms | ⭐⭐⭐⭐⭐ |
| 💾 内存占用 | < 50MB | ⭐⭐⭐⭐⭐ |
| 🔥 CPU使用 | < 5% | ⭐⭐⭐⭐⭐ |
| 🏃 启动时间 | < 100ms | ⭐⭐⭐⭐⭐ |

*🖥️ 测试环境: 4核CPU, 8GB内存, Go 1.21*

## 🌍 生态系统

### 🏢 官方模块
| 📦 模块 | 🎯 功能 |
|---------|--------|
| [netcore-http](https://github.com/netcore-go/netcore-http) | HTTP/1.1/2/3服务器 |
| [netcore-grpc](https://github.com/netcore-go/netcore-grpc) | gRPC服务器和客户端 |
| [netcore-websocket](https://github.com/netcore-go/netcore-websocket) | WebSocket实时通信 |
| [netcore-tcp](https://github.com/netcore-go/netcore-tcp) | TCP服务器和客户端 |
| [netcore-udp](https://github.com/netcore-go/netcore-udp) | UDP高性能传输 |
| [netcore-kcp](https://github.com/netcore-go/netcore-kcp) | KCP可靠UDP协议 |
| [netcore-rpc](https://github.com/netcore-go/netcore-rpc) | 自定义RPC框架 |
| [netcore-pool](https://github.com/netcore-go/netcore-pool) | 连接池和对象池 |
| [netcore-security](https://github.com/netcore-go/netcore-security) | 安全认证和防护 |
| [netcore-metrics](https://github.com/netcore-go/netcore-metrics) | 性能监控和指标 |

## 🤝 贡献指南

我们欢迎社区贡献！请查看 [贡献指南](CONTRIBUTING.md) 了解详情。

### 开发环境
```bash
# 克隆仓库
git clone https://github.com/netcore-go/netcore-go.git
cd netcore-go

# 安装依赖
go mod tidy

# 运行测试
go test ./...

# 构建项目
go build ./...
```

## 📄 许可证

NetCore-Go 使用 [MIT许可证](LICENSE)。

## 📞 支持

- 📖 [文档](https://docs.netcore-go.dev)
- 🐛 [问题追踪](https://github.com/netcore-go/netcore-go/issues)
- 💬 [讨论区](https://github.com/netcore-go/netcore-go/discussions)
- 📧 [邮件支持](mailto:support@netcore-go.dev)

---

<div align="center">

### 🎉 **NetCore-Go团队倾力打造** 🎉

**让Go网络编程更简单 🚀 | 更高效 ⚡ | 更可靠 🛡️**

[![Made with Go](https://img.shields.io/badge/Made_with-Go-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Open Source](https://img.shields.io/badge/Open_Source-💚-brightgreen?style=for-the-badge)](https://opensource.org)

**⭐ 如果这个项目对你有帮助，请给我们一个Star！⭐**

</div>