// Package middleware 中间件实现
// Author: NetCore-Go Team
// Created: 2024

package middleware

import (
	"fmt"
	"log"
	"time"

	httpserver "github.com/netcore-go/pkg/http"
)

// LoggerMiddleware 日志中间件
type LoggerMiddleware struct{}

// NewLoggerMiddleware 创建日志中间件
func NewLoggerMiddleware() *LoggerMiddleware {
	return &LoggerMiddleware{}
}

// Handle 处理日志记录
func (m *LoggerMiddleware) Handle(ctx *httpserver.HTTPContext, resp *httpserver.HTTPResponse, next httpserver.HTTPHandler) {
	start := time.Now()
	method := ctx.Method()
	path := ctx.Path()

	// 执行下一个处理器
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}

	duration := time.Since(start)
	log.Printf("%s %s %d %v", method, path, resp.StatusCode, duration)
}

// Logger 返回日志中间件实例
func Logger() httpserver.HTTPMiddleware {
	return NewLoggerMiddleware()
}

// RecoveryMiddleware 恢复中间件
type RecoveryMiddleware struct{}

// NewRecoveryMiddleware 创建恢复中间件
func NewRecoveryMiddleware() *RecoveryMiddleware {
	return &RecoveryMiddleware{}
}

// Handle 处理panic恢复
func (m *RecoveryMiddleware) Handle(ctx *httpserver.HTTPContext, resp *httpserver.HTTPResponse, next httpserver.HTTPHandler) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Panic recovered: %v", err)
			resp.StatusCode = 500
			resp.StatusText = "Internal Server Error"
			resp.Headers["Content-Type"] = "application/json"
			resp.Body = []byte(fmt.Sprintf(`{"error":"Internal Server Error","message":"%v"}`, err))
		}
	}()

	// 执行下一个处理器
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// Recovery 返回恢复中间件实例
func Recovery() httpserver.HTTPMiddleware {
	return NewRecoveryMiddleware()
}

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
func (m *CORSMiddleware) Handle(ctx *httpserver.HTTPContext, resp *httpserver.HTTPResponse, next httpserver.HTTPHandler) {
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

	resp.Headers["Access-Control-Allow-Methods"] = m.joinStrings(m.AllowMethods, ", ")
	resp.Headers["Access-Control-Allow-Headers"] = m.joinStrings(m.AllowHeaders, ", ")

	if m.MaxAge > 0 {
		resp.Headers["Access-Control-Max-Age"] = fmt.Sprintf("%d", m.MaxAge)
	}

	// 处理预检请求
	if ctx.Method() == "OPTIONS" {
		resp.StatusCode = 204
		resp.StatusText = "No Content"
		resp.Body = []byte{}
		return
	}

	// 执行下一个处理器
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// isAllowedOrigin 检查是否允许的源
func (m *CORSMiddleware) isAllowedOrigin(origin string) bool {
	for _, allowedOrigin := range m.AllowOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}
	return false
}

// joinStrings 连接字符串
func (m *CORSMiddleware) joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// CORS 返回CORS中间件实例
func CORS() httpserver.HTTPMiddleware {
	return NewCORSMiddleware()
}

// RequestIDMiddleware 请求ID中间件
type RequestIDMiddleware struct{}

// NewRequestIDMiddleware 创建请求ID中间件
func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{}
}

// Handle 处理请求ID
func (m *RequestIDMiddleware) Handle(ctx *httpserver.HTTPContext, resp *httpserver.HTTPResponse, next httpserver.HTTPHandler) {
	// 生成或获取请求ID
	requestID := ctx.Header("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// 设置请求ID到上下文
	ctx.Set("request_id", requestID)

	// 设置响应头
	resp.Headers["X-Request-ID"] = requestID

	// 执行下一个处理器
	if next != nil {
		next.ServeHTTP(ctx, resp)
	}
}

// RequestID 返回请求ID中间件实例
func RequestID() httpserver.HTTPMiddleware {
	return NewRequestIDMiddleware()
}