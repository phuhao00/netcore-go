// Package longpoll 长轮询实现
// Author: NetCore-Go Team
// Created: 2024

package longpoll

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Event 事件结构
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Data      interface{}            `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewEvent 创建新事件
func NewEvent(eventType string, data interface{}) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().Unix(),
		Metadata:  make(map[string]interface{}),
	}
}

// SetMetadata 设置元数据
func (e *Event) SetMetadata(key string, value interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
}

// GetMetadata 获取元数据
func (e *Event) GetMetadata(key string) (interface{}, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	value, exists := e.Metadata[key]
	return value, exists
}

// ToJSON 转换为JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// Subscription 订阅信息
type Subscription struct {
	ID          string
	ClientID    string
	Topics      []string
	Filters     map[string]interface{}
	Channel     chan *Event
	LastEventID string
	CreatedAt   time.Time
	LastActive  time.Time
	Timeout     time.Duration
	Context     context.Context
	Cancel      context.CancelFunc
}

// NewSubscription 创建新订阅
func NewSubscription(clientID string, topics []string, timeout time.Duration) *Subscription {
	ctx, cancel := context.WithCancel(context.Background())
	return &Subscription{
		ID:         generateSubscriptionID(),
		ClientID:   clientID,
		Topics:     topics,
		Filters:    make(map[string]interface{}),
		Channel:    make(chan *Event, 100), // 缓冲100个事件
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Timeout:    timeout,
		Context:    ctx,
		Cancel:     cancel,
	}
}

// AddFilter 添加过滤器
func (s *Subscription) AddFilter(key string, value interface{}) {
	s.Filters[key] = value
}

// RemoveFilter 移除过滤器
func (s *Subscription) RemoveFilter(key string) {
	delete(s.Filters, key)
}

// MatchesEvent 检查事件是否匹配订阅
func (s *Subscription) MatchesEvent(event *Event) bool {
	// 检查主题
	topicMatched := false
	for _, topic := range s.Topics {
		if topic == "*" || topic == event.Type {
			topicMatched = true
			break
		}
	}
	if !topicMatched {
		return false
	}

	// 检查过滤器
	for key, expectedValue := range s.Filters {
		if actualValue, exists := event.GetMetadata(key); !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// Close 关闭订阅
func (s *Subscription) Close() {
	s.Cancel()
	close(s.Channel)
}

// IsExpired 检查是否过期
func (s *Subscription) IsExpired() bool {
	return time.Since(s.LastActive) > s.Timeout
}

// UpdateLastActive 更新最后活跃时间
func (s *Subscription) UpdateLastActive() {
	s.LastActive = time.Now()
}

// LongPollManager 长轮询管理器
type LongPollManager struct {
	subscriptions map[string]*Subscription
	clientSubs    map[string][]*Subscription
	topicSubs     map[string][]*Subscription
	mutex         sync.RWMutex
	eventHistory  []*Event
	maxHistory    int
	cleanupTicker *time.Ticker
	stats         *LongPollStats
}

// LongPollStats 长轮询统计
type LongPollStats struct {
	TotalSubscriptions   int64 `json:"total_subscriptions"`
	ActiveSubscriptions  int64 `json:"active_subscriptions"`
	TotalEvents          int64 `json:"total_events"`
	EventsDelivered      int64 `json:"events_delivered"`
	ExpiredSubscriptions int64 `json:"expired_subscriptions"`
	AverageWaitTime      float64 `json:"average_wait_time_ms"`
	TopicCount           int   `json:"topic_count"`
	ClientCount          int   `json:"client_count"`
}

// NewLongPollManager 创建长轮询管理器
func NewLongPollManager(maxHistory int) *LongPollManager {
	manager := &LongPollManager{
		subscriptions: make(map[string]*Subscription),
		clientSubs:    make(map[string][]*Subscription),
		topicSubs:     make(map[string][]*Subscription),
		eventHistory:  make([]*Event, 0, maxHistory),
		maxHistory:    maxHistory,
		cleanupTicker: time.NewTicker(30 * time.Second),
		stats:         &LongPollStats{},
	}

	// 启动清理协程
	go manager.cleanup()

	return manager
}

// Subscribe 创建订阅
func (m *LongPollManager) Subscribe(clientID string, topics []string, timeout time.Duration) *Subscription {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	sub := NewSubscription(clientID, topics, timeout)
	m.subscriptions[sub.ID] = sub

	// 添加到客户端订阅映射
	m.clientSubs[clientID] = append(m.clientSubs[clientID], sub)

	// 添加到主题订阅映射
	for _, topic := range topics {
		m.topicSubs[topic] = append(m.topicSubs[topic], sub)
	}

	m.stats.TotalSubscriptions++
	m.stats.ActiveSubscriptions++

	return sub
}

// Unsubscribe 取消订阅
func (m *LongPollManager) Unsubscribe(subscriptionID string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	sub, exists := m.subscriptions[subscriptionID]
	if !exists {
		return false
	}

	// 从订阅映射中删除
	delete(m.subscriptions, subscriptionID)

	// 从客户端订阅映射中删除
	clientSubs := m.clientSubs[sub.ClientID]
	for i, s := range clientSubs {
		if s.ID == subscriptionID {
			m.clientSubs[sub.ClientID] = append(clientSubs[:i], clientSubs[i+1:]...)
			break
		}
	}

	// 从主题订阅映射中删除
	for _, topic := range sub.Topics {
		topicSubs := m.topicSubs[topic]
		for i, s := range topicSubs {
			if s.ID == subscriptionID {
				m.topicSubs[topic] = append(topicSubs[:i], topicSubs[i+1:]...)
				break
			}
		}
	}

	// 关闭订阅
	sub.Close()
	m.stats.ActiveSubscriptions--

	return true
}

// UnsubscribeClient 取消客户端的所有订阅
func (m *LongPollManager) UnsubscribeClient(clientID string) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	clientSubs, exists := m.clientSubs[clientID]
	if !exists {
		return 0
	}

	count := 0
	for _, sub := range clientSubs {
		// 从订阅映射中删除
		delete(m.subscriptions, sub.ID)

		// 从主题订阅映射中删除
		for _, topic := range sub.Topics {
			topicSubs := m.topicSubs[topic]
			for i, s := range topicSubs {
				if s.ID == sub.ID {
					m.topicSubs[topic] = append(topicSubs[:i], topicSubs[i+1:]...)
					break
				}
			}
		}

		// 关闭订阅
		sub.Close()
		count++
	}

	// 删除客户端订阅映射
	delete(m.clientSubs, clientID)
	m.stats.ActiveSubscriptions -= int64(count)

	return count
}

// Publish 发布事件
func (m *LongPollManager) Publish(event *Event) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 添加到历史记录
	m.addToHistory(event)
	m.stats.TotalEvents++

	deliveredCount := 0

	// 发送给匹配的订阅
	for _, sub := range m.subscriptions {
		if sub.MatchesEvent(event) {
			select {
			case sub.Channel <- event:
				sub.UpdateLastActive()
				deliveredCount++
			case <-sub.Context.Done():
				// 订阅已关闭，跳过
			default:
				// 通道已满，跳过
			}
		}
	}

	m.stats.EventsDelivered += int64(deliveredCount)
	return deliveredCount
}

// PublishToTopic 发布事件到指定主题
func (m *LongPollManager) PublishToTopic(topic string, data interface{}) int {
	event := NewEvent(topic, data)
	return m.Publish(event)
}

// PublishToClient 发布事件到指定客户端
func (m *LongPollManager) PublishToClient(clientID string, event *Event) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	clientSubs, exists := m.clientSubs[clientID]
	if !exists {
		return 0
	}

	deliveredCount := 0
	for _, sub := range clientSubs {
		if sub.MatchesEvent(event) {
			select {
			case sub.Channel <- event:
				sub.UpdateLastActive()
				deliveredCount++
			case <-sub.Context.Done():
				// 订阅已关闭，跳过
			default:
				// 通道已满，跳过
			}
		}
	}

	m.stats.EventsDelivered += int64(deliveredCount)
	return deliveredCount
}

// GetEvents 获取事件（长轮询）
func (m *LongPollManager) GetEvents(subscriptionID string, lastEventID string, timeout time.Duration) ([]*Event, error) {
	m.mutex.RLock()
	sub, exists := m.subscriptions[subscriptionID]
	m.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("subscription not found")
	}

	// 检查历史事件
	if lastEventID != "" {
		historyEvents := m.getHistoryAfter(lastEventID)
		if len(historyEvents) > 0 {
			return historyEvents, nil
		}
	}

	// 等待新事件
	ctx, cancel := context.WithTimeout(sub.Context, timeout)
	defer cancel()

	events := make([]*Event, 0)
	start := time.Now()

	for {
		select {
		case event := <-sub.Channel:
			events = append(events, event)
			sub.LastEventID = event.ID
			// 可以选择立即返回第一个事件，或者等待更多事件
			return events, nil
		case <-ctx.Done():
			// 超时或取消
			waitTime := float64(time.Since(start).Nanoseconds()) / 1e6
			m.updateAverageWaitTime(waitTime)
			return events, nil
		}
	}
}

// GetSubscription 获取订阅信息
func (m *LongPollManager) GetSubscription(subscriptionID string) (*Subscription, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	sub, exists := m.subscriptions[subscriptionID]
	return sub, exists
}

// GetClientSubscriptions 获取客户端的所有订阅
func (m *LongPollManager) GetClientSubscriptions(clientID string) []*Subscription {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.clientSubs[clientID]
}

// GetStats 获取统计信息
func (m *LongPollManager) GetStats() *LongPollStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := *m.stats
	stats.TopicCount = len(m.topicSubs)
	stats.ClientCount = len(m.clientSubs)

	return &stats
}

// addToHistory 添加到历史记录
func (m *LongPollManager) addToHistory(event *Event) {
	m.eventHistory = append(m.eventHistory, event)
	if len(m.eventHistory) > m.maxHistory {
		m.eventHistory = m.eventHistory[1:]
	}
}

// getHistoryAfter 获取指定事件ID之后的历史事件
func (m *LongPollManager) getHistoryAfter(lastEventID string) []*Event {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]*Event, 0)
	found := false

	for _, event := range m.eventHistory {
		if found {
			result = append(result, event)
		} else if event.ID == lastEventID {
			found = true
		}
	}

	return result
}

// updateAverageWaitTime 更新平均等待时间
func (m *LongPollManager) updateAverageWaitTime(waitTime float64) {
	// 简单的移动平均
	if m.stats.AverageWaitTime == 0 {
		m.stats.AverageWaitTime = waitTime
	} else {
		m.stats.AverageWaitTime = (m.stats.AverageWaitTime*0.9 + waitTime*0.1)
	}
}

// cleanup 清理过期订阅
func (m *LongPollManager) cleanup() {
	for range m.cleanupTicker.C {
		m.mutex.Lock()
		expiredSubs := make([]*Subscription, 0)

		for _, sub := range m.subscriptions {
			if sub.IsExpired() {
				expiredSubs = append(expiredSubs, sub)
			}
		}

		for _, sub := range expiredSubs {
			m.unsubscribeInternal(sub.ID)
			m.stats.ExpiredSubscriptions++
		}

		m.mutex.Unlock()
	}
}

// unsubscribeInternal 内部取消订阅（不加锁）
func (m *LongPollManager) unsubscribeInternal(subscriptionID string) {
	sub, exists := m.subscriptions[subscriptionID]
	if !exists {
		return
	}

	// 从订阅映射中删除
	delete(m.subscriptions, subscriptionID)

	// 从客户端订阅映射中删除
	clientSubs := m.clientSubs[sub.ClientID]
	for i, s := range clientSubs {
		if s.ID == subscriptionID {
			m.clientSubs[sub.ClientID] = append(clientSubs[:i], clientSubs[i+1:]...)
			break
		}
	}

	// 从主题订阅映射中删除
	for _, topic := range sub.Topics {
		topicSubs := m.topicSubs[topic]
		for i, s := range topicSubs {
			if s.ID == subscriptionID {
				m.topicSubs[topic] = append(topicSubs[:i], topicSubs[i+1:]...)
				break
			}
		}
	}

	// 关闭订阅
	sub.Close()
	m.stats.ActiveSubscriptions--
}

// Close 关闭长轮询管理器
func (m *LongPollManager) Close() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 关闭清理定时器
	m.cleanupTicker.Stop()

	// 关闭所有订阅
	for _, sub := range m.subscriptions {
		sub.Close()
	}

	// 清空映射
	m.subscriptions = make(map[string]*Subscription)
	m.clientSubs = make(map[string][]*Subscription)
	m.topicSubs = make(map[string][]*Subscription)
}

// 辅助函数

var (
	eventIDCounter        int64
	subscriptionIDCounter int64
)

func generateEventID() string {
	eventIDCounter++
	return fmt.Sprintf("event_%d_%d", time.Now().Unix(), eventIDCounter)
}

func generateSubscriptionID() string {
	subscriptionIDCounter++
	return fmt.Sprintf("sub_%d_%d", time.Now().Unix(), subscriptionIDCounter)
}