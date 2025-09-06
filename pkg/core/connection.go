// Package core 实现NetCore-Go网络库的基础连接
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// BaseConnection 基础连接实现
type BaseConnection struct {
	id         string
	conn       net.Conn
	state      int32 // 使用atomic操作的连接状态
	ctx        context.Context
	cancel     context.CancelFunc
	contextMu  sync.RWMutex
	contextMap map[interface{}]interface{}
	lastActive int64 // 最后活跃时间戳
	closeCh    chan struct{}
	closeOnce  sync.Once
}

// NewBaseConnection 创建新的基础连接
func NewBaseConnection(conn net.Conn) *BaseConnection {
	ctx, cancel := context.WithCancel(context.Background())
	return &BaseConnection{
		id:         uuid.New().String(),
		conn:       conn,
		state:      int32(StateConnected),
		ctx:        ctx,
		cancel:     cancel,
		contextMap: make(map[interface{}]interface{}),
		lastActive: time.Now().Unix(),
		closeCh:    make(chan struct{}),
	}
}

// ID 获取连接唯一标识
func (c *BaseConnection) ID() string {
	return c.id
}

// RemoteAddr 获取远程地址
func (c *BaseConnection) RemoteAddr() net.Addr {
	if c.conn == nil {
		return nil
	}
	return c.conn.RemoteAddr()
}

// LocalAddr 获取本地地址
func (c *BaseConnection) LocalAddr() net.Addr {
	if c.conn == nil {
		return nil
	}
	return c.conn.LocalAddr()
}

// Send 发送原始数据
func (c *BaseConnection) Send(data []byte) error {
	if !c.IsActive() {
		return fmt.Errorf("connection is not active")
	}

	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// 更新最后活跃时间
	atomic.StoreInt64(&c.lastActive, time.Now().Unix())

	_, err := c.conn.Write(data)
	return err
}

// SendMessage 发送消息对象
func (c *BaseConnection) SendMessage(msg Message) error {
	return c.Send(msg.Data)
}

// Close 关闭连接
func (c *BaseConnection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		// 设置状态为断开连接中
		atomic.StoreInt32(&c.state, int32(StateDisconnecting))

		// 取消上下文
		c.cancel()

		// 关闭网络连接
		if c.conn != nil {
			err = c.conn.Close()
		}

		// 设置状态为已断开连接
		atomic.StoreInt32(&c.state, int32(StateDisconnected))

		// 关闭通道
		close(c.closeCh)
	})
	return err
}

// IsActive 检查连接是否活跃
func (c *BaseConnection) IsActive() bool {
	state := ConnectionState(atomic.LoadInt32(&c.state))
	return state == StateConnected
}

// SetContext 设置连接上下文
func (c *BaseConnection) SetContext(key, value interface{}) {
	c.contextMu.Lock()
	defer c.contextMu.Unlock()
	c.contextMap[key] = value
}

// GetContext 获取连接上下文
func (c *BaseConnection) GetContext(key interface{}) interface{} {
	c.contextMu.RLock()
	defer c.contextMu.RUnlock()
	return c.contextMap[key]
}

// GetState 获取连接状态
func (c *BaseConnection) GetState() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// SetState 设置连接状态
func (c *BaseConnection) SetState(state ConnectionState) {
	atomic.StoreInt32(&c.state, int32(state))
}

// GetLastActive 获取最后活跃时间
func (c *BaseConnection) GetLastActive() time.Time {
	timestamp := atomic.LoadInt64(&c.lastActive)
	return time.Unix(timestamp, 0)
}

// UpdateLastActive 更新最后活跃时间
func (c *BaseConnection) UpdateLastActive() {
	atomic.StoreInt64(&c.lastActive, time.Now().Unix())
}

// Context 获取连接的上下文
func (c *BaseConnection) Context() context.Context {
	return c.ctx
}

// CloseCh 获取关闭通道
func (c *BaseConnection) CloseCh() <-chan struct{} {
	return c.closeCh
}

// GetRawConn 获取原始网络连接（用于协议特定操作）
func (c *BaseConnection) GetRawConn() net.Conn {
	return c.conn
}