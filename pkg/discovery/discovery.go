package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceInstance 服务实例
type ServiceInstance struct {
	ID       string            `json:"id"`       // 实例ID
	Name     string            `json:"name"`     // 服务名称
	Address  string            `json:"address"`  // 服务地址
	Port     int               `json:"port"`     // 服务端口
	Tags     []string          `json:"tags"`     // 服务标签
	Meta     map[string]string `json:"meta"`     // 元数据
	Health   HealthStatus      `json:"health"`   // 健康状态
	Weight   int               `json:"weight"`   // 权重
	Version  string            `json:"version"`  // 版本
	Region   string            `json:"region"`   // 区域
	Zone     string            `json:"zone"`     // 可用区
	Protocol string            `json:"protocol"` // 协议
}

// HealthStatus 健康状态
type HealthStatus string

const (
	Healthy   HealthStatus = "healthy"
	Unhealthy HealthStatus = "unhealthy"
	Unknown   HealthStatus = "unknown"
)

// GetEndpoint 获取服务端点
func (si *ServiceInstance) GetEndpoint() string {
	if si.Protocol != "" {
		return fmt.Sprintf("%s://%s:%d", si.Protocol, si.Address, si.Port)
	}
	return fmt.Sprintf("%s:%d", si.Address, si.Port)
}

// IsHealthy 检查是否健康
func (si *ServiceInstance) IsHealthy() bool {
	return si.Health == Healthy
}

// HasTag 检查是否包含标签
func (si *ServiceInstance) HasTag(tag string) bool {
	for _, t := range si.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// ServiceEvent 服务事件
type ServiceEvent struct {
	Type     EventType        `json:"type"`
	Service  string           `json:"service"`
	Instance *ServiceInstance `json:"instance"`
	Time     time.Time        `json:"time"`
}

// EventType 事件类型
type EventType string

const (
	EventServiceRegistered   EventType = "service_registered"
	EventServiceDeregistered EventType = "service_deregistered"
	EventServiceUpdated      EventType = "service_updated"
	EventHealthChanged       EventType = "health_changed"
)

// ServiceWatcher 服务监听器
type ServiceWatcher interface {
	Watch(ctx context.Context, serviceName string) (<-chan *ServiceEvent, error)
	Stop()
}

// ServiceRegistry 服务注册接口
type ServiceRegistry interface {
	// Register 注册服务实例
	Register(ctx context.Context, instance *ServiceInstance) error
	
	// Deregister 注销服务实例
	Deregister(ctx context.Context, instanceID string) error
	
	// Update 更新服务实例
	Update(ctx context.Context, instance *ServiceInstance) error
	
	// SetHealth 设置健康状态
	SetHealth(ctx context.Context, instanceID string, health HealthStatus) error
}

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	// GetService 获取服务实例
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	
	// GetHealthyServices 获取健康的服务实例
	GetHealthyServices(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	
	// GetServicesByTag 根据标签获取服务实例
	GetServicesByTag(ctx context.Context, serviceName, tag string) ([]*ServiceInstance, error)
	
	// Watch 监听服务变化
	Watch(ctx context.Context, serviceName string) (ServiceWatcher, error)
	
	// ListServices 列出所有服务
	ListServices(ctx context.Context) ([]string, error)
}

// ServiceClient 服务客户端（组合注册和发现）
type ServiceClient interface {
	ServiceRegistry
	ServiceDiscovery
	
	// Close 关闭客户端
	Close() error
}

// RegistrationOptions 注册选项
type RegistrationOptions struct {
	TTL           time.Duration     // 生存时间
	HealthCheck   *HealthCheck      // 健康检查
	Tags          []string          // 标签
	Meta          map[string]string // 元数据
	Weight        int               // 权重
	EnableTagging bool              // 启用标签
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Type     HealthCheckType   `json:"type"`     // 检查类型
	URL      string            `json:"url"`      // HTTP检查URL
	Interval time.Duration     `json:"interval"` // 检查间隔
	Timeout  time.Duration     `json:"timeout"`  // 超时时间
	Script   string            `json:"script"`   // 脚本检查
	TCP      string            `json:"tcp"`      // TCP检查地址
	Headers  map[string]string `json:"headers"`  // HTTP头
}

// HealthCheckType 健康检查类型
type HealthCheckType string

const (
	HealthCheckHTTP   HealthCheckType = "http"
	HealthCheckTCP    HealthCheckType = "tcp"
	HealthCheckScript HealthCheckType = "script"
	HealthCheckTTL    HealthCheckType = "ttl"
)

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	Provider    string            `json:"provider"`    // 提供者（consul, etcd, zookeeper等）
	Endpoints   []string          `json:"endpoints"`   // 端点列表
	Timeout     time.Duration     `json:"timeout"`     // 超时时间
	RetryCount  int               `json:"retry_count"` // 重试次数
	Namespace   string            `json:"namespace"`   // 命名空间
	Credentials map[string]string `json:"credentials"` // 认证信息
	TLS         *TLSConfig        `json:"tls"`         // TLS配置
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
	CAFile     string `json:"ca_file"`
	SkipVerify bool   `json:"skip_verify"`
}

// ServiceFilter 服务过滤器
type ServiceFilter func(*ServiceInstance) bool

// FilterByTag 按标签过滤
func FilterByTag(tag string) ServiceFilter {
	return func(instance *ServiceInstance) bool {
		return instance.HasTag(tag)
	}
}

// FilterByHealth 按健康状态过滤
func FilterByHealth(healthy bool) ServiceFilter {
	return func(instance *ServiceInstance) bool {
		if healthy {
			return instance.IsHealthy()
		}
		return !instance.IsHealthy()
	}
}

// FilterByRegion 按区域过滤
func FilterByRegion(region string) ServiceFilter {
	return func(instance *ServiceInstance) bool {
		return instance.Region == region
	}
}

// FilterByZone 按可用区过滤
func FilterByZone(zone string) ServiceFilter {
	return func(instance *ServiceInstance) bool {
		return instance.Zone == zone
	}
}

// FilterByVersion 按版本过滤
func FilterByVersion(version string) ServiceFilter {
	return func(instance *ServiceInstance) bool {
		return instance.Version == version
	}
}

// ApplyFilters 应用过滤器
func ApplyFilters(instances []*ServiceInstance, filters ...ServiceFilter) []*ServiceInstance {
	if len(filters) == 0 {
		return instances
	}
	
	var result []*ServiceInstance
	for _, instance := range instances {
		match := true
		for _, filter := range filters {
			if !filter(instance) {
				match = false
				break
			}
		}
		if match {
			result = append(result, instance)
		}
	}
	return result
}

// ServiceCache 服务缓存
type ServiceCache struct {
	mu       sync.RWMutex
	services map[string][]*ServiceInstance
	ttl      time.Duration
	updated  map[string]time.Time
}

// NewServiceCache 创建服务缓存
func NewServiceCache(ttl time.Duration) *ServiceCache {
	return &ServiceCache{
		services: make(map[string][]*ServiceInstance),
		ttl:      ttl,
		updated:  make(map[string]time.Time),
	}
}

// Get 获取缓存的服务
func (c *ServiceCache) Get(serviceName string) ([]*ServiceInstance, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	instances, exists := c.services[serviceName]
	if !exists {
		return nil, false
	}
	
	// 检查是否过期
	if updated, ok := c.updated[serviceName]; ok {
		if time.Since(updated) > c.ttl {
			return nil, false
		}
	}
	
	// 复制实例以避免并发修改
	result := make([]*ServiceInstance, len(instances))
	copy(result, instances)
	return result, true
}

// Set 设置缓存的服务
func (c *ServiceCache) Set(serviceName string, instances []*ServiceInstance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 复制实例以避免外部修改
	cached := make([]*ServiceInstance, len(instances))
	copy(cached, instances)
	
	c.services[serviceName] = cached
	c.updated[serviceName] = time.Now()
}

// Delete 删除缓存的服务
func (c *ServiceCache) Delete(serviceName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.services, serviceName)
	delete(c.updated, serviceName)
}

// Clear 清空缓存
func (c *ServiceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.services = make(map[string][]*ServiceInstance)
	c.updated = make(map[string]time.Time)
}

// Cleanup 清理过期缓存
func (c *ServiceCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for serviceName, updated := range c.updated {
		if now.Sub(updated) > c.ttl {
			delete(c.services, serviceName)
			delete(c.updated, serviceName)
		}
	}
}

// StartCleanupTimer 启动清理定时器
func (c *ServiceCache) StartCleanupTimer(interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.Cleanup()
		}
	}()
	return ticker
}

// ServiceManager 服务管理器
type ServiceManager struct {
	mu        sync.RWMutex
	client    ServiceClient
	cache     *ServiceCache
	watchers  map[string]ServiceWatcher
	instances map[string]*ServiceInstance // 本地注册的实例
}

// NewServiceManager 创建服务管理器
func NewServiceManager(client ServiceClient, cacheTimeout time.Duration) *ServiceManager {
	return &ServiceManager{
		client:    client,
		cache:     NewServiceCache(cacheTimeout),
		watchers:  make(map[string]ServiceWatcher),
		instances: make(map[string]*ServiceInstance),
	}
}

// RegisterService 注册服务
func (sm *ServiceManager) RegisterService(ctx context.Context, instance *ServiceInstance) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if err := sm.client.Register(ctx, instance); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}
	
	// 缓存本地注册的实例
	sm.instances[instance.ID] = instance
	return nil
}

// DeregisterService 注销服务
func (sm *ServiceManager) DeregisterService(ctx context.Context, instanceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if err := sm.client.Deregister(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}
	
	// 从本地缓存中移除
	delete(sm.instances, instanceID)
	return nil
}

// DiscoverService 发现服务
func (sm *ServiceManager) DiscoverService(ctx context.Context, serviceName string, useCache bool) ([]*ServiceInstance, error) {
	// 尝试从缓存获取
	if useCache {
		if instances, found := sm.cache.Get(serviceName); found {
			return instances, nil
		}
	}
	
	// 从服务发现获取
	instances, err := sm.client.GetHealthyServices(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service: %w", err)
	}
	
	// 更新缓存
	if useCache {
		sm.cache.Set(serviceName, instances)
	}
	
	return instances, nil
}

// WatchService 监听服务变化
func (sm *ServiceManager) WatchService(ctx context.Context, serviceName string) (<-chan *ServiceEvent, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 检查是否已经在监听
	if watcher, exists := sm.watchers[serviceName]; exists {
		return watcher.Watch(ctx, serviceName)
	}
	
	// 创建新的监听器
	watcher, err := sm.client.Watch(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to watch service: %w", err)
	}
	
	sm.watchers[serviceName] = watcher
	return watcher.Watch(ctx, serviceName)
}

// StopWatching 停止监听服务
func (sm *ServiceManager) StopWatching(serviceName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if watcher, exists := sm.watchers[serviceName]; exists {
		watcher.Stop()
		delete(sm.watchers, serviceName)
	}
}

// GetLocalInstances 获取本地注册的实例
func (sm *ServiceManager) GetLocalInstances() []*ServiceInstance {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	instances := make([]*ServiceInstance, 0, len(sm.instances))
	for _, instance := range sm.instances {
		instances = append(instances, instance)
	}
	return instances
}

// Close 关闭服务管理器
func (sm *ServiceManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 停止所有监听器
	for _, watcher := range sm.watchers {
		watcher.Stop()
	}
	sm.watchers = make(map[string]ServiceWatcher)
	
	// 注销所有本地实例
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	for instanceID := range sm.instances {
		sm.client.Deregister(ctx, instanceID)
	}
	sm.instances = make(map[string]*ServiceInstance)
	
	// 关闭客户端
	return sm.client.Close()
}