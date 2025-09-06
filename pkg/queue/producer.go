// Package queue 生产者实现
// Author: NetCore-Go Team
// Created: 2024

package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryProducer 内存队列生产者
type MemoryProducer struct {
	queue   Queue
	config  *QueueConfig
	stats   *ProducerStats
	mutex   sync.RWMutex
	running int32
	latencies []time.Duration
}

// NewMemoryProducer 创建内存队列生产者
func NewMemoryProducer(queue Queue, config *QueueConfig) *MemoryProducer {
	if config == nil {
		config = DefaultQueueConfig()
	}

	return &MemoryProducer{
		queue:  queue,
		config: config,
		stats: &ProducerStats{
			ProducerID: generateProducerID(),
		},
		latencies: make([]time.Duration, 0, 1000),
	}
}

// Start 启动生产者
func (p *MemoryProducer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return fmt.Errorf("producer already running")
	}
	return nil
}

// Stop 停止生产者
func (p *MemoryProducer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return fmt.Errorf("producer not running")
	}
	return nil
}

// Send 发送消息
func (p *MemoryProducer) Send(ctx context.Context, topic string, payload []byte) error {
	msg := NewMessage(topic, payload)
	return p.SendMessage(ctx, msg)
}

// SendMessage 发送消息对象
func (p *MemoryProducer) SendMessage(ctx context.Context, msg *Message) error {
	if atomic.LoadInt32(&p.running) != 1 {
		return fmt.Errorf("producer not running")
	}

	start := time.Now()
	err := p.queue.Enqueue(ctx, msg)
	latency := time.Since(start)

	// 更新统计
	p.updateStats(latency, err == nil)

	return err
}

// SendBatch 批量发送消息
func (p *MemoryProducer) SendBatch(ctx context.Context, messages []*Message) error {
	if atomic.LoadInt32(&p.running) != 1 {
		return fmt.Errorf("producer not running")
	}

	start := time.Now()
	err := p.queue.EnqueueBatch(ctx, messages)
	latency := time.Since(start)

	// 更新统计
	for range messages {
		p.updateStats(latency/time.Duration(len(messages)), err == nil)
	}

	return err
}

// SendDelayed 发送延迟消息
func (p *MemoryProducer) SendDelayed(ctx context.Context, topic string, payload []byte, delay time.Duration) error {
	msg := NewMessage(topic, payload)
	return p.queue.EnqueueDelayed(ctx, msg, delay)
}

// SendScheduled 发送定时消息
func (p *MemoryProducer) SendScheduled(ctx context.Context, topic string, payload []byte, scheduledAt time.Time) error {
	msg := NewMessage(topic, payload)
	return p.queue.EnqueueScheduled(ctx, msg, scheduledAt)
}

// SendWithPriority 发送带优先级的消息
func (p *MemoryProducer) SendWithPriority(ctx context.Context, topic string, payload []byte, priority Priority) error {
	msg := NewMessage(topic, payload)
	return p.queue.EnqueueWithPriority(ctx, msg, priority)
}

// BeginTransaction 开始事务（内存队列暂不支持事务）
func (p *MemoryProducer) BeginTransaction(ctx context.Context) (Transaction, error) {
	return nil, fmt.Errorf("transactions not supported in memory producer")
}

// GetStats 获取统计信息
func (p *MemoryProducer) GetStats() *ProducerStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 计算平均延迟
	if len(p.latencies) > 0 {
		var total time.Duration
		for _, latency := range p.latencies {
			total += latency
		}
		p.stats.AverageLatency = float64(total.Nanoseconds()) / float64(len(p.latencies)) / 1e6
	}

	// 计算错误率
	if p.stats.TotalSent > 0 {
		p.stats.ErrorRate = float64(p.stats.TotalFailed) / float64(p.stats.TotalSent)
	}

	// 深拷贝统计信息
	stats := *p.stats
	return &stats
}

// updateStats 更新统计信息
func (p *MemoryProducer) updateStats(latency time.Duration, success bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.stats.TotalSent++
	now := time.Now()
	p.stats.LastSentAt = &now

	if success {
		// 记录延迟
		p.latencies = append(p.latencies, latency)
		if len(p.latencies) > 1000 {
			p.latencies = p.latencies[1:]
		}
	} else {
		p.stats.TotalFailed++
	}

	// 计算发送速率（简化版本）
	p.stats.SendRate = float64(p.stats.TotalSent) / time.Since(now).Seconds()
}

// MemoryConsumer 内存队列消费者
type MemoryConsumer struct {
	queue       Queue
	config      *QueueConfig
	stats       *ConsumerStats
	handlers    map[string]MessageHandler
	mutex       sync.RWMutex
	running     int32
	workers     []*worker
	workerPool  chan *worker
	ctx         context.Context
	cancel      context.CancelFunc
}

// worker 工作协程
type worker struct {
	id       int
	consumer *MemoryConsumer
	active   int32
}

// NewMemoryConsumer 创建内存队列消费者
func NewMemoryConsumer(queue Queue, config *QueueConfig) *MemoryConsumer {
	if config == nil {
		config = DefaultQueueConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MemoryConsumer{
		queue:   queue,
		config:  config,
		stats: &ConsumerStats{
			ConsumerID:   generateConsumerID(),
			Concurrency:  config.Concurrency,
			BatchSize:    config.BatchSize,
			PollInterval: config.PollInterval,
		},
		handlers:   make(map[string]MessageHandler),
		workers:    make([]*worker, 0, config.Concurrency),
		workerPool: make(chan *worker, config.Concurrency),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动消费者
func (c *MemoryConsumer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return fmt.Errorf("consumer already running")
	}

	// 创建工作协程
	for i := 0; i < c.config.Concurrency; i++ {
		w := &worker{
			id:       i,
			consumer: c,
		}
		c.workers = append(c.workers, w)
		c.workerPool <- w
		go w.run(c.ctx)
	}

	c.stats.IsRunning = true
	return nil
}

// Stop 停止消费者
func (c *MemoryConsumer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&c.running, 1, 0) {
		return fmt.Errorf("consumer not running")
	}

	c.cancel()
	c.stats.IsRunning = false
	return nil
}

// Subscribe 订阅主题
func (c *MemoryConsumer) Subscribe(topic string, handler MessageHandler) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.handlers[topic] = handler
	c.stats.SubscribedTopics = append(c.stats.SubscribedTopics, topic)
	return nil
}

// Unsubscribe 取消订阅
func (c *MemoryConsumer) Unsubscribe(topic string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.handlers, topic)

	// 从订阅列表中移除
	for i, t := range c.stats.SubscribedTopics {
		if t == topic {
			c.stats.SubscribedTopics = append(c.stats.SubscribedTopics[:i], c.stats.SubscribedTopics[i+1:]...)
			break
		}
	}

	return nil
}

// SetConcurrency 设置并发数
func (c *MemoryConsumer) SetConcurrency(concurrency int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.stats.Concurrency = concurrency
	// 注意：这里简化实现，实际应该动态调整工作协程数量
}

// SetBatchSize 设置批处理大小
func (c *MemoryConsumer) SetBatchSize(batchSize int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.stats.BatchSize = batchSize
}

// SetPollInterval 设置轮询间隔
func (c *MemoryConsumer) SetPollInterval(interval time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.stats.PollInterval = interval
}

// IsRunning 检查是否正在运行
func (c *MemoryConsumer) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

// GetStats 获取统计信息
func (c *MemoryConsumer) GetStats() *ConsumerStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 计算处理速率
	if c.stats.TotalProcessed > 0 {
		c.stats.ProcessingRate = float64(c.stats.TotalProcessed) / time.Since(time.Now()).Seconds()
	}

	// 计算错误率
	if c.stats.TotalProcessed > 0 {
		c.stats.ErrorRate = float64(c.stats.TotalFailed) / float64(c.stats.TotalProcessed)
	}

	// 统计活跃工作协程
	activeWorkers := 0
	for _, w := range c.workers {
		if atomic.LoadInt32(&w.active) == 1 {
			activeWorkers++
		}
	}
	c.stats.ActiveWorkers = activeWorkers

	// 深拷贝统计信息
	stats := *c.stats
	stats.SubscribedTopics = make([]string, len(c.stats.SubscribedTopics))
	copy(stats.SubscribedTopics, c.stats.SubscribedTopics)

	return &stats
}

// worker.run 工作协程运行逻辑
func (w *worker) run(ctx context.Context) {
	ticker := time.NewTicker(w.consumer.stats.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processMessages(ctx)
		}
	}
}

// processMessages 处理消息
func (w *worker) processMessages(ctx context.Context) {
	w.consumer.mutex.RLock()
	handlers := make(map[string]MessageHandler)
	for topic, handler := range w.consumer.handlers {
		handlers[topic] = handler
	}
	w.consumer.mutex.RUnlock()

	// 处理每个订阅的主题
	for topic, handler := range handlers {
		// 批量获取消息
		messages, err := w.consumer.queue.DequeueBatch(ctx, topic, w.consumer.stats.BatchSize)
		if err != nil {
			continue
		}

		// 处理每个消息
		for _, msg := range messages {
			w.processMessage(ctx, msg, handler)
		}
	}
}

// processMessage 处理单个消息
func (w *worker) processMessage(ctx context.Context, msg *Message, handler MessageHandler) {
	atomic.StoreInt32(&w.active, 1)
	defer atomic.StoreInt32(&w.active, 0)

	start := time.Now()
	err := handler.Handle(ctx, msg)
	duration := time.Since(start)

	// 更新统计
	w.consumer.mutex.Lock()
	w.consumer.stats.TotalProcessed++
	now := time.Now()
	w.consumer.stats.LastProcessedAt = &now

	if err == nil {
		// 确认消息处理完成
		w.consumer.queue.Ack(ctx, msg.ID)
	} else {
		// 处理失败，拒绝消息
		w.consumer.stats.TotalFailed++
		w.consumer.queue.Nack(ctx, msg.ID, msg.ShouldRetry())
	}
	w.consumer.mutex.Unlock()

	_ = duration // 可以用于计算平均处理时间
}

// 辅助函数

var (
	producerIDCounter int64
	consumerIDCounter int64
)

func generateProducerID() string {
	producerIDCounter++
	return fmt.Sprintf("producer_%d_%d", time.Now().Unix(), producerIDCounter)
}

func generateConsumerID() string {
	consumerIDCounter++
	return fmt.Sprintf("consumer_%d_%d", time.Now().Unix(), consumerIDCounter)
}