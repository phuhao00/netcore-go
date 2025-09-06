# NetCore-Go Best Practices Guide

## 🏗️ Architecture Best Practices

### 1. Server Configuration

#### Proper Resource Limits

```go
config := &netcore.Config{
    Network:     "tcp",
    Address:     ":8080",
    MaxClients:  1000,           // Set based on expected load
    BufferSize:  4096,           // Tune based on message size
    ReadTimeout: time.Second * 30, // Prevent hanging connections
    WriteTimeout: time.Second * 10,
    IdleTimeout: time.Minute * 5,  // Clean up idle connections
}
```

#### Environment-based Configuration

```go
func loadConfig() *netcore.Config {
    return &netcore.Config{
        Network:     getEnv("SERVER_NETWORK", "tcp"),
        Address:     getEnv("SERVER_ADDRESS", ":8080"),
        MaxClients:  getEnvInt("MAX_CLIENTS", 1000),
        BufferSize:  getEnvInt("BUFFER_SIZE", 4096),
        ReadTimeout: getEnvDuration("READ_timeout", time.Second*30),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### 2. Error Handling

#### Comprehensive Error Handling

```go
func startServer() error {
    server, err := netcore.NewServer(config)
    if err != nil {
        return fmt.Errorf("failed to create server: %w", err)
    }
    
    // Set error handler for connection errors
    server.SetErrorHandler(func(conn core.Connection, err error) {
        log.Printf("Connection error from %s: %v", conn.RemoteAddr(), err)
        // Implement retry logic or cleanup
    })
    
    if err := server.Start(); err != nil {
        return fmt.Errorf("failed to start server: %w", err)
    }
    
    return nil
}
```

#### Graceful Error Recovery

```go
server.SetMessageHandler(func(conn core.Connection, data []byte) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic in message handler: %v", r)
            // Send error response to client
            errorResponse := []byte(`{"error":"Internal server error"}`)
            conn.Write(errorResponse)
        }
    }()
    
    // Your message handling logic here
    processMessage(conn, data)
})
```

### 3. Graceful Shutdown

#### Signal Handling

```go
func main() {
    server := setupServer()
    
    // Create shutdown channel
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
    
    // Start server in goroutine
    go func() {
        if err := server.Start(); err != nil {
            log.Printf("Server error: %v", err)
        }
    }()
    
    // Wait for shutdown signal
    <-shutdown
    log.Println("Shutting down server...")
    
    // Create shutdown context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
    defer cancel()
    
    // Graceful shutdown
    if err := server.StopWithContext(ctx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }
    
    log.Println("Server stopped")
}
```

## 🚀 Performance Best Practices

### 1. Connection Pooling

#### Optimal Pool Configuration

```go
// Configure connection pool based on expected load
poolConfig := &pool.Config{
    MaxConnections:    1000,              // Based on server capacity
    MaxIdleTime:       time.Minute * 5,   // Clean up idle connections
    CleanupInterval:   time.Minute,       // Regular cleanup
    HealthCheckPeriod: time.Second * 30,  // Monitor connection health
}

connPool := pool.NewConnectionPool(poolConfig)

// Use pool in connection handler
server.SetConnectHandler(func(conn core.Connection) {
    connPool.AddConnection(conn)
    log.Printf("Connection added to pool: %s (total: %d)", 
        conn.RemoteAddr(), connPool.GetStats().ActiveConnections)
})
```

### 2. Memory Management

#### Object Pooling

```go
// Use sync.Pool for frequently allocated objects
var messagePool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}

server.SetMessageHandler(func(conn core.Connection, data []byte) {
    // Get buffer from pool
    buffer := messagePool.Get().([]byte)
    defer messagePool.Put(buffer)
    
    // Use buffer for processing
    processWithBuffer(buffer, data)
})
```

#### Avoid Memory Leaks

```go
// Always clean up resources
server.SetDisconnectHandler(func(conn core.Connection) {
    // Remove from connection pool
    connPool.RemoveConnection(conn.ID())
    
    // Clean up connection-specific resources
    if userData := conn.GetMetadata("user_data"); userData != nil {
        // Clean up user-specific data
        cleanupUserData(userData)
        conn.SetMetadata("user_data", nil)
    }
    
    log.Printf("Connection cleaned up: %s", conn.RemoteAddr())
})
```

### 3. Goroutine Management

#### Worker Pool Pattern

```go
type WorkerPool struct {
    workers    int
    jobQueue   chan Job
    workerPool chan chan Job
    quit       chan bool
}

func NewWorkerPool(workers int, jobQueueSize int) *WorkerPool {
    return &WorkerPool{
        workers:    workers,
        jobQueue:   make(chan Job, jobQueueSize),
        workerPool: make(chan chan Job, workers),
        quit:       make(chan bool),
    }
}

// Use worker pool for message processing
server.SetMessageHandler(func(conn core.Connection, data []byte) {
    job := Job{
        Connection: conn,
        Data:       data,
    }
    
    select {
    case workerPool.jobQueue <- job:
        // Job queued successfully
    default:
        // Queue is full, handle overflow
        log.Printf("Job queue full, dropping message from %s", conn.RemoteAddr())
    }
})
```

## 🔒 Security Best Practices

### 1. Input Validation

#### Validate All Input

```go
func validateMessage(data []byte) error {
    // Check message size
    if len(data) > maxMessageSize {
        return fmt.Errorf("message too large: %d bytes", len(data))
    }
    
    // Validate JSON structure
    var msg map[string]interface{}
    if err := json.Unmarshal(data, &msg); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    
    // Validate required fields
    if _, ok := msg["type"]; !ok {
        return fmt.Errorf("missing required field: type")
    }
    
    return nil
}

server.SetMessageHandler(func(conn core.Connection, data []byte) {
    if err := validateMessage(data); err != nil {
        log.Printf("Invalid message from %s: %v", conn.RemoteAddr(), err)
        conn.Close()
        return
    }
    
    // Process valid message
    processMessage(conn, data)
})
```

### 2. Rate Limiting

#### Implement Rate Limiting

```go
// Per-connection rate limiting
type ConnectionLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
}

func (cl *ConnectionLimiter) Allow(connID string) bool {
    cl.mu.RLock()
    limiter, exists := cl.limiters[connID]
    cl.mu.RUnlock()
    
    if !exists {
        cl.mu.Lock()
        limiter = rate.NewLimiter(rate.Limit(10), 20) // 10 req/sec, burst 20
        cl.limiters[connID] = limiter
        cl.mu.Unlock()
    }
    
    return limiter.Allow()
}

connLimiter := &ConnectionLimiter{
    limiters: make(map[string]*rate.Limiter),
}

server.SetMessageHandler(func(conn core.Connection, data []byte) {
    if !connLimiter.Allow(conn.ID()) {
        log.Printf("Rate limit exceeded for %s", conn.RemoteAddr())
        conn.Write([]byte(`{"error":"Rate limit exceeded"}`))
        return
    }
    
    processMessage(conn, data)
})
```

### 3. Authentication and Authorization

#### JWT Authentication

```go
func authenticateConnection(conn core.Connection, token string) error {
    claims, err := validateJWT(token)
    if err != nil {
        return fmt.Errorf("invalid token: %w", err)
    }
    
    // Store user info in connection metadata
    conn.SetMetadata("user_id", claims.UserID)
    conn.SetMetadata("user_role", claims.Role)
    conn.SetMetadata("authenticated", true)
    
    return nil
}

func requireAuth(conn core.Connection) bool {
    auth := conn.GetMetadata("authenticated")
    return auth != nil && auth.(bool)
}

server.SetMessageHandler(func(conn core.Connection, data []byte) {
    if !requireAuth(conn) {
        conn.Write([]byte(`{"error":"Authentication required"}`))
        conn.Close()
        return
    }
    
    processMessage(conn, data)
})
```

## 📊 Monitoring and Logging

### 1. Structured Logging

#### Use Structured Logging

```go
import (
    "github.com/sirupsen/logrus"
)

func setupLogging() *logrus.Logger {
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{})
    logger.SetLevel(logrus.InfoLevel)
    
    return logger
}

logger := setupLogging()

server.SetMessageHandler(func(conn core.Connection, data []byte) {
    start := time.Now()
    
    logger.WithFields(logrus.Fields{
        "client_addr": conn.RemoteAddr(),
        "message_size": len(data),
        "connection_id": conn.ID(),
    }).Info("Processing message")
    
    processMessage(conn, data)
    
    logger.WithFields(logrus.Fields{
        "client_addr": conn.RemoteAddr(),
        "duration": time.Since(start),
        "connection_id": conn.ID(),
    }).Info("Message processed")
})
```

### 2. Metrics Collection

#### Collect Performance Metrics

```go
type ServerMetrics struct {
    TotalConnections    int64
    ActiveConnections   int64
    TotalMessages       int64
    MessageRate         float64
    AverageResponseTime time.Duration
    ErrorRate           float64
    mu                  sync.RWMutex
}

func (m *ServerMetrics) IncrementConnections() {
    atomic.AddInt64(&m.TotalConnections, 1)
    atomic.AddInt64(&m.ActiveConnections, 1)
}

func (m *ServerMetrics) DecrementConnections() {
    atomic.AddInt64(&m.ActiveConnections, -1)
}

func (m *ServerMetrics) IncrementMessages() {
    atomic.AddInt64(&m.TotalMessages, 1)
}

// Expose metrics via HTTP endpoint
http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(serverMetrics)
})
```

### 3. Health Checks

#### Implement Health Checks

```go
type HealthChecker struct {
    server *netcore.Server
    db     *sql.DB
    cache  *redis.Client
}

func (hc *HealthChecker) CheckHealth() map[string]interface{} {
    health := map[string]interface{}{
        "status": "healthy",
        "timestamp": time.Now(),
        "checks": map[string]interface{}{},
    }
    
    // Check server status
    if hc.server.IsRunning() {
        health["checks"].(map[string]interface{})["server"] = "healthy"
    } else {
        health["checks"].(map[string]interface{})["server"] = "unhealthy"
        health["status"] = "unhealthy"
    }
    
    // Check database connection
    if err := hc.db.Ping(); err != nil {
        health["checks"].(map[string]interface{})["database"] = "unhealthy"
        health["status"] = "unhealthy"
    } else {
        health["checks"].(map[string]interface{})["database"] = "healthy"
    }
    
    return health
}

// Expose health check endpoint
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    health := healthChecker.CheckHealth()
    
    w.Header().Set("Content-Type", "application/json")
    if health["status"] == "healthy" {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(health)
})
```

## 🧪 Testing Best Practices

### 1. Unit Testing

#### Test Server Components

```go
func TestTCPServer(t *testing.T) {
    config := &netcore.Config{
        Network: "tcp",
        Address: ":0", // Use random port for testing
        MaxClients: 10,
    }
    
    server, err := netcore.NewServer(config)
    require.NoError(t, err)
    
    // Set up test handlers
    var receivedData []byte
    server.SetMessageHandler(func(conn core.Connection, data []byte) {
        receivedData = data
        conn.Write([]byte("echo: " + string(data)))
    })
    
    // Start server
    go server.Start()
    defer server.Stop()
    
    // Wait for server to start
    time.Sleep(time.Millisecond * 100)
    
    // Test client connection
    conn, err := net.Dial("tcp", server.Address())
    require.NoError(t, err)
    defer conn.Close()
    
    // Send test message
    testMessage := "hello world"
    _, err = conn.Write([]byte(testMessage))
    require.NoError(t, err)
    
    // Read response
    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    require.NoError(t, err)
    
    expectedResponse := "echo: " + testMessage
    assert.Equal(t, expectedResponse, string(buffer[:n]))
    assert.Equal(t, []byte(testMessage), receivedData)
}
```

### 2. Integration Testing

#### Test Complete Workflows

```go
func TestWebSocketChat(t *testing.T) {
    // Start WebSocket server
    server := startTestWebSocketServer(t)
    defer server.Stop()
    
    // Connect multiple clients
    client1 := connectWebSocketClient(t, server.Address())
    client2 := connectWebSocketClient(t, server.Address())
    defer client1.Close()
    defer client2.Close()
    
    // Test message broadcasting
    message := map[string]interface{}{
        "type": "chat",
        "text": "Hello everyone!",
        "user": "user1",
    }
    
    // Send message from client1
    err := client1.WriteJSON(message)
    require.NoError(t, err)
    
    // Verify client2 receives the message
    var receivedMessage map[string]interface{}
    err = client2.ReadJSON(&receivedMessage)
    require.NoError(t, err)
    
    assert.Equal(t, message["text"], receivedMessage["text"])
    assert.Equal(t, message["user"], receivedMessage["user"])
}
```

### 3. Load Testing

#### Performance Testing

```go
func BenchmarkTCPServer(b *testing.B) {
    server := startTestTCPServer(b)
    defer server.Stop()
    
    // Create connection pool for testing
    connections := make([]net.Conn, 100)
    for i := 0; i < 100; i++ {
        conn, err := net.Dial("tcp", server.Address())
        require.NoError(b, err)
        connections[i] = conn
        defer conn.Close()
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        connIndex := 0
        for pb.Next() {
            conn := connections[connIndex%len(connections)]
            
            // Send message
            message := fmt.Sprintf("test message %d", connIndex)
            _, err := conn.Write([]byte(message))
            if err != nil {
                b.Fatal(err)
            }
            
            // Read response
            buffer := make([]byte, 1024)
            _, err = conn.Read(buffer)
            if err != nil {
                b.Fatal(err)
            }
            
            connIndex++
        }
    })
}
```

## 🚀 Deployment Best Practices

### 1. Configuration Management

#### Use Configuration Files

```yaml
# config.yaml
server:
  network: tcp
  address: ":8080"
  max_clients: 1000
  buffer_size: 4096
  read_timeout: 30s
  write_timeout: 10s

middleware:
  rate_limit:
    requests_per_second: 100
    burst_size: 200
  
  jwt:
    secret_key: ${JWT_SECRET}
    expiration: 24h

logging:
  level: info
  format: json
  output: stdout

metrics:
  enabled: true
  port: 9090
```

### 2. Docker Deployment

#### Dockerfile

```dockerfile
# Build stage
FROM golang:1.19-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/config.yaml .

EXPOSE 8080
CMD ["./server"]
```

### 3. Kubernetes Deployment

#### Deployment YAML

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: netcore-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: netcore-server
  template:
    metadata:
      labels:
        app: netcore-server
    spec:
      containers:
      - name: server
        image: netcore-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: netcore-secrets
              key: jwt-secret
        resources:
          requests:
            memory: "64Mi"
            cpu: "250m"
          limits:
            memory: "128Mi"
            cpu: "500m"
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
---
apiVersion: v1
kind: Service
metadata:
  name: netcore-service
spec:
  selector:
    app: netcore-server
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

These best practices will help you build robust, scalable, and maintainable applications with NetCore-Go.