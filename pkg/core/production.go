// Package core 定义NetCore-Go网络库的生产就绪系统
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// ProductionConfig 生产环境配置
type ProductionConfig struct {
	// 服务配置
	ServiceName    string `json:"service_name"`
	Version        string `json:"version"`
	Environment    string `json:"environment"`
	InstanceID     string `json:"instance_id"`

	// 日志配置
	Logging *LoggerConfig `json:"logging"`

	// 健康检查配置
	HealthCheck *HealthCheckConfig `json:"health_check"`

	// 监控配置
	Metrics *MetricsConfig `json:"metrics"`

	// 错误处理配置
	ErrorHandling *ErrorHandlingConfig `json:"error_handling"`

	// 性能配置
	Performance *PerformanceConfig `json:"performance"`

	// 安全配置
	Security *SecurityConfig `json:"security"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled         bool          `json:"enabled"`
	Port            int           `json:"port"`
	Path            string        `json:"path"`
	Interval        time.Duration `json:"interval"`
	Timeout         time.Duration `json:"timeout"`
	StartupTimeout  time.Duration `json:"startup_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// MetricsConfig 监控配置
type MetricsConfig struct {
	Enabled    bool   `json:"enabled"`
	Port       int    `json:"port"`
	Path       string `json:"path"`
	Namespace  string `json:"namespace"`
	Subsystem  string `json:"subsystem"`
}

// ErrorHandlingConfig 错误处理配置
type ErrorHandlingConfig struct {
	Enabled           bool          `json:"enabled"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	CircuitBreaker    bool          `json:"circuit_breaker"`
	PanicRecovery     bool          `json:"panic_recovery"`
	ErrorReporting    bool          `json:"error_reporting"`
	SentryDSN         string        `json:"sentry_dsn,omitempty"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxCPUUsage    float64       `json:"max_cpu_usage"`
	MaxMemoryUsage int64         `json:"max_memory_usage"`
	GCPercent      int           `json:"gc_percent"`
	MaxGoroutines  int           `json:"max_goroutines"`
	Profiler       bool          `json:"profiler"`
	ProfilerPort   int           `json:"profiler_port"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	TLSEnabled     bool     `json:"tls_enabled"`
	CertFile       string   `json:"cert_file,omitempty"`
	KeyFile        string   `json:"key_file,omitempty"`
	AllowedOrigins []string `json:"allowed_origins"`
	RateLimit      int      `json:"rate_limit"`
	SecureHeaders  bool     `json:"secure_headers"`
}

// DefaultProductionConfig 返回默认生产环境配置
func DefaultProductionConfig(serviceName string) *ProductionConfig {
	return &ProductionConfig{
		ServiceName: serviceName,
		Version:     "1.0.0",
		Environment: "production",
		InstanceID:  generateInstanceID(),
		Logging:     ProductionLoggerConfig(serviceName),
		HealthCheck: &HealthCheckConfig{
			Enabled:         true,
			Port:            8081,
			Path:            "/health",
			Interval:        30 * time.Second,
			Timeout:         5 * time.Second,
			StartupTimeout:  60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Metrics: &MetricsConfig{
			Enabled:   true,
			Port:      8082,
			Path:      "/metrics",
			Namespace: "netcore",
			Subsystem: serviceName,
		},
		ErrorHandling: &ErrorHandlingConfig{
			Enabled:        true,
			MaxRetries:     3,
			RetryDelay:     time.Second,
			CircuitBreaker: true,
			PanicRecovery:  true,
			ErrorReporting: true,
		},
		Performance: &PerformanceConfig{
			MaxCPUUsage:    80.0,
			MaxMemoryUsage: 1024 * 1024 * 1024, // 1GB
			GCPercent:      100,
			MaxGoroutines:  10000,
			Profiler:       false,
			ProfilerPort:   6060,
		},
		Security: &SecurityConfig{
			TLSEnabled:     false,
			AllowedOrigins: []string{"*"},
			RateLimit:      1000,
			SecureHeaders:  true,
		},
	}
}

// generateInstanceID 生成实例ID
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	pid := os.Getpid()
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%d-%d", hostname, pid, timestamp)
}

// ProductionManager 生产环境管理器
type ProductionManager struct {
	mu       sync.RWMutex
	config   *ProductionConfig
	logger   *Logger
	validator *Validator
	errorHandler ErrorHandler
	metrics  *MetricsCollector
	healthChecker *HealthChecker
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	startTime time.Time
}

// NewProductionManager 创建生产环境管理器
func NewProductionManager(config *ProductionConfig) (*ProductionManager, error) {
	if config == nil {
		return nil, NewError(ErrCodeInvalidConfig, "production config is required")
	}

	// 验证配置
	if err := validateProductionConfig(config); err != nil {
		return nil, NewErrorWithCause(ErrCodeInvalidConfig, "invalid production config", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	pm := &ProductionManager{
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	// 初始化日志器
	if config.Logging != nil {
		pm.logger = NewLogger(config.Logging)
		SetGlobalLogger(pm.logger)
	} else {
		pm.logger = GetGlobalLogger()
	}

	// 初始化验证器
	pm.validator = NewValidator()
	pm.setupValidationRules()

	// 初始化错误处理器
	pm.errorHandler = NewDefaultErrorHandler()

	// 初始化监控
	if config.Metrics != nil && config.Metrics.Enabled {
		pm.metrics = NewMetricsCollector(config.Metrics)
	}

	// 初始化健康检查
	if config.HealthCheck != nil && config.HealthCheck.Enabled {
		pm.healthChecker = NewHealthChecker(config.HealthCheck)
	}

	return pm, nil
}

// validateProductionConfig 验证生产环境配置
func validateProductionConfig(config *ProductionConfig) error {
	if config.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if config.Version == "" {
		return fmt.Errorf("version is required")
	}
	if config.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	return nil
}

// setupValidationRules 设置验证规则
func (pm *ProductionManager) setupValidationRules() {
	// 添加通用验证规则
	pm.validator.AddRule("ServiceName", Required)
	pm.validator.AddRule("Version", Required)
	pm.validator.AddRule("Environment", Required)
	pm.validator.AddRule("Port", Port)
	pm.validator.AddRule("Host", IP)
}

// Start 启动生产环境管理器
func (pm *ProductionManager) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return NewError(ErrCodeServerRunning, "production manager is already running")
	}

	pm.logger.Info("Starting production manager")

	// 设置运行时参数
	pm.setupRuntime()

	// 启动健康检查
	if pm.healthChecker != nil {
		if err := pm.healthChecker.Start(); err != nil {
			return NewErrorWithCause(ErrCodeServerStartFailed, "failed to start health checker", err)
		}
		pm.logger.Infof("Health checker started on port %d", pm.config.HealthCheck.Port)
	}

	// 启动监控
	if pm.metrics != nil {
		if err := pm.metrics.Start(); err != nil {
			return NewErrorWithCause(ErrCodeServerStartFailed, "failed to start metrics collector", err)
		}
		pm.logger.Infof("Metrics collector started on port %d", pm.config.Metrics.Port)
	}

	// 启动性能监控
	if pm.config.Performance != nil {
		go pm.monitorPerformance()
	}

	// 设置panic恢复
	if pm.config.ErrorHandling != nil && pm.config.ErrorHandling.PanicRecovery {
		pm.setupPanicRecovery()
	}

	pm.running = true
	pm.logger.Infof("Production manager started successfully for service %s v%s", 
		pm.config.ServiceName, pm.config.Version)

	return nil
}

// Stop 停止生产环境管理器
func (pm *ProductionManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return NewError(ErrCodeServerNotRunning, "production manager is not running")
	}

	pm.logger.Info("Stopping production manager")

	// 停止健康检查
	if pm.healthChecker != nil {
		if err := pm.healthChecker.Stop(); err != nil {
			pm.logger.ErrorWithErr("Failed to stop health checker", err)
		}
	}

	// 停止监控
	if pm.metrics != nil {
		if err := pm.metrics.Stop(); err != nil {
			pm.logger.ErrorWithErr("Failed to stop metrics collector", err)
		}
	}

	// 取消上下文
	pm.cancel()

	// 关闭日志器
	if pm.logger != nil {
		pm.logger.Close()
	}

	pm.running = false
	pm.logger.Info("Production manager stopped")

	return nil
}

// setupRuntime 设置运行时参数
func (pm *ProductionManager) setupRuntime() {
	if pm.config.Performance == nil {
		return
	}

	// 设置GC百分比
	if pm.config.Performance.GCPercent > 0 {
		debug.SetGCPercent(pm.config.Performance.GCPercent)
		pm.logger.Infof("Set GC percent to %d", pm.config.Performance.GCPercent)
	}

	// 设置最大处理器数
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	pm.logger.Infof("Set GOMAXPROCS to %d", numCPU)
}

// monitorPerformance 监控性能
func (pm *ProductionManager) monitorPerformance() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.checkPerformance()
		case <-pm.ctx.Done():
			return
		}
	}
}

// checkPerformance 检查性能指标
func (pm *ProductionManager) checkPerformance() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 检查内存使用
	memoryUsage := int64(m.Alloc)
	if pm.config.Performance.MaxMemoryUsage > 0 && memoryUsage > pm.config.Performance.MaxMemoryUsage {
		pm.logger.Warnf("High memory usage: %d bytes (max: %d)", 
			memoryUsage, pm.config.Performance.MaxMemoryUsage)
	}

	// 检查Goroutine数量
	numGoroutines := runtime.NumGoroutine()
	if pm.config.Performance.MaxGoroutines > 0 && numGoroutines > pm.config.Performance.MaxGoroutines {
		pm.logger.Warnf("High goroutine count: %d (max: %d)", 
			numGoroutines, pm.config.Performance.MaxGoroutines)
	}

	// 记录性能指标
	if pm.metrics != nil {
		pm.metrics.RecordMemoryUsage(float64(memoryUsage))
		pm.metrics.RecordGoroutineCount(float64(numGoroutines))
	}
}

// setupPanicRecovery 设置panic恢复
func (pm *ProductionManager) setupPanicRecovery() {
	// 这里可以设置全局的panic恢复机制
	// 在实际应用中，通常在HTTP中间件或gRPC拦截器中实现
}

// GetLogger 获取日志器
func (pm *ProductionManager) GetLogger() *Logger {
	return pm.logger
}

// GetValidator 获取验证器
func (pm *ProductionManager) GetValidator() *Validator {
	return pm.validator
}

// GetErrorHandler 获取错误处理器
func (pm *ProductionManager) GetErrorHandler() ErrorHandler {
	return pm.errorHandler
}

// GetMetrics 获取监控收集器
func (pm *ProductionManager) GetMetrics() *MetricsCollector {
	return pm.metrics
}

// GetHealthChecker 获取健康检查器
func (pm *ProductionManager) GetHealthChecker() *HealthChecker {
	return pm.healthChecker
}

// IsRunning 检查是否正在运行
func (pm *ProductionManager) IsRunning() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.running
}

// GetConfig 获取配置
func (pm *ProductionManager) GetConfig() *ProductionConfig {
	return pm.config
}

// GetUptime 获取运行时间
func (pm *ProductionManager) GetUptime() time.Duration {
	return time.Since(pm.startTime)
}

// GetStatus 获取状态信息
func (pm *ProductionManager) GetStatus() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := map[string]interface{}{
		"service_name":     pm.config.ServiceName,
		"version":          pm.config.Version,
		"environment":      pm.config.Environment,
		"instance_id":      pm.config.InstanceID,
		"running":          pm.running,
		"uptime_seconds":   int64(pm.GetUptime().Seconds()),
		"start_time":       pm.startTime.Unix(),
		"memory_alloc":     m.Alloc,
		"memory_total":     m.TotalAlloc,
		"memory_sys":       m.Sys,
		"gc_runs":          m.NumGC,
		"goroutines":       runtime.NumGoroutine(),
		"cpu_count":        runtime.NumCPU(),
		"go_version":       runtime.Version(),
	}

	// 添加健康检查状态
	if pm.healthChecker != nil {
		status["health_check_enabled"] = true
		status["health_check_port"] = pm.config.HealthCheck.Port
	} else {
		status["health_check_enabled"] = false
	}

	// 添加监控状态
	if pm.metrics != nil {
		status["metrics_enabled"] = true
		status["metrics_port"] = pm.config.Metrics.Port
	} else {
		status["metrics_enabled"] = false
	}

	return status
}

// HandleHTTPError HTTP错误处理中间件
func (pm *ProductionManager) HandleHTTPError(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				pm.logger.WithField("panic", err).Error("HTTP handler panic")
				
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				
				errorResp := map[string]interface{}{
					"error": "Internal server error",
					"code":  ErrCodeInternal,
				}
				json.NewEncoder(w).Encode(errorResp)
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// LogHTTPRequest HTTP请求日志中间件
func (pm *ProductionManager) LogHTTPRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 创建响应写入器包装器来捕获状态码
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapper, r)
		
		duration := time.Since(start)
		
		pm.logger.WithFields(map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": wrapper.statusCode,
			"duration_ms": duration.Milliseconds(),
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).Info("HTTP request")
		
		// 记录监控指标
		if pm.metrics != nil {
			pm.metrics.RecordHTTPRequest(r.Method, r.URL.Path, wrapper.statusCode, duration)
		}
	})
}

// responseWriter 响应写入器包装器
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsCollector 监控收集器（简化版）
type MetricsCollector struct {
	config *MetricsConfig
	server *http.Server
	mu     sync.RWMutex
	metrics map[string]interface{}
}

// NewMetricsCollector 创建监控收集器
func NewMetricsCollector(config *MetricsConfig) *MetricsCollector {
	return &MetricsCollector{
		config:  config,
		metrics: make(map[string]interface{}),
	}
}

// Start 启动监控收集器
func (mc *MetricsCollector) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(mc.config.Path, mc.handleMetrics)
	
	addr := fmt.Sprintf(":%d", mc.config.Port)
	mc.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	go func() {
		if err := mc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()
	
	return nil
}

// Stop 停止监控收集器
func (mc *MetricsCollector) Stop() error {
	if mc.server != nil {
		return mc.server.Close()
	}
	return nil
}

// handleMetrics 处理监控请求
func (mc *MetricsCollector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mc.metrics)
}

// RecordMemoryUsage 记录内存使用
func (mc *MetricsCollector) RecordMemoryUsage(usage float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics["memory_usage_bytes"] = usage
}

// RecordGoroutineCount 记录Goroutine数量
func (mc *MetricsCollector) RecordGoroutineCount(count float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics["goroutine_count"] = count
}

// RecordHTTPRequest 记录HTTP请求
func (mc *MetricsCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	key := fmt.Sprintf("http_requests_%s_%s", method, path)
	if count, exists := mc.metrics[key]; exists {
		mc.metrics[key] = count.(float64) + 1
	} else {
		mc.metrics[key] = 1.0
	}
	
	durationKey := fmt.Sprintf("http_duration_%s_%s", method, path)
	mc.metrics[durationKey] = duration.Milliseconds()
}

// HealthChecker 健康检查器（简化版）
type HealthChecker struct {
	config *HealthCheckConfig
	server *http.Server
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config *HealthCheckConfig) *HealthChecker {
	return &HealthChecker{
		config: config,
	}
}

// Start 启动健康检查器
func (hc *HealthChecker) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(hc.config.Path, hc.handleHealth)
	mux.HandleFunc(hc.config.Path+"/live", hc.handleLiveness)
	mux.HandleFunc(hc.config.Path+"/ready", hc.handleReadiness)
	
	addr := fmt.Sprintf(":%d", hc.config.Port)
	hc.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	go func() {
		if err := hc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()
	
	return nil
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() error {
	if hc.server != nil {
		return hc.server.Close()
	}
	return nil
}

// handleHealth 处理健康检查
func (hc *HealthChecker) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"checks": map[string]interface{}{
			"service": map[string]interface{}{
				"status": "healthy",
			},
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

// handleLiveness 处理存活检查
func (hc *HealthChecker) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// handleReadiness 处理就绪检查
func (hc *HealthChecker) handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}