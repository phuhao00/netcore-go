# RPC 协议

NetCore-Go 提供了高性能的自定义 RPC 协议实现，支持多种编解码器、连接池、负载均衡、拦截器等企业级功能。

## 快速开始

### RPC 服务器

```go
package main

import (
    "context"
    "log"
    
    "github.com/netcore-go/pkg/rpc"
)

// 定义服务接口
type UserService struct{}

// 服务方法
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
    // 模拟数据库查询
    user := &User{
        ID:    req.UserID,
        Name:  "John Doe",
        Email: "john@example.com",
    }
    
    return &GetUserResponse{User: user}, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
    // 模拟用户创建
    user := &User{
        ID:    generateUserID(),
        Name:  req.Name,
        Email: req.Email,
    }
    
    return &CreateUserResponse{User: user}, nil
}

// 请求和响应结构
type GetUserRequest struct {
    UserID string `json:"user_id"`
}

type GetUserResponse struct {
    User *User `json:"user"`
}

type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type CreateUserResponse struct {
    User *User `json:"user"`
}

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    // 创建 RPC 服务器
    server := rpc.NewServer(&rpc.ServerConfig{
        Network: "tcp",
        Address: ":8080",
        Codec:   "json", // 支持 json, gob, protobuf, msgpack
    })
    
    // 注册服务
    if err := server.RegisterService("UserService", &UserService{}); err != nil {
        log.Fatal(err)
    }
    
    // 启动服务器
    log.Println("RPC 服务器启动在 :8080")
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}

func generateUserID() string {
    return fmt.Sprintf("user_%d", time.Now().UnixNano())
}
```

### RPC 客户端

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/netcore-go/pkg/rpc"
)

func main() {
    // 创建 RPC 客户端
    client, err := rpc.NewClient(&rpc.ClientConfig{
        Network: "tcp",
        Address: "localhost:8080",
        Codec:   "json",
        Timeout: 10 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // 调用远程方法
    ctx := context.Background()
    
    // 获取用户
    getUserReq := &GetUserRequest{UserID: "user123"}
    var getUserResp GetUserResponse
    
    if err := client.Call(ctx, "UserService.GetUser", getUserReq, &getUserResp); err != nil {
        log.Printf("调用 GetUser 失败: %v", err)
    } else {
        log.Printf("获取用户成功: %+v", getUserResp.User)
    }
    
    // 创建用户
    createUserReq := &CreateUserRequest{
        Name:  "Jane Smith",
        Email: "jane@example.com",
    }
    var createUserResp CreateUserResponse
    
    if err := client.Call(ctx, "UserService.CreateUser", createUserReq, &createUserResp); err != nil {
        log.Printf("调用 CreateUser 失败: %v", err)
    } else {
        log.Printf("创建用户成功: %+v", createUserResp.User)
    }
}
```

## 协议格式

### 消息结构

```go
type Message struct {
    ID      uint64      `json:"id"`      // 消息 ID
    Type    MessageType `json:"type"`    // 消息类型
    Service string      `json:"service"` // 服务名
    Method  string      `json:"method"`  // 方法名
    Data    []byte      `json:"data"`    // 消息数据
    Error   string      `json:"error"`   // 错误信息
}

type MessageType int

const (
    MessageTypeRequest  MessageType = 1 // 请求消息
    MessageTypeResponse MessageType = 2 // 响应消息
    MessageTypeError    MessageType = 3 // 错误消息
    MessageTypeHeartbeat MessageType = 4 // 心跳消息
)
```

### 二进制协议

```
+--------+--------+--------+--------+--------+--------+--------+--------+
| Magic  | Version| Type   | Flags  |           Message ID              |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                        Service Length                                |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                        Method Length                                 |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                        Data Length                                   |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                        Service Name                                  |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                        Method Name                                   |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                        Message Data                                  |
+--------+--------+--------+--------+--------+--------+--------+--------+
```

## 配置选项

### 服务器配置

```go
type ServerConfig struct {
    // 网络配置
    Network string `json:"network" yaml:"network"` // tcp, tcp4, tcp6
    Address string `json:"address" yaml:"address"` // 监听地址
    
    // 编解码器
    Codec string `json:"codec" yaml:"codec"` // json, gob, protobuf, msgpack
    
    // 连接配置
    MaxConnections int           `json:"max_connections" yaml:"max_connections"`
    ReadTimeout    time.Duration `json:"read_timeout" yaml:"read_timeout"`
    WriteTimeout   time.Duration `json:"write_timeout" yaml:"write_timeout"`
    
    // 缓冲区配置
    ReadBufferSize  int `json:"read_buffer_size" yaml:"read_buffer_size"`
    WriteBufferSize int `json:"write_buffer_size" yaml:"write_buffer_size"`
    
    // 协程池配置
    WorkerPoolSize int `json:"worker_pool_size" yaml:"worker_pool_size"`
    
    // TLS 配置
    TLS *TLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
    
    // 拦截器
    Interceptors []ServerInterceptor `json:"-" yaml:"-"`
}

type TLSConfig struct {
    CertFile string `json:"cert_file" yaml:"cert_file"`
    KeyFile  string `json:"key_file" yaml:"key_file"`
}
```

### 客户端配置

```go
type ClientConfig struct {
    // 网络配置
    Network string `json:"network" yaml:"network"`
    Address string `json:"address" yaml:"address"`
    
    // 编解码器
    Codec string `json:"codec" yaml:"codec"`
    
    // 超时配置
    Timeout       time.Duration `json:"timeout" yaml:"timeout"`
    DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
    ReadTimeout   time.Duration `json:"read_timeout" yaml:"read_timeout"`
    WriteTimeout  time.Duration `json:"write_timeout" yaml:"write_timeout"`
    
    // 重试配置
    MaxRetries    int           `json:"max_retries" yaml:"max_retries"`
    RetryInterval time.Duration `json:"retry_interval" yaml:"retry_interval"`
    
    // 连接池配置
    PoolSize    int           `json:"pool_size" yaml:"pool_size"`
    IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
    
    // TLS 配置
    TLS *ClientTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
    
    // 拦截器
    Interceptors []ClientInterceptor `json:"-" yaml:"-"`
}

type ClientTLSConfig struct {
    CertFile           string `json:"cert_file" yaml:"cert_file"`
    KeyFile            string `json:"key_file" yaml:"key_file"`
    CAFile             string `json:"ca_file" yaml:"ca_file"`
    InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
}
```

## 编解码器

### 支持的编解码器

```go
type Codec interface {
    Name() string
    Encode(interface{}) ([]byte, error)
    Decode([]byte, interface{}) error
}

// JSON 编解码器
type JSONCodec struct{}

func (c *JSONCodec) Name() string {
    return "json"
}

func (c *JSONCodec) Encode(v interface{}) ([]byte, error) {
    return json.Marshal(v)
}

func (c *JSONCodec) Decode(data []byte, v interface{}) error {
    return json.Unmarshal(data, v)
}

// Gob 编解码器
type GobCodec struct{}

func (c *GobCodec) Name() string {
    return "gob"
}

func (c *GobCodec) Encode(v interface{}) ([]byte, error) {
    var buf bytes.Buffer
    enc := gob.NewEncoder(&buf)
    if err := enc.Encode(v); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

func (c *GobCodec) Decode(data []byte, v interface{}) error {
    buf := bytes.NewBuffer(data)
    dec := gob.NewDecoder(buf)
    return dec.Decode(v)
}
```

### 自定义编解码器

```go
// Protocol Buffers 编解码器
type ProtobufCodec struct{}

func (c *ProtobufCodec) Name() string {
    return "protobuf"
}

func (c *ProtobufCodec) Encode(v interface{}) ([]byte, error) {
    if pb, ok := v.(proto.Message); ok {
        return proto.Marshal(pb)
    }
    return nil, errors.New("value is not a protobuf message")
}

func (c *ProtobufCodec) Decode(data []byte, v interface{}) error {
    if pb, ok := v.(proto.Message); ok {
        return proto.Unmarshal(data, pb)
    }
    return errors.New("value is not a protobuf message")
}

// 注册自定义编解码器
func init() {
    rpc.RegisterCodec(&ProtobufCodec{})
}
```

## 拦截器

### 服务器拦截器

```go
type ServerInterceptor interface {
    Intercept(ctx context.Context, req *Request, info *ServerInfo, handler ServerHandler) (*Response, error)
}

type ServerHandler func(ctx context.Context, req *Request) (*Response, error)

type ServerInfo struct {
    Service string
    Method  string
}

// 日志拦截器
type LoggingInterceptor struct {
    logger *log.Logger
}

func (i *LoggingInterceptor) Intercept(ctx context.Context, req *Request, info *ServerInfo, handler ServerHandler) (*Response, error) {
    start := time.Now()
    
    i.logger.Printf("[RPC] 开始处理: %s.%s", info.Service, info.Method)
    
    resp, err := handler(ctx, req)
    
    duration := time.Since(start)
    if err != nil {
        i.logger.Printf("[RPC] 处理失败: %s.%s, 耗时: %v, 错误: %v", info.Service, info.Method, duration, err)
    } else {
        i.logger.Printf("[RPC] 处理成功: %s.%s, 耗时: %v", info.Service, info.Method, duration)
    }
    
    return resp, err
}

// 认证拦截器
type AuthInterceptor struct {
    tokenValidator TokenValidator
}

func (i *AuthInterceptor) Intercept(ctx context.Context, req *Request, info *ServerInfo, handler ServerHandler) (*Response, error) {
    // 从请求中提取认证信息
    token := req.GetMetadata("authorization")
    if token == "" {
        return nil, errors.New("missing authorization token")
    }
    
    // 验证 token
    userID, err := i.tokenValidator.Validate(token)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    
    // 将用户信息添加到上下文
    ctx = context.WithValue(ctx, "user_id", userID)
    
    return handler(ctx, req)
}

// 限流拦截器
type RateLimitInterceptor struct {
    limiter *rate.Limiter
}

func (i *RateLimitInterceptor) Intercept(ctx context.Context, req *Request, info *ServerInfo, handler ServerHandler) (*Response, error) {
    if !i.limiter.Allow() {
        return nil, errors.New("rate limit exceeded")
    }
    
    return handler(ctx, req)
}
```

### 客户端拦截器

```go
type ClientInterceptor interface {
    Intercept(ctx context.Context, method string, req, resp interface{}, invoker ClientInvoker) error
}

type ClientInvoker func(ctx context.Context, method string, req, resp interface{}) error

// 重试拦截器
type RetryInterceptor struct {
    maxRetries int
    interval   time.Duration
}

func (i *RetryInterceptor) Intercept(ctx context.Context, method string, req, resp interface{}, invoker ClientInvoker) error {
    var lastErr error
    
    for attempt := 0; attempt <= i.maxRetries; attempt++ {
        if attempt > 0 {
            select {
            case <-time.After(i.interval):
            case <-ctx.Done():
                return ctx.Err()
            }
        }
        
        err := invoker(ctx, method, req, resp)
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // 检查是否应该重试
        if !shouldRetry(err) {
            break
        }
    }
    
    return lastErr
}

func shouldRetry(err error) bool {
    // 网络错误或临时错误可以重试
    if netErr, ok := err.(net.Error); ok {
        return netErr.Temporary() || netErr.Timeout()
    }
    return false
}

// 超时拦截器
type TimeoutInterceptor struct {
    timeout time.Duration
}

func (i *TimeoutInterceptor) Intercept(ctx context.Context, method string, req, resp interface{}, invoker ClientInvoker) error {
    ctx, cancel := context.WithTimeout(ctx, i.timeout)
    defer cancel()
    
    return invoker(ctx, method, req, resp)
}
```

### 使用拦截器

```go
// 服务器端
server := rpc.NewServer(config)

// 添加拦截器
server.Use(&LoggingInterceptor{logger: log.New(os.Stdout, "[RPC] ", log.LstdFlags)})
server.Use(&AuthInterceptor{tokenValidator: &JWTValidator{}})
server.Use(&RateLimitInterceptor{limiter: rate.NewLimiter(100, 10)})

// 客户端
client, err := rpc.NewClient(config)
if err != nil {
    log.Fatal(err)
}

// 添加拦截器
client.Use(&RetryInterceptor{maxRetries: 3, interval: time.Second})
client.Use(&TimeoutInterceptor{timeout: 10 * time.Second})
```

## 连接池

### 连接池配置

```go
type PoolConfig struct {
    // 连接池大小
    Size int `json:"size" yaml:"size"`
    
    // 最大空闲连接数
    MaxIdle int `json:"max_idle" yaml:"max_idle"`
    
    // 空闲超时时间
    IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
    
    // 连接最大生存时间
    MaxLifetime time.Duration `json:"max_lifetime" yaml:"max_lifetime"`
    
    // 健康检查间隔
    HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
}
```

### 连接池实现

```go
type ConnectionPool struct {
    config   *PoolConfig
    factory  ConnectionFactory
    
    connections chan *Connection
    active      int
    mutex       sync.RWMutex
    
    closed bool
    done   chan struct{}
}

type ConnectionFactory func() (*Connection, error)

func NewConnectionPool(config *PoolConfig, factory ConnectionFactory) *ConnectionPool {
    pool := &ConnectionPool{
        config:      config,
        factory:     factory,
        connections: make(chan *Connection, config.Size),
        done:        make(chan struct{}),
    }
    
    // 启动健康检查
    go pool.healthCheck()
    
    return pool
}

func (p *ConnectionPool) Get() (*Connection, error) {
    p.mutex.RLock()
    if p.closed {
        p.mutex.RUnlock()
        return nil, errors.New("connection pool is closed")
    }
    p.mutex.RUnlock()
    
    select {
    case conn := <-p.connections:
        if conn.IsHealthy() {
            return conn, nil
        }
        conn.Close()
        // 继续创建新连接
    default:
        // 连接池为空，创建新连接
    }
    
    // 检查是否超过最大连接数
    p.mutex.Lock()
    if p.active >= p.config.Size {
        p.mutex.Unlock()
        return nil, errors.New("connection pool is full")
    }
    p.active++
    p.mutex.Unlock()
    
    conn, err := p.factory()
    if err != nil {
        p.mutex.Lock()
        p.active--
        p.mutex.Unlock()
        return nil, err
    }
    
    return conn, nil
}

func (p *ConnectionPool) Put(conn *Connection) {
    if p.closed || !conn.IsHealthy() {
        conn.Close()
        p.mutex.Lock()
        p.active--
        p.mutex.Unlock()
        return
    }
    
    select {
    case p.connections <- conn:
        // 连接已放回池中
    default:
        // 连接池已满，关闭连接
        conn.Close()
        p.mutex.Lock()
        p.active--
        p.mutex.Unlock()
    }
}

func (p *ConnectionPool) healthCheck() {
    ticker := time.NewTicker(p.config.HealthCheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            p.checkConnections()
        case <-p.done:
            return
        }
    }
}

func (p *ConnectionPool) checkConnections() {
    var healthyConns []*Connection
    
    // 检查池中的连接
    for {
        select {
        case conn := <-p.connections:
            if conn.IsHealthy() && time.Since(conn.CreatedAt()) < p.config.MaxLifetime {
                healthyConns = append(healthyConns, conn)
            } else {
                conn.Close()
                p.mutex.Lock()
                p.active--
                p.mutex.Unlock()
            }
        default:
            goto done
        }
    }
    
done:
    // 将健康的连接放回池中
    for _, conn := range healthyConns {
        select {
        case p.connections <- conn:
        default:
            conn.Close()
            p.mutex.Lock()
            p.active--
            p.mutex.Unlock()
        }
    }
}
```

## 负载均衡

### 负载均衡器接口

```go
type LoadBalancer interface {
    Select(servers []*ServerInfo) (*ServerInfo, error)
    UpdateServers(servers []*ServerInfo)
}

type ServerInfo struct {
    Address string
    Weight  int
    Active  bool
}
```

### 轮询负载均衡

```go
type RoundRobinBalancer struct {
    servers []*ServerInfo
    current int
    mutex   sync.RWMutex
}

func (b *RoundRobinBalancer) Select(servers []*ServerInfo) (*ServerInfo, error) {
    b.mutex.Lock()
    defer b.mutex.Unlock()
    
    if len(servers) == 0 {
        return nil, errors.New("no available servers")
    }
    
    // 过滤活跃服务器
    activeServers := make([]*ServerInfo, 0, len(servers))
    for _, server := range servers {
        if server.Active {
            activeServers = append(activeServers, server)
        }
    }
    
    if len(activeServers) == 0 {
        return nil, errors.New("no active servers")
    }
    
    server := activeServers[b.current%len(activeServers)]
    b.current++
    
    return server, nil
}

func (b *RoundRobinBalancer) UpdateServers(servers []*ServerInfo) {
    b.mutex.Lock()
    defer b.mutex.Unlock()
    
    b.servers = servers
    b.current = 0
}
```

### 加权随机负载均衡

```go
type WeightedRandomBalancer struct {
    servers []*ServerInfo
    weights []int
    total   int
    mutex   sync.RWMutex
    rand    *rand.Rand
}

func NewWeightedRandomBalancer() *WeightedRandomBalancer {
    return &WeightedRandomBalancer{
        rand: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (b *WeightedRandomBalancer) Select(servers []*ServerInfo) (*ServerInfo, error) {
    b.mutex.RLock()
    defer b.mutex.RUnlock()
    
    if len(servers) == 0 {
        return nil, errors.New("no available servers")
    }
    
    // 计算总权重
    totalWeight := 0
    activeServers := make([]*ServerInfo, 0, len(servers))
    
    for _, server := range servers {
        if server.Active {
            activeServers = append(activeServers, server)
            totalWeight += server.Weight
        }
    }
    
    if len(activeServers) == 0 {
        return nil, errors.New("no active servers")
    }
    
    if totalWeight == 0 {
        // 如果所有权重都为 0，使用随机选择
        return activeServers[b.rand.Intn(len(activeServers))], nil
    }
    
    // 加权随机选择
    target := b.rand.Intn(totalWeight)
    current := 0
    
    for _, server := range activeServers {
        current += server.Weight
        if current > target {
            return server, nil
        }
    }
    
    return activeServers[len(activeServers)-1], nil
}
```

## 服务发现集成

### 服务发现客户端

```go
type DiscoveryClient struct {
    registry   discovery.ServiceRegistry
    balancer   LoadBalancer
    serviceName string
    
    servers []*ServerInfo
    mutex   sync.RWMutex
}

func NewDiscoveryClient(registry discovery.ServiceRegistry, serviceName string) *DiscoveryClient {
    client := &DiscoveryClient{
        registry:    registry,
        balancer:    &RoundRobinBalancer{},
        serviceName: serviceName,
    }
    
    // 启动服务发现
    go client.watchServices()
    
    return client
}

func (c *DiscoveryClient) watchServices() {
    watcher, err := c.registry.Watch(context.Background(), c.serviceName)
    if err != nil {
        log.Printf("启动服务监听失败: %v", err)
        return
    }
    defer watcher.Close()
    
    for {
        event, err := watcher.Next()
        if err != nil {
            log.Printf("获取服务事件失败: %v", err)
            continue
        }
        
        c.handleServiceEvent(event)
    }
}

func (c *DiscoveryClient) handleServiceEvent(event *discovery.ServiceEvent) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    switch event.Type {
    case discovery.EventTypeAdd:
        c.addServer(event.Instance)
    case discovery.EventTypeUpdate:
        c.updateServer(event.Instance)
    case discovery.EventTypeDelete:
        c.removeServer(event.Instance)
    }
    
    c.balancer.UpdateServers(c.servers)
}

func (c *DiscoveryClient) GetServer() (*ServerInfo, error) {
    c.mutex.RLock()
    servers := make([]*ServerInfo, len(c.servers))
    copy(servers, c.servers)
    c.mutex.RUnlock()
    
    return c.balancer.Select(servers)
}
```

### RPC 客户端集成

```go
type DiscoveryRPCClient struct {
    discovery *DiscoveryClient
    config    *ClientConfig
    pools     map[string]*ConnectionPool
    mutex     sync.RWMutex
}

func NewDiscoveryRPCClient(registry discovery.ServiceRegistry, serviceName string, config *ClientConfig) *DiscoveryRPCClient {
    return &DiscoveryRPCClient{
        discovery: NewDiscoveryClient(registry, serviceName),
        config:    config,
        pools:     make(map[string]*ConnectionPool),
    }
}

func (c *DiscoveryRPCClient) Call(ctx context.Context, method string, req, resp interface{}) error {
    server, err := c.discovery.GetServer()
    if err != nil {
        return err
    }
    
    pool := c.getConnectionPool(server.Address)
    conn, err := pool.Get()
    if err != nil {
        return err
    }
    defer pool.Put(conn)
    
    return conn.Call(ctx, method, req, resp)
}

func (c *DiscoveryRPCClient) getConnectionPool(address string) *ConnectionPool {
    c.mutex.RLock()
    pool, exists := c.pools[address]
    c.mutex.RUnlock()
    
    if exists {
        return pool
    }
    
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    // 双重检查
    if pool, exists := c.pools[address]; exists {
        return pool
    }
    
    // 创建新的连接池
    poolConfig := &PoolConfig{
        Size:        c.config.PoolSize,
        MaxIdle:     c.config.PoolSize / 2,
        IdleTimeout: c.config.IdleTimeout,
    }
    
    factory := func() (*Connection, error) {
        return NewConnection(address, c.config)
    }
    
    pool = NewConnectionPool(poolConfig, factory)
    c.pools[address] = pool
    
    return pool
}
```

## 监控和指标

### 内置指标

```go
type RPCMetrics struct {
    // 服务器指标
    RequestsTotal        prometheus.Counter
    RequestDuration      prometheus.Histogram
    ActiveConnections    prometheus.Gauge
    
    // 客户端指标
    ClientRequestsTotal  prometheus.Counter
    ClientRequestDuration prometheus.Histogram
    ClientErrors         prometheus.Counter
    
    // 连接池指标
    PoolConnections      prometheus.GaugeVec
    PoolWaitDuration     prometheus.Histogram
}
```

### 指标收集

```go
type MetricsInterceptor struct {
    metrics *RPCMetrics
}

func (i *MetricsInterceptor) Intercept(ctx context.Context, req *Request, info *ServerInfo, handler ServerHandler) (*Response, error) {
    start := time.Now()
    
    // 增加请求计数
    i.metrics.RequestsTotal.Inc()
    
    resp, err := handler(ctx, req)
    
    // 记录请求时长
    duration := time.Since(start)
    i.metrics.RequestDuration.Observe(duration.Seconds())
    
    return resp, err
}
```

## 最佳实践

### 1. 服务定义

```go
// 使用接口定义服务
type UserServiceInterface interface {
    GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error)
    CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error)
    UpdateUser(ctx context.Context, req *UpdateUserRequest) (*UpdateUserResponse, error)
    DeleteUser(ctx context.Context, req *DeleteUserRequest) (*DeleteUserResponse, error)
}

// 实现接口
type UserService struct {
    userRepo UserRepository
    logger   Logger
}

func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
    // 参数验证
    if req.UserID == "" {
        return nil, errors.New("user_id is required")
    }
    
    // 业务逻辑
    user, err := s.userRepo.GetByID(ctx, req.UserID)
    if err != nil {
        s.logger.Errorf("获取用户失败: %v", err)
        return nil, err
    }
    
    return &GetUserResponse{User: user}, nil
}
```

### 2. 错误处理

```go
// 定义错误类型
type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

func (e *RPCError) Error() string {
    return fmt.Sprintf("RPC Error %d: %s", e.Code, e.Message)
}

// 错误码定义
const (
    ErrorCodeInvalidRequest = 400
    ErrorCodeUnauthorized   = 401
    ErrorCodeNotFound       = 404
    ErrorCodeInternalError  = 500
)

// 使用结构化错误
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
    if req.UserID == "" {
        return nil, &RPCError{
            Code:    ErrorCodeInvalidRequest,
            Message: "Invalid request",
            Details: "user_id is required",
        }
    }
    
    user, err := s.userRepo.GetByID(ctx, req.UserID)
    if err != nil {
        if errors.Is(err, ErrUserNotFound) {
            return nil, &RPCError{
                Code:    ErrorCodeNotFound,
                Message: "User not found",
                Details: fmt.Sprintf("user_id: %s", req.UserID),
            }
        }
        
        return nil, &RPCError{
            Code:    ErrorCodeInternalError,
            Message: "Internal server error",
        }
    }
    
    return &GetUserResponse{User: user}, nil
}
```

### 3. 上下文使用

```go
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
    // 检查上下文取消
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // 从上下文获取用户信息
    userID := ctx.Value("user_id").(string)
    
    // 传递上下文到下层调用
    user, err := s.userRepo.GetByID(ctx, req.UserID)
    if err != nil {
        return nil, err
    }
    
    return &GetUserResponse{User: user}, nil
}
```

### 4. 优雅关闭

```go
func main() {
    server := rpc.NewServer(config)
    server.RegisterService("UserService", &UserService{})
    
    // 处理关闭信号
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-c
        log.Println("正在关闭 RPC 服务器...")
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := server.Shutdown(ctx); err != nil {
            log.Printf("服务器关闭失败: %v", err)
        }
    }()
    
    server.Start()
}
```

## 故障排除

### 常见问题

1. **连接超时**
   - 检查网络连接
   - 调整超时配置
   - 验证服务器状态

2. **序列化错误**
   - 确认编解码器配置
   - 检查数据结构定义
   - 验证数据类型匹配

3. **内存泄漏**
   - 确保连接正确关闭
   - 检查协程退出
   - 监控连接池状态

### 调试工具

```go
// 启用调试模式
server.SetDebug(true)
client.SetDebug(true)

// 添加调试拦截器
server.Use(&DebugInterceptor{})
client.Use(&DebugInterceptor{})

// 启用性能分析
import _ "net/http/pprof"
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

## 示例项目

完整的示例项目可以在 `examples/rpc/` 目录中找到：

- `examples/rpc/basic/` - 基本 RPC 服务
- `examples/rpc/advanced/` - 高级功能示例
- `examples/rpc/microservice/` - 微服务架构
- `examples/rpc/discovery/` - 服务发现集成

## 参考资料

- [gRPC 官方文档](https://grpc.io/docs/)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)
- [微服务架构模式](https://microservices.io/)
- [分布式系统原理](./distributed-systems.md)