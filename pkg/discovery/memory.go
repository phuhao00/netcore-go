package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryRegistry 内存服务注册中心
type MemoryRegistry struct {
	mu        sync.RWMutex
	services  map[string]map[string]*ServiceInstance // serviceName -> instanceID -> instance
	watchers  map[string][]chan *ServiceEvent        // serviceName -> watchers
	ttl       map[string]time.Time                   // instanceID -> expiry time
	cleanup   *time.Ticker
	closed    bool
}

// NewMemoryRegistry 创建内存服务注册中心
func NewMemoryRegistry() *MemoryRegistry {
	r := &MemoryRegistry{
		services: make(map[string]map[string]*ServiceInstance),
		watchers: make(map[string][]chan *ServiceEvent),
		ttl:      make(map[string]time.Time),
	}
	
	// 启动清理定时器
	r.startCleanup()
	
	return r
}

// Register 注册服务实例
func (r *MemoryRegistry) Register(ctx context.Context, instance *ServiceInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return fmt.Errorf("registry is closed")
	}
	
	// 初始化服务映射
	if r.services[instance.Name] == nil {
		r.services[instance.Name] = make(map[string]*ServiceInstance)
	}
	
	// 检查实例是否已存在
	existingInstance := r.services[instance.Name][instance.ID]
	
	// 复制实例以避免外部修改
	newInstance := *instance
	r.services[instance.Name][instance.ID] = &newInstance
	
	// 设置TTL
	if instance.Meta != nil {
		if ttlStr, exists := instance.Meta["ttl"]; exists {
			if ttl, err := time.ParseDuration(ttlStr); err == nil {
				r.ttl[instance.ID] = time.Now().Add(ttl)
			}
		}
	}
	
	// 发送事件
	eventType := EventServiceRegistered
	if existingInstance != nil {
		eventType = EventServiceUpdated
	}
	
	r.notifyWatchers(instance.Name, &ServiceEvent{
		Type:     eventType,
		Service:  instance.Name,
		Instance: &newInstance,
		Time:     time.Now(),
	})
	
	return nil
}

// Deregister 注销服务实例
func (r *MemoryRegistry) Deregister(ctx context.Context, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return fmt.Errorf("registry is closed")
	}
	
	// 查找并删除实例
	var removedInstance *ServiceInstance
	var serviceName string
	
	for svcName, instances := range r.services {
		if instance, exists := instances[instanceID]; exists {
			removedInstance = instance
			serviceName = svcName
			delete(instances, instanceID)
			
			// 如果服务没有实例了，删除服务
			if len(instances) == 0 {
				delete(r.services, svcName)
			}
			break
		}
	}
	
	if removedInstance == nil {
		return fmt.Errorf("instance %s not found", instanceID)
	}
	
	// 删除TTL
	delete(r.ttl, instanceID)
	
	// 发送事件
	r.notifyWatchers(serviceName, &ServiceEvent{
		Type:     EventServiceDeregistered,
		Service:  serviceName,
		Instance: removedInstance,
		Time:     time.Now(),
	})
	
	return nil
}

// Update 更新服务实例
func (r *MemoryRegistry) Update(ctx context.Context, instance *ServiceInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return fmt.Errorf("registry is closed")
	}
	
	// 检查实例是否存在
	if r.services[instance.Name] == nil || r.services[instance.Name][instance.ID] == nil {
		return fmt.Errorf("instance %s not found", instance.ID)
	}
	
	// 更新实例
	newInstance := *instance
	r.services[instance.Name][instance.ID] = &newInstance
	
	// 发送事件
	r.notifyWatchers(instance.Name, &ServiceEvent{
		Type:     EventServiceUpdated,
		Service:  instance.Name,
		Instance: &newInstance,
		Time:     time.Now(),
	})
	
	return nil
}

// SetHealth 设置健康状态
func (r *MemoryRegistry) SetHealth(ctx context.Context, instanceID string, health HealthStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return fmt.Errorf("registry is closed")
	}
	
	// 查找实例
	var instance *ServiceInstance
	var serviceName string
	
	for svcName, instances := range r.services {
		if inst, exists := instances[instanceID]; exists {
			instance = inst
			serviceName = svcName
			break
		}
	}
	
	if instance == nil {
		return fmt.Errorf("instance %s not found", instanceID)
	}
	
	// 更新健康状态
	oldHealth := instance.Health
	instance.Health = health
	
	// 如果健康状态发生变化，发送事件
	if oldHealth != health {
		r.notifyWatchers(serviceName, &ServiceEvent{
			Type:     EventHealthChanged,
			Service:  serviceName,
			Instance: instance,
			Time:     time.Now(),
		})
	}
	
	return nil
}

// GetService 获取服务实例
func (r *MemoryRegistry) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.closed {
		return nil, fmt.Errorf("registry is closed")
	}
	
	instances, exists := r.services[serviceName]
	if !exists {
		return []*ServiceInstance{}, nil
	}
	
	// 复制实例列表
	result := make([]*ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		// 复制实例以避免外部修改
		copy := *instance
		result = append(result, &copy)
	}
	
	return result, nil
}

// GetHealthyServices 获取健康的服务实例
func (r *MemoryRegistry) GetHealthyServices(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	instances, err := r.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	
	// 过滤健康实例
	var healthy []*ServiceInstance
	for _, instance := range instances {
		if instance.IsHealthy() {
			healthy = append(healthy, instance)
		}
	}
	
	return healthy, nil
}

// GetServicesByTag 根据标签获取服务实例
func (r *MemoryRegistry) GetServicesByTag(ctx context.Context, serviceName, tag string) ([]*ServiceInstance, error) {
	instances, err := r.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	
	// 过滤包含指定标签的实例
	var tagged []*ServiceInstance
	for _, instance := range instances {
		if instance.HasTag(tag) {
			tagged = append(tagged, instance)
		}
	}
	
	return tagged, nil
}

// Watch 监听服务变化
func (r *MemoryRegistry) Watch(ctx context.Context, serviceName string) (ServiceWatcher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return nil, fmt.Errorf("registry is closed")
	}
	
	// 创建事件通道
	eventChan := make(chan *ServiceEvent, 100)
	
	// 添加到监听器列表
	r.watchers[serviceName] = append(r.watchers[serviceName], eventChan)
	
	// 创建监听器
	watcher := &memoryWatcher{
		registry:    r,
		serviceName: serviceName,
		eventChan:   eventChan,
		stopChan:    make(chan struct{}),
	}
	
	return watcher, nil
}

// ListServices 列出所有服务
func (r *MemoryRegistry) ListServices(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.closed {
		return nil, fmt.Errorf("registry is closed")
	}
	
	services := make([]string, 0, len(r.services))
	for serviceName := range r.services {
		services = append(services, serviceName)
	}
	
	return services, nil
}

// Close 关闭注册中心
func (r *MemoryRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return nil
	}
	
	r.closed = true
	
	// 停止清理定时器
	if r.cleanup != nil {
		r.cleanup.Stop()
	}
	
	// 关闭所有监听器
	for _, watchers := range r.watchers {
		for _, watcher := range watchers {
			close(watcher)
		}
	}
	
	// 清空数据
	r.services = make(map[string]map[string]*ServiceInstance)
	r.watchers = make(map[string][]chan *ServiceEvent)
	r.ttl = make(map[string]time.Time)
	
	return nil
}

// notifyWatchers 通知监听器
func (r *MemoryRegistry) notifyWatchers(serviceName string, event *ServiceEvent) {
	watchers := r.watchers[serviceName]
	for i, watcher := range watchers {
		select {
		case watcher <- event:
			// 事件发送成功
		default:
			// 通道已满，移除这个监听器
			close(watcher)
			r.watchers[serviceName] = append(watchers[:i], watchers[i+1:]...)
		}
	}
}

// startCleanup 启动清理定时器
func (r *MemoryRegistry) startCleanup() {
	r.cleanup = time.NewTicker(30 * time.Second)
	go func() {
		for range r.cleanup.C {
			r.cleanupExpiredInstances()
		}
	}()
}

// cleanupExpiredInstances 清理过期实例
func (r *MemoryRegistry) cleanupExpiredInstances() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return
	}
	
	now := time.Now()
	var expiredInstances []string
	
	// 查找过期实例
	for instanceID, expiry := range r.ttl {
		if now.After(expiry) {
			expiredInstances = append(expiredInstances, instanceID)
		}
	}
	
	// 删除过期实例
	for _, instanceID := range expiredInstances {
		for serviceName, instances := range r.services {
			if instance, exists := instances[instanceID]; exists {
				delete(instances, instanceID)
				delete(r.ttl, instanceID)
				
				// 如果服务没有实例了，删除服务
				if len(instances) == 0 {
					delete(r.services, serviceName)
				}
				
				// 发送事件
				r.notifyWatchers(serviceName, &ServiceEvent{
					Type:     EventServiceDeregistered,
					Service:  serviceName,
					Instance: instance,
					Time:     time.Now(),
				})
				break
			}
		}
	}
}

// memoryWatcher 内存监听器
type memoryWatcher struct {
	registry    *MemoryRegistry
	serviceName string
	eventChan   chan *ServiceEvent
	stopChan    chan struct{}
	stopped     bool
	mu          sync.Mutex
}

// Watch 监听服务变化
func (w *memoryWatcher) Watch(ctx context.Context, serviceName string) (<-chan *ServiceEvent, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.stopped {
		return nil, fmt.Errorf("watcher is stopped")
	}
	
	// 启动监听协程
	go func() {
		for {
			select {
			case <-ctx.Done():
				w.Stop()
				return
			case <-w.stopChan:
				return
			}
		}
	}()
	
	return w.eventChan, nil
}

// Stop 停止监听
func (w *memoryWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.stopped {
		return
	}
	
	w.stopped = true
	close(w.stopChan)
	
	// 从注册中心移除监听器
	w.registry.mu.Lock()
	watchers := w.registry.watchers[w.serviceName]
	for i, watcher := range watchers {
		if watcher == w.eventChan {
			w.registry.watchers[w.serviceName] = append(watchers[:i], watchers[i+1:]...)
			break
		}
	}
	w.registry.mu.Unlock()
}

// MemoryClient 内存服务客户端
type MemoryClient struct {
	*MemoryRegistry
}

// NewMemoryClient 创建内存服务客户端
func NewMemoryClient() *MemoryClient {
	return &MemoryClient{
		MemoryRegistry: NewMemoryRegistry(),
	}
}

// HealthChecker 健康检查器
type HealthChecker struct {
	mu       sync.RWMutex
	registry ServiceRegistry
	checks   map[string]*HealthCheck
	ticker   *time.Ticker
	stopped  bool
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(registry ServiceRegistry, interval time.Duration) *HealthChecker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	
	hc := &HealthChecker{
		registry: registry,
		checks:   make(map[string]*HealthCheck),
		ticker:   time.NewTicker(interval),
	}
	
	// 启动健康检查
	go hc.run()
	
	return hc
}

// AddCheck 添加健康检查
func (hc *HealthChecker) AddCheck(instanceID string, check *HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.checks[instanceID] = check
}

// RemoveCheck 移除健康检查
func (hc *HealthChecker) RemoveCheck(instanceID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	delete(hc.checks, instanceID)
}

// run 运行健康检查
func (hc *HealthChecker) run() {
	for range hc.ticker.C {
		hc.mu.RLock()
		if hc.stopped {
			hc.mu.RUnlock()
			return
		}
		
		// 复制检查列表以避免长时间持锁
		checks := make(map[string]*HealthCheck)
		for id, check := range hc.checks {
			checks[id] = check
		}
		hc.mu.RUnlock()
		
		// 执行健康检查
		for instanceID, check := range checks {
			go hc.performCheck(instanceID, check)
		}
	}
}

// performCheck 执行健康检查
func (hc *HealthChecker) performCheck(instanceID string, check *HealthCheck) {
	ctx, cancel := context.WithTimeout(context.Background(), check.Timeout)
	defer cancel()
	
	healthy := hc.checkHealth(ctx, check)
	
	// 更新健康状态
	status := Unhealthy
	if healthy {
		status = Healthy
	}
	
	hc.registry.SetHealth(ctx, instanceID, status)
}

// checkHealth 检查健康状态
func (hc *HealthChecker) checkHealth(ctx context.Context, check *HealthCheck) bool {
	switch check.Type {
	case HealthCheckHTTP:
		return hc.checkHTTP(ctx, check)
	case HealthCheckTCP:
		return hc.checkTCP(ctx, check)
	case HealthCheckScript:
		return hc.checkScript(ctx, check)
	case HealthCheckTTL:
		return true // TTL检查由注册中心处理
	default:
		return false
	}
}

// checkHTTP HTTP健康检查
func (hc *HealthChecker) checkHTTP(ctx context.Context, check *HealthCheck) bool {
	// 这里应该实现HTTP健康检查逻辑
	// 简化实现，实际应该发送HTTP请求
	return true
}

// checkTCP TCP健康检查
func (hc *HealthChecker) checkTCP(ctx context.Context, check *HealthCheck) bool {
	// 这里应该实现TCP健康检查逻辑
	// 简化实现，实际应该尝试TCP连接
	return true
}

// checkScript 脚本健康检查
func (hc *HealthChecker) checkScript(ctx context.Context, check *HealthCheck) bool {
	// 这里应该实现脚本健康检查逻辑
	// 简化实现，实际应该执行脚本
	return true
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.stopped = true
	if hc.ticker != nil {
		hc.ticker.Stop()
	}
}