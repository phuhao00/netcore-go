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

// Sampler `n�?type Sampler interface {
	Sample(entry *Entry) bool
	GetSampleRate() float64
	SetSampleRate(rate float64)
	Reset()
	GetStats() SamplerStats
}

// SamplerStats `n�?type SamplerStats struct {
	TotalSamples   int64   `json:"total_samples"`   // `n�?	AcceptedSamples int64  `json:"accepted_samples"` // `n�?	RejectedSamples int64  `json:"rejected_samples"` // `n�?	CurrentRate    float64 `json:"current_rate"`    // `n�?	EffectiveRate  float64 `json:"effective_rate"`  // `n�?}

// RandomSampler `n�?type RandomSampler struct {
	mu         sync.RWMutex
	sampleRate float64
	rng        *rand.Rand
	stats      SamplerStats
}

// NewRandomSampler `n�?func NewRandomSampler(sampleRate float64) *RandomSampler {
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

// Sample `n`nfunc (s *RandomSampler) Sample(entry *Entry) bool {
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

// GetSampleRate `n�?func (s *RandomSampler) GetSampleRate() float64 {
	s.mu.RLock()
	rate := s.sampleRate
	s.mu.RUnlock()
	return rate
}

// SetSampleRate `n�?func (s *RandomSampler) SetSampleRate(rate float64) {
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

// Reset `n�?func (s *RandomSampler) Reset() {
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
}

// GetStats `n`nfunc (s *RandomSampler) GetStats() SamplerStats {
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

// LevelBasedSampler `n`ntype LevelBasedSampler struct {
	mu         sync.RWMutex
	levelRates map[Level]float64
	rng        *rand.Rand
	stats      map[Level]SamplerStats
}

// NewLevelBasedSampler `n`nfunc NewLevelBasedSampler(levelRates map[Level]float64) *LevelBasedSampler {
	if levelRates == nil {
		levelRates = make(map[Level]float64)
	}
	
	// `n�?	defaultRates := map[Level]float64{
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

// Sample `n`nfunc (s *LevelBasedSampler) Sample(entry *Entry) bool {
	s.mu.RLock()
	rate, exists := s.levelRates[entry.Level]
	s.mu.RUnlock()
	
	if !exists {
		rate = 1.0 // `n100%`n
	}
	
	// `n
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

// GetSampleRate `n（`n）
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

// SetSampleRate `n（`n）
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

// SetLevelSampleRate `n`nfunc (s *LevelBasedSampler) SetLevelSampleRate(level Level, rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	s.mu.Lock()
	s.levelRates[level] = rate
	s.mu.Unlock()
}

// Reset `n�?func (s *LevelBasedSampler) Reset() {
	s.mu.Lock()
	s.stats = make(map[Level]SamplerStats)
	s.mu.Unlock()
}

// GetStats `n（`n�?func (s *LevelBasedSampler) GetStats() SamplerStats {
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

// GetLevelStats `n�?func (s *LevelBasedSampler) GetLevelStats(level Level) SamplerStats {
	s.mu.RLock()
	stats := s.stats[level]
	s.mu.RUnlock()
	return stats
}

// RateLimitSampler `n�?type RateLimitSampler struct {
	mu           sync.RWMutex
	maxRate      float64       // `n（`n）
	windowSize   time.Duration // `n
	tokens       float64       // `n�?	lastRefill   time.Time     // `n
	stats        SamplerStats
}

// NewRateLimitSampler `n�?func NewRateLimitSampler(maxRate float64, windowSize time.Duration) *RateLimitSampler {
	if maxRate <= 0 {
		maxRate = 100 // `n100�?	}
	if windowSize <= 0 {
		windowSize = time.Second
	}
	
	return &RateLimitSampler{
		maxRate:    maxRate,
		windowSize: windowSize,
		tokens:     maxRate,
		lastRefill: time.Now(),
		stats: SamplerStats{
			CurrentRate: 1.0, // `n�?00%
		},
	}
}

// Sample `n`nfunc (s *RateLimitSampler) Sample(entry *Entry) bool {
	atomic.AddInt64(&s.stats.TotalSamples, 1)
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// `n
	now := time.Now()
	elapsed := now.Sub(s.lastRefill)
	if elapsed > 0 {
		tokensToAdd := s.maxRate * elapsed.Seconds()
		s.tokens = math.Min(s.tokens+tokensToAdd, s.maxRate)
		s.lastRefill = now
	}
	
	// `n�?	if s.tokens >= 1.0 {
		s.tokens -= 1.0
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
		return true
	}
	
	atomic.AddInt64(&s.stats.RejectedSamples, 1)
	return false
}

// GetSampleRate `n�?func (s *RateLimitSampler) GetSampleRate() float64 {
	s.mu.RLock()
	rate := s.tokens / s.maxRate
	s.mu.RUnlock()
	return rate
}

// SetSampleRate `n（`n�?func (s *RateLimitSampler) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	
	s.mu.Lock()
	s.maxRate = rate * 100 // `n�?00
	s.tokens = math.Min(s.tokens, s.maxRate)
	s.mu.Unlock()
}

// SetMaxRate `n`nfunc (s *RateLimitSampler) SetMaxRate(maxRate float64) {
	if maxRate <= 0 {
		maxRate = 1
	}
	
	s.mu.Lock()
	s.maxRate = maxRate
	s.tokens = math.Min(s.tokens, maxRate)
	s.mu.Unlock()
}

// Reset `n�?func (s *RateLimitSampler) Reset() {
	s.mu.Lock()
	s.tokens = s.maxRate
	s.lastRefill = time.Now()
	s.mu.Unlock()
	
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
}

// GetStats `n`nfunc (s *RateLimitSampler) GetStats() SamplerStats {
	total := atomic.LoadInt64(&s.stats.TotalSamples)
	accepted := atomic.LoadInt64(&s.stats.AcceptedSamples)
	
	var effectiveRate float64
	if total > 0 {
		effectiveRate = float64(accepted) / float64(total)
	}
	
	return SamplerStats{
		TotalSamples:    total,
		AcceptedSamples: accepted,
		RejectedSamples: atomic.LoadInt64(&s.stats.RejectedSamples),
		CurrentRate:     s.GetSampleRate(),
		EffectiveRate:   effectiveRate,
	}
}

// HashBasedSampler `n`ntype HashBasedSampler struct {
	mu         sync.RWMutex
	sampleRate float64
	hashField  string // `n
	stats      SamplerStats
}

// NewHashBasedSampler `n`nfunc NewHashBasedSampler(sampleRate float64, hashField string) *HashBasedSampler {
	if sampleRate < 0 {
		sampleRate = 0
	} else if sampleRate > 1 {
		sampleRate = 1
	}
	
	if hashField == "" {
		hashField = "message" // `n
	}
	
	return &HashBasedSampler{
		sampleRate: sampleRate,
		hashField:  hashField,
		stats: SamplerStats{
			CurrentRate: sampleRate,
		},
	}
}

// Sample `n`nfunc (s *HashBasedSampler) Sample(entry *Entry) bool {
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
	
	// `n�?	var hashValue string
	switch hashField {
	case "message":
		hashValue = entry.Message
	default:
		if value, exists := entry.Fields[hashField]; exists {
			hashValue = fmt.Sprintf("%v", value)
		} else {
			hashValue = entry.Message
		}
	}
	
	// `n
	h := fnv.New64a()
	h.Write([]byte(hashValue))
	hash := h.Sum64()
	
	// `n�?	threshold := uint64(rate * float64(^uint64(0)))
	accept := hash <= threshold
	
	if accept {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
	} else {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
	}
	
	return accept
}

// GetSampleRate `n�?func (s *HashBasedSampler) GetSampleRate() float64 {
	s.mu.RLock()
	rate := s.sampleRate
	s.mu.RUnlock()
	return rate
}

// SetSampleRate `n�?func (s *HashBasedSampler) SetSampleRate(rate float64) {
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

// SetHashField `n`nfunc (s *HashBasedSampler) SetHashField(field string) {
	s.mu.Lock()
	s.hashField = field
	s.mu.Unlock()
}

// Reset `n�?func (s *HashBasedSampler) Reset() {
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
}

// GetStats `n`nfunc (s *HashBasedSampler) GetStats() SamplerStats {
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

// CompositeSampler `n�?type CompositeSampler struct {
	mu       sync.RWMutex
	samplers []Sampler
	mode     CompositeMode
	stats    SamplerStats
}

// CompositeMode `n`ntype CompositeMode int

const (
	CompositeAND CompositeMode = iota // `n�?	CompositeOR                       // `n�?)

// NewCompositeSampler `n�?func NewCompositeSampler(mode CompositeMode, samplers ...Sampler) *CompositeSampler {
	return &CompositeSampler{
		samplers: samplers,
		mode:     mode,
	}
}

// Sample `n`nfunc (s *CompositeSampler) Sample(entry *Entry) bool {
	atomic.AddInt64(&s.stats.TotalSamples, 1)
	
	s.mu.RLock()
	samplers := s.samplers
	mode := s.mode
	s.mu.RUnlock()
	
	if len(samplers) == 0 {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
		return true
	}
	
	var accept bool
	switch mode {
	case CompositeAND:
		accept = true
		for _, sampler := range samplers {
			if !sampler.Sample(entry) {
				accept = false
				break
			}
		}
	case CompositeOR:
		accept = false
		for _, sampler := range samplers {
			if sampler.Sample(entry) {
				accept = true
				break
			}
		}
	}
	
	if accept {
		atomic.AddInt64(&s.stats.AcceptedSamples, 1)
	} else {
		atomic.AddInt64(&s.stats.RejectedSamples, 1)
	}
	
	return accept
}

// GetSampleRate `n（`n）
func (s *CompositeSampler) GetSampleRate() float64 {
	s.mu.RLock()
	samplers := s.samplers
	s.mu.RUnlock()
	
	if len(samplers) == 0 {
		return 1.0
	}
	
	var total float64
	for _, sampler := range samplers {
		total += sampler.GetSampleRate()
	}
	
	return total / float64(len(samplers))
}

// SetSampleRate `n（`n）
func (s *CompositeSampler) SetSampleRate(rate float64) {
	s.mu.RLock()
	samplers := s.samplers
	s.mu.RUnlock()
	
	for _, sampler := range samplers {
		sampler.SetSampleRate(rate)
	}
}

// AddSampler `n�?func (s *CompositeSampler) AddSampler(sampler Sampler) {
	s.mu.Lock()
	s.samplers = append(s.samplers, sampler)
	s.mu.Unlock()
}

// Reset `n�?func (s *CompositeSampler) Reset() {
	s.mu.RLock()
	samplers := s.samplers
	s.mu.RUnlock()
	
	for _, sampler := range samplers {
		sampler.Reset()
	}
	
	atomic.StoreInt64(&s.stats.TotalSamples, 0)
	atomic.StoreInt64(&s.stats.AcceptedSamples, 0)
	atomic.StoreInt64(&s.stats.RejectedSamples, 0)
}

// GetStats `n`nfunc (s *CompositeSampler) GetStats() SamplerStats {
	total := atomic.LoadInt64(&s.stats.TotalSamples)
	accepted := atomic.LoadInt64(&s.stats.AcceptedSamples)
	
	var effectiveRate float64
	if total > 0 {
		effectiveRate = float64(accepted) / float64(total)
	}
	
	return SamplerStats{
		TotalSamples:    total,
		AcceptedSamples: accepted,
		RejectedSamples: atomic.LoadInt64(&s.stats.RejectedSamples),
		CurrentRate:     s.GetSampleRate(),
		EffectiveRate:   effectiveRate,
	}
}






