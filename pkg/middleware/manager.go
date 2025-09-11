// Package middleware 中间件管理器实现
// Author: NetCore-Go Team
// Created: 2024

package middleware

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// MiddlewareManager 中间件管理器
type MiddlewareManager struct {
	middlewares []core.Middleware
	mutex       sync.RWMutex
	enabled     map[string]bool
	configs     map[string]interface{}
}

// NewMiddlewareManager 创建中间件管理器
func NewMiddlewareManager() *MiddlewareManager {
	return &MiddlewareManager{
		middlewares: make([]core.Middleware, 0),
		enabled:     make(map[string]bool),
		configs:     make(map[string]interface{}),
	}
}

// Register 注册中间件
func (m *MiddlewareManager) Register(middleware core.Middleware) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.middlewares = append(m.middlewares, middleware)
	m.enabled[middleware.Name()] = true

	// 按优先级排序
	m.sortMiddlewares()
}

// RegisterWithConfig 注册带配置的中间件
func (m *MiddlewareManager) RegisterWithConfig(middleware core.Middleware, config interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.middlewares = append(m.middlewares, middleware)
	m.enabled[middleware.Name()] = true
	m.configs[middleware.Name()] = config

	// 按优先级排序
	m.sortMiddlewares()
}

// Unregister 注销中间件
func (m *MiddlewareManager) Unregister(name string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, middleware := range m.middlewares {
		if middleware.Name() == name {
			// 删除中间件
			m.middlewares = append(m.middlewares[:i], m.middlewares[i+1:]...)
			delete(m.enabled, name)
			delete(m.configs, name)
			return true
		}
	}
	return false
}

// Enable 启用中间件
func (m *MiddlewareManager) Enable(name string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.enabled[name]; exists {
		m.enabled[name] = true
		return true
	}
	return false
}

// Disable 禁用中间件
func (m *MiddlewareManager) Disable(name string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.enabled[name]; exists {
		m.enabled[name] = false
		return true
	}
	return false
}

// IsEnabled 检查中间件是否启用
func (m *MiddlewareManager) IsEnabled(name string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.enabled[name]
}

// GetConfig 获取中间件配置
func (m *MiddlewareManager) GetConfig(name string) (interface{}, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	config, exists := m.configs[name]
	return config, exists
}

// SetConfig 设置中间件配置
func (m *MiddlewareManager) SetConfig(name string, config interface{}) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.enabled[name]; exists {
		m.configs[name] = config
		return true
	}
	return false
}

// GetMiddleware 获取中间件
func (m *MiddlewareManager) GetMiddleware(name string) (core.Middleware, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, middleware := range m.middlewares {
		if middleware.Name() == name {
			return middleware, true
		}
	}
	return nil, false
}

// GetAllMiddlewares 获取所有中间件
func (m *MiddlewareManager) GetAllMiddlewares() []core.Middleware {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]core.Middleware, 0)
	for _, middleware := range m.middlewares {
		if m.enabled[middleware.Name()] {
			result = append(result, middleware)
		}
	}
	return result
}

// GetEnabledMiddlewares 获取启用的中间件
func (m *MiddlewareManager) GetEnabledMiddlewares() []core.Middleware {
	return m.GetAllMiddlewares()
}

// Process 处理中间件链
func (m *MiddlewareManager) Process(ctx core.Context, handler core.Handler) error {
	middlewares := m.GetEnabledMiddlewares()
	if len(middlewares) == 0 {
		return handler(ctx)
	}

	chain := core.NewMiddlewareChain(middlewares)
	return chain.Execute(ctx)
}

// sortMiddlewares 按优先级排序中间件
func (m *MiddlewareManager) sortMiddlewares() {
	sort.Slice(m.middlewares, func(i, j int) bool {
		return m.middlewares[i].Priority() > m.middlewares[j].Priority()
	})
}

// GetStats 获取中间件统计信息
func (m *MiddlewareManager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_middlewares"] = len(m.middlewares)
	stats["enabled_middlewares"] = len(m.GetEnabledMiddlewares())

	middlewareList := make([]map[string]interface{}, 0)
	for _, middleware := range m.middlewares {
		info := map[string]interface{}{
			"name":     middleware.Name(),
			"priority": middleware.Priority(),
			"enabled":  m.enabled[middleware.Name()],
		}

		// 添加特定中间件的统计信息
		switch mw := middleware.(type) {
		case *MetricsMiddleware:
			info["metrics"] = mw.GetMetrics()
		case *AdvancedRateLimitMiddleware:
			info["algorithm"] = mw.GetAlgorithm().String()
		case *CacheMiddleware:
			info["cache_stats"] = mw.GetStats()
		}

		middlewareList = append(middlewareList, info)
	}

	stats["middlewares"] = middlewareList
	return stats
}

// Clear 清空所有中间件
func (m *MiddlewareManager) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.middlewares = make([]core.Middleware, 0)
	m.enabled = make(map[string]bool)
	m.configs = make(map[string]interface{})
}

// Count 获取中间件数量
func (m *MiddlewareManager) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.middlewares)
}

// EnabledCount 获取启用的中间件数量
func (m *MiddlewareManager) EnabledCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	count := 0
	for _, enabled := range m.enabled {
		if enabled {
			count++
		}
	}
	return count
}

// ListNames 列出所有中间件名称
func (m *MiddlewareManager) ListNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.middlewares))
	for _, middleware := range m.middlewares {
		names = append(names, middleware.Name())
	}
	return names
}

// ListEnabledNames 列出启用的中间件名称
func (m *MiddlewareManager) ListEnabledNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0)
	for name, enabled := range m.enabled {
		if enabled {
			names = append(names, name)
		}
	}
	return names
}

// PresetConfigurations 预设配置
type PresetConfigurations struct {
	manager *MiddlewareManager
}

// NewPresetConfigurations 创建预设配置
func NewPresetConfigurations(manager *MiddlewareManager) *PresetConfigurations {
	return &PresetConfigurations{
		manager: manager,
	}
}

// WebAPI 配置Web API中间件
func (p *PresetConfigurations) WebAPI() {
	// 监控中间件
	metricsMiddleware := NewMetricsMiddleware()
	p.manager.Register(metricsMiddleware)

	// JWT认证中间件
	jwtConfig := DefaultJWTConfig()
	jwtMiddleware := NewJWTMiddleware(jwtConfig)
	p.manager.RegisterWithConfig(jwtMiddleware, jwtConfig)

	// 限流中间件
	rateLimitConfig := DefaultRateLimitConfig()
	rateLimitMiddleware := NewAdvancedRateLimitMiddleware(rateLimitConfig)
	p.manager.RegisterWithConfig(rateLimitMiddleware, rateLimitConfig)

	// 缓存中间件
	cacheConfig := DefaultCacheConfig()
	cacheMiddleware := NewCacheMiddleware(cacheConfig)
	p.manager.RegisterWithConfig(cacheMiddleware, cacheConfig)
}

// GameServer 配置游戏服务器中间件
func (p *PresetConfigurations) GameServer() {
	// 监控中间件
	metricsMiddleware := NewMetricsMiddleware()
	p.manager.Register(metricsMiddleware)

	// 限流中间件（更宽松的配置）
	rateLimitConfig := &RateLimitConfig{
		Algorithm:  TokenBucket,
		Limit:      1000,
		Window:     time.Minute,
		RefillRate: 100,
		KeyGenerator: func(ctx core.Context) string {
			return ctx.Connection().RemoteAddr().String()
		},
	}
	rateLimitMiddleware := NewAdvancedRateLimitMiddleware(rateLimitConfig)
	p.manager.RegisterWithConfig(rateLimitMiddleware, rateLimitConfig)
}

// Microservice 配置微服务中间件
func (p *PresetConfigurations) Microservice() {
	// 监控中间件
	metricsMiddleware := NewMetricsMiddleware()
	p.manager.Register(metricsMiddleware)

	// JWT认证中间件
	jwtConfig := &JWTConfig{
		SecretKey:     "microservice-secret",
		Issuer:        "microservice",
		Expiration:    2 * time.Hour,
		SkipPaths:     []string{"/health", "/metrics", "/ready"},
		TokenLookup:   "header:Authorization",
		TokenHeadName: "Bearer",
	}
	jwtMiddleware := NewJWTMiddleware(jwtConfig)
	p.manager.RegisterWithConfig(jwtMiddleware, jwtConfig)

	// 限流中间件
	rateLimitConfig := &RateLimitConfig{
		Algorithm:  SlidingWindow,
		Limit:      500,
		Window:     time.Minute,
		KeyGenerator: func(ctx core.Context) string {
			// 使用服务名和客户端IP
			serviceName := "unknown"
			if svc, exists := ctx.Get("service_name"); exists {
				if svcStr, ok := svc.(string); ok {
					serviceName = svcStr
				}
			}
			return fmt.Sprintf("%s:%s", serviceName, ctx.Connection().RemoteAddr().String())
		},
	}
	rateLimitMiddleware := NewAdvancedRateLimitMiddleware(rateLimitConfig)
	p.manager.RegisterWithConfig(rateLimitMiddleware, rateLimitConfig)

	// 缓存中间件
	cacheConfig := &CacheConfig{
		TTL:       10 * time.Minute,
		MaxSize:   5000,
		KeyPrefix: "ms:",
		SkipPaths: []string{"/health", "/metrics", "/ready"},
		CacheStore: NewMemoryCache(5000, 10*time.Minute),
	}
	cacheMiddleware := NewCacheMiddleware(cacheConfig)
	p.manager.RegisterWithConfig(cacheMiddleware, cacheConfig)
}

// Development 配置开发环境中间件
func (p *PresetConfigurations) Development() {
	// 监控中间件
	metricsMiddleware := NewMetricsMiddleware()
	p.manager.Register(metricsMiddleware)

	// 宽松的限流配置
	rateLimitConfig := &RateLimitConfig{
		Algorithm:  TokenBucket,
		Limit:      10000,
		Window:     time.Minute,
		RefillRate: 1000,
		KeyGenerator: func(ctx core.Context) string {
			return "dev"
		},
	}
	rateLimitMiddleware := NewAdvancedRateLimitMiddleware(rateLimitConfig)
	p.manager.RegisterWithConfig(rateLimitMiddleware, rateLimitConfig)
}

// Production 配置生产环境中间件
func (p *PresetConfigurations) Production() {
	// 监控中间件
	metricsMiddleware := NewMetricsMiddleware()
	p.manager.Register(metricsMiddleware)

	// JWT认证中间件
	jwtConfig := &JWTConfig{
		SecretKey:     "production-secret-key-change-me",
		Issuer:        "production",
		Expiration:    time.Hour,
		SkipPaths:     []string{"/health"},
		TokenLookup:   "header:Authorization",
		TokenHeadName: "Bearer",
	}
	jwtMiddleware := NewJWTMiddleware(jwtConfig)
	p.manager.RegisterWithConfig(jwtMiddleware, jwtConfig)

	// 严格的限流配置
	rateLimitConfig := &RateLimitConfig{
		Algorithm:  SlidingWindow,
		Limit:      100,
		Window:     time.Minute,
		KeyGenerator: func(ctx core.Context) string {
			return ctx.Connection().RemoteAddr().String()
		},
	}
	rateLimitMiddleware := NewAdvancedRateLimitMiddleware(rateLimitConfig)
	p.manager.RegisterWithConfig(rateLimitMiddleware, rateLimitConfig)

	// 缓存中间件
	cacheConfig := &CacheConfig{
		TTL:       5 * time.Minute,
		MaxSize:   10000,
		KeyPrefix: "prod:",
		SkipPaths: []string{"/health"},
		CacheStore: NewMemoryCache(10000, 5*time.Minute),
	}
	cacheMiddleware := NewCacheMiddleware(cacheConfig)
	p.manager.RegisterWithConfig(cacheMiddleware, cacheConfig)
}


