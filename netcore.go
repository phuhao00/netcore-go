// Package netcore 提供NetCore-Go高性能网络库的统一API接口
// Author: NetCore-Go Team
// Created: 2024

package netcore

import (
	"time"

	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/http"
	"github.com/netcore-go/pkg/tcp"
	"github.com/netcore-go/pkg/udp"
	"github.com/netcore-go/pkg/websocket"
)

// 导出核心类型和接口
type (
	// Server 服务器接口
	Server = core.Server
	// Connection 连接接口
	Connection = core.Connection
	// MessageHandler 消息处理器接口
	MessageHandler = core.MessageHandler
	// Middleware 中间件接口
	Middleware = core.Middleware
	// Context 上下文接口
	Context = core.Context
	// Handler 处理器函数类型
	Handler = core.Handler

	// Message 消息结构体
	Message = core.Message
	// MessageType 消息类型
	MessageType = core.MessageType
	// ConnectionState 连接状态
	ConnectionState = core.ConnectionState
	// ServerStats 服务器统计信息
	ServerStats = core.ServerStats
	// ServerConfig 服务器配置
	ServerConfig = core.ServerConfig
	// ServerOption 服务器选项函数
	ServerOption = core.ServerOption
	// PoolConfig 连接池配置
	PoolConfig = core.PoolConfig
)

// 导出常量
const (
	// 连接状态常量
	StateConnecting    = core.StateConnecting
	StateConnected     = core.StateConnected
	StateDisconnecting = core.StateDisconnecting
	StateDisconnected  = core.StateDisconnected
	StateError         = core.StateError

	// 消息类型常量
	MessageTypeText     = core.MessageTypeText
	MessageTypeBinary   = core.MessageTypeBinary
	MessageTypeJSON     = core.MessageTypeJSON
	MessageTypeProtobuf = core.MessageTypeProtobuf
	MessageTypeCustom   = core.MessageTypeCustom
	MessageTypePing     = core.MessageTypePing
	MessageTypePong     = core.MessageTypePong
	MessageTypeClose    = core.MessageTypeClose
)

// 服务器创建函数

// NewTCPServer 创建新的TCP服务器
func NewTCPServer(opts ...ServerOption) Server {
	return tcp.NewTCPServer(opts...)
}

// NewUDPServer 创建新的UDP服务器
func NewUDPServer(opts ...ServerOption) Server {
	return udp.NewUDPServer(opts...)
}

// NewWebSocketServer 创建新的WebSocket服务器
func NewWebSocketServer(opts ...ServerOption) Server {
	return websocket.NewServer(opts...)
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(opts ...ServerOption) core.Server {
	config := &core.ServerConfig{
		ReadBufferSize:       4096,
		WriteBufferSize:      4096,
		MaxConnections:       1000,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		IdleTimeout:          5 * time.Minute,
		EnableConnectionPool: false,
		EnableMemoryPool:     false,
		EnableGoroutinePool:  false,
		EnableHeartbeat:      true,
		HeartbeatInterval:    30 * time.Second,
		EnableReconnect:      false,
		MaxReconnectAttempts: 3,
		EnableMetrics:        false,
		MetricsPath:          "/metrics",
		EnablePprof:          false,
	}

	for _, opt := range opts {
		opt(config)
	}

	return http.NewHTTPServer(config)
}

// 配置选项函数

// WithReadBufferSize 设置读缓冲区大小
func WithReadBufferSize(size int) ServerOption {
	return core.WithReadBufferSize(size)
}

// WithWriteBufferSize 设置写缓冲区大小
func WithWriteBufferSize(size int) ServerOption {
	return core.WithWriteBufferSize(size)
}

// WithMaxConnections 设置最大连接数
func WithMaxConnections(max int) ServerOption {
	return core.WithMaxConnections(max)
}

// WithReadTimeout 设置读超时
func WithReadTimeout(timeout time.Duration) ServerOption {
	return core.WithReadTimeout(timeout)
}

// WithWriteTimeout 设置写超时
func WithWriteTimeout(timeout time.Duration) ServerOption {
	return core.WithWriteTimeout(timeout)
}

// WithIdleTimeout 设置空闲超时
func WithIdleTimeout(timeout time.Duration) ServerOption {
	return core.WithIdleTimeout(timeout)
}

// WithConnectionPool 启用/禁用连接池
func WithConnectionPool(enable bool) ServerOption {
	return core.WithConnectionPool(enable)
}

// WithMemoryPool 启用/禁用内存池
func WithMemoryPool(enable bool) ServerOption {
	return core.WithMemoryPool(enable)
}

// WithGoroutinePool 启用/禁用协程池
func WithGoroutinePool(enable bool) ServerOption {
	return core.WithGoroutinePool(enable)
}

// WithHeartbeat 设置心跳配置
func WithHeartbeat(enable bool, interval time.Duration) ServerOption {
	return core.WithHeartbeat(enable, interval)
}

// WithReconnect 设置重连配置
func WithReconnect(enable bool, maxAttempts int) ServerOption {
	return core.WithReconnect(enable, maxAttempts)
}

// WithMetrics 设置监控配置
func WithMetrics(enable bool, path string) ServerOption {
	return core.WithMetrics(enable, path)
}

// WithPprof 启用/禁用性能分析
func WithPprof(enable bool) ServerOption {
	return core.WithPprof(enable)
}

// 工具函数

// NewMessage 创建新消息
func NewMessage(msgType MessageType, data []byte) *Message {
	return core.NewMessage(msgType, data)
}

// DefaultServerConfig 返回默认服务器配置
func DefaultServerConfig() *ServerConfig {
	return core.DefaultServerConfig()
}

// DefaultPoolConfig 返回默认连接池配置
func DefaultPoolConfig() *PoolConfig {
	return core.DefaultPoolConfig()
}

// 中间件函数

// LoggingMiddleware 日志中间件
func LoggingMiddleware() Middleware {
	return core.LoggingMiddleware()
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(maxRequests int) Middleware {
	return core.RateLimitMiddleware(maxRequests)
}

// AuthMiddleware 认证中间件
func AuthMiddleware() Middleware {
	return core.AuthMiddleware()
}

// MetricsMiddleware 监控中间件
func MetricsMiddleware() Middleware {
	return core.MetricsMiddleware()
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() Middleware {
	return core.RecoveryMiddleware()
}

