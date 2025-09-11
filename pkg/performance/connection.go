package performance

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionReuseConfig 连接复用配置
type ConnectionReuseConfig struct {
	// 启用连接复用
	Enabled bool `json:"enabled" yaml:"enabled"`
	// 最大空闲连接数
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns"`
	// 每个主机最大空闲连接数
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	// 最大连接数
	MaxConnsPerHost int `json:"max_conns_per_host" yaml:"max_conns_per_host"`
	// 空闲连接超时
	IdleConnTimeout time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`
	// Keep-Alive间隔
	KeepAliveInterval time.Duration `json:"keep_alive_interval" yaml:"keep_alive_interval"`
	// Keep-Alive超时
	KeepAliveTimeout time.Duration `json:"keep_alive_timeout" yaml:"keep_alive_timeout"`
	// 连接超时
	DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	// 读取超时
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`
	// 写入超时
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	// 启用TCP_NODELAY
	TCPNoDelay bool `json:"tcp_no_delay" yaml:"tcp_no_delay"`
	// 启用SO_REUSEADDR
	ReuseAddr bool `json:"reuse_addr" yaml:"reuse_addr"`
	// 启用SO_REUSEPORT
	ReusePort bool `json:"reuse_port" yaml:"reuse_port"`
	// 发送缓冲区大小
	SendBufferSize int `json:"send_buffer_size" yaml:"send_buffer_size"`
	// 接收缓冲区大小
	ReceiveBufferSize int `json:"receive_buffer_size" yaml:"receive_buffer_size"`
}

// DefaultConnectionReuseConfig 默认连接复用配置
func DefaultConnectionReuseConfig() *ConnectionReuseConfig {
	return &ConnectionReuseConfig{
		Enabled:                 true,
		MaxIdleConns:            100,
		MaxIdleConnsPerHost:     10,
		MaxConnsPerHost:         50,
		IdleConnTimeout:         90 * time.Second,
		KeepAliveInterval:       30 * time.Second,
		KeepAliveTimeout:        60 * time.Second,
		DialTimeout:             10 * time.Second,
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            30 * time.Second,
		TCPNoDelay:              true,
		ReuseAddr:               true,
		ReusePort:               false,
		SendBufferSize:          64 * 1024,   // 64KB
		ReceiveBufferSize:       64 * 1024,   // 64KB
	}
}

// PooledConnection 池化连接
type PooledConnection struct {
	net.Conn
	pool         *ConnectionPool
	host         string
	createdAt    time.Time
	lastUsed     time.Time
	useCount     int64
	closed       bool
	mu           sync.RWMutex
	keepAlive    bool
	keepAliveCh  chan struct{}
	keepAliveCtx context.Context
	keepAliveCancel context.CancelFunc
}

// NewPooledConnection 创建池化连接
func NewPooledConnection(conn net.Conn, pool *ConnectionPool, host string) *PooledConnection {
	ctx, cancel := context.WithCancel(context.Background())
	pc := &PooledConnection{
		Conn:            conn,
		pool:            pool,
		host:            host,
		createdAt:       time.Now(),
		lastUsed:        time.Now(),
		keepAlive:       true,
		keepAliveCh:     make(chan struct{}, 1),
		keepAliveCtx:    ctx,
		keepAliveCancel: cancel,
	}

	// 启动Keep-Alive
	go pc.startKeepAlive()

	return pc
}

// Use 使用连接
func (pc *PooledConnection) Use() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.lastUsed = time.Now()
	atomic.AddInt64(&pc.useCount, 1)
}

// Release 释放连接回池
func (pc *PooledConnection) Release() error {
	pc.mu.RLock()
	closed := pc.closed
	pc.mu.RUnlock()

	if closed {
		return fmt.Errorf("connection already closed")
	}

	return pc.pool.putConnection(pc)
}

// Close 关闭连接
func (pc *PooledConnection) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil
	}

	pc.closed = true
	pc.keepAliveCancel()
	close(pc.keepAliveCh)

	return pc.Conn.Close()
}

// IsExpired 检查连接是否过期
func (pc *PooledConnection) IsExpired(timeout time.Duration) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return time.Since(pc.lastUsed) > timeout
}

// IsHealthy 检查连接是否健康
func (pc *PooledConnection) IsHealthy() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return false
	}

	// 尝试设置读取超时来检测连接状态
	if tcpConn, ok := pc.Conn.(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(time.Now().Add(1 * time.Millisecond)); err != nil {
			return false
		}
		
		// 尝试读取一个字节（非阻塞）
		buf := make([]byte, 1)
		_, err := tcpConn.Read(buf)
		
		// 重置读取超时
		tcpConn.SetReadDeadline(time.Time{})
		
		// 如果是超时错误，说明连接正常
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		
		// 其他错误说明连接有问题
		return err == nil
	}

	return true
}

// startKeepAlive 启动Keep-Alive
func (pc *PooledConnection) startKeepAlive() {
	if !pc.pool.config.Enabled {
		return
	}

	ticker := time.NewTicker(pc.pool.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pc.keepAliveCtx.Done():
			return
		case <-ticker.C:
			if !pc.sendKeepAlive() {
				return
			}
		}
	}
}

// sendKeepAlive 发送Keep-Alive
func (pc *PooledConnection) sendKeepAlive() bool {
	pc.mu.RLock()
	closed := pc.closed
	pc.mu.RUnlock()

	if closed {
		return false
	}

	// 对于TCP连接，设置Keep-Alive
	if tcpConn, ok := pc.Conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			return false
		}
		
		if err := tcpConn.SetKeepAlivePeriod(pc.pool.config.KeepAliveInterval); err != nil {
			return false
		}
	}

	return true
}

// GetStats 获取连接统计信息
func (pc *PooledConnection) GetStats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return map[string]interface{}{
		"host":       pc.host,
		"created_at": pc.createdAt,
		"last_used":  pc.lastUsed,
		"use_count":  atomic.LoadInt64(&pc.useCount),
		"closed":     pc.closed,
		"age":        time.Since(pc.createdAt),
		"idle_time":  time.Since(pc.lastUsed),
	}
}

// ConnectionPool 连接池
type ConnectionPool struct {
	config      *ConnectionReuseConfig
	connections map[string][]*PooledConnection
	stats       *ConnectionPoolStats
	mu          sync.RWMutex
	running     bool
	cancel      context.CancelFunc
	dialer      *net.Dialer
}

// ConnectionPoolStats 连接池统计信息
type ConnectionPoolStats struct {
	TotalConnections   int64 `json:"total_connections"`
	ActiveConnections  int64 `json:"active_connections"`
	IdleConnections    int64 `json:"idle_connections"`
	ConnectionHits     int64 `json:"connection_hits"`
	ConnectionMisses   int64 `json:"connection_misses"`
	ConnectionCreated  int64 `json:"connection_created"`
	ConnectionClosed   int64 `json:"connection_closed"`
	ConnectionReused   int64 `json:"connection_reused"`
	ConnectionExpired  int64 `json:"connection_expired"`
	AverageConnAge     time.Duration `json:"average_conn_age"`
	AverageUseCount    float64 `json:"average_use_count"`
}

// NewConnectionPool 创建连接池
func NewConnectionPool(config *ConnectionReuseConfig) *ConnectionPool {
	if config == nil {
		config = DefaultConnectionReuseConfig()
	}

	pool := &ConnectionPool{
		config:      config,
		connections: make(map[string][]*PooledConnection),
		stats:       &ConnectionPoolStats{},
	}

	// 配置拨号器
	pool.dialer = &net.Dialer{
		Timeout:   config.DialTimeout,
		KeepAlive: config.KeepAliveInterval,
	}

	return pool
}

// Start 启动连接池
func (cp *ConnectionPool) Start(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.running {
		return fmt.Errorf("connection pool already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	cp.cancel = cancel
	cp.running = true

	// 启动清理器
	go cp.cleaner(ctx)
	go cp.statsCollector(ctx)

	return nil
}

// Stop 停止连接池
func (cp *ConnectionPool) Stop() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.running {
		return fmt.Errorf("connection pool not running")
	}

	cp.cancel()
	cp.running = false

	// 关闭所有连接
	for host, conns := range cp.connections {
		for _, conn := range conns {
			conn.Close()
		}
		delete(cp.connections, host)
	}

	return nil
}

// GetConnection 获取连接
func (cp *ConnectionPool) GetConnection(host string) (*PooledConnection, error) {
	if !cp.config.Enabled {
		return cp.createConnection(host)
	}

	// 尝试从池中获取连接
	if conn := cp.getPooledConnection(host); conn != nil {
		atomic.AddInt64(&cp.stats.ConnectionHits, 1)
		atomic.AddInt64(&cp.stats.ConnectionReused, 1)
		conn.Use()
		return conn, nil
	}

	// 池中没有可用连接，创建新连接
	atomic.AddInt64(&cp.stats.ConnectionMisses, 1)
	return cp.createConnection(host)
}

// getPooledConnection 从池中获取连接
func (cp *ConnectionPool) getPooledConnection(host string) *PooledConnection {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	conns, exists := cp.connections[host]
	if !exists || len(conns) == 0 {
		return nil
	}

	// 从后面取连接（LIFO，更可能是热连接）
	for i := len(conns) - 1; i >= 0; i-- {
		conn := conns[i]
		
		// 检查连接是否健康且未过期
		if conn.IsHealthy() && !conn.IsExpired(cp.config.IdleConnTimeout) {
			// 从池中移除
			cp.connections[host] = append(conns[:i], conns[i+1:]...)
			atomic.AddInt64(&cp.stats.IdleConnections, -1)
			atomic.AddInt64(&cp.stats.ActiveConnections, 1)
			return conn
		} else {
			// 连接不健康或过期，关闭并移除
			conn.Close()
			cp.connections[host] = append(conns[:i], conns[i+1:]...)
			atomic.AddInt64(&cp.stats.ConnectionClosed, 1)
			atomic.AddInt64(&cp.stats.ConnectionExpired, 1)
			atomic.AddInt64(&cp.stats.IdleConnections, -1)
		}
	}

	return nil
}

// createConnection 创建新连接
func (cp *ConnectionPool) createConnection(host string) (*PooledConnection, error) {
	// 检查连接数限制
	if cp.config.MaxConnsPerHost > 0 {
		cp.mu.RLock()
		currentConns := len(cp.connections[host])
		cp.mu.RUnlock()
		
		if currentConns >= cp.config.MaxConnsPerHost {
			return nil, fmt.Errorf("max connections per host exceeded: %d", cp.config.MaxConnsPerHost)
		}
	}

	// 创建连接
	conn, err := cp.dialer.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	// 优化TCP连接
	if err := cp.optimizeTCPConnection(conn); err != nil {
		conn.Close()
		return nil, err
	}

	pooledConn := NewPooledConnection(conn, cp, host)
	atomic.AddInt64(&cp.stats.ConnectionCreated, 1)
	atomic.AddInt64(&cp.stats.TotalConnections, 1)
	atomic.AddInt64(&cp.stats.ActiveConnections, 1)

	return pooledConn, nil
}

// optimizeTCPConnection 优化TCP连接
func (cp *ConnectionPool) optimizeTCPConnection(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	// 设置TCP_NODELAY
	if cp.config.TCPNoDelay {
		if err := tcpConn.SetNoDelay(true); err != nil {
			return err
		}
	}

	// 设置Keep-Alive
	if err := tcpConn.SetKeepAlive(true); err != nil {
		return err
	}

	if err := tcpConn.SetKeepAlivePeriod(cp.config.KeepAliveInterval); err != nil {
		return err
	}

	// 设置缓冲区大小
	if cp.config.SendBufferSize > 0 {
		if err := tcpConn.SetWriteBuffer(cp.config.SendBufferSize); err != nil {
			return err
		}
	}

	if cp.config.ReceiveBufferSize > 0 {
		if err := tcpConn.SetReadBuffer(cp.config.ReceiveBufferSize); err != nil {
			return err
		}
	}

	return nil
}

// putConnection 将连接放回池中
func (cp *ConnectionPool) putConnection(conn *PooledConnection) error {
	if !cp.config.Enabled {
		return conn.Close()
	}

	// 检查连接是否健康
	if !conn.IsHealthy() {
		atomic.AddInt64(&cp.stats.ConnectionClosed, 1)
		atomic.AddInt64(&cp.stats.ActiveConnections, -1)
		return conn.Close()
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 检查池大小限制
	conns := cp.connections[conn.host]
	if len(conns) >= cp.config.MaxIdleConnsPerHost {
		// 池已满，关闭连接
		atomic.AddInt64(&cp.stats.ConnectionClosed, 1)
		atomic.AddInt64(&cp.stats.ActiveConnections, -1)
		return conn.Close()
	}

	// 将连接放回池中
	cp.connections[conn.host] = append(conns, conn)
	atomic.AddInt64(&cp.stats.ActiveConnections, -1)
	atomic.AddInt64(&cp.stats.IdleConnections, 1)

	return nil
}

// cleaner 清理过期连接
func (cp *ConnectionPool) cleaner(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cp.cleanExpiredConnections()
		}
	}
}

// cleanExpiredConnections 清理过期连接
func (cp *ConnectionPool) cleanExpiredConnections() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for host, conns := range cp.connections {
		validConns := make([]*PooledConnection, 0, len(conns))
		
		for _, conn := range conns {
			if conn.IsHealthy() && !conn.IsExpired(cp.config.IdleConnTimeout) {
				validConns = append(validConns, conn)
			} else {
				conn.Close()
				atomic.AddInt64(&cp.stats.ConnectionClosed, 1)
				atomic.AddInt64(&cp.stats.ConnectionExpired, 1)
				atomic.AddInt64(&cp.stats.IdleConnections, -1)
			}
		}
		
		if len(validConns) == 0 {
			delete(cp.connections, host)
		} else {
			cp.connections[host] = validConns
		}
	}
}

// statsCollector 统计信息收集器
func (cp *ConnectionPool) statsCollector(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cp.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (cp *ConnectionPool) updateStats() {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var totalAge time.Duration
	var totalUseCount int64
	var connCount int64

	for _, conns := range cp.connections {
		for _, conn := range conns {
			stats := conn.GetStats()
			totalAge += stats["age"].(time.Duration)
			totalUseCount += stats["use_count"].(int64)
			connCount++
		}
	}

	if connCount > 0 {
		cp.stats.AverageConnAge = totalAge / time.Duration(connCount)
		cp.stats.AverageUseCount = float64(totalUseCount) / float64(connCount)
	}
}

// GetStats 获取统计信息
func (cp *ConnectionPool) GetStats() *ConnectionPoolStats {
	cp.updateStats()
	return &ConnectionPoolStats{
		TotalConnections:  atomic.LoadInt64(&cp.stats.TotalConnections),
		ActiveConnections: atomic.LoadInt64(&cp.stats.ActiveConnections),
		IdleConnections:   atomic.LoadInt64(&cp.stats.IdleConnections),
		ConnectionHits:    atomic.LoadInt64(&cp.stats.ConnectionHits),
		ConnectionMisses:  atomic.LoadInt64(&cp.stats.ConnectionMisses),
		ConnectionCreated: atomic.LoadInt64(&cp.stats.ConnectionCreated),
		ConnectionClosed:  atomic.LoadInt64(&cp.stats.ConnectionClosed),
		ConnectionReused:  atomic.LoadInt64(&cp.stats.ConnectionReused),
		ConnectionExpired: atomic.LoadInt64(&cp.stats.ConnectionExpired),
		AverageConnAge:    cp.stats.AverageConnAge,
		AverageUseCount:   cp.stats.AverageUseCount,
	}
}

// GetConnectionsByHost 获取指定主机的连接信息
func (cp *ConnectionPool) GetConnectionsByHost(host string) []map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	conns, exists := cp.connections[host]
	if !exists {
		return nil
	}

	result := make([]map[string]interface{}, len(conns))
	for i, conn := range conns {
		result[i] = conn.GetStats()
	}

	return result
}

// GetAllHosts 获取所有主机列表
func (cp *ConnectionPool) GetAllHosts() []string {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	hosts := make([]string, 0, len(cp.connections))
	for host := range cp.connections {
		hosts = append(hosts, host)
	}

	return hosts
}

// CloseConnectionsForHost 关闭指定主机的所有连接
func (cp *ConnectionPool) CloseConnectionsForHost(host string) int {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	conns, exists := cp.connections[host]
	if !exists {
		return 0
	}

	count := len(conns)
	for _, conn := range conns {
		conn.Close()
		atomic.AddInt64(&cp.stats.ConnectionClosed, 1)
		atomic.AddInt64(&cp.stats.IdleConnections, -1)
	}

	delete(cp.connections, host)
	return count
}

// SetConfig 更新配置
func (cp *ConnectionPool) SetConfig(config *ConnectionReuseConfig) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.config = config

	// 更新拨号器配置
	cp.dialer.Timeout = config.DialTimeout
	cp.dialer.KeepAlive = config.KeepAliveInterval
}

// GetConfig 获取当前配置
func (cp *ConnectionPool) GetConfig() *ConnectionReuseConfig {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// 返回配置的副本
	config := *cp.config
	return &config
}

// Warmup 预热连接池
func (cp *ConnectionPool) Warmup(hosts []string, connectionsPerHost int) error {
	if !cp.config.Enabled {
		return fmt.Errorf("connection pool disabled")
	}

	for _, host := range hosts {
		for i := 0; i < connectionsPerHost; i++ {
			conn, err := cp.createConnection(host)
			if err != nil {
				return fmt.Errorf("failed to create connection to %s: %v", host, err)
			}
			
			// 立即放回池中
			if err := cp.putConnection(conn); err != nil {
				conn.Close()
				return fmt.Errorf("failed to put connection back to pool: %v", err)
			}
		}
	}

	return nil
}

// HealthCheck 健康检查
func (cp *ConnectionPool) HealthCheck() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	totalConns := 0
	healthyConns := 0
	hostStats := make(map[string]map[string]int)

	for host, conns := range cp.connections {
		hostHealthy := 0
		for _, conn := range conns {
			totalConns++
			if conn.IsHealthy() {
				healthyConns++
				hostHealthy++
			}
		}
		hostStats[host] = map[string]int{
			"total":   len(conns),
			"healthy": hostHealthy,
		}
	}

	return map[string]interface{}{
		"total_connections":   totalConns,
		"healthy_connections": healthyConns,
		"health_rate":         float64(healthyConns) / float64(totalConns+1),
		"host_stats":          hostStats,
		"pool_enabled":        cp.config.Enabled,
		"running":             cp.running,
	}
}