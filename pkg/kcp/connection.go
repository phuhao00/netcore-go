// Package kcp KCP连接实现
// Author: NetCore-Go Team
// Created: 2024

package kcp

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/xtaci/kcp-go/v5"

	"github.com/netcore-go/netcore/pkg/core"
)

// KCPConnection KCP连接实现
type KCPConnection struct {
	conn       *kcp.UDPSession
	server     *KCPServer
	id         string
	context    map[string]interface{}
	contextMu  sync.RWMutex
	closed     bool
	closeMu    sync.Mutex
	lastActive time.Time
	stats      *ConnectionStats
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	BytesReceived    int64     `json:"bytes_received"`
	BytesSent        int64     `json:"bytes_sent"`
	MessagesReceived int64     `json:"messages_received"`
	MessagesSent     int64     `json:"messages_sent"`
	ConnectedAt      time.Time `json:"connected_at"`
	LastActive       time.Time `json:"last_active"`
	ErrorCount       int64     `json:"error_count"`
	LastError        string    `json:"last_error"`
}

// NewKCPConnection 创建KCP连接
func NewKCPConnection(conn *kcp.UDPSession, server *KCPServer) *KCPConnection {
	now := time.Now()
	return &KCPConnection{
		conn:       conn,
		server:     server,
		id:        generateConnectionID(),
		context:    make(map[string]interface{}),
		lastActive: now,
		stats: &ConnectionStats{
			ConnectedAt: now,
			LastActive:  now,
		},
	}
}

// ID 获取连接ID
func (c *KCPConnection) ID() string {
	return c.id
}

// RemoteAddr 获取远程地址
func (c *KCPConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr 获取本地地址
func (c *KCPConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// Send 发送原始数据
func (c *KCPConnection) Send(data []byte) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	
	// 设置写入超时
	if c.server.Config.WriteTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.server.Config.WriteTimeout))
	}
	
	// 写入数据长度（4字节）
	length := len(data)
	lengthBytes := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	
	if _, err := c.conn.Write(lengthBytes); err != nil {
		c.stats.ErrorCount++
		c.stats.LastError = err.Error()
		return err
	}
	
	// 写入数据
	if _, err := c.conn.Write(data); err != nil {
		c.stats.ErrorCount++
		c.stats.LastError = err.Error()
		return err
	}
	
	c.stats.BytesSent += int64(len(data) + 4)
	c.lastActive = time.Now()
	c.stats.LastActive = c.lastActive
	
	return nil
}

// SendMessage 发送消息
func (c *KCPConnection) SendMessage(msg core.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}
	
	if err := c.Send(data); err != nil {
		return err
	}
	
	c.stats.MessagesSent++
	return nil
}

// ReadMessage 读取消息
func (c *KCPConnection) ReadMessage() (core.Message, error) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return core.Message{}, fmt.Errorf("connection closed")
	}
	
	// 设置读取超时
	if c.server.Config.ReadTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.server.Config.ReadTimeout))
	}
	
	// 读取数据长度（4字节）
	lengthBytes := make([]byte, 4)
	if _, err := c.conn.Read(lengthBytes); err != nil {
		c.stats.ErrorCount++
		c.stats.LastError = err.Error()
		return core.Message{}, err
	}
	
	length := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])
	if length <= 0 || length > 1024*1024 { // 限制最大1MB
		err := fmt.Errorf("invalid message length: %d", length)
		c.stats.ErrorCount++
		c.stats.LastError = err.Error()
		return core.Message{}, err
	}
	
	// 读取消息数据
	msgData := make([]byte, length)
	if _, err := c.conn.Read(msgData); err != nil {
		c.stats.ErrorCount++
		c.stats.LastError = err.Error()
		return core.Message{}, err
	}
	
	// 解析消息
	var msg core.Message
	if err := json.Unmarshal(msgData, &msg); err != nil {
		c.stats.ErrorCount++
		c.stats.LastError = err.Error()
		return core.Message{}, fmt.Errorf("failed to unmarshal message: %v", err)
	}
	
	c.stats.BytesReceived += int64(length + 4)
	c.stats.MessagesReceived++
	c.lastActive = time.Now()
	c.stats.LastActive = c.lastActive
	
	return msg, nil
}

// Close 关闭连接
func (c *KCPConnection) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	return c.conn.Close()
}

// IsActive 检查连接是否活跃
func (c *KCPConnection) IsActive() bool {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	return !c.closed
}

// SetContext 设置上下文
func (c *KCPConnection) SetContext(key, value interface{}) {
	c.contextMu.Lock()
	defer c.contextMu.Unlock()
	
	c.context[fmt.Sprintf("%v", key)] = value
}

// GetContext 获取上下文
func (c *KCPConnection) GetContext(key interface{}) interface{} {
	c.contextMu.RLock()
	defer c.contextMu.RUnlock()
	
	return c.context[fmt.Sprintf("%v", key)]
}

// LastActive 获取最后活跃时间
func (c *KCPConnection) LastActive() time.Time {
	return c.lastActive
}

// GetStats 获取连接统计信息
func (c *KCPConnection) GetStats() *ConnectionStats {
	return c.stats
}

// GetKCPStats 获取KCP特定统计信息
func (c *KCPConnection) GetKCPStats() map[string]interface{} {
	if c.conn == nil {
		return nil
	}
	
	// 获取KCP内部统计信息
	stats := map[string]interface{}{
		"connection_id":     c.id,
		"remote_addr":       c.conn.RemoteAddr().String(),
		"local_addr":        c.conn.LocalAddr().String(),
		"bytes_received":    c.stats.BytesReceived,
		"bytes_sent":        c.stats.BytesSent,
		"messages_received": c.stats.MessagesReceived,
		"messages_sent":     c.stats.MessagesSent,
		"connected_at":      c.stats.ConnectedAt,
		"last_active":       c.stats.LastActive,
		"error_count":       c.stats.ErrorCount,
		"last_error":        c.stats.LastError,
	}
	
	return stats
}

// SetKCPParameters 设置KCP参数
func (c *KCPConnection) SetKCPParameters(nodelay, interval, resend, nc int) {
	if c.conn != nil {
		c.conn.SetNoDelay(nodelay, interval, resend, nc)
	}
}

// SetWindowSize 设置窗口大小
func (c *KCPConnection) SetWindowSize(sndWnd, rcvWnd int) {
	if c.conn != nil {
		c.conn.SetWindowSize(sndWnd, rcvWnd)
	}
}

// SetMTU 设置MTU
func (c *KCPConnection) SetMTU(mtu int) {
	if c.conn != nil {
		c.conn.SetMtu(mtu)
	}
}

// SetDSCP 设置DSCP
func (c *KCPConnection) SetDSCP(dscp int) {
	if c.conn != nil {
		c.conn.SetDSCP(dscp)
	}
}

// SetACKNoDelay 设置ACK立即发送
func (c *KCPConnection) SetACKNoDelay(nodelay bool) {
	if c.conn != nil {
		c.conn.SetACKNoDelay(nodelay)
	}
}

// SetReadBuffer 设置读缓冲区大小
func (c *KCPConnection) SetReadBuffer(size int) {
	if c.conn != nil {
		c.conn.SetReadBuffer(size)
	}
}

// SetWriteBuffer 设置写缓冲区大小
func (c *KCPConnection) SetWriteBuffer(size int) {
	if c.conn != nil {
		c.conn.SetWriteBuffer(size)
	}
}

// Ping 发送ping消息
func (c *KCPConnection) Ping() error {
	pingMsg := core.Message{
		Type:      core.MessageTypeText,
		Data:      []byte("ping"),
		Timestamp: time.Now(),
	}
	
	return c.SendMessage(pingMsg)
}

// IsTimeout 检查连接是否超时
func (c *KCPConnection) IsTimeout(timeout time.Duration) bool {
	return time.Since(c.lastActive) > timeout
}

// generateConnectionID 生成连接ID
func generateConnectionID() string {
	return fmt.Sprintf("kcp-%d", time.Now().UnixNano())
}

