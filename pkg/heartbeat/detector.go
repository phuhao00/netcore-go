// Package heartbeat 心跳检测和重连机制实现
// Author: NetCore-Go Team
// Created: 2024

package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
)

// HeartbeatType 心跳类型
type HeartbeatType int

const (
	Ping HeartbeatType = iota
	Pong
	KeepAlive
	Custom
)

// String 返回心跳类型字符串
func (h HeartbeatType) String() string {
	switch h {
	case Ping:
		return "ping"
	case Pong:
		return "pong"
	case KeepAlive:
		return "keepalive"
	case Custom:
		return "custom"
	default:
		return "unknown"
	}
}

// HeartbeatMessage 心跳消息
type HeartbeatMessage struct {
	Type      HeartbeatType          `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      interface{}            `json:"data,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewHeartbeatMessage 创建心跳消息
func NewHeartbeatMessage(hbType HeartbeatType, data interface{}) *HeartbeatMessage {
	return &HeartbeatMessage{
		Type:      hbType,
		Timestamp: time.Now().Unix(),
		Data:      data,
		Metadata:  make(map[string]interface{}),
	}
}

// SetMetadata 设置元数据
func (h *HeartbeatMessage) SetMetadata(key string, value interface{}) {
	if h.Metadata == nil {
		h.Metadata = make(map[string]interface{})
	}
	h.Metadata[key] = value
}

// GetMetadata 获取元数据
func (h *HeartbeatMessage) GetMetadata(key string) (interface{}, bool) {
	if h.Metadata == nil {
		return nil, false
	}
	value, exists := h.Metadata[key]
	return value, exists
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Interval          time.Duration // 心跳间隔
	Timeout           time.Duration // 心跳超时时间
	MaxMissed         int           // 最大丢失心跳数
	Enabled           bool          // 是否启用心跳
	AutoReconnect     bool          // 是否自动重连
	ReconnectInterval time.Duration // 重连间隔
	MaxReconnectTries int           // 最大重连次数
	PingMessage       func() *HeartbeatMessage
	PongMessage       func(*HeartbeatMessage) *HeartbeatMessage
	OnHeartbeatSent   func(conn core.Connection, msg *HeartbeatMessage)
	OnHeartbeatRecv   func(conn core.Connection, msg *HeartbeatMessage)
	OnHeartbeatMissed func(conn core.Connection, missedCount int)
	OnConnectionLost  func(conn core.Connection, reason string)
	OnReconnectStart  func(conn core.Connection, attempt int)
	OnReconnectSuccess func(conn core.Connection, attempt int)
	OnReconnectFailed func(conn core.Connection, attempt int, err error)
}

// DefaultHeartbeatConfig 默认心跳配置
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval:          30 * time.Second,
		Timeout:           10 * time.Second,
		MaxMissed:         3,
		Enabled:           true,
		AutoReconnect:     true,
		ReconnectInterval: 5 * time.Second,
		MaxReconnectTries: 5,
		PingMessage: func() *HeartbeatMessage {
			return NewHeartbeatMessage(Ping, "ping")
		},
		PongMessage: func(ping *HeartbeatMessage) *HeartbeatMessage {
			pong := NewHeartbeatMessage(Pong, "pong")
			if ping != nil {
				pong.SetMetadata("ping_timestamp", ping.Timestamp)
			}
			return pong
		},
	}
}

// HeartbeatStats 心跳统计
type HeartbeatStats struct {
	ConnectionID      string    `json:"connection_id"`
	StartTime         time.Time `json:"start_time"`
	LastPingSent      time.Time `json:"last_ping_sent"`
	LastPongReceived  time.Time `json:"last_pong_received"`
	TotalPingsSent    int64     `json:"total_pings_sent"`
	TotalPongsReceived int64    `json:"total_pongs_received"`
	MissedHeartbeats  int       `json:"missed_heartbeats"`
	AverageLatency    float64   `json:"average_latency_ms"`
	MinLatency        float64   `json:"min_latency_ms"`
	MaxLatency        float64   `json:"max_latency_ms"`
	ReconnectAttempts int       `json:"reconnect_attempts"`
	LastReconnectTime time.Time `json:"last_reconnect_time"`
	IsConnected       bool      `json:"is_connected"`
	ConnectionLost    int64     `json:"connection_lost_count"`
}

// HeartbeatDetector 心跳检测器
type HeartbeatDetector struct {
	config     *HeartbeatConfig
	connection core.Connection
	stats      *HeartbeatStats
	ctx        context.Context
	cancel     context.CancelFunc
	mutex      sync.RWMutex
	running    int32
	latencies  []float64
	ticker     *time.Ticker
	timeout    *time.Timer
}

// NewHeartbeatDetector 创建心跳检测器
func NewHeartbeatDetector(conn core.Connection, config *HeartbeatConfig) *HeartbeatDetector {
	if config == nil {
		config = DefaultHeartbeatConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HeartbeatDetector{
		config:     config,
		connection: conn,
		stats: &HeartbeatStats{
			ConnectionID: conn.ID(),
			StartTime:    time.Now(),
			IsConnected:  true,
			MinLatency:   float64(^uint64(0) >> 1), // 最大值
		},
		ctx:       ctx,
		cancel:    cancel,
		latencies: make([]float64, 0, 100),
	}
}

// Start 启动心跳检测
func (h *HeartbeatDetector) Start() error {
	if !h.config.Enabled {
		return nil
	}

	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		return fmt.Errorf("heartbeat detector already running")
	}

	h.ticker = time.NewTicker(h.config.Interval)
	go h.heartbeatLoop()

	return nil
}

// Stop 停止心跳检测
func (h *HeartbeatDetector) Stop() {
	if !atomic.CompareAndSwapInt32(&h.running, 1, 0) {
		return
	}

	h.cancel()
	if h.ticker != nil {
		h.ticker.Stop()
	}
	if h.timeout != nil {
		h.timeout.Stop()
	}
}

// IsRunning 检查是否正在运行
func (h *HeartbeatDetector) IsRunning() bool {
	return atomic.LoadInt32(&h.running) == 1
}

// GetStats 获取统计信息
func (h *HeartbeatDetector) GetStats() *HeartbeatStats {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// 计算平均延迟
	if len(h.latencies) > 0 {
		var total float64
		for _, latency := range h.latencies {
			total += latency
		}
		h.stats.AverageLatency = total / float64(len(h.latencies))
	}

	// 深拷贝统计信息
	stats := *h.stats
	return &stats
}

// OnHeartbeatReceived 处理接收到的心跳消息
func (h *HeartbeatDetector) OnHeartbeatReceived(msg *HeartbeatMessage) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	switch msg.Type {
	case Ping:
		// 收到Ping，发送Pong
		if h.config.PongMessage != nil {
			pong := h.config.PongMessage(msg)
			h.sendHeartbeat(pong)
		}

	case Pong:
		// 收到Pong，更新统计
		h.stats.TotalPongsReceived++
		h.stats.LastPongReceived = time.Now()
		h.stats.MissedHeartbeats = 0

		// 计算延迟
		if pingTimestamp, exists := msg.GetMetadata("ping_timestamp"); exists {
			if pingTime, ok := pingTimestamp.(int64); ok {
				latency := float64(time.Now().Unix()-pingTime) * 1000 // 毫秒
				h.updateLatency(latency)
			}
		}

		// 重置超时定时器
		if h.timeout != nil {
			h.timeout.Stop()
		}

	case KeepAlive:
		// 收到KeepAlive，更新最后活跃时间
		h.stats.LastPongReceived = time.Now()
		h.stats.MissedHeartbeats = 0
	}

	// 调用回调函数
	if h.config.OnHeartbeatRecv != nil {
		h.config.OnHeartbeatRecv(h.connection, msg)
	}
}

// heartbeatLoop 心跳循环
func (h *HeartbeatDetector) heartbeatLoop() {
	defer func() {
		atomic.StoreInt32(&h.running, 0)
	}()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.ticker.C:
			h.sendPing()
		}
	}
}

// sendPing 发送Ping消息
func (h *HeartbeatDetector) sendPing() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.config.PingMessage != nil {
		ping := h.config.PingMessage()
		ping.SetMetadata("ping_timestamp", time.Now().Unix())

		if err := h.sendHeartbeat(ping); err == nil {
			h.stats.TotalPingsSent++
			h.stats.LastPingSent = time.Now()

			// 设置超时定时器
			if h.timeout != nil {
				h.timeout.Stop()
			}
			h.timeout = time.AfterFunc(h.config.Timeout, h.onHeartbeatTimeout)

			// 调用回调函数
			if h.config.OnHeartbeatSent != nil {
				h.config.OnHeartbeatSent(h.connection, ping)
			}
		} else {
			// 发送失败，可能连接已断开
			h.onConnectionLost("failed to send heartbeat: " + err.Error())
		}
	}
}

// sendHeartbeat 发送心跳消息
func (h *HeartbeatDetector) sendHeartbeat(msg *HeartbeatMessage) error {
	// 这里需要根据具体的连接类型来发送消息
	// 简化实现，假设连接有SendMessage方法
	if h.connection == nil || !h.connection.IsActive() {
		return fmt.Errorf("connection is not active")
	}

	// 创建核心消息
	coreMsg := &core.Message{
		Type:      core.MessageTypeJSON,
		Timestamp: time.Now(),
		Headers:   map[string]string{"type": "heartbeat"},
	}

	// 序列化心跳消息
	data, err := msg.ToJSON()
	if err != nil {
		return err
	}
	coreMsg.Data = data

	return h.connection.SendMessage(*coreMsg)
}

// onHeartbeatTimeout 心跳超时处理
func (h *HeartbeatDetector) onHeartbeatTimeout() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.stats.MissedHeartbeats++

	// 调用回调函数
	if h.config.OnHeartbeatMissed != nil {
		h.config.OnHeartbeatMissed(h.connection, h.stats.MissedHeartbeats)
	}

	// 检查是否超过最大丢失次数
	if h.stats.MissedHeartbeats >= h.config.MaxMissed {
		h.onConnectionLost(fmt.Sprintf("missed %d heartbeats", h.stats.MissedHeartbeats))
	}
}

// onConnectionLost 连接丢失处理
func (h *HeartbeatDetector) onConnectionLost(reason string) {
	h.stats.IsConnected = false
	h.stats.ConnectionLost++

	// 调用回调函数
	if h.config.OnConnectionLost != nil {
		h.config.OnConnectionLost(h.connection, reason)
	}

	// 如果启用自动重连，开始重连
	if h.config.AutoReconnect {
		go h.startReconnect()
	}
}

// startReconnect 开始重连
func (h *HeartbeatDetector) startReconnect() {
	for attempt := 1; attempt <= h.config.MaxReconnectTries; attempt++ {
		select {
		case <-h.ctx.Done():
			return
		default:
		}

		h.mutex.Lock()
		h.stats.ReconnectAttempts = attempt
		h.stats.LastReconnectTime = time.Now()
		h.mutex.Unlock()

		// 调用重连开始回调
		if h.config.OnReconnectStart != nil {
			h.config.OnReconnectStart(h.connection, attempt)
		}

		// 尝试重连（这里需要根据具体的连接类型实现）
		if err := h.reconnect(); err == nil {
			// 重连成功
			h.mutex.Lock()
			h.stats.IsConnected = true
			h.stats.MissedHeartbeats = 0
			h.mutex.Unlock()

			// 调用重连成功回调
			if h.config.OnReconnectSuccess != nil {
				h.config.OnReconnectSuccess(h.connection, attempt)
			}

			// 重新启动心跳检测
			if !h.IsRunning() {
				h.Start()
			}
			return
		} else {
			// 重连失败
			if h.config.OnReconnectFailed != nil {
				h.config.OnReconnectFailed(h.connection, attempt, err)
			}

			// 等待重连间隔
			if attempt < h.config.MaxReconnectTries {
				time.Sleep(h.config.ReconnectInterval)
			}
		}
	}
}

// reconnect 重连实现（需要根据具体连接类型实现）
func (h *HeartbeatDetector) reconnect() error {
	// 这里是简化实现，实际需要根据连接类型来实现重连逻辑
	if h.connection == nil {
		return fmt.Errorf("connection is nil")
	}

	// 检查连接是否已经活跃
	if h.connection.IsActive() {
		return nil
	}

	// 实际的重连逻辑需要在具体的连接实现中完成
	return fmt.Errorf("reconnect not implemented for this connection type")
}

// updateLatency 更新延迟统计
func (h *HeartbeatDetector) updateLatency(latency float64) {
	h.latencies = append(h.latencies, latency)
	if len(h.latencies) > 100 {
		h.latencies = h.latencies[1:] // 保持最近100个延迟记录
	}

	if latency < h.stats.MinLatency {
		h.stats.MinLatency = latency
	}
	if latency > h.stats.MaxLatency {
		h.stats.MaxLatency = latency
	}
}

// ToJSON 转换为JSON
func (h *HeartbeatMessage) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}

// FromJSON 从JSON解析
func (h *HeartbeatMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, h)
}

