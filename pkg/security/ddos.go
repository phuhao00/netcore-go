// Package security 防DDoS攻击机制
// Author: NetCore-Go Team
// Created: 2024

package security

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AttackType 攻击类型
type AttackType string

const (
	AttackTypeFlood       AttackType = "flood"        // 洪水攻击
	AttackTypeSlow        AttackType = "slow"         // 慢速攻击
	AttackTypeAmplification AttackType = "amplification" // 放大攻击
	AttackTypeReflection  AttackType = "reflection"   // 反射攻击
	AttackTypeBotnet      AttackType = "botnet"       // 僵尸网络
	AttackTypeVolumetric  AttackType = "volumetric"   // 容量攻击
)

// ThreatLevel 威胁级别
type ThreatLevel int

const (
	ThreatLevelLow ThreatLevel = iota
	ThreatLevelMedium
	ThreatLevelHigh
	ThreatLevelCritical
)

// String 返回威胁级别字符串
func (tl ThreatLevel) String() string {
	switch tl {
	case ThreatLevelLow:
		return "LOW"
	case ThreatLevelMedium:
		return "MEDIUM"
	case ThreatLevelHigh:
		return "HIGH"
	case ThreatLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// RateLimitRule 速率限制规则
type RateLimitRule struct {
	Name        string        `json:"name"`
	MaxRequests int           `json:"max_requests"`
	Window      time.Duration `json:"window"`
	BurstSize   int           `json:"burst_size"`
	Enabled     bool          `json:"enabled"`
}

// IPTracker IP追踪器
type IPTracker struct {
	IP            string    `json:"ip"`
	RequestCount  int64     `json:"request_count"`
	LastRequest   time.Time `json:"last_request"`
	FirstRequest  time.Time `json:"first_request"`
	Blocked       bool      `json:"blocked"`
	BlockedUntil  time.Time `json:"blocked_until"`
	ThreatLevel   ThreatLevel `json:"threat_level"`
	AttackTypes   []AttackType `json:"attack_types"`
	GeoLocation   string    `json:"geo_location"`
	UserAgent     string    `json:"user_agent"`
	RiskScore     int       `json:"risk_score"`
}

// ConnectionTracker 连接追踪器
type ConnectionTracker struct {
	ActiveConnections int64     `json:"active_connections"`
	TotalConnections  int64     `json:"total_connections"`
	LastUpdate        time.Time `json:"last_update"`
	BandwidthUsage    int64     `json:"bandwidth_usage"`
	PacketRate        float64   `json:"packet_rate"`
}

// DDoSProtector DDoS防护器
type DDoSProtector struct {
	// 配置
	config *DDoSConfig

	// IP追踪
	ipTrackers map[string]*IPTracker
	ipMutex    sync.RWMutex

	// 连接追踪
	connTracker *ConnectionTracker
	connMutex   sync.RWMutex

	// 速率限制
	rateLimiters map[string]*RateLimiter
	rlMutex      sync.RWMutex

	// 黑名单和白名单
	blacklist map[string]time.Time
	whitelist map[string]bool
	listMutex sync.RWMutex

	// 统计信息
	stats *DDoSStats

	// 事件回调
	onAttackDetected func(AttackInfo)
	onIPBlocked      func(string, time.Duration)
	onThresholdExceeded func(string, string, int64)

	// 控制
	running  int32
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// DDoSConfig DDoS防护配置
type DDoSConfig struct {
	// 基础配置
	Enabled bool `json:"enabled"`

	// 连接限制
	MaxConnectionsPerIP   int           `json:"max_connections_per_ip"`
	MaxTotalConnections   int           `json:"max_total_connections"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`

	// 速率限制
	RequestsPerSecond     int           `json:"requests_per_second"`
	BurstSize            int           `json:"burst_size"`
	RateLimitWindow      time.Duration `json:"rate_limit_window"`

	// 带宽限制
	MaxBandwidthPerIP    int64         `json:"max_bandwidth_per_ip"`
	MaxTotalBandwidth    int64         `json:"max_total_bandwidth"`

	// 检测阈值
	FloodThreshold       int           `json:"flood_threshold"`
	SlowConnectionThreshold time.Duration `json:"slow_connection_threshold"`
	SuspiciousPatternThreshold int     `json:"suspicious_pattern_threshold"`

	// 阻断配置
	BlockDuration        time.Duration `json:"block_duration"`
	AutoBlockEnabled     bool          `json:"auto_block_enabled"`
	ProgressiveBlocking  bool          `json:"progressive_blocking"`

	// 清理配置
	CleanupInterval      time.Duration `json:"cleanup_interval"`
	TrackerExpiry        time.Duration `json:"tracker_expiry"`

	// 地理位置过滤
	GeoFilterEnabled     bool          `json:"geo_filter_enabled"`
	AllowedCountries     []string      `json:"allowed_countries"`
	BlockedCountries     []string      `json:"blocked_countries"`
}

// DefaultDDoSConfig 返回默认DDoS防护配置
func DefaultDDoSConfig() *DDoSConfig {
	return &DDoSConfig{
		Enabled:                    true,
		MaxConnectionsPerIP:        100,
		MaxTotalConnections:        10000,
		ConnectionTimeout:          30 * time.Second,
		RequestsPerSecond:          100,
		BurstSize:                  200,
		RateLimitWindow:            time.Minute,
		MaxBandwidthPerIP:          10 * 1024 * 1024, // 10MB/s
		MaxTotalBandwidth:          1024 * 1024 * 1024, // 1GB/s
		FloodThreshold:             1000,
		SlowConnectionThreshold:    60 * time.Second,
		SuspiciousPatternThreshold: 50,
		BlockDuration:              15 * time.Minute,
		AutoBlockEnabled:           true,
		ProgressiveBlocking:        true,
		CleanupInterval:            5 * time.Minute,
		TrackerExpiry:              time.Hour,
		GeoFilterEnabled:           false,
	}
}

// DDoSStats DDoS统计信息
type DDoSStats struct {
	TotalRequests     int64 `json:"total_requests"`
	BlockedRequests   int64 `json:"blocked_requests"`
	ActiveConnections int64 `json:"active_connections"`
	BlockedIPs        int64 `json:"blocked_ips"`
	AttacksDetected   int64 `json:"attacks_detected"`
	BandwidthUsage    int64 `json:"bandwidth_usage"`
	LastAttack        time.Time `json:"last_attack"`
	Uptime            time.Time `json:"uptime"`
}

// AttackInfo 攻击信息
type AttackInfo struct {
	Type        AttackType    `json:"type"`
	SourceIP    string        `json:"source_ip"`
	ThreatLevel ThreatLevel   `json:"threat_level"`
	StartTime   time.Time     `json:"start_time"`
	Duration    time.Duration `json:"duration"`
	RequestRate float64       `json:"request_rate"`
	Description string        `json:"description"`
	Mitigated   bool          `json:"mitigated"`
}

// RateLimiter 速率限制器
type RateLimiter struct {
	tokens    int64
	maxTokens int64
	refillRate int64
	lastRefill time.Time
	mu        sync.Mutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(maxTokens, refillRate int64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// 补充令牌
	tokensToAdd := int64(elapsed.Seconds()) * rl.refillRate
	rl.tokens += tokensToAdd
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now

	// 检查是否有可用令牌
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// NewDDoSProtector 创建DDoS防护器
func NewDDoSProtector(config *DDoSConfig) *DDoSProtector {
	if config == nil {
		config = DefaultDDoSConfig()
	}

	protector := &DDoSProtector{
		config:       config,
		ipTrackers:   make(map[string]*IPTracker),
		connTracker:  &ConnectionTracker{LastUpdate: time.Now()},
		rateLimiters: make(map[string]*RateLimiter),
		blacklist:    make(map[string]time.Time),
		whitelist:    make(map[string]bool),
		stats:        &DDoSStats{Uptime: time.Now()},
		stopChan:     make(chan struct{}),
	}

	return protector
}

// Start 启动DDoS防护
func (ddp *DDoSProtector) Start() error {
	if !atomic.CompareAndSwapInt32(&ddp.running, 0, 1) {
		return fmt.Errorf("DDoS protector is already running")
	}

	if !ddp.config.Enabled {
		return fmt.Errorf("DDoS protection is disabled")
	}

	// 启动清理协程
	ddp.wg.Add(1)
	go ddp.cleanupLoop()

	// 启动监控协程
	ddp.wg.Add(1)
	go ddp.monitorLoop()

	return nil
}

// Stop 停止DDoS防护
func (ddp *DDoSProtector) Stop() error {
	if !atomic.CompareAndSwapInt32(&ddp.running, 1, 0) {
		return nil // 已经停止
	}

	close(ddp.stopChan)
	ddp.wg.Wait()

	return nil
}

// CheckRequest 检查请求是否允许
func (ddp *DDoSProtector) CheckRequest(clientIP string, userAgent string, requestSize int64) (bool, string) {
	if !ddp.config.Enabled {
		return true, ""
	}

	// 检查白名单
	ddp.listMutex.RLock()
	if ddp.whitelist[clientIP] {
		ddp.listMutex.RUnlock()
		return true, ""
	}
	ddp.listMutex.RUnlock()

	// 检查黑名单
	ddp.listMutex.RLock()
	if blockUntil, blocked := ddp.blacklist[clientIP]; blocked {
		ddp.listMutex.RUnlock()
		if time.Now().Before(blockUntil) {
			atomic.AddInt64(&ddp.stats.BlockedRequests, 1)
			return false, "IP is blacklisted"
		}
		// 黑名单已过期，移除
		ddp.listMutex.Lock()
		delete(ddp.blacklist, clientIP)
		ddp.listMutex.Unlock()
	}
	ddp.listMutex.RUnlock()

	// 更新IP追踪
	ddp.updateIPTracker(clientIP, userAgent, requestSize)

	// 检查速率限制
	if !ddp.checkRateLimit(clientIP) {
		atomic.AddInt64(&ddp.stats.BlockedRequests, 1)
		return false, "Rate limit exceeded"
	}

	// 检查连接限制
	if !ddp.checkConnectionLimit(clientIP) {
		atomic.AddInt64(&ddp.stats.BlockedRequests, 1)
		return false, "Connection limit exceeded"
	}

	// 检查攻击模式
	if attack := ddp.detectAttack(clientIP); attack != nil {
		ddp.handleAttack(*attack)
		atomic.AddInt64(&ddp.stats.BlockedRequests, 1)
		return false, fmt.Sprintf("Attack detected: %s", attack.Type)
	}

	atomic.AddInt64(&ddp.stats.TotalRequests, 1)
	return true, ""
}

// updateIPTracker 更新IP追踪信息
func (ddp *DDoSProtector) updateIPTracker(clientIP, userAgent string, requestSize int64) {
	ddp.ipMutex.Lock()
	defer ddp.ipMutex.Unlock()

	tracker, exists := ddp.ipTrackers[clientIP]
	if !exists {
		tracker = &IPTracker{
			IP:           clientIP,
			FirstRequest: time.Now(),
			ThreatLevel:  ThreatLevelLow,
			UserAgent:    userAgent,
		}
		ddp.ipTrackers[clientIP] = tracker
	}

	tracker.RequestCount++
	tracker.LastRequest = time.Now()

	// 更新带宽使用
	atomic.AddInt64(&ddp.stats.BandwidthUsage, requestSize)
}

// checkRateLimit 检查速率限制
func (ddp *DDoSProtector) checkRateLimit(clientIP string) bool {
	ddp.rlMutex.Lock()
	defer ddp.rlMutex.Unlock()

	limiter, exists := ddp.rateLimiters[clientIP]
	if !exists {
		limiter = NewRateLimiter(
			int64(ddp.config.BurstSize),
			int64(ddp.config.RequestsPerSecond),
		)
		ddp.rateLimiters[clientIP] = limiter
	}

	return limiter.Allow()
}

// checkConnectionLimit 检查连接限制
func (ddp *DDoSProtector) checkConnectionLimit(clientIP string) bool {
	ddp.ipMutex.RLock()
	tracker := ddp.ipTrackers[clientIP]
	ddp.ipMutex.RUnlock()

	if tracker == nil {
		return true
	}

	// 检查单IP连接数限制
	if tracker.RequestCount > int64(ddp.config.MaxConnectionsPerIP) {
		return false
	}

	// 检查总连接数限制
	ddp.connMutex.RLock()
	activeConns := ddp.connTracker.ActiveConnections
	ddp.connMutex.RUnlock()

	if activeConns >= int64(ddp.config.MaxTotalConnections) {
		return false
	}

	return true
}

// detectAttack 检测攻击
func (ddp *DDoSProtector) detectAttack(clientIP string) *AttackInfo {
	ddp.ipMutex.RLock()
	tracker := ddp.ipTrackers[clientIP]
	ddp.ipMutex.RUnlock()

	if tracker == nil {
		return nil
	}

	now := time.Now()
	duration := now.Sub(tracker.FirstRequest)

	// 检测洪水攻击
	if duration > 0 {
		requestRate := float64(tracker.RequestCount) / duration.Seconds()
		if requestRate > float64(ddp.config.FloodThreshold) {
			return &AttackInfo{
				Type:        AttackTypeFlood,
				SourceIP:    clientIP,
				ThreatLevel: ThreatLevelHigh,
				StartTime:   tracker.FirstRequest,
				Duration:    duration,
				RequestRate: requestRate,
				Description: fmt.Sprintf("Flood attack detected: %.2f requests/second", requestRate),
			}
		}
	}

	// 检测慢速攻击
	if duration > ddp.config.SlowConnectionThreshold {
		if tracker.RequestCount < 10 { // 长时间少量请求
			return &AttackInfo{
				Type:        AttackTypeSlow,
				SourceIP:    clientIP,
				ThreatLevel: ThreatLevelMedium,
				StartTime:   tracker.FirstRequest,
				Duration:    duration,
				Description: "Slow attack detected: long duration with few requests",
			}
		}
	}

	return nil
}

// handleAttack 处理攻击
func (ddp *DDoSProtector) handleAttack(attack AttackInfo) {
	atomic.AddInt64(&ddp.stats.AttacksDetected, 1)
	ddp.stats.LastAttack = time.Now()

	// 自动阻断
	if ddp.config.AutoBlockEnabled {
		blockDuration := ddp.config.BlockDuration

		// 渐进式阻断
		if ddp.config.ProgressiveBlocking {
			ddp.ipMutex.RLock()
			tracker := ddp.ipTrackers[attack.SourceIP]
			ddp.ipMutex.RUnlock()

			if tracker != nil {
				// 根据威胁级别调整阻断时间
				switch attack.ThreatLevel {
				case ThreatLevelCritical:
					blockDuration *= 4
				case ThreatLevelHigh:
					blockDuration *= 2
				case ThreatLevelMedium:
					blockDuration = blockDuration
				}
			}
		}

		ddp.BlockIP(attack.SourceIP, blockDuration)
	}

	// 调用攻击检测回调
	if ddp.onAttackDetected != nil {
		ddp.onAttackDetected(attack)
	}
}

// BlockIP 阻断IP
func (ddp *DDoSProtector) BlockIP(ip string, duration time.Duration) {
	ddp.listMutex.Lock()
	ddp.blacklist[ip] = time.Now().Add(duration)
	ddp.listMutex.Unlock()

	atomic.AddInt64(&ddp.stats.BlockedIPs, 1)

	// 调用IP阻断回调
	if ddp.onIPBlocked != nil {
		ddp.onIPBlocked(ip, duration)
	}
}

// UnblockIP 解除IP阻断
func (ddp *DDoSProtector) UnblockIP(ip string) {
	ddp.listMutex.Lock()
	delete(ddp.blacklist, ip)
	ddp.listMutex.Unlock()
}

// AddToWhitelist 添加到白名单
func (ddp *DDoSProtector) AddToWhitelist(ip string) {
	ddp.listMutex.Lock()
	ddp.whitelist[ip] = true
	ddp.listMutex.Unlock()
}

// RemoveFromWhitelist 从白名单移除
func (ddp *DDoSProtector) RemoveFromWhitelist(ip string) {
	ddp.listMutex.Lock()
	delete(ddp.whitelist, ip)
	ddp.listMutex.Unlock()
}

// cleanupLoop 清理循环
func (ddp *DDoSProtector) cleanupLoop() {
	defer ddp.wg.Done()

	ticker := time.NewTicker(ddp.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ddp.stopChan:
			return
		case <-ticker.C:
			ddp.cleanup()
		}
	}
}

// cleanup 清理过期数据
func (ddp *DDoSProtector) cleanup() {
	now := time.Now()

	// 清理过期的IP追踪器
	ddp.ipMutex.Lock()
	for ip, tracker := range ddp.ipTrackers {
		if now.Sub(tracker.LastRequest) > ddp.config.TrackerExpiry {
			delete(ddp.ipTrackers, ip)
		}
	}
	ddp.ipMutex.Unlock()

	// 清理过期的速率限制器
	ddp.rlMutex.Lock()
	for ip, limiter := range ddp.rateLimiters {
		if now.Sub(limiter.lastRefill) > ddp.config.TrackerExpiry {
			delete(ddp.rateLimiters, ip)
		}
	}
	ddp.rlMutex.Unlock()

	// 清理过期的黑名单
	ddp.listMutex.Lock()
	for ip, blockUntil := range ddp.blacklist {
		if now.After(blockUntil) {
			delete(ddp.blacklist, ip)
		}
	}
	ddp.listMutex.Unlock()
}

// monitorLoop 监控循环
func (ddp *DDoSProtector) monitorLoop() {
	defer ddp.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ddp.stopChan:
			return
		case <-ticker.C:
			ddp.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (ddp *DDoSProtector) updateStats() {
	ddp.connMutex.Lock()
	ddp.connTracker.LastUpdate = time.Now()

	// 计算活跃连接数
	ddp.ipMutex.RLock()
	activeConns := int64(len(ddp.ipTrackers))
	ddp.ipMutex.RUnlock()

	ddp.connTracker.ActiveConnections = activeConns
	atomic.StoreInt64(&ddp.stats.ActiveConnections, activeConns)
	ddp.connMutex.Unlock()
}

// GetStats 获取统计信息
func (ddp *DDoSProtector) GetStats() *DDoSStats {
	return ddp.stats
}

// GetIPTrackers 获取IP追踪器信息
func (ddp *DDoSProtector) GetIPTrackers() map[string]*IPTracker {
	ddp.ipMutex.RLock()
	defer ddp.ipMutex.RUnlock()

	result := make(map[string]*IPTracker)
	for ip, tracker := range ddp.ipTrackers {
		// 创建副本
		copy := *tracker
		result[ip] = &copy
	}

	return result
}

// SetEventHandlers 设置事件处理器
func (ddp *DDoSProtector) SetEventHandlers(
	onAttackDetected func(AttackInfo),
	onIPBlocked func(string, time.Duration),
	onThresholdExceeded func(string, string, int64),
) {
	ddp.onAttackDetected = onAttackDetected
	ddp.onIPBlocked = onIPBlocked
	ddp.onThresholdExceeded = onThresholdExceeded
}

// IsRunning 检查是否正在运行
func (ddp *DDoSProtector) IsRunning() bool {
	return atomic.LoadInt32(&ddp.running) == 1
}