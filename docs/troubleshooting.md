# NetCore-Go 故障排除指南

本指南提供了使用 NetCore-Go 框架时可能遇到的常见问题及其解决方案。

## 目录

- [安装和设置问题](#安装和设置问题)
- [服务发现问题](#服务发现问题)
- [网络和连接问题](#网络和连接问题)
- [性能问题](#性能问题)
- [内存和资源问题](#内存和资源问题)
- [日志和调试](#日志和调试)
- [配置问题](#配置问题)
- [部署问题](#部署问题)
- [常见错误代码](#常见错误代码)
- [诊断工具](#诊断工具)

## 安装和设置问题

### 问题：无法安装 NetCore-Go

**症状：**
```bash
go get github.com/netcore-go
# go: module github.com/netcore-go: not found
```

**解决方案：**
1. 确保 Go 版本 >= 1.21
2. 检查网络连接和代理设置
3. 使用正确的模块路径
4. 清理模块缓存：`go clean -modcache`

### 问题：依赖版本冲突

**症状：**
```
go: inconsistent vendoring
```

**解决方案：**
```bash
# 更新依赖
go mod tidy
go mod download

# 如果问题持续，重置模块
rm go.mod go.sum
go mod init your-module-name
go get github.com/netcore-go
```

### 问题：构建失败

**症状：**
```
build constraints exclude all Go files
```

**解决方案：**
1. 检查 Go 版本兼容性
2. 确保正确的构建标签
3. 验证文件结构和包声明

## 服务发现问题

### 问题：服务注册失败

**症状：**
```
ERROR: Failed to register service: connection refused
```

**诊断步骤：**
```go
// 检查服务发现配置
config := &discovery.Config{
    Type: "consul",
    Address: "localhost:8500",
    Timeout: 10 * time.Second,
}

// 测试连接
if err := discovery.TestConnection(config); err != nil {
    log.Printf("Connection test failed: %v", err)
}
```

**解决方案：**
1. 确保服务发现后端（Consul/etcd）正在运行
2. 检查网络连接和防火墙设置
3. 验证认证凭据
4. 检查服务发现配置

### 问题：服务发现超时

**症状：**
```
context deadline exceeded
```

**解决方案：**
```go
// 增加超时时间
config.Timeout = 30 * time.Second

// 启用重试机制
config.RetryAttempts = 3
config.RetryDelay = 5 * time.Second
```

### 问题：服务健康检查失败

**症状：**
服务显示为不健康状态

**解决方案：**
```go
// 配置健康检查
healthCheck := &discovery.HealthCheck{
    HTTP:     "http://localhost:8080/health",
    Interval: "10s",
    Timeout:  "3s",
    DeregisterCriticalServiceAfter: "30s",
}

// 确保健康检查端点正常工作
func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "timestamp": time.Now().Format(time.RFC3339),
    })
}
```

## 网络和连接问题

### 问题：连接被拒绝

**症状：**
```
connection refused
```

**诊断步骤：**
```bash
# 检查端口是否被占用
netstat -tulpn | grep :8080

# 测试网络连接
telnet localhost 8080

# 检查防火墙规则
sudo iptables -L
```

**解决方案：**
1. 确保服务正在监听正确的端口
2. 检查防火墙和安全组设置
3. 验证网络配置
4. 使用正确的绑定地址（0.0.0.0 vs localhost）

### 问题：TLS/SSL 连接问题

**症状：**
```
tls: handshake failure
```

**解决方案：**
```go
// 配置 TLS
tlsConfig := &tls.Config{
    InsecureSkipVerify: false, // 生产环境设为 false
    MinVersion:         tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
}

// 验证证书
if err := verifyCertificate(certFile); err != nil {
    log.Printf("Certificate verification failed: %v", err)
}
```

### 问题：负载均衡问题

**症状：**
请求分布不均匀

**解决方案：**
```go
// 配置负载均衡策略
config := &loadbalancer.Config{
    Strategy: "round_robin", // 或 "least_connections", "weighted"
    HealthCheck: true,
    HealthCheckInterval: 30 * time.Second,
}

// 监控负载均衡状态
stats := lb.GetStats()
log.Printf("Load balancer stats: %+v", stats)
```

## 性能问题

### 问题：高延迟

**诊断步骤：**
```go
// 添加性能监控
func performanceMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        duration := time.Since(start)
        
        if duration > 100*time.Millisecond {
            log.Printf("Slow request: %s %s took %v", 
                r.Method, r.URL.Path, duration)
        }
    })
}
```

**解决方案：**
1. 启用连接池
2. 优化数据库查询
3. 使用缓存
4. 调整超时设置
5. 启用 HTTP/2

### 问题：低吞吐量

**诊断步骤：**
```bash
# 使用 pprof 分析性能
go tool pprof http://localhost:6060/debug/pprof/profile

# 检查 CPU 使用情况
top -p $(pgrep your-service)

# 监控网络 I/O
iotop
```

**解决方案：**
```go
// 调整服务器配置
server := &http.Server{
    Addr:           ":8080",
    ReadTimeout:    10 * time.Second,
    WriteTimeout:   10 * time.Second,
    IdleTimeout:    60 * time.Second,
    MaxHeaderBytes: 1 << 20,
}

// 启用 goroutine 池
pool := &sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}
```

## 内存和资源问题

### 问题：内存泄漏

**诊断步骤：**
```bash
# 监控内存使用
go tool pprof http://localhost:6060/debug/pprof/heap

# 检查 goroutine 泄漏
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

**解决方案：**
```go
// 正确关闭资源
func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()
    
    // 使用 context 进行超时控制
    select {
    case result := <-processRequest(ctx):
        json.NewEncoder(w).Encode(result)
    case <-ctx.Done():
        http.Error(w, "Request timeout", http.StatusRequestTimeout)
    }
}

// 定期清理缓存
func cleanupCache() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        cache.Cleanup()
    }
}
```

### 问题：文件描述符耗尽

**症状：**
```
too many open files
```

**解决方案：**
```bash
# 检查当前限制
ulimit -n

# 增加文件描述符限制
ulimit -n 65536

# 永久设置（在 /etc/security/limits.conf）
* soft nofile 65536
* hard nofile 65536
```

```go
// 正确关闭连接
func makeRequest(url string) error {
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close() // 重要：关闭响应体
    
    // 处理响应
    return nil
}
```

## 日志和调试

### 启用调试日志

```go
// 配置详细日志
logger := core.NewLogger(&core.LoggerConfig{
    Level:  core.LogLevelDebug,
    Format: core.LogFormatJSON,
    Output: os.Stdout,
})

// 添加请求 ID 跟踪
func requestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        
        logger.WithField("request_id", requestID).
               WithField("method", r.Method).
               WithField("path", r.URL.Path).
               Info("Request started")
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 结构化日志

```go
// 使用结构化日志记录错误
func handleError(err error, ctx context.Context) {
    logger.WithField("error", err.Error()).
           WithField("request_id", ctx.Value("request_id")).
           WithField("stack_trace", string(debug.Stack())).
           Error("Request failed")
}
```

## 配置问题

### 问题：配置文件未找到

**症状：**
```
config file not found: config.yaml
```

**解决方案：**
```go
// 配置文件搜索路径
configPaths := []string{
    "./config.yaml",
    "./configs/config.yaml",
    "/etc/myapp/config.yaml",
    os.Getenv("CONFIG_PATH"),
}

for _, path := range configPaths {
    if path != "" && fileExists(path) {
        return loadConfig(path)
    }
}
```

### 问题：环境变量未设置

**解决方案：**
```go
// 提供默认值
func getEnvWithDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

// 验证必需的环境变量
requiredEnvs := []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"}
for _, env := range requiredEnvs {
    if os.Getenv(env) == "" {
        log.Fatalf("Required environment variable %s is not set", env)
    }
}
```

## 部署问题

### 问题：Docker 容器启动失败

**诊断步骤：**
```bash
# 检查容器日志
docker logs <container-id>

# 检查容器状态
docker inspect <container-id>

# 进入容器调试
docker exec -it <container-id> /bin/sh
```

**解决方案：**
```dockerfile
# 优化 Dockerfile
FROM golang:1.21-alpine AS builder

# 添加健康检查
HEALTHCHEK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# 使用非 root 用户
RUN adduser -D -s /bin/sh appuser
USER appuser
```

### 问题：Kubernetes 部署问题

**诊断步骤：**
```bash
# 检查 Pod 状态
kubectl get pods
kubectl describe pod <pod-name>
kubectl logs <pod-name>

# 检查服务
kubectl get svc
kubectl describe svc <service-name>
```

**解决方案：**
```yaml
# 添加就绪和存活探针
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:latest
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
```

## 常见错误代码

### NetCore-Go 错误代码

| 错误代码 | 描述 | 解决方案 |
|---------|------|----------|
| `NETCORE_001` | 服务发现连接失败 | 检查服务发现后端状态 |
| `NETCORE_002` | 配置验证失败 | 验证配置文件格式和内容 |
| `NETCORE_003` | 负载均衡器初始化失败 | 检查后端服务可用性 |
| `NETCORE_004` | TLS 证书验证失败 | 更新或重新生成证书 |
| `NETCORE_005` | 健康检查超时 | 调整健康检查配置 |

### HTTP 状态码处理

```go
// 统一错误处理
func handleHTTPError(w http.ResponseWriter, err error) {
    var statusCode int
    var message string
    
    switch e := err.(type) {
    case *core.NetCoreError:
        statusCode = e.HTTPStatusCode()
        message = e.Error()
    case *ValidationError:
        statusCode = http.StatusBadRequest
        message = "Validation failed: " + e.Error()
    default:
        statusCode = http.StatusInternalServerError
        message = "Internal server error"
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": message,
        "code":  statusCode,
        "timestamp": time.Now().Unix(),
    })
}
```

## 诊断工具

### 内置诊断端点

```go
// 添加诊断端点
func setupDiagnosticEndpoints(mux *http.ServeMux) {
    // 健康检查
    mux.HandleFunc("/health", healthCheckHandler)
    
    // 就绪检查
    mux.HandleFunc("/ready", readinessCheckHandler)
    
    // 指标
    mux.HandleFunc("/metrics", metricsHandler)
    
    // 配置信息
    mux.HandleFunc("/debug/config", configHandler)
    
    // 服务发现状态
    mux.HandleFunc("/debug/discovery", discoveryStatusHandler)
}
```

### 性能分析

```go
import _ "net/http/pprof"

// 启用 pprof
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

### 监控和告警

```go
// 集成 Prometheus 指标
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request duration in seconds",
        },
        []string{"method", "path", "status"},
    )
    
    requestCount = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )
)

func init() {
    prometheus.MustRegister(requestDuration, requestCount)
}
```

## 最佳实践

### 错误处理

1. **使用结构化错误**：提供详细的错误信息和上下文
2. **实现重试机制**：对于临时性错误进行重试
3. **记录错误日志**：包含足够的调试信息
4. **优雅降级**：在部分功能不可用时提供备选方案

### 监控和观察性

1. **添加指标收集**：监控关键性能指标
2. **实现分布式跟踪**：跟踪请求在系统中的流转
3. **设置告警**：对异常情况及时告警
4. **定期健康检查**：确保服务正常运行

### 性能优化

1. **使用连接池**：复用数据库和 HTTP 连接
2. **启用缓存**：缓存频繁访问的数据
3. **优化序列化**：使用高效的序列化格式
4. **调整超时设置**：根据实际情况设置合理的超时时间

## 获取帮助

如果本指南未能解决您的问题，请：

1. 查看 [GitHub Issues](https://github.com/netcore-go/netcore-go/issues)
2. 搜索 [文档](https://netcore-go.dev/docs)
3. 加入 [社区讨论](https://github.com/netcore-go/netcore-go/discussions)
4. 提交新的 Issue 并提供：
   - 详细的错误描述
   - 复现步骤
   - 环境信息（Go 版本、操作系统等）
   - 相关日志和配置文件

---

**注意**：本指南会持续更新，请定期查看最新版本。