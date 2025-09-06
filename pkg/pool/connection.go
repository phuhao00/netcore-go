// Package pool 实现NetCore-Go网络库的连接池
// Author: NetCore-Go Team
// Created: 2024

package pool

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionPool 连接池接口
type ConnectionPool interface {
	// Get 获取连接
	Get() (net.Conn, error)
	// Put 归还连接
	Put(conn net.Conn) error
	// Close 关闭连接池
	Close() error
	// Stats 获取统计信息
	Stats() *ConnectionPoolStats
}

// TCPConnectionPool TCP连接池实现
type TCPConnectionPool struct {
	address     string
	minSize     int
	maxSize     int
	idleTimeout time.Duration
	connTimeout time.Duration
	connections chan *pooledConnection
	stats       *ConnectionPoolStats
	running     int32
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// pooledConnection 池化连接
type pooledConnection struct {
	conn     net.Conn
	lastUsed time.Time
	pool     *TCPConnectionPool
}

// ConnectionPoolConfig 连接池配置
type ConnectionPoolConfig struct {
	// Address 连接地址
	Address string
	// MinSize 最小连接数
	MinSize int
	// MaxSize 最大连接数
	MaxSize int
	// IdleTimeout 空闲超时时间
	IdleTimeout time.Duration
	// ConnTimeout 连接超时时间
	ConnTimeout time.Duration
}

// DefaultConnectionPoolConfig 返回默认连接池配置
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MinSize:     5,
		MaxSize:     50,
		IdleTimeout: 300 * time.Second,
		ConnTimeout: 30 * time.Second,
	}
}

// NewTCPConnectionPool 创建新的TCP连接池
func NewTCPConnectionPool(config *ConnectionPoolConfig) *TCPConnectionPool {
	if config == nil {
		config = DefaultConnectionPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &TCPConnectionPool{
		address:     config.Address,
		minSize:     config.MinSize,
		maxSize:     config.MaxSize,
		idleTimeout: config.IdleTimeout,
		connTimeout: config.ConnTimeout,
		connections: make(chan *pooledConnection, config.MaxSize),
		stats:       NewConnectionPoolStats(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动连接池
func (p *TCPConnectionPool) Start() error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return fmt.Errorf("connection pool is already running")
	}

	// 创建最小数量的连接
	for i := 0; i < p.minSize; i++ {
		conn, err := p.createConnection()
		if err != nil {
			return fmt.Errorf("failed to create initial connections: %w", err)
		}
		p.connections <- conn
	}

	// 启动清理协程
	p.wg.Add(1)
	go p.cleaner()

	return nil
}

// Get 获取连接
func (p *TCPConnectionPool) Get() (net.Conn, error) {
	if atomic.LoadInt32(&p.running) == 0 {
		return nil, fmt.Errorf("connection pool is not running")
	}

	select {
	case pooledConn := <-p.connections:
		// 检查连接是否仍然有效
		if p.isConnectionValid(pooledConn.conn) {
			pooledConn.lastUsed = time.Now()
			atomic.AddInt64(&p.stats.Gets, 1)
			atomic.AddInt64(&p.stats.InUse, 1)
			return pooledConn.conn, nil
		}
		// 连接无效，关闭并创建新连接
		pooledConn.conn.Close()
		atomic.AddInt64(&p.stats.Closed, 1)
	default:
	}
	
	// 创建新连接
	conn, err := p.createConnection()
	if err != nil {
		atomic.AddInt64(&p.stats.Errors, 1)
		return nil, err
	}
	atomic.AddInt64(&p.stats.Gets, 1)
	atomic.AddInt64(&p.stats.InUse, 1)
	return conn.conn, nil
}

// Put 归还连接
func (p *TCPConnectionPool) Put(conn net.Conn) error {
	if atomic.LoadInt32(&p.running) == 0 {
		conn.Close()
		return fmt.Errorf("connection pool is not running")
	}

	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// 检查连接是否仍然有效
	if !p.isConnectionValid(conn) {
		conn.Close()
		atomic.AddInt64(&p.stats.Closed, 1)
		atomic.AddInt64(&p.stats.InUse, -1)
		return nil
	}

	pooledConn := &pooledConnection{
		conn:     conn,
		lastUsed: time.Now(),
		pool:     p,
	}

	select {
	case p.connections <- pooledConn:
		atomic.AddInt64(&p.stats.Puts, 1)
		atomic.AddInt64(&p.stats.InUse, -1)
		return nil
	default:
		// 连接池已满，关闭连接
		conn.Close()
		atomic.AddInt64(&p.stats.Closed, 1)
		atomic.AddInt64(&p.stats.InUse, -1)
		return nil
	}
}

// Close 关闭连接池
func (p *TCPConnectionPool) Close() error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return nil // 已经关闭
	}

	// 取消上下文
	p.cancel()

	// 关闭所有连接
	close(p.connections)
	for pooledConn := range p.connections {
		pooledConn.conn.Close()
		atomic.AddInt64(&p.stats.Closed, 1)
	}

	// 等待清理协程结束
	p.wg.Wait()

	return nil
}

// Stats 获取统计信息
func (p *TCPConnectionPool) Stats() *ConnectionPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &ConnectionPoolStats{
		Created:   atomic.LoadInt64(&p.stats.Created),
		Gets:      atomic.LoadInt64(&p.stats.Gets),
		Puts:      atomic.LoadInt64(&p.stats.Puts),
		Closed:    atomic.LoadInt64(&p.stats.Closed),
		InUse:     atomic.LoadInt64(&p.stats.InUse),
		Errors:    atomic.LoadInt64(&p.stats.Errors),
		PoolSize:  int32(len(p.connections)),
		MaxSize:   int32(p.maxSize),
	}
}

// createConnection 创建新连接
func (p *TCPConnectionPool) createConnection() (*pooledConnection, error) {
	ctx, cancel := context.WithTimeout(p.ctx, p.connTimeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.address)
	if err != nil {
		return nil, err
	}

	atomic.AddInt64(&p.stats.Created, 1)
	return &pooledConnection{
		conn:     conn,
		lastUsed: time.Now(),
		pool:     p,
	}, nil
}

// isConnectionValid 检查连接是否有效
func (p *TCPConnectionPool) isConnectionValid(conn net.Conn) bool {
	if conn == nil {
		return false
	}

	// 设置很短的读超时来检查连接状态
	conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	conn.SetReadDeadline(time.Time{}) // 清除超时

	// 如果是超时错误，说明连接正常但没有数据
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// 其他错误说明连接有问题
	return err == nil
}

// cleaner 清理过期连接
func (p *TCPConnectionPool) cleaner() {
	defer p.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.cleanIdleConnections()
		}
	}
}

// cleanIdleConnections 清理空闲连接
func (p *TCPConnectionPool) cleanIdleConnections() {
	now := time.Now()
	var toClose []*pooledConnection

	// 收集需要关闭的连接
	for {
		select {
		case pooledConn := <-p.connections:
			if now.Sub(pooledConn.lastUsed) > p.idleTimeout {
				toClose = append(toClose, pooledConn)
			} else {
				// 连接还没过期，放回池中
				select {
				case p.connections <- pooledConn:
				default:
					// 池满了，关闭连接
					toClose = append(toClose, pooledConn)
				}
			}
		default:
			// 没有更多连接了
			goto cleanup
		}
	}

cleanup:
	// 关闭过期连接
	for _, pooledConn := range toClose {
		pooledConn.conn.Close()
		atomic.AddInt64(&p.stats.Closed, 1)
	}
}

// ConnectionPoolStats 连接池统计信息
type ConnectionPoolStats struct {
	// Created 创建的连接数
	Created int64 `json:"created"`
	// Gets 获取操作次数
	Gets int64 `json:"gets"`
	// Puts 归还操作次数
	Puts int64 `json:"puts"`
	// Closed 关闭的连接数
	Closed int64 `json:"closed"`
	// InUse 正在使用的连接数
	InUse int64 `json:"in_use"`
	// Errors 错误次数
	Errors int64 `json:"errors"`
	// PoolSize 当前池大小
	PoolSize int32 `json:"pool_size"`
	// MaxSize 最大池大小
	MaxSize int32 `json:"max_size"`
}

// NewConnectionPoolStats 创建新的连接池统计信息
func NewConnectionPoolStats() *ConnectionPoolStats {
	return &ConnectionPoolStats{}
}

// HitRate 计算命中率
func (s *ConnectionPoolStats) HitRate() float64 {
	if s.Gets == 0 {
		return 0
	}
	return float64(s.Gets-s.Created) / float64(s.Gets)
}