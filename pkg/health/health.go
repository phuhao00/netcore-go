// Package health 健康检查实现
// Author: NetCore-Go Team
// Created: 2024

package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// CheckType 检查类型
type CheckType string

const (
	CheckTypeLiveness  CheckType = "liveness"
	CheckTypeReadiness CheckType = "readiness"
	CheckTypeStartup   CheckType = "startup"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthCheckResult
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Name         string       `json:"name"`
	Status       HealthStatus `json:"status"`
	Message      string       `json:"message,omitempty"`
	Error        string       `json:"error,omitempty"`
	Timestamp    time.Time    `json:"timestamp"`
	Duration     time.Duration `json:"duration"`
	ResponseTime int64        `json:"response_time_ms"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    HealthStatus                    `json:"status"`
	Timestamp time.Time                     `json:"timestamp"`
	Duration  time.Duration                 `json:"duration"`
	Checks    map[string]HealthCheckResult  `json:"checks"`
	Metadata  map[string]interface{}        `json:"metadata,omitempty"`
}

// HealthManager 健康管理器
type HealthManager struct {
	mu           sync.RWMutex
	config       *HealthConfig
	checkers     map[string]HealthChecker
	server       *http.Server
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	stats        *HealthStats
	lastResults  map[string]HealthCheckResult
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	// 服务器配置
	Host string `json:"host"`
	Port int    `json:"port"`
	Path string `json:"path"`

	// 检查配置
	Timeout         time.Duration `json:"timeout"`
	Interval        time.Duration `json:"interval"`
	StartupTimeout  time.Duration `json:"startup_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// 缓存配置
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`

	// 详细信息
	Verbose bool `json:"verbose"`
	IncludeMetadata bool `json:"include_metadata"`

	// 自定义端点
	CustomEndpoints map[string]string `json:"custom_endpoints"`
}

// HealthStats 健康检查统计
type HealthStats struct {
	TotalChecks         int64   `json:"total_checks"`
	HealthyChecks       int64   `json:"healthy_checks"`
	UnhealthyChecks     int64   `json:"unhealthy_checks"`
	DegradedChecks      int64   `json:"degraded_checks"`
	AverageLatency      int64   `json:"average_latency_ms"`
	AverageResponseTime float64 `json:"average_response_time_ms"`
	LastCheckTime       time.Time `json:"last_check_time"`
	Uptime              int64   `json:"uptime_seconds"`
	StartTime           int64   `json:"start_time"`
}

// DefaultHealthConfig 返回默认健康检查配置
func DefaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		Host:            "0.0.0.0",
		Port:            8081,
		Path:            "/health",
		Timeout:         5 * time.Second,
		Interval:        30 * time.Second,
		StartupTimeout:  60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		CacheEnabled:    true,
		CacheTTL:        10 * time.Second,
		Verbose:         false,
		IncludeMetadata: true,
		CustomEndpoints: make(map[string]string),
	}
}

// NewHealthManager 创建健康管理器
func NewHealthManager(config *HealthConfig) *HealthManager {
	if config == nil {
		config = DefaultHealthConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthManager{
		config:      config,
		checkers:    make(map[string]HealthChecker),
		ctx:         ctx,
		cancel:      cancel,
		stats:       &HealthStats{StartTime: time.Now().Unix()},
		lastResults: make(map[string]HealthCheckResult),
	}
}

// RegisterChecker 注册健康检查器
func (h *HealthManager) RegisterChecker(checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[checker.Name()] = checker
}

// UnregisterChecker 注销健康检查器
func (h *HealthManager) UnregisterChecker(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checkers, name)
	delete(h.lastResults, name)
}

// Start 启动健康检查服务
func (h *HealthManager) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return fmt.Errorf("health manager is already running")
	}

	// 创建HTTP服务器
	mux := http.NewServeMux()
	mux.HandleFunc(h.config.Path, h.handleHealth)
	mux.HandleFunc(h.config.Path+"/live", h.handleLiveness)
	mux.HandleFunc(h.config.Path+"/ready", h.handleReadiness)
	mux.HandleFunc(h.config.Path+"/startup", h.handleStartup)
	mux.HandleFunc(h.config.Path+"/stats", h.handleStats)

	// 注册自定义端点
	for path, handler := range h.config.CustomEndpoints {
		mux.HandleFunc(path, h.createCustomHandler(handler))
	}

	addr := fmt.Sprintf("%s:%d", h.config.Host, h.config.Port)
	h.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	h.running = true

	// 启动HTTP服务器
	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	// 启动定期检查
	if h.config.Interval > 0 {
		go h.periodicCheck()
	}

	return nil
}

// Stop 停止健康检查服务
func (h *HealthManager) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return nil
	}

	h.running = false
	h.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), h.config.ShutdownTimeout)
	defer cancel()

	return h.server.Shutdown(ctx)
}

// Check 执行所有健康检查
func (h *HealthManager) Check(ctx context.Context) HealthResponse {
	start := time.Now()

	h.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range h.checkers {
		checkers[name] = checker
	}
	h.mu.RUnlock()

	// 执行检查
	checks := make(map[string]HealthCheckResult)
	overallStatus := HealthStatusHealthy

	for name, checker := range checkers {
		// 检查缓存
		if h.config.CacheEnabled {
			h.mu.RLock()
			if lastResult, exists := h.lastResults[name]; exists {
				if time.Since(lastResult.Timestamp) < h.config.CacheTTL {
					checks[name] = lastResult
					h.mu.RUnlock()
					continue
				}
			}
			h.mu.RUnlock()
		}

		// 执行检查
		checkCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
		result := checker.Check(checkCtx)
		cancel()

		checks[name] = result

		// 缓存结果
		if h.config.CacheEnabled {
			h.mu.Lock()
			h.lastResults[name] = result
			h.mu.Unlock()
		}

		// 更新整体状态
		if result.Status == HealthStatusUnhealthy {
			overallStatus = HealthStatusUnhealthy
		} else if result.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	duration := time.Since(start)

	// 更新统计信息
	h.updateStats(overallStatus, duration)

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Duration:  duration,
		Checks:    checks,
	}

	// 添加元数据
	if h.config.IncludeMetadata {
		response.Metadata = map[string]interface{}{
			"version":    "1.0.0",
			"uptime":     time.Since(time.Unix(h.stats.StartTime, 0)).Seconds(),
			"timestamp":  time.Now().Unix(),
			"checks_count": len(checks),
		}
	}

	return response
}

// handleHealth 处理健康检查请求
func (h *HealthManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	response := h.Check(ctx)

	w.Header().Set("Content-Type", "application/json")

	// 设置HTTP状态码
	switch response.Status {
	case HealthStatusHealthy:
		w.WriteHeader(http.StatusOK)
	case HealthStatusDegraded:
		w.WriteHeader(http.StatusOK) // 降级状态仍返回200
	case HealthStatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleLiveness 处理存活检查
func (h *HealthManager) handleLiveness(w http.ResponseWriter, r *http.Request) {
	// 存活检查只检查服务是否运行
	if h.running {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","check":"liveness"}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","check":"liveness"}`))
	}
}

// handleReadiness 处理就绪检查
func (h *HealthManager) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	response := h.Check(ctx)

	w.Header().Set("Content-Type", "application/json")

	// 就绪检查要求所有检查都通过
	if response.Status == HealthStatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	readyResponse := map[string]interface{}{
		"status": response.Status,
		"check":  "readiness",
		"timestamp": response.Timestamp,
	}

	if h.config.Verbose {
		readyResponse["checks"] = response.Checks
	}

	json.NewEncoder(w).Encode(readyResponse)
}

// handleStartup 处理启动检查
func (h *HealthManager) handleStartup(w http.ResponseWriter, r *http.Request) {
	// 启动检查检查服务是否已完全启动
	uptime := time.Since(time.Unix(h.stats.StartTime, 0))

	if uptime < h.config.StartupTimeout {
		// 如果还在启动期内，执行检查
		ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
		defer cancel()

		response := h.Check(ctx)

		w.Header().Set("Content-Type", "application/json")

		if response.Status == HealthStatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		startupResponse := map[string]interface{}{
			"status": response.Status,
			"check":  "startup",
			"uptime": uptime.Seconds(),
			"timestamp": time.Now(),
		}

		json.NewEncoder(w).Encode(startupResponse)
	} else {
		// 启动期已过，认为已启动
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","check":"startup","message":"startup completed"}`))
	}
}

// handleStats 处理统计信息请求
func (h *HealthManager) handleStats(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	stats := *h.stats
	stats.Uptime = time.Since(time.Unix(h.stats.StartTime, 0)).Milliseconds() / 1000
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

// createCustomHandler 创建自定义处理器
func (h *HealthManager) createCustomHandler(handlerName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 这里可以根据handlerName执行不同的逻辑
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"handler": handlerName,
			"status":  "healthy",
			"timestamp": time.Now(),
		}
		json.NewEncoder(w).Encode(response)
	}
}

// periodicCheck 定期检查
func (h *HealthManager) periodicCheck() {
	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
			h.Check(ctx)
			cancel()
		}
	}
}

// updateStats 更新统计信息
func (h *HealthManager) updateStats(status HealthStatus, duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stats.TotalChecks++
	h.stats.LastCheckTime = time.Now()

	if status == HealthStatusHealthy {
		h.stats.HealthyChecks++
	} else {
		h.stats.UnhealthyChecks++
	}

	// 更新平均延迟
	if h.stats.TotalChecks == 1 {
		h.stats.AverageLatency = duration.Milliseconds()
	} else {
		h.stats.AverageLatency = (h.stats.AverageLatency + duration.Milliseconds()) / 2
	}
}

// GetStats 获取统计信息
func (h *HealthManager) GetStats() *HealthStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	stats := *h.stats
	stats.Uptime = time.Since(time.Unix(h.stats.StartTime, 0)).Milliseconds() / 1000
	return &stats
}

// IsRunning 检查是否运行
func (h *HealthManager) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// GetAddress 获取健康检查地址
func (h *HealthManager) GetAddress() string {
	return fmt.Sprintf("%s:%d", h.config.Host, h.config.Port)
}

// 内置健康检查器

// BasicHealthChecker 基础健康检查器
type BasicHealthChecker struct {
	name string
}

func NewBasicHealthChecker(name string) *BasicHealthChecker {
	return &BasicHealthChecker{name: name}
}

func (b *BasicHealthChecker) Name() string {
	return b.name
}

func (b *BasicHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	return HealthCheckResult{
		Status:    HealthStatusHealthy,
		Message:   "Service is running",
		Timestamp: start,
		Duration:  time.Since(start),
	}
}

// DatabaseHealthChecker 数据库健康检查器
type DatabaseHealthChecker struct {
	name string
	db   interface{} // 数据库连接接口
}

func NewDatabaseHealthChecker(name string, db interface{}) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		name: name,
		db:   db,
	}
}

func (d *DatabaseHealthChecker) Name() string {
	return d.name
}

func (d *DatabaseHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	// 这里应该实现实际的数据库连接检查
	// 为了示例，我们假设检查总是成功的
	return HealthCheckResult{
		Status:    HealthStatusHealthy,
		Message:   "Database connection is healthy",
		Timestamp: start,
		Duration:  time.Since(start),
		Metadata: map[string]interface{}{
			"connection_pool_size": 10,
			"active_connections":   5,
		},
	}
}