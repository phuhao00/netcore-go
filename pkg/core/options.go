// Package core 定义NetCore-Go网络库的配置选项
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"time"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	// 网络配置
	ReadBufferSize  int           `json:"read_buffer_size"`
	WriteBufferSize int           `json:"write_buffer_size"`
	MaxConnections  int           `json:"max_connections"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`

	// 性能配置
	EnableConnectionPool bool `json:"enable_connection_pool"`
	EnableMemoryPool     bool `json:"enable_memory_pool"`
	EnableGoroutinePool  bool `json:"enable_goroutine_pool"`

	// 功能配置
	EnableHeartbeat      bool          `json:"enable_heartbeat"`
	HeartbeatInterval    time.Duration `json:"heartbeat_interval"`
	EnableReconnect      bool          `json:"enable_reconnect"`
	MaxReconnectAttempts int           `json:"max_reconnect_attempts"`

	// 监控配置
	EnableMetrics bool   `json:"enable_metrics"`
	MetricsPath   string `json:"metrics_path"`
	EnablePprof   bool   `json:"enable_pprof"`
}

// DefaultServerConfig 返回默认服务器配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		// 网络配置默认值
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		MaxConnections:  10000,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     300 * time.Second,

		// 性能配置默认值
		EnableConnectionPool: true,
		EnableMemoryPool:     true,
		EnableGoroutinePool:  true,

		// 功能配置默认值
		EnableHeartbeat:      true,
		HeartbeatInterval:    30 * time.Second,
		EnableReconnect:      false,
		MaxReconnectAttempts: 3,

		// 监控配置默认值
		EnableMetrics: true,
		MetricsPath:   "/metrics",
		EnablePprof:   false,
	}
}

// ServerOption 服务器选项函数类型
type ServerOption func(*ServerConfig)

// WithReadBufferSize 设置读缓冲区大小
func WithReadBufferSize(size int) ServerOption {
	return func(config *ServerConfig) {
		config.ReadBufferSize = size
	}
}

// WithWriteBufferSize 设置写缓冲区大小
func WithWriteBufferSize(size int) ServerOption {
	return func(config *ServerConfig) {
		config.WriteBufferSize = size
	}
}

// WithMaxConnections 设置最大连接数
func WithMaxConnections(max int) ServerOption {
	return func(config *ServerConfig) {
		config.MaxConnections = max
	}
}

// WithReadTimeout 设置读超时
func WithReadTimeout(timeout time.Duration) ServerOption {
	return func(config *ServerConfig) {
		config.ReadTimeout = timeout
	}
}

// WithWriteTimeout 设置写超时
func WithWriteTimeout(timeout time.Duration) ServerOption {
	return func(config *ServerConfig) {
		config.WriteTimeout = timeout
	}
}

// WithIdleTimeout 设置空闲超时
func WithIdleTimeout(timeout time.Duration) ServerOption {
	return func(config *ServerConfig) {
		config.IdleTimeout = timeout
	}
}

// WithConnectionPool 启用/禁用连接池
func WithConnectionPool(enable bool) ServerOption {
	return func(config *ServerConfig) {
		config.EnableConnectionPool = enable
	}
}

// WithMemoryPool 启用/禁用内存池
func WithMemoryPool(enable bool) ServerOption {
	return func(config *ServerConfig) {
		config.EnableMemoryPool = enable
	}
}

// WithGoroutinePool 启用/禁用协程池
func WithGoroutinePool(enable bool) ServerOption {
	return func(config *ServerConfig) {
		config.EnableGoroutinePool = enable
	}
}

// WithHeartbeat 设置心跳配置
func WithHeartbeat(enable bool, interval time.Duration) ServerOption {
	return func(config *ServerConfig) {
		config.EnableHeartbeat = enable
		config.HeartbeatInterval = interval
	}
}

// WithReconnect 设置重连配置
func WithReconnect(enable bool, maxAttempts int) ServerOption {
	return func(config *ServerConfig) {
		config.EnableReconnect = enable
		config.MaxReconnectAttempts = maxAttempts
	}
}

// WithMetrics 设置监控配置
func WithMetrics(enable bool, path string) ServerOption {
	return func(config *ServerConfig) {
		config.EnableMetrics = enable
		config.MetricsPath = path
	}
}

// WithPprof 启用/禁用性能分析
func WithPprof(enable bool) ServerOption {
	return func(config *ServerConfig) {
		config.EnablePprof = enable
	}
}

// PoolConfig 连接池配置
type PoolConfig struct {
	// InitialSize 初始大小
	InitialSize int `json:"initial_size"`
	// MaxSize 最大大小
	MaxSize int `json:"max_size"`
	// IdleTimeout 空闲超时
	IdleTimeout time.Duration `json:"idle_timeout"`
}

// DefaultPoolConfig 返回默认连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		InitialSize: 10,
		MaxSize:     100,
		IdleTimeout: 300 * time.Second,
	}
}