// Package database 数据库连接池实现
// Author: NetCore-Go Team
// Created: 2024

package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
)

// ConnectionPool 数据库连接池接口
type ConnectionPool interface {
	Get(ctx context.Context) (*sql.Conn, error)
	Put(conn *sql.Conn) error
	Close() error
	Stats() *PoolStats
	HealthCheck() error
}

// Pool 数据库连接池实现
type Pool struct {
	mu           sync.RWMutex
	config       *PoolConfig
	db           *sql.DB
	connections  chan *pooledConnection
	closed       int32
	stats        *PoolStats
	ctx          context.Context
	cancel       context.CancelFunc
	cleanupTicker *time.Ticker
}

// PoolConfig 连接池配置
type PoolConfig struct {
	// 数据库连接配置
	DriverName     string `json:"driver_name"`
	DataSourceName string `json:"data_source_name"`

	// 连接池大小配置
	MaxOpenConns    int `json:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int `json:"max_idle_conns"`    // 最大空闲连接数
	MinIdleConns    int `json:"min_idle_conns"`    // 最小空闲连接数

	// 连接生命周期配置
	MaxConnLifetime time.Duration `json:"max_conn_lifetime"` // 连接最大生命周期
	MaxConnIdleTime time.Duration `json:"max_conn_idle_time"` // 连接最大空闲时间
	ConnTimeout     time.Duration `json:"conn_timeout"`      // 连接超时

	// 健康检查配置
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckQuery    string        `json:"health_check_query"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`

	// 重试配置
	MaxRetries    int           `json:"max_retries"`
	RetryInterval time.Duration `json:"retry_interval"`

	// 监控配置
	MetricsEnabled bool `json:"metrics_enabled"`
	SlowQueryThreshold time.Duration `json:"slow_query_threshold"`

	// 清理配置
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// PoolStats 连接池统计信息
type PoolStats struct {
	// 连接统计
	MaxOpenConnections     int32 `json:"max_open_connections"`
	OpenConnections        int32 `json:"open_connections"`
	InUseConnections       int32 `json:"in_use_connections"`
	IdleConnections        int32 `json:"idle_connections"`

	// 操作统计
	TotalConnections       int64 `json:"total_connections"`
	SuccessfulConnections  int64 `json:"successful_connections"`
	FailedConnections      int64 `json:"failed_connections"`
	ConnectionsReused      int64 `json:"connections_reused"`
	ConnectionsCreated     int64 `json:"connections_created"`
	ConnectionsClosed      int64 `json:"connections_closed"`

	// 时间统计
	AverageConnectTime     time.Duration `json:"average_connect_time"`
	AverageQueryTime       time.Duration `json:"average_query_time"`
	MaxConnectTime         time.Duration `json:"max_connect_time"`
	MaxQueryTime           time.Duration `json:"max_query_time"`

	// 错误统计
	TimeoutErrors          int64 `json:"timeout_errors"`
	ConnectionErrors       int64 `json:"connection_errors"`
	QueryErrors            int64 `json:"query_errors"`
	SlowQueries            int64 `json:"slow_queries"`

	// 健康检查统计
	HealthCheckCount       int64     `json:"health_check_count"`
	HealthCheckFailures    int64     `json:"health_check_failures"`
	LastHealthCheck        time.Time `json:"last_health_check"`
	Healthy                bool      `json:"healthy"`

	// 其他统计
	Uptime                 time.Duration `json:"uptime"`
	StartTime              time.Time     `json:"start_time"`
}

// pooledConnection 池化连接
type pooledConnection struct {
	conn      *sql.Conn
	createdAt time.Time
	lastUsed  time.Time
	useCount  int64
	healthy   bool
}

// DefaultPoolConfig 返回默认连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		DriverName:              "postgres",
		MaxOpenConns:            25,
		MaxIdleConns:            10,
		MinIdleConns:            2,
		MaxConnLifetime:         1 * time.Hour,
		MaxConnIdleTime:         10 * time.Minute,
		ConnTimeout:             30 * time.Second,
		HealthCheckEnabled:      true,
		HealthCheckInterval:     30 * time.Second,
		HealthCheckQuery:        "SELECT 1",
		HealthCheckTimeout:      5 * time.Second,
		MaxRetries:              3,
		RetryInterval:           1 * time.Second,
		MetricsEnabled:          true,
		SlowQueryThreshold:      1 * time.Second,
		CleanupInterval:         5 * time.Minute,
	}
}

// NewPool 创建新的连接池
func NewPool(config *PoolConfig) (*Pool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid pool config: %w", err)
	}

	// 创建数据库连接
	db, err := sql.Open(config.DriverName, config.DataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置数据库连接池
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.MaxConnLifetime)
	db.SetConnMaxIdleTime(config.MaxConnIdleTime)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	pool := &Pool{
		config:      config,
		db:          db,
		connections: make(chan *pooledConnection, config.MaxIdleConns),
		stats: &PoolStats{
			MaxOpenConnections: int32(config.MaxOpenConns),
			StartTime:          time.Now(),
			Healthy:            true,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动后台任务
	pool.startBackgroundTasks()

	// 预热连接池
	if err := pool.warmup(); err != nil {
		fmt.Printf("Pool warmup warning: %v\n", err)
	}

	return pool, nil
}

// validateConfig 验证配置
func validateConfig(config *PoolConfig) error {
	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive")
	}
	if config.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns cannot be negative")
	}
	if config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("max_idle_conns cannot be greater than max_open_conns")
	}
	if config.MinIdleConns < 0 {
		return fmt.Errorf("min_idle_conns cannot be negative")
	}
	if config.MinIdleConns > config.MaxIdleConns {
		return fmt.Errorf("min_idle_conns cannot be greater than max_idle_conns")
	}
	if config.ConnTimeout <= 0 {
		return fmt.Errorf("conn_timeout must be positive")
	}
	return nil
}

// Get 获取连接
func (p *Pool) Get(ctx context.Context) (*sql.Conn, error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, fmt.Errorf("connection pool is closed")
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		p.updateConnectTime(duration)
	}()

	// 尝试从池中获取连接
	select {
	case pooledConn := <-p.connections:
		if p.isConnectionValid(pooledConn) {
			pooledConn.lastUsed = time.Now()
			pooledConn.useCount++
			atomic.AddInt64(&p.stats.ConnectionsReused, 1)
			atomic.AddInt32(&p.stats.InUseConnections, 1)
			atomic.AddInt32(&p.stats.IdleConnections, -1)
			return pooledConn.conn, nil
		} else {
			// 连接无效，关闭并创建新连接
			p.closeConnection(pooledConn)
		}
	default:
		// 池中没有可用连接
	}

	// 创建新连接
	return p.createConnection(ctx)
}

// Put 归还连接
func (p *Pool) Put(conn *sql.Conn) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return conn.Close()
	}

	atomic.AddInt32(&p.stats.InUseConnections, -1)

	// 检查连接是否健康
	if err := p.pingConnection(conn); err != nil {
		return conn.Close()
	}

	// 尝试将连接放回池中
	pooledConn := &pooledConnection{
		conn:     conn,
		lastUsed: time.Now(),
		healthy:  true,
	}

	select {
	case p.connections <- pooledConn:
		atomic.AddInt32(&p.stats.IdleConnections, 1)
		return nil
	default:
		// 池已满，关闭连接
		return conn.Close()
	}
}

// Close 关闭连接池
func (p *Pool) Close() error {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return fmt.Errorf("connection pool already closed")
	}

	p.cancel()

	// 停止清理任务
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
	}

	// 关闭所有池中的连接
	close(p.connections)
	for pooledConn := range p.connections {
		p.closeConnection(pooledConn)
	}

	// 关闭数据库
	return p.db.Close()
}

// Stats 获取统计信息
func (p *Pool) Stats() *PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := *p.stats
	stats.Uptime = time.Since(p.stats.StartTime)
	stats.OpenConnections = int32(len(p.connections)) + p.stats.InUseConnections

	return &stats
}

// HealthCheck 健康检查
func (p *Pool) HealthCheck() error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return fmt.Errorf("connection pool is closed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.config.HealthCheckTimeout)
	defer cancel()

	start := time.Now()
	atomic.AddInt64(&p.stats.HealthCheckCount, 1)

	// 执行健康检查查询
	var result int
	err := p.db.QueryRowContext(ctx, p.config.HealthCheckQuery).Scan(&result)
	if err != nil {
		atomic.AddInt64(&p.stats.HealthCheckFailures, 1)
		p.stats.Healthy = false
		return fmt.Errorf("health check failed: %w", err)
	}

	p.stats.Healthy = true
	p.stats.LastHealthCheck = time.Now()

	// 更新查询时间统计
	duration := time.Since(start)
	p.updateQueryTime(duration)

	return nil
}

// createConnection 创建新连接
func (p *Pool) createConnection(ctx context.Context) (*sql.Conn, error) {
	if int(atomic.LoadInt32(&p.stats.OpenConnections)) >= p.config.MaxOpenConns {
		return nil, fmt.Errorf("maximum number of connections reached")
	}

	connCtx, cancel := context.WithTimeout(ctx, p.config.ConnTimeout)
	defer cancel()

	conn, err := p.db.Conn(connCtx)
	if err != nil {
		atomic.AddInt64(&p.stats.FailedConnections, 1)
		atomic.AddInt64(&p.stats.ConnectionErrors, 1)
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	atomic.AddInt64(&p.stats.TotalConnections, 1)
	atomic.AddInt64(&p.stats.SuccessfulConnections, 1)
	atomic.AddInt64(&p.stats.ConnectionsCreated, 1)
	atomic.AddInt32(&p.stats.OpenConnections, 1)
	atomic.AddInt32(&p.stats.InUseConnections, 1)

	return conn, nil
}

// isConnectionValid 检查连接是否有效
func (p *Pool) isConnectionValid(pooledConn *pooledConnection) bool {
	// 检查连接年龄
	if p.config.MaxConnLifetime > 0 && time.Since(pooledConn.createdAt) > p.config.MaxConnLifetime {
		return false
	}

	// 检查空闲时间
	if p.config.MaxConnIdleTime > 0 && time.Since(pooledConn.lastUsed) > p.config.MaxConnIdleTime {
		return false
	}

	// 检查连接健康状态
	if !pooledConn.healthy {
		return false
	}

	return true
}

// pingConnection 测试连接
func (p *Pool) pingConnection(conn *sql.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return conn.PingContext(ctx)
}

// closeConnection 关闭连接
func (p *Pool) closeConnection(pooledConn *pooledConnection) {
	if pooledConn != nil && pooledConn.conn != nil {
		pooledConn.conn.Close()
		atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
		atomic.AddInt32(&p.stats.OpenConnections, -1)
	}
}

// warmup 预热连接池
func (p *Pool) warmup() error {
	if p.config.MinIdleConns <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < p.config.MinIdleConns; i++ {
		conn, err := p.db.Conn(ctx)
		if err != nil {
			return fmt.Errorf("failed to create warmup connection %d: %w", i, err)
		}

		pooledConn := &pooledConnection{
			conn:      conn,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
			healthy:   true,
		}

		select {
		case p.connections <- pooledConn:
			atomic.AddInt32(&p.stats.IdleConnections, 1)
			atomic.AddInt32(&p.stats.OpenConnections, 1)
		default:
			// 池已满
			conn.Close()
			break
		}
	}

	return nil
}

// startBackgroundTasks 启动后台任务
func (p *Pool) startBackgroundTasks() {
	// 启动清理任务
	if p.config.CleanupInterval > 0 {
		p.cleanupTicker = time.NewTicker(p.config.CleanupInterval)
		go p.cleanupLoop()
	}

	// 启动健康检查任务
	if p.config.HealthCheckEnabled && p.config.HealthCheckInterval > 0 {
		go p.healthCheckLoop()
	}
}

// cleanupLoop 清理循环
func (p *Pool) cleanupLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.cleanupTicker.C:
			p.cleanup()
		}
	}
}

// cleanup 清理过期连接
func (p *Pool) cleanup() {
	now := time.Now()
	var validConnections []*pooledConnection

	// 收集所有连接
	for {
		select {
		case pooledConn := <-p.connections:
			if p.isConnectionValid(pooledConn) {
				validConnections = append(validConnections, pooledConn)
			} else {
				p.closeConnection(pooledConn)
				atomic.AddInt32(&p.stats.IdleConnections, -1)
			}
		default:
			// 没有更多连接
			goto putBack
		}
	}

putBack:
	// 将有效连接放回池中
	for _, pooledConn := range validConnections {
		select {
		case p.connections <- pooledConn:
			// 成功放回
		default:
			// 池已满，关闭连接
			p.closeConnection(pooledConn)
			atomic.AddInt32(&p.stats.IdleConnections, -1)
		}
	}
}

// healthCheckLoop 健康检查循环
func (p *Pool) healthCheckLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.HealthCheck(); err != nil {
				fmt.Printf("Health check failed: %v\n", err)
			}
		}
	}
}

// updateConnectTime 更新连接时间统计
func (p *Pool) updateConnectTime(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stats.MaxConnectTime < duration {
		p.stats.MaxConnectTime = duration
	}

	if p.stats.AverageConnectTime == 0 {
		p.stats.AverageConnectTime = duration
	} else {
		p.stats.AverageConnectTime = (p.stats.AverageConnectTime + duration) / 2
	}
}

// updateQueryTime 更新查询时间统计
func (p *Pool) updateQueryTime(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stats.MaxQueryTime < duration {
		p.stats.MaxQueryTime = duration
	}

	if p.stats.AverageQueryTime == 0 {
		p.stats.AverageQueryTime = duration
	} else {
		p.stats.AverageQueryTime = (p.stats.AverageQueryTime + duration) / 2
	}

	// 检查慢查询
	if p.config.SlowQueryThreshold > 0 && duration > p.config.SlowQueryThreshold {
		atomic.AddInt64(&p.stats.SlowQueries, 1)
	}
}

// IsClosed 检查连接池是否已关闭
func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.closed) == 1
}

// GetDB 获取底层数据库连接（用于不需要池化的操作）
func (p *Pool) GetDB() *sql.DB {
	return p.db
}

// SetMaxOpenConns 动态设置最大打开连接数
func (p *Pool) SetMaxOpenConns(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.MaxOpenConns = n
	p.db.SetMaxOpenConns(n)
	atomic.StoreInt32(&p.stats.MaxOpenConnections, int32(n))
}

// SetMaxIdleConns 动态设置最大空闲连接数
func (p *Pool) SetMaxIdleConns(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.MaxIdleConns = n
	p.db.SetMaxIdleConns(n)
}

// SetConnMaxLifetime 动态设置连接最大生命周期
func (p *Pool) SetConnMaxLifetime(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.MaxConnLifetime = d
	p.db.SetConnMaxLifetime(d)
}

// SetConnMaxIdleTime 动态设置连接最大空闲时间
func (p *Pool) SetConnMaxIdleTime(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.MaxConnIdleTime = d
	p.db.SetConnMaxIdleTime(d)
}

// 便利函数

// WithTransaction 在事务中执行函数
func (p *Pool) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	conn, err := p.Get(ctx)
	if err != nil {
		return err
	}
	defer p.Put(conn)

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Query 执行查询
func (p *Pool) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		p.updateQueryTime(duration)
	}()

	return p.db.QueryContext(ctx, query, args...)
}

// QueryRow 执行单行查询
func (p *Pool) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		p.updateQueryTime(duration)
	}()

	return p.db.QueryRowContext(ctx, query, args...)
}

// Exec 执行语句
func (p *Pool) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		p.updateQueryTime(duration)
	}()

	return p.db.ExecContext(ctx, query, args...)
}