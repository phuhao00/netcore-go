# NetCore-Go 高级功能指南

本文档介绍 NetCore-Go 框架的高级功能模块，包括安全性增强、性能优化、监控告警、集群支持等企业级特性。

## 目录

- [安全性增强](#安全性增强)
- [性能优化](#性能优化)
- [监控和告警](#监控和告警)
- [集群和分布式支持](#集群和分布式支持)
- [使用示例](#使用示例)
- [配置参考](#配置参考)

## 安全性增强

### TLS/SSL 支持

提供完整的 TLS/SSL 加密传输支持，包括证书管理、自动更新和自签名证书生成。

```go
import "github.com/netcore-go/pkg/security"

// 创建 TLS 管理器
tlsConfig := security.DefaultTLSConfig()
tlsConfig.CertFile = "server.crt"
tlsConfig.KeyFile = "server.key"
tlsConfig.AutoReload = true

tlsManager := security.NewTLSManager(tlsConfig)
if err := tlsManager.Start(ctx); err != nil {
    log.Fatal(err)
}

// 获取 TLS 配置
tlsConf, err := tlsManager.GetTLSConfig()
if err != nil {
    log.Fatal(err)
}
```

**主要特性：**
- 自动证书加载和验证
- 证书热重载
- 自签名证书生成
- 多种加密套件支持
- OCSP 装订支持

### 访问控制和权限管理

实现基于角色的访问控制（RBAC）系统，支持用户认证、权限检查和会话管理。

```go
// 创建认证管理器
authConfig := security.DefaultAuthConfig()
authConfig.JWTSecret = "your-secret-key"
authConfig.SessionTimeout = 24 * time.Hour

authManager := security.NewAuthManager(authConfig)

// 用户认证
user, err := authManager.AuthenticateUser("username", "password")
if err != nil {
    log.Printf("Authentication failed: %v", err)
}

// 权限检查
if !authManager.CheckPermission(user.ID, "resource", "action") {
    log.Println("Access denied")
}
```

**主要特性：**
- JWT 令牌认证
- 基于角色的权限控制
- 会话管理
- 密码加密存储
- 多因素认证支持

### 安全审计日志

记录和分析系统安全事件，提供完整的审计追踪能力。

```go
// 创建审计日志器
auditConfig := security.DefaultAuditConfig()
auditConfig.LogFile = "audit.log"
auditConfig.MaxFileSize = 100 * 1024 * 1024 // 100MB

auditLogger := security.NewAuditLogger(auditConfig)

// 记录安全事件
auditLogger.LogEvent(security.AuditEvent{
    Type:      security.EventTypeLogin,
    Level:     security.LevelInfo,
    UserID:    "user123",
    Resource:  "/api/login",
    Action:    "LOGIN",
    IPAddress: "192.168.1.100",
    Timestamp: time.Now(),
    Details: map[string]interface{}{
        "success": true,
        "method":  "password",
    },
})
```

**主要特性：**
- 结构化日志记录
- 事件分类和级别
- 自动日志轮转
- 实时告警
- 查询和分析接口

### DDoS 攻击防护

实现多层 DDoS 防护机制，包括速率限制、IP 黑名单和攻击检测。

```go
// 创建 DDoS 防护器
ddosConfig := security.DefaultDDoSConfig()
ddosConfig.MaxRequestsPerSecond = 100
ddosConfig.BurstSize = 200
ddosConfig.BlockDuration = 5 * time.Minute

ddosProtector := security.NewDDoSProtector(ddosConfig)

// 检查请求
if !ddosProtector.CheckRequest(clientIP, endpoint) {
    // 请求被阻止
    http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

**主要特性：**
- 令牌桶算法限流
- IP 地址追踪
- 自动黑名单管理
- 攻击模式检测
- 实时统计和告警

## 性能优化

### 零拷贝网络 IO

实现高性能的零拷贝数据传输，减少内存拷贝开销。

```go
// 创建零拷贝管理器
zeroCopyConfig := performance.DefaultZeroCopyConfig()
zeroCopyConfig.BufferSize = 64 * 1024
zeroCopyConfig.MaxConcurrent = 100

zeroCopyManager := performance.NewZeroCopyManager(zeroCopyConfig)

// 零拷贝文件传输
file, _ := os.Open("large-file.dat")
defer file.Close()

sent, err := zeroCopyManager.SendFile(conn, file, 0, -1)
if err != nil {
    log.Printf("Send file failed: %v", err)
}

log.Printf("Sent %d bytes", sent)
```

**主要特性：**
- 系统调用优化（sendfile、splice）
- 内存映射支持
- 缓冲区池管理
- 统计信息收集
- 跨平台兼容

### 内存优化和 GC 调优

智能内存管理和垃圾回收优化，提升应用性能。

```go
// 创建内存管理器
memoryConfig := performance.DefaultMemoryConfig()
memoryConfig.GCPercent = 100
memoryConfig.MaxMemoryMB = 2048
memoryConfig.PoolEnabled = true

memoryManager := performance.NewMemoryManager(memoryConfig)

// 获取优化的缓冲区
buf := memoryManager.GetBuffer(1024)
defer memoryManager.PutBuffer(buf)

// 强制 GC（在需要时）
memoryManager.ForceGC()

// 获取内存统计
stats := memoryManager.GetStats()
log.Printf("Memory usage: %.2f%%", stats.MemoryUsage*100)
```

**主要特性：**
- 内存池管理
- 动态 GC 调优
- 内存泄漏检测
- 实时监控
- 自动优化

### 智能负载均衡

实现多种负载均衡算法和自适应调整机制。

```go
// 创建智能负载均衡器
lbConfig := loadbalancer.DefaultSmartLoadBalancerConfig()
lbConfig.Strategy = loadbalancer.AdaptiveLoadBalancing
lbConfig.HealthCheckInterval = 30 * time.Second

loadBalancer := loadbalancer.NewSmartLoadBalancer(lbConfig)

// 添加后端服务器
backend1 := loadbalancer.NewBackend("server1", "192.168.1.10:8080", 100)
backend2 := loadbalancer.NewBackend("server2", "192.168.1.11:8080", 100)

loadBalancer.AddBackend(backend1)
loadBalancer.AddBackend(backend2)

// 选择后端服务器
backend, err := loadBalancer.SelectBackend(clientIP, sessionID)
if err != nil {
    log.Printf("No backend available: %v", err)
}
```

**支持的算法：**
- 轮询（Round Robin）
- 加权轮询（Weighted Round Robin）
- 最少连接（Least Connections）
- 加权最少连接（Weighted Least Connections）
- IP 哈希（IP Hash）
- 一致性哈希（Consistent Hash）
- 最短响应时间（Least Response Time）
- 自适应负载均衡（Adaptive Load Balancing）

### 连接复用和 Keep-Alive 优化

高效的连接池管理和 Keep-Alive 优化。

```go
// 创建连接池
connPoolConfig := performance.DefaultConnectionReuseConfig()
connPoolConfig.MaxIdleConns = 100
connPoolConfig.MaxIdleConnsPerHost = 10
connPoolConfig.IdleConnTimeout = 90 * time.Second

connectionPool := performance.NewConnectionPool(connPoolConfig)

// 获取连接
conn, err := connectionPool.GetConnection("example.com:80")
if err != nil {
    log.Printf("Failed to get connection: %v", err)
}
defer conn.Release()

// 使用连接
conn.Use()
// ... 执行网络操作
```

**主要特性：**
- 连接池管理
- 自动健康检查
- Keep-Alive 优化
- 连接复用统计
- 过期连接清理

## 监控和告警

### 健康检查端点

提供全面的系统健康监控和检查机制。

```go
// 创建健康检查器
healthConfig := health.DefaultHealthCheckConfig()
healthConfig.Port = 8081
healthConfig.Verbose = true

healthChecker := health.NewHealthChecker(healthConfig, "1.0.0")

// 注册自定义健康检查
healthChecker.RegisterCheck(health.NewDatabaseHealthCheck("database", "postgres://..."))
healthChecker.RegisterCheck(health.NewHTTPHealthCheck("api", "http://api.example.com/health"))
healthChecker.RegisterCheck(health.NewMemoryHealthCheck("memory", 1024))

// 启动健康检查服务
if err := healthChecker.Start(ctx); err != nil {
    log.Fatal(err)
}
```

**检查端点：**
- `/health` - 完整健康报告
- `/ready` - 就绪检查
- `/live` - 存活检查
- `/metrics` - 指标信息

### 告警规则引擎

智能告警系统，支持多种告警规则和通知渠道。

```go
// 创建告警引擎
alertConfig := alert.DefaultAlertEngineConfig()
alertConfig.EvaluationInterval = 30 * time.Second

alertEngine := alert.NewAlertEngine(alertConfig, metricProvider)

// 注册通知器
alertEngine.RegisterNotifier(alert.NewConsoleNotifier())
// alertEngine.RegisterNotifier(alert.NewEmailNotifier(emailConfig))
// alertEngine.RegisterNotifier(alert.NewSlackNotifier(slackConfig))

// 添加告警规则
rule := &alert.AlertRule{
    ID:          "high_cpu_usage",
    Name:        "High CPU Usage",
    Description: "CPU usage is above 80%",
    Level:       alert.LevelWarning,
    Duration:    5 * time.Minute,
    Conditions: []alert.AlertCondition{
        {
            Metric:      "cpu_usage_percent",
            Operator:    alert.OpGreaterThan,
            Threshold:   80.0,
            TimeWindow:  5 * time.Minute,
            Aggregation: "avg",
        },
    },
    Notifiers: []string{"console"},
    Enabled:   true,
}

if err := alertEngine.AddRule(rule); err != nil {
    log.Fatal(err)
}
```

**主要特性：**
- 灵活的规则配置
- 多种比较操作符
- 聚合函数支持
- 告警分组和抑制
- 多通道通知

### 分布式追踪支持

实现完整的分布式追踪系统，支持请求链路跟踪。

```go
// 创建追踪器提供者
tracingConfig := tracing.DefaultTracingConfig()
tracingConfig.ServiceName = "my-service"
tracingConfig.SampleRate = 0.1 // 10% 采样率

exporter := tracing.NewConsoleExporter()
// exporter := tracing.NewJaegerExporter(jaegerConfig)
// exporter := tracing.NewZipkinExporter(zipkinConfig)

tracerProvider := tracing.NewTracerProvider(tracingConfig, exporter)
tracer := tracerProvider.Tracer("my-service")

// 创建跨度
ctx, span := tracer.Start(ctx, "process_request", 
    tracing.WithSpanKind(tracing.SpanKindServer),
    tracing.WithAttributes(map[string]interface{}{
        "user.id": "12345",
        "request.size": 1024,
    }),
)
defer span.End()

// 添加事件和属性
span.AddEvent("validation_started", nil)
span.SetAttribute("validation.result", "success")

// 记录错误
if err != nil {
    span.RecordError(err)
    span.SetStatus(tracing.StatusError, err.Error())
}
```

**主要特性：**
- OpenTelemetry 兼容
- 多种导出器支持
- 采样策略配置
- 自动仪表化
- HTTP 中间件集成

## 集群和分布式支持

### 节点发现和管理

自动服务发现和集群节点管理。

```go
// 服务注册
registry := discovery.NewConsulRegistry(consulConfig)
service := &discovery.Service{
    ID:      "service-1",
    Name:    "my-service",
    Address: "192.168.1.100",
    Port:    8080,
    Tags:    []string{"api", "v1"},
    Health:  "http://192.168.1.100:8080/health",
}

if err := registry.Register(service); err != nil {
    log.Fatal(err)
}

// 服务发现
services, err := registry.Discover("my-service")
if err != nil {
    log.Fatal(err)
}
```

### 数据一致性保证

分布式环境下的数据一致性机制。

```go
// 分布式锁
lockManager := consensus.NewEtcdLockManager(etcdConfig)
lock, err := lockManager.AcquireLock("resource-key", 30*time.Second)
if err != nil {
    log.Fatal(err)
}
defer lock.Release()

// 执行需要同步的操作
// ...
```

## 使用示例

### 完整的高级服务器示例

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/netcore-go/pkg/core"
    "github.com/netcore-go/pkg/security"
    "github.com/netcore-go/pkg/performance"
    "github.com/netcore-go/pkg/health"
    "github.com/netcore-go/pkg/alert"
    "github.com/netcore-go/pkg/tracing"
)

func main() {
    ctx := context.Background()
    
    // 创建核心服务器
    server := core.NewServer(&core.Config{
        Host: "localhost",
        Port: 8080,
    })
    
    // 安全性组件
    tlsManager := security.NewTLSManager(security.DefaultTLSConfig())
    authManager := security.NewAuthManager(security.DefaultAuthConfig())
    auditLogger := security.NewAuditLogger(security.DefaultAuditConfig())
    ddosProtector := security.NewDDoSProtector(security.DefaultDDoSConfig())
    
    // 性能优化组件
    memoryManager := performance.NewMemoryManager(performance.DefaultMemoryConfig())
    zeroCopyManager := performance.NewZeroCopyManager(performance.DefaultZeroCopyConfig())
    connectionPool := performance.NewConnectionPool(performance.DefaultConnectionReuseConfig())
    
    // 监控组件
    healthChecker := health.NewHealthChecker(health.DefaultHealthCheckConfig(), "1.0.0")
    alertEngine := alert.NewAlertEngine(alert.DefaultAlertEngineConfig(), nil)
    tracerProvider := tracing.NewTracerProvider(tracing.DefaultTracingConfig(), tracing.NewConsoleExporter())
    
    // 启动所有组件
    components := []interface{ Start(context.Context) error }{
        tlsManager, authManager, auditLogger, ddosProtector,
        memoryManager, zeroCopyManager, connectionPool,
        healthChecker, alertEngine, tracerProvider,
    }
    
    for _, component := range components {
        if err := component.Start(ctx); err != nil {
            log.Fatalf("Failed to start component: %v", err)
        }
    }
    
    // 启动服务器
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Advanced NetCore server started successfully")
    
    // 优雅关闭
    // ...
}
```

### HTTP 中间件集成

```go
func setupMiddleware(server *core.Server, tracer tracing.Tracer, ddosProtector *security.DDoSProtector) {
    // 追踪中间件
    server.Use(tracing.HTTPMiddleware(tracer))
    
    // DDoS 防护中间件
    server.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !ddosProtector.CheckRequest(r.RemoteAddr, r.URL.Path) {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    })
    
    // 认证中间件
    server.Use(authMiddleware)
    
    // 审计中间件
    server.Use(auditMiddleware)
}
```

## 配置参考

### 环境变量

```bash
# 服务配置
NETCORE_HOST=localhost
NETCORE_PORT=8080
NETCORE_ENV=production

# 安全配置
NETCORE_TLS_CERT_FILE=/path/to/cert.pem
NETCORE_TLS_KEY_FILE=/path/to/key.pem
NETCORE_JWT_SECRET=your-secret-key

# 性能配置
NETCORE_MAX_CONNECTIONS=1000
NETCORE_BUFFER_SIZE=65536
NETCORE_GC_PERCENT=100

# 监控配置
NETCORE_HEALTH_PORT=8081
NETCORE_METRICS_ENABLED=true
NETCORE_TRACING_SAMPLE_RATE=0.1

# 告警配置
NETCORE_ALERT_ENABLED=true
NETCORE_ALERT_INTERVAL=30s
```

### 配置文件示例

```yaml
# netcore.yaml
server:
  host: localhost
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  max_connections: 1000

security:
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
    auto_reload: true
  
  auth:
    jwt_secret: "your-secret-key"
    session_timeout: 24h
    bcrypt_cost: 12
  
  ddos:
    enabled: true
    max_requests_per_second: 100
    burst_size: 200
    block_duration: 5m

performance:
  memory:
    enabled: true
    gc_percent: 100
    max_memory_mb: 2048
    pool_enabled: true
  
  zero_copy:
    enabled: true
    buffer_size: 65536
    max_concurrent: 100
  
  connection_pool:
    enabled: true
    max_idle_conns: 100
    max_idle_conns_per_host: 10
    idle_conn_timeout: 90s

monitoring:
  health:
    enabled: true
    port: 8081
    check_interval: 30s
    verbose: true
  
  tracing:
    enabled: true
    service_name: "netcore-service"
    sample_rate: 0.1
    exporter: "console"
  
  alerts:
    enabled: true
    evaluation_interval: 30s
    default_notifiers: ["console"]
```

## 性能基准

### 零拷贝 IO 性能

```
Benchmark_RegularCopy-8    1000000    1200 ns/op    1024 B/op    2 allocs/op
Benchmark_ZeroCopy-8       5000000     240 ns/op       0 B/op    0 allocs/op
```

### 内存池性能

```
Benchmark_DirectAlloc-8    2000000     800 ns/op    1024 B/op    1 allocs/op
Benchmark_PoolAlloc-8     10000000     160 ns/op       0 B/op    0 allocs/op
```

### 负载均衡性能

```
Benchmark_RoundRobin-8     5000000     300 ns/op       0 B/op    0 allocs/op
Benchmark_Adaptive-8       3000000     500 ns/op      48 B/op    1 allocs/op
```

## 故障排除

### 常见问题

1. **TLS 证书加载失败**
   - 检查证书文件路径和权限
   - 验证证书格式和有效期
   - 查看 TLS 管理器日志

2. **内存使用率过高**
   - 调整 GC 参数
   - 启用内存池
   - 检查内存泄漏

3. **连接池耗尽**
   - 增加最大连接数
   - 减少空闲超时时间
   - 检查连接释放逻辑

4. **告警规则不触发**
   - 验证指标数据源
   - 检查规则条件设置
   - 确认评估间隔配置

### 调试技巧

1. **启用详细日志**
   ```go
   logger.SetLevel("debug")
   ```

2. **查看组件统计**
   ```go
   stats := component.GetStats()
   log.Printf("Component stats: %+v", stats)
   ```

3. **健康检查诊断**
   ```bash
   curl http://localhost:8081/health
   ```

4. **追踪信息查看**
   ```bash
   curl -H "X-Trace-Debug: true" http://localhost:8080/api/endpoint
   ```

## 最佳实践

1. **安全配置**
   - 定期更新 TLS 证书
   - 使用强密码策略
   - 启用审计日志
   - 配置适当的 DDoS 防护阈值

2. **性能优化**
   - 根据负载调整连接池大小
   - 监控内存使用情况
   - 使用零拷贝 IO 处理大文件
   - 选择合适的负载均衡算法

3. **监控告警**
   - 设置关键指标告警
   - 配置多级告警通知
   - 定期检查健康状态
   - 启用分布式追踪

4. **运维部署**
   - 使用配置文件管理
   - 实现优雅关闭
   - 配置日志轮转
   - 监控资源使用

## 更多资源

- [API 参考文档](./API_REFERENCE.md)
- [架构设计文档](./ARCHITECTURE.md)
- [性能调优指南](./PERFORMANCE_TUNING.md)
- [安全最佳实践](./SECURITY_BEST_PRACTICES.md)
- [部署指南](./DEPLOYMENT_GUIDE.md)