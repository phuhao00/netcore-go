// Package http HTTP连接实现
// Author: NetCore-Go Team
// Created: 2024

package http

import (
	"net"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// HTTPConnection HTTP连接实现
type HTTPConnection struct {
	id         string
	rawConn    net.Conn
	server     *HTTPServer
	lastActive time.Time
	mu         sync.RWMutex
	context    map[interface{}]interface{}
	closed     bool
}

// NewHTTPConnection 创建新的HTTP连接
func NewHTTPConnection(rawConn net.Conn, server *HTTPServer) *HTTPConnection {
	return &HTTPConnection{
		id:         generateConnectionID(),
		rawConn:    rawConn,
		server:     server,
		lastActive: time.Now(),
		context:    make(map[interface{}]interface{}),
	}
}

// ID 返回连接ID
func (c *HTTPConnection) ID() string {
	return c.id
}

// RemoteAddr 返回远程地址
func (c *HTTPConnection) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

// LocalAddr 返回本地地址
func (c *HTTPConnection) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

// Send 发送原始数据
func (c *HTTPConnection) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return core.ErrConnectionClosed
	}

	c.lastActive = time.Now()
	_, err := c.rawConn.Write(data)
	return err
}

// SendMessage 发送消息（HTTP连接不直接支持消息发送）
func (c *HTTPConnection) SendMessage(msg core.Message) error {
	return c.Send(msg.Data)
}

// Close 关闭连接
func (c *HTTPConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.rawConn.Close()
}

// IsActive 检查连接是否活跃
func (c *HTTPConnection) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed
}

// SetContext 设置上下文值
func (c *HTTPConnection) SetContext(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.context[key] = value
}

// GetContext 获取上下文值
func (c *HTTPConnection) GetContext(key interface{}) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.context[key]
}

// updateLastActive 更新最后活跃时间
func (c *HTTPConnection) updateLastActive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastActive = time.Now()
}

