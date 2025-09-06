# NetCore-Go API Documentation

## Core Interfaces

### Server Interface

```go
type Server interface {
    Start() error
    Stop() error
    SetMessageHandler(handler MessageHandler)
    SetConnectHandler(handler ConnectHandler)
    SetDisconnectHandler(handler DisconnectHandler)
    GetStats() *ServerStats
}
```

### Connection Interface

```go
type Connection interface {
    ID() string
    RemoteAddr() string
    LocalAddr() string
    Write(data []byte) error
    Close() error
    IsActive() bool
    GetMetadata(key string) interface{}
    SetMetadata(key string, value interface{})
}
```

### Configuration

```go
type Config struct {
    Network        string        // "tcp", "udp", "websocket", "http"
    Address        string        // Server address (e.g., ":8080")
    MaxClients     int           // Maximum concurrent clients
    BufferSize     int           // Buffer size for read/write operations
    ReadTimeout    time.Duration // Read timeout
    WriteTimeout   time.Duration // Write timeout
    IdleTimeout    time.Duration // Idle connection timeout
    
    // WebSocket specific
    Path           string        // WebSocket path (e.g., "/ws")
    
    // HTTP specific
    ReadHeaderTimeout time.Duration
    MaxHeaderBytes    int
    
    // TLS configuration
    TLSConfig      *tls.Config
    CertFile       string
    KeyFile        string
}
```

## Middleware System

### Middleware Interface

```go
type Middleware interface {
    Process(ctx *Context, next NextFunc) error
    Name() string
    Priority() int
}
```

### Built-in Middleware

#### JWT Authentication

```go
jwtConfig := &middleware.JWTConfig{
    SecretKey:     "your-secret-key",
    TokenLookup:   "header:Authorization",
    TokenHeadName: "Bearer",
    Expiration:    time.Hour * 24,
}
jwtMiddleware := middleware.NewJWTMiddleware(jwtConfig)
```

#### Rate Limiting

```go
rateLimitConfig := &middleware.RateLimitConfig{
    Algorithm:         middleware.TokenBucket,
    RequestsPerSecond: 100,
    BurstSize:        200,
}
rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimitConfig)
```

#### Caching

```go
cacheConfig := &middleware.CacheConfig{
    Store:      middleware.MemoryStore,
    TTL:        time.Minute * 10,
    MaxEntries: 1000,
}
cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig)
```

## Connection Pooling

```go
poolConfig := &pool.Config{
    MaxConnections:    1000,
    MaxIdleTime:       time.Minute * 5,
    CleanupInterval:   time.Minute,
    HealthCheckPeriod: time.Second * 30,
}

connPool := pool.NewConnectionPool(poolConfig)
connPool.AddConnection(conn)
conn := connPool.GetConnection(connID)
connPool.RemoveConnection(connID)
```

## Message Queues

### Memory Queue

```go
queueConfig := &queue.Config{
    MaxSize:     10000,
    Workers:     10,
    BufferSize:  100,
}

memoryQueue := queue.NewMemoryQueue(queueConfig)
producer := queue.NewMemoryProducer(memoryQueue)
consumer := queue.NewMemoryConsumer(memoryQueue)

// Publish message
message := &queue.Message{
    ID:       "msg-1",
    Topic:    "user.created",
    Data:     []byte(`{"user_id": 123}`),
    Priority: queue.HighPriority,
}
producer.Publish("user.created", message)

// Subscribe to messages
consumer.Subscribe("user.created", func(msg *queue.Message) error {
    log.Printf("Received message: %s", string(msg.Data))
    return nil
})
```

## Long Polling

```go
longPollConfig := &longpoll.Config{
    MaxSubscriptions: 1000,
    EventTimeout:     time.Second * 30,
    CleanupInterval:  time.Minute,
}

longPollManager := longpoll.NewManager(longPollConfig)
longPollManager.Start()

// Publish event
event := &longpoll.Event{
    ID:   "event-1",
    Type: "user.updated",
    Data: map[string]interface{}{"user_id": 123},
}
longPollManager.Publish("user.events", event)

// Subscribe to events
subscription := longPollManager.Subscribe("user.events")
select {
case event := <-subscription.Events:
    log.Printf("Received event: %+v", event)
case <-time.After(time.Second * 30):
    log.Println("Timeout waiting for events")
}
```

## Heartbeat and Reconnection

```go
heartbeatConfig := &heartbeat.Config{
    Interval:    time.Second * 30,
    Timeout:     time.Second * 10,
    MaxRetries:  3,
    Type:        heartbeat.TypePing,
    AutoRestart: true,
}

detector := heartbeat.NewDetector(heartbeatConfig)
detector.Start()

// Reconnection manager
reconnectConfig := &heartbeat.ReconnectConfig{
    Strategy:     heartbeat.ExponentialBackoff,
    InitialDelay: time.Second,
    MaxDelay:     time.Minute * 5,
    MaxRetries:   10,
}

reconnectManager := heartbeat.NewReconnectManager(reconnectConfig)
reconnectManager.SetConnectionFactory(func() (interface{}, error) {
    return net.Dial("tcp", "localhost:8080")
})
reconnectManager.Start()
```