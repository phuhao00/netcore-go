// Package protocol 协议协商和回退机制
// Author: NetCore-Go Team
// Created: 2024

package protocol

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/netcore-go/pkg/http2"
	"github.com/netcore-go/pkg/http3"
)

// ProtocolType 协议类型
type ProtocolType int

const (
	HTTP1 ProtocolType = iota
	HTTP2
	HTTP3
)

func (p ProtocolType) String() string {
	switch p {
	case HTTP1:
		return "HTTP/1.1"
	case HTTP2:
		return "HTTP/2"
	case HTTP3:
		return "HTTP/3"
	default:
		return "Unknown"
	}
}

// ProtocolNegotiator 协议协商器
type ProtocolNegotiator struct {
	mu              sync.RWMutex
	config          *NegotiatorConfig
	http1Server     *http.Server
	http2Server     *http2.HTTP2Server
	http3Server     *http3.HTTP3Server
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
	stats           *NegotiatorStats
	protocolSupport map[ProtocolType]bool
}

// NegotiatorConfig 协商器配置
type NegotiatorConfig struct {
	// 基础配置
	Host string `json:"host"`
	HTTP1Port int `json:"http1_port"`
	HTTP2Port int `json:"http2_port"`
	HTTP3Port int `json:"http3_port"`

	// TLS配置
	TLSEnabled bool        `json:"tls_enabled"`
	CertFile   string      `json:"cert_file"`
	KeyFile    string      `json:"key_file"`
	TLSConfig  *tls.Config `json:"-"`

	// 协议支持
	EnableHTTP1 bool `json:"enable_http1"`
	EnableHTTP2 bool `json:"enable_http2"`
	EnableHTTP3 bool `json:"enable_http3"`

	// 回退策略
	FallbackStrategy FallbackStrategy `json:"fallback_strategy"`
	MaxRetries       int              `json:"max_retries"`
	RetryDelay       time.Duration    `json:"retry_delay"`

	// 协议特定配置
	HTTP2Config *http2.HTTP2Config `json:"http2_config"`
	HTTP3Config *http3.HTTP3Config `json:"http3_config"`

	// 健康检查
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`
}

// FallbackStrategy 回退策略
type FallbackStrategy int

const (
	FallbackSequential FallbackStrategy = iota // 顺序回退: HTTP/3 -> HTTP/2 -> HTTP/1.1
	FallbackParallel                            // 并行尝试
	FallbackClientPreference                    // 基于客户端偏好
	FallbackLoadBased                           // 基于负载
)

// NegotiatorStats 协商器统计
type NegotiatorStats struct {
	TotalConnections     int64                    `json:"total_connections"`
	ProtocolConnections  map[ProtocolType]int64   `json:"protocol_connections"`
	SuccessfulNegotiations int64                  `json:"successful_negotiations"`
	FailedNegotiations   int64                    `json:"failed_negotiations"`
	FallbackCount        map[ProtocolType]int64   `json:"fallback_count"`
	AverageNegotiationTime time.Duration          `json:"average_negotiation_time"`
	ProtocolPreferences  map[string]ProtocolType `json:"protocol_preferences"`
}

// DefaultNegotiatorConfig 返回默认协商器配置
func DefaultNegotiatorConfig() *NegotiatorConfig {
	return &NegotiatorConfig{
		Host:      "localhost",
		HTTP1Port: 8080,
		HTTP2Port: 8443,
		HTTP3Port: 8443,
		TLSEnabled: true,
		EnableHTTP1: true,
		EnableHTTP2: true,
		EnableHTTP3: true,
		FallbackStrategy: FallbackSequential,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
		HealthCheckEnabled: true,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
	}
}

// NewProtocolNegotiator 创建协议协商器
func NewProtocolNegotiator(config *NegotiatorConfig) *ProtocolNegotiator {
	if config == nil {
		config = DefaultNegotiatorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	stats := &NegotiatorStats{
		ProtocolConnections: make(map[ProtocolType]int64),
		FallbackCount:       make(map[ProtocolType]int64),
		ProtocolPreferences: make(map[string]ProtocolType),
	}

	protocolSupport := make(map[ProtocolType]bool)
	protocolSupport[HTTP1] = config.EnableHTTP1
	protocolSupport[HTTP2] = config.EnableHTTP2
	protocolSupport[HTTP3] = config.EnableHTTP3

	return &ProtocolNegotiator{
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
		stats:           stats,
		protocolSupport: protocolSupport,
	}
}

// Start 启动协议协商器
func (n *ProtocolNegotiator) Start(handler http.Handler) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return fmt.Errorf("protocol negotiator is already running")
	}

	// 启动HTTP/1.1服务器
	if n.config.EnableHTTP1 {
		if err := n.startHTTP1Server(handler); err != nil {
			return fmt.Errorf("failed to start HTTP/1.1 server: %w", err)
		}
	}

	// 启动HTTP/2服务器
	if n.config.EnableHTTP2 {
		if err := n.startHTTP2Server(handler); err != nil {
			return fmt.Errorf("failed to start HTTP/2 server: %w", err)
		}
	}

	// 启动HTTP/3服务器
	if n.config.EnableHTTP3 {
		if err := n.startHTTP3Server(handler); err != nil {
			return fmt.Errorf("failed to start HTTP/3 server: %w", err)
		}
	}

	n.running = true

	// 启动健康检查
	if n.config.HealthCheckEnabled {
		go n.healthCheckLoop()
	}

	return nil
}

// Stop 停止协议协商器
func (n *ProtocolNegotiator) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.running = false
	n.cancel()

	// 停止所有服务器
	var errors []error

	if n.http1Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := n.http1Server.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("HTTP/1.1 server shutdown error: %w", err))
		}
		cancel()
	}

	if n.http2Server != nil {
		if err := n.http2Server.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("HTTP/2 server stop error: %w", err))
		}
	}

	if n.http3Server != nil {
		if err := n.http3Server.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("HTTP/3 server stop error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	return nil
}

// startHTTP1Server 启动HTTP/1.1服务器
func (n *ProtocolNegotiator) startHTTP1Server(handler http.Handler) error {
	addr := fmt.Sprintf("%s:%d", n.config.Host, n.config.HTTP1Port)
	n.http1Server = &http.Server{
		Addr:    addr,
		Handler: n.wrapHandler(handler, HTTP1),
	}

	go func() {
		var err error
		if n.config.TLSEnabled {
			err = n.http1Server.ListenAndServeTLS(n.config.CertFile, n.config.KeyFile)
		} else {
			err = n.http1Server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP/1.1 server error: %v\n", err)
		}
	}()

	return nil
}

// startHTTP2Server 启动HTTP/2服务器
func (n *ProtocolNegotiator) startHTTP2Server(handler http.Handler) error {
	config := n.config.HTTP2Config
	if config == nil {
		config = http2.DefaultHTTP2Config()
		config.Host = n.config.Host
		config.Port = n.config.HTTP2Port
		config.TLSEnabled = n.config.TLSEnabled
		config.CertFile = n.config.CertFile
		config.KeyFile = n.config.KeyFile
		config.TLSConfig = n.config.TLSConfig
	}

	n.http2Server = http2.NewHTTP2Server(config)
	n.http2Server.SetHandler(n.wrapHandler(handler, HTTP2))

	return n.http2Server.Start()
}

// startHTTP3Server 启动HTTP/3服务器
func (n *ProtocolNegotiator) startHTTP3Server(handler http.Handler) error {
	config := n.config.HTTP3Config
	if config == nil {
		config = http3.DefaultHTTP3Config()
		config.Host = n.config.Host
		config.Port = n.config.HTTP3Port
		config.CertFile = n.config.CertFile
		config.KeyFile = n.config.KeyFile
		config.TLSConfig = n.config.TLSConfig
	}

	n.http3Server = http3.NewHTTP3Server(config)
	n.http3Server.SetHandler(n.wrapHandler(handler, HTTP3))

	return n.http3Server.Start()
}

// wrapHandler 包装处理器以添加统计
func (n *ProtocolNegotiator) wrapHandler(handler http.Handler, protocol ProtocolType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 更新统计
		n.mu.Lock()
		n.stats.TotalConnections++
		n.stats.ProtocolConnections[protocol]++
		n.stats.SuccessfulNegotiations++
		n.mu.Unlock()

		// 设置协议头
		w.Header().Set("Server-Protocol", protocol.String())

		// 处理请求
		handler.ServeHTTP(w, r)

		// 更新平均协商时间
		duration := time.Since(start)
		n.mu.Lock()
		n.stats.AverageNegotiationTime = (n.stats.AverageNegotiationTime + duration) / 2
		n.mu.Unlock()
	})
}

// healthCheckLoop 健康检查循环
func (n *ProtocolNegotiator) healthCheckLoop() {
	ticker := time.NewTicker(n.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (n *ProtocolNegotiator) performHealthCheck() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 检查HTTP/1.1服务器
	if n.config.EnableHTTP1 && n.http1Server != nil {
		n.protocolSupport[HTTP1] = n.checkServerHealth(fmt.Sprintf("http://%s:%d/health", n.config.Host, n.config.HTTP1Port))
	}

	// 检查HTTP/2服务器
	if n.config.EnableHTTP2 && n.http2Server != nil {
		n.protocolSupport[HTTP2] = n.http2Server.IsRunning()
	}

	// 检查HTTP/3服务器
	if n.config.EnableHTTP3 && n.http3Server != nil {
		n.protocolSupport[HTTP3] = n.http3Server.IsRunning()
	}
}

// checkServerHealth 检查服务器健康状态
func (n *ProtocolNegotiator) checkServerHealth(url string) bool {
	client := &http.Client{
		Timeout: n.config.HealthCheckTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetStats 获取统计信息
func (n *ProtocolNegotiator) GetStats() *NegotiatorStats {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.stats
}

// GetProtocolSupport 获取协议支持状态
func (n *ProtocolNegotiator) GetProtocolSupport() map[ProtocolType]bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.protocolSupport
}

// IsRunning 检查协商器是否运行
func (n *ProtocolNegotiator) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.running
}

// GetPreferredProtocol 获取客户端首选协议
func (n *ProtocolNegotiator) GetPreferredProtocol(userAgent string) ProtocolType {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if preferred, exists := n.stats.ProtocolPreferences[userAgent]; exists {
		return preferred
	}

	// 默认返回最高版本的可用协议
	if n.protocolSupport[HTTP3] {
		return HTTP3
	}
	if n.protocolSupport[HTTP2] {
		return HTTP2
	}
	return HTTP1
}