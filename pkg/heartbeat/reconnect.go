// Package heartbeat 重连管理器实现
// Author: NetCore-Go Team
// Created: 2024

package heartbeat

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
)

// ReconnectStrategy 重连策略
type ReconnectStrategy int

const (
	FixedInterval ReconnectStrategy = iota // 固定间隔
	ExponentialBackoff                     // 指数退避
	LinearBackoff                          // 线性退避
	RandomJitter                           // 随机抖动
	CustomStrategy                         // 自定义策略
)

// String 返回策略名称
func (r ReconnectStrategy) String() string {
	switch r {
	case FixedInterval:
		return "fixed_interval"
	case ExponentialBackoff:
		return "exponential_backoff"
	case LinearBackoff:
		return "linear_backoff"
	case RandomJitter:
		return "random_jitter"
	case CustomStrategy:
		return "custom"
	default:
		return "unknown"
	}
}

// ReconnectConfig 重连配置
type ReconnectConfig struct {
	Strategy          ReconnectStrategy
	MaxAttempts       int           // 最大重连次数，0表示无限重连
	InitialInterval   time.Duration // 初始重连间隔
	MaxInterval       time.Duration // 最大重连间隔
	Multiplier        float64       // 退避倍数（用于指数退避）
	JitterRange       float64       // 抖动范围（0-1）
	Timeout           time.Duration // 单次重连超时时间
	Enabled           bool          // 是否启用重连
	ResetOnSuccess    bool          // 成功后是否重置计数器
	CustomIntervalFunc func(attempt int) time.Duration // 自定义间隔函数

	// 回调函数
	OnReconnectStart   func(attempt int, nextInterval time.Duration)
	OnReconnectSuccess func(attempt int, duration time.Duration)
	OnReconnectFailed  func(attempt int, err error)
	OnMaxAttemptsReached func(totalAttempts int)
}

// DefaultReconnectConfig 默认重连配置
func DefaultReconnectConfig() *ReconnectConfig {
	return &ReconnectConfig{
		Strategy:        ExponentialBackoff,
		MaxAttempts:     10,
		InitialInterval: 1 * time.Second,
		MaxInterval:     60 * time.Second,
		Multiplier:      2.0,
		JitterRange:     0.1,
		Timeout:         30 * time.Second,
		Enabled:         true,
		ResetOnSuccess:  true,
	}
}

// ReconnectStats 重连统计
type ReconnectStats struct {
	TotalAttempts     int           `json:"total_attempts"`
	SuccessfulReconnects int        `json:"successful_reconnects"`
	FailedAttempts    int           `json:"failed_attempts"`
	CurrentAttempt    int           `json:"current_attempt"`
	LastAttemptTime   time.Time     `json:"last_attempt_time"`
	LastSuccessTime   time.Time     `json:"last_success_time"`
	TotalDowntime     time.Duration `json:"total_downtime"`
	AverageReconnectTime float64    `json:"average_reconnect_time_ms"`
	LongestDowntime   time.Duration `json:"longest_downtime"`
	ShortestReconnectTime time.Duration `json:"shortest_reconnect_time"`
	IsReconnecting    bool          `json:"is_reconnecting"`
	NextAttemptIn     time.Duration `json:"next_attempt_in"`
}

// ConnectionFactory 连接工厂接口
type ConnectionFactory interface {
	CreateConnection() (core.Connection, error)
	ValidateConnection(conn core.Connection) error
	CloseConnection(conn core.Connection) error
}

// ReconnectManager 重连管理器
type ReconnectManager struct {
	config    *ReconnectConfig
	factory   ConnectionFactory
	connection core.Connection
	stats     *ReconnectStats
	ctx       context.Context
	cancel    context.CancelFunc
	mutex     sync.RWMutex
	running   int32
	reconnectTimes []time.Duration
	downtimeStart  time.Time
	timer     *time.Timer
}

// NewReconnectManager 创建重连管理器
func NewReconnectManager(factory ConnectionFactory, config *ReconnectConfig) *ReconnectManager {
	if config == nil {
		config = DefaultReconnectConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ReconnectManager{
		config:  config,
		factory: factory,
		stats: &ReconnectStats{
			ShortestReconnectTime: time.Duration(math.MaxInt64),
		},
		ctx:            ctx,
		cancel:         cancel,
		reconnectTimes: make([]time.Duration, 0, 100),
	}
}

// SetConnection 设置当前连接
func (r *ReconnectManager) SetConnection(conn core.Connection) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.connection = conn
	if conn != nil && conn.IsActive() {
		r.onReconnectSuccess()
	}
}

// GetConnection 获取当前连接
func (r *ReconnectManager) GetConnection() core.Connection {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.connection
}

// Start 启动重连管理器
func (r *ReconnectManager) Start() error {
	if !r.config.Enabled {
		return nil
	}

	if !atomic.CompareAndSwapInt32(&r.running, 0, 1) {
		return fmt.Errorf("reconnect manager already running")
	}

	// 如果当前没有连接或连接不活跃，开始重连
	if r.connection == nil || !r.connection.IsActive() {
		go r.startReconnect()
	}

	return nil
}

// Stop 停止重连管理器
func (r *ReconnectManager) Stop() {
	if !atomic.CompareAndSwapInt32(&r.running, 1, 0) {
		return
	}

	r.cancel()
	if r.timer != nil {
		r.timer.Stop()
	}
}

// IsRunning 检查是否正在运行
func (r *ReconnectManager) IsRunning() bool {
	return atomic.LoadInt32(&r.running) == 1
}

// TriggerReconnect 触发重连
func (r *ReconnectManager) TriggerReconnect() {
	if !r.IsRunning() {
		return
	}

	r.mutex.Lock()
	if !r.stats.IsReconnecting {
		r.stats.IsReconnecting = true
		r.downtimeStart = time.Now()
		go r.startReconnect()
	}
	r.mutex.Unlock()
}

// GetStats 获取统计信息
func (r *ReconnectManager) GetStats() *ReconnectStats {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 计算平均重连时间
	if len(r.reconnectTimes) > 0 {
		var total time.Duration
		for _, duration := range r.reconnectTimes {
			total += duration
		}
		r.stats.AverageReconnectTime = float64(total.Nanoseconds()) / float64(len(r.reconnectTimes)) / 1e6
	}

	// 深拷贝统计信息
	stats := *r.stats
	return &stats
}

// startReconnect 开始重连过程
func (r *ReconnectManager) startReconnect() {
	r.mutex.Lock()
	r.stats.IsReconnecting = true
	if r.downtimeStart.IsZero() {
		r.downtimeStart = time.Now()
	}
	r.mutex.Unlock()

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		r.mutex.Lock()
		currentAttempt := r.stats.CurrentAttempt + 1
		r.stats.CurrentAttempt = currentAttempt
		r.stats.TotalAttempts++
		r.stats.LastAttemptTime = time.Now()
		r.mutex.Unlock()

		// 检查是否超过最大重连次数
		if r.config.MaxAttempts > 0 && currentAttempt > r.config.MaxAttempts {
			r.onMaxAttemptsReached()
			return
		}

		// 计算重连间隔
		interval := r.calculateInterval(currentAttempt)

		// 调用重连开始回调
		if r.config.OnReconnectStart != nil {
			r.config.OnReconnectStart(currentAttempt, interval)
		}

		// 等待重连间隔
		if currentAttempt > 1 {
			r.mutex.Lock()
			r.stats.NextAttemptIn = interval
			r.mutex.Unlock()

			r.timer = time.NewTimer(interval)
			select {
			case <-r.ctx.Done():
				r.timer.Stop()
				return
			case <-r.timer.C:
				// 继续重连
			}
		}

		// 尝试重连
		start := time.Now()
		conn, err := r.attemptReconnect()
		duration := time.Since(start)

		if err == nil && conn != nil {
			// 重连成功
			r.mutex.Lock()
			r.connection = conn
			r.stats.SuccessfulReconnects++
			r.stats.LastSuccessTime = time.Now()
			r.stats.IsReconnecting = false
			r.stats.NextAttemptIn = 0

			// 更新重连时间统计
			r.reconnectTimes = append(r.reconnectTimes, duration)
			if len(r.reconnectTimes) > 100 {
				r.reconnectTimes = r.reconnectTimes[1:]
			}

			if duration < r.stats.ShortestReconnectTime {
				r.stats.ShortestReconnectTime = duration
			}

			// 更新停机时间
			if !r.downtimeStart.IsZero() {
				downtime := time.Since(r.downtimeStart)
				r.stats.TotalDowntime += downtime
				if downtime > r.stats.LongestDowntime {
					r.stats.LongestDowntime = downtime
				}
				r.downtimeStart = time.Time{}
			}

			// 重置计数器
			if r.config.ResetOnSuccess {
				r.stats.CurrentAttempt = 0
			}
			r.mutex.Unlock()

			// 调用成功回调
			if r.config.OnReconnectSuccess != nil {
				r.config.OnReconnectSuccess(currentAttempt, duration)
			}

			return
		} else {
			// 重连失败
			r.mutex.Lock()
			r.stats.FailedAttempts++
			r.mutex.Unlock()

			// 调用失败回调
			if r.config.OnReconnectFailed != nil {
				r.config.OnReconnectFailed(currentAttempt, err)
			}
		}
	}
}

// attemptReconnect 尝试重连
func (r *ReconnectManager) attemptReconnect() (core.Connection, error) {
	if r.factory == nil {
		return nil, fmt.Errorf("connection factory is nil")
	}

	// 创建上下文用于超时控制
	ctx, cancel := context.WithTimeout(r.ctx, r.config.Timeout)
	defer cancel()

	// 在goroutine中执行连接创建
	type result struct {
		conn core.Connection
		err  error
	}

	resultChan := make(chan result, 1)
	go func() {
		conn, err := r.factory.CreateConnection()
		if err == nil && conn != nil {
			// 验证连接
			if err = r.factory.ValidateConnection(conn); err != nil {
				r.factory.CloseConnection(conn)
				conn = nil
			}
		}
		resultChan <- result{conn: conn, err: err}
	}()

	// 等待结果或超时
	select {
	case res := <-resultChan:
		return res.conn, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("reconnect timeout after %v", r.config.Timeout)
	}
}

// calculateInterval 计算重连间隔
func (r *ReconnectManager) calculateInterval(attempt int) time.Duration {
	var interval time.Duration

	switch r.config.Strategy {
	case FixedInterval:
		interval = r.config.InitialInterval

	case ExponentialBackoff:
		interval = time.Duration(float64(r.config.InitialInterval) * math.Pow(r.config.Multiplier, float64(attempt-1)))

	case LinearBackoff:
		interval = time.Duration(int64(r.config.InitialInterval) * int64(attempt))

	case RandomJitter:
		base := r.config.InitialInterval
		jitter := time.Duration(float64(base) * r.config.JitterRange * (rand.Float64()*2 - 1))
		interval = base + jitter

	case CustomStrategy:
		if r.config.CustomIntervalFunc != nil {
			interval = r.config.CustomIntervalFunc(attempt)
		} else {
			interval = r.config.InitialInterval
		}

	default:
		interval = r.config.InitialInterval
	}

	// 确保间隔不超过最大值
	if interval > r.config.MaxInterval {
		interval = r.config.MaxInterval
	}

	// 确保间隔不小于0
	if interval < 0 {
		interval = r.config.InitialInterval
	}

	return interval
}

// onReconnectSuccess 重连成功处理
func (r *ReconnectManager) onReconnectSuccess() {
	r.stats.IsReconnecting = false
	r.stats.NextAttemptIn = 0

	if r.config.ResetOnSuccess {
		r.stats.CurrentAttempt = 0
	}

	// 更新停机时间
	if !r.downtimeStart.IsZero() {
		downtime := time.Since(r.downtimeStart)
		r.stats.TotalDowntime += downtime
		if downtime > r.stats.LongestDowntime {
			r.stats.LongestDowntime = downtime
		}
		r.downtimeStart = time.Time{}
	}
}

// onMaxAttemptsReached 达到最大重连次数处理
func (r *ReconnectManager) onMaxAttemptsReached() {
	r.mutex.Lock()
	r.stats.IsReconnecting = false
	r.stats.NextAttemptIn = 0
	totalAttempts := r.stats.TotalAttempts
	r.mutex.Unlock()

	// 调用回调函数
	if r.config.OnMaxAttemptsReached != nil {
		r.config.OnMaxAttemptsReached(totalAttempts)
	}

	// 停止重连管理器
	r.Stop()
}

// Reset 重置重连管理器
func (r *ReconnectManager) Reset() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.stats = &ReconnectStats{
		ShortestReconnectTime: time.Duration(math.MaxInt64),
	}
	r.reconnectTimes = make([]time.Duration, 0, 100)
	r.downtimeStart = time.Time{}
}

// UpdateConfig 更新配置
func (r *ReconnectManager) UpdateConfig(config *ReconnectConfig) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if config != nil {
		r.config = config
	}
}

// GetConfig 获取配置
func (r *ReconnectManager) GetConfig() *ReconnectConfig {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 深拷贝配置
	config := *r.config
	return &config
}