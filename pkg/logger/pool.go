// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"
)

// ObjectPool `n�?type ObjectPool interface {
	Get() interface{}
	Put(interface{})
	Size() int
	Reset()
}

// EntryPool Entry`n�?type EntryPool struct {
	pool sync.Pool
	size int64
}

// NewEntryPool `nEntry`n�?func NewEntryPool() *EntryPool {
	return &EntryPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Entry{
					Fields: make(map[string]interface{}),
				}
			},
		},
	}
}

// Get `nEntry`n`nfunc (p *EntryPool) Get() *Entry {
	entry := p.pool.Get().(*Entry)
	atomic.AddInt64(&p.size, -1)
	return entry
}

// Put `nEntry`n`nfunc (p *EntryPool) Put(entry *Entry) {
	if entry == nil {
		return
	}
	
	// `nEntry`n
	entry.Time = time.Time{}
	entry.Level = InfoLevel
	entry.Message = ""
	entry.Error = ""
	entry.Caller = nil
	
	// `nmap`n`nfor k := range entry.Fields {
		delete(entry.Fields, k)
	}
	
	p.pool.Put(entry)
	atomic.AddInt64(&p.size, 1)
}

// Size `n�?func (p *EntryPool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset `n�?func (p *EntryPool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return &Entry{
				Fields: make(map[string]interface{}),
			}
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// BufferPool `n`ntype BufferPool struct {
	pool     sync.Pool
	size     int64
	maxSize  int // `n
	initSize int // `n�?}

// NewBufferPool `n`nfunc NewBufferPool(initSize, maxSize int) *BufferPool {
	if initSize <= 0 {
		initSize = 1024 // `n1KB
	}
	if maxSize <= 0 {
		maxSize = 64 * 1024 // `n64KB
	}
	
	return &BufferPool{
		initSize: initSize,
		maxSize:  maxSize,
		pool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, initSize))
			},
		},
	}
}

// Get `n�?func (p *BufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get().(*bytes.Buffer)
	buf.Reset()
	atomic.AddInt64(&p.size, -1)
	return buf
}

// Put `n�?func (p *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	
	// `n，`n�?	if buf.Cap() > p.maxSize {
		return
	}
	
	buf.Reset()
	p.pool.Put(buf)
	atomic.AddInt64(&p.size, 1)
}

// Size `n�?func (p *BufferPool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset `n�?func (p *BufferPool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, p.initSize))
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// SlicePool `n�?type SlicePool struct {
	pool     sync.Pool
	size     int64
	maxSize  int // `n�?	initSize int // `n
}

// NewSlicePool `n�?func NewSlicePool(initSize, maxSize int) *SlicePool {
	if initSize <= 0 {
		initSize = 64 // `n64`n�?	}
	if maxSize <= 0 {
		maxSize = 1024 // `n1024`n�?	}
	
	return &SlicePool{
		initSize: initSize,
		maxSize:  maxSize,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, initSize)
			},
		},
	}
}

// Get `n`nfunc (p *SlicePool) Get() []byte {
	slice := p.pool.Get().([]byte)
	slice = slice[:0] // `n�?	atomic.AddInt64(&p.size, -1)
	return slice
}

// Put `n`nfunc (p *SlicePool) Put(slice []byte) {
	if slice == nil {
		return
	}
	
	// `n，`n`nif cap(slice) > p.maxSize {
		return
	}
	
	slice = slice[:0] // `n
	p.pool.Put(slice)
	atomic.AddInt64(&p.size, 1)
}

// Size `n�?func (p *SlicePool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset `n�?func (p *SlicePool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, p.initSize)
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// MapPool `n�?type MapPool struct {
	pool    sync.Pool
	size    int64
	maxSize int // `n�?}

// NewMapPool `n�?func NewMapPool(maxSize int) *MapPool {
	if maxSize <= 0 {
		maxSize = 100 // `n100`n
	}
	
	return &MapPool{
		maxSize: maxSize,
		pool: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{})
			},
		},
	}
}

// Get `n`nfunc (p *MapPool) Get() map[string]interface{} {
	m := p.pool.Get().(map[string]interface{})
	// `n�?	for k := range m {
		delete(m, k)
	}
	atomic.AddInt64(&p.size, -1)
	return m
}

// Put `n`nfunc (p *MapPool) Put(m map[string]interface{}) {
	if m == nil {
		return
	}
	
	// `n，`n`nif len(m) > p.maxSize {
		return
	}
	
	// `n`nfor k := range m {
		delete(m, k)
	}
	
	p.pool.Put(m)
	atomic.AddInt64(&p.size, 1)
}

// Size `n�?func (p *MapPool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset `n�?func (p *MapPool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{})
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// PoolManager `n`ntype PoolManager struct {
	mu sync.RWMutex
	
	entryPool  *EntryPool
	bufferPool *BufferPool
	slicePool  *SlicePool
	mapPool    *MapPool
	
	// `n
	stats PoolStats
	
	// `n
	config PoolConfig
}

// PoolConfig `n�?type PoolConfig struct {
	BufferInitSize int `json:"buffer_init_size"` // `n�?	BufferMaxSize  int `json:"buffer_max_size"`  // `n�?	SliceInitSize  int `json:"slice_init_size"`  // `n
	SliceMaxSize   int `json:"slice_max_size"`   // `n�?	MapMaxSize     int `json:"map_max_size"`     // `n�?	EnableMetrics  bool `json:"enable_metrics"`   // `n
}

// PoolStats `n�?type PoolStats struct {
	EntryGets    int64 `json:"entry_gets"`    // Entry`n
	EntryPuts    int64 `json:"entry_puts"`    // Entry`n
	BufferGets   int64 `json:"buffer_gets"`   // Buffer`n
	BufferPuts   int64 `json:"buffer_puts"`   // Buffer`n
	SliceGets    int64 `json:"slice_gets"`    // Slice`n
	SlicePuts    int64 `json:"slice_puts"`    // Slice`n
	MapGets      int64 `json:"map_gets"`      // Map`n
	MapPuts      int64 `json:"map_puts"`      // Map`n
	TotalResets  int64 `json:"total_resets"`  // `n�?}

// NewPoolManager `n`nfunc NewPoolManager(config PoolConfig) *PoolManager {
	// `n�?	if config.BufferInitSize <= 0 {
		config.BufferInitSize = 1024
	}
	if config.BufferMaxSize <= 0 {
		config.BufferMaxSize = 64 * 1024
	}
	if config.SliceInitSize <= 0 {
		config.SliceInitSize = 64
	}
	if config.SliceMaxSize <= 0 {
		config.SliceMaxSize = 1024
	}
	if config.MapMaxSize <= 0 {
		config.MapMaxSize = 100
	}
	
	return &PoolManager{
		entryPool:  NewEntryPool(),
		bufferPool: NewBufferPool(config.BufferInitSize, config.BufferMaxSize),
		slicePool:  NewSlicePool(config.SliceInitSize, config.SliceMaxSize),
		mapPool:    NewMapPool(config.MapMaxSize),
		config:     config,
	}
}

// GetEntry `nEntry`n`nfunc (pm *PoolManager) GetEntry() *Entry {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.EntryGets, 1)
	}
	return pm.entryPool.Get()
}

// PutEntry `nEntry`n`nfunc (pm *PoolManager) PutEntry(entry *Entry) {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.EntryPuts, 1)
	}
	pm.entryPool.Put(entry)
}

// GetBuffer `n�?func (pm *PoolManager) GetBuffer() *bytes.Buffer {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.BufferGets, 1)
	}
	return pm.bufferPool.Get()
}

// PutBuffer `n�?func (pm *PoolManager) PutBuffer(buf *bytes.Buffer) {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.BufferPuts, 1)
	}
	pm.bufferPool.Put(buf)
}

// GetSlice `n`nfunc (pm *PoolManager) GetSlice() []byte {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.SliceGets, 1)
	}
	return pm.slicePool.Get()
}

// PutSlice `n`nfunc (pm *PoolManager) PutSlice(slice []byte) {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.SlicePuts, 1)
	}
	pm.slicePool.Put(slice)
}

// GetMap `n`nfunc (pm *PoolManager) GetMap() map[string]interface{} {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.MapGets, 1)
	}
	return pm.mapPool.Get()
}

// PutMap `n`nfunc (pm *PoolManager) PutMap(m map[string]interface{}) {
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.MapPuts, 1)
	}
	pm.mapPool.Put(m)
}

// GetStats `n`nfunc (pm *PoolManager) GetStats() PoolStats {
	pm.mu.RLock()
	stats := PoolStats{
		EntryGets:   atomic.LoadInt64(&pm.stats.EntryGets),
		EntryPuts:   atomic.LoadInt64(&pm.stats.EntryPuts),
		BufferGets:  atomic.LoadInt64(&pm.stats.BufferGets),
		BufferPuts:  atomic.LoadInt64(&pm.stats.BufferPuts),
		SliceGets:   atomic.LoadInt64(&pm.stats.SliceGets),
		SlicePuts:   atomic.LoadInt64(&pm.stats.SlicePuts),
		MapGets:     atomic.LoadInt64(&pm.stats.MapGets),
		MapPuts:     atomic.LoadInt64(&pm.stats.MapPuts),
		TotalResets: atomic.LoadInt64(&pm.stats.TotalResets),
	}
	pm.mu.RUnlock()
	return stats
}

// GetPoolSizes `n`nfunc (pm *PoolManager) GetPoolSizes() map[string]int {
	return map[string]int{
		"entry":  pm.entryPool.Size(),
		"buffer": pm.bufferPool.Size(),
		"slice":  pm.slicePool.Size(),
		"map":    pm.mapPool.Size(),
	}
}

// Reset `n`nfunc (pm *PoolManager) Reset() {
	pm.mu.Lock()
	pm.entryPool.Reset()
	pm.bufferPool.Reset()
	pm.slicePool.Reset()
	pm.mapPool.Reset()
	
	if pm.config.EnableMetrics {
		atomic.AddInt64(&pm.stats.TotalResets, 1)
	}
	pm.mu.Unlock()
}

// ResetStats `n`nfunc (pm *PoolManager) ResetStats() {
	atomic.StoreInt64(&pm.stats.EntryGets, 0)
	atomic.StoreInt64(&pm.stats.EntryPuts, 0)
	atomic.StoreInt64(&pm.stats.BufferGets, 0)
	atomic.StoreInt64(&pm.stats.BufferPuts, 0)
	atomic.StoreInt64(&pm.stats.SliceGets, 0)
	atomic.StoreInt64(&pm.stats.SlicePuts, 0)
	atomic.StoreInt64(&pm.stats.MapGets, 0)
	atomic.StoreInt64(&pm.stats.MapPuts, 0)
	atomic.StoreInt64(&pm.stats.TotalResets, 0)
}

// GetHitRate `n�?func (pm *PoolManager) GetHitRate() map[string]float64 {
	stats := pm.GetStats()
	
	calcHitRate := func(gets, puts int64) float64 {
		if gets == 0 {
			return 0
		}
		return float64(puts) / float64(gets)
	}
	
	return map[string]float64{
		"entry":  calcHitRate(stats.EntryGets, stats.EntryPuts),
		"buffer": calcHitRate(stats.BufferGets, stats.BufferPuts),
		"slice":  calcHitRate(stats.SliceGets, stats.SlicePuts),
		"map":    calcHitRate(stats.MapGets, stats.MapPuts),
	}
}

// `n`nvar (
	globalPoolManager *PoolManager
	globalPoolOnce    sync.Once
)

// GetGlobalPoolManager `n`nfunc GetGlobalPoolManager() *PoolManager {
	globalPoolOnce.Do(func() {
		globalPoolManager = NewPoolManager(PoolConfig{
			BufferInitSize: 1024,
			BufferMaxSize:  64 * 1024,
			SliceInitSize:  64,
			SliceMaxSize:   1024,
			MapMaxSize:     100,
			EnableMetrics:  true,
		})
	})
	return globalPoolManager
}

// SetGlobalPoolManager `n`nfunc SetGlobalPoolManager(pm *PoolManager) {
	globalPoolManager = pm
}






