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

// ErrorHandler `n�?type ErrorHandler interface {
	HandleError(err error, ctx map[string]interface{})
	GetErrorCount() int64
	Reset()
}

// RecoveryManager `n�?type RecoveryManager struct {
	mu sync.RWMutex
	
	// `n�?	errorHandlers []ErrorHandler
	
	// `n
	recoveryStrategies map[string]RecoveryStrategy
	
	// `n�?	circuitBreakers map[string]*CircuitBreaker
	
	// `n
	retryConfig RetryConfig
	
	// `n
	stats RecoveryStats
	
	// `n
	config RecoveryConfig
	
	// `n
	onPanic    func(interface{}, []byte) // panic`n
	onError    func(error)              // `n
	onRecovery func(string)             // `n
}

// RecoveryConfig `n`ntype RecoveryConfig struct {
	EnablePanicRecovery bool          `json:"enable_panic_recovery"` // `npanic`n
	EnableErrorRetry    bool          `json:"enable_error_retry"`    // `n
	MaxRetryAttempts    int           `json:"max_retry_attempts"`    // `n�?	RetryInterval       time.Duration `json:"retry_interval"`        // `n
	CircuitBreakerThreshold int       `json:"circuit_breaker_threshold"` // `n�?	RecoveryTimeout     time.Duration `json:"recovery_timeout"`      // `n
	EnableMetrics       bool          `json:"enable_metrics"`        // `n
}

// RetryConfig `n`ntype RetryConfig struct {
	MaxAttempts   int           `json:"max_attempts"`   // `n�?	InitialDelay  time.Duration `json:"initial_delay"`  // `n
	MaxDelay      time.Duration `json:"max_delay"`      // `n�?	BackoffFactor float64       `json:"backoff_factor"` // `n�?	Jitter        bool          `json:"jitter"`         // `n
}

// RecoveryStats `n`ntype RecoveryStats struct {
	TotalPanics     int64 `json:"total_panics"`     // `npanic`n
	TotalErrors     int64 `json:"total_errors"`     // `n�?	TotalRecoveries int64 `json:"total_recoveries"` // `n�?	TotalRetries    int64 `json:"total_retries"`    // `n�?	SuccessfulRetries int64 `json:"successful_retries"` // `n
	FailedRetries   int64 `json:"failed_retries"`   // `n
	CircuitBreakerTrips int64 `json:"circuit_breaker_trips"` // `n�?}

// RecoveryStrategy `n`ntype RecoveryStrategy interface {
	Recover(ctx context.Context, err error) error
	CanRecover(err error) bool
	GetName() string
}

// CircuitBreaker `n�?type CircuitBreaker struct {
	mu sync.RWMutex
	
	name         string
	threshold    int           // `n�?	timeout      time.Duration // `n
	failureCount int64         // `n
	lastFailure  time.Time     // `n�?	state        CircuitState  // `n�?	
	// `n
	onStateChange func(string, CircuitState, CircuitState)
}

// CircuitState `n�?type CircuitState int

const (
	CircuitClosed CircuitState = iota // `n�?	CircuitOpen                       // `n�?	CircuitHalfOpen                   // `n�?)

// String `n`nfunc (s CircuitState) String() string {
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

// NewRecoveryManager `n�?func NewRecoveryManager(config RecoveryConfig) *RecoveryManager {
	// `n�?	if config.MaxRetryAttempts <= 0 {
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

// AddErrorHandler `n�?func (rm *RecoveryManager) AddErrorHandler(handler ErrorHandler) {
	rm.mu.Lock()
	rm.errorHandlers = append(rm.errorHandlers, handler)
	rm.mu.Unlock()
}

// AddRecoveryStrategy `n`nfunc (rm *RecoveryManager) AddRecoveryStrategy(name string, strategy RecoveryStrategy) {
	rm.mu.Lock()
	rm.recoveryStrategies[name] = strategy
	rm.mu.Unlock()
}

// GetCircuitBreaker `n�?func (rm *RecoveryManager) GetCircuitBreaker(name string) *CircuitBreaker {
	rm.mu.Lock()
	cb, exists := rm.circuitBreakers[name]
	if !exists {
		cb = NewCircuitBreaker(name, rm.config.CircuitBreakerThreshold, rm.config.RecoveryTimeout)
		rm.circuitBreakers[name] = cb
	}
	rm.mu.Unlock()
	return cb
}

// HandlePanic `npanic
func (rm *RecoveryManager) HandlePanic() {
	if !rm.config.EnablePanicRecovery {
		return
	}
	
	if r := recover(); r != nil {
		atomic.AddInt64(&rm.stats.TotalPanics, 1)
		
		// `n
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stackTrace := buf[:n]
		
		// `npanic`n`nif rm.onPanic != nil {
			rm.onPanic(r, stackTrace)
		}
		
		// `n
		err := fmt.Errorf("panic recovered: %v", r)
		rm.handleError(err, map[string]interface{}{
			"panic_value": r,
			"stack_trace": string(stackTrace),
		})
		
		atomic.AddInt64(&rm.stats.TotalRecoveries, 1)
		
		// `n`nif rm.onRecovery != nil {
			rm.onRecovery(fmt.Sprintf("panic: %v", r))
		}
	}
}

// HandleError `n`nfunc (rm *RecoveryManager) HandleError(err error, ctx map[string]interface{}) error {
	if err == nil {
		return nil
	}
	
	atomic.AddInt64(&rm.stats.TotalErrors, 1)
	
	// `n`nif rm.onError != nil {
		rm.onError(err)
	}
	
	return rm.handleError(err, ctx)
}

// handleError `n`nfunc (rm *RecoveryManager) handleError(err error, ctx map[string]interface{}) error {
	// `n�?	rm.mu.RLock()
	handlers := rm.errorHandlers
	rm.mu.RUnlock()
	
	for _, handler := range handlers {
		handler.HandleError(err, ctx)
	}
	
	// `n
	rm.mu.RLock()
	strategies := rm.recoveryStrategies
	rm.mu.RUnlock()
	
	for name, strategy := range strategies {
		if strategy.CanRecover(err) {
			ctx, cancel := context.WithTimeout(context.Background(), rm.config.RecoveryTimeout)
			defer cancel()
			
			if recoveryErr := strategy.Recover(ctx, err); recoveryErr == nil {
				atomic.AddInt64(&rm.stats.TotalRecoveries, 1)
				
				// `n`nif rm.onRecovery != nil {
					rm.onRecovery(fmt.Sprintf("strategy: %s", name))
				}
				
				return nil
			}
		}
	}
	
	return err
}

// RetryWithBackoff `n`nfunc (rm *RecoveryManager) RetryWithBackoff(ctx context.Context, operation func() error) error {
	if !rm.config.EnableErrorRetry {
		return operation()
	}
	
	var lastErr error
	delay := rm.retryConfig.InitialDelay
	
	for attempt := 0; attempt <= rm.retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			atomic.AddInt64(&rm.stats.TotalRetries, 1)
			
			// `n
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			
			// `n
			delay = time.Duration(float64(delay) * rm.retryConfig.BackoffFactor)
			if delay > rm.retryConfig.MaxDelay {
				delay = rm.retryConfig.MaxDelay
			}
			
			// `n`nif rm.retryConfig.Jitter {
				jitter := time.Duration(float64(delay) * 0.1)
				delay += time.Duration(time.Now().UnixNano()%int64(jitter))
			}
		}
		
		err := operation()
		if err == nil {
			if attempt > 0 {
				atomic.AddInt64(&rm.stats.SuccessfulRetries, 1)
			}
			return nil
		}
		
		lastErr = err
		
		// `n�?		if !rm.shouldRetry(err) {
			break
		}
	}
	
	if rm.retryConfig.MaxAttempts > 0 {
		atomic.AddInt64(&rm.stats.FailedRetries, 1)
	}
	
	return lastErr
}

// shouldRetry `n`nfunc (rm *RecoveryManager) shouldRetry(err error) bool {
	// `n
	// `n：`n，`n�?	return true // `n，`n
}

// SetCallbacks `n`nfunc (rm *RecoveryManager) SetCallbacks(
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

// GetStats `n`nfunc (rm *RecoveryManager) GetStats() RecoveryStats {
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

// ResetStats `n`nfunc (rm *RecoveryManager) ResetStats() {
	atomic.StoreInt64(&rm.stats.TotalPanics, 0)
	atomic.StoreInt64(&rm.stats.TotalErrors, 0)
	atomic.StoreInt64(&rm.stats.TotalRecoveries, 0)
	atomic.StoreInt64(&rm.stats.TotalRetries, 0)
	atomic.StoreInt64(&rm.stats.SuccessfulRetries, 0)
	atomic.StoreInt64(&rm.stats.FailedRetries, 0)
	atomic.StoreInt64(&rm.stats.CircuitBreakerTrips, 0)
}

// NewCircuitBreaker `n�?func NewCircuitBreaker(name string, threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:      name,
		threshold: threshold,
		timeout:   timeout,
		state:     CircuitClosed,
	}
}

// Call `n（`n）
func (cb *CircuitBreaker) Call(operation func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// `n�?	switch cb.state {
	case CircuitOpen:
		// `n`nif time.Since(cb.lastFailure) > cb.timeout {
			cb.setState(CircuitHalfOpen)
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	case CircuitHalfOpen:
		// `n，`n�?	}
	
	// `n
	err := operation()
	if err != nil {
		cb.onFailure()
		return err
	}
	
	cb.onSuccess()
	return nil
}

// onSuccess `n`nfunc (cb *CircuitBreaker) onSuccess() {
	if cb.state == CircuitHalfOpen {
		cb.setState(CircuitClosed)
		cb.failureCount = 0
	}
}

// onFailure `n`nfunc (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailure = time.Now()
	
	if cb.failureCount >= int64(cb.threshold) {
		cb.setState(CircuitOpen)
	}
}

// setState `n�?func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	
	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, oldState, newState)
	}
}

// GetState `n�?func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()
	return state
}

// GetFailureCount `n`nfunc (cb *CircuitBreaker) GetFailureCount() int64 {
	cb.mu.RLock()
	count := cb.failureCount
	cb.mu.RUnlock()
	return count
}

// Reset `n�?func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	cb.failureCount = 0
	cb.setState(CircuitClosed)
	cb.mu.Unlock()
}

// SetStateChangeCallback `n�?func (cb *CircuitBreaker) SetStateChangeCallback(callback func(string, CircuitState, CircuitState)) {
	cb.mu.Lock()
	cb.onStateChange = callback
	cb.mu.Unlock()
}

// DefaultErrorHandler `n�?type DefaultErrorHandler struct {
	mu         sync.RWMutex
	errorCount int64
	logger     *Logger
}

// NewDefaultErrorHandler `n�?func NewDefaultErrorHandler(logger *Logger) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError `n`nfunc (h *DefaultErrorHandler) HandleError(err error, ctx map[string]interface{}) {
	atomic.AddInt64(&h.errorCount, 1)
	
	if h.logger != nil {
		logger := h.logger.WithError(err)
		for k, v := range ctx {
			logger = logger.WithField(k, v)
		}
		logger.Error("Error handled by recovery manager")
	}
}

// GetErrorCount `n`nfunc (h *DefaultErrorHandler) GetErrorCount() int64 {
	return atomic.LoadInt64(&h.errorCount)
}

// Reset `n`nfunc (h *DefaultErrorHandler) Reset() {
	atomic.StoreInt64(&h.errorCount, 0)
}

// FileRecoveryStrategy `n`ntype FileRecoveryStrategy struct {
	name string
}

// NewFileRecoveryStrategy `n`nfunc NewFileRecoveryStrategy() *FileRecoveryStrategy {
	return &FileRecoveryStrategy{
		name: "file_recovery",
	}
}

// Recover `n`nfunc (s *FileRecoveryStrategy) Recover(ctx context.Context, err error) error {
	// `n
	// `n：`n、`n`nreturn nil
}

// CanRecover `n`nfunc (s *FileRecoveryStrategy) CanRecover(err error) bool {
	// `n�?	// `n�?	return true
}

// GetName `n`nfunc (s *FileRecoveryStrategy) GetName() string {
	return s.name
}

// NetworkRecoveryStrategy `n`ntype NetworkRecoveryStrategy struct {
	name string
}

// NewNetworkRecoveryStrategy `n`nfunc NewNetworkRecoveryStrategy() *NetworkRecoveryStrategy {
	return &NetworkRecoveryStrategy{
		name: "network_recovery",
	}
}

// Recover `n`nfunc (s *NetworkRecoveryStrategy) Recover(ctx context.Context, err error) error {
	// `n
	// `n：`n、`n`nreturn nil
}

// CanRecover `n`nfunc (s *NetworkRecoveryStrategy) CanRecover(err error) bool {
	// `n�?	// `n�?	return true
}

// GetName `n`nfunc (s *NetworkRecoveryStrategy) GetName() string {
	return s.name
}






