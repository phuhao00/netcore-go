// Package core 定义NetCore-Go网络库的核心接口
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"net"
)

// Server 服务器接口，定义所有协议服务器的统一行为
type Server interface {
	// Start 启动服务器，监听指定地址
	Start(addr string) error
	// Stop 停止服务器
	Stop() error
	// SetHandler 设置消息处理器
	SetHandler(handler MessageHandler)
	// SetMiddleware 设置中间件
	SetMiddleware(middleware ...Middleware)
	// GetStats 获取服务器统计信息
	GetStats() *ServerStats
}

// Connection 连接接口，定义客户端连接的统一行为
type Connection interface {
	// ID 获取连接唯一标识
	ID() string
	// RemoteAddr 获取远程地址
	RemoteAddr() net.Addr
	// LocalAddr 获取本地地址
	LocalAddr() net.Addr
	// Send 发送原始数据
	Send(data []byte) error
	// SendMessage 发送消息对象
	SendMessage(msg Message) error
	// Close 关闭连接
	Close() error
	// IsActive 检查连接是否活跃
	IsActive() bool
	// SetContext 设置连接上下文
	SetContext(key, value interface{})
	// GetContext 获取连接上下文
	GetContext(key interface{}) interface{}
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	// OnConnect 连接建立时调用
	OnConnect(conn Connection)
	// OnMessage 收到消息时调用
	OnMessage(conn Connection, msg Message)
	// OnDisconnect 连接断开时调用
	OnDisconnect(conn Connection, err error)
}

// Middleware 中间件接口
type Middleware interface {
	// Name 获取中间件名称
	Name() string
	// Priority 获取中间件优先级
	Priority() int
	// Process 处理请求
	Process(ctx Context, next Handler) error
}

// Context 上下文接口
type Context interface {
	// Connection 获取连接对象
	Connection() Connection
	// Message 获取消息对象
	Message() Message
	// Set 设置上下文值
	Set(key string, value interface{})
	// Get 获取上下文值
	Get(key string) (interface{}, bool)
	// Abort 中止处理
	Abort()
	// IsAborted 检查是否已中止
	IsAborted() bool
}

// Handler 处理器函数类型
type Handler func(ctx Context) error