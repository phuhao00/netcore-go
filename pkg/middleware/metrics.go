// Package middleware 监控中间件实现
// Author: NetCore-Go Team
// Created: 2024

package middleware

import (
	"sync"
	"time"

	"github.com/phuhao00/netcore-go/pkg/core"
)

// MetricsData 监控数据
type MetricsData struct {
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	TotalLatency    time.Duration `json:"total_latency"`
	AverageLatency  time.Duration `json:"average_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	MinLatency      time.Duration `json:"min_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
}

// MetricsMiddleware 监控中间件
type MetricsMiddleware struct {
	data  *MetricsData
	mutex sync.RWMutex
}

// NewMetricsMiddleware 创建监控中间件
func NewMetricsMiddleware() *MetricsMiddleware {
	return &MetricsMiddleware{
		data: &MetricsData{
			MinLatency: time.Duration(^uint64(0) >> 1), // 设置为最大值
		},
	}
}

// Name 获取中间件名称
func (m *MetricsMiddleware) Name() string {
	return "metrics"
}

// Priority 获取中间件优先级
func (m *MetricsMiddleware) Priority() int {
	return 100 // 最高优先级，确保能监控到所有请求
}

// Process 处理请求
func (m *MetricsMiddleware) Process(ctx core.Context, next core.Handler) error {
	start := time.Now()
	
	// 执行下一个中间件或处理器
	err := next(ctx)
	
	// 计算延迟
	latency := time.Since(start)
	
	// 更新统计数据
	m.updateMetrics(latency, err != nil)
	
	return err
}

// updateMetrics 更新监控数据
func (m *MetricsMiddleware) updateMetrics(latency time.Duration, hasError bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.data.RequestCount++
	m.data.LastRequestTime = time.Now()
	
	if hasError {
		m.data.ErrorCount++
	}
	
	// 更新延迟统计
	m.data.TotalLatency += latency
	m.data.AverageLatency = time.Duration(int64(m.data.TotalLatency) / m.data.RequestCount)
	
	if latency > m.data.MaxLatency {
		m.data.MaxLatency = latency
	}
	
	if latency < m.data.MinLatency {
		m.data.MinLatency = latency
	}
}

// GetMetrics 获取监控数据
func (m *MetricsMiddleware) GetMetrics() *MetricsData {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// 返回数据副本
	return &MetricsData{
		RequestCount:    m.data.RequestCount,
		ErrorCount:      m.data.ErrorCount,
		TotalLatency:    m.data.TotalLatency,
		AverageLatency:  m.data.AverageLatency,
		MaxLatency:      m.data.MaxLatency,
		MinLatency:      m.data.MinLatency,
		LastRequestTime: m.data.LastRequestTime,
	}
}

// Reset 重置监控数据
func (m *MetricsMiddleware) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.data = &MetricsData{
		MinLatency: time.Duration(^uint64(0) >> 1),
	}
}

// GetErrorRate 获取错误率
func (m *MetricsMiddleware) GetErrorRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.data.RequestCount == 0 {
		return 0
	}
	
	return float64(m.data.ErrorCount) / float64(m.data.RequestCount)
}

// GetRequestsPerSecond 获取每秒请求数（基于最近的请求时间估算）
func (m *MetricsMiddleware) GetRequestsPerSecond() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.data.RequestCount == 0 || m.data.LastRequestTime.IsZero() {
		return 0
	}
	
	// 简单估算：假设请求均匀分布
	duration := time.Since(m.data.LastRequestTime)
	if duration.Seconds() == 0 {
		return 0
	}
	
	return float64(m.data.RequestCount) / duration.Seconds()
}