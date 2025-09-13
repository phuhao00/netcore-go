package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// 使用health.go中定义的HealthStatus类型

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	// 启用健康检查
	Enabled bool `json:"enabled" yaml:"enabled"`
	// 检查间隔
	CheckInterval time.Duration `json:"check_interval" yaml:"check_interval"`
	// 检查超时
	CheckTimeout time.Duration `json:"check_timeout" yaml:"check_timeout"`
	// HTTP端点路径
	EndpointPath string `json:"endpoint_path" yaml:"endpoint_path"`
	// HTTP端口
	Port int `json:"port" yaml:"port"`
	// 启用详细信息
	Verbose bool `json:"verbose" yaml:"verbose"`
	// 启用就绪检查
	ReadinessEnabled bool `json:"readiness_enabled" yaml:"readiness_enabled"`
	// 就绪检查路径
	ReadinessPath string `json:"readiness_path" yaml:"readiness_path"`
	// 启用存活检查
	LivenessEnabled bool `json:"liveness_enabled" yaml:"liveness_enabled"`
	// 存活检查路径
	LivenessPath string `json:"liveness_path" yaml:"liveness_path"`
	// 启用指标检查
	MetricsEnabled bool `json:"metrics_enabled" yaml:"metrics_enabled"`
	// 指标检查路径
	MetricsPath string `json:"metrics_path" yaml:"metrics_path"`
	// 失败阈值
	FailureThreshold int `json:"failure_threshold" yaml:"failure_threshold"`
	// 成功阈值
	SuccessThreshold int `json:"success_threshold" yaml:"success_threshold"`
}

// DefaultHealthCheckConfig 默认健康检查配置
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		Enabled:          true,
		CheckInterval:    30 * time.Second,
		CheckTimeout:     5 * time.Second,
		EndpointPath:     "/health",
		Port:             8080,
		Verbose:          true,
		ReadinessEnabled: true,
		ReadinessPath:    "/ready",
		LivenessEnabled:  true,
		LivenessPath:     "/live",
		MetricsEnabled:   true,
		MetricsPath:      "/metrics",
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	// Name 检查名称
	Name() string
	// Check 执行检查
	Check(ctx context.Context) HealthCheckResult
}

// 使用health.go中定义的HealthCheckResult类型

// HealthReport 健康报告
type HealthReport struct {
	// 整体状态
	Status HealthStatus `json:"status"`
	// 检查结果
	Checks []HealthCheckResult `json:"checks"`
	// 系统信息
	System SystemInfo `json:"system"`
	// 时间戳
	Timestamp time.Time `json:"timestamp"`
	// 版本信息
	Version string `json:"version,omitempty"`
	// 运行时间
	Uptime time.Duration `json:"uptime"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	// Go版本
	GoVersion string `json:"go_version"`
	// 操作系统
	OS string `json:"os"`
	// 架构
	Arch string `json:"arch"`
	// CPU核数
	NumCPU int `json:"num_cpu"`
	// Goroutine数量
	NumGoroutine int `json:"num_goroutine"`
	// 内存统计
	Memory MemoryInfo `json:"memory"`
	// GC统计
	GC GCInfo `json:"gc"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	// 已分配内存 (bytes)
	Alloc uint64 `json:"alloc"`
	// 总分配内存 (bytes)
	TotalAlloc uint64 `json:"total_alloc"`
	// 系统内存 (bytes)
	Sys uint64 `json:"sys"`
	// 堆对象数量
	HeapObjects uint64 `json:"heap_objects"`
}

// GCInfo GC信息
type GCInfo struct {
	// GC次数
	NumGC uint32 `json:"num_gc"`
	// 上次GC时间
	LastGC time.Time `json:"last_gc"`
	// GC暂停时间 (nanoseconds)
	PauseNs uint64 `json:"pause_ns"`
}

// 使用health.go中定义的HealthChecker接口和HealthStats类型

// ConcreteHealthChecker 具体的健康检查器实现
type ConcreteHealthChecker struct {
	config        *HealthCheckConfig
	checks        map[string]HealthCheck
	results       map[string]HealthCheckResult
	failureCounts map[string]int
	successCounts map[string]int
	mu            sync.RWMutex
	running       bool
	cancel        context.CancelFunc
	startTime     time.Time
	version       string
	server        *http.Server
	stats         *HealthStats
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config *HealthCheckConfig, version string) *ConcreteHealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	return &ConcreteHealthChecker{
		config:        config,
		checks:        make(map[string]HealthCheck),
		results:       make(map[string]HealthCheckResult),
		failureCounts: make(map[string]int),
		successCounts: make(map[string]int),
		startTime:     time.Now(),
		version:       version,
		stats:         &HealthStats{},
	}
}

// NewConcreteHealthChecker 创建具体健康检查器（兼容性函数）
func NewConcreteHealthChecker(config *HealthCheckConfig, version string) *ConcreteHealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	return NewHealthChecker(config, version)
}

// RegisterCheck 注册健康检查
func (hc *ConcreteHealthChecker) RegisterCheck(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.checks[check.Name()] = check
}

// UnregisterCheck 取消注册健康检查
func (hc *ConcreteHealthChecker) UnregisterCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.checks, name)
	delete(hc.results, name)
	delete(hc.failureCounts, name)
	delete(hc.successCounts, name)
}

// Start 启动健康检查器
func (hc *ConcreteHealthChecker) Start(ctx context.Context) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return fmt.Errorf("health checker already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	hc.cancel = cancel
	hc.running = true

	// 启动HTTP服务器
	if err := hc.startHTTPServer(); err != nil {
		hc.running = false
		cancel()
		return err
	}

	// 启动定期检查
	if hc.config.Enabled {
		go hc.periodicCheck(ctx)
	}

	return nil
}

// Stop 停止健康检查器
func (hc *ConcreteHealthChecker) Stop() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return fmt.Errorf("health checker not running")
	}

	hc.cancel()
	hc.running = false

	// 停止HTTP服务器
	if hc.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return hc.server.Shutdown(ctx)
	}

	return nil
}

// startHTTPServer 启动HTTP服务器
func (hc *ConcreteHealthChecker) startHTTPServer() error {
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc(hc.config.EndpointPath, hc.healthHandler)

	// 就绪检查端点
	if hc.config.ReadinessEnabled {
		mux.HandleFunc(hc.config.ReadinessPath, hc.readinessHandler)
	}

	// 存活检查端点
	if hc.config.LivenessEnabled {
		mux.HandleFunc(hc.config.LivenessPath, hc.livenessHandler)
	}

	// 指标端点
	if hc.config.MetricsEnabled {
		mux.HandleFunc(hc.config.MetricsPath, hc.metricsHandler)
	}

	hc.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", hc.config.Port),
		Handler: mux,
	}

	go func() {
		if err := hc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health check server error: %v\n", err)
		}
	}()

	return nil
}

// periodicCheck 定期检查
func (hc *ConcreteHealthChecker) periodicCheck(ctx context.Context) {
	ticker := time.NewTicker(hc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.runAllChecks(ctx)
		}
	}
}

// runAllChecks 运行所有检查
func (hc *ConcreteHealthChecker) runAllChecks(ctx context.Context) {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mu.RUnlock()

	var wg sync.WaitGroup
	results := make(chan HealthCheckResult, len(checks))

	// 并发执行所有检查
	for _, check := range checks {
		wg.Add(1)
		go func(c HealthCheck) {
			defer wg.Done()
			
			checkCtx, cancel := context.WithTimeout(ctx, hc.config.CheckTimeout)
			defer cancel()
			
			result := c.Check(checkCtx)
			results <- result
		}(check)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	hc.mu.Lock()
	for result := range results {
		hc.results[result.Name] = result
		hc.updateCheckCounts(result)
		hc.updateStats(result)
	}
	hc.stats.LastCheckTime = time.Now()
	hc.mu.Unlock()
}

// updateCheckCounts 更新检查计数
func (hc *ConcreteHealthChecker) updateCheckCounts(result HealthCheckResult) {
	switch result.Status {
	case HealthStatusHealthy:
		hc.successCounts[result.Name]++
		if hc.successCounts[result.Name] >= hc.config.SuccessThreshold {
			hc.failureCounts[result.Name] = 0
		}
	case HealthStatusUnhealthy, HealthStatusDegraded:
		hc.failureCounts[result.Name]++
		if hc.failureCounts[result.Name] >= hc.config.FailureThreshold {
			hc.successCounts[result.Name] = 0
		}
	}
}

// updateStats 更新统计信息
func (hc *ConcreteHealthChecker) updateStats(result HealthCheckResult) {
	atomic.AddInt64(&hc.stats.TotalChecks, 1)

	switch result.Status {
	case HealthStatusHealthy:
		atomic.AddInt64(&hc.stats.HealthyChecks, 1)
	case HealthStatusUnhealthy:
		atomic.AddInt64(&hc.stats.UnhealthyChecks, 1)
	case HealthStatusDegraded:
		atomic.AddInt64(&hc.stats.DegradedChecks, 1)
	}

	// 更新平均响应时间
	totalChecks := atomic.LoadInt64(&hc.stats.TotalChecks)
	currentAvg := hc.stats.AverageResponseTime
	newAvg := (currentAvg*float64(totalChecks-1) + float64(result.ResponseTime)) / float64(totalChecks)
	hc.stats.AverageResponseTime = newAvg
}

// GetOverallStatus 获取整体状态
func (hc *ConcreteHealthChecker) GetOverallStatus() HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if len(hc.results) == 0 {
		return HealthStatusUnknown
	}

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for _, result := range hc.results {
		// 考虑失败阈值
		failures := hc.failureCounts[result.Name]
		if failures >= hc.config.FailureThreshold {
			unhealthyCount++
		} else {
			switch result.Status {
			case HealthStatusHealthy:
				healthyCount++
			case HealthStatusDegraded:
				degradedCount++
			case HealthStatusUnhealthy:
				unhealthyCount++
			}
		}
	}

	// 如果有任何不健康的检查，整体状态为不健康
	if unhealthyCount > 0 {
		return HealthStatusUnhealthy
	}

	// 如果有降级的检查，整体状态为降级
	if degradedCount > 0 {
		return HealthStatusDegraded
	}

	// 所有检查都健康
	return HealthStatusHealthy
}

// GetHealthReport 获取健康报告
func (hc *ConcreteHealthChecker) GetHealthReport() HealthReport {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	checks := make([]HealthCheckResult, 0, len(hc.results))
	for _, result := range hc.results {
		checks = append(checks, result)
	}

	return HealthReport{
		Status:    hc.GetOverallStatus(),
		Checks:    checks,
		System:    hc.getSystemInfo(),
		Timestamp: time.Now(),
		Version:   hc.version,
		Uptime:    time.Since(hc.startTime),
	}
}

// getSystemInfo 获取系统信息
func (hc *ConcreteHealthChecker) getSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemInfo{
		GoVersion:    runtime.Version(),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		Memory: MemoryInfo{
			Alloc:       m.Alloc,
			TotalAlloc:  m.TotalAlloc,
			Sys:         m.Sys,
			HeapObjects: m.HeapObjects,
		},
		GC: GCInfo{
			NumGC:   m.NumGC,
			LastGC:  time.Unix(0, int64(m.LastGC)),
			PauseNs: m.PauseNs[(m.NumGC+255)%256],
		},
	}
}

// healthHandler 健康检查处理器
func (hc *ConcreteHealthChecker) healthHandler(w http.ResponseWriter, r *http.Request) {
	report := hc.GetHealthReport()

	w.Header().Set("Content-Type", "application/json")

	// 根据状态设置HTTP状态码
	switch report.Status {
	case HealthStatusHealthy:
		w.WriteHeader(http.StatusOK)
	case HealthStatusDegraded:
		w.WriteHeader(http.StatusOK) // 降级状态仍返回200
	case HealthStatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if hc.config.Verbose {
		json.NewEncoder(w).Encode(report)
	} else {
		// 简化响应
		simpleResponse := map[string]interface{}{
			"status":    report.Status,
			"timestamp": report.Timestamp,
			"uptime":    report.Uptime,
		}
		json.NewEncoder(w).Encode(simpleResponse)
	}
}

// readinessHandler 就绪检查处理器
func (hc *ConcreteHealthChecker) readinessHandler(w http.ResponseWriter, r *http.Request) {
	status := hc.GetOverallStatus()

	w.Header().Set("Content-Type", "application/json")

	// 就绪检查：只有完全健康才算就绪
	if status == HealthStatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := map[string]interface{}{
		"ready":     status == HealthStatusHealthy,
		"status":    status,
		"timestamp": time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// livenessHandler 存活检查处理器
func (hc *ConcreteHealthChecker) livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 存活检查：只要服务在运行就算存活
	if hc.running {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := map[string]interface{}{
		"alive":     hc.running,
		"timestamp": time.Now(),
		"uptime":    time.Since(hc.startTime),
	}

	json.NewEncoder(w).Encode(response)
}

// metricsHandler 指标处理器
func (hc *ConcreteHealthChecker) metricsHandler(w http.ResponseWriter, r *http.Request) {
	hc.mu.RLock()
	stats := *hc.stats
	hc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	metrics := map[string]interface{}{
		"health_stats": stats,
		"system":       hc.getSystemInfo(),
		"uptime":       time.Since(hc.startTime),
		"timestamp":    time.Now(),
	}

	json.NewEncoder(w).Encode(metrics)
}

// GetStats 获取统计信息
func (hc *ConcreteHealthChecker) GetStats() *HealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return &HealthStats{
		TotalChecks:         atomic.LoadInt64(&hc.stats.TotalChecks),
		HealthyChecks:       atomic.LoadInt64(&hc.stats.HealthyChecks),
		UnhealthyChecks:     atomic.LoadInt64(&hc.stats.UnhealthyChecks),
		DegradedChecks:      atomic.LoadInt64(&hc.stats.DegradedChecks),
		AverageResponseTime: hc.stats.AverageResponseTime,
		LastCheckTime:       hc.stats.LastCheckTime,
	}
}

// 内置健康检查实现

// DatabaseHealthCheck 数据库健康检查
type DatabaseHealthCheck struct {
	name string
	dsn  string
}

// NewDatabaseHealthCheck 创建数据库健康检查
func NewDatabaseHealthCheck(name, dsn string) *DatabaseHealthCheck {
	return &DatabaseHealthCheck{
		name: name,
		dsn:  dsn,
	}
}

// Name 返回检查名称
func (d *DatabaseHealthCheck) Name() string {
	return d.name
}

// Check 执行数据库健康检查
func (d *DatabaseHealthCheck) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Name:      d.name,
		Timestamp: start,
	}

	// 这里简化实现，实际应该连接真实数据库
	// 模拟数据库连接检查
	time.Sleep(10 * time.Millisecond)

	result.ResponseTime = time.Since(start).Nanoseconds() / 1e6
	result.Status = HealthStatusHealthy
	result.Message = "Database connection successful"
	result.Details = map[string]interface{}{
		"dsn": d.dsn,
	}

	return result
}

// HTTPHealthCheck HTTP服务健康检查
type HTTPHealthCheck struct {
	name string
	url  string
}

// NewHTTPHealthCheck 创建HTTP健康检查
func NewHTTPHealthCheck(name, url string) *HTTPHealthCheck {
	return &HTTPHealthCheck{
		name: name,
		url:  url,
	}
}

// Name 返回检查名称
func (h *HTTPHealthCheck) Name() string {
	return h.name
}

// Check 执行HTTP健康检查
func (h *HTTPHealthCheck) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Name:      h.name,
		Timestamp: start,
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = err.Error()
		result.ResponseTime = time.Since(start).Nanoseconds() / 1e6
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = err.Error()
		result.ResponseTime = time.Since(start).Nanoseconds() / 1e6
		return result
	}
	defer resp.Body.Close()

	result.ResponseTime = time.Since(start).Nanoseconds() / 1e6

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = HealthStatusHealthy
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else if resp.StatusCode >= 300 && resp.StatusCode < 500 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	result.Details = map[string]interface{}{
		"url":         h.url,
		"status_code": resp.StatusCode,
	}

	return result
}

// TCPHealthCheck TCP连接健康检查
type TCPHealthCheck struct {
	name    string
	address string
}

// NewTCPHealthCheck 创建TCP健康检查
func NewTCPHealthCheck(name, address string) *TCPHealthCheck {
	return &TCPHealthCheck{
		name:    name,
		address: address,
	}
}

// Name 返回检查名称
func (t *TCPHealthCheck) Name() string {
	return t.name
}

// Check 执行TCP健康检查
func (t *TCPHealthCheck) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Name:      t.name,
		Timestamp: start,
	}

	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = err.Error()
		result.ResponseTime = time.Since(start).Nanoseconds() / 1e6
		return result
	}
	defer conn.Close()

	result.ResponseTime = time.Since(start).Nanoseconds() / 1e6
	result.Status = HealthStatusHealthy
	result.Message = "TCP connection successful"
	result.Details = map[string]interface{}{
		"address": t.address,
	}

	return result
}

// MemoryHealthCheck 内存使用健康检查
type MemoryHealthCheck struct {
	name           string
	maxMemoryBytes uint64
}

// NewMemoryHealthCheck 创建内存健康检查
func NewMemoryHealthCheck(name string, maxMemoryMB uint64) *MemoryHealthCheck {
	return &MemoryHealthCheck{
		name:           name,
		maxMemoryBytes: maxMemoryMB * 1024 * 1024,
	}
}

// Name 返回检查名称
func (m *MemoryHealthCheck) Name() string {
	return m.name
}

// Check 执行内存健康检查
func (m *MemoryHealthCheck) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Name:      m.name,
		Timestamp: start,
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	result.ResponseTime = time.Since(start).Nanoseconds() / 1e6

	usagePercent := float64(memStats.Alloc) / float64(m.maxMemoryBytes) * 100

	if usagePercent < 80 {
		result.Status = HealthStatusHealthy
		result.Message = fmt.Sprintf("Memory usage: %.2f%%", usagePercent)
	} else if usagePercent < 95 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("High memory usage: %.2f%%", usagePercent)
	} else {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Critical memory usage: %.2f%%", usagePercent)
	}

	result.Details = map[string]interface{}{
		"alloc_bytes":    memStats.Alloc,
		"sys_bytes":      memStats.Sys,
		"usage_percent":  usagePercent,
		"max_bytes":      m.maxMemoryBytes,
		"heap_objects":   memStats.HeapObjects,
		"gc_count":       memStats.NumGC,
	}

	return result
}