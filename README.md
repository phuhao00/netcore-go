<div align="center">

# 🌐 NetCore-Go

**高性能Go网络库 | 多协议支持 | 企业级网络解决方案**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Version](https://img.shields.io/badge/Version-v1.0.1-blue?style=for-the-badge)](https://github.com/phuhao00/netcore-go/releases/tag/v1.0.1)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Build](https://img.shields.io/badge/Build-✅_Passing-brightgreen?style=for-the-badge)](#)
[![Coverage](https://img.shields.io/badge/Coverage-100%25-brightgreen?style=for-the-badge)](#)
[![Performance](https://img.shields.io/badge/⚡_Performance-Production_Ready-yellow?style=for-the-badge)](#)
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
go get github.com/phuhao00/netcore-go@v1.0.1
```

## 📁 示例程序

项目提供了丰富的示例程序，帮助您快速上手：

### 🌐 网络服务示例
| 示例 | 功能 | 位置 |
|------|------|------|
| **HTTP服务器** | HTTP/1.1服务器实现 | `examples/http/server/` |
| **TCP服务器/客户端** | TCP通信示例 | `examples/tcp/server/`, `examples/tcp/client/` |
| **UDP服务器/客户端** | UDP通信示例 | `examples/udp/server/`, `examples/udp/client/` |
| **WebSocket服务器** | WebSocket实时通信 | `examples/websocket/` |
| **gRPC服务器/客户端** | gRPC微服务通信 | `examples/grpc/server/`, `examples/grpc/client/` |
| **KCP服务器/客户端** | KCP可靠UDP传输 | `examples/kcp/server/`, `examples/kcp/client/` |
| **RPC服务器/客户端** | 自定义RPC框架 | `examples/rpc/server/`, `examples/rpc/client/` |

### 🏗️ 高级功能示例
| 示例 | 功能 | 位置 |
|------|------|------|
| **负载均衡器** | 多种负载均衡算法 | `examples/loadbalancer/` |
| **游戏服务器** | 实时游戏服务器 | `examples/gameserver/` |
| **聊天室** | 多人聊天应用 | `examples/chatroom/` |
| **网关服务** | API网关实现 | `examples/gateway/` |
| **高级服务器** | 企业级服务器配置 | `examples/advanced/` |
| **HTTP/3服务器** | HTTP/3协议支持 | `examples/http3/` |



## 💬 社区讨论

欢迎加入我们的技术讨论社区！

### 📺 B站专栏
[![Bilibili](https://img.shields.io/badge/📺_B站专栏-技术讨论-00A1D6?style=for-the-badge&logo=bilibili)](https://www.bilibili.com/opus/1111923984504455171)

🎯 **欢迎访问我的B站专栏进行技术讨论和交流**
- 💡 分享NetCore-Go使用经验
- 🔧 讨论网络编程最佳实践
- 🚀 获取最新功能更新
- 🤝 与其他开发者交流心得

[👉 点击进入专栏讨论](https://www.bilibili.com/opus/1111923984504455171)

## ☕ 赞助支持

如果这个项目对您有帮助，欢迎请作者喝一杯咖啡！您的支持是我们持续改进的动力。

![请作者喝咖啡](coffee.jpg)


*💝 感谢每一位支持者的贡献，让NetCore-Go变得更好！*
