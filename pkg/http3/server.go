// Package http3 HTTP/3服务器实现
// Author: NetCore-Go Team
// Created: 2024

package http3

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	// TODO: Uncomment when quic-go dependencies are available
	// "github.com/quic-go/quic-go"
	// "github.com/quic-go/quic-go/http3"

	"github.com/netcore-go/pkg/core"
)

// HTTP3Server HTTP/3服务器
type HTTP3Server struct {
	mu       sync.RWMutex
	config   *HTTP3Config
	server   *http3.Server
	handler  http.Handler
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *HTTP3Stats
	listener *quic.Listener
}

// HTTP3Config HTTP/3配置
type HTTP3Config struct {
	// 基础配置
	Host string `json:"host"`
	Port int    `json:"port"`

	// TLS配置 (HTTP/3必需)
	CertFile  string      `json:"cert_file"`
	KeyFile   string      `json:"key_file"`
	TLSConfig *tls.Config `json:"-"`

	// QUIC特定配置
	MaxBidiStreams         int64         `json:"max_bidi_streams"`
	MaxUniStreams          int64         `json:"max_uni_streams"`
	MaxStreamReceiveWindow uint64        `json:"max_stream_receive_window"`
	MaxConnectionReceiveWindow uint64    `json:"max_connection_receive_window"`
	MaxIdleTimeout         time.Duration `json:"max_idle_timeout"`
	KeepAlivePeriod        time.Duration `json:"keep_alive_period"`
	HandshakeIdleTimeout   time.Duration `json:"handshake_idle_timeout"`
	MaxIncomingStreams     int64         `json:"max_incoming_streams"`
	MaxIncomingUniStreams  int64         `json:"max_incoming_uni_streams"`

	// HTTP/3特定配置
	MaxHeaderBytes         int           `json:"max_header_bytes"`
	AdditionalSettings     map[uint64]uint64 `json:"additional_settings"`
	StreamHijacker         func(http3.StreamType, quic.Connection, http3.Stream, error) (hijacked bool) `json:"-"`
	UniStreamHijacker      func(http3.StreamType, quic.Connection, quic.ReceiveStream, error) (hijacked bool) `json:"-"`

	// 性能配置
	EnableDatagrams        bool          `json:"enable_datagrams"`
	QUICConfig             *quic.Config  `json:"-"`
}

// HTTP3Stats HTTP/3统计信息
type HTTP3Stats struct {
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
	PacketsSent         int64 `json:"packets_sent"`
	PacketsReceived     int64 `json:"packets_received"`
	PacketsLost         int64 `json:"packets_lost"`
	RTT                 int64 `json:"rtt_ns"`
}

// DefaultHTTP3Config 返回默认HTTP/3配置
func DefaultHTTP3Config() *HTTP3Config {
	return &HTTP3Config{
		Host:                       "localhost",
		Port:                       8443,
		MaxBidiStreams:             100,
		MaxUniStreams:              100,
		MaxStreamReceiveWindow:     6291456,  // 6MB
		MaxConnectionReceiveWindow: 15728640, // 15MB
		MaxIdleTimeout:             300 * time.Second,
		KeepAlivePeriod:            30 * time.Second,
		HandshakeIdleTimeout:       10 * time.Second,
		MaxIncomingStreams:         100,
		MaxIncomingUniStreams:      100,
		MaxHeaderBytes:             1048576, // 1MB
		EnableDatagrams:            false,
		AdditionalSettings:         make(map[uint64]uint64),
	}
}

// NewHTTP3Server 创建HTTP/3服务器
func NewHTTP3Server(config *HTTP3Config) *HTTP3Server {
	if config == nil {
		config = DefaultHTTP3Config()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HTTP3Server{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		stats:  &HTTP3Stats{},
	}
}

// SetHandler 设置HTTP处理器
func (s *HTTP3Server) SetHandler(handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// Start 启动HTTP/3服务器
func (s *HTTP3Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("HTTP/3 server is already running")
	}

	// 加载TLS证书
	tlsConfig := s.config.TLSConfig
	if tlsConfig == nil {
		cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h3"},
		}
	}

	// 创建QUIC配置
	quicConfig := s.config.QUICConfig
	if quicConfig == nil {
		quicConfig = &quic.Config{
			MaxBidiStreamNum:               s.config.MaxBidiStreams,
			MaxUniStreamNum:                s.config.MaxUniStreams,
			MaxStreamReceiveWindow:         s.config.MaxStreamReceiveWindow,
			MaxConnectionReceiveWindow:     s.config.MaxConnectionReceiveWindow,
			MaxIdleTimeout:                 s.config.MaxIdleTimeout,
			KeepAlivePeriod:                s.config.KeepAlivePeriod,
			HandshakeIdleTimeout:           s.config.HandshakeIdleTimeout,
			MaxIncomingStreams:             s.config.MaxIncomingStreams,
			MaxIncomingUniStreams:          s.config.MaxIncomingUniStreams,
			EnableDatagrams:                s.config.EnableDatagrams,
		}
	}

	// 创建UDP监听器
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %w", err)
	}

	// 创建QUIC监听器
	listener, err := quic.Listen(udpConn, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to create QUIC listener: %w", err)
	}
	s.listener = listener

	// 创建HTTP/3服务器
	s.server = &http3.Server{
		Handler:               s.createHandler(),
		MaxHeaderBytes:        s.config.MaxHeaderBytes,
		AdditionalSettings:    s.config.AdditionalSettings,
		StreamHijacker:        s.config.StreamHijacker,
		UniStreamHijacker:     s.config.UniStreamHijacker,
	}

	s.running = true

	// 启动服务器
	go func() {
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()

		if err := s.server.Serve(listener); err != nil {
			fmt.Printf("HTTP/3 server error: %v\n", err)
		}
	}()

	return nil
}

// Stop 停止HTTP/3服务器
func (s *HTTP3Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭HTTP/3服务器
	if err := s.server.CloseGracefully(); err != nil {
		return fmt.Errorf("failed to close HTTP/3 server gracefully: %w", err)
	}

	// 关闭QUIC监听器
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return fmt.Errorf("failed to close QUIC listener: %w", err)
		}
	}

	return nil
}

// createHandler 创建处理器
func (s *HTTP3Server) createHandler() http.Handler {
	handler := s.handler
	if handler == nil {
		handler = http.DefaultServeMux
	}

	// 添加统计中间件
	return s.statsMiddleware(handler)
}

// statsMiddleware 统计中间件
func (s *HTTP3Server) statsMiddleware(next http.Handler) http.Handler {
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
func (s *HTTP3Server) GetStats() *HTTP3Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// IsRunning 检查服务器是否运行
func (s *HTTP3Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetAddress 获取服务器地址
func (s *HTTP3Server) GetAddress() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// SetTLSConfig 设置TLS配置
func (s *HTTP3Server) SetTLSConfig(config *tls.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.TLSConfig = config
}

// SetQUICConfig 设置QUIC配置
func (s *HTTP3Server) SetQUICConfig(config *quic.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.QUICConfig = config
}

// GetQUICStats 获取QUIC连接统计
func (s *HTTP3Server) GetQUICStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_connections"] = s.stats.TotalConnections
	stats["active_connections"] = s.stats.ActiveConnections
	stats["packets_sent"] = s.stats.PacketsSent
	stats["packets_received"] = s.stats.PacketsReceived
	stats["packets_lost"] = s.stats.PacketsLost
	stats["rtt_ns"] = s.stats.RTT

	return stats
}