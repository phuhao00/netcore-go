// Package queue 消息队列接口定义
// Author: NetCore-Go Team
// Created: 2024

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Priority 消息优先级
type Priority int

const (
	LowPriority Priority = iota
	NormalPriority
	HighPriority
	CriticalPriority
)

// String 返回优先级字符串
func (p Priority) String() string {
	switch p {
	case LowPriority:
		return "low"
	case NormalPriority:
		return "normal"
	case HighPriority:
		return "high"
	case CriticalPriority:
		return "critical"
	default:
		return "unknown"
	}
}

// MessageStatus 消息状态
type MessageStatus int

const (
	Pending MessageStatus = iota
	Processing
	Completed
	Failed
	Retrying
	DeadLetter
)

// String 返回状态字符串
func (s MessageStatus) String() string {
	switch s {
	case Pending:
		return "pending"
	case Processing:
		return "processing"
	case Completed:
		return "completed"
	case Failed:
		return "failed"
	case Retrying:
		return "retrying"
	case DeadLetter:
		return "dead_letter"
	default:
		return "unknown"
	}
}

// Message 队列消息
type Message struct {
	ID          string                 `json:"id"`
	Topic       string                 `json:"topic"`
	Payload     []byte                 `json:"payload"`
	Headers     map[string]string      `json:"headers"`
	Priority    Priority               `json:"priority"`
	Status      MessageStatus          `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
	Timeout     time.Duration          `json:"timeout"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewMessage 创建新消息
func NewMessage(topic string, payload []byte) *Message {
	return &Message{
		ID:        generateMessageID(),
		Topic:     topic,
		Payload:   payload,
		Headers:   make(map[string]string),
		Priority:  NormalPriority,
		Status:    Pending,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// SetHeader 设置消息头
func (m *Message) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// GetHeader 获取消息头
func (m *Message) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// SetMetadata 设置元数据
func (m *Message) SetMetadata(key string, value interface{}) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
}

// GetMetadata 获取元数据
func (m *Message) GetMetadata(key string) (interface{}, bool) {
	if m.Metadata == nil {
		return nil, false
	}
	value, exists := m.Metadata[key]
	return value, exists
}

// SetPayloadJSON 设置JSON载荷
func (m *Message) SetPayloadJSON(data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	m.Payload = payload
	m.SetHeader("Content-Type", "application/json")
	return nil
}

// GetPayloadJSON 获取JSON载荷
func (m *Message) GetPayloadJSON(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// IsExpired 检查消息是否过期
func (m *Message) IsExpired() bool {
	if m.Timeout <= 0 {
		return false
	}
	return time.Since(m.CreatedAt) > m.Timeout
}

// ShouldRetry 检查是否应该重试
func (m *Message) ShouldRetry() bool {
	return m.RetryCount < m.MaxRetries
}

// MarkProcessing 标记为处理中
func (m *Message) MarkProcessing() {
	m.Status = Processing
	now := time.Now()
	m.ProcessedAt = &now
}

// MarkCompleted 标记为完成
func (m *Message) MarkCompleted() {
	m.Status = Completed
}

// MarkFailed 标记为失败
func (m *Message) MarkFailed() {
	m.Status = Failed
}

// MarkRetrying 标记为重试
func (m *Message) MarkRetrying() {
	m.Status = Retrying
	m.RetryCount++
}

// MarkDeadLetter 标记为死信
func (m *Message) MarkDeadLetter() {
	m.Status = DeadLetter
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	Handle(ctx context.Context, msg *Message) error
}

// MessageHandlerFunc 消息处理器函数类型
type MessageHandlerFunc func(ctx context.Context, msg *Message) error

// Handle 实现MessageHandler接口
func (f MessageHandlerFunc) Handle(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

// QueueStats 队列统计信息
type QueueStats struct {
	TotalMessages     int64 `json:"total_messages"`
	PendingMessages   int64 `json:"pending_messages"`
	ProcessingMessages int64 `json:"processing_messages"`
	CompletedMessages int64 `json:"completed_messages"`
	FailedMessages    int64 `json:"failed_messages"`
	DeadLetterMessages int64 `json:"dead_letter_messages"`
	AverageProcessTime float64 `json:"average_process_time_ms"`
	Throughput        float64 `json:"throughput_per_second"`
	ErrorRate         float64 `json:"error_rate"`
	OldestMessage     *time.Time `json:"oldest_message,omitempty"`
	NewestMessage     *time.Time `json:"newest_message,omitempty"`
}

// Queue 队列接口
type Queue interface {
	// 基本操作
	Enqueue(ctx context.Context, msg *Message) error
	Dequeue(ctx context.Context, topic string) (*Message, error)
	DequeueWithTimeout(ctx context.Context, topic string, timeout time.Duration) (*Message, error)
	Ack(ctx context.Context, messageID string) error
	Nack(ctx context.Context, messageID string, requeue bool) error

	// 批量操作
	EnqueueBatch(ctx context.Context, messages []*Message) error
	DequeueBatch(ctx context.Context, topic string, count int) ([]*Message, error)

	// 延迟消息
	EnqueueDelayed(ctx context.Context, msg *Message, delay time.Duration) error
	EnqueueScheduled(ctx context.Context, msg *Message, scheduledAt time.Time) error

	// 优先级队列
	EnqueueWithPriority(ctx context.Context, msg *Message, priority Priority) error

	// 队列管理
	Purge(ctx context.Context, topic string) error
	Delete(ctx context.Context, topic string) error
	Exists(ctx context.Context, topic string) (bool, error)
	Size(ctx context.Context, topic string) (int64, error)

	// 消息管理
	GetMessage(ctx context.Context, messageID string) (*Message, error)
	UpdateMessage(ctx context.Context, msg *Message) error
	DeleteMessage(ctx context.Context, messageID string) error

	// 统计信息
	GetStats(ctx context.Context, topic string) (*QueueStats, error)
	GetGlobalStats(ctx context.Context) (*QueueStats, error)

	// 生命周期
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	HealthCheck(ctx context.Context) error
}

// Consumer 消费者接口
type Consumer interface {
	// 启动消费
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// 订阅主题
	Subscribe(topic string, handler MessageHandler) error
	Unsubscribe(topic string) error

	// 配置
	SetConcurrency(concurrency int)
	SetBatchSize(batchSize int)
	SetPollInterval(interval time.Duration)

	// 状态
	IsRunning() bool
	GetStats() *ConsumerStats
}

// ConsumerStats 消费者统计信息
type ConsumerStats struct {
	ConsumerID        string    `json:"consumer_id"`
	SubscribedTopics  []string  `json:"subscribed_topics"`
	Concurrency       int       `json:"concurrency"`
	BatchSize         int       `json:"batch_size"`
	PollInterval      time.Duration `json:"poll_interval"`
	TotalProcessed    int64     `json:"total_processed"`
	TotalFailed       int64     `json:"total_failed"`
	ProcessingRate    float64   `json:"processing_rate"`
	ErrorRate         float64   `json:"error_rate"`
	LastProcessedAt   *time.Time `json:"last_processed_at,omitempty"`
	IsRunning         bool      `json:"is_running"`
	ActiveWorkers     int       `json:"active_workers"`
}

// Producer 生产者接口
type Producer interface {
	// 发送消息
	Send(ctx context.Context, topic string, payload []byte) error
	SendMessage(ctx context.Context, msg *Message) error
	SendBatch(ctx context.Context, messages []*Message) error

	// 延迟发送
	SendDelayed(ctx context.Context, topic string, payload []byte, delay time.Duration) error
	SendScheduled(ctx context.Context, topic string, payload []byte, scheduledAt time.Time) error

	// 优先级发送
	SendWithPriority(ctx context.Context, topic string, payload []byte, priority Priority) error

	// 事务支持
	BeginTransaction(ctx context.Context) (Transaction, error)

	// 生命周期
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// 统计
	GetStats() *ProducerStats
}

// ProducerStats 生产者统计信息
type ProducerStats struct {
	ProducerID      string    `json:"producer_id"`
	TotalSent       int64     `json:"total_sent"`
	TotalFailed     int64     `json:"total_failed"`
	SendRate        float64   `json:"send_rate"`
	ErrorRate       float64   `json:"error_rate"`
	LastSentAt      *time.Time `json:"last_sent_at,omitempty"`
	AverageLatency  float64   `json:"average_latency_ms"`
}

// Transaction 事务接口
type Transaction interface {
	Send(ctx context.Context, msg *Message) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// QueueConfig 队列配置
type QueueConfig struct {
	Type              string        `json:"type"` // memory, redis, rabbitmq, kafka, etc.
	URL               string        `json:"url,omitempty"`
	MaxSize           int64         `json:"max_size"`
	MaxRetries        int           `json:"max_retries"`
	DefaultTimeout    time.Duration `json:"default_timeout"`
	VisibilityTimeout time.Duration `json:"visibility_timeout"`
	DeadLetterQueue   string        `json:"dead_letter_queue,omitempty"`
	Persistent        bool          `json:"persistent"`
	Compression       bool          `json:"compression"`
	Encryption        bool          `json:"encryption"`
	BatchSize         int           `json:"batch_size"`
	PollInterval      time.Duration `json:"poll_interval"`
	Concurrency       int           `json:"concurrency"`
	Metrics           bool          `json:"metrics"`
}

// DefaultQueueConfig 默认队列配置
func DefaultQueueConfig() *QueueConfig {
	return &QueueConfig{
		Type:              "memory",
		MaxSize:           10000,
		MaxRetries:        3,
		DefaultTimeout:    30 * time.Second,
		VisibilityTimeout: 30 * time.Second,
		Persistent:        false,
		Compression:       false,
		Encryption:        false,
		BatchSize:         10,
		PollInterval:      1 * time.Second,
		Concurrency:       5,
		Metrics:           true,
	}
}

// QueueFactory 队列工厂接口
type QueueFactory interface {
	CreateQueue(config *QueueConfig) (Queue, error)
	CreateProducer(config *QueueConfig) (Producer, error)
	CreateConsumer(config *QueueConfig) (Consumer, error)
}

// 辅助函数

var messageIDCounter int64

func generateMessageID() string {
	messageIDCounter++
	return fmt.Sprintf("msg_%d_%d", time.Now().Unix(), messageIDCounter)
}