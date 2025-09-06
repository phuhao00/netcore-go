// Package pool 实现NetCore-Go网络库的资源池
// Author: NetCore-Go Team
// Created: 2024

package pool

import (
	"sync"
)

// MemoryPool 内存池接口
type MemoryPool interface {
	// Get 获取缓冲区
	Get() []byte
	// Put 归还缓冲区
	Put(buf []byte)
	// Size 获取缓冲区大小
	Size() int
	// Stats 获取池统计信息
	Stats() *PoolStats
}

// BytePool 字节缓冲区池实现
type BytePool struct {
	bufferSize int
	pool       sync.Pool
	stats      *PoolStats
	mu         sync.RWMutex
}

// NewBytePool 创建新的字节缓冲区池
func NewBytePool(bufferSize int) *BytePool {
	p := &BytePool{
		bufferSize: bufferSize,
		stats:      NewPoolStats(),
	}
	p.pool = sync.Pool{
		New: func() interface{} {
			p.mu.Lock()
			p.stats.Created++
			p.mu.Unlock()
			return make([]byte, bufferSize)
		},
	}
	return p
}

// Get 获取缓冲区
func (p *BytePool) Get() []byte {
	p.mu.Lock()
	p.stats.Gets++
	p.stats.InUse++
	p.mu.Unlock()

	buf := p.pool.Get().([]byte)
	return buf[:0] // 重置长度但保留容量
}

// Put 归还缓冲区
func (p *BytePool) Put(buf []byte) {
	if buf == nil || cap(buf) != p.bufferSize {
		return
	}

	p.mu.Lock()
	p.stats.Puts++
	p.stats.InUse--
	p.mu.Unlock()

	p.pool.Put(buf)
}

// Size 获取缓冲区大小
func (p *BytePool) Size() int {
	return p.bufferSize
}

// Stats 获取池统计信息
func (p *BytePool) Stats() *PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return &PoolStats{
		Created: p.stats.Created,
		Gets:    p.stats.Gets,
		Puts:    p.stats.Puts,
		InUse:   p.stats.InUse,
	}
}

// MultiSizeMemoryPool 多尺寸内存池
type MultiSizeMemoryPool struct {
	pools map[int]*BytePool
	sizes []int
	mu    sync.RWMutex
}

// NewMultiSizeMemoryPool 创建多尺寸内存池
func NewMultiSizeMemoryPool(sizes []int) *MultiSizeMemoryPool {
	pools := make(map[int]*BytePool)
	for _, size := range sizes {
		pools[size] = NewBytePool(size)
	}
	return &MultiSizeMemoryPool{
		pools: pools,
		sizes: sizes,
	}
}

// Get 获取指定大小的缓冲区
func (p *MultiSizeMemoryPool) Get(size int) []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 找到最小的满足需求的池
	for _, poolSize := range p.sizes {
		if poolSize >= size {
			if pool, exists := p.pools[poolSize]; exists {
				return pool.Get()
			}
		}
	}

	// 如果没有合适的池，直接分配
	return make([]byte, 0, size)
}

// Put 归还缓冲区
func (p *MultiSizeMemoryPool) Put(buf []byte) {
	if buf == nil {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	size := cap(buf)
	if pool, exists := p.pools[size]; exists {
		pool.Put(buf)
	}
}

// GetStats 获取所有池的统计信息
func (p *MultiSizeMemoryPool) GetStats() map[int]*PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[int]*PoolStats)
	for size, pool := range p.pools {
		stats[size] = pool.Stats()
	}
	return stats
}

// PoolStats 池统计信息
type PoolStats struct {
	// Created 创建的对象数量
	Created int64 `json:"created"`
	// Gets 获取操作次数
	Gets int64 `json:"gets"`
	// Puts 归还操作次数
	Puts int64 `json:"puts"`
	// InUse 正在使用的对象数量
	InUse int64 `json:"in_use"`
}

// NewPoolStats 创建新的池统计信息
func NewPoolStats() *PoolStats {
	return &PoolStats{}
}

// HitRate 计算命中率
func (s *PoolStats) HitRate() float64 {
	if s.Gets == 0 {
		return 0
	}
	return float64(s.Gets-s.Created) / float64(s.Gets)
}

// 默认内存池实例
var (
	// DefaultMemoryPool 默认内存池（4KB缓冲区）
	DefaultMemoryPool = NewBytePool(4096)

	// DefaultMultiSizePool 默认多尺寸内存池
	DefaultMultiSizePool = NewMultiSizeMemoryPool([]int{
		512,   // 512B
		1024,  // 1KB
		2048,  // 2KB
		4096,  // 4KB
		8192,  // 8KB
		16384, // 16KB
		32768, // 32KB
		65536, // 64KB
	})
)

// GetBuffer 从默认内存池获取缓冲区
func GetBuffer() []byte {
	return DefaultMemoryPool.Get()
}

// PutBuffer 归还缓冲区到默认内存池
func PutBuffer(buf []byte) {
	DefaultMemoryPool.Put(buf)
}

// GetSizedBuffer 从默认多尺寸池获取指定大小的缓冲区
func GetSizedBuffer(size int) []byte {
	return DefaultMultiSizePool.Get(size)
}

// PutSizedBuffer 归还缓冲区到默认多尺寸池
func PutSizedBuffer(buf []byte) {
	DefaultMultiSizePool.Put(buf)
}