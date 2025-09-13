// Package circuitbreaker 熔断器中间件实现
// Author: NetCore-Go Team
// Created: 2024

package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// State 熔断器状态
type State int32

const (
	StateClosed   State = iota // 关闭状态（正常）
	StateOpen                  // 开启状态（熔断）
	StateHalfOpen              // 半开状态（试探）
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker 熔断器接口
type CircuitBreaker interface {
	Execute(ctx context.Context, fn func() error) error
	ExecuteHTTP(next http.Handler) http.Handler
	GetState() State
	GetStats() *Stats
	Reset()
	ForceOpen()
	ForceClose()
}

// circuitBreaker 熔断器实现
type circuitBreaker struct {
	mu           sync.RWMutex
	config       *Config
	state        int32 // atomic
	stats        *Stats
	lastFailTime int64 // atomic, unix nano
	nextRetry    int64 // atomic, unix nano
	generation   int64 // atomic
	onStateChange func(from, to State)
}

// Config 熔断器配置
type Config struct {
	// 基础配置
	Name        string        `json:"name"`
	Timeout     time.Duration `json:"timeout"`
	MaxRequests uint32        `json:"max_requests"`

	// 失败阈值配置
	FailureThreshold    uint32  `json:"failure_threshold"`     // 失败次数阈值
	FailureRatio        float64 `json:"failure_ratio"`        // 失败率阈值 (0.0-1.0)
	MinRequestThreshold uint32  `json:"min_request_threshold"` // 最小请求数阈值

	// 时间配置
	SleepWindow     time.Duration `json:"sleep_window"`     // 熔断器开启后的等待时间
	RollingWindow   time.Duration `json:"rolling_window"`   // 滚动窗口时间
	RetryTimeout    time.Duration `json:"retry_timeout"`    // 重试超时时间

	// 半开状态配置
	HalfOpenMaxRequests uint32 `json:"half_open_max_requests"` // 半开状态最大请求数
	HalfOpenSuccessThreshold uint32 `json:"half_open_success_threshold"` // 半开状态成功阈值

	// 错误判断函数
	IsFailure func(error) bool `json:"-"`
	IsSuccess func(error) bool `json:"-"`

	// HTTP特定配置
	HTTPStatusCodes []int `json:"http_status_codes"` // 被认为是失败的HTTP状态码

	// 回调函数
	OnStateChange func(from, to State) `json:"-"`
	OnOpen        func()               `json:"-"`
	OnHalfOpen    func()               `json:"-"`
	OnClose       func()               `json:"-"`
}

// Stats 熔断器统计信息
type Stats struct {
	mu                    sync.RWMutex
	Requests              uint64        `json:"requests"`
	Successes             uint64        `json:"successes"`
	Failures              uint64        `json:"failures"`
	Timeouts              uint64        `json:"timeouts"`
	CircuitBreakerErrors  uint64        `json:"circuit_breaker_errors"`
	TotalDuration         time.Duration `json:"total_duration"`
	AverageDuration       time.Duration `json:"average_duration"`
	MaxDuration           time.Duration `json:"max_duration"`
	MinDuration           time.Duration `json:"min_duration"`
	LastRequestTime       time.Time     `json:"last_request_time"`
	LastFailureTime       time.Time     `json:"last_failure_time"`
	LastSuccessTime       time.Time     `json:"last_success_time"`
	StateChanges          uint64        `json:"state_changes"`
	CurrentGeneration     int64         `json:"current_generation"`
	ConsecutiveFailures   uint32        `json:"consecutive_failures"`
	ConsecutiveSuccesses  uint32        `json:"consecutive_successes"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Name:                     "default",
		Timeout:                  30 * time.Second,
		MaxRequests:              100,
		FailureThreshold:         5,
		FailureRatio:             0.5,
		MinRequestThreshold:      10,
		SleepWindow:              60 * time.Second,
		RollingWindow:            60 * time.Second,
		RetryTimeout:             5 * time.Second,
		HalfOpenMaxRequests:      3,
		HalfOpenSuccessThreshold: 2,
		HTTPStatusCodes:          []int{500, 502, 503, 504},
		IsFailure: func(err error) bool {
			return err != nil
		},
		IsSuccess: func(err error) bool {
			return err == nil
		},
	}
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(config *Config) CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}

	// 验证配置
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.FailureRatio <= 0 || config.FailureRatio > 1 {
		config.FailureRatio = 0.5
	}
	if config.SleepWindow <= 0 {
		config.SleepWindow = 60 * time.Second
	}
	if config.HalfOpenMaxRequests == 0 {
		config.HalfOpenMaxRequests = 3
	}

	cb := &circuitBreaker{
		config:        config,
		state:         int32(StateClosed),
		stats:         &Stats{MinDuration: time.Duration(^uint64(0) >> 1)},
		onStateChange: config.OnStateChange,
	}

	return cb
}

// Execute 执行函数
func (cb *circuitBreaker) Execute(ctx context.Context, fn func() error) error {
	generation := atomic.LoadInt64(&cb.generation)
	state := State(atomic.LoadInt32(&cb.state))

	// 检查是否允许执行
	if !cb.allowRequest(state) {
		cb.recordFailure(ErrCircuitBreakerOpen)
		return ErrCircuitBreakerOpen
	}

	// 创建超时上下文
	var cancel context.CancelFunc
	if cb.config.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cb.config.Timeout)
		defer cancel()
	}

	start := time.Now()
	var err error

	// 在goroutine中执行函数以支持超时
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = fn()
	}()

	// 等待执行完成或超时
	select {
	case <-done:
		// 函数执行完成
	case <-ctx.Done():
		// 超时或取消
		err = ctx.Err()
	}

	duration := time.Since(start)

	// 记录结果
	if cb.isSuccess(err) {
		cb.recordSuccess(duration, generation)
	} else {
		cb.recordFailure(err)
	}

	return err
}

// ExecuteHTTP 执行HTTP请求
func (cb *circuitBreaker) ExecuteHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		generation := atomic.LoadInt64(&cb.generation)
		state := State(atomic.LoadInt32(&cb.state))

		// 检查是否允许执行
		if !cb.allowRequest(state) {
			cb.recordFailure(ErrCircuitBreakerOpen)
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}

		// 创建响应记录器
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		start := time.Now()

		// 执行请求
		next.ServeHTTP(recorder, r)

		duration := time.Since(start)

		// 判断是否成功
		if cb.isHTTPSuccess(recorder.statusCode) {
			cb.recordSuccess(duration, generation)
		} else {
			cb.recordFailure(fmt.Errorf("HTTP %d", recorder.statusCode))
		}
	})
}

// allowRequest 检查是否允许请求
func (cb *circuitBreaker) allowRequest(state State) bool {
	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以进入半开状态
		return cb.canEnterHalfOpen()
	case StateHalfOpen:
		// 半开状态下限制请求数
		cb.stats.mu.RLock()
		requests := cb.stats.Requests
		cb.stats.mu.RUnlock()
		return requests < uint64(cb.config.HalfOpenMaxRequests)
	default:
		return false
	}
}

// canEnterHalfOpen 检查是否可以进入半开状态
func (cb *circuitBreaker) canEnterHalfOpen() bool {
	now := time.Now().UnixNano()
	lastFailTime := atomic.LoadInt64(&cb.lastFailTime)
	return now-lastFailTime >= cb.config.SleepWindow.Nanoseconds()
}

// isSuccess 判断是否成功
func (cb *circuitBreaker) isSuccess(err error) bool {
	if cb.config.IsSuccess != nil {
		return cb.config.IsSuccess(err)
	}
	return err == nil
}

// isHTTPSuccess 判断HTTP响应是否成功
func (cb *circuitBreaker) isHTTPSuccess(statusCode int) bool {
	// 检查是否在失败状态码列表中
	for _, code := range cb.config.HTTPStatusCodes {
		if statusCode == code {
			return false
		}
	}
	// 2xx和3xx认为是成功
	return statusCode >= 200 && statusCode < 400
}

// recordSuccess 记录成功
func (cb *circuitBreaker) recordSuccess(duration time.Duration, generation int64) {
	cb.stats.mu.Lock()
	cb.stats.Requests++
	cb.stats.Successes++
	cb.stats.ConsecutiveSuccesses++
	cb.stats.ConsecutiveFailures = 0
	cb.stats.LastRequestTime = time.Now()
	cb.stats.LastSuccessTime = time.Now()
	cb.updateDuration(duration)
	cb.stats.mu.Unlock()

	state := State(atomic.LoadInt32(&cb.state))

	switch state {
	case StateHalfOpen:
		// 半开状态下检查是否可以关闭熔断器
		if cb.stats.ConsecutiveSuccesses >= cb.config.HalfOpenSuccessThreshold {
			cb.setState(StateClosed, generation)
		}
	case StateOpen:
		// 开启状态下转为半开状态
		cb.setState(StateHalfOpen, generation)
	}
}

// recordFailure 记录失败
func (cb *circuitBreaker) recordFailure(err error) {
	cb.stats.mu.Lock()
	cb.stats.Requests++
	cb.stats.Failures++
	cb.stats.ConsecutiveFailures++
	cb.stats.ConsecutiveSuccesses = 0
	cb.stats.LastRequestTime = time.Now()
	cb.stats.LastFailureTime = time.Now()

	if errors.Is(err, context.DeadlineExceeded) {
		cb.stats.Timeouts++
	} else if errors.Is(err, ErrCircuitBreakerOpen) {
		cb.stats.CircuitBreakerErrors++
	}
	cb.stats.mu.Unlock()

	atomic.StoreInt64(&cb.lastFailTime, time.Now().UnixNano())

	state := State(atomic.LoadInt32(&cb.state))
	generation := atomic.LoadInt64(&cb.generation)

	switch state {
	case StateClosed:
		// 检查是否需要开启熔断器
		if cb.shouldOpen() {
			cb.setState(StateOpen, generation)
		}
	case StateHalfOpen:
		// 半开状态下失败则重新开启
		cb.setState(StateOpen, generation)
	}
}

// shouldOpen 检查是否应该开启熔断器
func (cb *circuitBreaker) shouldOpen() bool {
	cb.stats.mu.RLock()
	defer cb.stats.mu.RUnlock()

	// 检查最小请求数
	if cb.stats.Requests < uint64(cb.config.MinRequestThreshold) {
		return false
	}

	// 检查连续失败次数
	if cb.stats.ConsecutiveFailures >= cb.config.FailureThreshold {
		return true
	}

	// 检查失败率
	if cb.stats.Requests > 0 {
		failureRatio := float64(cb.stats.Failures) / float64(cb.stats.Requests)
		return failureRatio >= cb.config.FailureRatio
	}

	return false
}

// setState 设置状态
func (cb *circuitBreaker) setState(newState State, generation int64) {
	oldState := State(atomic.SwapInt32(&cb.state, int32(newState)))

	if oldState != newState {
		// 更新代数
		atomic.AddInt64(&cb.generation, 1)

		// 重置统计信息
		if newState == StateClosed {
			cb.resetStats()
		}

		// 更新状态变更计数
		cb.stats.mu.Lock()
		cb.stats.StateChanges++
		cb.stats.CurrentGeneration = atomic.LoadInt64(&cb.generation)
		cb.stats.mu.Unlock()

		// 调用回调函数
		if cb.onStateChange != nil {
			cb.onStateChange(oldState, newState)
		}

		switch newState {
		case StateOpen:
			if cb.config.OnOpen != nil {
				cb.config.OnOpen()
			}
		case StateHalfOpen:
			if cb.config.OnHalfOpen != nil {
				cb.config.OnHalfOpen()
			}
		case StateClosed:
			if cb.config.OnClose != nil {
				cb.config.OnClose()
			}
		}
	}
}

// updateDuration 更新持续时间统计
func (cb *circuitBreaker) updateDuration(duration time.Duration) {
	cb.stats.TotalDuration += duration

	if duration > cb.stats.MaxDuration {
		cb.stats.MaxDuration = duration
	}
	if duration < cb.stats.MinDuration {
		cb.stats.MinDuration = duration
	}

	if cb.stats.Requests > 0 {
		cb.stats.AverageDuration = cb.stats.TotalDuration / time.Duration(cb.stats.Requests)
	}
}

// resetStats 重置统计信息
func (cb *circuitBreaker) resetStats() {
	cb.stats.mu.Lock()
	defer cb.stats.mu.Unlock()

	cb.stats.Requests = 0
	cb.stats.Successes = 0
	cb.stats.Failures = 0
	cb.stats.Timeouts = 0
	cb.stats.ConsecutiveFailures = 0
	cb.stats.ConsecutiveSuccesses = 0
	cb.stats.TotalDuration = 0
	cb.stats.AverageDuration = 0
	cb.stats.MaxDuration = 0
	cb.stats.MinDuration = time.Duration(^uint64(0) >> 1)
}

// GetState 获取当前状态
func (cb *circuitBreaker) GetState() State {
	return State(atomic.LoadInt32(&cb.state))
}

// GetStats 获取统计信息
func (cb *circuitBreaker) GetStats() *Stats {
	cb.stats.mu.RLock()
	defer cb.stats.mu.RUnlock()

	// 创建副本
	stats := *cb.stats
	return &stats
}

// Reset 重置熔断器
func (cb *circuitBreaker) Reset() {
	cb.setState(StateClosed, atomic.LoadInt64(&cb.generation))
	cb.resetStats()
}

// ForceOpen 强制开启熔断器
func (cb *circuitBreaker) ForceOpen() {
	cb.setState(StateOpen, atomic.LoadInt64(&cb.generation))
}

// ForceClose 强制关闭熔断器
func (cb *circuitBreaker) ForceClose() {
	cb.setState(StateClosed, atomic.LoadInt64(&cb.generation))
}

// responseRecorder HTTP响应记录器
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// 错误定义
var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrTimeout            = errors.New("request timeout")
)

// CircuitBreakerManager 熔断器管理器
type CircuitBreakerManager struct {
	mu      sync.RWMutex
	breakers map[string]CircuitBreaker
}

// NewCircuitBreakerManager 创建熔断器管理器
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]CircuitBreaker),
	}
}

// GetOrCreate 获取或创建熔断器
func (m *CircuitBreakerManager) GetOrCreate(name string, config *Config) CircuitBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if breaker, exists := m.breakers[name]; exists {
		return breaker
	}

	// 创建新的熔断器
	if config == nil {
		config = DefaultConfig()
	}
	config.Name = name

	breaker = NewCircuitBreaker(config)
	m.breakers[name] = breaker

	return breaker
}

// Get 获取熔断器
func (m *CircuitBreakerManager) Get(name string) (CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	breaker, exists := m.breakers[name]
	return breaker, exists
}

// Remove 移除熔断器
func (m *CircuitBreakerManager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.breakers, name)
}

// List 列出所有熔断器
func (m *CircuitBreakerManager) List() map[string]CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]CircuitBreaker)
	for name, breaker := range m.breakers {
		result[name] = breaker
	}
	return result
}

// GetAllStats 获取所有熔断器统计信息
func (m *CircuitBreakerManager) GetAllStats() map[string]*Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Stats)
	for name, breaker := range m.breakers {
		result[name] = breaker.GetStats()
	}
	return result
}

// 全局熔断器管理器
var globalManager = NewCircuitBreakerManager()

// GetGlobalManager 获取全局熔断器管理器
func GetGlobalManager() *CircuitBreakerManager {
	return globalManager
}

// 便利函数

// Execute 使用全局管理器执行函数
func Execute(name string, fn func() error) error {
	breaker := globalManager.GetOrCreate(name, nil)
	return breaker.Execute(context.Background(), fn)
}

// ExecuteWithConfig 使用指定配置执行函数
func ExecuteWithConfig(name string, config *Config, fn func() error) error {
	breaker := globalManager.GetOrCreate(name, config)
	return breaker.Execute(context.Background(), fn)
}

// HTTPMiddleware 创建HTTP中间件
func HTTPMiddleware(name string, config *Config) func(http.Handler) http.Handler {
	breaker := globalManager.GetOrCreate(name, config)
	return func(next http.Handler) http.Handler {
		return breaker.ExecuteHTTP(next)
	}
}