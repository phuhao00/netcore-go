// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 日志指标统计
type Metrics struct {
	mu sync.RWMutex
	
	// 基础统计
	TotalLogs    int64 // 总日志数
	DroppedLogs  int64 // 丢弃的日志数
	ErrorLogs    int64 // 错误日志数
	WriteErrors  int64 // 写入错误数
	FormatErrors int64 // 格式化错误数
	
	// 性能统计
	TotalWriteTime  int64 // 总写入时间(纳秒)
	TotalFormatTime int64 // 总格式化时间(纳秒)
	MaxWriteTime    int64 // 最大写入时间(纳秒)
	MaxFormatTime   int64 // 最大格式化时间(纳秒)
	
	// 级别统计
	LevelCounts map[Level]int64
	
	// 时间统计
	StartTime time.Time
	LastReset time.Time
	
	// 缓冲区统计
	BufferSize     int64 // 缓冲区大小
	BufferUsage    int64 // 缓冲区使用量
	MaxBufferUsage int64 // 最大缓冲区使用量
	
	// 采样统计
	SampledLogs int64   // 采样日志数
	SkippedLogs int64   // 跳过日志数
	SampleRate  float64 // 采样率
}

// NewMetrics 创建新的指标统计
func NewMetrics() *Metrics {
	return &Metrics{
		LevelCounts: make(map[Level]int64),
		StartTime:   time.Now(),
		LastReset:   time.Now(),
		SampleRate:  1.0,
	}
}

// IncrementTotal 增加总日志计数
func (m *Metrics) IncrementTotal() {
	atomic.AddInt64(&m.TotalLogs, 1)
}

// IncrementDropped 增加丢弃日志计数
func (m *Metrics) IncrementDropped() {
	atomic.AddInt64(&m.DroppedLogs, 1)
}

// IncrementError 增加错误日志计数
func (m *Metrics) IncrementError() {
	atomic.AddInt64(&m.ErrorLogs, 1)
}

// IncrementWriteError 增加写入错误计数
func (m *Metrics) IncrementWriteError() {
	atomic.AddInt64(&m.WriteErrors, 1)
}

// IncrementFormatError 增加格式化错误计数
func (m *Metrics) IncrementFormatError() {
	atomic.AddInt64(&m.FormatErrors, 1)
}

// IncrementLevel 增加特定级别的日志计数
func (m *Metrics) IncrementLevel(level Level) {
	m.mu.Lock()
	m.LevelCounts[level]++
	m.mu.Unlock()
}

// AddWriteTime 添加写入时间统计
func (m *Metrics) AddWriteTime(duration time.Duration) {
	ns := duration.Nanoseconds()
	atomic.AddInt64(&m.TotalWriteTime, ns)
	
	// 更新最大写入时间
	for {
		current := atomic.LoadInt64(&m.MaxWriteTime)
		if ns <= current || atomic.CompareAndSwapInt64(&m.MaxWriteTime, current, ns) {
			break
		}
	}
}

// AddFormatTime 添加格式化时间统计
func (m *Metrics) AddFormatTime(duration time.Duration) {
	ns := duration.Nanoseconds()
	atomic.AddInt64(&m.TotalFormatTime, ns)
	
	// 更新最大格式化时间
	for {
		current := atomic.LoadInt64(&m.MaxFormatTime)
		if ns <= current || atomic.CompareAndSwapInt64(&m.MaxFormatTime, current, ns) {
			break
		}
	}
}

// UpdateBufferUsage 更新缓冲区使用量统计
func (m *Metrics) UpdateBufferUsage(usage int64) {
	atomic.StoreInt64(&m.BufferUsage, usage)
	
	// 更新最大缓冲区使用量
	for {
		current := atomic.LoadInt64(&m.MaxBufferUsage)
		if usage <= current || atomic.CompareAndSwapInt64(&m.MaxBufferUsage, current, usage) {
			break
		}
	}
}

// IncrementSampled 增加采样日志计数
func (m *Metrics) IncrementSampled() {
	atomic.AddInt64(&m.SampledLogs, 1)
}

// IncrementSkipped 增加跳过日志计数
func (m *Metrics) IncrementSkipped() {
	atomic.AddInt64(&m.SkippedLogs, 1)
}

// SetSampleRate 设置采样率
func (m *Metrics) SetSampleRate(rate float64) {
	m.mu.Lock()
	m.SampleRate = rate
	m.mu.Unlock()
}

// GetSnapshot 获取指标快照
func (m *Metrics) GetSnapshot() *MetricsSnapshot {
	m.mu.RLock()
	levelCounts := make(map[Level]int64)
	for k, v := range m.LevelCounts {
		levelCounts[k] = v
	}
	sampleRate := m.SampleRate
	startTime := m.StartTime
	lastReset := m.LastReset
	m.mu.RUnlock()
	
	return &MetricsSnapshot{
		TotalLogs:        atomic.LoadInt64(&m.TotalLogs),
		DroppedLogs:      atomic.LoadInt64(&m.DroppedLogs),
		ErrorLogs:        atomic.LoadInt64(&m.ErrorLogs),
		WriteErrors:      atomic.LoadInt64(&m.WriteErrors),
		FormatErrors:     atomic.LoadInt64(&m.FormatErrors),
		TotalWriteTime:   atomic.LoadInt64(&m.TotalWriteTime),
		TotalFormatTime:  atomic.LoadInt64(&m.TotalFormatTime),
		MaxWriteTime:     atomic.LoadInt64(&m.MaxWriteTime),
		MaxFormatTime:    atomic.LoadInt64(&m.MaxFormatTime),
		LevelCounts:      levelCounts,
		StartTime:        startTime,
		LastReset:        lastReset,
		BufferSize:       atomic.LoadInt64(&m.BufferSize),
		BufferUsage:      atomic.LoadInt64(&m.BufferUsage),
		MaxBufferUsage:   atomic.LoadInt64(&m.MaxBufferUsage),
		SampledLogs:      atomic.LoadInt64(&m.SampledLogs),
		SkippedLogs:      atomic.LoadInt64(&m.SkippedLogs),
		SampleRate:       sampleRate,
	}
}

// Reset 重置所有指标统计
func (m *Metrics) Reset() {
	atomic.StoreInt64(&m.TotalLogs, 0)
	atomic.StoreInt64(&m.DroppedLogs, 0)
	atomic.StoreInt64(&m.ErrorLogs, 0)
	atomic.StoreInt64(&m.WriteErrors, 0)
	atomic.StoreInt64(&m.FormatErrors, 0)
	atomic.StoreInt64(&m.TotalWriteTime, 0)
	atomic.StoreInt64(&m.TotalFormatTime, 0)
	atomic.StoreInt64(&m.MaxWriteTime, 0)
	atomic.StoreInt64(&m.MaxFormatTime, 0)
	atomic.StoreInt64(&m.BufferUsage, 0)
	atomic.StoreInt64(&m.MaxBufferUsage, 0)
	atomic.StoreInt64(&m.SampledLogs, 0)
	atomic.StoreInt64(&m.SkippedLogs, 0)
	
	m.mu.Lock()
	for k := range m.LevelCounts {
		m.LevelCounts[k] = 0
	}
	m.LastReset = time.Now()
	m.mu.Unlock()
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	TotalLogs        int64             `json:"total_logs"`
	DroppedLogs      int64             `json:"dropped_logs"`
	ErrorLogs        int64             `json:"error_logs"`
	WriteErrors      int64             `json:"write_errors"`
	FormatErrors     int64             `json:"format_errors"`
	TotalWriteTime   int64             `json:"total_write_time_ns"`
	TotalFormatTime  int64             `json:"total_format_time_ns"`
	MaxWriteTime     int64             `json:"max_write_time_ns"`
	MaxFormatTime    int64             `json:"max_format_time_ns"`
	LevelCounts      map[Level]int64   `json:"level_counts"`
	StartTime        time.Time         `json:"start_time"`
	LastReset        time.Time         `json:"last_reset"`
	BufferSize       int64             `json:"buffer_size"`
	BufferUsage      int64             `json:"buffer_usage"`
	MaxBufferUsage   int64             `json:"max_buffer_usage"`
	SampledLogs      int64             `json:"sampled_logs"`
	SkippedLogs      int64             `json:"skipped_logs"`
	SampleRate       float64           `json:"sample_rate"`
}

// GetAverageWriteTime 获取平均写入时间(纳秒)
func (s *MetricsSnapshot) GetAverageWriteTime() float64 {
	if s.TotalLogs == 0 {
		return 0
	}
	return float64(s.TotalWriteTime) / float64(s.TotalLogs)
}

// GetAverageFormatTime 获取平均格式化时间(纳秒)
func (s *MetricsSnapshot) GetAverageFormatTime() float64 {
	if s.TotalLogs == 0 {
		return 0
	}
	return float64(s.TotalFormatTime) / float64(s.TotalLogs)
}

// GetLogsPerSecond 获取每秒日志数
func (s *MetricsSnapshot) GetLogsPerSecond() float64 {
	duration := time.Since(s.StartTime).Seconds()
	if duration == 0 {
		return 0
	}
	return float64(s.TotalLogs) / duration
}

// GetErrorRate 获取错误率
func (s *MetricsSnapshot) GetErrorRate() float64 {
	if s.TotalLogs == 0 {
		return 0
	}
	return float64(s.ErrorLogs) / float64(s.TotalLogs)
}

// GetDropRate 获取丢弃率
func (s *MetricsSnapshot) GetDropRate() float64 {
	total := s.TotalLogs + s.DroppedLogs
	if total == 0 {
		return 0
	}
	return float64(s.DroppedLogs) / float64(total)
}

// GetBufferUtilization 获取缓冲区利用率
func (s *MetricsSnapshot) GetBufferUtilization() float64 {
	if s.BufferSize == 0 {
		return 0
	}
	return float64(s.BufferUsage) / float64(s.BufferSize)
}

// GetSamplingEfficiency 获取采样效率
func (s *MetricsSnapshot) GetSamplingEfficiency() float64 {
	total := s.SampledLogs + s.SkippedLogs
	if total == 0 {
		return 0
	}
	return float64(s.SampledLogs) / float64(total)
}