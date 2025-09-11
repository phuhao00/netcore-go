package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netcore-go/pkg/alert"
	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/health"
	"github.com/netcore-go/pkg/loadbalancer"
	"github.com/netcore-go/pkg/logger"
	"github.com/netcore-go/pkg/performance"
	"github.com/netcore-go/pkg/security"
	"github.com/netcore-go/pkg/tracing"
)

// AdvancedServer 高级服务器示例
type AdvancedServer struct {
	server           *core.Server
	logger           *logger.Logger
	healthChecker    *health.HealthChecker
	alertEngine      *alert.AlertEngine
	tracerProvider   *tracing.TracerProvider
	loadBalancer     *loadbalancer.SmartLoadBalancer
	connectionPool   *performance.ConnectionPool
	memoryManager    *performance.MemoryManager
	zeroCopyManager  *performance.ZeroCopyManager
	tlsManager       *security.TLSManager
	authManager      *security.AuthManager
	auditLogger      *security.AuditLogger
	ddosProtector    *security.DDoSProtector
}

// NewAdvancedServer 创建高级服务器
func NewAdvancedServer() *AdvancedServer {
	// 创建日志器
	loggerConfig := logger.DefaultConfig()
	loggerConfig.Level = "info"
	loggerConfig.Format = "json"
	logger := logger.New(loggerConfig)

	// 创建核心服务器
	serverConfig := &core.Config{
		Host:           "localhost",
		Port:           8080,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxConnections: 1000,
	}
	server := core.NewServer(serverConfig)

	// 创建健康检查器
	healthConfig := health.DefaultHealthCheckConfig()
	healthConfig.Port = 8081
	healthChecker := health.NewHealthChecker(healthConfig, "1.0.0")

	// 注册健康检查
	healthChecker.RegisterCheck(health.NewHTTPHealthCheck("self", "http://localhost:8080/ping"))
	healthChecker.RegisterCheck(health.NewMemoryHealthCheck("memory", 1024)) // 1GB限制

	// 创建追踪器
	tracingConfig := tracing.DefaultTracingConfig()
	tracingConfig.ServiceName = "advanced-netcore-server"
	exporter := tracing.NewConsoleExporter()
	tracerProvider := tracing.NewTracerProvider(tracingConfig, exporter)

	// 创建告警引擎
	alertConfig := alert.DefaultAlertEngineConfig()
	alertEngine := alert.NewAlertEngine(alertConfig, nil)
	alertEngine.RegisterNotifier(alert.NewConsoleNotifier())

	// 创建负载均衡器
	lbConfig := loadbalancer.DefaultSmartLoadBalancerConfig()
	lbConfig.Strategy = loadbalancer.AdaptiveLoadBalancing
	loadBalancer := loadbalancer.NewSmartLoadBalancer(lbConfig)

	// 添加后端服务器
	backend1 := loadbalancer.NewBackend("backend1", "localhost:8082", 100)
	backend2 := loadbalancer.NewBackend("backend2", "localhost:8083", 100)
	loadBalancer.AddBackend(backend1)
	loadBalancer.AddBackend(backend2)

	// 创建连接池
	connPoolConfig := performance.DefaultConnectionReuseConfig()
	connectionPool := performance.NewConnectionPool(connPoolConfig)

	// 创建内存管理器
	memoryConfig := performance.DefaultMemoryConfig()
	memoryManager := performance.NewMemoryManager(memoryConfig)

	// 创建零拷贝管理器
	zeroCopyConfig := performance.DefaultZeroCopyConfig()
	zeroCopyManager := performance.NewZeroCopyManager(zeroCopyConfig)

	// 创建TLS管理器
	tlsConfig := security.DefaultTLSConfig()
	tlsManager := security.NewTLSManager(tlsConfig)

	// 创建认证管理器
	authConfig := security.DefaultAuthConfig()
	authManager := security.NewAuthManager(authConfig)

	// 创建审计日志器
	auditConfig := security.DefaultAuditConfig()
	auditLogger := security.NewAuditLogger(auditConfig)

	// 创建DDoS防护器
	ddosConfig := security.DefaultDDoSConfig()
	ddosProtector := security.NewDDoSProtector(ddosConfig)

	return &AdvancedServer{
		server:          server,
		logger:          logger,
		healthChecker:   healthChecker,
		alertEngine:     alertEngine,
		tracerProvider:  tracerProvider,
		loadBalancer:    loadBalancer,
		connectionPool:  connectionPool,
		memoryManager:   memoryManager,
		zeroCopyManager: zeroCopyManager,
		tlsManager:      tlsManager,
		authManager:     authManager,
		auditLogger:     auditLogger,
		ddosProtector:   ddosProtector,
	}
}

// Start 启动服务器
func (as *AdvancedServer) Start(ctx context.Context) error {
	as.logger.Info("Starting advanced NetCore server...")

	// 启动各个组件
	if err := as.tracerProvider.Start(ctx); err != nil {
		return fmt.Errorf("failed to start tracer provider: %v", err)
	}

	if err := as.healthChecker.Start(ctx); err != nil {
		return fmt.Errorf("failed to start health checker: %v", err)
	}

	if err := as.alertEngine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start alert engine: %v", err)
	}

	if err := as.loadBalancer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start load balancer: %v", err)
	}

	if err := as.connectionPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection pool: %v", err)
	}

	if err := as.memoryManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start memory manager: %v", err)
	}

	if err := as.zeroCopyManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start zero copy manager: %v", err)
	}

	if err := as.tlsManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start TLS manager: %v", err)
	}

	if err := as.authManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start auth manager: %v", err)
	}

	if err := as.auditLogger.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audit logger: %v", err)
	}

	if err := as.ddosProtector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start DDoS protector: %v", err)
	}

	// 设置路由
	as.setupRoutes()

	// 启动服务器
	if err := as.server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	as.logger.Info("Advanced NetCore server started successfully")
	return nil
}

// Stop 停止服务器
func (as *AdvancedServer) Stop(ctx context.Context) error {
	as.logger.Info("Stopping advanced NetCore server...")

	// 停止各个组件
	if err := as.server.Stop(); err != nil {
		as.logger.Error("Failed to stop server", "error", err)
	}

	if err := as.ddosProtector.Stop(); err != nil {
		as.logger.Error("Failed to stop DDoS protector", "error", err)
	}

	if err := as.auditLogger.Stop(); err != nil {
		as.logger.Error("Failed to stop audit logger", "error", err)
	}

	if err := as.authManager.Stop(); err != nil {
		as.logger.Error("Failed to stop auth manager", "error", err)
	}

	if err := as.tlsManager.Stop(); err != nil {
		as.logger.Error("Failed to stop TLS manager", "error", err)
	}

	if err := as.zeroCopyManager.Stop(); err != nil {
		as.logger.Error("Failed to stop zero copy manager", "error", err)
	}

	if err := as.memoryManager.Stop(); err != nil {
		as.logger.Error("Failed to stop memory manager", "error", err)
	}

	if err := as.connectionPool.Stop(); err != nil {
		as.logger.Error("Failed to stop connection pool", "error", err)
	}

	if err := as.loadBalancer.Stop(); err != nil {
		as.logger.Error("Failed to stop load balancer", "error", err)
	}

	if err := as.alertEngine.Stop(); err != nil {
		as.logger.Error("Failed to stop alert engine", "error", err)
	}

	if err := as.healthChecker.Stop(); err != nil {
		as.logger.Error("Failed to stop health checker", "error", err)
	}

	if err := as.tracerProvider.Shutdown(ctx); err != nil {
		as.logger.Error("Failed to shutdown tracer provider", "error", err)
	}

	as.logger.Info("Advanced NetCore server stopped")
	return nil
}

// setupRoutes 设置路由
func (as *AdvancedServer) setupRoutes() {
	// 获取追踪器
	tracer := as.tracerProvider.Tracer("http-server")

	// 创建HTTP服务器
	mux := http.NewServeMux()

	// 应用中间件
	handler := as.applyMiddleware(mux, tracer)

	// 基本路由
	mux.HandleFunc("/ping", as.pingHandler)
	mux.HandleFunc("/api/stats", as.statsHandler)
	mux.HandleFunc("/api/metrics", as.metricsHandler)
	mux.HandleFunc("/api/trace", as.traceHandler)
	mux.HandleFunc("/api/secure", as.secureHandler)
	mux.HandleFunc("/api/loadbalance", as.loadBalanceHandler)

	// 设置HTTP服务器
	as.server.SetHandler(handler)
}

// applyMiddleware 应用中间件
func (as *AdvancedServer) applyMiddleware(handler http.Handler, tracer tracing.Tracer) http.Handler {
	// 应用追踪中间件
	handler = tracing.HTTPMiddleware(tracer)(handler)

	// 应用DDoS防护中间件
	handler = as.ddosMiddleware(handler)

	// 应用认证中间件
	handler = as.authMiddleware(handler)

	// 应用审计中间件
	handler = as.auditMiddleware(handler)

	// 应用日志中间件
	handler = as.loggingMiddleware(handler)

	return handler
}

// ddosMiddleware DDoS防护中间件
func (as *AdvancedServer) ddosMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !as.ddosProtector.CheckRequest(r.RemoteAddr, r.URL.Path) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware 认证中间件
func (as *AdvancedServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳过公开端点
		if r.URL.Path == "/ping" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// 检查认证
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 验证令牌（简化实现）
		if !as.authManager.ValidateToken(token) {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// auditMiddleware 审计中间件
func (as *AdvancedServer) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 记录请求
		as.auditLogger.LogEvent(security.AuditEvent{
			Type:        security.EventTypeAPIAccess,
			Level:       security.LevelInfo,
			UserID:      r.Header.Get("X-User-ID"),
			Resource:    r.URL.Path,
			Action:      r.Method,
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
			Timestamp:   start,
			Details: map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"query":  r.URL.RawQuery,
			},
		})

		next.ServeHTTP(w, r)

		// 记录响应时间
		duration := time.Since(start)
		as.logger.Info("Request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", duration,
			"remote_addr", r.RemoteAddr,
		)
	})
}

// loggingMiddleware 日志中间件
func (as *AdvancedServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		as.logger.Info("HTTP Request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

// pingHandler ping处理器
func (as *AdvancedServer) pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}

// statsHandler 统计处理器
func (as *AdvancedServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"health":       as.healthChecker.GetStats(),
		"alert":        as.alertEngine.GetStats(),
		"tracing":      as.tracerProvider.GetStats(),
		"loadbalancer": as.loadBalancer.GetStats(),
		"connection":   as.connectionPool.GetStats(),
		"memory":       as.memoryManager.GetStats(),
		"zerocopy":     as.zeroCopyManager.GetStats(),
		"ddos":         as.ddosProtector.GetStats(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%+v", stats)
}

// metricsHandler 指标处理器
func (as *AdvancedServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"memory_usage":    as.memoryManager.GetMemoryUsage(),
		"connection_pool": as.connectionPool.GetStats(),
		"load_balancer":   as.loadBalancer.GetStats(),
		"health_status":   as.healthChecker.GetOverallStatus(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%+v", metrics)
}

// traceHandler 追踪处理器
func (as *AdvancedServer) traceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := as.tracerProvider.Tracer("trace-handler")

	// 创建子跨度
	ctx, span := tracer.Start(ctx, "process-trace-request")
	defer span.End()

	// 模拟一些处理
	span.SetAttribute("request.id", "12345")
	span.AddEvent("processing started", map[string]interface{}{
		"timestamp": time.Now(),
	})

	// 模拟处理时间
	time.Sleep(100 * time.Millisecond)

	span.AddEvent("processing completed", map[string]interface{}{
		"timestamp": time.Now(),
	})
	span.SetStatus(tracing.StatusOK, "Request processed successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"trace_id":"%s","span_id":"%s"}`,
		span.SpanContext().TraceID.String(),
		span.SpanContext().SpanID.String())
}

// secureHandler 安全处理器
func (as *AdvancedServer) secureHandler(w http.ResponseWriter, r *http.Request) {
	// 记录安全事件
	as.auditLogger.LogEvent(security.AuditEvent{
		Type:      security.EventTypeSecurityAccess,
		Level:     security.LevelInfo,
		UserID:    r.Header.Get("X-User-ID"),
		Resource:  "/api/secure",
		Action:    "ACCESS",
		IPAddress: r.RemoteAddr,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"endpoint": "secure",
			"method":   r.Method,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"message":"Secure endpoint accessed","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}

// loadBalanceHandler 负载均衡处理器
func (as *AdvancedServer) loadBalanceHandler(w http.ResponseWriter, r *http.Request) {
	// 选择后端服务器
	backend, err := as.loadBalancer.SelectBackend(r.RemoteAddr, "")
	if err != nil {
		http.Error(w, "No backend available", http.StatusServiceUnavailable)
		return
	}

	// 模拟请求处理
	start := time.Now()
	time.Sleep(50 * time.Millisecond) // 模拟处理时间
	duration := time.Since(start)

	// 更新后端统计
	backend.AddRequest(duration, true)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"backend":"%s","address":"%s","duration_ms":%d}`,
		backend.ID, backend.Address, duration.Milliseconds())
}

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建高级服务器
	server := NewAdvancedServer()

	// 启动服务器
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Advanced NetCore server is running...")
	log.Println("Main server: http://localhost:8080")
	log.Println("Health check: http://localhost:8081/health")
	log.Println("Press Ctrl+C to stop")

	// 等待停止信号
	<-sigChan
	log.Println("Shutting down server...")

	// 创建关闭上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 停止服务器
	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}