// Package core 定义NetCore-Go网络库的核心数据类型
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"fmt"
	"time"
)

// ConnectionState 连接状态枚举
type ConnectionState int

const (
	// StateConnecting 连接中
	StateConnecting ConnectionState = iota
	// StateConnected 已连接
	StateConnected
	// StateDisconnecting 断开连接中
	StateDisconnecting
	// StateDisconnected 已断开连接
	StateDisconnected
	// StateError 连接错误
	StateError
)

// String 返回连接状态的字符串表示
func (s ConnectionState) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnecting:
		return "disconnecting"
	case StateDisconnected:
		return "disconnected"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// MessageType 消息类型枚举
type MessageType int

const (
	// MessageTypeText 文本消息
	MessageTypeText MessageType = iota
	// MessageTypeBinary 二进制消息
	MessageTypeBinary
	// MessageTypeJSON JSON消息
	MessageTypeJSON
	// MessageTypeProtobuf Protobuf消息
	MessageTypeProtobuf
	// MessageTypeCustom 自定义消息
	MessageTypeCustom
	// MessageTypePing Ping消息（WebSocket）
	MessageTypePing
	// MessageTypePong Pong消息（WebSocket）
	MessageTypePong
	// MessageTypeClose 关闭消息（WebSocket）
	MessageTypeClose
)

// String 返回消息类型的字符串表示
func (t MessageType) String() string {
	switch t {
	case MessageTypeText:
		return "text"
	case MessageTypeBinary:
		return "binary"
	case MessageTypeJSON:
		return "json"
	case MessageTypeProtobuf:
		return "protobuf"
	case MessageTypeCustom:
		return "custom"
	case MessageTypePing:
		return "ping"
	case MessageTypePong:
		return "pong"
	case MessageTypeClose:
		return "close"
	default:
		return "unknown"
	}
}

// Message 消息结构体
type Message struct {
	// Type 消息类型
	Type MessageType `json:"type"`
	// Data 消息数据
	Data []byte `json:"data"`
	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
	// Headers 消息头
	Headers map[string]string `json:"headers,omitempty"`
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, data []byte) *Message {
	return &Message{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
		Headers:   make(map[string]string),
	}
}

// SetHeader 设置消息头
func (m *Message) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// GetHeader 获取消息头
func (m *Message) GetHeader(key string) (string, bool) {
	if m.Headers == nil {
		return "", false
	}
	value, exists := m.Headers[key]
	return value, exists
}

// ServerStats 服务器统计信息
type ServerStats struct {
	// ActiveConnections 活跃连接数
	ActiveConnections int64 `json:"active_connections"`
	// TotalConnections 总连接数
	TotalConnections int64 `json:"total_connections"`
	// MessagesReceived 接收消息数
	MessagesReceived int64 `json:"messages_received"`
	// MessagesSent 发送消息数
	MessagesSent int64 `json:"messages_sent"`
	// BytesReceived 接收字节数
	BytesReceived int64 `json:"bytes_received"`
	// BytesSent 发送字节数
	BytesSent int64 `json:"bytes_sent"`
	// StartTime 启动时间
	StartTime time.Time `json:"start_time"`
	// Uptime 运行时间（秒）
	Uptime int64 `json:"uptime_seconds"`
	// ErrorCount 错误计数
	ErrorCount int64 `json:"error_count"`
	// LastError 最后一个错误
	LastError string `json:"last_error"`
}

// NewServerStats 创建新的服务器统计信息
func NewServerStats() *ServerStats {
	return &ServerStats{
		StartTime: time.Now(),
	}
}

// UpdateUptime 更新运行时间
func (s *ServerStats) UpdateUptime() {
	s.Uptime = int64(time.Since(s.StartTime).Seconds())
}

// 错误定义
var (
	ErrServerNotRunning   = fmt.Errorf("server is not running")
	ErrServerRunning      = fmt.Errorf("server is already running")
	ErrInvalidConfig      = fmt.Errorf("invalid server config")
	ErrConnectionClosed   = fmt.Errorf("connection is closed")
)