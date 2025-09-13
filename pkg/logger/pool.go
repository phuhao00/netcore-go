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

// ObjectPool 对象池接口
type ObjectPool interface {
	Get() interface{}
	Put(interface{})
	Size() int
	Reset()
}

// EntryPool Entry对象池
type EntryPool struct {
	pool sync.Pool
	size int64
}

// NewEntryPool 创建Entry对象池
func NewEntryPool() *EntryPool {
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

// Get 获取Entry对象
func (p *EntryPool) Get() *Entry {
	entry := p.pool.Get().(*Entry)
	atomic.AddInt64(&p.size, -1)
	return entry
}

// Put 归还Entry对象
func (p *EntryPool) Put(entry *Entry) {
	if entry == nil {
		return
	}
	
	// 重置Entry字段
	entry.Time = time.Time{}
	entry.Level = InfoLevel
	entry.Message = ""
	entry.Error = ""
	entry.Caller = nil
	
	// 清空map字段
	for k := range entry.Fields {
		delete(entry.Fields, k)
	}
	
	p.pool.Put(entry)
	atomic.AddInt64(&p.size, 1)
}

// Size 获取池大小
func (p *EntryPool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset 重置对象池
func (p *EntryPool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return &Entry{
				Fields: make(map[string]interface{}),
			}
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// BufferPool 缓冲区对象池
type BufferPool struct {
	pool     sync.Pool
	size     int64
	maxSize  int // 最大缓冲区大小
	initSize int // 初始缓冲区大小
}

// NewBufferPool 创建缓冲区对象池
func NewBufferPool(initSize, maxSize int) *BufferPool {
	if initSize <= 0 {
		initSize = 1024 // 默认1KB
	}
	if maxSize <= 0 {
		maxSize = 64 * 1024 // 默认64KB
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

// Get 获取缓冲区
func (p *BufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get().(*bytes.Buffer)
	buf.Reset()
	atomic.AddInt64(&p.size, -1)
	return buf
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	
	// 如果缓冲区太大，不放回池中
	if buf.Cap() > p.maxSize {
		return
	}
	
	buf.Reset()
	p.pool.Put(buf)
	atomic.AddInt64(&p.size, 1)
}

// Size 获取池大小
func (p *BufferPool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset 重置对象池
func (p *BufferPool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, p.initSize))
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// SlicePool 字节切片对象池
type SlicePool struct {
	pool     sync.Pool
	size     int64
	maxSize  int // 最大切片容量
	initSize int // 初始切片容量
}

// NewSlicePool 创建字节切片对象池
func NewSlicePool(initSize, maxSize int) *SlicePool {
	if initSize <= 0 {
		initSize = 64 // 默认64字节
	}
	if maxSize <= 0 {
		maxSize = 1024 // 默认1024字节
	}
	
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

// Get 获取字节切片
func (p *SlicePool) Get() []byte {
	slice := p.pool.Get().([]byte)
	slice = slice[:0] // 重置长度
	atomic.AddInt64(&p.size, -1)
	return slice
}

// Put 归还字节切片
func (p *SlicePool) Put(slice []byte) {
	if slice == nil {
		return
	}
	
	// 如果切片太大，不放回池中
	if cap(slice) > p.maxSize {
		return
	}
	
	slice = slice[:0] // 重置长度
	p.pool.Put(slice)
	atomic.AddInt64(&p.size, 1)
}

// Size 获取池大小
func (p *SlicePool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset 重置对象池
func (p *SlicePool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, p.initSize)
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// MapPool 映射对象池
type MapPool struct {
	pool    sync.Pool
	size    int64
	maxSize int // 最大映射大小
}

// NewMapPool 创建映射对象池
func NewMapPool(maxSize int) *MapPool {
	if maxSize <= 0 {
		maxSize = 100 // 默认100个键值对
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

// Get 获取映射
func (p *MapPool) Get() map[string]interface{} {
	m := p.pool.Get().(map[string]interface{})
	atomic.AddInt64(&p.size, -1)
	return m
}

// Put 归还映射
func (p *MapPool) Put(m map[string]interface{}) {
	if m == nil {
		return
	}
	
	// 如果映射太大，不放回池中
	if len(m) > p.maxSize {
		return
	}
	
	// 清空映射
	for k := range m {
		delete(m, k)
	}
	
	p.pool.Put(m)
	atomic.AddInt64(&p.size, 1)
}

// Size 获取池大小
func (p *MapPool) Size() int {
	return int(atomic.LoadInt64(&p.size))
}

// Reset 重置对象池
func (p *MapPool) Reset() {
	p.pool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{})
		},
	}
	atomic.StoreInt64(&p.size, 0)
}

// PoolManager 对象池管理器
type PoolManager struct {
	entryPool  *EntryPool
	bufferPool *BufferPool
	slicePool  *SlicePool
	mapPool    *MapPool
	mu         sync.RWMutex
}

// NewPoolManager 创建对象池管理器
func NewPoolManager() *PoolManager {
	return &PoolManager{
		entryPool:  NewEntryPool(),
		bufferPool: NewBufferPool(1024, 64*1024),
		slicePool:  NewSlicePool(64, 1024),
		mapPool:    NewMapPool(100),
	}
}

// GetEntry 获取Entry对象
func (pm *PoolManager) GetEntry() *Entry {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.entryPool.Get()
}

// PutEntry 归还Entry对象
func (pm *PoolManager) PutEntry(entry *Entry) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pm.entryPool.Put(entry)
}

// GetBuffer 获取缓冲区
func (pm *PoolManager) GetBuffer() *bytes.Buffer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.bufferPool.Get()
}

// PutBuffer 归还缓冲区
func (pm *PoolManager) PutBuffer(buf *bytes.Buffer) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pm.bufferPool.Put(buf)
}

// GetSlice 获取字节切片
func (pm *PoolManager) GetSlice() []byte {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.slicePool.Get()
}

// PutSlice 归还字节切片
func (pm *PoolManager) PutSlice(slice []byte) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pm.slicePool.Put(slice)
}

// GetMap 获取映射
func (pm *PoolManager) GetMap() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.mapPool.Get()
}

// PutMap 归还映射
func (pm *PoolManager) PutMap(m map[string]interface{}) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pm.mapPool.Put(m)
}

// GetStats 获取池统计信息
func (pm *PoolManager) GetStats() map[string]int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	return map[string]int{
		"entry_pool_size":  pm.entryPool.Size(),
		"buffer_pool_size": pm.bufferPool.Size(),
		"slice_pool_size":  pm.slicePool.Size(),
		"map_pool_size":    pm.mapPool.Size(),
	}
}

// Reset 重置所有对象池
func (pm *PoolManager) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.entryPool.Reset()
	pm.bufferPool.Reset()
	pm.slicePool.Reset()
	pm.mapPool.Reset()
}






