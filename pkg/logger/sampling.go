// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Sampler 采样器接口
type Sampler interface {
	Sample(entry *Entry) bool
	GetSampleRate() float64
	SetSampleRate(rate float64)
	Reset()
	GetStats() SamplerStats
}

// SamplerStats 采样器统计信息
type SamplerStats struct {
	TotalSamples   int64   `json:"total_samples"`   // 总采样数
	AcceptedSamples int64  `json:"accepted_samples"` // 接受的采样数
	RejectedSamples int64  `json:"rejected_samples"` // 拒绝的采样数
	CurrentRate    float64 `json:"current_rate"`    // 当前采样率
	EffectiveRate  float64 `json:"effective_rate"`  // 实际采样率
}

// RandomSampler 随机采样器
type RandomSampler struct {
	mu         sync.RWMutex
	sampleRate float64
	rng        *rand.Rand
	stats      SamplerStats
}

// NewRandomSampler 创建随机采样器
func NewRandomSampler(sampleRate float64) *RandomSampler {
	if sampleRate < 0 {
		sampleRate = 0
	} else if sampleRate > 1 {
		sampleRate = 1
	}
	
	return &RandomSampler{
		sampleRate: sampleRate,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		stats: SamplerStats{
			CurrentRate: sampleRate,
		},
	}
}

// Sample 执行采样
func (s *RandomSampler) Sample(entry *Entry) bool {
	atomic.AddInt64(&s.stats.TotalSamples, 1)
	
	s.mu.RLock()
	rate := s.sampleRate
	s.mu.RUnlock()
	
	if rate >= 1.0 {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
		return true
	}
	
	if rate <= 0.0 {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
		return false
	}
	
	s.mu.Lock()
	accept := s.rng.Float64() < rate
	s.mu.Unlock()
	
	if accept {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
	} else {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
	}
	
	return accept
}

// GetSampleRate 获取采样率
func (s *RandomSampler) GetSampleRate() float64 {
	s.mu.RLock()
	rate := s.sampleRate
	s.mu.RUnlock()
	return rate
}

// SetSampleRate 设置采样率
func (s *RandomSampler) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	s.mu.Lock()
	s.sampleRate = rate
	s.stats.CurrentRate = rate
	s.mu.Unlock()
}

// Reset 重置统计信息
func (s *RandomSampler) Reset() {
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
}

// GetStats 获取统计信息
func (s *RandomSampler) GetStats() SamplerStats {
	total := atomic.LoadInt64(&s.stats.TotalSamples)
	accepted := atomic.LoadInt64(&s.stats.AcceptedSamples)
	
	var effectiveRate float64
	if total > 0 {
		effectiveRate = float64(accepted) / float64(total)
	}
	
	s.mu.RLock()
	currentRate := s.stats.CurrentRate
	s.mu.RUnlock()
	
	return SamplerStats{
		TotalSamples:    total,
		AcceptedSamples: accepted,
		RejectedSamples: atomic.LoadInt64(&s.stats.RejectedSamples),
		CurrentRate:     currentRate,
		EffectiveRate:   effectiveRate,
	}
}

// LevelBasedSampler 基于级别的采样器
type LevelBasedSampler struct {
	mu         sync.RWMutex
	levelRates map[Level]float64
	rng        *rand.Rand
	stats      map[Level]SamplerStats
}

// NewLevelBasedSampler 创建基于级别的采样器
func NewLevelBasedSampler(levelRates map[Level]float64) *LevelBasedSampler {
	if levelRates == nil {
		levelRates = make(map[Level]float64)
	}
	
	// 设置默认采样率
	defaultRates := map[Level]float64{
		TraceLevel: 0.1,  // 10%
		DebugLevel: 0.3,  // 30%
		InfoLevel:  0.8,  // 80%
		WarnLevel:  1.0,  // 100%
		ErrorLevel: 1.0,  // 100%
		FatalLevel: 1.0,  // 100%
		PanicLevel: 1.0,  // 100%
	}
	
	for level, rate := range defaultRates {
		if _, exists := levelRates[level]; !exists {
			levelRates[level] = rate
		}
	}
	
	return &LevelBasedSampler{
		levelRates: levelRates,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		stats:      make(map[Level]SamplerStats),
	}
}

// Sample 执行采样
func (s *LevelBasedSampler) Sample(entry *Entry) bool {
	s.mu.RLock()
	rate, exists := s.levelRates[entry.Level]
	s.mu.RUnlock()
	
	if !exists {
		rate = 1.0 // 默认100%采样
	}
	
	// 更新统计信息
	s.mu.Lock()
	stats := s.stats[entry.Level]
	stats.TotalSamples++
	stats.CurrentRate = rate
	
	accept := rate >= 1.0 || (rate > 0.0 && s.rng.Float64() < rate)
	if accept {
		stats.AcceptedSamples++
	} else {
		stats.RejectedSamples++
	}
	
	if stats.TotalSamples > 0 {
		stats.EffectiveRate = float64(stats.AcceptedSamples) / float64(stats.TotalSamples)
	}
	
	s.stats[entry.Level] = stats
	s.mu.Unlock()
	
	return accept
}

// GetSampleRate 获取平均采样率
func (s *LevelBasedSampler) GetSampleRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if len(s.levelRates) == 0 {
		return 0
	}
	
	var total float64
	for _, rate := range s.levelRates {
		total += rate
	}
	
	return total / float64(len(s.levelRates))
}

// SetSampleRate 设置所有级别的采样率
func (s *LevelBasedSampler) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	s.mu.Lock()
	for level := range s.levelRates {
		s.levelRates[level] = rate
	}
	s.mu.Unlock()
}

// SetLevelSampleRate 设置特定级别的采样率
func (s *LevelBasedSampler) SetLevelSampleRate(level Level, rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	s.mu.Lock()
	s.levelRates[level] = rate
	s.mu.Unlock()
}

// Reset 重置统计信息
func (s *LevelBasedSampler) Reset() {
	s.mu.Lock()
	s.stats = make(map[Level]SamplerStats)
	s.mu.Unlock()
}

// GetStats 获取统计信息
func (s *LevelBasedSampler) GetStats() SamplerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var totalSamples, acceptedSamples, rejectedSamples int64
	var totalRate float64
	
	for _, stats := range s.stats {
		totalSamples += stats.TotalSamples
		acceptedSamples += stats.AcceptedSamples
		rejectedSamples += stats.RejectedSamples
		totalRate += stats.CurrentRate
	}
	
	var currentRate, effectiveRate float64
	if len(s.stats) > 0 {
		currentRate = totalRate / float64(len(s.stats))
	}
	if totalSamples > 0 {
		effectiveRate = float64(acceptedSamples) / float64(totalSamples)
	}
	
	return SamplerStats{
		TotalSamples:    totalSamples,
		AcceptedSamples: acceptedSamples,
		RejectedSamples: rejectedSamples,
		CurrentRate:     currentRate,
		EffectiveRate:   effectiveRate,
	}
}

// GetLevelStats 获取特定级别的统计信息
func (s *LevelBasedSampler) GetLevelStats(level Level) SamplerStats {
	s.mu.RLock()
	stats, exists := s.stats[level]
	s.mu.RUnlock()
	
	if !exists {
		return SamplerStats{}
	}
	
	return stats
}

// HashBasedSampler 基于哈希的采样器
type HashBasedSampler struct {
	mu         sync.RWMutex
	sampleRate float64
	hashField  string // 用于哈希的字段名
	stats      SamplerStats
}

// NewHashBasedSampler 创建基于哈希的采样器
func NewHashBasedSampler(sampleRate float64, hashField string) *HashBasedSampler {
	if sampleRate < 0 {
		sampleRate = 0
	} else if sampleRate > 1 {
		sampleRate = 1
	}
	
	if hashField == "" {
		hashField = "message" // 默认使用消息字段
	}
	
	return &HashBasedSampler{
		sampleRate: sampleRate,
		hashField:  hashField,
		stats: SamplerStats{
			CurrentRate: sampleRate,
		},
	}
}

// Sample 执行采样
func (s *HashBasedSampler) Sample(entry *Entry) bool {
	atomic.AddInt64(&s.stats.TotalSamples, 1)
	
	s.mu.RLock()
	rate := s.sampleRate
	hashField := s.hashField
	s.mu.RUnlock()
	
	if rate >= 1.0 {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
		return true
	}
	
	if rate <= 0.0 {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
		return false
	}
	
	// 获取哈希值
	var hashValue string
	switch hashField {
	case "message":
		hashValue = entry.Message
	case "logger":
		// LoggerName field does not exist in Entry struct, use level instead
		hashValue = entry.Level.String()
	default:
		if value, exists := entry.Fields[hashField]; exists {
			hashValue = fmt.Sprintf("%v", value)
		} else {
			hashValue = entry.Message // 回退到消息
		}
	}
	
	// 计算哈希
	h := fnv.New64a()
	h.Write([]byte(hashValue))
	hash := h.Sum64()
	
	// 基于哈希值决定是否采样
	threshold := uint64(rate * math.MaxUint64)
	accept := hash <= threshold
	
	if accept {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
	} else {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
	}
	
	return accept
}

// GetSampleRate 获取采样率
func (s *HashBasedSampler) GetSampleRate() float64 {
	s.mu.RLock()
	rate := s.sampleRate
	s.mu.RUnlock()
	return rate
}

// SetSampleRate 设置采样率
func (s *HashBasedSampler) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	s.mu.Lock()
	s.sampleRate = rate
	s.stats.CurrentRate = rate
	s.mu.Unlock()
}

// Reset 重置统计信息
func (s *HashBasedSampler) Reset() {
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
}

// GetStats 获取统计信息
func (s *HashBasedSampler) GetStats() SamplerStats {
	total := atomic.LoadInt64(&s.stats.TotalSamples)
	accepted := atomic.LoadInt64(&s.stats.AcceptedSamples)
	
	var effectiveRate float64
	if total > 0 {
		effectiveRate = float64(accepted) / float64(total)
	}
	
	s.mu.RLock()
	currentRate := s.stats.CurrentRate
	s.mu.RUnlock()
	
	return SamplerStats{
		TotalSamples:    total,
		AcceptedSamples: accepted,
		RejectedSamples: atomic.LoadInt64(&s.stats.RejectedSamples),
		CurrentRate:     currentRate,
		EffectiveRate:   effectiveRate,
	}
}

// AdaptiveSampler 自适应采样器
type AdaptiveSampler struct {
	mu           sync.RWMutex
	baseSampleRate float64
	currentRate    float64
	targetTPS      float64 // 目标每秒事务数
	adjustInterval time.Duration
	lastAdjust     time.Time
	stats          SamplerStats
	rng            *rand.Rand
}

// NewAdaptiveSampler 创建自适应采样器
func NewAdaptiveSampler(baseSampleRate, targetTPS float64) *AdaptiveSampler {
	if baseSampleRate < 0 {
		baseSampleRate = 0
	} else if baseSampleRate > 1 {
		baseSampleRate = 1
	}
	
	if targetTPS <= 0 {
		targetTPS = 1000 // 默认1000 TPS
	}
	
	return &AdaptiveSampler{
		baseSampleRate: baseSampleRate,
		currentRate:    baseSampleRate,
		targetTPS:      targetTPS,
		adjustInterval: 10 * time.Second, // 每10秒调整一次
		lastAdjust:     time.Now(),
		stats: SamplerStats{
			CurrentRate: baseSampleRate,
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Sample 执行采样
func (s *AdaptiveSampler) Sample(entry *Entry) bool {
	atomic.AddInt64(&s.stats.TotalSamples, 1)
	
	// 检查是否需要调整采样率
	s.adjustRateIfNeeded()
	
	s.mu.RLock()
	rate := s.currentRate
	s.mu.RUnlock()
	
	if rate >= 1.0 {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
		return true
	}
	
	if rate <= 0.0 {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
		return false
	}
	
	s.mu.Lock()
	accept := s.rng.Float64() < rate
	s.mu.Unlock()
	
	if accept {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
	} else {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
	}
	
	return accept
}

// adjustRateIfNeeded 根据需要调整采样率
func (s *AdaptiveSampler) adjustRateIfNeeded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	if now.Sub(s.lastAdjust) < s.adjustInterval {
		return
	}
	
	// 计算当前TPS
	total := atomic.LoadInt64(&s.stats.TotalSamples)
	duration := now.Sub(s.lastAdjust).Seconds()
	currentTPS := float64(total) / duration
	
	// 调整采样率
	if currentTPS > s.targetTPS {
		// TPS过高，降低采样率
		s.currentRate *= 0.9
		if s.currentRate < 0.01 {
			s.currentRate = 0.01 // 最低1%
		}
	} else if currentTPS < s.targetTPS*0.8 {
		// TPS过低，提高采样率
		s.currentRate *= 1.1
		if s.currentRate > 1.0 {
			s.currentRate = 1.0
		}
	}
	
	s.stats.CurrentRate = s.currentRate
	s.lastAdjust = now
	
	// 重置计数器
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
}

// GetSampleRate 获取当前采样率
func (s *AdaptiveSampler) GetSampleRate() float64 {
	s.mu.RLock()
	rate := s.currentRate
	s.mu.RUnlock()
	return rate
}

// SetSampleRate 设置基础采样率
func (s *AdaptiveSampler) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	s.mu.Lock()
	s.baseSampleRate = rate
	s.currentRate = rate
	s.stats.CurrentRate = rate
	s.mu.Unlock()
}

// Reset 重置统计信息
func (s *AdaptiveSampler) Reset() {
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
	
	s.mu.Lock()
	s.currentRate = s.baseSampleRate
	s.stats.CurrentRate = s.baseSampleRate
	s.lastAdjust = time.Now()
	s.mu.Unlock()
}

// GetStats 获取统计信息
func (s *AdaptiveSampler) GetStats() SamplerStats {
	total := atomic.LoadInt64(&s.stats.TotalSamples)
	accepted := atomic.LoadInt64(&s.stats.AcceptedSamples)
	
	var effectiveRate float64
	if total > 0 {
		effectiveRate = float64(accepted) / float64(total)
	}
	
	s.mu.RLock()
	currentRate := s.stats.CurrentRate
	s.mu.RUnlock()
	
	return SamplerStats{
		TotalSamples:    total,
		AcceptedSamples: accepted,
		RejectedSamples: atomic.LoadInt64(&s.stats.RejectedSamples),
		CurrentRate:     currentRate,
		EffectiveRate:   effectiveRate,
	}
}






