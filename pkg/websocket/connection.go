// Package websocket WebSocket连接实现
// Author: NetCore-Go Team
// Created: 2024

package websocket

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// Connection WebSocket连接
type Connection struct {
	mu         sync.RWMutex
	id         string
	conn       net.Conn
	config     *core.ServerConfig
	active     bool
	lastActive time.Time
	context    map[interface{}]interface{}
}

// NewConnection 创建WebSocket连接
func NewConnection(conn net.Conn, config *core.ServerConfig) *Connection {
	return &Connection{
		id:         generateConnectionID(),
		conn:       conn,
		config:     config,
		active:     true,
		lastActive: time.Now(),
		context:    make(map[interface{}]interface{}),
	}
}

// ID 获取连接ID
func (c *Connection) ID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// RemoteAddr 获取远程地址
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr 获取本地地址
func (c *Connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// Send 发送原始数据
func (c *Connection) Send(data []byte) error {
	// WebSocket需要封装成帧
	msg := core.Message{
		Type:      core.MessageTypeBinary,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.SendMessage(msg)
}

// SendMessage 发送消息
func (c *Connection) SendMessage(msg core.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return fmt.Errorf("connection is closed")
	}

	// 设置写超时
	if c.config.WriteTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}

	// 构造WebSocket帧
	frame, err := c.buildFrame(msg)
	if err != nil {
		return fmt.Errorf("failed to build frame: %w", err)
	}

	// 发送帧
	_, err = c.conn.Write(frame)
	if err != nil {
		c.active = false
		return fmt.Errorf("failed to write frame: %w", err)
	}

	return nil
}

// ReadMessage 读取消息
func (c *Connection) ReadMessage() (core.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return core.Message{}, fmt.Errorf("connection is closed")
	}

	// 设置读超时
	if c.config.ReadTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	// 读取帧头
	header := make([]byte, 2)
	_, err := c.conn.Read(header)
	if err != nil {
		c.active = false
		return core.Message{}, fmt.Errorf("failed to read frame header: %w", err)
	}

	// 解析帧头
	fin := (header[0] & 0x80) != 0
	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	payloadLen := int(header[1] & 0x7F)

	// 读取扩展长度
	if payloadLen == 126 {
		extLen := make([]byte, 2)
		_, err = c.conn.Read(extLen)
		if err != nil {
			c.active = false
			return core.Message{}, fmt.Errorf("failed to read extended length: %w", err)
		}
		payloadLen = int(binary.BigEndian.Uint16(extLen))
	} else if payloadLen == 127 {
		extLen := make([]byte, 8)
		_, err = c.conn.Read(extLen)
		if err != nil {
			c.active = false
			return core.Message{}, fmt.Errorf("failed to read extended length: %w", err)
		}
		payloadLen = int(binary.BigEndian.Uint64(extLen))
	}

	// 读取掩码
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		_, err = c.conn.Read(maskKey)
		if err != nil {
			c.active = false
			return core.Message{}, fmt.Errorf("failed to read mask key: %w", err)
		}
	}

	// 读取载荷数据
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		_, err = c.conn.Read(payload)
		if err != nil {
			c.active = false
			return core.Message{}, fmt.Errorf("failed to read payload: %w", err)
		}

		// 解除掩码
		if masked {
			for i := 0; i < len(payload); i++ {
				payload[i] ^= maskKey[i%4]
			}
		}
	}

	// 更新最后活跃时间
	c.lastActive = time.Now()

	// 构造消息
	msg := core.Message{
		Type:      c.opcodeToMessageType(opcode),
		Data:      payload,
		Timestamp: time.Now(),
	}

	// 处理分片消息（简化实现，不支持分片）
	if !fin {
		return core.Message{}, fmt.Errorf("fragmented messages not supported")
	}

	return msg, nil
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return nil
	}

	c.active = false

	// 发送关闭帧
	closeFrame := []byte{0x88, 0x00} // FIN=1, OPCODE=8 (Close), No payload
	c.conn.Write(closeFrame)

	// 关闭底层连接
	return c.conn.Close()
}

// IsActive 检查连接是否活跃
func (c *Connection) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

// SetContext 设置上下文
func (c *Connection) SetContext(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.context[key] = value
}

// GetContext 获取上下文
func (c *Connection) GetContext(key interface{}) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.context[key]
}

// LastActive 获取最后活跃时间
func (c *Connection) LastActive() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastActive
}

// UpdateLastActive 更新最后活跃时间
func (c *Connection) UpdateLastActive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastActive = time.Now()
}

// buildFrame 构造WebSocket帧
func (c *Connection) buildFrame(msg core.Message) ([]byte, error) {
	opcode := c.messageTypeToOpcode(msg.Type)
	payload := msg.Data
	payloadLen := len(payload)

	// 计算帧大小
	frameSize := 2 // 基本头部
	if payloadLen < 126 {
		// 短长度
	} else if payloadLen < 65536 {
		frameSize += 2 // 扩展长度16位
	} else {
		frameSize += 8 // 扩展长度64位
	}
	frameSize += payloadLen

	// 构造帧
	frame := make([]byte, frameSize)
	offset := 0

	// 第一个字节：FIN=1, RSV=000, OPCODE
	frame[offset] = 0x80 | byte(opcode)
	offset++

	// 第二个字节：MASK=0, Payload Length
	if payloadLen < 126 {
		frame[offset] = byte(payloadLen)
		offset++
	} else if payloadLen < 65536 {
		frame[offset] = 126
		offset++
		binary.BigEndian.PutUint16(frame[offset:], uint16(payloadLen))
		offset += 2
	} else {
		frame[offset] = 127
		offset++
		binary.BigEndian.PutUint64(frame[offset:], uint64(payloadLen))
		offset += 8
	}

	// 载荷数据
	copy(frame[offset:], payload)

	return frame, nil
}

// messageTypeToOpcode 消息类型转操作码
func (c *Connection) messageTypeToOpcode(msgType core.MessageType) byte {
	switch msgType {
	case core.MessageTypeText:
		return OpCodeText
	case core.MessageTypeBinary:
		return OpCodeBinary
	case core.MessageTypePing:
		return OpCodePing
	case core.MessageTypePong:
		return OpCodePong
	case core.MessageTypeClose:
		return OpCodeClose
	default:
		return OpCodeBinary
	}
}

// opcodeToMessageType 操作码转消息类型
func (c *Connection) opcodeToMessageType(opcode byte) core.MessageType {
	switch opcode {
	case OpCodeText:
		return core.MessageTypeText
	case OpCodeBinary:
		return core.MessageTypeBinary
	case OpCodePing:
		return core.MessageTypePing
	case OpCodePong:
		return core.MessageTypePong
	case OpCodeClose:
		return core.MessageTypeClose
	default:
		return core.MessageTypeBinary
	}
}

// generateConnectionID 生成连接ID
func generateConnectionID() string {
	return fmt.Sprintf("ws-%d", time.Now().UnixNano())
}

