// Package core 实现NetCore-Go网络库的中间件系统
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"sync"
)

// BaseContext 基础上下文实现
type BaseContext struct {
	connection Connection
	message    Message
	values     map[string]interface{}
	mu         sync.RWMutex
	aborted    bool
}

// NewBaseContext 创建新的基础上下文
func NewBaseContext(conn Connection, msg Message) *BaseContext {
	return &BaseContext{
		connection: conn,
		message:    msg,
		values:     make(map[string]interface{}),
	}
}

// Connection 获取连接对象
func (c *BaseContext) Connection() Connection {
	return c.connection
}

// Message 获取消息对象
func (c *BaseContext) Message() Message {
	return c.message
}

// Set 设置上下文值
func (c *BaseContext) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

// Get 获取上下文值
func (c *BaseContext) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.values[key]
	return value, exists
}

// Abort 中止处理
func (c *BaseContext) Abort() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aborted = true
}

// IsAborted 检查是否已中止
func (c *BaseContext) IsAborted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aborted
}

// MiddlewareChain 中间件链
type MiddlewareChain struct {
	middlewares []Middleware
	index       int
}

// NewMiddlewareChain 创建新的中间件链
func NewMiddlewareChain(middlewares []Middleware) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
		index:       0,
	}
}

// Next 执行下一个中间件
func (chain *MiddlewareChain) Next(ctx Context) error {
	if chain.index >= len(chain.middlewares) {
		return nil
	}

	if ctx.IsAborted() {
		return nil
	}

	middleware := chain.middlewares[chain.index]
	chain.index++

	return middleware.Process(ctx, chain.Next)
}

// Execute 执行中间件链
func (chain *MiddlewareChain) Execute(ctx Context) error {
	chain.index = 0
	return chain.Next(ctx)
}

// BaseMiddleware 基础中间件实现
type BaseMiddleware struct {
	name     string
	priority int
	handler  func(ctx Context, next Handler) error
}

// NewBaseMiddleware 创建新的基础中间件
func NewBaseMiddleware(name string, priority int, handler func(ctx Context, next Handler) error) *BaseMiddleware {
	return &BaseMiddleware{
		name:     name,
		priority: priority,
		handler:  handler,
	}
}

// Name 获取中间件名称
func (m *BaseMiddleware) Name() string {
	return m.name
}

// Priority 获取中间件优先级
func (m *BaseMiddleware) Priority() int {
	return m.priority
}

// Process 处理请求
func (m *BaseMiddleware) Process(ctx Context, next Handler) error {
	return m.handler(ctx, next)
}

// 预定义的常用中间件

// LoggingMiddleware 日志中间件
func LoggingMiddleware() Middleware {
	return NewBaseMiddleware("logging", 100, func(ctx Context, next Handler) error {
		// 这里可以添加日志记录逻辑
		// fmt.Printf("[%s] Connection: %s, Message Type: %s\n", 
		//     time.Now().Format(time.RFC3339), 
		//     ctx.Connection().ID(), 
		//     ctx.Message().Type.String())
		return next(ctx)
	})
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(maxRequests int) Middleware {
	return NewBaseMiddleware("rate_limit", 90, func(ctx Context, next Handler) error {
		// 这里可以添加限流逻辑
		// 简单示例：检查连接的请求频率
		connID := ctx.Connection().ID()
		if value, exists := ctx.Get("request_count_" + connID); exists {
			if count, ok := value.(int); ok && count >= maxRequests {
				ctx.Abort()
				return nil
			} else {
				ctx.Set("request_count_"+connID, count+1)
			}
		} else {
			ctx.Set("request_count_"+connID, 1)
		}
		return next(ctx)
	})
}

// AuthMiddleware 认证中间件
func AuthMiddleware() Middleware {
	return NewBaseMiddleware("auth", 80, func(ctx Context, next Handler) error {
		// 这里可以添加认证逻辑
		// 检查消息头中的认证信息
		msg := ctx.Message()
		if token, exists := msg.GetHeader("Authorization"); exists {
			// 验证token
			if token == "" {
				ctx.Abort()
				return nil
			}
			ctx.Set("authenticated", true)
		} else {
			ctx.Set("authenticated", false)
		}
		return next(ctx)
	})
}

// MetricsMiddleware 监控中间件
func MetricsMiddleware() Middleware {
	return NewBaseMiddleware("metrics", 70, func(ctx Context, next Handler) error {
		// 这里可以添加监控指标收集逻辑
		// 记录请求处理时间、成功率等
		// start := time.Now()
		err := next(ctx)
		// duration := time.Since(start)
		// 记录指标
		return err
	})
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() Middleware {
	return NewBaseMiddleware("recovery", 60, func(ctx Context, next Handler) error {
		defer func() {
			if r := recover(); r != nil {
				// 记录panic信息
				// log.Printf("Panic recovered: %v", r)
				ctx.Abort()
			}
		}()
		return next(ctx)
	})
}