package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/netcore-go/pkg/config"
	"github.com/netcore-go/pkg/discovery"
	"github.com/netcore-go/pkg/loadbalancer"
	"github.com/netcore-go/pkg/logger"
	"github.com/netcore-go/pkg/metrics"
)

// Route 路由配置
type Route struct {
	Path        string                    `json:"path" yaml:"path"`               // 路径匹配
	Method      string                    `json:"method" yaml:"method"`           // HTTP方法
	Service     string                    `json:"service" yaml:"service"`         // 目标服务
	Rewrite     string                    `json:"rewrite" yaml:"rewrite"`         // 路径重写
	Timeout     time.Duration             `json:"timeout" yaml:"timeout"`         // 超时时间
	Retries     int                       `json:"retries" yaml:"retries"`         // 重试次数
	LoadBalance loadbalancer.Algorithm    `json:"load_balance" yaml:"load_balance"` // 负载均衡算法
	Middleware  []string                  `json:"middleware" yaml:"middleware"`   // 中间件列表
	Headers     map[string]string         `json:"headers" yaml:"headers"`         // 添加的请求头
	Auth        *AuthConfig               `json:"auth" yaml:"auth"`               // 认证配置
	RateLimit   *RateLimitConfig          `json:"rate_limit" yaml:"rate_limit"`   // 限流配置
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Type    string   `json:"type" yaml:"type"` // jwt, basic, apikey
	Secret  string   `json:"secret" yaml:"secret"`
	Header  string   `json:"header" yaml:"header"`
	Roles   []string `json:"roles" yaml:"roles"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled bool          `json:"enabled" yaml:"enabled"`
	Rate    int           `json:"rate" yaml:"rate"`       // 每秒请求数
	Burst   int           `json:"burst" yaml:"burst"`     // 突发请求数
	Window  time.Duration `json:"window" yaml:"window"`   // 时间窗口
	Key     string        `json:"key" yaml:"key"`         // 限流键（ip, user, api_key）
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	Server    ServerConfig              `json:"server" yaml:"server"`
	Discovery DiscoveryConfig           `json:"discovery" yaml:"discovery"`
	Routes    []Route                   `json:"routes" yaml:"routes"`
	CORS      CORSConfig                `json:"cors" yaml:"cors"`
	Security  SecurityConfig            `json:"security" yaml:"security"`
	Metrics   MetricsConfig             `json:"metrics" yaml:"metrics"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `json:"host" yaml:"host"`
	Port         int           `json:"port" yaml:"port"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
}

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	Enabled   bool          `json:"enabled" yaml:"enabled"`
	Provider  string        `json:"provider" yaml:"provider"`
	Endpoints []string      `json:"endpoints" yaml:"endpoints"`
	Timeout   time.Duration `json:"timeout" yaml:"timeout"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	Enabled          bool     `json:"enabled" yaml:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers" yaml:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers" yaml:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `json:"max_age" yaml:"max_age"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	Enabled           bool     `json:"enabled" yaml:"enabled"`
	TrustedProxies    []string `json:"trusted_proxies" yaml:"trusted_proxies"`
	HideServerHeader  bool     `json:"hide_server_header" yaml:"hide_server_header"`
	ContentTypeNosniff bool     `json:"content_type_nosniff" yaml:"content_type_nosniff"`
	FrameOptions      string   `json:"frame_options" yaml:"frame_options"`
	ContentSecurityPolicy string `json:"content_security_policy" yaml:"content_security_policy"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Path    string `json:"path" yaml:"path"`
	Port    int    `json:"port" yaml:"port"`
}

// Gateway API网关
type Gateway struct {
	config          *GatewayConfig
	serviceRegistry discovery.ServiceClient
	lbManager       *loadbalancer.Manager
	routes          map[string]*Route
	server          *http.Server
	mu              sync.RWMutex
	
	// 指标
	requestCounter   *metrics.Counter
	requestDuration  *metrics.Histogram
	errorCounter     *metrics.Counter
	serviceCounter   *metrics.Counter
}

// NewGateway 创建API网关
func NewGateway(config *GatewayConfig) *Gateway {
	if config == nil {
		config = getDefaultConfig()
	}
	
	gw := &Gateway{
		config: config,
		routes: make(map[string]*Route),
		lbManager: loadbalancer.NewManager(&loadbalancer.Config{
			Algorithm:   loadbalancer.RoundRobin,
			HealthCheck: true,
			MaxRetries:  3,
		}),
	}
	
	// 初始化服务发现
	if config.Discovery.Enabled {
		gw.serviceRegistry = discovery.NewMemoryClient()
	}
	
	// 初始化指标
	gw.initMetrics()
	
	// 加载路由
	gw.loadRoutes()
	
	return gw
}

// initMetrics 初始化指标
func (gw *Gateway) initMetrics() {
	gw.requestCounter = metrics.RegisterCounter(
		"gateway_requests_total",
		"Total number of requests",
		metrics.Labels{"method": "", "path": "", "status": ""},
	)
	
	gw.requestDuration = metrics.RegisterHistogram(
		"gateway_request_duration_seconds",
		"Request duration in seconds",
		metrics.Labels{"method": "", "path": ""},
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	)
	
	gw.errorCounter = metrics.RegisterCounter(
		"gateway_errors_total",
		"Total number of errors",
		metrics.Labels{"type": "", "service": ""},
	)
	
	gw.serviceCounter = metrics.RegisterCounter(
		"gateway_service_requests_total",
		"Total number of service requests",
		metrics.Labels{"service": "", "status": ""},
	)
}

// loadRoutes 加载路由
func (gw *Gateway) loadRoutes() {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	
	for i := range gw.config.Routes {
		route := &gw.config.Routes[i]
		key := fmt.Sprintf("%s:%s", route.Method, route.Path)
		gw.routes[key] = route
	}
}

// Start 启动网关
func (gw *Gateway) Start() error {
	// 创建HTTP处理器
	mux := http.NewServeMux()
	
	// 健康检查端点
	mux.HandleFunc("/health", gw.handleHealth)
	
	// 网关信息端点
	mux.HandleFunc("/gateway/info", gw.handleGatewayInfo)
	mux.HandleFunc("/gateway/routes", gw.handleRoutes)
	mux.HandleFunc("/gateway/services", gw.handleServices)
	
	// 指标端点
	if gw.config.Metrics.Enabled {
		mux.Handle(gw.config.Metrics.Path, metrics.PrometheusHandler(nil))
	}
	
	// 主要的代理处理器
	mux.HandleFunc("/", gw.handleProxy)
	
	// 应用中间件
	handler := gw.applyMiddleware(mux)
	
	// 创建服务器
	addr := fmt.Sprintf("%s:%d", gw.config.Server.Host, gw.config.Server.Port)
	gw.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  gw.config.Server.ReadTimeout,
		WriteTimeout: gw.config.Server.WriteTimeout,
		IdleTimeout:  gw.config.Server.IdleTimeout,
	}
	
	logger.Infof("API网关启动在 %s", addr)
	return gw.server.ListenAndServe()
}

// Stop 停止网关
func (gw *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// 关闭服务发现
	if gw.serviceRegistry != nil {
		gw.serviceRegistry.Close()
	}
	
	// 关闭负载均衡管理器
	if gw.lbManager != nil {
		gw.lbManager.Close()
	}
	
	return gw.server.Shutdown(ctx)
}

// applyMiddleware 应用中间件
func (gw *Gateway) applyMiddleware(handler http.Handler) http.Handler {
	// CORS中间件
	if gw.config.CORS.Enabled {
		handler = gw.corsMiddleware(handler)
	}
	
	// 安全中间件
	if gw.config.Security.Enabled {
		handler = gw.securityMiddleware(handler)
	}
	
	// 日志中间件
	handler = gw.loggingMiddleware(handler)
	
	// 指标中间件
	handler = gw.metricsMiddleware(handler)
	
	return handler
}

// handleProxy 处理代理请求
func (gw *Gateway) handleProxy(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// 查找匹配的路由
	route := gw.findRoute(r)
	if route == nil {
		gw.writeError(w, http.StatusNotFound, "Route not found")
		return
	}
	
	// 获取服务实例
	instances, err := gw.getServiceInstances(route.Service)
	if err != nil {
		logger.WithError(err).Errorf("获取服务实例失败: %s", route.Service)
		gw.writeError(w, http.StatusServiceUnavailable, "Service unavailable")
		return
	}
	
	if len(instances) == 0 {
		gw.writeError(w, http.StatusServiceUnavailable, "No available instances")
		return
	}
	
	// 负载均衡选择实例
	instance, err := gw.lbManager.Select(r.Context(), instances, route.LoadBalance)
	if err != nil {
		logger.WithError(err).Error("负载均衡选择失败")
		gw.writeError(w, http.StatusServiceUnavailable, "Load balancer error")
		return
	}
	
	// 应用路由中间件
	if err := gw.applyRouteMiddleware(w, r, route); err != nil {
		return
	}
	
	// 执行代理请求
	gw.proxyRequest(w, r, route, instance, start)
}

// findRoute 查找匹配的路由
func (gw *Gateway) findRoute(r *http.Request) *Route {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	
	// 精确匹配
	key := fmt.Sprintf("%s:%s", r.Method, r.URL.Path)
	if route, exists := gw.routes[key]; exists {
		return route
	}
	
	// 前缀匹配
	for _, route := range gw.routes {
		if route.Method == r.Method || route.Method == "*" {
			if strings.HasPrefix(r.URL.Path, route.Path) {
				return route
			}
		}
	}
	
	return nil
}

// getServiceInstances 获取服务实例
func (gw *Gateway) getServiceInstances(serviceName string) ([]*discovery.ServiceInstance, error) {
	if gw.serviceRegistry == nil {
		// 如果没有服务发现，返回静态配置的实例
		return gw.getStaticInstances(serviceName), nil
	}
	
	return gw.serviceRegistry.GetHealthyServices(context.Background(), serviceName)
}

// getStaticInstances 获取静态配置的实例
func (gw *Gateway) getStaticInstances(serviceName string) []*discovery.ServiceInstance {
	// 这里可以从配置文件或环境变量读取静态服务实例
	// 简化实现，返回默认实例
	switch serviceName {
	case "user-service":
		return []*discovery.ServiceInstance{
			{ID: "user-1", Name: "user-service", Address: "localhost", Port: 8081, Health: discovery.Healthy},
		}
	case "order-service":
		return []*discovery.ServiceInstance{
			{ID: "order-1", Name: "order-service", Address: "localhost", Port: 8082, Health: discovery.Healthy},
		}
	case "payment-service":
		return []*discovery.ServiceInstance{
			{ID: "payment-1", Name: "payment-service", Address: "localhost", Port: 8083, Health: discovery.Healthy},
		}
	default:
		return nil
	}
}

// applyRouteMiddleware 应用路由中间件
func (gw *Gateway) applyRouteMiddleware(w http.ResponseWriter, r *http.Request, route *Route) error {
	// 认证中间件
	if route.Auth != nil && route.Auth.Enabled {
		if err := gw.authMiddleware(w, r, route.Auth); err != nil {
			return err
		}
	}
	
	// 限流中间件
	if route.RateLimit != nil && route.RateLimit.Enabled {
		if err := gw.rateLimitMiddleware(w, r, route.RateLimit); err != nil {
			return err
		}
	}
	
	return nil
}

// proxyRequest 执行代理请求
func (gw *Gateway) proxyRequest(w http.ResponseWriter, r *http.Request, route *Route, instance *discovery.ServiceInstance, start time.Time) {
	// 构建目标URL
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", instance.Address, instance.Port),
	}
	
	// 路径重写
	path := r.URL.Path
	if route.Rewrite != "" {
		path = strings.Replace(path, route.Path, route.Rewrite, 1)
	}
	targetURL.Path = path
	targetURL.RawQuery = r.URL.RawQuery
	
	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	
	// 自定义Director
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// 添加自定义头部
		for key, value := range route.Headers {
			req.Header.Set(key, value)
		}
		
		// 添加网关信息
		req.Header.Set("X-Gateway", "NetCore-Go")
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Request-ID", gw.generateRequestID())
	}
	
	// 自定义错误处理
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.WithError(err).Errorf("代理请求失败: %s", targetURL.String())
		gw.errorCounter.Inc()
		gw.lbManager.UpdateStats(instance.ID, false, time.Since(start))
		gw.writeError(w, http.StatusBadGateway, "Proxy error")
	}
	
	// 自定义响应修改
	proxy.ModifyResponse = func(resp *http.Response) error {
		// 记录成功的请求
		success := resp.StatusCode < 500
		gw.lbManager.UpdateStats(instance.ID, success, time.Since(start))
		
		// 更新服务计数器
		gw.serviceCounter.Inc()
		
		// 添加响应头
		resp.Header.Set("X-Gateway", "NetCore-Go")
		resp.Header.Set("X-Service-Instance", instance.ID)
		
		return nil
	}
	
	// 设置超时
	if route.Timeout > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), route.Timeout)
		defer cancel()
		r = r.WithContext(ctx)
	}
	
	// 执行代理
	proxy.ServeHTTP(w, r)
}

// 中间件实现

// corsMiddleware CORS中间件
func (gw *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	cors := gw.config.CORS
	
		// 设置CORS头部
		if len(cors.AllowedOrigins) > 0 {
			w.Header().Set("Access-Control-Allow-Origin", strings.Join(cors.AllowedOrigins, ","))
		}
		if len(cors.AllowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(cors.AllowedMethods, ","))
		}
		if len(cors.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(cors.AllowedHeaders, ","))
		}
		if len(cors.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(cors.ExposedHeaders, ","))
		}
		if cors.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if cors.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cors.MaxAge))
		}
		
		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// securityMiddleware 安全中间件
func (gw *Gateway) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		security := gw.config.Security
		
		// 隐藏服务器头部
		if security.HideServerHeader {
			w.Header().Set("Server", "")
		}
		
		// 内容类型嗅探保护
		if security.ContentTypeNosniff {
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		
		// 框架选项
		if security.FrameOptions != "" {
			w.Header().Set("X-Frame-Options", security.FrameOptions)
		}
		
		// 内容安全策略
		if security.ContentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", security.ContentSecurityPolicy)
		}
		
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 日志中间件
func (gw *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 包装ResponseWriter以捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		next.ServeHTTP(wrapped, r)
		
		// 记录请求日志
		logger.WithFields(map[string]interface{}{
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     wrapped.statusCode,
			"duration":   time.Since(start),
			"remote_addr": r.RemoteAddr,
			"user_agent": r.UserAgent(),
		}).Info("HTTP请求")
	})
}

// metricsMiddleware 指标中间件
func (gw *Gateway) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 包装ResponseWriter以捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		next.ServeHTTP(wrapped, r)
		
		// 更新指标
		duration := time.Since(start).Seconds()
		gw.requestDuration.Observe(duration)
		gw.requestCounter.Inc()
	})
}

// authMiddleware 认证中间件
func (gw *Gateway) authMiddleware(w http.ResponseWriter, r *http.Request, auth *AuthConfig) error {
	// 简化的认证实现
	header := auth.Header
	if header == "" {
		header = "Authorization"
	}
	
	token := r.Header.Get(header)
	if token == "" {
		gw.writeError(w, http.StatusUnauthorized, "Missing authentication token")
		return fmt.Errorf("missing token")
	}
	
	// 这里应该实现实际的token验证逻辑
	// 简化实现，只检查token是否等于配置的secret
	if auth.Type == "bearer" && token != "Bearer "+auth.Secret {
		gw.writeError(w, http.StatusUnauthorized, "Invalid token")
		return fmt.Errorf("invalid token")
	}
	
	return nil
}

// rateLimitMiddleware 限流中间件
func (gw *Gateway) rateLimitMiddleware(w http.ResponseWriter, r *http.Request, rateLimit *RateLimitConfig) error {
	// 简化的限流实现
	// 实际应用中应该使用Redis或内存存储来实现分布式限流
	
	// 获取限流键
	key := gw.getRateLimitKey(r, rateLimit.Key)
	
	// 这里应该实现实际的限流逻辑
	// 简化实现，总是允许通过
	_ = key
	
	return nil
}

// getRateLimitKey 获取限流键
func (gw *Gateway) getRateLimitKey(r *http.Request, keyType string) string {
	switch keyType {
	case "ip":
		return r.RemoteAddr
	case "user":
		return r.Header.Get("X-User-ID")
	case "api_key":
		return r.Header.Get("X-API-Key")
	default:
		return r.RemoteAddr
	}
}

// 处理器实现

// handleHealth 健康检查处理器
func (gw *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now(),
	})
}

// handleGatewayInfo 网关信息处理器
func (gw *Gateway) handleGatewayInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    "NetCore-Go API Gateway",
		"version": "1.0.0",
		"routes":  len(gw.routes),
		"uptime":  time.Since(time.Now()).String(),
	})
}

// handleRoutes 路由列表处理器
func (gw *Gateway) handleRoutes(w http.ResponseWriter, r *http.Request) {
	gw.mu.RLock()
	routes := make([]map[string]interface{}, 0, len(gw.routes))
	for _, route := range gw.routes {
		routes = append(routes, map[string]interface{}{
			"path":         route.Path,
			"method":       route.Method,
			"service":      route.Service,
			"load_balance": route.LoadBalance,
		})
	}
	gw.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"routes": routes,
	})
}

// handleServices 服务列表处理器
func (gw *Gateway) handleServices(w http.ResponseWriter, r *http.Request) {
	services := make(map[string]interface{})
	
	if gw.serviceRegistry != nil {
		serviceNames, _ := gw.serviceRegistry.ListServices(context.Background())
		for _, name := range serviceNames {
			instances, _ := gw.serviceRegistry.GetService(context.Background(), name)
			services[name] = map[string]interface{}{
				"instances": len(instances),
				"healthy":   len(gw.getHealthyInstances(instances)),
			}
		}
	} else {
		// 静态服务列表
		services["user-service"] = map[string]interface{}{"instances": 1, "healthy": 1}
		services["order-service"] = map[string]interface{}{"instances": 1, "healthy": 1}
		services["payment-service"] = map[string]interface{}{"instances": 1, "healthy": 1}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
	})
}

// 辅助函数

// getHealthyInstances 获取健康实例
func (gw *Gateway) getHealthyInstances(instances []*discovery.ServiceInstance) []*discovery.ServiceInstance {
	var healthy []*discovery.ServiceInstance
	for _, instance := range instances {
		if instance.IsHealthy() {
			healthy = append(healthy, instance)
		}
	}
	return healthy
}

// generateRequestID 生成请求ID
func (gw *Gateway) generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// writeError 写入错误响应
func (gw *Gateway) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"status":  status,
		"timestamp": time.Now(),
	})
}

// responseWriter 包装ResponseWriter以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *GatewayConfig {
	return &GatewayConfig{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Discovery: DiscoveryConfig{
			Enabled: false,
		},
		Routes: []Route{
			{
				Path:        "/api/users",
				Method:      "*",
				Service:     "user-service",
				Timeout:     10 * time.Second,
				Retries:     3,
				LoadBalance: loadbalancer.RoundRobin,
			},
			{
				Path:        "/api/orders",
				Method:      "*",
				Service:     "order-service",
				Timeout:     10 * time.Second,
				Retries:     3,
				LoadBalance: loadbalancer.RoundRobin,
			},
			{
				Path:        "/api/payments",
				Method:      "*",
				Service:     "payment-service",
				Timeout:     10 * time.Second,
				Retries:     3,
				LoadBalance: loadbalancer.RoundRobin,
			},
		},
		CORS: CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
			MaxAge:         3600,
		},
		Security: SecurityConfig{
			Enabled:               true,
			HideServerHeader:      true,
			ContentTypeNosniff:    true,
			FrameOptions:          "DENY",
			ContentSecurityPolicy: "default-src 'self'",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    9090,
		},
	}
}

func main() {
	// 初始化日志
	logger.SetGlobalLogger(logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: logger.NewJSONFormatter(),
		Caller:    true,
	}))
	
	// 加载配置
	var gatewayConfig *GatewayConfig
	cfg, err := config.AutoLoadConfig()
	if err != nil {
		logger.WithError(err).Warn("加载配置失败，使用默认配置")
		gatewayConfig = getDefaultConfig()
	} else {
		if err := cfg.Unmarshal(&gatewayConfig); err != nil {
			logger.WithError(err).Warn("解析配置失败，使用默认配置")
			gatewayConfig = getDefaultConfig()
		}
	}
	
	// 启动系统指标收集
	metrics.StartSystemCollector(&metrics.SystemCollectorConfig{
		Enabled:         true,
		CollectInterval: 10 * time.Second,
		Namespace:       "gateway",
	})
	defer metrics.StopSystemCollector()
	
	// 启动Prometheus导出器
	if gatewayConfig.Metrics.Enabled {
		go func() {
			addr := fmt.Sprintf(":%d", gatewayConfig.Metrics.Port)
			if err := metrics.StartPrometheusExporter(addr); err != nil {
				logger.WithError(err).Error("Prometheus导出器启动失败")
			}
		}()
		defer metrics.StopPrometheusExporter()
	}
	
	// 创建API网关
	gateway := NewGateway(gatewayConfig)
	
	// 启动网关
	go func() {
		if err := gateway.Start(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("网关启动失败")
		}
	}()
	
	logger.Info("API网关已启动")
	logger.Infof("网关地址: http://%s:%d", gatewayConfig.Server.Host, gatewayConfig.Server.Port)
	logger.Info("管理端点:")
	logger.Info("  - 健康检查: /health")
	logger.Info("  - 网关信息: /gateway/info")
	logger.Info("  - 路由列表: /gateway/routes")
	logger.Info("  - 服务列表: /gateway/services")
	if gatewayConfig.Metrics.Enabled {
		logger.Infof("  - 指标端点: http://localhost:%d%s", gatewayConfig.Metrics.Port, gatewayConfig.Metrics.Path)
	}
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	logger.Info("正在关闭网关...")
	
	// 优雅关闭
	if err := gateway.Stop(); err != nil {
		logger.WithError(err).Error("网关关闭失败")
	} else {
		logger.Info("网关已关闭")
	}
}