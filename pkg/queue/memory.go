// Package queue 内存队列实现
// Author: NetCore-Go Team
// Created: 2024

package queue

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryQueue 内存队列实现
type MemoryQueue struct {
	config    *QueueConfig
	topics    map[string]*topicQueue
	mutex     sync.RWMutex
	running   int32
	stats     *QueueStats
	statsMutex sync.RWMutex
	processTimes []time.Duration
	cleanupTicker *time.Ticker
}

// topicQueue 主题队列
type topicQueue struct {
	messages    []*Message
	processing  map[string]*Message // 正在处理的消息
	deadLetter  []*Message
	mutex       sync.RWMutex
	lastAccess  time.Time
	notifyChan  chan struct{}
}

// NewMemoryQueue 创建内存队列
func NewMemoryQueue(config *QueueConfig) *MemoryQueue {
	if config == nil {
		config = DefaultQueueConfig()
	}

	return &MemoryQueue{
		config:       config,
		topics:       make(map[string]*topicQueue),
		stats:        &QueueStats{},
		processTimes: make([]time.Duration, 0, 1000),
		cleanupTicker: time.NewTicker(time.Minute),
	}
}

// Start 启动队列
func (q *MemoryQueue) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&q.running, 0, 1) {
		return fmt.Errorf("queue already running")
	}

	// 启动清理协程
	go q.cleanup(ctx)

	return nil
}

// Stop 停止队列
func (q *MemoryQueue) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&q.running, 1, 0) {
		return fmt.Errorf("queue not running")
	}

	q.cleanupTicker.Stop()
	return nil
}

// HealthCheck 健康检查
func (q *MemoryQueue) HealthCheck(ctx context.Context) error {
	if atomic.LoadInt32(&q.running) != 1 {
		return fmt.Errorf("queue not running")
	}
	return nil
}

// Enqueue 入队消息
func (q *MemoryQueue) Enqueue(ctx context.Context, msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	// 检查队列大小限制
	if q.config.MaxSize > 0 {
		if q.getTotalSize() >= q.config.MaxSize {
			return fmt.Errorf("queue is full")
		}
	}

	// 设置默认值
	if msg.MaxRetries == 0 {
		msg.MaxRetries = q.config.MaxRetries
	}
	if msg.Timeout == 0 {
		msg.Timeout = q.config.DefaultTimeout
	}

	topic := q.getOrCreateTopic(msg.Topic)
	topic.mutex.Lock()
	defer topic.mutex.Unlock()

	// 根据优先级插入消息
	q.insertByPriority(topic, msg)
	topic.lastAccess = time.Now()

	// 通知等待的消费者
	select {
	case topic.notifyChan <- struct{}{}:
	default:
	}

	// 更新统计
	q.updateStats(func(stats *QueueStats) {
		stats.TotalMessages++
		stats.PendingMessages++
		if stats.NewestMessage == nil || msg.CreatedAt.After(*stats.NewestMessage) {
			stats.NewestMessage = &msg.CreatedAt
		}
	})

	return nil
}

// Dequeue 出队消息
func (q *MemoryQueue) Dequeue(ctx context.Context, topicName string) (*Message, error) {
	return q.DequeueWithTimeout(ctx, topicName, 0)
}

// DequeueWithTimeout 带超时的出队
func (q *MemoryQueue) DequeueWithTimeout(ctx context.Context, topicName string, timeout time.Duration) (*Message, error) {
	topic := q.getTopic(topicName)
	if topic == nil {
		return nil, fmt.Errorf("topic not found: %s", topicName)
	}

	// 尝试立即获取消息
	if msg := q.dequeueMessage(topic); msg != nil {
		return msg, nil
	}

	// 如果没有超时设置，直接返回
	if timeout == 0 {
		return nil, nil
	}

	// 等待新消息或超时
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, nil
		case <-topic.notifyChan:
			if msg := q.dequeueMessage(topic); msg != nil {
				return msg, nil
			}
		}
	}
}

// dequeueMessage 从主题队列中取出消息
func (q *MemoryQueue) dequeueMessage(topic *topicQueue) *Message {
	topic.mutex.Lock()
	defer topic.mutex.Unlock()

	// 检查是否有可用消息
	for i, msg := range topic.messages {
		// 跳过已过期的消息
		if msg.IsExpired() {
			msg.MarkDeadLetter()
			topic.deadLetter = append(topic.deadLetter, msg)
			topic.messages = append(topic.messages[:i], topic.messages[i+1:]...)
			continue
		}

		// 跳过延迟消息
		if msg.ScheduledAt != nil && time.Now().Before(*msg.ScheduledAt) {
			continue
		}

		// 找到可处理的消息
		msg.MarkProcessing()
		topic.processing[msg.ID] = msg
		topic.messages = append(topic.messages[:i], topic.messages[i+1:]...)
		topic.lastAccess = time.Now()

		// 更新统计
		q.updateStats(func(stats *QueueStats) {
			stats.PendingMessages--
			stats.ProcessingMessages++
			if stats.OldestMessage == nil || msg.CreatedAt.Before(*stats.OldestMessage) {
				stats.OldestMessage = &msg.CreatedAt
			}
		})

		return msg
	}

	return nil
}

// Ack 确认消息处理完成
func (q *MemoryQueue) Ack(ctx context.Context, messageID string) error {
	topic, msg := q.findProcessingMessage(messageID)
	if topic == nil || msg == nil {
		return fmt.Errorf("message not found or not processing: %s", messageID)
	}

	topic.mutex.Lock()
	defer topic.mutex.Unlock()

	// 从处理中移除
	delete(topic.processing, messageID)
	msg.MarkCompleted()

	// 记录处理时间
	if msg.ProcessedAt != nil {
		processTime := time.Since(*msg.ProcessedAt)
		q.recordProcessTime(processTime)
	}

	// 更新统计
	q.updateStats(func(stats *QueueStats) {
		stats.ProcessingMessages--
		stats.CompletedMessages++
	})

	return nil
}

// Nack 拒绝消息
func (q *MemoryQueue) Nack(ctx context.Context, messageID string, requeue bool) error {
	topic, msg := q.findProcessingMessage(messageID)
	if topic == nil || msg == nil {
		return fmt.Errorf("message not found or not processing: %s", messageID)
	}

	topic.mutex.Lock()
	defer topic.mutex.Unlock()

	// 从处理中移除
	delete(topic.processing, messageID)

	if requeue && msg.ShouldRetry() {
		// 重新入队
		msg.MarkRetrying()
		q.insertByPriority(topic, msg)

		// 通知等待的消费者
		select {
		case topic.notifyChan <- struct{}{}:
		default:
		}

		// 更新统计
		q.updateStats(func(stats *QueueStats) {
			stats.ProcessingMessages--
			stats.PendingMessages++
		})
	} else {
		// 移到死信队列
		msg.MarkDeadLetter()
		topic.deadLetter = append(topic.deadLetter, msg)

		// 更新统计
		q.updateStats(func(stats *QueueStats) {
			stats.ProcessingMessages--
			stats.FailedMessages++
			stats.DeadLetterMessages++
		})
	}

	return nil
}

// EnqueueBatch 批量入队
func (q *MemoryQueue) EnqueueBatch(ctx context.Context, messages []*Message) error {
	for _, msg := range messages {
		if err := q.Enqueue(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// DequeueBatch 批量出队
func (q *MemoryQueue) DequeueBatch(ctx context.Context, topicName string, count int) ([]*Message, error) {
	messages := make([]*Message, 0, count)
	for i := 0; i < count; i++ {
		msg, err := q.Dequeue(ctx, topicName)
		if err != nil {
			return messages, err
		}
		if msg == nil {
			break
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// EnqueueDelayed 延迟入队
func (q *MemoryQueue) EnqueueDelayed(ctx context.Context, msg *Message, delay time.Duration) error {
	scheduledAt := time.Now().Add(delay)
	msg.ScheduledAt = &scheduledAt
	return q.Enqueue(ctx, msg)
}

// EnqueueScheduled 定时入队
func (q *MemoryQueue) EnqueueScheduled(ctx context.Context, msg *Message, scheduledAt time.Time) error {
	msg.ScheduledAt = &scheduledAt
	return q.Enqueue(ctx, msg)
}

// EnqueueWithPriority 带优先级入队
func (q *MemoryQueue) EnqueueWithPriority(ctx context.Context, msg *Message, priority Priority) error {
	msg.Priority = priority
	return q.Enqueue(ctx, msg)
}

// Purge 清空主题
func (q *MemoryQueue) Purge(ctx context.Context, topicName string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	topic, exists := q.topics[topicName]
	if !exists {
		return fmt.Errorf("topic not found: %s", topicName)
	}

	topic.mutex.Lock()
	defer topic.mutex.Unlock()

	// 清空所有消息
	pendingCount := int64(len(topic.messages))
	processingCount := int64(len(topic.processing))
	deadLetterCount := int64(len(topic.deadLetter))

	topic.messages = make([]*Message, 0)
	topic.processing = make(map[string]*Message)
	topic.deadLetter = make([]*Message, 0)

	// 更新统计
	q.updateStats(func(stats *QueueStats) {
		stats.PendingMessages -= pendingCount
		stats.ProcessingMessages -= processingCount
		stats.DeadLetterMessages -= deadLetterCount
	})

	return nil
}

// Delete 删除主题
func (q *MemoryQueue) Delete(ctx context.Context, topicName string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	topic, exists := q.topics[topicName]
	if !exists {
		return fmt.Errorf("topic not found: %s", topicName)
	}

	// 先清空主题
	q.Purge(ctx, topicName)

	// 关闭通知通道
	close(topic.notifyChan)

	// 删除主题
	delete(q.topics, topicName)

	return nil
}

// Exists 检查主题是否存在
func (q *MemoryQueue) Exists(ctx context.Context, topicName string) (bool, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	_, exists := q.topics[topicName]
	return exists, nil
}

// Size 获取主题大小
func (q *MemoryQueue) Size(ctx context.Context, topicName string) (int64, error) {
	topic := q.getTopic(topicName)
	if topic == nil {
		return 0, fmt.Errorf("topic not found: %s", topicName)
	}

	topic.mutex.RLock()
	defer topic.mutex.RUnlock()

	return int64(len(topic.messages)), nil
}

// GetMessage 获取消息
func (q *MemoryQueue) GetMessage(ctx context.Context, messageID string) (*Message, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	for _, topic := range q.topics {
		topic.mutex.RLock()
		// 在待处理消息中查找
		for _, msg := range topic.messages {
			if msg.ID == messageID {
				topic.mutex.RUnlock()
				return msg, nil
			}
		}
		// 在处理中消息中查找
		if msg, exists := topic.processing[messageID]; exists {
			topic.mutex.RUnlock()
			return msg, nil
		}
		// 在死信队列中查找
		for _, msg := range topic.deadLetter {
			if msg.ID == messageID {
				topic.mutex.RUnlock()
				return msg, nil
			}
		}
		topic.mutex.RUnlock()
	}

	return nil, fmt.Errorf("message not found: %s", messageID)
}

// UpdateMessage 更新消息
func (q *MemoryQueue) UpdateMessage(ctx context.Context, msg *Message) error {
	// 内存队列中消息更新比较复杂，这里简化实现
	return fmt.Errorf("update message not supported in memory queue")
}

// DeleteMessage 删除消息
func (q *MemoryQueue) DeleteMessage(ctx context.Context, messageID string) error {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	for _, topic := range q.topics {
		topic.mutex.Lock()
		// 在待处理消息中查找并删除
		for i, msg := range topic.messages {
			if msg.ID == messageID {
				topic.messages = append(topic.messages[:i], topic.messages[i+1:]...)
				topic.mutex.Unlock()
				q.updateStats(func(stats *QueueStats) {
					stats.PendingMessages--
				})
				return nil
			}
		}
		// 在处理中消息中查找并删除
		if _, exists := topic.processing[messageID]; exists {
			delete(topic.processing, messageID)
			topic.mutex.Unlock()
			q.updateStats(func(stats *QueueStats) {
				stats.ProcessingMessages--
			})
			return nil
		}
		// 在死信队列中查找并删除
		for i, msg := range topic.deadLetter {
			if msg.ID == messageID {
				topic.deadLetter = append(topic.deadLetter[:i], topic.deadLetter[i+1:]...)
				topic.mutex.Unlock()
				q.updateStats(func(stats *QueueStats) {
					stats.DeadLetterMessages--
				})
				return nil
			}
		}
		topic.mutex.Unlock()
	}

	return fmt.Errorf("message not found: %s", messageID)
}

// GetStats 获取主题统计
func (q *MemoryQueue) GetStats(ctx context.Context, topicName string) (*QueueStats, error) {
	topic := q.getTopic(topicName)
	if topic == nil {
		return nil, fmt.Errorf("topic not found: %s", topicName)
	}

	topic.mutex.RLock()
	defer topic.mutex.RUnlock()

	stats := &QueueStats{
		PendingMessages:    int64(len(topic.messages)),
		ProcessingMessages: int64(len(topic.processing)),
		DeadLetterMessages: int64(len(topic.deadLetter)),
	}

	// 计算最老和最新消息时间
	for _, msg := range topic.messages {
		if stats.OldestMessage == nil || msg.CreatedAt.Before(*stats.OldestMessage) {
			stats.OldestMessage = &msg.CreatedAt
		}
		if stats.NewestMessage == nil || msg.CreatedAt.After(*stats.NewestMessage) {
			stats.NewestMessage = &msg.CreatedAt
		}
	}

	return stats, nil
}

// GetGlobalStats 获取全局统计
func (q *MemoryQueue) GetGlobalStats(ctx context.Context) (*QueueStats, error) {
	q.statsMutex.RLock()
	defer q.statsMutex.RUnlock()

	// 计算平均处理时间
	if len(q.processTimes) > 0 {
		var total time.Duration
		for _, duration := range q.processTimes {
			total += duration
		}
		q.stats.AverageProcessTime = float64(total.Nanoseconds()) / float64(len(q.processTimes)) / 1e6
	}

	// 计算错误率
	if q.stats.TotalMessages > 0 {
		q.stats.ErrorRate = float64(q.stats.FailedMessages) / float64(q.stats.TotalMessages)
	}

	// 深拷贝统计信息
	stats := *q.stats
	return &stats, nil
}

// 辅助方法

// getOrCreateTopic 获取或创建主题
func (q *MemoryQueue) getOrCreateTopic(topicName string) *topicQueue {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	topic, exists := q.topics[topicName]
	if !exists {
		topic = &topicQueue{
			messages:   make([]*Message, 0),
			processing: make(map[string]*Message),
			deadLetter: make([]*Message, 0),
			lastAccess: time.Now(),
			notifyChan: make(chan struct{}, 1),
		}
		q.topics[topicName] = topic
	}

	return topic
}

// getTopic 获取主题
func (q *MemoryQueue) getTopic(topicName string) *topicQueue {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	return q.topics[topicName]
}

// insertByPriority 按优先级插入消息
func (q *MemoryQueue) insertByPriority(topic *topicQueue, msg *Message) {
	// 找到插入位置
	insertIndex := len(topic.messages)
	for i, existingMsg := range topic.messages {
		if msg.Priority > existingMsg.Priority {
			insertIndex = i
			break
		}
	}

	// 插入消息
	if insertIndex == len(topic.messages) {
		topic.messages = append(topic.messages, msg)
	} else {
		topic.messages = append(topic.messages[:insertIndex+1], topic.messages[insertIndex:]...)
		topic.messages[insertIndex] = msg
	}
}

// findProcessingMessage 查找正在处理的消息
func (q *MemoryQueue) findProcessingMessage(messageID string) (*topicQueue, *Message) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	for _, topic := range q.topics {
		topic.mutex.RLock()
		if msg, exists := topic.processing[messageID]; exists {
			topic.mutex.RUnlock()
			return topic, msg
		}
		topic.mutex.RUnlock()
	}

	return nil, nil
}

// getTotalSize 获取总大小
func (q *MemoryQueue) getTotalSize() int64 {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var total int64
	for _, topic := range q.topics {
		topic.mutex.RLock()
		total += int64(len(topic.messages) + len(topic.processing) + len(topic.deadLetter))
		topic.mutex.RUnlock()
	}

	return total
}

// updateStats 更新统计信息
func (q *MemoryQueue) updateStats(updater func(*QueueStats)) {
	q.statsMutex.Lock()
	defer q.statsMutex.Unlock()

	updater(q.stats)
}

// recordProcessTime 记录处理时间
func (q *MemoryQueue) recordProcessTime(duration time.Duration) {
	q.statsMutex.Lock()
	defer q.statsMutex.Unlock()

	q.processTimes = append(q.processTimes, duration)
	if len(q.processTimes) > 1000 {
		q.processTimes = q.processTimes[1:]
	}
}

// cleanup 清理过期消息和空闲主题
func (q *MemoryQueue) cleanup(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.cleanupTicker.C:
			q.performCleanup()
		}
	}
}

// performCleanup 执行清理
func (q *MemoryQueue) performCleanup() {
	q.mutex.RLock()
	topicsToClean := make([]string, 0)
	for topicName, topic := range q.topics {
		// 检查主题是否长时间未访问
		if time.Since(topic.lastAccess) > time.Hour {
			topicsToClean = append(topicsToClean, topicName)
		}
	}
	q.mutex.RUnlock()

	// 清理空闲主题
	for _, topicName := range topicsToClean {
		topic := q.getTopic(topicName)
		if topic != nil {
			topic.mutex.RLock()
			isEmpty := len(topic.messages) == 0 && len(topic.processing) == 0
			topic.mutex.RUnlock()

			if isEmpty {
				q.Delete(context.Background(), topicName)
			}
		}
	}

	// 清理过期的处理中消息
	q.mutex.RLock()
	for _, topic := range q.topics {
		topic.mutex.Lock()
		for messageID, msg := range topic.processing {
			if msg.ProcessedAt != nil && time.Since(*msg.ProcessedAt) > q.config.VisibilityTimeout {
				// 消息处理超时，重新入队或移到死信队列
				delete(topic.processing, messageID)
				if msg.ShouldRetry() {
					msg.MarkRetrying()
					q.insertByPriority(topic, msg)
				} else {
					msg.MarkDeadLetter()
					topic.deadLetter = append(topic.deadLetter, msg)
				}
			}
		}
		topic.mutex.Unlock()
	}
	q.mutex.RUnlock()
}