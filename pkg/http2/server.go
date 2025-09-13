// Package http2 HTTP/2服务器实现
// Author: NetCore-Go Team
// Created: 2024

package http2

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// HTTP2Server HTTP/2服务器
type HTTP2Server struct {
	mu       sync.RWMutex
	config   *HTTP2Config
	server   *http.Server
	h2Server *http2.Server
	handler  http.Handler
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *HTTP2Stats
}

// HTTP2Config HTTP/2配置
type HTTP2Config struct {
	// 基础配置
	Host string `json:"host"`
	Port int    `json:"port"`

	// TLS配置
	TLSEnabled  bool   `json:"tls_enabled"`
	CertFile    string `json:"cert_file"`
	KeyFile     string `json:"key_file"`
	TLSConfig   *tls.Config

	// HTTP/2特定配置
	MaxConcurrentStreams         uint32        `json:"max_concurrent_streams"`
	MaxReadFrameSize             uint32        `json:"max_read_frame_size"`
	PermitProhibitedCipherSuites bool          `json:"permit_prohibited_cipher_suites"`
	IdleTimeout                  time.Duration `json:"idle_timeout"`
	MaxUploadBufferPerConnection int32         `json:"max_upload_buffer_per_connection"`
	MaxUploadBufferPerStream     int32         `json:"max_upload_buffer_per_stream"`

	// 性能配置
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	MaxHeaderBytes    int           `json:"max_header_bytes"`
	EnableH2C         bool          `json:"enable_h2c"` // HTTP/2 Cleartext
}

// HTTP2Stats HTTP/2统计信息
type HTTP2Stats struct {
	TotalConnections    int64 `json:"total_connections"`
	ActiveConnections   int64 `json:"active_connections"`
	TotalStreams        int64 `json:"total_streams"`
	ActiveStreams       int64 `json:"active_streams"`
	TotalRequests       int64 `json:"total_requests"`
	TotalResponses      int64 `json:"total_responses"`
	BytesReceived       int64 `json:"bytes_received"`
	BytesSent           int64 `json:"bytes_sent"`
	AverageResponseTime int64 `json:"average_response_time_ns"`
	ErrorCount          int64 `json:"error_count"`
}

// DefaultHTTP2Config 返回默认HTTP/2配置
func DefaultHTTP2Config() *HTTP2Config {
	return &HTTP2Config{
		Host:                         "localhost",
		Port:                         8443,
		TLSEnabled:                   true,
		MaxConcurrentStreams:         250,
		MaxReadFrameSize:             1048576, // 1MB
		PermitProhibitedCipherSuites: false,
		IdleTimeout:                  300 * time.Second,
		MaxUploadBufferPerConnection: 1048576, // 1MB
		MaxUploadBufferPerStream:     1048576, // 1MB
		ReadTimeout:                  30 * time.Second,
		WriteTimeout:                 30 * time.Second,
		MaxHeaderBytes:               1048576, // 1MB
		EnableH2C:                    false,
	}
}

// NewHTTP2Server 创建HTTP/2服务器
func NewHTTP2Server(config *HTTP2Config) *HTTP2Server {
	if config == nil {
		config = DefaultHTTP2Config()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HTTP2Server{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		stats:  &HTTP2Stats{},
	}
}

// SetHandler 设置HTTP处理器
func (s *HTTP2Server) SetHandler(handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// Start 启动HTTP/2服务器
func (s *HTTP2Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("HTTP/2 server is already running")
	}

	// 创建HTTP/2服务器配置
	h2Server := &http2.Server{
		MaxConcurrentStreams:         s.config.MaxConcurrentStreams,
		MaxReadFrameSize:             s.config.MaxReadFrameSize,
		PermitProhibitedCipherSuites: s.config.PermitProhibitedCipherSuites,
		IdleTimeout:                  s.config.IdleTimeout,
		MaxUploadBufferPerConnection: s.config.MaxUploadBufferPerConnection,
		MaxUploadBufferPerStream:     s.config.MaxUploadBufferPerStream,
	}

	s.h2Server = h2Server

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:           addr,
		Handler:        s.createHandler(),
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	// 配置HTTP/2
	if err := http2.ConfigureServer(s.server, h2Server); err != nil {
		return fmt.Errorf("failed to configure HTTP/2 server: %w", err)
	}

	s.running = true

	// 启动服务器
	go func() {
		var err error
		if s.config.TLSEnabled {
			// HTTPS with HTTP/2
			if s.config.TLSConfig != nil {
				s.server.TLSConfig = s.config.TLSConfig
			}
			err = s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
		} else if s.config.EnableH2C {
			// HTTP/2 Cleartext
			err = s.server.ListenAndServe()
		} else {
			err = fmt.Errorf("HTTP/2 requires TLS or H2C to be enabled")
		}

		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP/2 server error: %v\n", err)
		}
	}()

	return nil
}

// Stop 停止HTTP/2服务器
func (s *HTTP2Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// createHandler 创建处理器
func (s *HTTP2Server) createHandler() http.Handler {
	handler := s.handler
	if handler == nil {
		handler = http.DefaultServeMux
	}

	// 添加统计中间件
	handler = s.statsMiddleware(handler)

	// 如果启用H2C，包装处理器
	if s.config.EnableH2C {
		return h2c.NewHandler(handler, s.h2Server)
	}

	return handler
}

// statsMiddleware 统计中间件
func (s *HTTP2Server) statsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 更新请求统计
		s.stats.TotalRequests++

		// 处理请求
		next.ServeHTTP(w, r)

		// 更新响应统计
		s.stats.TotalResponses++
		duration := time.Since(start)
		s.stats.AverageResponseTime = (s.stats.AverageResponseTime + duration.Nanoseconds()) / 2
	})
}

// GetStats 获取统计信息
func (s *HTTP2Server) GetStats() *HTTP2Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// IsRunning 检查服务器是否运行
func (s *HTTP2Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetAddress 获取服务器地址
func (s *HTTP2Server) GetAddress() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// SetTLSConfig 设置TLS配置
func (s *HTTP2Server) SetTLSConfig(config *tls.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.TLSConfig = config
}