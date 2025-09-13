package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/netcore-go/pkg/config"
	"github.com/netcore-go/pkg/logger"
	"github.com/netcore-go/pkg/metrics"
)

// Message 消息结构
type Message struct {
	Type      string    `json:"type"`      // 消息类型
	UserID    string    `json:"user_id"`   // 用户ID
	Username  string    `json:"username"`  // 用户名
	RoomID    string    `json:"room_id"`   // 房间ID
	Content   string    `json:"content"`   // 消息内容
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// MessageType 消息类型常量
const (
	MsgTypeJoin    = "join"
	MsgTypeLeave   = "leave"
	MsgTypeChat    = "chat"
	MsgTypeSystem  = "system"
	MsgTypeUserList = "user_list"
)

// Client 客户端连接
type Client struct {
	ID       string
	Username string
	RoomID   string
	Conn     *websocket.Conn
	Send     chan *Message
	Hub      *Hub
	mu       sync.RWMutex
}

// NewClient 创建新客户端
func NewClient(id, username, roomID string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:       id,
		Username: username,
		RoomID:   roomID,
		Conn:     conn,
		Send:     make(chan *Message, 256),
		Hub:      hub,
	}
}

// ReadPump 读取消息
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
	
	// 设置读取参数
	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.WithError(err).Error("WebSocket读取错误")
			}
			break
		}
		
		// 设置消息元数据
		msg.UserID = c.ID
		msg.Username = c.Username
		msg.RoomID = c.RoomID
		msg.Timestamp = time.Now()
		
		// 发送到Hub处理
		c.Hub.Broadcast <- &msg
		
		// 更新指标
		metrics.DefaultRegistry.(*metrics.Registry)
	}
}

// WritePump 写入消息
func (c *Client) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			if err := c.Conn.WriteJSON(message); err != nil {
				logger.WithError(err).Error("WebSocket写入错误")
				return
			}
			
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Room 聊天室
type Room struct {
	ID      string
	Name    string
	Clients map[*Client]bool
	mu      sync.RWMutex
}

// NewRoom 创建新房间
func NewRoom(id, name string) *Room {
	return &Room{
		ID:      id,
		Name:    name,
		Clients: make(map[*Client]bool),
	}
}

// AddClient 添加客户端
func (r *Room) AddClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[client] = true
}

// RemoveClient 移除客户端
func (r *Room) RemoveClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Clients[client]; ok {
		delete(r.Clients, client)
		close(client.Send)
	}
}

// GetClients 获取客户端列表
func (r *Room) GetClients() []*Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	clients := make([]*Client, 0, len(r.Clients))
	for client := range r.Clients {
		clients = append(clients, client)
	}
	return clients
}

// GetUserList 获取用户列表
func (r *Room) GetUserList() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	usernames := make([]string, 0, len(r.Clients))
	for client := range r.Clients {
		usernames = append(usernames, client.Username)
	}
	return usernames
}

// Hub 消息中心
type Hub struct {
	Rooms      map[string]*Room
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *Message
	mu         sync.RWMutex
	
	// 指标
	connectedClients *metrics.Gauge
	messageCounter   *metrics.Counter
	roomCounter      *metrics.Gauge
}

// NewHub 创建消息中心
func NewHub() *Hub {
	return &Hub{
		Rooms:      make(map[string]*Room),
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *Message),
		
		// 初始化指标
		connectedClients: metrics.RegisterGauge(
			"chatroom_connected_clients",
			"Number of connected clients",
			metrics.Labels{},
		),
		messageCounter: metrics.RegisterCounter(
			"chatroom_messages_total",
			"Total number of messages",
			metrics.Labels{},
		),
		roomCounter: metrics.RegisterGauge(
			"chatroom_active_rooms",
			"Number of active rooms",
			metrics.Labels{},
		),
	}
}

// Run 运行消息中心
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.handleClientRegister(client)
			
		case client := <-h.Unregister:
			h.handleClientUnregister(client)
			
		case message := <-h.Broadcast:
			h.handleMessage(message)
		}
	}
}

// handleClientRegister 处理客户端注册
func (h *Hub) handleClientRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 添加客户端
	h.Clients[client] = true
	
	// 获取或创建房间
	room, exists := h.Rooms[client.RoomID]
	if !exists {
		room = NewRoom(client.RoomID, fmt.Sprintf("Room %s", client.RoomID))
		h.Rooms[client.RoomID] = room
	}
	
	// 添加到房间
	room.AddClient(client)
	
	// 更新指标
	h.connectedClients.Set(float64(len(h.Clients)))
	h.roomCounter.Set(float64(len(h.Rooms)))
	
	// 发送加入消息
	joinMsg := &Message{
		Type:      MsgTypeSystem,
		Content:   fmt.Sprintf("%s 加入了聊天室", client.Username),
		RoomID:    client.RoomID,
		Timestamp: time.Now(),
	}
	
	h.broadcastToRoom(client.RoomID, joinMsg)
	
	// 发送用户列表
	userListMsg := &Message{
		Type:      MsgTypeUserList,
		Content:   fmt.Sprintf("%v", room.GetUserList()),
		RoomID:    client.RoomID,
		Timestamp: time.Now(),
	}
	
	h.broadcastToRoom(client.RoomID, userListMsg)
	
	logger.WithFields(map[string]interface{}{
		"client_id": client.ID,
		"username":  client.Username,
		"room_id":   client.RoomID,
	}).Info("客户端连接")
}

// handleClientUnregister 处理客户端注销
func (h *Hub) handleClientUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if _, ok := h.Clients[client]; ok {
		// 从客户端列表移除
		delete(h.Clients, client)
		
		// 从房间移除
		if room, exists := h.Rooms[client.RoomID]; exists {
			room.RemoveClient(client)
			
			// 如果房间为空，删除房间
			if len(room.Clients) == 0 {
				delete(h.Rooms, client.RoomID)
			} else {
				// 发送离开消息
				leaveMsg := &Message{
					Type:      MsgTypeSystem,
					Content:   fmt.Sprintf("%s 离开了聊天室", client.Username),
					RoomID:    client.RoomID,
					Timestamp: time.Now(),
				}
				
				h.broadcastToRoom(client.RoomID, leaveMsg)
				
				// 更新用户列表
				userListMsg := &Message{
					Type:      MsgTypeUserList,
					Content:   fmt.Sprintf("%v", room.GetUserList()),
					RoomID:    client.RoomID,
					Timestamp: time.Now(),
				}
				
				h.broadcastToRoom(client.RoomID, userListMsg)
			}
		}
		
		// 更新指标
		h.connectedClients.Set(float64(len(h.Clients)))
		h.roomCounter.Set(float64(len(h.Rooms)))
		
		logger.WithFields(map[string]interface{}{
			"client_id": client.ID,
			"username":  client.Username,
			"room_id":   client.RoomID,
		}).Info("客户端断开")
	}
}

// handleMessage 处理消息
func (h *Hub) handleMessage(message *Message) {
	// 更新消息计数
	h.messageCounter.Inc()
	
	// 记录消息日志
	logger.WithFields(map[string]interface{}{
		"type":     message.Type,
		"user_id":  message.UserID,
		"username": message.Username,
		"room_id":  message.RoomID,
		"content":  message.Content,
	}).Info("收到消息")
	
	// 广播到房间
	h.broadcastToRoom(message.RoomID, message)
}

// broadcastToRoom 广播消息到房间
func (h *Hub) broadcastToRoom(roomID string, message *Message) {
	room, exists := h.Rooms[roomID]
	if !exists {
		return
	}
	
	for client := range room.Clients {
		select {
		case client.Send <- message:
		default:
			// 客户端发送缓冲区满，关闭连接
			room.RemoveClient(client)
			delete(h.Clients, client)
		}
	}
}

// GetStats 获取统计信息
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return map[string]interface{}{
		"connected_clients": len(h.Clients),
		"active_rooms":      len(h.Rooms),
		"total_messages":    h.messageCounter.Value(),
	}
}

// ChatServer 聊天服务器
type ChatServer struct {
	hub      *Hub
	upgrader websocket.Upgrader
	server   *http.Server
	config   *ServerConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int    `json:"port" yaml:"port"`
	Host         string `json:"host" yaml:"host"`
	StaticDir    string `json:"static_dir" yaml:"static_dir"`
	AllowOrigins string `json:"allow_origins" yaml:"allow_origins"`
}

// NewChatServer 创建聊天服务器
func NewChatServer(config *ServerConfig) *ChatServer {
	if config == nil {
		config = &ServerConfig{
			Port:         8080,
			Host:         "localhost",
			StaticDir:    "./static",
			AllowOrigins: "*",
		}
	}
	
	return &ChatServer{
		hub:    NewHub(),
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
	}
}

// Start 启动服务器
func (s *ChatServer) Start() error {
	// 启动Hub
	go s.hub.Run()
	
	// 设置路由
	mux := http.NewServeMux()
	
	// WebSocket端点
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// API端点
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/rooms", s.handleRooms)
	
	// 静态文件
	mux.Handle("/", http.FileServer(http.Dir(s.config.StaticDir)))
	
	// 指标端点
	mux.Handle("/metrics", metrics.PrometheusHandler(nil))
	
	// 创建服务器
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	logger.Infof("聊天服务器启动在 %s", addr)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *ChatServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	return s.server.Shutdown(ctx)
}

// handleWebSocket 处理WebSocket连接
func (s *ChatServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 获取参数
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")
	roomID := r.URL.Query().Get("room_id")
	
	if userID == "" || username == "" || roomID == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}
	
	// 升级到WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithError(err).Error("WebSocket升级失败")
		return
	}
	
	// 创建客户端
	client := NewClient(userID, username, roomID, conn, s.hub)
	
	// 注册客户端
	s.hub.Register <- client
	
	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}

// handleStats 处理统计信息请求
func (s *ChatServer) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	stats := s.hub.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// handleRooms 处理房间列表请求
func (s *ChatServer) handleRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	s.hub.mu.RLock()
	rooms := make([]map[string]interface{}, 0, len(s.hub.Rooms))
	for _, room := range s.hub.Rooms {
		rooms = append(rooms, map[string]interface{}{
			"id":          room.ID,
			"name":        room.Name,
			"client_count": len(room.Clients),
			"users":       room.GetUserList(),
		})
	}
	s.hub.mu.RUnlock()
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rooms": rooms,
	})
}

func main() {
	// 初始化日志
	logger.SetGlobalLogger(logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "json",
		Output:    "console",
	}))
	
	// 加载配置
	cfg, err := config.AutoLoadConfig()
	if err != nil {
		logger.WithError(err).Warn("加载配置失败，使用默认配置")
	}
	
	// 解析服务器配置
	var serverConfig ServerConfig
	if cfg != nil {
		if err := cfg.UnmarshalKey("server", &serverConfig); err != nil {
			logger.WithError(err).Warn("解析服务器配置失败，使用默认配置")
			serverConfig = ServerConfig{
				Port:         8080,
				Host:         "localhost",
				StaticDir:    "./static",
				AllowOrigins: "*",
			}
		}
	} else {
		serverConfig = ServerConfig{
			Port:         8080,
			Host:         "localhost",
			StaticDir:    "./static",
			AllowOrigins: "*",
		}
	}
	
	// 启动系统指标收集
	metrics.StartSystemCollector(&metrics.SystemCollectorConfig{
		Enabled:         true,
		CollectInterval: 10 * time.Second,
		Namespace:       "chatroom",
	})
	defer metrics.StopSystemCollector()
	
	// 启动Prometheus导出器
	go func() {
		if err := metrics.StartPrometheusExporter(":9090"); err != nil {
			logger.WithError(err).Error("Prometheus导出器启动失败")
		}
	}()
	defer metrics.StopPrometheusExporter()
	
	// 创建聊天服务器
	server := NewChatServer(&serverConfig)
	
	// 启动服务器
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("服务器启动失败")
		}
	}()
	
	logger.Info("聊天室服务器已启动")
	logger.Infof("WebSocket端点: ws://%s:%d/ws", serverConfig.Host, serverConfig.Port)
	logger.Infof("Web界面: http://%s:%d", serverConfig.Host, serverConfig.Port)
	logger.Info("指标端点: http://localhost:9090/metrics")
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	logger.Info("正在关闭服务器...")
	
	// 优雅关闭
	if err := server.Stop(); err != nil {
		logger.WithError(err).Error("服务器关闭失败")
	} else {
		logger.Info("服务器已关闭")
	}
}