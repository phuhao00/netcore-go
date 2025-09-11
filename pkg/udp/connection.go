// Package udp UDP连接实现
// Author: NetCore-Go Team
// Created: 2024

package udp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
)

// UDPConnection UDP伪连接实现
// UDP是无连接协议，这里基于客户端地址创建伪连接来统一接口
type UDPConnection struct {
	id         string
	conn       *net.UDPConn // 服务器的UDP连接
	clientAddr *net.UDPAddr // 客户端地址
	lastActive int64        // 最后活跃时间（Unix纳秒）
	active     int32        // 连接是否活跃
	context    sync.Map     // 连接上下文
	closeCh    chan struct{} // 关闭信号通道
	mu         sync.RWMutex
}

// NewUDPConnection 创建新的UDP伪连接
func NewUDPConnection(conn *net.UDPConn, clientAddr *net.UDPAddr) *UDPConnection {
	return &UDPConnection{
		id:         generateConnectionID(clientAddr),
		conn:       conn,
		clientAddr: clientAddr,
		lastActive: time.Now().UnixNano(),
		active:     1,
		closeCh:    make(chan struct{}),
	}
}

// ID 获取连接唯一标识
func (c *UDPConnection) ID() string {
	return c.id
}

// RemoteAddr 获取远程地址
func (c *UDPConnection) RemoteAddr() net.Addr {
	return c.clientAddr
}

// LocalAddr 获取本地地址
func (c *UDPConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// Send 发送原始数据到客户端
func (c *UDPConnection) Send(data []byte) error {
	if !c.IsActive() {
		return fmt.Errorf("connection is closed")
	}

	n, err := c.conn.WriteToUDP(data, c.clientAddr)
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: sent %d bytes, expected %d", n, len(data))
	}

	c.UpdateLastActive()
	return nil
}

// SendMessage 发送消息对象到客户端
func (c *UDPConnection) SendMessage(msg core.Message) error {
	return c.Send(msg.Data)
}

// Close 关闭连接
func (c *UDPConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.active, 1, 0) {
		return nil // 已经关闭
	}

	close(c.closeCh)
	return nil
}

// IsActive 检查连接是否活跃
func (c *UDPConnection) IsActive() bool {
	return atomic.LoadInt32(&c.active) == 1
}

// SetContext 设置连接上下文
func (c *UDPConnection) SetContext(key, value interface{}) {
	c.context.Store(key, value)
}

// GetContext 获取连接上下文
func (c *UDPConnection) GetContext(key interface{}) interface{} {
	if value, ok := c.context.Load(key); ok {
		return value
	}
	return nil
}

// UpdateLastActive 更新最后活跃时间
func (c *UDPConnection) UpdateLastActive() {
	atomic.StoreInt64(&c.lastActive, time.Now().UnixNano())
}

// GetLastActive 获取最后活跃时间
func (c *UDPConnection) GetLastActive() time.Time {
	return time.Unix(0, atomic.LoadInt64(&c.lastActive))
}

// CloseCh 获取关闭信号通道
func (c *UDPConnection) CloseCh() <-chan struct{} {
	return c.closeCh
}

// GetClientAddr 获取客户端地址
func (c *UDPConnection) GetClientAddr() *net.UDPAddr {
	return c.clientAddr
}

// GetServerConn 获取服务器UDP连接
func (c *UDPConnection) GetServerConn() *net.UDPConn {
	return c.conn
}

// generateConnectionID 生成连接ID
func generateConnectionID(addr *net.UDPAddr) string {
	return fmt.Sprintf("udp-%s-%d", addr.IP.String(), addr.Port)
}

// String 返回连接的字符串表示
func (c *UDPConnection) String() string {
	return fmt.Sprintf("UDPConnection{id=%s, remote=%s, active=%t}", 
		c.id, c.clientAddr.String(), c.IsActive())
}

