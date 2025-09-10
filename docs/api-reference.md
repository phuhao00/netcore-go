# API 参考文档

本文档提供了 NetCore-Go 所有公共 API 的详细参考信息。

## 目录

- [核心包](#核心包)
  - [server](#server-包)
  - [websocket](#websocket-包)
  - [http](#http-包)
  - [rpc](#rpc-包)
  - [grpc](#grpc-包)
  - [kcp](#kcp-包)
- [基础设施包](#基础设施包)
  - [config](#config-包)
  - [logger](#logger-包)
  - [metrics](#metrics-包)
  - [discovery](#discovery-包)
  - [loadbalancer](#loadbalancer-包)
- [中间件包](#中间件包)
  - [middleware](#middleware-包)
- [工具包](#工具包)
  - [codec](#codec-包)
  - [pool](#pool-包)
  - [utils](#utils-包)

## 核心包

### server 包

提供高性能的 TCP/UDP 服务器实现。

#### 类型

##### Config

```go
type Config struct {
    Network         string        `json:"network" yaml:"network"`
    Address         string        `json:"address" yaml:"address"`
    MaxConnections  int           `json:"max_connections" yaml:"max_connections"`
    BufferSize      int           `json:"buffer_size" yaml:"buffer_size"`
    ReadTimeout     time.Duration `json:"read_timeout" yaml:"read_timeout"`
    WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout"`
    KeepAlive       bool          `json:"keep_alive" yaml:"keep_alive"`
    KeepAlivePeriod time.Duration `json:"keep_alive_period" yaml:"keep_alive_period"`
    TLS             *TLSConfig    `json:"tls,omitempty" yaml:"tls,omitempty"`
    WorkerPool      *WorkerPoolConfig `json:"worker_pool,omitempty" yaml:"worker_pool,omitempty"`
}
```

服务器配置结构体。

**字段说明：**
- `Network`: 网络类型（"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"）
- `Address`: 监听地址（如 ":8080", "localhost:8080"）
- `MaxConnections`: 最大连接数（仅 TCP）
- `BufferSize`: 缓冲区大小
- `ReadTimeout`: 读取超时时间
- `WriteTimeout`: 写入超时时间
- `KeepAlive`: 是否启用保持连接（仅 TCP）
- `KeepAlivePeriod`: 保持连接间隔时间
- `TLS`: TLS 配置
- `WorkerPool`: 协程池配置

##### Server

```go
type Server interface {
    Start() error
    Stop() error
    Shutdown(ctx context.Context) error
    SetHandler(handler Handler)
    Use(middleware Middleware)
    GetStats() *Stats
}
```

服务器接口定义。

**方法说明：**
- `Start()`: 启动服务器
- `Stop()`: 停止服务器
- `Shutdown(ctx)`: 优雅关闭服务器
- `SetHandler(handler)`: 设置消息处理器
- `Use(middleware)`: 添加中间件
- `GetStats()`: 获取服务器统计信息

##### Connection

```go
type Connection interface {
    Read([]byte) (int, error)
    Write([]byte) (int, error)
    Close() error
    LocalAddr() net.Addr
    RemoteAddr() net.Addr
    SetAttribute(key string, value interface{})
    GetAttribute(key string) interface{}
    RemoveAttribute(key string)
    IsActive() bool
    SetReadDeadline(time.Time) error
    SetWriteDeadline(time.Time) error
    SetDeadline(time.Time) error
}
```

连接接口定义。

##### Handler

```go
type Handler interface {
    OnConnect(conn Connection)
    OnMessage(conn Connection, data []byte)
    OnDisconnect(conn Connection)
}
```

TCP 消息处理器接口。

##### UDPHandler

```go
type UDPHandler interface {
    OnMessage(addr net.Addr, data []byte)
}
```

UDP 消息处理器接口。

#### 函数

##### NewTCPServer

```go
func NewTCPServer(config *Config) Server
```

创建新的 TCP 服务器实例。

**参数：**
- `config`: 服务器配置

**返回值：**
- `Server`: 服务器实例

**示例：**
```go
config := &server.Config{
    Network: "tcp",
    Address: ":8080",
    MaxConnections: 1000,
}
srv := server.NewTCPServer(config)
```

##### NewUDPServer

```go
func NewUDPServer(config *Config) Server
```

创建新的 UDP 服务器实例。

### websocket 包

提供 WebSocket 服务器实现。

#### 类型

##### Config

```go
type Config struct {
    CheckOrigin       func(*http.Request) bool
    Subprotocols      []string
    ReadBufferSize    int
    WriteBufferSize   int
    HandshakeTimeout  time.Duration
    EnableCompression bool
    PingPeriod        time.Duration
    PongWait          time.Duration
    WriteWait         time.Duration
    MaxMessageSize    int64
    MaxConnections    int
}
```

WebSocket 服务器配置。

##### Server

```go
type Server interface {
    HandleWebSocket(w http.ResponseWriter, r *http.Request)
    SetHandler(handler Handler)
    Use(middleware Middleware)
    EnableMetrics(registry prometheus.Registerer)
    GetHub() *Hub
}
```

WebSocket 服务器接口。

##### Connection

```go
type Connection interface {
    WriteMessage(messageType int, data []byte) error
    ReadMessage() (messageType int, p []byte, err error)
    Close() error
    LocalAddr() net.Addr
    RemoteAddr() net.Addr
    SetAttribute(key string, value interface{})
    GetAttribute(key string) interface{}
    RemoveAttribute(key string)
}
```

WebSocket 连接接口。

##### Handler

```go
type Handler interface {
    OnConnect(conn Connection)
    OnMessage(conn Connection, messageType int, data []byte)
    OnDisconnect(conn Connection)
}
```

WebSocket 消息处理器接口。

##### Hub

```go
type Hub struct {
    // 导出的方法
}

func (h *Hub) Register(conn Connection)
func (h *Hub) Unregister(conn Connection)
func (h *Hub) JoinRoom(conn Connection, room string)
func (h *Hub) LeaveRoom(conn Connection, room string)
func (h *Hub) BroadcastToRoom(room string, data interface{})
func (h *Hub) BroadcastToAll(data interface{})
func (h *Hub) GetRoomCount() int
func (h *Hub) GetConnectionCount() int
```

WebSocket 连接中心，用于管理连接和房间。

#### 函数

##### NewServer

```go
func NewServer(config *Config) *Server
```

创建新的 WebSocket 服务器。

##### NewHub

```go
func NewHub() *Hub
```

创建新的连接中心。

### rpc 包

提供自定义 RPC 协议实现。

#### 类型

##### ServerConfig

```go
type ServerConfig struct {
    Network         string
    Address         string
    Codec           string
    MaxConnections  int
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    ReadBufferSize  int
    WriteBufferSize int
    WorkerPoolSize  int
    TLS             *TLSConfig
    Interceptors    []ServerInterceptor
}
```

RPC 服务器配置。

##### ClientConfig

```go
type ClientConfig struct {
    Network       string
    Address       string
    Codec         string
    Timeout       time.Duration
    DialTimeout   time.Duration
    ReadTimeout   time.Duration
    WriteTimeout  time.Duration
    MaxRetries    int
    RetryInterval time.Duration
    PoolSize      int
    IdleTimeout   time.Duration
    TLS           *ClientTLSConfig
    Interceptors  []ClientInterceptor
}
```

RPC 客户端配置。

##### Server

```go
type Server interface {
    RegisterService(name string, service interface{}) error
    Start() error
    Stop() error
    Shutdown(ctx context.Context) error
    Use(interceptor ServerInterceptor)
}
```

RPC 服务器接口。

##### Client

```go
type Client interface {
    Call(ctx context.Context, method string, req, resp interface{}) error
    Close() error
    Use(interceptor ClientInterceptor)
}
```

RPC 客户端接口。

#### 函数

##### NewServer

```go
func NewServer(config *ServerConfig) Server
```

创建新的 RPC 服务器。

##### NewClient

```go
func NewClient(config *ClientConfig) (Client, error)
```

创建新的 RPC 客户端。

### grpc 包

提供 gRPC 协议集成。

#### 类型

##### ServerConfig

```go
type ServerConfig struct {
    Network         string
    Address         string
    MaxRecvMsgSize  int
    MaxSendMsgSize  int
    ConnectionTimeout time.Duration
    TLS             *TLSConfig
    Interceptors    []grpc.UnaryServerInterceptor
    StreamInterceptors []grpc.StreamServerInterceptor
}
```

gRPC 服务器配置。

##### ClientConfig

```go
type ClientConfig struct {
    Target          string
    MaxRecvMsgSize  int
    MaxSendMsgSize  int
    Timeout         time.Duration
    TLS             *ClientTLSConfig
    Interceptors    []grpc.UnaryClientInterceptor
    StreamInterceptors []grpc.StreamClientInterceptor
}
```

gRPC 客户端配置。

#### 函数

##### NewServer

```go
func NewServer(config *ServerConfig) *grpc.Server
```

创建新的 gRPC 服务器。

##### NewClient

```go
func NewClient(config *ClientConfig) (*grpc.ClientConn, error)
```

创建新的 gRPC 客户端连接。

### kcp 包

提供 KCP 协议实现。

#### 类型

##### Config

```go
type Config struct {
    Address         string
    MTU             int
    SndWnd          int
    RcvWnd          int
    NoDelay         bool
    Interval        int
    Resend          int
    NoCongestion    bool
    StreamMode      bool
    Encryption      string
    Key             string
}
```

KCP 配置。

##### Server

```go
type Server interface {
    Start() error
    Stop() error
    SetHandler(handler Handler)
}
```

KCP 服务器接口。

##### Client

```go
type Client interface {
    Connect() error
    Write(data []byte) error
    Read() ([]byte, error)
    Close() error
}
```

KCP 客户端接口。

#### 函数

##### NewServer

```go
func NewServer(config *Config) Server
```

创建新的 KCP 服务器。

##### NewClient

```go
func NewClient(config *Config) Client
```

创建新的 KCP 客户端。

## 基础设施包

### config 包

提供配置管理功能。

#### 类型

##### Manager

```go
type Manager interface {
    Load(sources ...Source) error
    Get(key string) interface{}
    GetString(key string) string
    GetInt(key string) int
    GetBool(key string) bool
    GetDuration(key string) time.Duration
    Set(key string, value interface{})
    Watch(key string, callback func(interface{}))
    Save() error
}
```

配置管理器接口。

##### Source

```go
type Source interface {
    Name() string
    Load() (map[string]interface{}, error)
    Watch(callback func(map[string]interface{})) error
}
```

配置源接口。

#### 函数

##### NewManager

```go
func NewManager() Manager
```

创建新的配置管理器。

##### NewFileSource

```go
func NewFileSource(filename string) Source
```

创建文件配置源。

##### NewEnvSource

```go
func NewEnvSource(prefix string) Source
```

创建环境变量配置源。

### logger 包

提供结构化日志功能。

#### 类型

##### Logger

```go
type Logger interface {
    Debug(args ...interface{})
    Info(args ...interface{})
    Warn(args ...interface{})
    Error(args ...interface{})
    Fatal(args ...interface{})
    
    Debugf(format string, args ...interface{})
    Infof(format string, args ...interface{})
    Warnf(format string, args ...interface{})
    Errorf(format string, args ...interface{})
    Fatalf(format string, args ...interface{})
    
    WithField(key string, value interface{}) Logger
    WithFields(fields Fields) Logger
    WithError(err error) Logger
}
```

日志器接口。

##### Config

```go
type Config struct {
    Level      Level
    Format     string
    Output     string
    Filename   string
    MaxSize    int
    MaxAge     int
    MaxBackups int
    Compress   bool
    Hooks      []Hook
}
```

日志配置。

#### 函数

##### New

```go
func New(config *Config) Logger
```

创建新的日志器。

##### GetLogger

```go
func GetLogger(name string) Logger
```

获取命名日志器。

### metrics 包

提供监控指标功能。

#### 类型

##### Registry

```go
type Registry interface {
    Register(metric Metric) error
    Unregister(name string) error
    Get(name string) Metric
    Collect() []*Sample
}
```

指标注册表接口。

##### Counter

```go
type Counter interface {
    Inc()
    Add(delta float64)
    Get() float64
}
```

计数器接口。

##### Gauge

```go
type Gauge interface {
    Set(value float64)
    Inc()
    Dec()
    Add(delta float64)
    Sub(delta float64)
    Get() float64
}
```

仪表盘接口。

##### Histogram

```go
type Histogram interface {
    Observe(value float64)
    GetCount() uint64
    GetSum() float64
}
```

直方图接口。

#### 函数

##### NewRegistry

```go
func NewRegistry() Registry
```

创建新的指标注册表。

##### NewCounter

```go
func NewCounter(name string, labels Labels) Counter
```

创建新的计数器。

##### NewGauge

```go
func NewGauge(name string, labels Labels) Gauge
```

创建新的仪表盘。

##### NewHistogram

```go
func NewHistogram(name string, labels Labels, buckets []float64) Histogram
```

创建新的直方图。

### discovery 包

提供服务发现功能。

#### 类型

##### ServiceRegistry

```go
type ServiceRegistry interface {
    Register(ctx context.Context, instance *ServiceInstance) error
    Unregister(ctx context.Context, instance *ServiceInstance) error
    GetServices(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
    Watch(ctx context.Context, serviceName string) (Watcher, error)
}
```

服务注册表接口。

##### ServiceInstance

```go
type ServiceInstance struct {
    ID       string
    Name     string
    Address  string
    Port     int
    Health   HealthStatus
    Weight   int
    Metadata map[string]string
    Tags     []string
}
```

服务实例结构。

##### Watcher

```go
type Watcher interface {
    Next() (*ServiceEvent, error)
    Close() error
}
```

服务监听器接口。

#### 函数

##### NewMemoryClient

```go
func NewMemoryClient() ServiceRegistry
```

创建内存服务注册客户端。

##### NewEtcdClient

```go
func NewEtcdClient(endpoints []string) (ServiceRegistry, error)
```

创建 Etcd 服务注册客户端。

### loadbalancer 包

提供负载均衡功能。

#### 类型

##### LoadBalancer

```go
type LoadBalancer interface {
    Select(ctx context.Context, instances []*ServiceInstance) (*ServiceInstance, error)
    UpdateInstances(instances []*ServiceInstance)
}
```

负载均衡器接口。

##### Algorithm

```go
type Algorithm int

const (
    RoundRobin Algorithm = iota
    WeightedRoundRobin
    Random
    WeightedRandom
    LeastConnections
    ConsistentHash
    IPHash
)
```

负载均衡算法枚举。

#### 函数

##### NewLoadBalancer

```go
func NewLoadBalancer(algorithm Algorithm) LoadBalancer
```

创建新的负载均衡器。

##### Select

```go
func Select(ctx context.Context, instances []*ServiceInstance, algorithm Algorithm) (*ServiceInstance, error)
```

使用指定算法选择服务实例。

## 中间件包

### middleware 包

提供各种中间件实现。

#### HTTP 中间件

##### CORS

```go
func CORS(config *CORSConfig) func(http.Handler) http.Handler
```

跨域资源共享中间件。

##### RateLimit

```go
func RateLimit(config *RateLimitConfig) func(http.Handler) http.Handler
```

限流中间件。

##### Auth

```go
func Auth(config *AuthConfig) func(http.Handler) http.Handler
```

认证中间件。

##### Logging

```go
func Logging(logger Logger) func(http.Handler) http.Handler
```

日志中间件。

##### Metrics

```go
func Metrics(registry Registry) func(http.Handler) http.Handler
```

指标中间件。

#### RPC 中间件

##### ServerLogging

```go
func ServerLogging(logger Logger) ServerInterceptor
```

RPC 服务器日志拦截器。

##### ClientRetry

```go
func ClientRetry(config *RetryConfig) ClientInterceptor
```

RPC 客户端重试拦截器。

## 工具包

### codec 包

提供编解码器实现。

#### 类型

##### Codec

```go
type Codec interface {
    Name() string
    Encode(interface{}) ([]byte, error)
    Decode([]byte, interface{}) error
}
```

编解码器接口。

#### 函数

##### GetCodec

```go
func GetCodec(name string) Codec
```

获取指定名称的编解码器。

##### RegisterCodec

```go
func RegisterCodec(codec Codec)
```

注册编解码器。

### pool 包

提供对象池实现。

#### 类型

##### Pool

```go
type Pool interface {
    Get() interface{}
    Put(interface{})
    Size() int
    Close() error
}
```

对象池接口。

#### 函数

##### NewPool

```go
func NewPool(factory func() interface{}, config *Config) Pool
```

创建新的对象池。

### utils 包

提供实用工具函数。

#### 函数

##### GetLocalIP

```go
func GetLocalIP() (string, error)
```

获取本地 IP 地址。

##### GenerateID

```go
func GenerateID() string
```

生成唯一 ID。

##### ParseDuration

```go
func ParseDuration(s string) (time.Duration, error)
```

解析时间间隔字符串。

##### Retry

```go
func Retry(fn func() error, maxRetries int, interval time.Duration) error
```

重试执行函数。

## 错误类型

### 通用错误

```go
var (
    ErrInvalidConfig     = errors.New("invalid configuration")
    ErrConnectionClosed  = errors.New("connection closed")
    ErrTimeout          = errors.New("operation timeout")
    ErrServiceNotFound  = errors.New("service not found")
    ErrMethodNotFound   = errors.New("method not found")
    ErrInvalidRequest   = errors.New("invalid request")
    ErrInvalidResponse  = errors.New("invalid response")
    ErrRateLimitExceeded = errors.New("rate limit exceeded")
    ErrUnauthorized     = errors.New("unauthorized")
    ErrForbidden        = errors.New("forbidden")
)
```

### 错误处理

```go
// 检查错误类型
if errors.Is(err, ErrTimeout) {
    // 处理超时错误
}

// 包装错误
err = fmt.Errorf("failed to connect to server: %w", err)

// 自定义错误类型
type NetworkError struct {
    Op   string
    Addr string
    Err  error
}

func (e *NetworkError) Error() string {
    return fmt.Sprintf("%s %s: %v", e.Op, e.Addr, e.Err)
}

func (e *NetworkError) Unwrap() error {
    return e.Err
}
```

## 常量

### 网络类型

```go
const (
    NetworkTCP  = "tcp"
    NetworkTCP4 = "tcp4"
    NetworkTCP6 = "tcp6"
    NetworkUDP  = "udp"
    NetworkUDP4 = "udp4"
    NetworkUDP6 = "udp6"
)
```

### 编解码器类型

```go
const (
    CodecJSON     = "json"
    CodecGob      = "gob"
    CodecProtobuf = "protobuf"
    CodecMsgPack  = "msgpack"
)
```

### 日志级别

```go
const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
    LevelFatal
)
```

## 使用示例

### 基本服务器

```go
package main

import (
    "github.com/netcore-go/pkg/server"
    "github.com/netcore-go/pkg/logger"
)

func main() {
    // 创建日志器
    log := logger.New(&logger.Config{
        Level:  logger.LevelInfo,
        Format: "json",
        Output: "stdout",
    })
    
    // 创建服务器
    srv := server.NewTCPServer(&server.Config{
        Network: "tcp",
        Address: ":8080",
        MaxConnections: 1000,
    })
    
    // 设置处理器
    srv.SetHandler(&EchoHandler{logger: log})
    
    // 启动服务器
    log.Info("服务器启动在 :8080")
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}

type EchoHandler struct {
    logger logger.Logger
}

func (h *EchoHandler) OnConnect(conn server.Connection) {
    h.logger.Infof("客户端连接: %s", conn.RemoteAddr())
}

func (h *EchoHandler) OnMessage(conn server.Connection, data []byte) {
    conn.Write(data)
}

func (h *EchoHandler) OnDisconnect(conn server.Connection) {
    h.logger.Infof("客户端断开: %s", conn.RemoteAddr())
}
```

### 微服务客户端

```go
package main

import (
    "context"
    "time"
    
    "github.com/netcore-go/pkg/rpc"
    "github.com/netcore-go/pkg/discovery"
    "github.com/netcore-go/pkg/loadbalancer"
)

func main() {
    // 创建服务发现客户端
    registry := discovery.NewMemoryClient()
    
    // 创建 RPC 客户端
    client, err := rpc.NewClient(&rpc.ClientConfig{
        Network: "tcp",
        Address: "localhost:8080",
        Codec:   "json",
        Timeout: 10 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()
    
    // 调用远程方法
    ctx := context.Background()
    var response UserResponse
    
    err = client.Call(ctx, "UserService.GetUser", &UserRequest{
        ID: "123",
    }, &response)
    
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("用户信息: %+v\n", response.User)
}
```

## 版本兼容性

### 语义化版本

NetCore-Go 遵循 [语义化版本](https://semver.org/) 规范：

- **主版本号**：不兼容的 API 修改
- **次版本号**：向下兼容的功能性新增
- **修订号**：向下兼容的问题修正

### API 稳定性

- **稳定 API**：主要的公共接口，保证向后兼容
- **实验性 API**：标记为 `// Experimental` 的 API，可能会发生变化
- **内部 API**：`internal/` 包中的 API，不保证稳定性

### 弃用策略

1. 弃用的 API 会在文档中标记 `// Deprecated`
2. 弃用的 API 至少保留一个主版本周期
3. 提供迁移指南和替代方案

## 性能指标

### 基准测试结果

| 功能 | QPS | 延迟 (P99) | 内存使用 |
|------|-----|-----------|----------|
| TCP 服务器 | 100K+ | < 1ms | < 50MB |
| WebSocket | 50K+ | < 2ms | < 100MB |
| RPC 调用 | 80K+ | < 1.5ms | < 80MB |
| HTTP 网关 | 60K+ | < 3ms | < 120MB |

### 性能优化建议

1. **连接池**：使用连接池减少连接创建开销
2. **缓冲区**：合理设置缓冲区大小
3. **协程池**：控制协程数量避免过度创建
4. **内存池**：使用对象池减少 GC 压力
5. **批量处理**：批量处理消息提高吞吐量

## 故障排除

### 常见问题

1. **连接超时**
   - 检查网络连接
   - 调整超时配置
   - 验证防火墙设置

2. **内存泄漏**
   - 确保连接正确关闭
   - 检查协程是否正确退出
   - 使用 pprof 分析内存使用

3. **性能问题**
   - 调整缓冲区大小
   - 使用连接池和协程池
   - 优化消息处理逻辑

### 调试工具

```go
// 启用调试模式
server.SetDebug(true)

// 启用性能分析
import _ "net/http/pprof"
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// 查看内存使用
go tool pprof http://localhost:6060/debug/pprof/heap

// 查看 CPU 使用
go tool pprof http://localhost:6060/debug/pprof/profile
```

## 社区和支持

- **GitHub**: https://github.com/netcore-go/netcore-go
- **文档**: https://netcore-go.github.io/docs
- **问题反馈**: https://github.com/netcore-go/netcore-go/issues
- **讨论区**: https://github.com/netcore-go/netcore-go/discussions
- **邮件列表**: netcore-go@googlegroups.com

---

本文档持续更新中，如有疑问或建议，请通过 GitHub Issues 反馈。