// Package core 实现NetCore-Go网络库的基础服务器
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// BaseServer 基础服务器实现
type BaseServer struct {
	config      *ServerConfig
	handler     MessageHandler
	middlewares []Middleware
	stats       *ServerStats
	connections sync.Map // map[string]*BaseConnection
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	running     int32
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// NewBaseServer 创建新的基础服务器
func NewBaseServer(opts ...ServerOption) *BaseServer {
	config := DefaultServerConfig()
	for _, opt := range opts {
		opt(config)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &BaseServer{
		config: config,
		stats:  NewServerStats(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动服务器
func (s *BaseServer) Start(addr string) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("server is already running")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		atomic.StoreInt32(&s.running, 0)
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.stats.StartTime = time.Now()

	// 启动接受连接的goroutine
	s.wg.Add(1)
	go s.acceptLoop()

	// 启动统计更新goroutine
	s.wg.Add(1)
	go s.statsUpdateLoop()

	// 启动心跳检测goroutine
	if s.config.EnableHeartbeat {
		s.wg.Add(1)
		go s.heartbeatLoop()
	}

	return nil
}

// Stop 停止服务器
func (s *BaseServer) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return fmt.Errorf("server is not running")
	}

	// 取消上下文
	s.cancel()

	// 关闭监听器
	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有连接
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*BaseConnection); ok {
			conn.Close()
		}
		return true
	})

	// 等待所有goroutine结束
	s.wg.Wait()

	return nil
}

// SetHandler 设置消息处理器
func (s *BaseServer) SetHandler(handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// SetMiddleware 设置中间件
func (s *BaseServer) SetMiddleware(middleware ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = middleware
	// 按优先级排序中间件
	sort.Slice(s.middlewares, func(i, j int) bool {
		return s.middlewares[i].Priority() > s.middlewares[j].Priority()
	})
}

// GetStats 获取服务器统计信息
func (s *BaseServer) GetStats() *ServerStats {
	s.stats.UpdateUptime()
	return s.stats
}

// GetConfig 获取服务器配置
func (s *BaseServer) GetConfig() *ServerConfig {
	return s.config
}

// acceptLoop 接受连接循环
func (s *BaseServer) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				atomic.AddInt64(&s.stats.ErrorCount, 1)
				s.stats.LastError = err.Error()
				continue
			}
		}

		// 检查连接数限制
		if s.getConnectionCount() >= int64(s.config.MaxConnections) {
			conn.Close()
			continue
		}

		// 创建连接对象
		baseConn := NewBaseConnection(conn)
		s.connections.Store(baseConn.ID(), baseConn)

		// 更新统计信息
		atomic.AddInt64(&s.stats.TotalConnections, 1)
		atomic.AddInt64(&s.stats.ActiveConnections, 1)

		// 启动连接处理goroutine
		s.wg.Add(1)
		go s.handleConnection(baseConn)
	}
}

// handleConnection 处理单个连接
func (s *BaseServer) handleConnection(conn *BaseConnection) {
	defer s.wg.Done()
	defer func() {
		// 清理连接
		s.connections.Delete(conn.ID())
		atomic.AddInt64(&s.stats.ActiveConnections, -1)
		conn.Close()

		// 调用断开连接处理器
		s.mu.RLock()
		handler := s.handler
		s.mu.RUnlock()
		if handler != nil {
			handler.OnDisconnect(conn, nil)
		}
	}()

	// 调用连接建立处理器
	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()
	if handler != nil {
		handler.OnConnect(conn)
	}

	// 读取消息循环
	buffer := make([]byte, s.config.ReadBufferSize)
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-conn.CloseCh():
			return
		default:
		}

		// 设置读超时
		if s.config.ReadTimeout > 0 {
			conn.GetRawConn().SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		}

		n, err := conn.GetRawConn().Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		if n > 0 {
			// 更新统计信息
			atomic.AddInt64(&s.stats.MessagesReceived, 1)
			atomic.AddInt64(&s.stats.BytesReceived, int64(n))
			conn.UpdateLastActive()

			// 创建消息对象
			msg := NewMessage(MessageTypeBinary, buffer[:n])

			// 调用消息处理器
			if handler != nil {
				handler.OnMessage(conn, *msg)
			}
		}
	}
}

// statsUpdateLoop 统计信息更新循环
func (s *BaseServer) statsUpdateLoop() {
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
func (s *BaseServer) heartbeatLoop() {
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
func (s *BaseServer) checkIdleConnections() {
	now := time.Now()
	idleTimeout := s.config.IdleTimeout

	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*BaseConnection); ok {
			if now.Sub(conn.GetLastActive()) > idleTimeout {
				conn.Close()
			}
		}
		return true
	})
}

// getConnectionCount 获取当前连接数
func (s *BaseServer) getConnectionCount() int64 {
	var count int64
	s.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// IsRunning 检查服务器是否正在运行
func (s *BaseServer) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}