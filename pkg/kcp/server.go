// Package kcp KCP协议服务器实现
// Author: NetCore-Go Team
// Created: 2024

package kcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtaci/kcp-go/v5"

	"github.com/phuhao00/netcore-go/pkg/core"
	"github.com/phuhao00/netcore-go/pkg/pool"
)

// KCPServer KCP服务器实现
type KCPServer struct {
	core.BaseServer
	listener    *kcp.Listener
	connections map[string]*KCPConnection
	connMu      sync.RWMutex
	kcpConfig   *KCPConfig
}

// KCPConfig KCP配置
type KCPConfig struct {
	// KCP参数
	Nodelay    int  // 是否启用 nodelay模式，0不启用；1启用
	Interval   int  // 协议内部工作的 interval，单位毫秒
	Resend     int  // 快速重传模式，默认0关闭，可以设置2（2次ACK跨越将会直接重传）
	NC         int  // 是否关闭流控，默认是0代表不关闭，1代表关闭
	SndWnd     int  // 发送窗口大小，单位是包
	RcvWnd     int  // 接收窗口大小，单位是包
	MTU        int  // 最大传输单元
	DSCP       int  // DSCP标记
	AckNodelay bool // 是否立即发送ACK
	
	// 加密配置
	Key        []byte // 加密密钥
	Crypt      string // 加密算法："aes", "tea", "xor", "none"
	
	// 压缩配置
	DataShard   int // 数据分片数
	ParityShard int // 校验分片数
}

// DefaultKCPConfig 默认KCP配置
func DefaultKCPConfig() *KCPConfig {
	return &KCPConfig{
		Nodelay:     1,   // 启用nodelay
		Interval:    10,  // 10ms间隔
		Resend:      2,   // 快速重传
		NC:          1,   // 关闭流控
		SndWnd:      128, // 发送窗口
		RcvWnd:      128, // 接收窗口
		MTU:         1400,
		DSCP:        0,
		AckNodelay:  true,
		Crypt:       "aes",
		Key:         []byte("netcore-go-kcp-key-1234567890123456"), // 32字节密钥
		DataShard:   10,
		ParityShard: 3,
	}
}

// NewKCPServer 创建KCP服务器
func NewKCPServer(opts ...core.ServerOption) *KCPServer {
	server := &KCPServer{
		connections: make(map[string]*KCPConnection),
		kcpConfig:   DefaultKCPConfig(),
	}
	
	// 应用配置选项
	for _, opt := range opts {
		opt(&server.BaseServer)
	}
	
	// 设置默认配置
	if server.Config.ReadBufferSize == 0 {
		server.Config.ReadBufferSize = 4096
	}
	if server.Config.WriteBufferSize == 0 {
		server.Config.WriteBufferSize = 4096
	}
	if server.Config.MaxConnections == 0 {
		server.Config.MaxConnections = 1000
	}
	
	return server
}

// SetKCPConfig 设置KCP配置
func (s *KCPServer) SetKCPConfig(config *KCPConfig) {
	s.kcpConfig = config
}

// Start 启动KCP服务器
func (s *KCPServer) Start(addr string) error {
	// 创建KCP监听器
	var err error
	
	// 根据加密算法创建监听器
	switch s.kcpConfig.Crypt {
	case "aes":
		s.listener, err = kcp.ListenWithOptions(addr, nil, s.kcpConfig.DataShard, s.kcpConfig.ParityShard)
	case "none":
		s.listener, err = kcp.Listen(addr)
	default:
		s.listener, err = kcp.ListenWithOptions(addr, nil, s.kcpConfig.DataShard, s.kcpConfig.ParityShard)
	}
	
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	
	s.Listener = s.listener
	s.Running = true
	s.Stats.StartTime = time.Now()
	
	// 启动连接池
	if s.Config.EnableConnectionPool {
		s.ConnPool = pool.NewConnectionPool(pool.PoolConfig{
			InitialSize: 10,
			MaxSize:     s.Config.MaxConnections,
			IdleTimeout: 5 * time.Minute,
		})
	}
	
	// 启动内存池
	if s.Config.EnableMemoryPool {
		s.MemPool = pool.NewMemoryPool(s.Config.ReadBufferSize, 100)
	}
	
	// 启动协程池
	if s.Config.EnableGoroutinePool {
		s.GoroutinePool = pool.NewGoroutinePool(1000, 10000)
	}
	
	go s.acceptLoop()
	
	return nil
}

// acceptLoop 接受连接循环
func (s *KCPServer) acceptLoop() {
	for s.Running {
		conn, err := s.listener.AcceptKCP()
		if err != nil {
			if s.Running {
				s.Stats.ErrorCount++
				s.Stats.LastError = err.Error()
			}
			continue
		}
		
		// 检查连接数限制
		if s.Stats.ActiveConnections >= int64(s.Config.MaxConnections) {
			conn.Close()
			continue
		}
		
		// 配置KCP参数
		s.configureKCP(conn)
		
		// 创建KCP连接
		kcpConn := NewKCPConnection(conn, s)
		
		// 添加到连接管理
		s.connMu.Lock()
		s.connections[kcpConn.ID()] = kcpConn
		s.connMu.Unlock()
		
		// 使用协程池处理连接
		if s.GoroutinePool != nil {
			s.GoroutinePool.Submit(func() {
				s.handleConnection(kcpConn)
			})
		} else {
			go s.handleConnection(kcpConn)
		}
	}
}

// configureKCP 配置KCP参数
func (s *KCPServer) configureKCP(conn *kcp.UDPSession) {
	// 设置KCP参数
	conn.SetNoDelay(
		s.kcpConfig.Nodelay,
		s.kcpConfig.Interval,
		s.kcpConfig.Resend,
		s.kcpConfig.NC,
	)
	
	// 设置窗口大小
	conn.SetWindowSize(s.kcpConfig.SndWnd, s.kcpConfig.RcvWnd)
	
	// 设置MTU
	conn.SetMtu(s.kcpConfig.MTU)
	
	// 设置DSCP
	if s.kcpConfig.DSCP > 0 {
		conn.SetDSCP(s.kcpConfig.DSCP)
	}
	
	// 设置ACK立即发送
	if s.kcpConfig.AckNodelay {
		conn.SetACKNoDelay(true)
	}
	
	// 设置读写缓冲区
	conn.SetReadBuffer(s.Config.ReadBufferSize)
	conn.SetWriteBuffer(s.Config.WriteBufferSize)
	
	// 设置超时
	if s.Config.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(s.Config.ReadTimeout))
	}
	if s.Config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(s.Config.WriteTimeout))
	}
}

// handleConnection 处理KCP连接
func (s *KCPServer) handleConnection(conn *KCPConnection) {
	defer func() {
		if r := recover(); r != nil {
			s.Stats.ErrorCount++
			s.Stats.LastError = fmt.Sprintf("panic: %v", r)
		}
		
		// 从连接管理中移除
		s.connMu.Lock()
		delete(s.connections, conn.ID())
		s.connMu.Unlock()
		
		conn.Close()
	}()
	
	atomic.AddInt64(&s.Stats.ActiveConnections, 1)
	atomic.AddInt64(&s.Stats.TotalConnections, 1)
	defer atomic.AddInt64(&s.Stats.ActiveConnections, -1)
	
	// 调用连接处理器
	if s.Handler != nil {
		s.Handler.OnConnect(conn)
		defer s.Handler.OnDisconnect(conn, nil)
	}
	
	// 处理消息
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			if s.Handler != nil {
				s.Handler.OnDisconnect(conn, err)
			}
			break
		}
		
		atomic.AddInt64(&s.Stats.MessagesReceived, 1)
		atomic.AddInt64(&s.Stats.BytesReceived, int64(len(msg.Data)))
		
		// 处理消息
		if s.Handler != nil {
			s.Handler.OnMessage(conn, msg)
		}
	}
}

// Broadcast 广播消息到所有连接
func (s *KCPServer) Broadcast(msg core.Message) error {
	s.connMu.RLock()
	connections := make([]*KCPConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		if conn.IsActive() {
			connections = append(connections, conn)
		}
	}
	s.connMu.RUnlock()
	
	var errors []error
	for _, conn := range connections {
		if err := conn.SendMessage(msg); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("broadcast failed for %d connections", len(errors))
	}
	
	return nil
}

// GetConnection 获取指定连接
func (s *KCPServer) GetConnection(id string) *KCPConnection {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.connections[id]
}

// GetConnections 获取所有活跃连接
func (s *KCPServer) GetConnections() []*KCPConnection {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	
	connections := make([]*KCPConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		if conn.IsActive() {
			connections = append(connections, conn)
		}
	}
	
	return connections
}

// Stop 停止KCP服务器
func (s *KCPServer) Stop() error {
	s.Running = false
	
	// 关闭所有连接
	s.connMu.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.connections = make(map[string]*KCPConnection)
	s.connMu.Unlock()
	
	if s.listener != nil {
		return s.listener.Close()
	}
	
	return nil
}

// GetStats 获取服务器统计信息
func (s *KCPServer) GetStats() *core.ServerStats {
	return &s.Stats
}

// SetHandler 设置消息处理器
func (s *KCPServer) SetHandler(handler core.MessageHandler) {
	s.Handler = handler
}

// SetMiddleware 设置中间件
func (s *KCPServer) SetMiddleware(middleware ...core.Middleware) {
	s.Middlewares = middleware
}

// GetKCPStats 获取KCP特定统计信息
func (s *KCPServer) GetKCPStats() map[string]interface{} {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	
	stats := map[string]interface{}{
		"active_connections": len(s.connections),
		"kcp_config":         s.kcpConfig,
	}
	
	// 收集每个连接的KCP统计信息
	connStats := make([]map[string]interface{}, 0)
	for _, conn := range s.connections {
		if conn.IsActive() {
			connStats = append(connStats, conn.GetKCPStats())
		}
	}
	stats["connection_stats"] = connStats
	
	return stats
}