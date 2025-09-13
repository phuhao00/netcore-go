// Package netcore 提供NetCore-Go框架的主要入口点
// Author: NetCore-Go Team
// Created: 2024

package netcore

import (
	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/http"
	"github.com/netcore-go/pkg/kcp"
	"github.com/netcore-go/pkg/rpc"
	"github.com/netcore-go/pkg/tcp"
	"github.com/netcore-go/pkg/udp"
	"github.com/netcore-go/pkg/websocket"
)

// NewTCPServer 创建TCP服务器
func NewTCPServer(opts ...core.ServerOption) core.Server {
	return tcp.NewTCPServer(opts...)
}

// NewUDPServer 创建UDP服务器
func NewUDPServer(opts ...core.ServerOption) core.Server {
	return udp.NewUDPServer(opts...)
}

// NewWebSocketServer 创建WebSocket服务器
func NewWebSocketServer(opts ...core.ServerOption) core.Server {
	return websocket.NewServer(opts...)
}

// NewRPCServer 创建RPC服务器
func NewRPCServer(opts ...core.ServerOption) *rpc.RPCServer {
	return rpc.NewRPCServer(opts...)
}

// NewServer 创建通用服务器
func NewServer(config *Config) core.Server {
	return tcp.NewTCPServer()
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(opts ...core.ServerOption) *http.HTTPServer {
	// 创建HTTP服务器配置
	httpConfig := &http.ServerConfig{
		Address: ":8080",
		Port:    8080,
	}
	return http.NewHTTPServer(httpConfig)
}

// NewKCPServer 创建KCP服务器
func NewKCPServer(opts ...core.ServerOption) *kcp.KCPServer {
	return kcp.NewKCPServer(opts...)
}

// Config 服务器配置
type Config struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// 导出常用的配置选项
var (
	WithMaxConnections    = core.WithMaxConnections
	WithReadBufferSize    = core.WithReadBufferSize
	WithWriteBufferSize   = core.WithWriteBufferSize
	WithHeartbeat         = core.WithHeartbeat
	WithConnectionPool    = core.WithConnectionPool
	WithMemoryPool        = core.WithMemoryPool
	WithGoroutinePool     = core.WithGoroutinePool
	WithReadTimeout       = core.WithReadTimeout
	WithWriteTimeout      = core.WithWriteTimeout
	WithIdleTimeout       = core.WithIdleTimeout
	WithMetrics           = core.WithMetrics
)

// 导出常用的中间件
var (
	LoggingMiddleware  = core.LoggingMiddleware
	MetricsMiddleware  = core.MetricsMiddleware
	RecoveryMiddleware = core.RecoveryMiddleware
	AuthMiddleware     = core.AuthMiddleware
	RateLimitMiddleware = core.RateLimitMiddleware
)

// 导出核心类型
type (
	Connection     = core.Connection
	Message        = core.Message
	MessageHandler = core.MessageHandler
	Middleware     = core.Middleware
	Context        = core.Context
	ServerStats    = core.ServerStats
	MessageType    = core.MessageType
)

// 导出消息类型常量
var (
	MessageTypeText   = core.MessageTypeText
	MessageTypeBinary = core.MessageTypeBinary
	MessageTypeClose  = core.MessageTypeClose
	MessageTypePing   = core.MessageTypePing
	MessageTypePong   = core.MessageTypePong
)

// 导出核心函数
var (
	NewMessage = core.NewMessage
)