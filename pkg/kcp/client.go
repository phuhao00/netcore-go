// Package kcp KCP客户端实现
// Author: NetCore-Go Team
// Created: 2024

package kcp

import (
	"fmt"
	"sync"
	"time"

	"github.com/xtaci/kcp-go/v5"

	"github.com/phuhao00/netcore-go/pkg/core"
)

// KCPClient KCP客户端
type KCPClient struct {
	conn      *KCPConnection
	address   string
	kcpConfig *KCPConfig
	mu        sync.RWMutex
	closed    bool
	timeout   time.Duration
	retryCount int
	onMessage func(core.Message)
	onConnect func()
	onDisconnect func(error)
}

// NewKCPClient 创建KCP客户端
func NewKCPClient(address string, options ...ClientOption) *KCPClient {
	client := &KCPClient{
		address:    address,
		kcpConfig:  DefaultKCPConfig(),
		timeout:    30 * time.Second,
		retryCount: 3,
	}
	
	// 应用配置选项
	for _, option := range options {
		option(client)
	}
	
	return client
}

// ClientOption 客户端配置选项
type ClientOption func(*KCPClient)

// WithKCPConfig 设置KCP配置
func WithKCPConfig(config *KCPConfig) ClientOption {
	return func(client *KCPClient) {
		client.kcpConfig = config
	}
}

// WithClientTimeout 设置超时时间
func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(client *KCPClient) {
		client.timeout = timeout
	}
}

// WithClientRetryCount 设置重试次数
func WithClientRetryCount(retryCount int) ClientOption {
	return func(client *KCPClient) {
		client.retryCount = retryCount
	}
}

// WithOnMessage 设置消息处理回调
func WithOnMessage(callback func(core.Message)) ClientOption {
	return func(client *KCPClient) {
		client.onMessage = callback
	}
}

// WithOnConnect 设置连接建立回调
func WithOnConnect(callback func()) ClientOption {
	return func(client *KCPClient) {
		client.onConnect = callback
	}
}

// WithOnDisconnect 设置连接断开回调
func WithOnDisconnect(callback func(error)) ClientOption {
	return func(client *KCPClient) {
		client.onDisconnect = callback
	}
}

// Connect 连接到KCP服务器
func (c *KCPClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil && c.conn.IsActive() {
		return nil // 已经连接
	}
	
	// 创建KCP连接
	var kcpConn *kcp.UDPSession
	var err error
	
	// 根据加密算法创建连接
	switch c.kcpConfig.Crypt {
	case "aes":
		kcpConn, err = kcp.DialWithOptions(c.address, nil, c.kcpConfig.DataShard, c.kcpConfig.ParityShard)
	case "none":
		kcpConn, err = kcp.Dial(c.address)
	default:
		kcpConn, err = kcp.DialWithOptions(c.address, nil, c.kcpConfig.DataShard, c.kcpConfig.ParityShard)
	}
	
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", c.address, err)
	}
	
	// 配置KCP参数
	c.configureKCP(kcpConn)
	
	// 创建连接包装器
	c.conn = &KCPConnection{
		conn:       kcpConn,
		id:        generateConnectionID(),
		context:    make(map[string]interface{}),
		lastActive: time.Now(),
		stats: &ConnectionStats{
			ConnectedAt: time.Now(),
			LastActive:  time.Now(),
		},
	}
	
	// 启动消息接收循环
	go c.receiveLoop()
	
	// 调用连接建立回调
	if c.onConnect != nil {
		c.onConnect()
	}
	
	return nil
}

// configureKCP 配置KCP参数
func (c *KCPClient) configureKCP(conn *kcp.UDPSession) {
	// 设置KCP参数
	conn.SetNoDelay(
		c.kcpConfig.Nodelay,
		c.kcpConfig.Interval,
		c.kcpConfig.Resend,
		c.kcpConfig.NC,
	)
	
	// 设置窗口大小
	conn.SetWindowSize(c.kcpConfig.SndWnd, c.kcpConfig.RcvWnd)
	
	// 设置MTU
	conn.SetMtu(c.kcpConfig.MTU)
	
	// 设置DSCP
	if c.kcpConfig.DSCP > 0 {
		conn.SetDSCP(c.kcpConfig.DSCP)
	}
	
	// 设置ACK立即发送
	if c.kcpConfig.AckNodelay {
		conn.SetACKNoDelay(true)
	}
	
	// 设置读写缓冲区
	conn.SetReadBuffer(4096)
	conn.SetWriteBuffer(4096)
}

// receiveLoop 消息接收循环
func (c *KCPClient) receiveLoop() {
	defer func() {
		if r := recover(); r != nil {
			if c.onDisconnect != nil {
				c.onDisconnect(fmt.Errorf("panic in receive loop: %v", r))
			}
		}
	}()
	
	for {
		c.mu.RLock()
		conn := c.conn
		closed := c.closed
		c.mu.RUnlock()
		
		if closed || conn == nil || !conn.IsActive() {
			break
		}
		
		msg, err := conn.ReadMessage()
		if err != nil {
			if c.onDisconnect != nil {
				c.onDisconnect(err)
			}
			break
		}
		
		// 处理消息
		if c.onMessage != nil {
			c.onMessage(msg)
		}
	}
}

// Send 发送原始数据
func (c *KCPClient) Send(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.closed || c.conn == nil {
		return fmt.Errorf("client not connected")
	}
	
	return c.conn.Send(data)
}

// SendMessage 发送消息
func (c *KCPClient) SendMessage(msg core.Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.closed || c.conn == nil {
		return fmt.Errorf("client not connected")
	}
	
	return c.conn.SendMessage(msg)
}

// SendWithRetry 带重试的发送
func (c *KCPClient) SendWithRetry(msg core.Message) error {
	var lastErr error
	
	for i := 0; i <= c.retryCount; i++ {
		err := c.SendMessage(msg)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// 如果是最后一次重试，直接返回错误
		if i == c.retryCount {
			break
		}
		
		// 等待一段时间后重试
		time.Sleep(time.Duration(i+1) * time.Second)
		
		// 如果连接断开，尝试重连
		if !c.IsConnected() {
			if err := c.Connect(); err != nil {
				lastErr = err
				continue
			}
		}
	}
	
	return lastErr
}

// Ping 发送ping消息
func (c *KCPClient) Ping() error {
	pingMsg := core.Message{
		Type:      core.MessageTypeText,
		Data:      []byte("ping"),
		Timestamp: time.Now(),
	}
	
	return c.SendMessage(pingMsg)
}

// Close 关闭客户端
func (c *KCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	
	if c.conn != nil {
		return c.conn.Close()
	}
	
	return nil
}

// IsConnected 检查是否已连接
func (c *KCPClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return !c.closed && c.conn != nil && c.conn.IsActive()
}

// GetConnection 获取连接
func (c *KCPClient) GetConnection() *KCPConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// GetStats 获取连接统计信息
func (c *KCPClient) GetStats() *ConnectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn == nil {
		return nil
	}
	
	return c.conn.GetStats()
}

// GetKCPStats 获取KCP特定统计信息
func (c *KCPClient) GetKCPStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn == nil {
		return nil
	}
	
	return c.conn.GetKCPStats()
}

// Reconnect 重新连接
func (c *KCPClient) Reconnect() error {
	// 关闭现有连接
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.closed = false
	c.mu.Unlock()
	
	// 重新连接
	return c.Connect()
}

// AutoReconnect 自动重连
func (c *KCPClient) AutoReconnect(interval time.Duration, maxAttempts int) {
	go func() {
		attempts := 0
		
		for {
			if c.IsConnected() {
				time.Sleep(interval)
				continue
			}
			
			if maxAttempts > 0 && attempts >= maxAttempts {
				if c.onDisconnect != nil {
					c.onDisconnect(fmt.Errorf("max reconnect attempts reached"))
				}
				break
			}
			
			attempts++
			if err := c.Reconnect(); err != nil {
				time.Sleep(interval)
				continue
			}
			
			// 重连成功，重置计数器
			attempts = 0
		}
	}()
}

// SetKCPParameters 设置KCP参数
func (c *KCPClient) SetKCPParameters(nodelay, interval, resend, nc int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn != nil {
		c.conn.SetKCPParameters(nodelay, interval, resend, nc)
	}
}

// SetWindowSize 设置窗口大小
func (c *KCPClient) SetWindowSize(sndWnd, rcvWnd int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn != nil {
		c.conn.SetWindowSize(sndWnd, rcvWnd)
	}
}