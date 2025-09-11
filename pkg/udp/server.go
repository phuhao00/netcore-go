// Package udp 实现NetCore-Go网络库的UDP服务器
// Author: NetCore-Go Team
// Created: 2024

package udp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
)

// UDPServer UDP服务器实现
type UDPServer struct {
	config      *core.ServerConfig
	handler     core.MessageHandler
	middlewares []core.Middleware
	stats       *core.ServerStats
	connections sync.Map // map[string]*UDPConnection 基于客户端地址的伪连接
	conn        *net.UDPConn
	ctx         context.Context
	cancel      context.CancelFunc
	running     int32
	wg          sync.WaitGroup
	mu          sync.RWMutex

	// UDP特有配置
	broadcastEnabled bool
	multicastGroups  []net.IP
}

// NewUDPServer 创建新的UDP服务器
func NewUDPServer(opts ...core.ServerOption) core.Server {
	config := core.DefaultServerConfig()
	for _, opt := range opts {
		opt(config)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &UDPServer{
		config: config,
		stats:  core.NewServerStats(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动UDP服务器
func (s *UDPServer) Start(addr string) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("server is already running")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		atomic.StoreInt32(&s.running, 0)
		return fmt.Errorf("failed to resolve UDP address %s: %w", addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		atomic.StoreInt32(&s.running, 0)
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.conn = conn
	s.stats.StartTime = time.Now()

	// 启用广播功能（如果配置了）
	if s.broadcastEnabled {
		if err := s.conn.SetWriteBuffer(s.config.WriteBufferSize); err != nil {
			return fmt.Errorf("failed to set write buffer: %w", err)
		}
	}

	// 启动数据包接收循环
	s.wg.Add(1)
	go s.receiveLoop()

	// 启动统计更新循环
	s.wg.Add(1)
	go s.statsUpdateLoop()

	// 启动心跳检测循环
	if s.config.EnableHeartbeat {
		s.wg.Add(1)
		go s.heartbeatLoop()
	}

	return nil
}

// Stop 停止UDP服务器
func (s *UDPServer) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return fmt.Errorf("server is not running")
	}

	// 取消上下文
	s.cancel()

	// 关闭UDP连接
	if s.conn != nil {
		s.conn.Close()
	}

	// 关闭所有伪连接
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*UDPConnection); ok {
			conn.Close()
		}
		return true
	})

	// 等待所有goroutine结束
	s.wg.Wait()

	return nil
}

// SetHandler 设置消息处理器
func (s *UDPServer) SetHandler(handler core.MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// SetMiddleware 设置中间件
func (s *UDPServer) SetMiddleware(middleware ...core.Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = middleware
}

// GetStats 获取服务器统计信息
func (s *UDPServer) GetStats() *core.ServerStats {
	s.stats.UpdateUptime()
	return s.stats
}

// receiveLoop UDP数据包接收循环
func (s *UDPServer) receiveLoop() {
	defer s.wg.Done()

	buffer := make([]byte, s.config.ReadBufferSize)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 设置读超时
		if s.config.ReadTimeout > 0 {
			s.conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		}

		n, clientAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				atomic.AddInt64(&s.stats.ErrorCount, 1)
				s.stats.LastError = err.Error()
				continue
			}
		}

		if n > 0 {
			// 更新统计信息
			atomic.AddInt64(&s.stats.MessagesReceived, 1)
			atomic.AddInt64(&s.stats.BytesReceived, int64(n))

			// 获取或创建伪连接
			udpConn := s.getOrCreateConnection(clientAddr)
			udpConn.UpdateLastActive()

			// 创建消息对象
			msg := core.NewMessage(core.MessageTypeBinary, buffer[:n])

			// 调用消息处理器
			s.mu.RLock()
			handler := s.handler
			s.mu.RUnlock()
			if handler != nil {
				handler.OnMessage(udpConn, *msg)
			}
		}
	}
}

// getOrCreateConnection 获取或创建UDP伪连接
func (s *UDPServer) getOrCreateConnection(addr *net.UDPAddr) *UDPConnection {
	addrStr := addr.String()
	if conn, ok := s.connections.Load(addrStr); ok {
		return conn.(*UDPConnection)
	}

	// 创建新的伪连接
	udpConn := NewUDPConnection(s.conn, addr)
	s.connections.Store(addrStr, udpConn)

	// 更新统计信息
	atomic.AddInt64(&s.stats.TotalConnections, 1)
	atomic.AddInt64(&s.stats.ActiveConnections, 1)

	// 调用连接建立处理器
	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()
	if handler != nil {
		handler.OnConnect(udpConn)
	}

	return udpConn
}

// statsUpdateLoop 统计信息更新循环
func (s *UDPServer) statsUpdateLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.stats.UpdateUptime()
		}
	}
}

// heartbeatLoop 心跳检测循环
func (s *UDPServer) heartbeatLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkIdleConnections()
		}
	}
}

// checkIdleConnections 检查空闲连接
func (s *UDPServer) checkIdleConnections() {
	now := time.Now()
	idleTimeout := s.config.IdleTimeout

	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*UDPConnection); ok {
			if now.Sub(conn.GetLastActive()) > idleTimeout {
				// 从连接池中移除
				s.connections.Delete(key)
				atomic.AddInt64(&s.stats.ActiveConnections, -1)
				
				// 调用断开连接处理器
				s.mu.RLock()
				handler := s.handler
				s.mu.RUnlock()
				if handler != nil {
					handler.OnDisconnect(conn, fmt.Errorf("connection idle timeout"))
				}
				
				conn.Close()
			}
		}
		return true
	})
}

// EnableBroadcast 启用广播功能
func (s *UDPServer) EnableBroadcast() error {
	s.broadcastEnabled = true
	if s.conn != nil {
		return s.conn.SetWriteBuffer(s.config.WriteBufferSize)
	}
	return nil
}

// JoinMulticastGroup 加入组播组
func (s *UDPServer) JoinMulticastGroup(groupAddr net.IP) error {
	s.multicastGroups = append(s.multicastGroups, groupAddr)
	// 实际的组播加入逻辑可以在这里实现
	return nil
}

// Broadcast 广播消息到所有连接
func (s *UDPServer) Broadcast(data []byte) error {
	var lastErr error
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*UDPConnection); ok {
			if err := conn.Send(data); err != nil {
				lastErr = err
			}
		}
		return true
	})
	return lastErr
}

// IsRunning 检查服务器是否正在运行
func (s *UDPServer) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// GetConfig 获取服务器配置
func (s *UDPServer) GetConfig() *core.ServerConfig {
	return s.config
}

