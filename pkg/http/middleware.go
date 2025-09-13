// Package http HTTP中间件实现
// Author: NetCore-Go Team
// Created: 2024

package http

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// CORSMiddleware CORS中间件
type CORSMiddleware struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

// NewCORSMiddleware 创建CORS中间件
func NewCORSMiddleware() *CORSMiddleware {
	return &CORSMiddleware{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       86400, // 24小时
	}
}

// Handle 处理CORS
func (m *CORSMiddleware) Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler) {
	// 设置CORS头部
	origin := ctx.Header("Origin")
	if origin != "" && m.isAllowedOrigin(origin) {
		resp.Headers["Access-Control-Allow-Origin"] = origin
	} else if len(m.AllowOrigins) == 1 && m.AllowOrigins[0] == "*" {
		resp.Headers["Access-Control-Allow-Origin"] = "*"
	}

	if m.AllowCredentials {
		resp.Headers["Access-Control-Allow-Credentials"] = "true"
	}

	// 处理预检请求
	if ctx.Method() == "OPTIONS" {
		resp.Headers["Access-Control-Allow-Methods"] = strings.Join(m.AllowMethods, ", ")
		resp.Headers["Access-Control-Allow-Headers"] = strings.Join(m.AllowHeaders, ", ")
		resp.Headers["Access-Control-Max-Age"] = strconv.Itoa(m.MaxAge)
		resp.StatusCode = 204
		resp.StatusText = "No Content"
		resp.Body = []byte{}
		return
	}

	// 继续处理请求
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// isAllowedOrigin 检查是否允许的源
func (m *CORSMiddleware) isAllowedOrigin(origin string) bool {
	for _, allowed := range m.AllowOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// LoggingMiddleware 日志中间件
type LoggingMiddleware struct {
	logger *log.Logger
}

// NewLoggingMiddleware 创建日志中间件
func NewLoggingMiddleware(logger *log.Logger) *LoggingMiddleware {
	if logger == nil {
		logger = log.Default()
	}
	return &LoggingMiddleware{
		logger: logger,
	}
}

// Handle 处理日志记录
func (m *LoggingMiddleware) Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler) {
	start := time.Now()

	// 继续处理请求
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}

	// 记录日志
	duration := time.Since(start)
	m.logger.Printf("%s %s %d %v %s",
		ctx.Method(),
		ctx.Path(),
		resp.StatusCode,
		duration,
		ctx.Connection.RemoteAddr(),
	)
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	TokenValidator func(token string) (interface{}, error)
	SkipPaths      []string
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(validator func(string) (interface{}, error)) *AuthMiddleware {
	return &AuthMiddleware{
		TokenValidator: validator,
		SkipPaths:      []string{},
	}
}

// Handle 处理认证
func (m *AuthMiddleware) Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler) {
	// 检查是否跳过认证
	for _, skipPath := range m.SkipPaths {
		if ctx.Path() == skipPath {
			if next != nil {
				next.ServeHTTP(ctx, resp)
			}
			return
		}
	}

	// 获取Authorization头部
	auth := ctx.Header("Authorization")
	if auth == "" {
		ctx.Error(resp, 401, "Missing Authorization header")
		return
	}

	// 解析Bearer token
	if !strings.HasPrefix(auth, "Bearer ") {
		ctx.Error(resp, 401, "Invalid Authorization header format")
		return
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		ctx.Error(resp, 401, "Missing token")
		return
	}

	// 验证token
	user, err := m.TokenValidator(token)
	if err != nil {
		ctx.Error(resp, 401, "Invalid token")
		return
	}

	// 将用户信息存储到上下文
	ctx.Set("user", user)

	// 继续处理请求
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// RateLimitMiddleware 限流中间件
type RateLimitMiddleware struct {
	Requests int           // 请求数量
	Window   time.Duration // 时间窗口
	counters map[string]*rateLimitCounter
}

type rateLimitCounter struct {
	count     int
	lastReset time.Time
}

// NewRateLimitMiddleware 创建限流中间件
func NewRateLimitMiddleware(requests int, window time.Duration) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		Requests: requests,
		Window:   window,
		counters: make(map[string]*rateLimitCounter),
	}
}

// Handle 处理限流
func (m *RateLimitMiddleware) Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler) {
	clientIP := m.getClientIP(ctx)

	// 获取或创建计数器
	counter, exists := m.counters[clientIP]
	if !exists {
		counter = &rateLimitCounter{
			count:     0,
			lastReset: time.Now(),
		}
		m.counters[clientIP] = counter
	}

	// 检查是否需要重置计数器
	if time.Since(counter.lastReset) > m.Window {
		counter.count = 0
		counter.lastReset = time.Now()
	}

	// 检查是否超过限制
	if counter.count >= m.Requests {
		resp.Headers["X-RateLimit-Limit"] = strconv.Itoa(m.Requests)
		resp.Headers["X-RateLimit-Remaining"] = "0"
		resp.Headers["X-RateLimit-Reset"] = strconv.FormatInt(counter.lastReset.Add(m.Window).Unix(), 10)
		ctx.Error(resp, 429, "Too Many Requests")
		return
	}

	// 增加计数
	counter.count++

	// 设置限流头部
	resp.Headers["X-RateLimit-Limit"] = strconv.Itoa(m.Requests)
	resp.Headers["X-RateLimit-Remaining"] = strconv.Itoa(m.Requests - counter.count)
	resp.Headers["X-RateLimit-Reset"] = strconv.FormatInt(counter.lastReset.Add(m.Window).Unix(), 10)

	// 继续处理请求
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// getClientIP 获取客户端IP
func (m *RateLimitMiddleware) getClientIP(ctx *HTTPContext) string {
	// 检查X-Forwarded-For头部
	if xff := ctx.Header("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// 检查X-Real-IP头部
	if xri := ctx.Header("X-Real-IP"); xri != "" {
		return xri
	}

	// 使用连接的远程地址
	remoteAddr := ctx.Connection.RemoteAddr().String()
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// RecoveryMiddleware 恢复中间件
type RecoveryMiddleware struct {
	logger *log.Logger
}

// NewRecoveryMiddleware 创建恢复中间件
func NewRecoveryMiddleware(logger *log.Logger) *RecoveryMiddleware {
	if logger == nil {
		logger = log.Default()
	}
	return &RecoveryMiddleware{
		logger: logger,
	}
}

// Handle 处理panic恢复
func (m *RecoveryMiddleware) Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Printf("Panic recovered: %v", r)
			ctx.Error(resp, 500, "Internal Server Error")
		}
	}()

	// 继续处理请求
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// 函数形式的中间件，用于兼容性

// DefaultCORSMiddleware 返回默认CORS中间件函数
func DefaultCORSMiddleware() HTTPMiddleware {
	return NewCORSMiddleware()
}

// DefaultLoggerMiddleware 返回默认日志中间件函数
func DefaultLoggerMiddleware() HTTPMiddleware {
	return NewLoggingMiddleware(nil)
}

// DefaultRecoveryMiddleware 返回默认恢复中间件函数
func DefaultRecoveryMiddleware() HTTPMiddleware {
	return NewRecoveryMiddleware(nil)
}

// DefaultRequestIDMiddleware 返回默认请求ID中间件函数
func DefaultRequestIDMiddleware() HTTPMiddleware {
	return &RequestIDMiddlewareImpl{}
}

// RequestIDMiddlewareImpl 请求ID中间件实现
type RequestIDMiddlewareImpl struct{}

// Handle 处理请求ID
func (m *RequestIDMiddlewareImpl) Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler) {
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())
	resp.Headers["X-Request-ID"] = requestID
	ctx.Set("request_id", requestID)
	
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}