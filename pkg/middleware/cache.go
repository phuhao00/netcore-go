// Package middleware 缓存中间件实现
// Author: NetCore-Go Team
// Created: 2024

package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      interface{} `json:"data"`
	ExpiredAt int64       `json:"expired_at"`
	CreatedAt int64       `json:"created_at"`
}

// IsExpired 检查是否过期
func (e *CacheEntry) IsExpired() bool {
	if e.ExpiredAt == 0 {
		return false // 永不过期
	}
	return time.Now().Unix() > e.ExpiredAt
}

// CacheStore 缓存存储接口
type CacheStore interface {
	Get(key string) (*CacheEntry, bool)
	Set(key string, entry *CacheEntry) error
	Delete(key string) error
	Clear() error
	Stats() CacheStats
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits        int64 `json:"hits"`
	Misses      int64 `json:"misses"`
	Sets        int64 `json:"sets"`
	Deletes     int64 `json:"deletes"`
	Size        int64 `json:"size"`
	HitRate     float64 `json:"hit_rate"`
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	data    map[string]*CacheEntry
	mutex   sync.RWMutex
	stats   CacheStats
	maxSize int64
	ttl     time.Duration
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int64, ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		data:    make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

// Get 获取缓存
func (c *MemoryCache) Get(key string) (*CacheEntry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	if entry.IsExpired() {
		c.stats.Misses++
		go c.Delete(key) // 异步删除过期条目
		return nil, false
	}

	c.stats.Hits++
	c.updateHitRate()
	return entry, true
}

// Set 设置缓存
func (c *MemoryCache) Set(key string, entry *CacheEntry) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查大小限制
	if c.maxSize > 0 && int64(len(c.data)) >= c.maxSize {
		// 简单的LRU：删除最旧的条目
		c.evictOldest()
	}

	c.data[key] = entry
	c.stats.Sets++
	c.stats.Size = int64(len(c.data))

	return nil
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.data[key]; exists {
		delete(c.data, key)
		c.stats.Deletes++
		c.stats.Size = int64(len(c.data))
	}

	return nil
}

// Clear 清空缓存
func (c *MemoryCache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = make(map[string]*CacheEntry)
	c.stats.Size = 0

	return nil
}

// Stats 获取统计信息
func (c *MemoryCache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.updateHitRate()
	return c.stats
}

// updateHitRate 更新命中率
func (c *MemoryCache) updateHitRate() {
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total)
	}
}

// evictOldest 驱逐最旧的条目
func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime int64 = time.Now().Unix()

	for key, entry := range c.data {
		if entry.CreatedAt < oldestTime {
			oldestTime = entry.CreatedAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// cleanup 清理过期条目
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		for key, entry := range c.data {
			if entry.IsExpired() {
				delete(c.data, key)
				c.stats.Deletes++
			}
		}
		c.stats.Size = int64(len(c.data))
		c.mutex.Unlock()
	}
}

// CacheConfig 缓存配置
type CacheConfig struct {
	TTL         time.Duration
	MaxSize     int64
	KeyPrefix   string
	SkipPaths   []string
	CacheStore  CacheStore
	KeyGenerator func(ctx core.Context) string
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		TTL:       5 * time.Minute,
		MaxSize:   1000,
		KeyPrefix: "netcore:",
		SkipPaths: []string{"/health", "/metrics"},
		CacheStore: NewMemoryCache(1000, 5*time.Minute),
		KeyGenerator: func(ctx core.Context) string {
			// 默认使用连接ID和消息内容生成key
			connID := ctx.Connection().ID()
			msg := ctx.Message()
			if msg != nil {
				hash := md5.Sum(msg.GetData())
				return fmt.Sprintf("%s:%s", connID, hex.EncodeToString(hash[:]))
			}
			return connID
		},
	}
}

// CacheMiddleware 缓存中间件
type CacheMiddleware struct {
	config *CacheConfig
}

// NewCacheMiddleware 创建缓存中间件
func NewCacheMiddleware(config *CacheConfig) *CacheMiddleware {
	if config == nil {
		config = DefaultCacheConfig()
	}
	return &CacheMiddleware{
		config: config,
	}
}

// Process 处理缓存
func (m *CacheMiddleware) Process(ctx core.Context, next core.Handler) error {
	// 检查是否跳过缓存
	if m.shouldSkip(ctx) {
		return next(ctx)
	}

	// 生成缓存key
	cacheKey := m.config.KeyPrefix + m.config.KeyGenerator(ctx)

	// 尝试从缓存获取
	if entry, exists := m.config.CacheStore.Get(cacheKey); exists {
		// 缓存命中，直接返回缓存数据
		ctx.Set("cache_hit", true)
		ctx.Set("cached_data", entry.Data)
		ctx.Set("cache_created_at", entry.CreatedAt)
		return nil
	}

	// 缓存未命中，执行下一个处理器
	ctx.Set("cache_hit", false)
	err := next(ctx)
	if err != nil {
		return err
	}

	// 获取处理结果并缓存
	if result, exists := ctx.Get("response_data"); exists {
		entry := &CacheEntry{
			Data:      result,
			CreatedAt: time.Now().Unix(),
		}

		if m.config.TTL > 0 {
			entry.ExpiredAt = time.Now().Add(m.config.TTL).Unix()
		}

		m.config.CacheStore.Set(cacheKey, entry)
	}

	return nil
}

// Name 获取中间件名称
func (m *CacheMiddleware) Name() string {
	return "cache"
}

// Priority 获取中间件优先级
func (m *CacheMiddleware) Priority() int {
	return 60
}

// shouldSkip 检查是否应该跳过缓存
func (m *CacheMiddleware) shouldSkip(ctx core.Context) bool {
	// 检查路径是否在跳过列表中
	if path, exists := ctx.Get("request_path"); exists {
		if pathStr, ok := path.(string); ok {
			for _, skipPath := range m.config.SkipPaths {
				if pathStr == skipPath {
					return true
				}
			}
		}
	}

	// 检查HTTP方法，只缓存GET请求
	if method, exists := ctx.Get("request_method"); exists {
		if methodStr, ok := method.(string); ok {
			return methodStr != "GET"
		}
	}

	return false
}

// GetStats 获取缓存统计信息
func (m *CacheMiddleware) GetStats() CacheStats {
	return m.config.CacheStore.Stats()
}

// ClearCache 清空缓存
func (m *CacheMiddleware) ClearCache() error {
	return m.config.CacheStore.Clear()
}

// DeleteCache 删除指定缓存
func (m *CacheMiddleware) DeleteCache(key string) error {
	return m.config.CacheStore.Delete(m.config.KeyPrefix + key)
}

