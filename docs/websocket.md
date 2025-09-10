# WebSocket 服务器

NetCore-Go 提供了功能完整的 WebSocket 服务器实现，支持实时双向通信、房间管理、消息广播等功能。

## 快速开始

### 基本 WebSocket 服务器

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
            return true // 允许所有来源，生产环境需要严格检查
        },
        BufferSize: 1024,
    })
    
    // 设置消息处理器
    ws.SetHandler(&EchoHandler{})
    
    // 注册 WebSocket 路由
    http.HandleFunc("/ws", ws.HandleWebSocket)
    
    // 启动 HTTP 服务器
    log.Println("WebSocket 服务器启动在 :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// WebSocket 消息处理器
type EchoHandler struct{}

func (h *EchoHandler) OnConnect(conn websocket.Connection) {
    log.Printf("WebSocket 连接建立: %s", conn.RemoteAddr())
}

func (h *EchoHandler) OnMessage(conn websocket.Connection, messageType int, data []byte) {
    log.Printf("收到消息: %s", string(data))
    
    // 回显消息
    if err := conn.WriteMessage(messageType, data); err != nil {
        log.Printf("发送消息失败: %v", err)
    }
}

func (h *EchoHandler) OnDisconnect(conn websocket.Connection) {
    log.Printf("WebSocket 连接断开: %s", conn.RemoteAddr())
}
```

### 前端 JavaScript 客户端

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket 测试</title>
</head>
<body>
    <div id="messages"></div>
    <input type="text" id="messageInput" placeholder="输入消息...">
    <button onclick="sendMessage()">发送</button>
    
    <script>
        const ws = new WebSocket('ws://localhost:8080/ws');
        const messages = document.getElementById('messages');
        
        ws.onopen = function(event) {
            console.log('WebSocket 连接已建立');
            addMessage('系统', '连接已建立');
        };
        
        ws.onmessage = function(event) {
            console.log('收到消息:', event.data);
            addMessage('服务器', event.data);
        };
        
        ws.onclose = function(event) {
            console.log('WebSocket 连接已关闭');
            addMessage('系统', '连接已关闭');
        };
        
        ws.onerror = function(error) {
            console.error('WebSocket 错误:', error);
            addMessage('系统', '连接错误');
        };
        
        function sendMessage() {
            const input = document.getElementById('messageInput');
            const message = input.value;
            if (message) {
                ws.send(message);
                addMessage('客户端', message);
                input.value = '';
            }
        }
        
        function addMessage(sender, message) {
            const div = document.createElement('div');
            div.textContent = `[${sender}] ${message}`;
            messages.appendChild(div);
        }
        
        // 回车发送消息
        document.getElementById('messageInput').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });
    </script>
</body>
</html>
```

## 配置选项

### WebSocket 配置

```go
type Config struct {
    // 检查请求来源
    CheckOrigin func(*http.Request) bool
    
    // 子协议选择
    Subprotocols []string
    
    // 读写缓冲区大小
    ReadBufferSize  int
    WriteBufferSize int
    
    // 握手超时时间
    HandshakeTimeout time.Duration
    
    // 压缩支持
    EnableCompression bool
    
    // 心跳配置
    PingPeriod  time.Duration // 发送 ping 的间隔
    PongWait    time.Duration // 等待 pong 的超时时间
    WriteWait   time.Duration // 写入超时时间
    
    // 最大消息大小
    MaxMessageSize int64
    
    // 连接限制
    MaxConnections int
}
```

### 配置示例

```go
config := &websocket.Config{
    CheckOrigin: func(r *http.Request) bool {
        // 检查请求来源
        origin := r.Header.Get("Origin")
        return origin == "https://example.com" || origin == "https://app.example.com"
    },
    
    Subprotocols: []string{"chat", "echo"},
    
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    
    HandshakeTimeout: 10 * time.Second,
    
    EnableCompression: true,
    
    // 心跳配置
    PingPeriod: 54 * time.Second,
    PongWait:   60 * time.Second,
    WriteWait:  10 * time.Second,
    
    MaxMessageSize: 512 * 1024, // 512KB
    MaxConnections: 10000,
}
```

## 消息处理

### 处理器接口

```go
type Handler interface {
    OnConnect(conn Connection)
    OnMessage(conn Connection, messageType int, data []byte)
    OnDisconnect(conn Connection)
}
```

### 消息类型

```go
const (
    TextMessage   = 1 // 文本消息
    BinaryMessage = 2 // 二进制消息
    CloseMessage  = 8 // 关闭消息
    PingMessage   = 9 // Ping 消息
    PongMessage   = 10 // Pong 消息
)
```

### 高级消息处理

```go
type ChatHandler struct {
    hub     *websocket.Hub
    logger  *log.Logger
    metrics *metrics.Registry
}

func (h *ChatHandler) OnConnect(conn websocket.Connection) {
    h.logger.Printf("用户连接: %s", conn.RemoteAddr())
    h.metrics.Counter("websocket_connections_total").Inc()
    
    // 设置连接属性
    conn.SetAttribute("user_id", generateUserID())
    conn.SetAttribute("join_time", time.Now())
    
    // 加入默认房间
    h.hub.JoinRoom(conn, "general")
    
    // 发送欢迎消息
    welcome := map[string]interface{}{
        "type":    "welcome",
        "message": "欢迎加入聊天室！",
        "user_id": conn.GetAttribute("user_id"),
    }
    
    h.sendJSON(conn, welcome)
}

func (h *ChatHandler) OnMessage(conn websocket.Connection, messageType int, data []byte) {
    h.metrics.Counter("websocket_messages_total").Inc()
    
    switch messageType {
    case websocket.TextMessage:
        h.handleTextMessage(conn, data)
    case websocket.BinaryMessage:
        h.handleBinaryMessage(conn, data)
    case websocket.PingMessage:
        conn.WriteMessage(websocket.PongMessage, nil)
    }
}

func (h *ChatHandler) handleTextMessage(conn websocket.Connection, data []byte) {
    var msg map[string]interface{}
    if err := json.Unmarshal(data, &msg); err != nil {
        h.sendError(conn, "Invalid JSON format")
        return
    }
    
    msgType, ok := msg["type"].(string)
    if !ok {
        h.sendError(conn, "Missing message type")
        return
    }
    
    switch msgType {
    case "chat":
        h.handleChatMessage(conn, msg)
    case "join_room":
        h.handleJoinRoom(conn, msg)
    case "leave_room":
        h.handleLeaveRoom(conn, msg)
    case "private_message":
        h.handlePrivateMessage(conn, msg)
    default:
        h.sendError(conn, "Unknown message type")
    }
}

func (h *ChatHandler) handleChatMessage(conn websocket.Connection, msg map[string]interface{}) {
    content, ok := msg["content"].(string)
    if !ok || content == "" {
        h.sendError(conn, "Message content is required")
        return
    }
    
    room, ok := msg["room"].(string)
    if !ok {
        room = "general"
    }
    
    // 构建广播消息
    broadcastMsg := map[string]interface{}{
        "type":      "chat",
        "content":   content,
        "user_id":   conn.GetAttribute("user_id"),
        "room":      room,
        "timestamp": time.Now().Unix(),
    }
    
    // 广播到房间
    h.hub.BroadcastToRoom(room, broadcastMsg)
}

func (h *ChatHandler) OnDisconnect(conn websocket.Connection) {
    userID := conn.GetAttribute("user_id")
    joinTime := conn.GetAttribute("join_time").(time.Time)
    duration := time.Since(joinTime)
    
    h.logger.Printf("用户断开: %s, 在线时长: %v", userID, duration)
    h.metrics.Counter("websocket_disconnections_total").Inc()
    h.metrics.Histogram("websocket_connection_duration").Observe(duration.Seconds())
    
    // 从所有房间移除
    h.hub.LeaveAllRooms(conn)
}

func (h *ChatHandler) sendJSON(conn websocket.Connection, data interface{}) {
    jsonData, err := json.Marshal(data)
    if err != nil {
        h.logger.Printf("JSON 序列化失败: %v", err)
        return
    }
    
    if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
        h.logger.Printf("发送消息失败: %v", err)
    }
}

func (h *ChatHandler) sendError(conn websocket.Connection, message string) {
    errorMsg := map[string]interface{}{
        "type":    "error",
        "message": message,
    }
    h.sendJSON(conn, errorMsg)
}
```

## 房间管理

### Hub 中心

```go
type Hub struct {
    // 房间管理
    rooms map[string]*Room
    
    // 连接管理
    connections map[Connection]bool
    
    // 注册和注销通道
    register   chan Connection
    unregister chan Connection
    
    // 广播通道
    broadcast chan *BroadcastMessage
    
    mutex sync.RWMutex
}

type Room struct {
    Name        string
    Connections map[Connection]bool
    Created     time.Time
    mutex       sync.RWMutex
}

type BroadcastMessage struct {
    Room    string
    Data    interface{}
    Exclude Connection // 排除的连接
}
```

### Hub 实现

```go
func NewHub() *Hub {
    return &Hub{
        rooms:       make(map[string]*Room),
        connections: make(map[Connection]bool),
        register:    make(chan Connection),
        unregister:  make(chan Connection),
        broadcast:   make(chan *BroadcastMessage),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case conn := <-h.register:
            h.registerConnection(conn)
            
        case conn := <-h.unregister:
            h.unregisterConnection(conn)
            
        case message := <-h.broadcast:
            h.broadcastMessage(message)
        }
    }
}

func (h *Hub) JoinRoom(conn Connection, roomName string) {
    h.mutex.Lock()
    defer h.mutex.Unlock()
    
    room, exists := h.rooms[roomName]
    if !exists {
        room = &Room{
            Name:        roomName,
            Connections: make(map[Connection]bool),
            Created:     time.Now(),
        }
        h.rooms[roomName] = room
    }
    
    room.mutex.Lock()
    room.Connections[conn] = true
    room.mutex.Unlock()
    
    // 设置连接的房间信息
    rooms := conn.GetAttribute("rooms")
    if rooms == nil {
        rooms = make(map[string]bool)
    }
    rooms.(map[string]bool)[roomName] = true
    conn.SetAttribute("rooms", rooms)
}

func (h *Hub) LeaveRoom(conn Connection, roomName string) {
    h.mutex.Lock()
    defer h.mutex.Unlock()
    
    room, exists := h.rooms[roomName]
    if !exists {
        return
    }
    
    room.mutex.Lock()
    delete(room.Connections, conn)
    isEmpty := len(room.Connections) == 0
    room.mutex.Unlock()
    
    // 如果房间为空，删除房间
    if isEmpty {
        delete(h.rooms, roomName)
    }
    
    // 更新连接的房间信息
    rooms := conn.GetAttribute("rooms")
    if rooms != nil {
        delete(rooms.(map[string]bool), roomName)
    }
}

func (h *Hub) BroadcastToRoom(roomName string, data interface{}) {
    h.broadcast <- &BroadcastMessage{
        Room: roomName,
        Data: data,
    }
}

func (h *Hub) BroadcastToAll(data interface{}) {
    h.broadcast <- &BroadcastMessage{
        Room: "", // 空字符串表示广播给所有连接
        Data: data,
    }
}
```

## 中间件支持

### 中间件接口

```go
type Middleware interface {
    Process(conn Connection, messageType int, data []byte, next MiddlewareFunc) error
}

type MiddlewareFunc func(conn Connection, messageType int, data []byte) error
```

### 认证中间件

```go
type AuthMiddleware struct {
    tokenValidator TokenValidator
}

func (m *AuthMiddleware) Process(conn websocket.Connection, messageType int, data []byte, next websocket.MiddlewareFunc) error {
    // 检查连接是否已认证
    if !conn.GetAttribute("authenticated").(bool) {
        if messageType != websocket.TextMessage {
            return errors.New("authentication required")
        }
        
        var authMsg map[string]interface{}
        if err := json.Unmarshal(data, &authMsg); err != nil {
            return errors.New("invalid message format")
        }
        
        if authMsg["type"] != "auth" {
            return errors.New("authentication required")
        }
        
        token, ok := authMsg["token"].(string)
        if !ok || !m.tokenValidator.Validate(token) {
            return errors.New("invalid token")
        }
        
        conn.SetAttribute("authenticated", true)
        conn.SetAttribute("user_id", getUserIDFromToken(token))
        
        // 发送认证成功消息
        authResponse := map[string]interface{}{
            "type":    "auth_success",
            "user_id": conn.GetAttribute("user_id"),
        }
        
        jsonData, _ := json.Marshal(authResponse)
        conn.WriteMessage(websocket.TextMessage, jsonData)
        
        return nil // 不继续处理认证消息
    }
    
    return next(conn, messageType, data)
}
```

### 限流中间件

```go
type RateLimitMiddleware struct {
    limiters map[string]*rate.Limiter
    mutex    sync.RWMutex
}

func (m *RateLimitMiddleware) Process(conn websocket.Connection, messageType int, data []byte, next websocket.MiddlewareFunc) error {
    userID := conn.GetAttribute("user_id").(string)
    
    m.mutex.RLock()
    limiter, exists := m.limiters[userID]
    m.mutex.RUnlock()
    
    if !exists {
        m.mutex.Lock()
        limiter = rate.NewLimiter(10, 5) // 10 QPS, 突发 5
        m.limiters[userID] = limiter
        m.mutex.Unlock()
    }
    
    if !limiter.Allow() {
        errorMsg := map[string]interface{}{
            "type":    "error",
            "message": "Rate limit exceeded",
        }
        
        jsonData, _ := json.Marshal(errorMsg)
        conn.WriteMessage(websocket.TextMessage, jsonData)
        
        return errors.New("rate limit exceeded")
    }
    
    return next(conn, messageType, data)
}
```

## 心跳机制

### 自动心跳

```go
type Connection struct {
    conn        *gorilla.Conn
    pingTicker  *time.Ticker
    pongWait    time.Duration
    writeWait   time.Duration
    pingPeriod  time.Duration
    
    // 其他字段...
}

func (c *Connection) startHeartbeat() {
    c.pingTicker = time.NewTicker(c.pingPeriod)
    
    go func() {
        defer c.pingTicker.Stop()
        
        for {
            select {
            case <-c.pingTicker.C:
                c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
                if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                    log.Printf("发送 ping 失败: %v", err)
                    return
                }
                
            case <-c.done:
                return
            }
        }
    }()
    
    // 设置 pong 处理器
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
        return nil
    })
}
```

### 手动心跳检测

```go
func (h *HeartbeatHandler) OnMessage(conn websocket.Connection, messageType int, data []byte) {
    if messageType == websocket.PingMessage {
        // 响应 pong
        conn.WriteMessage(websocket.PongMessage, data)
        return
    }
    
    if messageType == websocket.TextMessage {
        var msg map[string]interface{}
        json.Unmarshal(data, &msg)
        
        if msg["type"] == "ping" {
            response := map[string]interface{}{
                "type":      "pong",
                "timestamp": time.Now().Unix(),
            }
            
            jsonData, _ := json.Marshal(response)
            conn.WriteMessage(websocket.TextMessage, jsonData)
            return
        }
    }
    
    // 处理其他消息...
}
```

## 安全考虑

### CORS 配置

```go
config := &websocket.Config{
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        
        // 允许的域名列表
        allowedOrigins := []string{
            "https://example.com",
            "https://app.example.com",
            "https://admin.example.com",
        }
        
        for _, allowed := range allowedOrigins {
            if origin == allowed {
                return true
            }
        }
        
        return false
    },
}
```

### 输入验证

```go
func validateMessage(data []byte) error {
    // 检查消息大小
    if len(data) > 10*1024 { // 10KB 限制
        return errors.New("message too large")
    }
    
    // 检查 JSON 格式
    var msg map[string]interface{}
    if err := json.Unmarshal(data, &msg); err != nil {
        return errors.New("invalid JSON format")
    }
    
    // 检查必需字段
    if _, ok := msg["type"]; !ok {
        return errors.New("missing message type")
    }
    
    // 检查消息类型
    msgType, ok := msg["type"].(string)
    if !ok {
        return errors.New("invalid message type")
    }
    
    allowedTypes := []string{"chat", "join_room", "leave_room", "ping"}
    for _, allowed := range allowedTypes {
        if msgType == allowed {
            return nil
        }
    }
    
    return errors.New("unknown message type")
}
```

### XSS 防护

```go
import "html"

func sanitizeMessage(content string) string {
    // HTML 转义
    content = html.EscapeString(content)
    
    // 长度限制
    if len(content) > 1000 {
        content = content[:1000] + "..."
    }
    
    return content
}
```

## 性能优化

### 连接池

```go
type ConnectionPool struct {
    connections chan *Connection
    factory     func() (*Connection, error)
    maxSize     int
    currentSize int
    mutex       sync.Mutex
}

func (p *ConnectionPool) Get() (*Connection, error) {
    select {
    case conn := <-p.connections:
        return conn, nil
    default:
        return p.factory()
    }
}

func (p *ConnectionPool) Put(conn *Connection) {
    select {
    case p.connections <- conn:
    default:
        conn.Close()
    }
}
```

### 消息批处理

```go
type MessageBatcher struct {
    messages [][]byte
    timer    *time.Timer
    mutex    sync.Mutex
    
    batchSize int
    timeout   time.Duration
    handler   func([][]byte)
}

func (b *MessageBatcher) Add(data []byte) {
    b.mutex.Lock()
    defer b.mutex.Unlock()
    
    b.messages = append(b.messages, data)
    
    if len(b.messages) >= b.batchSize {
        b.flush()
    } else if b.timer == nil {
        b.timer = time.AfterFunc(b.timeout, func() {
            b.mutex.Lock()
            b.flush()
            b.mutex.Unlock()
        })
    }
}

func (b *MessageBatcher) flush() {
    if len(b.messages) > 0 {
        b.handler(b.messages)
        b.messages = b.messages[:0]
    }
    
    if b.timer != nil {
        b.timer.Stop()
        b.timer = nil
    }
}
```

## 监控和指标

### 内置指标

```go
type WebSocketMetrics struct {
    ConnectionsTotal     prometheus.Counter
    ConnectionsActive    prometheus.Gauge
    MessagesTotal        prometheus.Counter
    MessagesSent         prometheus.Counter
    MessagesReceived     prometheus.Counter
    MessageSize          prometheus.Histogram
    ConnectionDuration   prometheus.Histogram
    ErrorsTotal          prometheus.Counter
    RoomsTotal           prometheus.Gauge
    RoomMembers          prometheus.GaugeVec
}
```

### 启用监控

```go
func main() {
    // 创建指标注册表
    registry := prometheus.NewRegistry()
    
    // 创建 WebSocket 服务器
    ws := websocket.NewServer(config)
    ws.EnableMetrics(registry)
    
    // 启动指标服务器
    go func() {
        http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
        log.Fatal(http.ListenAndServe(":9090", nil))
    }()
    
    // 启动 WebSocket 服务器
    http.HandleFunc("/ws", ws.HandleWebSocket)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## 完整示例：聊天室

### 服务器端

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "time"
    
    "github.com/netcore-go/pkg/websocket"
)

type ChatServer struct {
    hub    *websocket.Hub
    server *websocket.Server
}

func NewChatServer() *ChatServer {
    hub := websocket.NewHub()
    
    config := &websocket.Config{
        CheckOrigin: func(r *http.Request) bool {
            return true // 生产环境需要严格检查
        },
        PingPeriod: 54 * time.Second,
        PongWait:   60 * time.Second,
        WriteWait:  10 * time.Second,
    }
    
    server := websocket.NewServer(config)
    server.SetHandler(&ChatHandler{hub: hub})
    
    return &ChatServer{
        hub:    hub,
        server: server,
    }
}

func (s *ChatServer) Start() {
    // 启动 Hub
    go s.hub.Run()
    
    // 注册路由
    http.HandleFunc("/ws", s.server.HandleWebSocket)
    http.HandleFunc("/", s.serveHome)
    
    // 启动服务器
    log.Println("聊天服务器启动在 :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *ChatServer) serveHome(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "index.html")
}

type ChatHandler struct {
    hub *websocket.Hub
}

func (h *ChatHandler) OnConnect(conn websocket.Connection) {
    log.Printf("用户连接: %s", conn.RemoteAddr())
    
    // 注册连接
    h.hub.Register(conn)
    
    // 加入默认房间
    h.hub.JoinRoom(conn, "general")
    
    // 发送欢迎消息
    welcome := Message{
        Type:      "system",
        Content:   "欢迎加入聊天室！",
        Timestamp: time.Now(),
    }
    
    h.sendMessage(conn, welcome)
}

func (h *ChatHandler) OnMessage(conn websocket.Connection, messageType int, data []byte) {
    if messageType != websocket.TextMessage {
        return
    }
    
    var msg Message
    if err := json.Unmarshal(data, &msg); err != nil {
        log.Printf("解析消息失败: %v", err)
        return
    }
    
    msg.Timestamp = time.Now()
    
    switch msg.Type {
    case "chat":
        h.handleChatMessage(conn, msg)
    case "join_room":
        h.handleJoinRoom(conn, msg)
    case "leave_room":
        h.handleLeaveRoom(conn, msg)
    }
}

func (h *ChatHandler) OnDisconnect(conn websocket.Connection) {
    log.Printf("用户断开: %s", conn.RemoteAddr())
    
    // 注销连接
    h.hub.Unregister(conn)
}

type Message struct {
    Type      string    `json:"type"`
    Content   string    `json:"content"`
    Room      string    `json:"room,omitempty"`
    UserID    string    `json:"user_id,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

func (h *ChatHandler) handleChatMessage(conn websocket.Connection, msg Message) {
    if msg.Room == "" {
        msg.Room = "general"
    }
    
    msg.UserID = conn.RemoteAddr().String() // 简单的用户标识
    
    // 广播到房间
    h.hub.BroadcastToRoom(msg.Room, msg)
}

func (h *ChatHandler) sendMessage(conn websocket.Connection, msg Message) {
    data, err := json.Marshal(msg)
    if err != nil {
        log.Printf("序列化消息失败: %v", err)
        return
    }
    
    if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
        log.Printf("发送消息失败: %v", err)
    }
}

func main() {
    server := NewChatServer()
    server.Start()
}
```

## 最佳实践

### 1. 连接管理

```go
// 设置合理的超时时间
config := &websocket.Config{
    HandshakeTimeout: 10 * time.Second,
    PingPeriod:       54 * time.Second,
    PongWait:         60 * time.Second,
    WriteWait:        10 * time.Second,
}

// 限制连接数
config.MaxConnections = 10000

// 限制消息大小
config.MaxMessageSize = 512 * 1024 // 512KB
```

### 2. 错误处理

```go
func (h *Handler) OnMessage(conn websocket.Connection, messageType int, data []byte) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("处理消息时发生 panic: %v", r)
            conn.Close()
        }
    }()
    
    // 消息处理逻辑
}
```

### 3. 资源清理

```go
func (h *Handler) OnDisconnect(conn websocket.Connection) {
    // 清理用户相关资源
    userID := conn.GetAttribute("user_id")
    if userID != nil {
        h.userManager.RemoveUser(userID.(string))
    }
    
    // 从所有房间移除
    h.hub.LeaveAllRooms(conn)
    
    // 关闭相关通道
    if ch := conn.GetAttribute("message_channel"); ch != nil {
        close(ch.(chan []byte))
    }
}
```

### 4. 安全检查

```go
// 严格的来源检查
config.CheckOrigin = func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    return isAllowedOrigin(origin)
}

// 输入验证
func validateInput(data []byte) error {
    if len(data) > maxMessageSize {
        return errors.New("message too large")
    }
    
    // 其他验证逻辑
    return nil
}
```

## 故障排除

### 常见问题

1. **连接建立失败**
   - 检查 CORS 设置
   - 验证 WebSocket URL
   - 确认防火墙配置

2. **消息丢失**
   - 检查缓冲区大小
   - 验证心跳配置
   - 确认错误处理逻辑

3. **内存泄漏**
   - 确保连接正确关闭
   - 检查房间清理逻辑
   - 验证协程退出

### 调试工具

```go
// 启用调试日志
ws.SetDebug(true)

// 添加调试中间件
ws.Use(&DebugMiddleware{})

// 监控连接状态
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        log.Printf("活跃连接数: %d", ws.GetActiveConnections())
        log.Printf("房间数: %d", hub.GetRoomCount())
    }
}()
```

## 参考资料

- [WebSocket RFC 6455](https://tools.ietf.org/html/rfc6455)
- [Gorilla WebSocket](https://github.com/gorilla/websocket)
- [WebSocket 安全指南](https://owasp.org/www-community/attacks/WebSocket_security)
- [实时应用最佳实践](./real