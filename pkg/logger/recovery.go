// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	HandleError(err error, ctx map[string]interface{})
	GetErrorCount() int64
	Reset()
}

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	mu sync.RWMutex
	
	// 错误处理器列表
	errorHandlers []ErrorHandler
	
	// 恢复策略
	recoveryStrategies map[string]RecoveryStrategy
	
	// 熔断器
	circuitBreakers map[string]*CircuitBreaker
	
	// 重试配置
	retryConfig RetryConfig
	
	// 统计信息
	stats RecoveryStats
	
	// 配置
	config RecoveryConfig
	
	// 回调函数
	onPanic    func(interface{}, []byte) // panic回调
	onError    func(error)              // 错误回调
	onRecovery func(string)             // 恢复回调
}

// RecoveryConfig 恢复配置
type RecoveryConfig struct {
	EnablePanicRecovery     bool          `json:"enable_panic_recovery"`     // 启用panic恢复
	EnableErrorRetry        bool          `json:"enable_error_retry"`        // 启用错误重试
	MaxRetryAttempts        int           `json:"max_retry_attempts"`        // 最大重试次数
	RetryInterval           time.Duration `json:"retry_interval"`            // 重试间隔
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold"` // 熔断器阈值
	RecoveryTimeout         time.Duration `json:"recovery_timeout"`          // 恢复超时
	EnableMetrics           bool          `json:"enable_metrics"`            // 启用指标
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts   int           `json:"max_attempts"`   // 最大尝试次数
	InitialDelay  time.Duration `json:"initial_delay"`  // 初始延迟
	MaxDelay      time.Duration `json:"max_delay"`      // 最大延迟
	BackoffFactor float64       `json:"backoff_factor"` // 退避因子
	Jitter        bool          `json:"jitter"`         // 抖动
}

// RecoveryStats 恢复统计
type RecoveryStats struct {
	TotalPanics           int64 `json:"total_panics"`           // 总panic数
	TotalErrors           int64 `json:"total_errors"`           // 总错误数
	TotalRecoveries       int64 `json:"total_recoveries"`       // 总恢复数
	TotalRetries          int64 `json:"total_retries"`          // 总重试数
	SuccessfulRetries     int64 `json:"successful_retries"`     // 成功重试数
	FailedRetries         int64 `json:"failed_retries"`         // 失败重试数
	CircuitBreakerTrips   int64 `json:"circuit_breaker_trips"`   // 熔断器触发数
}

// RecoveryStrategy 恢复策略接口
type RecoveryStrategy interface {
	Recover(ctx context.Context, err error) error
	CanRecover(err error) bool
	GetName() string
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu sync.RWMutex
	
	name         string
	threshold    int           // 失败阈值
	timeout      time.Duration // 超时时间
	failureCount int64         // 失败计数
	lastFailure  time.Time     // 最后失败时间
	state        CircuitState  // 熔断器状态
	
	// 状态变化回调
	onStateChange func(string, CircuitState, CircuitState)
}

// CircuitState 熔断器状态
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // 关闭状态
	CircuitOpen                         // 开启状态
	CircuitHalfOpen                     // 半开状态
)

// String 返回状态字符串
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// NewRecoveryManager 创建恢复管理器
func NewRecoveryManager(config RecoveryConfig) *RecoveryManager {
	// 设置默认值
	if config.MaxRetryAttempts <= 0 {
		config.MaxRetryAttempts = 3
	}
	if config.RetryInterval <= 0 {
		config.RetryInterval = time.Second
	}
	if config.CircuitBreakerThreshold <= 0 {
		config.CircuitBreakerThreshold = 5
	}
	if config.RecoveryTimeout <= 0 {
		config.RecoveryTimeout = 30 * time.Second
	}
	
	return &RecoveryManager{
		recoveryStrategies: make(map[string]RecoveryStrategy),
		circuitBreakers:    make(map[string]*CircuitBreaker),
		config:             config,
		retryConfig: RetryConfig{
			MaxAttempts:   config.MaxRetryAttempts,
			InitialDelay:  config.RetryInterval,
			MaxDelay:      config.RetryInterval * 10,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
	}
}

// AddErrorHandler 添加错误处理器
func (rm *RecoveryManager) AddErrorHandler(handler ErrorHandler) {
	rm.mu.Lock()
	rm.errorHandlers = append(rm.errorHandlers, handler)
	rm.mu.Unlock()
}

// AddRecoveryStrategy 添加恢复策略
func (rm *RecoveryManager) AddRecoveryStrategy(name string, strategy RecoveryStrategy) {
	rm.mu.Lock()
	rm.recoveryStrategies[name] = strategy
	rm.mu.Unlock()
}

// GetCircuitBreaker 获取熔断器
func (rm *RecoveryManager) GetCircuitBreaker(name string) *CircuitBreaker {
	rm.mu.Lock()
	cb, exists := rm.circuitBreakers[name]
	if !exists {
		cb = NewCircuitBreaker(name, rm.config.CircuitBreakerThreshold, rm.config.RecoveryTimeout)
		rm.circuitBreakers[name] = cb
	}
	rm.mu.Unlock()
	return cb
}

// HandlePanic 处理panic
func (rm *RecoveryManager) HandlePanic() {
	if !rm.config.EnablePanicRecovery {
		return
	}
	
	if r := recover(); r != nil {
		atomic.AddInt64(&rm.stats.TotalPanics, 1)
		
		// 获取堆栈信息
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stackTrace := buf[:n]
		
		// 调用panic回调
		if rm.onPanic != nil {
			rm.onPanic(r, stackTrace)
		}
		
		// 处理错误
		err := fmt.Errorf("panic recovered: %v", r)
		rm.handleError(err, map[string]interface{}{
			"panic_value": r,
			"stack_trace": string(stackTrace),
		})
		
		atomic.AddInt64(&rm.stats.TotalRecoveries, 1)
		
		// 调用恢复回调
		if rm.onRecovery != nil {
			rm.onRecovery(fmt.Sprintf("panic: %v", r))
		}
	}
}

// HandleError 处理错误
func (rm *RecoveryManager) HandleError(err error, ctx map[string]interface{}) error {
	if err == nil {
		return nil
	}
	
	atomic.AddInt64(&rm.stats.TotalErrors, 1)
	
	// 调用错误回调
	if rm.onError != nil {
		rm.onError(err)
	}
	
	return rm.handleError(err, ctx)
}

// handleError 内部错误处理
func (rm *RecoveryManager) handleError(err error, ctx map[string]interface{}) error {
	// 调用错误处理器
	rm.mu.RLock()
	handlers := make([]ErrorHandler, len(rm.errorHandlers))
	copy(handlers, rm.errorHandlers)
	rm.mu.RUnlock()
	
	for _, handler := range handlers {
		handler.HandleError(err, ctx)
	}
	
	// 尝试恢复策略
	if rm.config.EnableErrorRetry {
		return rm.tryRecovery(err, ctx)
	}
	
	return err
}

// tryRecovery 尝试恢复
func (rm *RecoveryManager) tryRecovery(err error, ctx map[string]interface{}) error {
	rm.mu.RLock()
	strategies := make(map[string]RecoveryStrategy)
	for k, v := range rm.recoveryStrategies {
		strategies[k] = v
	}
	rm.mu.RUnlock()
	
	for name, strategy := range strategies {
		if strategy.CanRecover(err) {
			ctx := context.Background()
			if recoveryErr := strategy.Recover(ctx, err); recoveryErr == nil {
				atomic.AddInt64(&rm.stats.SuccessfulRetries, 1)
				if rm.onRecovery != nil {
					rm.onRecovery(fmt.Sprintf("recovered using strategy: %s", name))
				}
				return nil
			}
			atomic.AddInt64(&rm.stats.FailedRetries, 1)
		}
	}
	
	return err
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:      name,
		threshold: threshold,
		timeout:   timeout,
		state:     CircuitClosed,
	}
}

// Call 调用熔断器保护的函数
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// 检查熔断器状态
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.setState(CircuitHalfOpen)
		} else {
			return fmt.Errorf("circuit breaker %s is open", cb.name)
		}
	}
	
	// 执行函数
	err := fn()
	if err != nil {
		cb.onFailure()
		return err
	}
	
	cb.onSuccess()
	return nil
}

// onSuccess 成功回调
func (cb *CircuitBreaker) onSuccess() {
	if cb.state == CircuitHalfOpen {
		cb.setState(CircuitClosed)
	}
	atomic.StoreInt64(&cb.failureCount, 0)
}

// onFailure 失败回调
func (cb *CircuitBreaker) onFailure() {
	atomic.AddInt64(&cb.failureCount, 1)
	cb.lastFailure = time.Now()
	
	if atomic.LoadInt64(&cb.failureCount) >= int64(cb.threshold) {
		cb.setState(CircuitOpen)
	}
}

// setState 设置状态
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	
	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, oldState, newState)
	}
}

// GetState 获取状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount 获取失败计数
func (cb *CircuitBreaker) GetFailureCount() int64 {
	return atomic.LoadInt64(&cb.failureCount)
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.StoreInt64(&cb.failureCount, 0)
	cb.setState(CircuitClosed)
}

// SetCallbacks 设置回调函数
func (rm *RecoveryManager) SetCallbacks(
	onPanic func(interface{}, []byte),
	onError func(error),
	onRecovery func(string),
) {
	rm.mu.Lock()
	rm.onPanic = onPanic
	rm.onError = onError
	rm.onRecovery = onRecovery
	rm.mu.Unlock()
}

// GetStats 获取统计信息
func (rm *RecoveryManager) GetStats() RecoveryStats {
	return RecoveryStats{
		TotalPanics:         atomic.LoadInt64(&rm.stats.TotalPanics),
		TotalErrors:         atomic.LoadInt64(&rm.stats.TotalErrors),
		TotalRecoveries:     atomic.LoadInt64(&rm.stats.TotalRecoveries),
		TotalRetries:        atomic.LoadInt64(&rm.stats.TotalRetries),
		SuccessfulRetries:   atomic.LoadInt64(&rm.stats.SuccessfulRetries),
		FailedRetries:       atomic.LoadInt64(&rm.stats.FailedRetries),
		CircuitBreakerTrips: atomic.LoadInt64(&rm.stats.CircuitBreakerTrips),
	}
}

// ResetStats 重置统计信息
func (rm *RecoveryManager) ResetStats() {
	atomic.StoreInt64(&rm.stats.TotalPanics, 0)
	atomic.StoreInt64(&rm.stats.TotalErrors, 0)
	atomic.StoreInt64(&rm.stats.TotalRecoveries, 0)
	atomic.StoreInt64(&rm.stats.TotalRetries, 0)
	atomic.StoreInt64(&rm.stats.SuccessfulRetries, 0)
	atomic.StoreInt64(&rm.stats.FailedRetries, 0)
	atomic.StoreInt64(&rm.stats.CircuitBreakerTrips, 0)
}






