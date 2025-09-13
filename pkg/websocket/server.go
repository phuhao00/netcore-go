// Package websocket WebSocket服务器实现
// Author: NetCore-Go Team
// Created: 2024

package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

const (
	// WebSocket魔法字符串
	webSocketMagicString = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	
	// WebSocket操作码
	OpCodeContinuation = 0x0
	OpCodeText         = 0x1
	OpCodeBinary       = 0x2
	OpCodeClose        = 0x8
	OpCodePing         = 0x9
	OpCodePong         = 0xa
)

// Server WebSocket服务器
type Server struct {
	mu          sync.RWMutex
	listener    net.Listener
	connections map[string]*Connection
	handler     core.MessageHandler
	middlewares []core.Middleware
	config      *core.ServerConfig
	stats       *core.ServerStats
	running     bool
}

// NewServer 创建WebSocket服务器
func NewServer(opts ...core.ServerOption) *Server {
	config := &core.ServerConfig{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		MaxConnections:    1000,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       5 * time.Minute,
		EnableHeartbeat:   true,
		HeartbeatInterval: 30 * time.Second,
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(config)
	}

	server := &Server{
		connections: make(map[string]*Connection),
		config:      config,
		stats: &core.ServerStats{
			StartTime: time.Now(),
		},
	}

	// 初始化资源池
	server.initResourcePools()

	return server
}

// Start 启动服务器
func (s *Server) Start(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.running = true

	// 启动接受连接的goroutine
	go s.acceptConnections()

	// 启动心跳检测
	if s.config.EnableHeartbeat {
		go s.heartbeatChecker()
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("server is not running")
	}

	s.running = false

	// 关闭监听器
	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有连接
	for _, conn := range s.connections {
		conn.Close()
	}

	// 清空连接映射
	s.connections = make(map[string]*Connection)

	return nil
}

// SetHandler 设置消息处理器
func (s *Server) SetHandler(handler core.MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// SetMiddleware 设置中间件
func (s *Server) SetMiddleware(middleware ...core.Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = middleware
}

// GetStats 获取服务器统计信息
func (s *Server) GetStats() *core.ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := *s.stats
	stats.ActiveConnections = int64(len(s.connections))
	stats.Uptime = int64(time.Since(s.stats.StartTime).Seconds())
	return &stats
}

// acceptConnections 接受新连接
func (s *Server) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				s.updateStats(func(stats *core.ServerStats) {
					stats.ErrorCount++
					stats.LastError = err.Error()
				})
			}
			continue
		}

		// 检查连接数限制
		s.mu.RLock()
		if len(s.connections) >= s.config.MaxConnections {
			s.mu.RUnlock()
			conn.Close()
			continue
		}
		s.mu.RUnlock()

		// 处理WebSocket握手
		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *Server) handleConnection(netConn net.Conn) {
	defer netConn.Close()

	// 执行WebSocket握手
	if err := s.performHandshake(netConn); err != nil {
		s.updateStats(func(stats *core.ServerStats) {
			stats.ErrorCount++
			stats.LastError = err.Error()
		})
		return
	}

	// 创建WebSocket连接
	conn := NewConnection(netConn, s.config)

	// 添加到连接映射
	s.mu.Lock()
	s.connections[conn.ID()] = conn
	s.mu.Unlock()

	// 更新统计信息
	s.updateStats(func(stats *core.ServerStats) {
		stats.TotalConnections++
	})

	// 调用连接处理器
	if s.handler != nil {
		s.handler.OnConnect(conn)
	}

	// 开始消息循环
	s.messageLoop(conn)

	// 连接关闭后清理
	s.mu.Lock()
	delete(s.connections, conn.ID())
	s.mu.Unlock()

	// 调用断开连接处理器
	if s.handler != nil {
		s.handler.OnDisconnect(conn, nil)
	}
}

// performHandshake 执行WebSocket握手
func (s *Server) performHandshake(conn net.Conn) error {
	// 读取HTTP请求
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read handshake request: %w", err)
	}

	request := string(buffer[:n])
	lines := strings.Split(request, "\r\n")

	// 解析请求头
	headers := make(map[string]string)
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			headers[strings.ToLower(parts[0])] = parts[1]
		}
	}

	// 验证WebSocket请求
	if !s.validateHandshake(headers) {
		return fmt.Errorf("invalid WebSocket handshake")
	}

	// 生成响应
	response := s.generateHandshakeResponse(headers["sec-websocket-key"])

	// 发送响应
	_, err = conn.Write([]byte(response))
	if err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}

	return nil
}

// validateHandshake 验证WebSocket握手
func (s *Server) validateHandshake(headers map[string]string) bool {
	// 检查必需的头部
	if headers["upgrade"] != "websocket" {
		return false
	}
	if !strings.Contains(strings.ToLower(headers["connection"]), "upgrade") {
		return false
	}
	if headers["sec-websocket-version"] != "13" {
		return false
	}
	if headers["sec-websocket-key"] == "" {
		return false
	}

	return true
}

// generateHandshakeResponse 生成握手响应
func (s *Server) generateHandshakeResponse(key string) string {
	// 计算Sec-WebSocket-Accept
	h := sha1.New()
	h.Write([]byte(key + webSocketMagicString))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n"+
			"\r\n",
		accept,
	)
}

// messageLoop 消息循环
func (s *Server) messageLoop(conn *Connection) {
	for conn.IsActive() {
		msg, err := conn.ReadMessage()
		if err != nil {
			if conn.IsActive() {
				s.updateStats(func(stats *core.ServerStats) {
					stats.ErrorCount++
					stats.LastError = err.Error()
				})
			}
			break
		}

		// 更新统计信息
		s.updateStats(func(stats *core.ServerStats) {
			stats.MessagesReceived++
			stats.BytesReceived += int64(len(msg.Data))
		})

		// 处理控制帧
		if s.handleControlFrame(conn, msg) {
			continue
		}

		// 调用消息处理器
		if s.handler != nil {
			s.handler.OnMessage(conn, msg)
		}
	}
}

// handleControlFrame 处理控制帧
func (s *Server) handleControlFrame(conn *Connection, msg core.Message) bool {
	switch msg.Type {
	case core.MessageTypePing:
		// 响应Pong
		pongMsg := core.Message{
			Type:      core.MessageTypePong,
			Data:      msg.Data,
			Timestamp: time.Now(),
		}
		conn.SendMessage(pongMsg)
		return true
	case core.MessageTypePong:
		// 更新最后活跃时间
		conn.UpdateLastActive()
		return true
	case core.MessageTypeClose:
		// 关闭连接
		conn.Close()
		return true
	}
	return false
}

// heartbeatChecker 心跳检测
func (s *Server) heartbeatChecker() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for s.running {
		select {
		case <-ticker.C:
			s.checkHeartbeat()
		}
	}
}

// checkHeartbeat 检查心跳
func (s *Server) checkHeartbeat() {
	s.mu.RLock()
	connections := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.RUnlock()

	now := time.Now()
	for _, conn := range connections {
		if now.Sub(conn.LastActive()) > s.config.IdleTimeout {
			// 发送ping帧
			pingMsg := core.Message{
				Type:      core.MessageTypePing,
				Data:      []byte("ping"),
				Timestamp: now,
			}
			if err := conn.SendMessage(pingMsg); err != nil {
				// 连接已断开，关闭它
				conn.Close()
			}
		}
	}
}

// updateStats 更新统计信息
func (s *Server) updateStats(fn func(*core.ServerStats)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.stats)
}

// Broadcast 广播消息到所有连接
func (s *Server) Broadcast(msg core.Message) {
	s.mu.RLock()
	connections := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.RUnlock()

	for _, conn := range connections {
		if conn.IsActive() {
			conn.SendMessage(msg)
		}
	}

	// 更新统计信息
	s.updateStats(func(stats *core.ServerStats) {
		stats.MessagesSent += int64(len(connections))
		stats.BytesSent += int64(len(msg.Data) * len(connections))
	})
}

// initResourcePools 初始化资源池
func (s *Server) initResourcePools() {
	// 初始化连接池
	s.connections = make(map[string]*Connection, s.config.MaxConnections)
	
	// 初始化消息缓冲池
	// 这里可以使用sync.Pool来复用消息对象，减少GC压力
	messagePool := &sync.Pool{
		New: func() interface{} {
			return &core.Message{}
		},
	}
	
	// 初始化字节缓冲池
	bufferPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096)
		},
	}
	
	// 将资源池存储到服务器配置中（如果需要的话）
	// 这里只是示例，实际使用时可以根据需要调整
	_ = messagePool
	_ = bufferPool
	
	// 初始化工作协程池（用于处理消息）
	workerPoolSize := s.config.MaxConnections / 10
	if workerPoolSize < 10 {
		workerPoolSize = 10
	}
	if workerPoolSize > 100 {
		workerPoolSize = 100
	}
	
	// 启动工作协程池
	for i := 0; i < workerPoolSize; i++ {
		go s.messageWorker(i)
	}
}

// messageWorker 消息处理工作协程
func (s *Server) messageWorker(workerID int) {
	// 这是一个简化的工作协程实现
	// 实际使用时可以通过channel来分发任务
	for s.running {
		// 模拟工作协程等待任务
		time.Sleep(100 * time.Millisecond)
		
		// 这里可以处理来自消息队列的任务
		// 例如：处理广播消息、批量发送等
	}
}

// GetConnectionCount 获取当前连接数
func (s *Server) GetConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}

// GetConnection 根据ID获取连接
func (s *Server) GetConnection(id string) *Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connections[id]
}

// CloseConnection 关闭指定连接
func (s *Server) CloseConnection(id string) error {
	s.mu.RLock()
	conn, exists := s.connections[id]
	s.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("connection not found: %s", id)
	}
	
	conn.Close()
	return nil
}




