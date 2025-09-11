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

	"github.com/netcore-go/pkg/core"
)

// KCPServer KCP服务器实现
type KCPServer struct {
	core.BaseServer
	listener    *kcp.Listener
	connections map[string]*KCPConnection
	connMu      sync.RWMutex
	kcpConfig   *KCPConfig
	running     int32
	handler     core.MessageHandler
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
	
	// 创建基础服务器
	server.BaseServer = *core.NewBaseServer(opts...)
	
	// 设置默认配置
	config := server.BaseServer.GetConfig()
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = 4096
	}
	if config.WriteBufferSize == 0 {
		config.WriteBufferSize = 4096
	}
	if config.MaxConnections == 0 {
		config.MaxConnections = 1000
	}
	
	return server
}

// SetHandler 设置消息处理器
func (s *KCPServer) SetHandler(handler core.MessageHandler) {
	s.handler = handler
}

// Stop 停止KCP服务器
func (s *KCPServer) Stop() error {
	atomic.StoreInt32(&s.running, 0)
	
	if s.listener != nil {
		return s.listener.Close()
	}
	
	return nil
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
	var listener net.Listener
	switch s.kcpConfig.Crypt {
	case "aes":
		listener, err = kcp.ListenWithOptions(addr, nil, s.kcpConfig.DataShard, s.kcpConfig.ParityShard)
	case "none":
		listener, err = kcp.Listen(addr)
	default:
		listener, err = kcp.ListenWithOptions(addr, nil, s.kcpConfig.DataShard, s.kcpConfig.ParityShard)
	}
	
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	
	// 类型断言为KCP监听器
	s.listener = listener.(*kcp.Listener)
	// 设置运行状态
	atomic.StoreInt32(&s.running, 1)
	stats := s.BaseServer.GetStats()
	stats.StartTime = time.Now()
	
	// 启动连接池（暂时注释掉）
	// if s.config.EnableConnectionPool {
	//	s.ConnPool = pool.NewConnectionPool(pool.PoolConfig{
	//		InitialSize: 10,
	//		MaxSize:     s.config.MaxConnections,
	//		IdleTimeout: 5 * time.Minute,
	//	})
	// }
	
	// 启动内存池（暂时注释掉）
	// if s.config.EnableMemoryPool {
	//	s.MemPool = pool.NewMemoryPool(s.config.ReadBufferSize, 100)
	// }
	
	// 启动协程池（暂时注释掉）
	// if s.config.EnableGoroutinePool {
	//	s.GoroutinePool = pool.NewGoroutinePool(1000, 10000)
	// }
	
	go s.acceptLoop()
	
	return nil
}

// acceptLoop 接受连接循环
func (s *KCPServer) acceptLoop() {
	for atomic.LoadInt32(&s.running) == 1 {
		conn, err := s.listener.AcceptKCP()
		if err != nil {
			if atomic.LoadInt32(&s.running) == 1 {
				stats := s.BaseServer.GetStats()
				stats.ErrorCount++
				stats.LastError = err.Error()
			}
			continue
		}
		
		// 检查连接数限制
		stats := s.BaseServer.GetStats()
		config := s.BaseServer.GetConfig()
		if stats.ActiveConnections >= int64(config.MaxConnections) {
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
		
		// 使用协程池处理连接（暂时注释掉）
		// if s.GoroutinePool != nil {
		//	s.GoroutinePool.Submit(func() {
		//		s.handleConnection(kcpConn)
		//	})
		// } else {
			go s.handleConnection(kcpConn)
		// }
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
	config := s.BaseServer.GetConfig()
	conn.SetReadBuffer(config.ReadBufferSize)
	conn.SetWriteBuffer(config.WriteBufferSize)
	
	// 设置超时
	if config.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	}
	if config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
	}
}

// handleConnection 处理KCP连接
func (s *KCPServer) handleConnection(conn *KCPConnection) {
	defer func() {
		if r := recover(); r != nil {
			stats := s.BaseServer.GetStats()
			stats.ErrorCount++
			stats.LastError = fmt.Sprintf("panic: %v", r)
		}
		
		// 从连接管理中移除
		s.connMu.Lock()
		delete(s.connections, conn.ID())
		s.connMu.Unlock()
		
		conn.Close()
	}()
	
	stats := s.BaseServer.GetStats()
	atomic.AddInt64(&stats.ActiveConnections, 1)
	atomic.AddInt64(&stats.TotalConnections, 1)
	defer atomic.AddInt64(&stats.ActiveConnections, -1)
	
	// 调用连接处理器
	if s.handler != nil {
		s.handler.OnConnect(conn)
		defer s.handler.OnDisconnect(conn, nil)
	}
	
	// 处理消息
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			if s.handler != nil {
				s.handler.OnDisconnect(conn, err)
			}
			break
		}
		
		stats := s.BaseServer.GetStats()
		atomic.AddInt64(&stats.MessagesReceived, 1)
		atomic.AddInt64(&stats.BytesReceived, int64(len(msg.Data)))
		
		// 处理消息
		if s.handler != nil {
			s.handler.OnMessage(conn, msg)
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

// GetStats 获取服务器统计信息
func (s *KCPServer) GetStats() *core.ServerStats {
	return s.BaseServer.GetStats()
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

