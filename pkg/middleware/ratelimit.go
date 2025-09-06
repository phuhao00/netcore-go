// Package middleware 高级限流中间件实现
// Author: NetCore-Go Team
// Created: 2024

package middleware

import (
	"fmt"
	"sync"
	"time"

	"../core"
)

// RateLimitAlgorithm 限流算法类型
type RateLimitAlgorithm int

const (
	TokenBucket RateLimitAlgorithm = iota
	LeakyBucket
	FixedWindow
	SlidingWindow
	SlidingLog
)

// String 返回算法名称
func (r RateLimitAlgorithm) String() string {
	switch r {
	case TokenBucket:
		return "token_bucket"
	case LeakyBucket:
		return "leaky_bucket"
	case FixedWindow:
		return "fixed_window"
	case SlidingWindow:
		return "sliding_window"
	case SlidingLog:
		return "sliding_log"
	default:
		return "unknown"
	}
}

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(key string) bool
	AllowN(key string, n int) bool
	Reset(key string)
	GetStats(key string) *RateLimitStats
}

// RateLimitStats 限流统计
type RateLimitStats struct {
	Key            string    `json:"key"`
	Algorithm      string    `json:"algorithm"`
	Limit          int       `json:"limit"`
	Remaining      int       `json:"remaining"`
	ResetTime      time.Time `json:"reset_time"`
	TotalRequests  int64     `json:"total_requests"`
	AllowedRequests int64    `json:"allowed_requests"`
	BlockedRequests int64    `json:"blocked_requests"`
	LastAccess     time.Time `json:"last_access"`
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	capacity   int           // 桶容量
	refillRate int           // 令牌补充速率（每秒）
	buckets    map[string]*tokenBucket
	mutex      sync.RWMutex
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
	stats      *RateLimitStats
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(capacity, refillRate int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		capacity:   capacity,
		refillRate: refillRate,
		buckets:    make(map[string]*tokenBucket),
	}
}

// Allow 检查是否允许请求
func (t *TokenBucketLimiter) Allow(key string) bool {
	return t.AllowN(key, 1)
}

// AllowN 检查是否允许N个请求
func (t *TokenBucketLimiter) AllowN(key string, n int) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	bucket, exists := t.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     t.capacity,
			lastRefill: time.Now(),
			stats: &RateLimitStats{
				Key:       key,
				Algorithm: "token_bucket",
				Limit:     t.capacity,
				ResetTime: time.Now().Add(time.Second),
			},
		}
		t.buckets[key] = bucket
	}

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * t.refillRate
	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > t.capacity {
			bucket.tokens = t.capacity
		}
		bucket.lastRefill = now
	}

	// 更新统计
	bucket.stats.TotalRequests++
	bucket.stats.LastAccess = now
	bucket.stats.Remaining = bucket.tokens

	// 检查是否有足够的令牌
	if bucket.tokens >= n {
		bucket.tokens -= n
		bucket.stats.AllowedRequests++
		bucket.stats.Remaining = bucket.tokens
		return true
	}

	bucket.stats.BlockedRequests++
	return false
}

// Reset 重置限流器
func (t *TokenBucketLimiter) Reset(key string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if bucket, exists := t.buckets[key]; exists {
		bucket.tokens = t.capacity
		bucket.lastRefill = time.Now()
	}
}

// GetStats 获取统计信息
func (t *TokenBucketLimiter) GetStats(key string) *RateLimitStats {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if bucket, exists := t.buckets[key]; exists {
		return bucket.stats
	}
	return nil
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	limit      int           // 限制数量
	windowSize time.Duration // 窗口大小
	windows    map[string]*slidingWindow
	mutex      sync.RWMutex
}

type slidingWindow struct {
	requests []time.Time
	stats    *RateLimitStats
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(limit int, windowSize time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		limit:      limit,
		windowSize: windowSize,
		windows:    make(map[string]*slidingWindow),
	}
}

// Allow 检查是否允许请求
func (s *SlidingWindowLimiter) Allow(key string) bool {
	return s.AllowN(key, 1)
}

// AllowN 检查是否允许N个请求
func (s *SlidingWindowLimiter) AllowN(key string, n int) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	window, exists := s.windows[key]
	if !exists {
		window = &slidingWindow{
			requests: make([]time.Time, 0),
			stats: &RateLimitStats{
				Key:       key,
				Algorithm: "sliding_window",
				Limit:     s.limit,
				ResetTime: time.Now().Add(s.windowSize),
			},
		}
		s.windows[key] = window
	}

	now := time.Now()
	cutoff := now.Add(-s.windowSize)

	// 清理过期请求
	validRequests := make([]time.Time, 0)
	for _, reqTime := range window.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	window.requests = validRequests

	// 更新统计
	window.stats.TotalRequests++
	window.stats.LastAccess = now
	window.stats.Remaining = s.limit - len(window.requests)

	// 检查是否超过限制
	if len(window.requests)+n <= s.limit {
		for i := 0; i < n; i++ {
			window.requests = append(window.requests, now)
		}
		window.stats.AllowedRequests++
		window.stats.Remaining = s.limit - len(window.requests)
		return true
	}

	window.stats.BlockedRequests++
	return false
}

// Reset 重置限流器
func (s *SlidingWindowLimiter) Reset(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if window, exists := s.windows[key]; exists {
		window.requests = make([]time.Time, 0)
	}
}

// GetStats 获取统计信息
func (s *SlidingWindowLimiter) GetStats(key string) *RateLimitStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if window, exists := s.windows[key]; exists {
		return window.stats
	}
	return nil
}

// FixedWindowLimiter 固定窗口限流器
type FixedWindowLimiter struct {
	limit      int           // 限制数量
	windowSize time.Duration // 窗口大小
	windows    map[string]*fixedWindow
	mutex      sync.RWMutex
}

type fixedWindow struct {
	count     int
	windowStart time.Time
	stats     *RateLimitStats
}

// NewFixedWindowLimiter 创建固定窗口限流器
func NewFixedWindowLimiter(limit int, windowSize time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		limit:      limit,
		windowSize: windowSize,
		windows:    make(map[string]*fixedWindow),
	}
}

// Allow 检查是否允许请求
func (f *FixedWindowLimiter) Allow(key string) bool {
	return f.AllowN(key, 1)
}

// AllowN 检查是否允许N个请求
func (f *FixedWindowLimiter) AllowN(key string, n int) bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	window, exists := f.windows[key]
	if !exists {
		window = &fixedWindow{
			count:       0,
			windowStart: time.Now(),
			stats: &RateLimitStats{
				Key:       key,
				Algorithm: "fixed_window",
				Limit:     f.limit,
			},
		}
		f.windows[key] = window
	}

	now := time.Now()

	// 检查是否需要重置窗口
	if now.Sub(window.windowStart) >= f.windowSize {
		window.count = 0
		window.windowStart = now
		window.stats.ResetTime = now.Add(f.windowSize)
	}

	// 更新统计
	window.stats.TotalRequests++
	window.stats.LastAccess = now
	window.stats.Remaining = f.limit - window.count

	// 检查是否超过限制
	if window.count+n <= f.limit {
		window.count += n
		window.stats.AllowedRequests++
		window.stats.Remaining = f.limit - window.count
		return true
	}

	window.stats.BlockedRequests++
	return false
}

// Reset 重置限流器
func (f *FixedWindowLimiter) Reset(key string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if window, exists := f.windows[key]; exists {
		window.count = 0
		window.windowStart = time.Now()
	}
}

// GetStats 获取统计信息
func (f *FixedWindowLimiter) GetStats(key string) *RateLimitStats {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	if window, exists := f.windows[key]; exists {
		return window.stats
	}
	return nil
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Algorithm    RateLimitAlgorithm
	Limit        int
	Window       time.Duration
	RefillRate   int // 仅用于令牌桶
	KeyGenerator func(ctx core.Context) string
	SkipPaths    []string
	OnBlocked    func(ctx core.Context, stats *RateLimitStats)
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Algorithm:  TokenBucket,
		Limit:      100,
		Window:     time.Minute,
		RefillRate: 10,
		KeyGenerator: func(ctx core.Context) string {
			// 默认使用连接ID作为key
			return ctx.Connection().ID()
		},
		SkipPaths: []string{"/health", "/metrics"},
		OnBlocked: func(ctx core.Context, stats *RateLimitStats) {
			// 默认处理：设置错误信息
			ctx.Set("rate_limit_error", fmt.Sprintf("Rate limit exceeded: %d/%d", stats.Limit-stats.Remaining, stats.Limit))
			ctx.Set("rate_limit_reset", stats.ResetTime)
		},
	}
}

// AdvancedRateLimitMiddleware 高级限流中间件
type AdvancedRateLimitMiddleware struct {
	config  *RateLimitConfig
	limiter RateLimiter
}

// NewAdvancedRateLimitMiddleware 创建高级限流中间件
func NewAdvancedRateLimitMiddleware(config *RateLimitConfig) *AdvancedRateLimitMiddleware {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	var limiter RateLimiter
	switch config.Algorithm {
	case TokenBucket:
		limiter = NewTokenBucketLimiter(config.Limit, config.RefillRate)
	case SlidingWindow:
		limiter = NewSlidingWindowLimiter(config.Limit, config.Window)
	case FixedWindow:
		limiter = NewFixedWindowLimiter(config.Limit, config.Window)
	default:
		limiter = NewTokenBucketLimiter(config.Limit, config.RefillRate)
	}

	return &AdvancedRateLimitMiddleware{
		config:  config,
		limiter: limiter,
	}
}

// Process 处理限流
func (m *AdvancedRateLimitMiddleware) Process(ctx core.Context, next core.Handler) error {
	// 检查是否跳过限流
	if m.shouldSkip(ctx) {
		return next(ctx)
	}

	// 生成限流key
	key := m.config.KeyGenerator(ctx)

	// 检查是否允许请求
	if !m.limiter.Allow(key) {
		// 请求被限流
		stats := m.limiter.GetStats(key)
		if m.config.OnBlocked != nil {
			m.config.OnBlocked(ctx, stats)
		}
		ctx.Abort()
		return fmt.Errorf("rate limit exceeded")
	}

	// 将限流统计信息添加到上下文
	stats := m.limiter.GetStats(key)
	if stats != nil {
		ctx.Set("rate_limit_stats", stats)
		ctx.Set("rate_limit_remaining", stats.Remaining)
		ctx.Set("rate_limit_limit", stats.Limit)
		ctx.Set("rate_limit_reset", stats.ResetTime)
	}

	return next(ctx)
}

// Name 获取中间件名称
func (m *AdvancedRateLimitMiddleware) Name() string {
	return "advanced_rate_limit"
}

// Priority 获取中间件优先级
func (m *AdvancedRateLimitMiddleware) Priority() int {
	return 90
}

// shouldSkip 检查是否应该跳过限流
func (m *AdvancedRateLimitMiddleware) shouldSkip(ctx core.Context) bool {
	// 检查路径是否在跳过列表中
	if path, exists := ctx.Get("request_path"); exists {
		if pathStr, ok := path.(string); ok {
			for _, skipPath := range m.config.SkipPaths {
				if pathStr == skipPath {
					return true
				}
			}
		}
	}
	return false
}

// GetStats 获取限流统计
func (m *AdvancedRateLimitMiddleware) GetStats(key string) *RateLimitStats {
	return m.limiter.GetStats(key)
}

// Reset 重置限流器
func (m *AdvancedRateLimitMiddleware) Reset(key string) {
	m.limiter.Reset(key)
}

// GetAlgorithm 获取限流算法
func (m *AdvancedRateLimitMiddleware) GetAlgorithm() RateLimitAlgorithm {
	return m.config.Algorithm
}