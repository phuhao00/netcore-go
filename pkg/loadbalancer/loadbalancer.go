package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/discovery"
)

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// Select 选择一个服务实例
	Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error)
	
	// Name 返回负载均衡器名称
	Name() string
	
	// UpdateStats 更新统计信息
	UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration)
}

// Algorithm 负载均衡算法
type Algorithm string

const (
	RoundRobin          Algorithm = "round_robin"
	WeightedRoundRobin  Algorithm = "weighted_round_robin"
	Random              Algorithm = "random"
	WeightedRandom      Algorithm = "weighted_random"
	LeastConnections    Algorithm = "least_connections"
	LeastResponseTime   Algorithm = "least_response_time"
	ConsistentHash      Algorithm = "consistent_hash"
	IPHash              Algorithm = "ip_hash"
	HealthyFirst        Algorithm = "healthy_first"
	GeographyAware      Algorithm = "geography_aware"
)

// Config 负载均衡器配置
type Config struct {
	Algorithm       Algorithm     `json:"algorithm"`
	HealthCheck     bool          `json:"health_check"`
	MaxRetries      int           `json:"max_retries"`
	RetryTimeout    time.Duration `json:"retry_timeout"`
	StatsWindow     time.Duration `json:"stats_window"`
	HashKey         string        `json:"hash_key"`         // 一致性哈希的键
	PreferredRegion string        `json:"preferred_region"` // 地理位置感知的首选区域
	PreferredZone   string        `json:"preferred_zone"`   // 地理位置感知的首选可用区
}

// Stats 实例统计信息
type Stats struct {
	mu              sync.RWMutex
	connections     int64         // 当前连接数
	totalRequests   int64         // 总请求数
	successRequests int64         // 成功请求数
	failedRequests  int64         // 失败请求数
	totalTime       time.Duration // 总响应时间
	avgResponseTime time.Duration // 平均响应时间
	lastUsed        time.Time     // 最后使用时间
	lastSuccess     time.Time     // 最后成功时间
	lastFailure     time.Time     // 最后失败时间
}

// UpdateRequest 更新请求统计
func (s *Stats) UpdateRequest(success bool, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.totalRequests++
	s.totalTime += duration
	s.lastUsed = time.Now()
	
	if success {
		s.successRequests++
		s.lastSuccess = time.Now()
	} else {
		s.failedRequests++
		s.lastFailure = time.Now()
	}
	
	// 计算平均响应时间
	if s.totalRequests > 0 {
		s.avgResponseTime = s.totalTime / time.Duration(s.totalRequests)
	}
}

// AddConnection 增加连接数
func (s *Stats) AddConnection() {
	atomic.AddInt64(&s.connections, 1)
}

// RemoveConnection 减少连接数
func (s *Stats) RemoveConnection() {
	atomic.AddInt64(&s.connections, -1)
}

// GetConnections 获取当前连接数
func (s *Stats) GetConnections() int64 {
	return atomic.LoadInt64(&s.connections)
}

// GetSuccessRate 获取成功率
func (s *Stats) GetSuccessRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.totalRequests == 0 {
		return 1.0
	}
	return float64(s.successRequests) / float64(s.totalRequests)
}

// GetAvgResponseTime 获取平均响应时间
func (s *Stats) GetAvgResponseTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.avgResponseTime
}

// Manager 负载均衡管理器
type Manager struct {
	mu           sync.RWMutex
	balancers    map[string]LoadBalancer
	stats        map[string]*Stats // 实例ID -> 统计信息
	config       *Config
	statsCleanup *time.Ticker
}

// NewManager 创建负载均衡管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Algorithm:    RoundRobin,
			HealthCheck:  true,
			MaxRetries:   3,
			RetryTimeout: 5 * time.Second,
			StatsWindow:  5 * time.Minute,
		}
	}
	
	m := &Manager{
		balancers: make(map[string]LoadBalancer),
		stats:     make(map[string]*Stats),
		config:    config,
	}
	
	// 启动统计信息清理
	m.startStatsCleanup()
	
	return m
}

// GetBalancer 获取负载均衡器
func (m *Manager) GetBalancer(algorithm Algorithm) LoadBalancer {
	m.mu.RLock()
	if lb, exists := m.balancers[string(algorithm)]; exists {
		m.mu.RUnlock()
		return lb
	}
	m.mu.RUnlock()
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 双重检查
	if lb, exists := m.balancers[string(algorithm)]; exists {
		return lb
	}
	
	// 创建新的负载均衡器
	lb := m.createBalancer(algorithm)
	m.balancers[string(algorithm)] = lb
	return lb
}

// createBalancer 创建负载均衡器
func (m *Manager) createBalancer(algorithm Algorithm) LoadBalancer {
	switch algorithm {
	case RoundRobin:
		return NewRoundRobinBalancer()
	case WeightedRoundRobin:
		return NewWeightedRoundRobinBalancer()
	case Random:
		return NewRandomBalancer()
	case WeightedRandom:
		return NewWeightedRandomBalancer()
	case LeastConnections:
		return NewLeastConnectionsBalancer(m)
	case LeastResponseTime:
		return NewLeastResponseTimeBalancer(m)
	case ConsistentHash:
		return NewConsistentHashBalancer(m.config.HashKey)
	case IPHash:
		return NewIPHashBalancer()
	case HealthyFirst:
		return NewHealthyFirstBalancer(NewRoundRobinBalancer())
	case GeographyAware:
		return NewGeographyAwareBalancer(m.config.PreferredRegion, m.config.PreferredZone, NewRoundRobinBalancer())
	default:
		return NewRoundRobinBalancer()
	}
}

// Select 选择服务实例
func (m *Manager) Select(ctx context.Context, instances []*discovery.ServiceInstance, algorithm Algorithm) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no available instances")
	}
	
	// 过滤健康的实例
	if m.config.HealthCheck {
		instances = m.filterHealthyInstances(instances)
		if len(instances) == 0 {
			return nil, errors.New("no healthy instances available")
		}
	}
	
	lb := m.GetBalancer(algorithm)
	return lb.Select(ctx, instances)
}

// UpdateStats 更新统计信息
func (m *Manager) UpdateStats(instanceID string, success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	stats, exists := m.stats[instanceID]
	if !exists {
		stats = &Stats{}
		m.stats[instanceID] = stats
	}
	
	stats.UpdateRequest(success, duration)
}

// GetStats 获取统计信息
func (m *Manager) GetStats(instanceID string) *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.stats[instanceID]
}

// filterHealthyInstances 过滤健康实例
func (m *Manager) filterHealthyInstances(instances []*discovery.ServiceInstance) []*discovery.ServiceInstance {
	var healthy []*discovery.ServiceInstance
	for _, instance := range instances {
		if instance.IsHealthy() {
			healthy = append(healthy, instance)
		}
	}
	return healthy
}

// startStatsCleanup 启动统计信息清理
func (m *Manager) startStatsCleanup() {
	m.statsCleanup = time.NewTicker(m.config.StatsWindow)
	go func() {
		for range m.statsCleanup.C {
			m.cleanupStats()
		}
	}()
}

// cleanupStats 清理过期统计信息
func (m *Manager) cleanupStats() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for instanceID, stats := range m.stats {
		stats.mu.RLock()
		lastUsed := stats.lastUsed
		stats.mu.RUnlock()
		
		if now.Sub(lastUsed) > m.config.StatsWindow {
			delete(m.stats, instanceID)
		}
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	if m.statsCleanup != nil {
		m.statsCleanup.Stop()
	}
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	mu      sync.Mutex
	counter uint64
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Select 选择实例
func (rb *RoundRobinBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	rb.mu.Lock()
	index := rb.counter % uint64(len(instances))
	rb.counter++
	rb.mu.Unlock()
	
	return instances[index], nil
}

// Name 返回名称
func (rb *RoundRobinBalancer) Name() string {
	return "round_robin"
}

// UpdateStats 更新统计信息
func (rb *RoundRobinBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	// 轮询算法不需要统计信息
}

// WeightedRoundRobinBalancer 加权轮询负载均衡器
type WeightedRoundRobinBalancer struct {
	mu      sync.Mutex
	weights map[string]int // 实例ID -> 当前权重
}

// NewWeightedRoundRobinBalancer 创建加权轮询负载均衡器
func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		weights: make(map[string]int),
	}
}

// Select 选择实例
func (wrr *WeightedRoundRobinBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	wrr.mu.Lock()
	defer wrr.mu.Unlock()
	
	// 计算总权重
	totalWeight := 0
	for _, instance := range instances {
		weight := instance.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
		
		// 增加当前权重
		wrr.weights[instance.ID] += weight
	}
	
	// 找到权重最大的实例
	var selected *discovery.ServiceInstance
	maxWeight := -1
	for _, instance := range instances {
		if wrr.weights[instance.ID] > maxWeight {
			maxWeight = wrr.weights[instance.ID]
			selected = instance
		}
	}
	
	if selected != nil {
		// 减少选中实例的权重
		wrr.weights[selected.ID] -= totalWeight
	}
	
	return selected, nil
}

// Name 返回名称
func (wrr *WeightedRoundRobinBalancer) Name() string {
	return "weighted_round_robin"
}

// UpdateStats 更新统计信息
func (wrr *WeightedRoundRobinBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	// 加权轮询算法不需要统计信息
}

// RandomBalancer 随机负载均衡器
type RandomBalancer struct {
	rnd *rand.Rand
	mu  sync.Mutex
}

// NewRandomBalancer 创建随机负载均衡器
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select 选择实例
func (r *RandomBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	r.mu.Lock()
	index := r.rnd.Intn(len(instances))
	r.mu.Unlock()
	
	return instances[index], nil
}

// Name 返回名称
func (r *RandomBalancer) Name() string {
	return "random"
}

// UpdateStats 更新统计信息
func (r *RandomBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	// 随机算法不需要统计信息
}

// WeightedRandomBalancer 加权随机负载均衡器
type WeightedRandomBalancer struct {
	rnd *rand.Rand
	mu  sync.Mutex
}

// NewWeightedRandomBalancer 创建加权随机负载均衡器
func NewWeightedRandomBalancer() *WeightedRandomBalancer {
	return &WeightedRandomBalancer{
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select 选择实例
func (wr *WeightedRandomBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	// 计算总权重
	totalWeight := 0
	for _, instance := range instances {
		weight := instance.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}
	
	wr.mu.Lock()
	random := wr.rnd.Intn(totalWeight)
	wr.mu.Unlock()
	
	// 根据权重选择实例
	currentWeight := 0
	for _, instance := range instances {
		weight := instance.Weight
		if weight <= 0 {
			weight = 1
		}
		currentWeight += weight
		if random < currentWeight {
			return instance, nil
		}
	}
	
	return instances[len(instances)-1], nil
}

// Name 返回名称
func (wr *WeightedRandomBalancer) Name() string {
	return "weighted_random"
}

// UpdateStats 更新统计信息
func (wr *WeightedRandomBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	// 加权随机算法不需要统计信息
}

// LeastConnectionsBalancer 最少连接数负载均衡器
type LeastConnectionsBalancer struct {
	manager *Manager
}

// NewLeastConnectionsBalancer 创建最少连接数负载均衡器
func NewLeastConnectionsBalancer(manager *Manager) *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		manager: manager,
	}
}

// Select 选择实例
func (lc *LeastConnectionsBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	var selected *discovery.ServiceInstance
	minConnections := int64(-1)
	
	for _, instance := range instances {
		stats := lc.manager.GetStats(instance.ID)
		connections := int64(0)
		if stats != nil {
			connections = stats.GetConnections()
		}
		
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selected = instance
		}
	}
	
	return selected, nil
}

// Name 返回名称
func (lc *LeastConnectionsBalancer) Name() string {
	return "least_connections"
}

// UpdateStats 更新统计信息
func (lc *LeastConnectionsBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	lc.manager.UpdateStats(instance.ID, success, duration)
}

// LeastResponseTimeBalancer 最短响应时间负载均衡器
type LeastResponseTimeBalancer struct {
	manager *Manager
}

// NewLeastResponseTimeBalancer 创建最短响应时间负载均衡器
func NewLeastResponseTimeBalancer(manager *Manager) *LeastResponseTimeBalancer {
	return &LeastResponseTimeBalancer{
		manager: manager,
	}
}

// Select 选择实例
func (lrt *LeastResponseTimeBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	var selected *discovery.ServiceInstance
	minResponseTime := time.Duration(-1)
	
	for _, instance := range instances {
		stats := lrt.manager.GetStats(instance.ID)
		responseTime := time.Duration(0)
		if stats != nil {
			responseTime = stats.GetAvgResponseTime()
		}
		
		if minResponseTime == -1 || responseTime < minResponseTime {
			minResponseTime = responseTime
			selected = instance
		}
	}
	
	return selected, nil
}

// Name 返回名称
func (lrt *LeastResponseTimeBalancer) Name() string {
	return "least_response_time"
}

// UpdateStats 更新统计信息
func (lrt *LeastResponseTimeBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	lrt.manager.UpdateStats(instance.ID, success, duration)
}

// ConsistentHashBalancer 一致性哈希负载均衡器
type ConsistentHashBalancer struct {
	mu      sync.RWMutex
	hashKey string
	ring    map[uint32]*discovery.ServiceInstance
	keys    []uint32
}

// NewConsistentHashBalancer 创建一致性哈希负载均衡器
func NewConsistentHashBalancer(hashKey string) *ConsistentHashBalancer {
	return &ConsistentHashBalancer{
		hashKey: hashKey,
		ring:    make(map[uint32]*discovery.ServiceInstance),
	}
}

// Select 选择实例
func (ch *ConsistentHashBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	// 重建哈希环
	ch.rebuildRing(instances)
	
	// 获取哈希键值
	hashValue := ch.getHashValue(ctx)
	
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	if len(ch.keys) == 0 {
		return instances[0], nil
	}
	
	// 在哈希环中查找
	idx := sort.Search(len(ch.keys), func(i int) bool {
		return ch.keys[i] >= hashValue
	})
	
	if idx == len(ch.keys) {
		idx = 0
	}
	
	return ch.ring[ch.keys[idx]], nil
}

// rebuildRing 重建哈希环
func (ch *ConsistentHashBalancer) rebuildRing(instances []*discovery.ServiceInstance) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	ch.ring = make(map[uint32]*discovery.ServiceInstance)
	ch.keys = nil
	
	for _, instance := range instances {
		// 为每个实例创建多个虚拟节点
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("%s:%d", instance.GetEndpoint(), i)
			hash := ch.hash(key)
			ch.ring[hash] = instance
			ch.keys = append(ch.keys, hash)
		}
	}
	
	sort.Slice(ch.keys, func(i, j int) bool {
		return ch.keys[i] < ch.keys[j]
	})
}

// getHashValue 获取哈希值
func (ch *ConsistentHashBalancer) getHashValue(ctx context.Context) uint32 {
	if ch.hashKey != "" {
		if value := ctx.Value(ch.hashKey); value != nil {
			return ch.hash(fmt.Sprintf("%v", value))
		}
	}
	
	// 默认使用随机值
	return ch.hash(fmt.Sprintf("%d", time.Now().UnixNano()))
}

// hash 计算哈希值
func (ch *ConsistentHashBalancer) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// Name 返回名称
func (ch *ConsistentHashBalancer) Name() string {
	return "consistent_hash"
}

// UpdateStats 更新统计信息
func (ch *ConsistentHashBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	// 一致性哈希算法不需要统计信息
}

// IPHashBalancer IP哈希负载均衡器
type IPHashBalancer struct{}

// NewIPHashBalancer 创建IP哈希负载均衡器
func NewIPHashBalancer() *IPHashBalancer {
	return &IPHashBalancer{}
}

// Select 选择实例
func (ip *IPHashBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	// 获取客户端IP
	clientIP := "127.0.0.1" // 默认值
	if value := ctx.Value("client_ip"); value != nil {
		clientIP = fmt.Sprintf("%v", value)
	}
	
	// 计算哈希值
	h := fnv.New32a()
	h.Write([]byte(clientIP))
	hash := h.Sum32()
	
	index := int(hash) % len(instances)
	return instances[index], nil
}

// Name 返回名称
func (ip *IPHashBalancer) Name() string {
	return "ip_hash"
}

// UpdateStats 更新统计信息
func (ip *IPHashBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	// IP哈希算法不需要统计信息
}

// HealthyFirstBalancer 健康优先负载均衡器
type HealthyFirstBalancer struct {
	fallback LoadBalancer
}

// NewHealthyFirstBalancer 创建健康优先负载均衡器
func NewHealthyFirstBalancer(fallback LoadBalancer) *HealthyFirstBalancer {
	return &HealthyFirstBalancer{
		fallback: fallback,
	}
}

// Select 选择实例
func (hf *HealthyFirstBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	// 优先选择健康的实例
	var healthy []*discovery.ServiceInstance
	for _, instance := range instances {
		if instance.IsHealthy() {
			healthy = append(healthy, instance)
		}
	}
	
	if len(healthy) > 0 {
		return hf.fallback.Select(ctx, healthy)
	}
	
	// 如果没有健康实例，使用所有实例
	return hf.fallback.Select(ctx, instances)
}

// Name 返回名称
func (hf *HealthyFirstBalancer) Name() string {
	return "healthy_first"
}

// UpdateStats 更新统计信息
func (hf *HealthyFirstBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	hf.fallback.UpdateStats(instance, success, duration)
}

// GeographyAwareBalancer 地理位置感知负载均衡器
type GeographyAwareBalancer struct {
	preferredRegion string
	preferredZone   string
	fallback        LoadBalancer
}

// NewGeographyAwareBalancer 创建地理位置感知负载均衡器
func NewGeographyAwareBalancer(preferredRegion, preferredZone string, fallback LoadBalancer) *GeographyAwareBalancer {
	return &GeographyAwareBalancer{
		preferredRegion: preferredRegion,
		preferredZone:   preferredZone,
		fallback:        fallback,
	}
}

// Select 选择实例
func (ga *GeographyAwareBalancer) Select(ctx context.Context, instances []*discovery.ServiceInstance) (*discovery.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("no instances available")
	}
	
	// 优先选择同区域同可用区的实例
	if ga.preferredRegion != "" && ga.preferredZone != "" {
		var sameZone []*discovery.ServiceInstance
		for _, instance := range instances {
			if instance.Region == ga.preferredRegion && instance.Zone == ga.preferredZone {
				sameZone = append(sameZone, instance)
			}
		}
		if len(sameZone) > 0 {
			return ga.fallback.Select(ctx, sameZone)
		}
	}
	
	// 其次选择同区域的实例
	if ga.preferredRegion != "" {
		var sameRegion []*discovery.ServiceInstance
		for _, instance := range instances {
			if instance.Region == ga.preferredRegion {
				sameRegion = append(sameRegion, instance)
			}
		}
		if len(sameRegion) > 0 {
			return ga.fallback.Select(ctx, sameRegion)
		}
	}
	
	// 最后使用所有实例
	return ga.fallback.Select(ctx, instances)
}

// Name 返回名称
func (ga *GeographyAwareBalancer) Name() string {
	return "geography_aware"
}

// UpdateStats 更新统计信息
func (ga *GeographyAwareBalancer) UpdateStats(instance *discovery.ServiceInstance, success bool, duration time.Duration) {
	ga.fallback.UpdateStats(instance, success, duration)
}

// 默认负载均衡管理器
var DefaultManager = NewManager(nil)

// Select 使用默认管理器选择实例
func Select(ctx context.Context, instances []*discovery.ServiceInstance, algorithm Algorithm) (*discovery.ServiceInstance, error) {
	return DefaultManager.Select(ctx, instances, algorithm)
}

// UpdateStats 使用默认管理器更新统计信息
func UpdateStats(instanceID string, success bool, duration time.Duration) {
	DefaultManager.UpdateStats(instanceID, success, duration)
}