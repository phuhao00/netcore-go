// Package rpc RPC连接实现
// Author: NetCore-Go Team
// Created: 2024

package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// RPCConnection RPC连接实现
type RPCConnection struct {
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	server   *RPCServer
	id       string
	context  map[string]interface{}
	contextMu sync.RWMutex
	closed   bool
	closeMu  sync.Mutex
	lastActive time.Time
}

// NewRPCConnection 创建RPC连接
func NewRPCConnection(conn net.Conn, server *RPCServer) *RPCConnection {
	return &RPCConnection{
		conn:       conn,
		reader:     bufio.NewReader(conn),
		writer:     bufio.NewWriter(conn),
		server:     server,
		id:        generateConnectionID(),
		context:    make(map[string]interface{}),
		lastActive: time.Now(),
	}
}

// ID 获取连接ID
func (c *RPCConnection) ID() string {
	return c.id
}

// RemoteAddr 获取远程地址
func (c *RPCConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr 获取本地地址
func (c *RPCConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// Send 发送原始数据
func (c *RPCConnection) Send(data []byte) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	
	_, err := c.writer.Write(data)
	if err != nil {
		return err
	}
	
	return c.writer.Flush()
}

// SendMessage 发送消息
func (c *RPCConnection) SendMessage(msg core.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}
	
	return c.Send(data)
}

// ReadRequest 读取RPC请求
func (c *RPCConnection) ReadRequest() (*RPCRequest, error) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return nil, fmt.Errorf("connection closed")
	}
	
	// 设置读取超时
	config := c.server.BaseServer.GetConfig()
	if config.ReadTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	}
	
	// 读取请求长度
	lengthBytes := make([]byte, 4)
	if _, err := c.reader.Read(lengthBytes); err != nil {
		return nil, err
	}
	
	length := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])
	if length <= 0 || length > 1024*1024 { // 限制最大1MB
		return nil, fmt.Errorf("invalid request length: %d", length)
	}
	
	// 读取请求数据
	requestData := make([]byte, length)
	if _, err := c.reader.Read(requestData); err != nil {
		return nil, err
	}
	
	// 解析请求
	var request RPCRequest
	if err := json.Unmarshal(requestData, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %v", err)
	}
	
	c.lastActive = time.Now()
	return &request, nil
}

// WriteResponse 写入RPC响应
func (c *RPCConnection) WriteResponse(response *RPCResponse) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	
	// 设置写入超时
	config := c.server.BaseServer.GetConfig()
	if config.WriteTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
	}
	
	// 序列化响应
	responseData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %v", err)
	}
	
	// 写入响应长度
	length := len(responseData)
	lengthBytes := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	
	if _, err := c.writer.Write(lengthBytes); err != nil {
		return err
	}
	
	// 写入响应数据
	if _, err := c.writer.Write(responseData); err != nil {
		return err
	}
	
	return c.writer.Flush()
}

// Close 关闭连接
func (c *RPCConnection) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	return c.conn.Close()
}

// IsActive 检查连接是否活跃
func (c *RPCConnection) IsActive() bool {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	
	return !c.closed
}

// SetContext 设置上下文
func (c *RPCConnection) SetContext(key, value interface{}) {
	c.contextMu.Lock()
	defer c.contextMu.Unlock()
	
	c.context[fmt.Sprintf("%v", key)] = value
}

// GetContext 获取上下文
func (c *RPCConnection) GetContext(key interface{}) interface{} {
	c.contextMu.RLock()
	defer c.contextMu.RUnlock()
	
	return c.context[fmt.Sprintf("%v", key)]
}

// LastActive 获取最后活跃时间
func (c *RPCConnection) LastActive() time.Time {
	return c.lastActive
}

// generateConnectionID 生成连接ID
func generateConnectionID() string {
	return fmt.Sprintf("rpc-%d", time.Now().UnixNano())
}

