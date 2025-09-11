package performance

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// MemoryConfig 内存优化配置
type MemoryConfig struct {
	// 启用内存优化
	Enabled bool `json:"enabled" yaml:"enabled"`
	// GC目标百分比
	GCPercent int `json:"gc_percent" yaml:"gc_percent"`
	// 最大内存限制 (MB)
	MaxMemoryMB int64 `json:"max_memory_mb" yaml:"max_memory_mb"`
	// 内存池启用
	PoolEnabled bool `json:"pool_enabled" yaml:"pool_enabled"`
	// 预分配缓冲区大小
	PreallocBufferSize int `json:"prealloc_buffer_size" yaml:"prealloc_buffer_size"`
	// 监控间隔
	MonitorInterval time.Duration `json:"monitor_interval" yaml:"monitor_interval"`
	// 强制GC阈值
	ForceGCThreshold float64 `json:"force_gc_threshold" yaml:"force_gc_threshold"`
	// 启用内存压缩
	EnableCompaction bool `json:"enable_compaction" yaml:"enable_compaction"`
}

// DefaultMemoryConfig 默认内存配置
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		Enabled:            true,
		GCPercent:          100,
		MaxMemoryMB:        1024, // 1GB
		PoolEnabled:        true,
		PreallocBufferSize: 64 * 1024, // 64KB
		MonitorInterval:    30 * time.Second,
		ForceGCThreshold:   0.8, // 80%
		EnableCompaction:   true,
	}
}

// MemoryStats 内存统计信息
type MemoryStats struct {
	// 当前分配的内存 (bytes)
	Allocated int64 `json:"allocated"`
	// 总分配的内存 (bytes)
	TotalAlloc int64 `json:"total_alloc"`
	// 系统内存 (bytes)
	SysMemory int64 `json:"sys_memory"`
	// GC次数
	GCCount int64 `json:"gc_count"`
	// 上次GC时间
	LastGCTime time.Time `json:"last_gc_time"`
	// GC暂停时间 (nanoseconds)
	GCPauseTime int64 `json:"gc_pause_time"`
	// 堆对象数量
	HeapObjects int64 `json:"heap_objects"`
	// 内存使用率
	MemoryUsage float64 `json:"memory_usage"`
	// 池命中率
	PoolHitRate float64 `json:"pool_hit_rate"`
	// 池分配次数
	PoolAllocations int64 `json:"pool_allocations"`
	// 池释放次数
	PoolReleases int64 `json:"pool_releases"`
}

// MemoryPool 内存池
type MemoryPool struct {
	name      string
	size      int
	pool      *sync.Pool
	allocated int64
	released  int64
	hits      int64
	misses    int64
}

// NewMemoryPool 创建内存池
func NewMemoryPool(name string, size int) *MemoryPool {
	mp := &MemoryPool{
		name: name,
		size: size,
	}

	mp.pool = &sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&mp.misses, 1)
			return make([]byte, size)
		},
	}

	return mp
}

// Get 获取缓冲区
func (mp *MemoryPool) Get() []byte {
	atomic.AddInt64(&mp.allocated, 1)
	atomic.AddInt64(&mp.hits, 1)
	return mp.pool.Get().([]byte)
}

// Put 归还缓冲区
func (mp *MemoryPool) Put(buf []byte) {
	atomic.AddInt64(&mp.released, 1)
	// 重置缓冲区
	for i := range buf {
		buf[i] = 0
	}
	mp.pool.Put(buf)
}

// Stats 获取池统计信息
func (mp *MemoryPool) Stats() (allocated, released, hits, misses int64) {
	return atomic.LoadInt64(&mp.allocated),
		atomic.LoadInt64(&mp.released),
		atomic.LoadInt64(&mp.hits),
		atomic.LoadInt64(&mp.misses)
}

// MemoryManager 内存管理器
type MemoryManager struct {
	config    *MemoryConfig
	pools     map[int]*MemoryPool
	stats     *MemoryStats
	mu        sync.RWMutex
	running   bool
	cancel    context.CancelFunc
	lastStats runtime.MemStats
	alertChan chan MemoryAlert
}

// MemoryAlert 内存告警
type MemoryAlert struct {
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// NewMemoryManager 创建内存管理器
func NewMemoryManager(config *MemoryConfig) *MemoryManager {
	if config == nil {
		config = DefaultMemoryConfig()
	}

	mm := &MemoryManager{
		config:    config,
		pools:     make(map[int]*MemoryPool),
		stats:     &MemoryStats{},
		alertChan: make(chan MemoryAlert, 100),
	}

	// 初始化常用大小的内存池
	if config.PoolEnabled {
		sizes := []int{1024, 4096, 16384, 65536, 262144} // 1KB, 4KB, 16KB, 64KB, 256KB
		for _, size := range sizes {
			mm.pools[size] = NewMemoryPool(fmt.Sprintf("pool_%d", size), size)
		}
	}

	return mm
}

// Start 启动内存管理器
func (mm *MemoryManager) Start(ctx context.Context) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.running {
		return fmt.Errorf("memory manager already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	mm.cancel = cancel
	mm.running = true

	// 设置GC参数
	if mm.config.GCPercent >= 0 {
		debug.SetGCPercent(mm.config.GCPercent)
	}

	// 启动监控
	go mm.monitor(ctx)
	go mm.gcOptimizer(ctx)

	return nil
}

// Stop 停止内存管理器
func (mm *MemoryManager) Stop() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if !mm.running {
		return fmt.Errorf("memory manager not running")
	}

	mm.cancel()
	mm.running = false
	close(mm.alertChan)

	return nil
}

// GetBuffer 获取缓冲区
func (mm *MemoryManager) GetBuffer(size int) []byte {
	if !mm.config.PoolEnabled {
		return make([]byte, size)
	}

	// 找到最接近的池大小
	poolSize := mm.findPoolSize(size)
	if pool, exists := mm.pools[poolSize]; exists {
		buf := pool.Get()
		if len(buf) >= size {
			return buf[:size]
		}
	}

	// 如果没有合适的池，直接分配
	return make([]byte, size)
}

// PutBuffer 归还缓冲区
func (mm *MemoryManager) PutBuffer(buf []byte) {
	if !mm.config.PoolEnabled {
		return
	}

	size := cap(buf)
	poolSize := mm.findPoolSize(size)
	if pool, exists := mm.pools[poolSize]; exists && cap(buf) == poolSize {
		pool.Put(buf[:cap(buf)])
	}
}

// findPoolSize 找到合适的池大小
func (mm *MemoryManager) findPoolSize(size int) int {
	for poolSize := range mm.pools {
		if poolSize >= size {
			return poolSize
		}
	}
	// 如果没有合适的池，返回最大的池大小
	maxSize := 0
	for poolSize := range mm.pools {
		if poolSize > maxSize {
			maxSize = poolSize
		}
	}
	return maxSize
}

// monitor 监控内存使用情况
func (mm *MemoryManager) monitor(ctx context.Context) {
	ticker := time.NewTicker(mm.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mm.updateStats()
			mm.checkAlerts()
		}
	}
}

// updateStats 更新统计信息
func (mm *MemoryManager) updateStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.stats.Allocated = int64(m.Alloc)
	mm.stats.TotalAlloc = int64(m.TotalAlloc)
	mm.stats.SysMemory = int64(m.Sys)
	mm.stats.GCCount = int64(m.NumGC)
	mm.stats.HeapObjects = int64(m.HeapObjects)

	if m.NumGC > mm.lastStats.NumGC {
		mm.stats.LastGCTime = time.Unix(0, int64(m.LastGC))
		mm.stats.GCPauseTime = int64(m.PauseNs[(m.NumGC+255)%256])
	}

	// 计算内存使用率
	if mm.config.MaxMemoryMB > 0 {
		maxBytes := mm.config.MaxMemoryMB * 1024 * 1024
		mm.stats.MemoryUsage = float64(m.Alloc) / float64(maxBytes)
	}

	// 计算池命中率
	var totalHits, totalMisses int64
	for _, pool := range mm.pools {
		_, _, hits, misses := pool.Stats()
		totalHits += hits
		totalMisses += misses
	}

	if totalHits+totalMisses > 0 {
		mm.stats.PoolHitRate = float64(totalHits) / float64(totalHits+totalMisses)
	}

	mm.lastStats = m
}

// checkAlerts 检查告警条件
func (mm *MemoryManager) checkAlerts() {
	mm.mu.RLock()
	stats := *mm.stats
	mm.mu.RUnlock()

	// 内存使用率告警
	if stats.MemoryUsage > mm.config.ForceGCThreshold {
		alert := MemoryAlert{
			Type:      "memory_usage",
			Level:     "warning",
			Message:   fmt.Sprintf("Memory usage is %.2f%%, exceeding threshold", stats.MemoryUsage*100),
			Value:     stats.MemoryUsage,
			Threshold: mm.config.ForceGCThreshold,
			Timestamp: time.Now(),
		}

		select {
		case mm.alertChan <- alert:
		default:
			// 告警通道满了，丢弃告警
		}
	}

	// GC频率告警
	if time.Since(stats.LastGCTime) > 5*time.Minute {
		alert := MemoryAlert{
			Type:      "gc_frequency",
			Level:     "info",
			Message:   "GC has not run for more than 5 minutes",
			Value:     float64(time.Since(stats.LastGCTime).Minutes()),
			Threshold: 5.0,
			Timestamp: time.Now(),
		}

		select {
		case mm.alertChan <- alert:
		default:
		}
	}
}

// gcOptimizer GC优化器
func (mm *MemoryManager) gcOptimizer(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mm.optimizeGC()
		}
	}
}

// optimizeGC 优化GC
func (mm *MemoryManager) optimizeGC() {
	mm.mu.RLock()
	usage := mm.stats.MemoryUsage
	mm.mu.RUnlock()

	// 如果内存使用率过高，强制GC
	if usage > mm.config.ForceGCThreshold {
		runtime.GC()
		
		// 如果启用了内存压缩，尝试压缩
		if mm.config.EnableCompaction {
			debug.FreeOSMemory()
		}
	}

	// 动态调整GC百分比
	if usage > 0.6 {
		// 内存使用率高，降低GC百分比，更频繁GC
		debug.SetGCPercent(50)
	} else if usage < 0.3 {
		// 内存使用率低，提高GC百分比，减少GC频率
		debug.SetGCPercent(200)
	} else {
		// 正常范围，使用配置的GC百分比
		debug.SetGCPercent(mm.config.GCPercent)
	}
}

// GetStats 获取统计信息
func (mm *MemoryManager) GetStats() *MemoryStats {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// 计算池统计信息
	var totalAllocations, totalReleases int64
	for _, pool := range mm.pools {
		alloc, release, _, _ := pool.Stats()
		totalAllocations += alloc
		totalReleases += release
	}

	stats := *mm.stats
	stats.PoolAllocations = totalAllocations
	stats.PoolReleases = totalReleases

	return &stats
}

// GetAlerts 获取告警通道
func (mm *MemoryManager) GetAlerts() <-chan MemoryAlert {
	return mm.alertChan
}

// ForceGC 强制垃圾回收
func (mm *MemoryManager) ForceGC() {
	runtime.GC()
	if mm.config.EnableCompaction {
		debug.FreeOSMemory()
	}
}

// GetMemoryUsage 获取当前内存使用情况
func (mm *MemoryManager) GetMemoryUsage() (allocated, sys, gcCount uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc, m.Sys, uint64(m.NumGC)
}

// PreallocateBuffers 预分配缓冲区
func (mm *MemoryManager) PreallocateBuffers(count int) {
	if !mm.config.PoolEnabled {
		return
	}

	for _, pool := range mm.pools {
		for i := 0; i < count; i++ {
			buf := pool.Get()
			pool.Put(buf)
		}
	}
}

// MemoryProfiler 内存分析器
type MemoryProfiler struct {
	manager   *MemoryManager
	samples   []MemoryStats
	mu        sync.RWMutex
	maxSamples int
}

// NewMemoryProfiler 创建内存分析器
func NewMemoryProfiler(manager *MemoryManager, maxSamples int) *MemoryProfiler {
	return &MemoryProfiler{
		manager:   manager,
		samples:   make([]MemoryStats, 0, maxSamples),
		maxSamples: maxSamples,
	}
}

// Sample 采样内存统计
func (mp *MemoryProfiler) Sample() {
	stats := mp.manager.GetStats()

	mp.mu.Lock()
	defer mp.mu.Unlock()

	if len(mp.samples) >= mp.maxSamples {
		// 移除最老的样本
		copy(mp.samples, mp.samples[1:])
		mp.samples = mp.samples[:len(mp.samples)-1]
	}

	mp.samples = append(mp.samples, *stats)
}

// GetSamples 获取样本数据
func (mp *MemoryProfiler) GetSamples() []MemoryStats {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	samples := make([]MemoryStats, len(mp.samples))
	copy(samples, mp.samples)
	return samples
}

// AnalyzeMemoryLeaks 分析内存泄漏
func (mp *MemoryProfiler) AnalyzeMemoryLeaks() []string {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var issues []string

	if len(mp.samples) < 2 {
		return issues
	}

	// 检查内存是否持续增长
	first := mp.samples[0]
	last := mp.samples[len(mp.samples)-1]

	if last.Allocated > first.Allocated*2 {
		issues = append(issues, "Memory allocation has doubled during sampling period")
	}

	// 检查GC效果
	if last.GCCount > first.GCCount+10 && last.Allocated > first.Allocated {
		issues = append(issues, "Frequent GC but memory still growing")
	}

	// 检查池效率
	if last.PoolHitRate < 0.8 {
		issues = append(issues, fmt.Sprintf("Low pool hit rate: %.2f%%", last.PoolHitRate*100))
	}

	return issues
}

// UnsafeMemoryOperations 不安全的内存操作（高性能场景）
type UnsafeMemoryOperations struct{}

// BytesToString 零拷贝字节数组转字符串
func (UnsafeMemoryOperations) BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytes 零拷贝字符串转字节数组
func (UnsafeMemoryOperations) StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

// CopyMemory 高性能内存拷贝
func (UnsafeMemoryOperations) CopyMemory(dst, src []byte) {
	if len(dst) != len(src) {
		panic("dst and src must have the same length")
	}

	if len(src) == 0 {
		return
	}

	// 使用memmove进行高效拷贝
	*(*[]byte)(unsafe.Pointer(&dst)) = src
}

// GetUnsafeOperations 获取不安全操作实例
func GetUnsafeOperations() *UnsafeMemoryOperations {
	return &UnsafeMemoryOperations{}
}