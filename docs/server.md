# TCP/UDP 服务器

NetCore-Go 提供了高性能的 TCP 和 UDP 服务器实现，支持连接池、中间件、协程池等企业级功能。

## 快速开始

### TCP 服务器

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/netcore-go/pkg/server"
)

func main() {
    // 创建服务器配置
    config := &server.Config{
        Network:        "tcp",
        Address:        ":8080",
        MaxConnections: 1000,
        BufferSize:     4096,
        ReadTimeout:    "30s",
        WriteTimeout:   "30s",
    }
    
    // 创建服务器
    srv := server.NewTCPServer(config)
    
    // 设置消息处理器
    srv.SetHandler(&EchoHandler{})
    
    // 启动服务器
    log.Printf("TCP 服务器启动在 %s", config.Address)
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}

// 消息处理器
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

### UDP 服务器

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/netcore-go/pkg/server"
)

func main() {
    config := &server.Config{
        Network:    "udp",
        Address:    ":8080",
        BufferSize: 1024,
    }
    
    srv := server.NewUDPServer(config)
    srv.SetHandler(&UDPHandler{})
    
    log.Printf("UDP 服务器启动在 %s", config.Address)
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}

type UDPHandler struct{}

func (h *UDPHandler) OnMessage(addr net.Addr, data []byte) {
    fmt.Printf("收到来自 %s 的消息: %s\n", addr, string(data))
    // 处理 UDP 消息
}
```

## 配置选项

### 服务器配置

```go
type Config struct {
    // 网络类型: "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"
    Network string `json:"network" yaml:"network"`
    
    // 监听地址: ":8080", "localhost:8080", "0.0.0.0:8080"
    Address string `json:"address" yaml:"address"`
    
    // 最大连接数 (仅 TCP)
    MaxConnections int `json:"max_connections" yaml:"max_connections"`
    
    // 缓冲区大小
    BufferSize int `json:"buffer_size" yaml:"buffer_size"`
    
    // 读取超时
    ReadTimeout string `json:"read_timeout" yaml:"read_timeout"`
    
    // 写入超时
    WriteTimeout string `json:"write_timeout" yaml:"write_timeout"`
    
    // 保持连接 (仅 TCP)
    KeepAlive bool `json:"keep_alive" yaml:"keep_alive"`
    
    // 保持连接间隔
    KeepAlivePeriod string `json:"keep_alive_period" yaml:"keep_alive_period"`
    
    // 启用 TLS
    TLS *TLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
    
    // 协程池配置
    WorkerPool *WorkerPoolConfig `json:"worker_pool,omitempty" yaml:"worker_pool,omitempty"`
}

type TLSConfig struct {
    CertFile string `json:"cert_file" yaml:"cert_file"`
    KeyFile  string `json:"key_file" yaml:"key_file"`
}

type WorkerPoolConfig struct {
    Size      int `json:"size" yaml:"size"`
    QueueSize int `json:"queue_size" yaml:"queue_size"`
}
```

### 配置示例

```yaml
# config.yaml
server:
  network: "tcp"
  address: ":8080"
  max_connections: 10000
  buffer_size: 4096
  read_timeout: "30s"
  write_timeout: "30s"
  keep_alive: true
  keep_alive_period: "60s"
  
  # TLS 配置
  tls:
    cert_file: "server.crt"
    key_file: "server.key"
  
  # 协程池配置
  worker_pool:
    size: 100
    queue_size: 1000
```

## 消息处理器

### TCP 处理器接口

```go
type TCPHandler interface {
    OnConnect(conn Connection)
    OnMessage(conn Connection, data []byte)
    OnDisconnect(conn Connection)
}
```

### UDP 处理器接口

```go
type UDPHandler interface {
    OnMessage(addr net.Addr, data []byte)
}
```

### 高级处理器示例

```go
type AdvancedHandler struct {
    logger  *log.Logger
    metrics *metrics.Registry
}

func (h *AdvancedHandler) OnConnect(conn server.Connection) {
    h.logger.Printf("新连接: %s", conn.RemoteAddr())
    h.metrics.Counter("connections_total").Inc()
    
    // 设置连接属性
    conn.SetAttribute("connect_time", time.Now())
    conn.SetAttribute("user_id", "anonymous")
}

func (h *AdvancedHandler) OnMessage(conn server.Connection, data []byte) {
    h.metrics.Counter("messages_total").Inc()
    h.metrics.Histogram("message_size").Observe(float64(len(data)))
    
    // 解析消息
    msg, err := parseMessage(data)
    if err != nil {
        h.logger.Printf("解析消息失败: %v", err)
        conn.Write([]byte("ERROR: Invalid message format"))
        return
    }
    
    // 处理不同类型的消息
    switch msg.Type {
    case "ping":
        conn.Write([]byte("pong"))
    case "echo":
        conn.Write(msg.Data)
    case "auth":
        h.handleAuth(conn, msg)
    default:
        conn.Write([]byte("ERROR: Unknown message type"))
    }
}

func (h *AdvancedHandler) OnDisconnect(conn server.Connection) {
    connectTime := conn.GetAttribute("connect_time").(time.Time)
    duration := time.Since(connectTime)
    
    h.logger.Printf("连接断开: %s, 持续时间: %v", conn.RemoteAddr(), duration)
    h.metrics.Counter("disconnections_total").Inc()
    h.metrics.Histogram("connection_duration").Observe(duration.Seconds())
}
```

## 中间件

### 中间件接口

```go
type Middleware interface {
    Process(conn Connection, data []byte, next MiddlewareFunc) error
}

type MiddlewareFunc func(conn Connection, data []byte) error
```

### 内置中间件

#### 日志中间件

```go
type LoggingMiddleware struct {
    logger *log.Logger
}

func (m *LoggingMiddleware) Process(conn server.Connection, data []byte, next server.MiddlewareFunc) error {
    start := time.Now()
    
    m.logger.Printf("[%s] 收到消息: %d 字节", conn.RemoteAddr(), len(data))
    
    err := next(conn, data)
    
    duration := time.Since(start)
    m.logger.Printf("[%s] 处理完成: %v, 耗时: %v", conn.RemoteAddr(), err, duration)
    
    return err
}
```

#### 限流中间件

```go
type RateLimitMiddleware struct {
    limiter *rate.Limiter
}

func (m *RateLimitMiddleware) Process(conn server.Connection, data []byte, next server.MiddlewareFunc) error {
    if !m.limiter.Allow() {
        return errors.New("rate limit exceeded")
    }
    
    return next(conn, data)
}
```

#### 认证中间件

```go
type AuthMiddleware struct {
    tokenValidator TokenValidator
}

func (m *AuthMiddleware) Process(conn server.Connection, data []byte, next server.MiddlewareFunc) error {
    // 检查连接是否已认证
    if !conn.GetAttribute("authenticated").(bool) {
        // 尝试从消息中提取认证信息
        token := extractToken(data)
        if token == "" {
            return errors.New("authentication required")
        }
        
        if !m.tokenValidator.Validate(token) {
            return errors.New("invalid token")
        }
        
        conn.SetAttribute("authenticated", true)
        conn.SetAttribute("user_id", getUserID(token))
    }
    
    return next(conn, data)
}
```

### 使用中间件

```go
func main() {
    srv := server.NewTCPServer(config)
    
    // 添加中间件
    srv.Use(&LoggingMiddleware{logger: log.New(os.Stdout, "[SERVER] ", log.LstdFlags)})
    srv.Use(&RateLimitMiddleware{limiter: rate.NewLimiter(100, 10)}) // 100 QPS, 突发 10
    srv.Use(&AuthMiddleware{tokenValidator: &JWTValidator{}})
    
    srv.SetHandler(&BusinessHandler{})
    
    srv.Start()
}
```

## 连接管理

### 连接接口

```go
type Connection interface {
    // 基本操作
    Read([]byte) (int, error)
    Write([]byte) (int, error)
    Close() error
    
    // 地址信息
    LocalAddr() net.Addr
    RemoteAddr() net.Addr
    
    // 属性管理
    SetAttribute(key string, value interface{})
    GetAttribute(key string) interface{}
    RemoveAttribute(key string)
    
    // 状态检查
    IsActive() bool
    
    // 超时设置
    SetReadDeadline(time.Time) error
    SetWriteDeadline(time.Time) error
    SetDeadline(time.Time) error
}
```

### 连接池

```go
// 连接池配置
type PoolConfig struct {
    MaxIdle     int           // 最大空闲连接数
    MaxActive   int           // 最大活跃连接数
    IdleTimeout time.Duration // 空闲超时时间
}

// 使用连接池
func main() {
    poolConfig := &server.PoolConfig{
        MaxIdle:     10,
        MaxActive:   100,
        IdleTimeout: 5 * time.Minute,
    }
    
    srv := server.NewTCPServer(config)
    srv.SetConnectionPool(poolConfig)
    
    srv.Start()
}
```

## TLS 支持

### 基本 TLS 配置

```go
config := &server.Config{
    Network: "tcp",
    Address: ":8443",
    TLS: &server.TLSConfig{
        CertFile: "server.crt",
        KeyFile:  "server.key",
    },
}

srv := server.NewTCPServer(config)
```

### 高级 TLS 配置

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    caCertPool,
    MinVersion:   tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
}

srv := server.NewTCPServerWithTLS(config, tlsConfig)
```

## 性能优化

### 协程池

```go
// 配置协程池
workerPoolConfig := &server.WorkerPoolConfig{
    Size:      100,   // 工作协程数量
    QueueSize: 1000,  // 任务队列大小
}

config.WorkerPool = workerPoolConfig
```

### 内存池

```go
// 使用内存池减少 GC 压力
type BufferPool struct {
    pool sync.Pool
}

func (p *BufferPool) Get() []byte {
    buf := p.pool.Get()
    if buf == nil {
        return make([]byte, 4096)
    }
    return buf.([]byte)
}

func (p *BufferPool) Put(buf []byte) {
    p.pool.Put(buf)
}
```

### 零拷贝优化

```go
// 使用 sendfile 系统调用进行零拷贝传输
func (conn *TCPConnection) SendFile(file *os.File) error {
    if tcpConn, ok := conn.rawConn.(*net.TCPConn); ok {
        return sendfile(tcpConn, file)
    }
    return errors.New("not a TCP connection")
}
```

## 监控和指标

### 内置指标

```go
// 服务器自动收集的指标
type ServerMetrics struct {
    ConnectionsTotal     prometheus.Counter   // 总连接数
    ConnectionsActive    prometheus.Gauge     // 活跃连接数
    MessagesTotal        prometheus.Counter   // 总消息数
    MessageSize          prometheus.Histogram // 消息大小分布
    ProcessingDuration   prometheus.Histogram // 处理时间分布
    ErrorsTotal          prometheus.Counter   // 错误总数
}
```

### 启用监控

```go
func main() {
    srv := server.NewTCPServer(config)
    
    // 启用 Prometheus 指标
    srv.EnableMetrics(true)
    
    // 启动指标服务器
    go func() {
        http.Handle("/metrics", promhttp.Handler())
        log.Fatal(http.ListenAndServe(":9090", nil))
    }()
    
    srv.Start()
}
```

## 最佳实践

### 1. 错误处理

```go
func (h *Handler) OnMessage(conn server.Connection, data []byte) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("处理消息时发生 panic: %v", r)
            conn.Close()
        }
    }()
    
    // 处理消息逻辑
}
```

### 2. 资源清理

```go
func (h *Handler) OnDisconnect(conn server.Connection) {
    // 清理连接相关资源
    if userID := conn.GetAttribute("user_id"); userID != nil {
        h.userManager.RemoveUser(userID.(string))
    }
    
    // 关闭相关通道
    if ch := conn.GetAttribute("message_channel"); ch != nil {
        close(ch.(chan []byte))
    }
}
```

### 3. 优雅关闭

```go
func main() {
    srv := server.NewTCPServer(config)
    
    // 处理关闭信号
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-c
        log.Println("正在关闭服务器...")
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := srv.Shutdown(ctx); err != nil {
            log.Printf("服务器关闭失败: %v", err)
        }
    }()
    
    srv.Start()
}
```

### 4. 连接限制

```go
// 基于 IP 的连接限制
type IPLimitMiddleware struct {
    connections map[string]int
    maxPerIP    int
    mutex       sync.RWMutex
}

func (m *IPLimitMiddleware) OnConnect(conn server.Connection) error {
    ip := getIP(conn.RemoteAddr())
    
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    if m.connections[ip] >= m.maxPerIP {
        return errors.New("too many connections from this IP")
    }
    
    m.connections[ip]++
    return nil
}
```

## 故障排除

### 常见问题

1. **连接被拒绝**
   - 检查端口是否被占用
   - 确认防火墙设置
   - 验证监听地址配置

2. **内存泄漏**
   - 确保连接正确关闭
   - 检查协程是否正确退出
   - 使用内存分析工具

3. **性能问题**
   - 调整缓冲区大小
   - 优化消息处理逻辑
   - 使用协程池

### 调试工具

```go
// 启用调试模式
srv.SetDebug(true)

// 添加调试中间件
srv.Use(&DebugMiddleware{})

// 启用性能分析
import _ "net/http/pprof"
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

## 示例项目

完整的示例项目可以在 `examples/` 目录中找到：

- `examples/tcp-server/` - 基本 TCP 服务器
- `examples/udp-server/` - 基本 UDP 服务器
- `examples/tls-server/` - TLS 安全服务器
- `examples/chat-server/` - 聊天服务器
- `examples/game-server/` - 游戏服务器

## 参考资料

- [Go 网络编程](https://golang.org/pkg/net/)
- [TCP/IP 协议详解](https://en.wikipedia.org/wiki/Internet_protocol_suite)
- [TLS 最佳实践](https://wiki.mozilla.org/Security/Server_Side_TLS)
- [性能优化指南](./performance.md)