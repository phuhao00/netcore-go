// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics `n`ntype Metrics struct {
	mu sync.RWMutex
	
	// `n�?	TotalLogs     int64 `json:"total_logs"`     // `n
	DroppedLogs   int64 `json:"dropped_logs"`   // `n
	ErrorLogs     int64 `json:"error_logs"`     // `n�?	WriteErrors   int64 `json:"write_errors"`   // `n�?	FormatErrors  int64 `json:"format_errors"`  // `n
	
	// `n
	TotalWriteTime   int64 `json:"total_write_time_ns"`   // `n�?`n)
	TotalFormatTime  int64 `json:"total_format_time_ns"`  // `n(`n)
	MaxWriteTime     int64 `json:"max_write_time_ns"`     // `n�?`n)
	MaxFormatTime    int64 `json:"max_format_time_ns"`    // `n(`n)
	
	// `n
	LevelCounts map[Level]int64 `json:"level_counts"` // `n�?	
	// `n
	StartTime time.Time `json:"start_time"` // `n�?	LastReset time.Time `json:"last_reset"` // `n
	
	// `n�?	BufferSize     int64 `json:"buffer_size"`     // `n�?	BufferUsage    int64 `json:"buffer_usage"`    // `n
	MaxBufferUsage int64 `json:"max_buffer_usage"` // `n�?	
	// `n
	SampledLogs  int64 `json:"sampled_logs"`  // `n
	SkippedLogs  int64 `json:"skipped_logs"`  // `n
	SampleRate   float64 `json:"sample_rate"`   // `n�?}

// NewMetrics `n`nfunc NewMetrics() *Metrics {
	now := time.Now()
	return &Metrics{
		LevelCounts: make(map[Level]int64),
		StartTime:   now,
		LastReset:   now,
		SampleRate:  1.0, // `n100%`n
	}
}

// IncrementTotal `n�?func (m *Metrics) IncrementTotal() {
	atomic.AddInt64(&m.TotalLogs, 1)
}

// IncrementDropped `n`nfunc (m *Metrics) IncrementDropped() {
	atomic.AddInt64(&m.DroppedLogs, 1)
}

// IncrementError `n`nfunc (m *Metrics) IncrementError() {
	atomic.AddInt64(&m.ErrorLogs, 1)
}

// IncrementWriteError `n`nfunc (m *Metrics) IncrementWriteError() {
	atomic.AddInt64(&m.WriteErrors, 1)
}

// IncrementFormatError `n�?func (m *Metrics) IncrementFormatError() {
	atomic.AddInt64(&m.FormatErrors, 1)
}

// IncrementLevel `n�?func (m *Metrics) IncrementLevel(level Level) {
	m.mu.Lock()
	m.LevelCounts[level]++
	m.mu.Unlock()
}

// AddWriteTime `n`nfunc (m *Metrics) AddWriteTime(duration time.Duration) {
	ns := duration.Nanoseconds()
	atomic.AddInt64(&m.TotalWriteTime, ns)
	
	// `n�?	for {
		current := atomic.LoadInt64(&m.MaxWriteTime)
		if ns <= current || atomic.CompareAndSwapInt64(&m.MaxWriteTime, current, ns) {
			break
		}
	}
}

// AddFormatTime `n�?func (m *Metrics) AddFormatTime(duration time.Duration) {
	ns := duration.Nanoseconds()
	atomic.AddInt64(&m.TotalFormatTime, ns)
	
	// `n`nfor {
		current := atomic.LoadInt64(&m.MaxFormatTime)
		if ns <= current || atomic.CompareAndSwapInt64(&m.MaxFormatTime, current, ns) {
			break
		}
	}
}

// UpdateBufferUsage `n`nfunc (m *Metrics) UpdateBufferUsage(usage int64) {
	atomic.StoreInt64(&m.BufferUsage, usage)
	
	// `n�?	for {
		current := atomic.LoadInt64(&m.MaxBufferUsage)
		if usage <= current || atomic.CompareAndSwapInt64(&m.MaxBufferUsage, current, usage) {
			break
		}
	}
}

// IncrementSampled `n`nfunc (m *Metrics) IncrementSampled() {
	atomic.AddInt64(&m.SampledLogs, 1)
}

// IncrementSkipped `n`nfunc (m *Metrics) IncrementSkipped() {
	atomic.AddInt64(&m.SkippedLogs, 1)
}

// SetSampleRate `n�?func (m *Metrics) SetSampleRate(rate float64) {
	m.mu.Lock()
	m.SampleRate = rate
	m.mu.Unlock()
}

// GetSnapshot `n`nfunc (m *Metrics) GetSnapshot() *MetricsSnapshot {
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

// Reset `n`nfunc (m *Metrics) Reset() {
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

// MetricsSnapshot `n`ntype MetricsSnapshot struct {
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

// GetAverageWriteTime `n(`n)
func (s *MetricsSnapshot) GetAverageWriteTime() float64 {
	if s.TotalLogs == 0 {
		return 0
	}
	return float64(s.TotalWriteTime) / float64(s.TotalLogs)
}

// GetAverageFormatTime `n�?`n)
func (s *MetricsSnapshot) GetAverageFormatTime() float64 {
	if s.TotalLogs == 0 {
		return 0
	}
	return float64(s.TotalFormatTime) / float64(s.TotalLogs)
}

// GetLogsPerSecond `n�?func (s *MetricsSnapshot) GetLogsPerSecond() float64 {
	duration := time.Since(s.StartTime).Seconds()
	if duration == 0 {
		return 0
	}
	return float64(s.TotalLogs) / duration
}

// GetErrorRate `n�?func (s *MetricsSnapshot) GetErrorRate() float64 {
	if s.TotalLogs == 0 {
		return 0
	}
	return float64(s.ErrorLogs) / float64(s.TotalLogs)
}

// GetDropRate `n�?func (s *MetricsSnapshot) GetDropRate() float64 {
	total := s.TotalLogs + s.DroppedLogs
	if total == 0 {
		return 0
	}
	return float64(s.DroppedLogs) / float64(total)
}

// GetBufferUtilization `n`nfunc (s *MetricsSnapshot) GetBufferUtilization() float64 {
	if s.BufferSize == 0 {
		return 0
	}
	return float64(s.BufferUsage) / float64(s.BufferSize)
}

// GetSamplingEfficiency `n`nfunc (s *MetricsSnapshot) GetSamplingEfficiency() float64 {
	total := s.SampledLogs + s.SkippedLogs
	if total == 0 {
		return 0
	}
	return float64(s.SampledLogs) / float64(total)
}

// PerformanceMonitor `n�?type PerformanceMonitor struct {
	mu      sync.RWMutex
	metrics *Metrics
	enabled bool
	
	// `n�?	maxWriteTime   time.Duration // `n�?	maxFormatTime  time.Duration // `n�?	maxErrorRate   float64       // `n�?	maxDropRate    float64       // `n�?	
	// `n
	onSlowWrite   func(duration time.Duration)
	onSlowFormat  func(duration time.Duration)
	onHighError   func(rate float64)
	onHighDrop    func(rate float64)
}

// NewPerformanceMonitor `n�?func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		metrics:       NewMetrics(),
		enabled:       true,
		maxWriteTime:  100 * time.Millisecond,  // `n100ms`n�?		maxFormatTime: 10 * time.Millisecond,   // `n10ms`n�?		maxErrorRate:  0.01,                    // `n1%`n�?		maxDropRate:   0.05,                    // `n5%`n�?	}
}

// SetEnabled `n`nfunc (pm *PerformanceMonitor) SetEnabled(enabled bool) {
	pm.mu.Lock()
	pm.enabled = enabled
	pm.mu.Unlock()
}

// IsEnabled `n�?func (pm *PerformanceMonitor) IsEnabled() bool {
	pm.mu.RLock()
	enabled := pm.enabled
	pm.mu.RUnlock()
	return enabled
}

// GetMetrics `n`nfunc (pm *PerformanceMonitor) GetMetrics() *Metrics {
	return pm.metrics
}

// SetThresholds `n�?func (pm *PerformanceMonitor) SetThresholds(maxWriteTime, maxFormatTime time.Duration, maxErrorRate, maxDropRate float64) {
	pm.mu.Lock()
	pm.maxWriteTime = maxWriteTime
	pm.maxFormatTime = maxFormatTime
	pm.maxErrorRate = maxErrorRate
	pm.maxDropRate = maxDropRate
	pm.mu.Unlock()
}

// SetCallbacks `n`nfunc (pm *PerformanceMonitor) SetCallbacks(
	onSlowWrite func(duration time.Duration),
	onSlowFormat func(duration time.Duration),
	onHighError func(rate float64),
	onHighDrop func(rate float64),
) {
	pm.mu.Lock()
	pm.onSlowWrite = onSlowWrite
	pm.onSlowFormat = onSlowFormat
	pm.onHighError = onHighError
	pm.onHighDrop = onHighDrop
	pm.mu.Unlock()
}

// RecordWrite `n`nfunc (pm *PerformanceMonitor) RecordWrite(duration time.Duration, err error) {
	if !pm.IsEnabled() {
		return
	}
	
	pm.metrics.AddWriteTime(duration)
	
	if err != nil {
		pm.metrics.IncrementWriteError()
	}
	
	// `n
	pm.mu.RLock()
	maxWriteTime := pm.maxWriteTime
	onSlowWrite := pm.onSlowWrite
	pm.mu.RUnlock()
	
	if duration > maxWriteTime && onSlowWrite != nil {
		onSlowWrite(duration)
	}
}

// RecordFormat `n�?func (pm *PerformanceMonitor) RecordFormat(duration time.Duration, err error) {
	if !pm.IsEnabled() {
		return
	}
	
	pm.metrics.AddFormatTime(duration)
	
	if err != nil {
		pm.metrics.IncrementFormatError()
	}
	
	// `n�?	pm.mu.RLock()
	maxFormatTime := pm.maxFormatTime
	onSlowFormat := pm.onSlowFormat
	pm.mu.RUnlock()
	
	if duration > maxFormatTime && onSlowFormat != nil {
		onSlowFormat(duration)
	}
}

// CheckThresholds `n�?func (pm *PerformanceMonitor) CheckThresholds() {
	if !pm.IsEnabled() {
		return
	}
	
	snapshot := pm.metrics.GetSnapshot()
	
	pm.mu.RLock()
	maxErrorRate := pm.maxErrorRate
	maxDropRate := pm.maxDropRate
	onHighError := pm.onHighError
	onHighDrop := pm.onHighDrop
	pm.mu.RUnlock()
	
	// `n
	errorRate := snapshot.GetErrorRate()
	if errorRate > maxErrorRate && onHighError != nil {
		onHighError(errorRate)
	}
	
	// `n
	dropRate := snapshot.GetDropRate()
	if dropRate > maxDropRate && onHighDrop != nil {
		onHighDrop(dropRate)
	}
}






