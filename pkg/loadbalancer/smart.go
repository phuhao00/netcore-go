package loadbalancer

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// AdaptiveLoadBalancing 自适应负载均衡策略
const AdaptiveLoadBalancing Algorithm = "adaptive"

// LoadBalanceStrategy 负载均衡策略类型
type LoadBalanceStrategy Algorithm

// WeightedLeastConnections 加权最少连接算法
const WeightedLeastConnections Algorithm = "weighted_least_connections"

// String 返回算法的字符串表示
func (a Algorithm) String() string {
	return string(a)
}

// String 返回策略的字符串表示
func (s LoadBalanceStrategy) String() string {
	return string(s)
}



// SmartLoadBalancerConfig 智能负载均衡配置
type SmartLoadBalancerConfig struct {
	// 负载均衡策略
	Strategy Algorithm `json:"strategy" yaml:"strategy"`
	// 健康检查间隔
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	// 健康检查超时
	HealthCheckTimeout time.Duration `json:"health_check_timeout" yaml:"health_check_timeout"`
	// 最大重试次数
	MaxRetries int `json:"max_retries" yaml:"max_retries"`
	// 故障转移启用
	FailoverEnabled bool `json:"failover_enabled" yaml:"failover_enabled"`
	// 自适应调整间隔
	AdaptiveInterval time.Duration `json:"adaptive_interval" yaml:"adaptive_interval"`
	// 响应时间窗口大小
	ResponseTimeWindow int `json:"response_time_window" yaml:"response_time_window"`
	// 连接超时
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`
	// 启用会话保持
	SessionAffinity bool `json:"session_affinity" yaml:"session_affinity"`
	// 会话超时
	SessionTimeout time.Duration `json:"session_timeout" yaml:"session_timeout"`
}

// DefaultSmartLoadBalancerConfig 默认配置
func DefaultSmartLoadBalancerConfig() *SmartLoadBalancerConfig {
	return &SmartLoadBalancerConfig{
		Strategy:            AdaptiveLoadBalancing,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		MaxRetries:          3,
		FailoverEnabled:     true,
		AdaptiveInterval:    60 * time.Second,
		ResponseTimeWindow:  100,
		ConnectionTimeout:   10 * time.Second,
		SessionAffinity:     false,
		SessionTimeout:      30 * time.Minute,
	}
}

// Backend 后端服务器
type Backend struct {
	// 服务器ID
	ID string `json:"id"`
	// 服务器地址
	Address string `json:"address"`
	// 权重
	Weight int `json:"weight"`
	// 是否健康
	Healthy bool `json:"healthy"`
	// 当前连接数
	Connections int64 `json:"connections"`
	// 总请求数
	TotalRequests int64 `json:"total_requests"`
	// 成功请求数
	SuccessRequests int64 `json:"success_requests"`
	// 失败请求数
	FailedRequests int64 `json:"failed_requests"`
	// 平均响应时间 (毫秒)
	AvgResponseTime float64 `json:"avg_response_time"`
	// 响应时间历史
	ResponseTimes []float64 `json:"-"`
	// 最后健康检查时间
	LastHealthCheck time.Time `json:"last_health_check"`
	// 最后错误时间
	LastError time.Time `json:"last_error"`
	// 错误计数
	ErrorCount int64 `json:"error_count"`
	// CPU使用率
	CPUUsage float64 `json:"cpu_usage"`
	// 内存使用率
	MemoryUsage float64 `json:"memory_usage"`
	// 负载分数 (0-1, 越低越好)
	LoadScore float64 `json:"load_score"`
	// 互斥锁
	mu sync.RWMutex
}

// NewBackend 创建后端服务器
func NewBackend(id, address string, weight int) *Backend {
	return &Backend{
		ID:              id,
		Address:         address,
		Weight:          weight,
		Healthy:         true,
		ResponseTimes:   make([]float64, 0, 100),
		LastHealthCheck: time.Now(),
		LoadScore:       0.0,
	}
}

// AddConnection 增加连接数
func (b *Backend) AddConnection() {
	atomic.AddInt64(&b.Connections, 1)
}

// RemoveConnection 减少连接数
func (b *Backend) RemoveConnection() {
	atomic.AddInt64(&b.Connections, -1)
}

// AddRequest 添加请求统计
func (b *Backend) AddRequest(responseTime time.Duration, success bool) {
	atomic.AddInt64(&b.TotalRequests, 1)

	if success {
		atomic.AddInt64(&b.SuccessRequests, 1)
	} else {
		atomic.AddInt64(&b.FailedRequests, 1)
		atomic.AddInt64(&b.ErrorCount, 1)
		b.LastError = time.Now()
	}

	// 更新响应时间
	b.mu.Lock()
	respTime := float64(responseTime.Nanoseconds()) / 1e6 // 转换为毫秒
	b.ResponseTimes = append(b.ResponseTimes, respTime)

	// 保持响应时间历史在合理范围内
	if len(b.ResponseTimes) > 100 {
		b.ResponseTimes = b.ResponseTimes[1:]
	}

	// 计算平均响应时间
	var sum float64
	for _, rt := range b.ResponseTimes {
		sum += rt
	}
	b.AvgResponseTime = sum / float64(len(b.ResponseTimes))
	b.mu.Unlock()

	// 更新负载分数
	b.updateLoadScore()
}

// updateLoadScore 更新负载分数
func (b *Backend) updateLoadScore() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.Healthy {
		b.LoadScore = 1.0 // 不健康的服务器负载分数最高
		return
	}

	// 综合考虑多个因素计算负载分数
	connectionScore := float64(b.Connections) / 1000.0 // 连接数因子
	responseTimeScore := b.AvgResponseTime / 1000.0     // 响应时间因子
	errorScore := float64(b.ErrorCount) / 100.0        // 错误率因子
	cpuScore := b.CPUUsage                              // CPU使用率因子
	memoryScore := b.MemoryUsage                        // 内存使用率因子

	// 加权计算负载分数
	b.LoadScore = (connectionScore*0.2 + responseTimeScore*0.3 + errorScore*0.2 + cpuScore*0.15 + memoryScore*0.15)

	// 确保分数在0-1范围内
	if b.LoadScore > 1.0 {
		b.LoadScore = 1.0
	}
}

// GetStats 获取统计信息
func (b *Backend) GetStats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return map[string]interface{}{
		"id":                 b.ID,
		"address":            b.Address,
		"weight":             b.Weight,
		"healthy":            b.Healthy,
		"connections":        atomic.LoadInt64(&b.Connections),
		"total_requests":     atomic.LoadInt64(&b.TotalRequests),
		"success_requests":   atomic.LoadInt64(&b.SuccessRequests),
		"failed_requests":    atomic.LoadInt64(&b.FailedRequests),
		"avg_response_time":  b.AvgResponseTime,
		"error_count":        atomic.LoadInt64(&b.ErrorCount),
		"cpu_usage":          b.CPUUsage,
		"memory_usage":       b.MemoryUsage,
		"load_score":         b.LoadScore,
		"last_health_check": b.LastHealthCheck,
		"last_error":         b.LastError,
	}
}

// SmartLoadBalancer 智能负载均衡器
type SmartLoadBalancer struct {
	config           *SmartLoadBalancerConfig
	backends         []*Backend
	currentIndex     int64
	sessions         map[string]*Backend // 会话保持
	consistentHash   *ConsistentHashRing
	mu               sync.RWMutex
	running          bool
	cancel           context.CancelFunc
	stats            *LoadBalancerStats
	adaptiveWeights  map[string]float64 // 自适应权重
	rand             *rand.Rand
}

// LoadBalancerStats 负载均衡器统计
type LoadBalancerStats struct {
	TotalRequests    int64     `json:"total_requests"`
	SuccessRequests  int64     `json:"success_requests"`
	FailedRequests   int64     `json:"failed_requests"`
	AvgResponseTime  float64   `json:"avg_response_time"`
	ActiveBackends   int       `json:"active_backends"`
	HealthyBackends  int       `json:"healthy_backends"`
	LastStrategySwitch time.Time `json:"last_strategy_switch"`
	CurrentStrategy  string    `json:"current_strategy"`
}

// NewSmartLoadBalancer 创建智能负载均衡器
func NewSmartLoadBalancer(config *SmartLoadBalancerConfig) *SmartLoadBalancer {
	if config == nil {
		config = DefaultSmartLoadBalancerConfig()
	}

	lb := &SmartLoadBalancer{
		config:          config,
		backends:        make([]*Backend, 0),
		sessions:        make(map[string]*Backend),
		consistentHash:  NewConsistentHashRing(100, nil),
		stats:           &LoadBalancerStats{},
		adaptiveWeights: make(map[string]float64),
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	lb.stats.CurrentStrategy = config.Strategy.String()

	return lb
}

// AddBackend 添加后端服务器
func (lb *SmartLoadBalancer) AddBackend(backend *Backend) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.backends = append(lb.backends, backend)
	lb.consistentHash.Add(backend.ID)
	lb.adaptiveWeights[backend.ID] = float64(backend.Weight)

	lb.updateStats()
}

// RemoveBackend 移除后端服务器
func (lb *SmartLoadBalancer) RemoveBackend(backendID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i, backend := range lb.backends {
		if backend.ID == backendID {
			lb.backends = append(lb.backends[:i], lb.backends[i+1:]...)
			lb.consistentHash.Remove(backendID)
			delete(lb.adaptiveWeights, backendID)
			break
		}
	}

	// 清理会话
	for sessionID, backend := range lb.sessions {
		if backend.ID == backendID {
			delete(lb.sessions, sessionID)
		}
	}

	lb.updateStats()
}

// Start 启动负载均衡器
func (lb *SmartLoadBalancer) Start(ctx context.Context) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.running {
		return fmt.Errorf("load balancer already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	lb.cancel = cancel
	lb.running = true

	// 启动健康检查
	go lb.healthChecker(ctx)

	// 启动自适应调整
	if lb.config.Strategy == AdaptiveLoadBalancing {
		go lb.adaptiveAdjuster(ctx)
	}

	// 启动会话清理
	if lb.config.SessionAffinity {
		go lb.sessionCleaner(ctx)
	}

	return nil
}

// Stop 停止负载均衡器
func (lb *SmartLoadBalancer) Stop() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if !lb.running {
		return fmt.Errorf("load balancer not running")
	}

	lb.cancel()
	lb.running = false

	return nil
}

// SelectBackend 选择后端服务器
func (lb *SmartLoadBalancer) SelectBackend(clientIP string, sessionID string) (*Backend, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.backends) == 0 {
		return nil, fmt.Errorf("no backends available")
	}

	// 检查会话保持
	if lb.config.SessionAffinity && sessionID != "" {
		if backend, exists := lb.sessions[sessionID]; exists && backend.Healthy {
			return backend, nil
		}
	}

	// 获取健康的后端服务器
	healthyBackends := lb.getHealthyBackends()
	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("no healthy backends available")
	}

	var selected *Backend
	var err error

	// 根据策略选择后端
	switch lb.config.Strategy {
	case RoundRobin:
		selected = lb.selectRoundRobin(healthyBackends)
	case WeightedRoundRobin:
		selected = lb.selectWeightedRoundRobin(healthyBackends)
	case LeastConnections:
		selected = lb.selectLeastConnections(healthyBackends)
	case WeightedLeastConnections:
		selected = lb.selectWeightedLeastConnections(healthyBackends)
	case IPHash:
		selected = lb.selectIPHash(healthyBackends, clientIP)
	case ConsistentHash:
		selected = lb.selectConsistentHash(healthyBackends, clientIP)
	case Random:
		selected = lb.selectRandom(healthyBackends)
	case WeightedRandom:
		selected = lb.selectWeightedRandom(healthyBackends)
	case LeastResponseTime:
		selected = lb.selectLeastResponseTime(healthyBackends)
	case AdaptiveLoadBalancing:
		selected = lb.selectAdaptive(healthyBackends)
	default:
		selected = lb.selectRoundRobin(healthyBackends)
	}

	if selected == nil {
		return nil, fmt.Errorf("failed to select backend")
	}

	// 更新会话
	if lb.config.SessionAffinity && sessionID != "" {
		lb.sessions[sessionID] = selected
	}

	atomic.AddInt64(&lb.stats.TotalRequests, 1)
	return selected, err
}

// getHealthyBackends 获取健康的后端服务器
func (lb *SmartLoadBalancer) getHealthyBackends() []*Backend {
	healthy := make([]*Backend, 0, len(lb.backends))
	for _, backend := range lb.backends {
		if backend.Healthy {
			healthy = append(healthy, backend)
		}
	}
	return healthy
}

// selectRoundRobin 轮询选择
func (lb *SmartLoadBalancer) selectRoundRobin(backends []*Backend) *Backend {
	index := atomic.AddInt64(&lb.currentIndex, 1) % int64(len(backends))
	return backends[index]
}

// selectWeightedRoundRobin 加权轮询选择
func (lb *SmartLoadBalancer) selectWeightedRoundRobin(backends []*Backend) *Backend {
	totalWeight := 0
	for _, backend := range backends {
		totalWeight += backend.Weight
	}

	if totalWeight == 0 {
		return lb.selectRoundRobin(backends)
	}

	index := atomic.AddInt64(&lb.currentIndex, 1) % int64(totalWeight)
	currentWeight := int64(0)

	for _, backend := range backends {
		currentWeight += int64(backend.Weight)
		if index < currentWeight {
			return backend
		}
	}

	return backends[0]
}

// selectLeastConnections 最少连接选择
func (lb *SmartLoadBalancer) selectLeastConnections(backends []*Backend) *Backend {
	var selected *Backend
	minConnections := int64(math.MaxInt64)

	for _, backend := range backends {
		connections := atomic.LoadInt64(&backend.Connections)
		if connections < minConnections {
			minConnections = connections
			selected = backend
		}
	}

	return selected
}

// selectWeightedLeastConnections 加权最少连接选择
func (lb *SmartLoadBalancer) selectWeightedLeastConnections(backends []*Backend) *Backend {
	var selected *Backend
	minRatio := math.MaxFloat64

	for _, backend := range backends {
		connections := atomic.LoadInt64(&backend.Connections)
		weight := float64(backend.Weight)
		if weight == 0 {
			weight = 1
		}
		ratio := float64(connections) / weight

		if ratio < minRatio {
			minRatio = ratio
			selected = backend
		}
	}

	return selected
}

// selectIPHash IP哈希选择
func (lb *SmartLoadBalancer) selectIPHash(backends []*Backend, clientIP string) *Backend {
	hash := lb.hashString(clientIP)
	index := hash % uint32(len(backends))
	return backends[index]
}

// selectConsistentHash 一致性哈希选择
func (lb *SmartLoadBalancer) selectConsistentHash(backends []*Backend, clientIP string) *Backend {
	backendID := lb.consistentHash.Get(clientIP)
	for _, backend := range backends {
		if backend.ID == backendID {
			return backend
		}
	}
	// 如果找不到，回退到轮询
	return lb.selectRoundRobin(backends)
}

// selectRandom 随机选择
func (lb *SmartLoadBalancer) selectRandom(backends []*Backend) *Backend {
	index := lb.rand.Intn(len(backends))
	return backends[index]
}

// selectWeightedRandom 加权随机选择
func (lb *SmartLoadBalancer) selectWeightedRandom(backends []*Backend) *Backend {
	totalWeight := 0
	for _, backend := range backends {
		totalWeight += backend.Weight
	}

	if totalWeight == 0 {
		return lb.selectRandom(backends)
	}

	random := lb.rand.Intn(totalWeight)
	currentWeight := 0

	for _, backend := range backends {
		currentWeight += backend.Weight
		if random < currentWeight {
			return backend
		}
	}

	return backends[0]
}

// selectLeastResponseTime 最短响应时间选择
func (lb *SmartLoadBalancer) selectLeastResponseTime(backends []*Backend) *Backend {
	var selected *Backend
	minResponseTime := math.MaxFloat64

	for _, backend := range backends {
		backend.mu.RLock()
		responseTime := backend.AvgResponseTime
		backend.mu.RUnlock()

		if responseTime < minResponseTime {
			minResponseTime = responseTime
			selected = backend
		}
	}

	return selected
}

// selectAdaptive 自适应选择
func (lb *SmartLoadBalancer) selectAdaptive(backends []*Backend) *Backend {
	var selected *Backend
	minLoadScore := math.MaxFloat64

	for _, backend := range backends {
		backend.mu.RLock()
		loadScore := backend.LoadScore
		backend.mu.RUnlock()

		// 结合自适应权重
		adaptiveWeight := lb.adaptiveWeights[backend.ID]
		adjustedScore := loadScore / adaptiveWeight

		if adjustedScore < minLoadScore {
			minLoadScore = adjustedScore
			selected = backend
		}
	}

	return selected
}

// hashString 字符串哈希
func (lb *SmartLoadBalancer) hashString(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

// healthChecker 健康检查器
func (lb *SmartLoadBalancer) healthChecker(ctx context.Context) {
	ticker := time.NewTicker(lb.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lb.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (lb *SmartLoadBalancer) performHealthCheck() {
	lb.mu.RLock()
	backends := make([]*Backend, len(lb.backends))
	copy(backends, lb.backends)
	lb.mu.RUnlock()

	for _, backend := range backends {
		go lb.checkBackendHealth(backend)
	}

	lb.updateStats()
}

// checkBackendHealth 检查后端健康状态
func (lb *SmartLoadBalancer) checkBackendHealth(backend *Backend) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", backend.Address, lb.config.HealthCheckTimeout)
	duration := time.Since(start)

	backend.mu.Lock()
	backend.LastHealthCheck = time.Now()

	if err != nil {
		backend.Healthy = false
		backend.LastError = time.Now()
		atomic.AddInt64(&backend.ErrorCount, 1)
	} else {
		backend.Healthy = true
		conn.Close()
		
		// 更新响应时间
		respTime := float64(duration.Nanoseconds()) / 1e6
		backend.ResponseTimes = append(backend.ResponseTimes, respTime)
		if len(backend.ResponseTimes) > lb.config.ResponseTimeWindow {
			backend.ResponseTimes = backend.ResponseTimes[1:]
		}
		
		// 重新计算平均响应时间
		var sum float64
		for _, rt := range backend.ResponseTimes {
			sum += rt
		}
		backend.AvgResponseTime = sum / float64(len(backend.ResponseTimes))
	}

	backend.mu.Unlock()

	// 更新负载分数
	backend.updateLoadScore()
}

// adaptiveAdjuster 自适应调整器
func (lb *SmartLoadBalancer) adaptiveAdjuster(ctx context.Context) {
	ticker := time.NewTicker(lb.config.AdaptiveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lb.adjustAdaptiveWeights()
		}
	}
}

// adjustAdaptiveWeights 调整自适应权重
func (lb *SmartLoadBalancer) adjustAdaptiveWeights() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, backend := range lb.backends {
		if !backend.Healthy {
			lb.adaptiveWeights[backend.ID] = 0.1 // 不健康的服务器权重很低
			continue
		}

		// 基于性能指标调整权重
		backend.mu.RLock()
		loadScore := backend.LoadScore
		successRate := float64(backend.SuccessRequests) / float64(backend.TotalRequests+1)
		backend.mu.RUnlock()

		// 计算新权重
		newWeight := float64(backend.Weight) * successRate * (1.0 - loadScore)
		if newWeight < 0.1 {
			newWeight = 0.1
		}
		if newWeight > 10.0 {
			newWeight = 10.0
		}

		lb.adaptiveWeights[backend.ID] = newWeight
	}
}

// sessionCleaner 会话清理器
func (lb *SmartLoadBalancer) sessionCleaner(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lb.cleanExpiredSessions()
		}
	}
}

// cleanExpiredSessions 清理过期会话
func (lb *SmartLoadBalancer) cleanExpiredSessions() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// 这里简化实现，实际应该记录会话创建时间
	// 清理不健康后端的会话
	for sessionID, backend := range lb.sessions {
		if !backend.Healthy {
			delete(lb.sessions, sessionID)
		}
	}
}

// updateStats 更新统计信息
func (lb *SmartLoadBalancer) updateStats() {
	lb.stats.ActiveBackends = len(lb.backends)
	lb.stats.HealthyBackends = len(lb.getHealthyBackends())
	lb.stats.CurrentStrategy = lb.config.Strategy.String()

	// 计算总体统计
	var totalRequests, successRequests, failedRequests int64
	var totalResponseTime float64
	healthyCount := 0

	for _, backend := range lb.backends {
		totalRequests += atomic.LoadInt64(&backend.TotalRequests)
		successRequests += atomic.LoadInt64(&backend.SuccessRequests)
		failedRequests += atomic.LoadInt64(&backend.FailedRequests)

		if backend.Healthy {
			backend.mu.RLock()
			totalResponseTime += backend.AvgResponseTime
			backend.mu.RUnlock()
			healthyCount++
		}
	}

	lb.stats.TotalRequests = totalRequests
	lb.stats.SuccessRequests = successRequests
	lb.stats.FailedRequests = failedRequests

	if healthyCount > 0 {
		lb.stats.AvgResponseTime = totalResponseTime / float64(healthyCount)
	}
}

// GetStats 获取统计信息
func (lb *SmartLoadBalancer) GetStats() *LoadBalancerStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	lb.updateStats()
	return &LoadBalancerStats{
		TotalRequests:      lb.stats.TotalRequests,
		SuccessRequests:    lb.stats.SuccessRequests,
		FailedRequests:     lb.stats.FailedRequests,
		AvgResponseTime:    lb.stats.AvgResponseTime,
		ActiveBackends:     lb.stats.ActiveBackends,
		HealthyBackends:    lb.stats.HealthyBackends,
		LastStrategySwitch: lb.stats.LastStrategySwitch,
		CurrentStrategy:    lb.stats.CurrentStrategy,
	}
}

// GetBackends 获取所有后端服务器
func (lb *SmartLoadBalancer) GetBackends() []*Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	backends := make([]*Backend, len(lb.backends))
	copy(backends, lb.backends)
	return backends
}

// SetStrategy 设置负载均衡策略
func (lb *SmartLoadBalancer) SetStrategy(strategy LoadBalanceStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.config.Strategy = Algorithm(strategy)
	lb.stats.LastStrategySwitch = time.Now()
	lb.stats.CurrentStrategy = strategy.String()
}

// ConsistentHashRing 一致性哈希环实现
type ConsistentHashRing struct {
	hash     func(data []byte) uint32
	replicas int
	keys     []int // Sorted
	hashMap  map[int]string
	mu       sync.RWMutex
}

// NewConsistentHashRing 创建一致性哈希环
func NewConsistentHashRing(replicas int, fn func([]byte) uint32) *ConsistentHashRing {
	m := &ConsistentHashRing{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = func(data []byte) uint32 {
			h := uint32(0)
			for _, b := range data {
				h = h*31 + uint32(b)
			}
			return h
		}
	}
	return m
}

// Add 添加节点
func (m *ConsistentHashRing) Add(keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(fmt.Sprintf("%s%d", key, i))))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

// Remove 移除节点
func (m *ConsistentHashRing) Remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(fmt.Sprintf("%s%d", key, i))))
		delete(m.hashMap, hash)
		
		// 从keys中移除
		for j, k := range m.keys {
			if k == hash {
				m.keys = append(m.keys[:j], m.keys[j+1:]...)
				break
			}
		}
	}
}

// Get 获取节点
func (m *ConsistentHashRing) Get(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))

	// 二分查找
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	// 如果超出范围，使用第一个节点
	if idx == len(m.keys) {
		idx = 0
	}

	return m.hashMap[m.keys[idx]]
}