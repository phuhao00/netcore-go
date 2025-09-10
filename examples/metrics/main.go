package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netcore-go/pkg/metrics"
)

func main() {
	fmt.Println("=== NetCore-Go 监控指标系统示例 ===")
	
	// 示例1: 基本指标使用
	fmt.Println("\n1. 基本指标使用示例")
	basicMetricsExample()
	
	// 示例2: 直方图和摘要指标
	fmt.Println("\n2. 直方图和摘要指标示例")
	histogramSummaryExample()
	
	// 示例3: 系统指标收集
	fmt.Println("\n3. 系统指标收集示例")
	systemMetricsExample()
	
	// 示例4: Prometheus导出
	fmt.Println("\n4. Prometheus导出示例")
	prometheusExample()
	
	// 示例5: HTTP中间件指标
	fmt.Println("\n5. HTTP中间件指标示例")
	httpMiddlewareExample()
	
	// 示例6: 自定义指标收集器
	fmt.Println("\n6. 自定义指标收集器示例")
	customCollectorExample()
	
	// 示例7: 业务指标监控
	fmt.Println("\n7. 业务指标监控示例")
	businessMetricsExample()
	
	// 启动完整的监控服务
	fmt.Println("\n8. 启动完整监控服务")
	startMonitoringService()
}

// basicMetricsExample 基本指标使用示例
func basicMetricsExample() {
	// 创建计数器
	requestCounter := metrics.RegisterCounter(
		"http_requests_total",
		"Total number of HTTP requests",
		metrics.Labels{"method": "GET", "endpoint": "/api/users"},
	)
	
	// 创建仪表盘指标
	activeConnections := metrics.RegisterGauge(
		"active_connections",
		"Number of active connections",
		metrics.Labels{"protocol": "http"},
	)
	
	// 模拟指标更新
	for i := 0; i < 10; i++ {
		requestCounter.Inc()
		activeConnections.Set(float64(rand.Intn(100) + 50))
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Printf("请求总数: %v\n", requestCounter.Value())
	fmt.Printf("活跃连接数: %v\n", activeConnections.Value())
	
	// 收集样本
	samples := metrics.Gather()
	fmt.Printf("收集到 %d 个指标样本\n", len(samples))
	for _, sample := range samples[:5] { // 只显示前5个
		fmt.Printf("  %s\n", sample.String())
	}
}

// histogramSummaryExample 直方图和摘要指标示例
func histogramSummaryExample() {
	// 创建直方图指标（用于测量请求延迟）
	requestDuration := metrics.RegisterHistogram(
		"http_request_duration_seconds",
		"HTTP request duration in seconds",
		metrics.Labels{"method": "POST"},
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	)
	
	// 创建摘要指标（用于测量响应大小）
	responseSize := metrics.RegisterSummary(
		"http_response_size_bytes",
		"HTTP response size in bytes",
		metrics.Labels{"content_type": "application/json"},
		[]float64{0.5, 0.9, 0.95, 0.99},
		10*time.Minute,
	)
	
	// 模拟请求处理
	for i := 0; i < 100; i++ {
		// 模拟请求延迟（0.001-2秒）
		duration := rand.Float64() * 2
		requestDuration.Observe(duration)
		
		// 模拟响应大小（100-10000字节）
		size := float64(rand.Intn(9900) + 100)
		responseSize.Observe(size)
		
		if i%20 == 0 {
			fmt.Printf("处理了 %d 个请求\n", i+1)
		}
	}
	
	fmt.Printf("直方图指标值: %+v\n", requestDuration.Value())
	fmt.Printf("摘要指标值: %+v\n", responseSize.Value())
}

// systemMetricsExample 系统指标收集示例
func systemMetricsExample() {
	// 启动系统指标收集器
	metrics.StartSystemCollector(&metrics.SystemCollectorConfig{
		Enabled:         true,
		CollectInterval: 2 * time.Second,
		Namespace:       "netcore",
	})
	
	// 等待收集几次指标
	time.Sleep(5 * time.Second)
	
	// 获取系统指标
	systemMetrics := metrics.GetSystemMetrics()
	fmt.Println("系统指标:")
	for name, value := range systemMetrics {
		fmt.Printf("  %s: %v\n", name, value)
	}
	
	// 停止系统指标收集器
	metrics.StopSystemCollector()
}

// prometheusExample Prometheus导出示例
func prometheusExample() {
	// 创建一些测试指标
	testCounter := metrics.RegisterCounter(
		"test_counter_total",
		"A test counter",
		metrics.Labels{"service": "example"},
	)
	testGauge := metrics.RegisterGauge(
		"test_gauge",
		"A test gauge",
		metrics.Labels{"type": "memory"},
	)
	
	// 更新指标
	testCounter.Add(42)
	testGauge.Set(3.14)
	
	// 创建Prometheus导出器
	exporter := metrics.NewPrometheusExporter(&metrics.PrometheusConfig{
		Addr: ":9091",
		Path: "/metrics",
	})
	
	// 启动导出器（非阻塞）
	go func() {
		if err := exporter.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus导出器启动失败: %v", err)
		}
	}()
	
	fmt.Println("Prometheus导出器已启动，访问 http://localhost:9091/metrics 查看指标")
	
	// 等待一段时间
	time.Sleep(3 * time.Second)
	
	// 停止导出器
	exporter.Stop()
}

// httpMiddlewareExample HTTP中间件指标示例
func httpMiddlewareExample() {
	// 创建HTTP服务器
	mux := http.NewServeMux()
	
	// 添加一些测试路由
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		// 模拟处理时间
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users": []}`))
	})
	
	mux.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
		// 模拟处理时间
		time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"orders": []}`))
	})
	
	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal Server Error"}`))
	})
	
	// 添加Prometheus中间件
	handler := metrics.PrometheusMiddleware(nil)(mux)
	
	// 添加指标端点
	mux.Handle("/metrics", metrics.PrometheusHandler(nil))
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	
	// 启动服务器（非阻塞）
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器启动失败: %v", err)
		}
	}()
	
	fmt.Println("HTTP服务器已启动，访问 http://localhost:8080/metrics 查看指标")
	
	// 模拟一些请求
	go func() {
		time.Sleep(1 * time.Second) // 等待服务器启动
		
		for i := 0; i < 20; i++ {
			endpoints := []string{"/api/users", "/api/orders", "/api/error"}
			endpoint := endpoints[rand.Intn(len(endpoints))]
			
			resp, err := http.Get("http://localhost:8080" + endpoint)
			if err == nil {
				resp.Body.Close()
			}
			
			time.Sleep(200 * time.Millisecond)
		}
	}()
	
	// 等待一段时间
	time.Sleep(5 * time.Second)
	
	// 停止服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// customCollectorExample 自定义指标收集器示例
func customCollectorExample() {
	// 创建自定义指标
	cpuTemp := metrics.RegisterGauge(
		"cpu_temperature_celsius",
		"CPU temperature in Celsius",
		metrics.Labels{"core": "0"},
	)
	
	diskUsage := metrics.RegisterGauge(
		"disk_usage_percent",
		"Disk usage percentage",
		metrics.Labels{"device": "/dev/sda1"},
	)
	
	// 启动系统收集器并添加自定义指标
	collector := metrics.NewSystemCollector(&metrics.SystemCollectorConfig{
		Enabled:         true,
		CollectInterval: 1 * time.Second,
		Namespace:       "custom",
	})
	
	// 添加自定义指标收集函数
	collector.AddCustomMetric("cpu_temp", func() float64 {
		// 模拟CPU温度（30-80度）
		temp := 30 + rand.Float64()*50
		cpuTemp.Set(temp)
		return temp
	})
	
	collector.AddCustomMetric("disk_usage", func() float64 {
		// 模拟磁盘使用率（20-95%）
		usage := 20 + rand.Float64()*75
		diskUsage.Set(usage)
		return usage
	})
	
	// 启动收集器
	collector.Start()
	
	// 运行一段时间
	time.Sleep(3 * time.Second)
	
	fmt.Printf("CPU温度: %.2f°C\n", cpuTemp.Value())
	fmt.Printf("磁盘使用率: %.2f%%\n", diskUsage.Value())
	
	// 停止收集器
	collector.Stop()
}

// businessMetricsExample 业务指标监控示例
func businessMetricsExample() {
	// 创建业务指标
	userRegistrations := metrics.RegisterCounter(
		"user_registrations_total",
		"Total number of user registrations",
		metrics.Labels{"source": "web"},
	)
	
	orderValue := metrics.RegisterHistogram(
		"order_value_dollars",
		"Order value in dollars",
		metrics.Labels{"category": "electronics"},
		[]float64{10, 50, 100, 500, 1000, 5000},
	)
	
	loginAttempts := metrics.RegisterCounter(
		"login_attempts_total",
		"Total number of login attempts",
		metrics.Labels{"status": "success"},
	)
	
	failedLogins := metrics.RegisterCounter(
		"login_attempts_total",
		"Total number of login attempts",
		metrics.Labels{"status": "failed"},
	)
	
	// 模拟业务活动
	for i := 0; i < 50; i++ {
		// 模拟用户注册
		if rand.Float64() < 0.1 { // 10%概率
			userRegistrations.Inc()
		}
		
		// 模拟订单
		if rand.Float64() < 0.3 { // 30%概率
			value := 10 + rand.Float64()*1000
			orderValue.Observe(value)
		}
		
		// 模拟登录
		if rand.Float64() < 0.8 { // 80%成功率
			loginAttempts.Inc()
		} else {
			failedLogins.Inc()
		}
		
		time.Sleep(50 * time.Millisecond)
	}
	
	fmt.Printf("用户注册数: %v\n", userRegistrations.Value())
	fmt.Printf("成功登录数: %v\n", loginAttempts.Value())
	fmt.Printf("失败登录数: %v\n", failedLogins.Value())
	fmt.Printf("订单价值分布: %+v\n", orderValue.Value())
}

// startMonitoringService 启动完整的监控服务
func startMonitoringService() {
	fmt.Println("启动完整的监控服务...")
	
	// 启动系统指标收集
	metrics.StartSystemCollector(&metrics.SystemCollectorConfig{
		Enabled:         true,
		CollectInterval: 5 * time.Second,
		Namespace:       "netcore",
	})
	
	// 创建一些业务指标
	appRequests := metrics.RegisterCounter(
		"app_requests_total",
		"Total application requests",
		metrics.Labels{"service": "api"},
	)
	
	appErrors := metrics.RegisterCounter(
		"app_errors_total",
		"Total application errors",
		metrics.Labels{"service": "api"},
	)
	
	responseTime := metrics.RegisterHistogram(
		"app_response_time_seconds",
		"Application response time",
		metrics.Labels{"service": "api"},
		nil,
	)
	
	// 启动Prometheus导出器
	go func() {
		if err := metrics.StartPrometheusExporter(":9090"); err != nil {
			log.Printf("Prometheus导出器启动失败: %v", err)
		}
	}()
	
	// 模拟应用活动
	go func() {
		for {
			// 模拟请求
			appRequests.Inc()
			
			// 模拟响应时间
			start := time.Now()
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			responseTime.Observe(time.Since(start).Seconds())
			
			// 模拟错误（5%概率）
			if rand.Float64() < 0.05 {
				appErrors.Inc()
			}
			
			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		}
	}()
	
	fmt.Println("监控服务已启动:")
	fmt.Println("  - Prometheus指标: http://localhost:9090/metrics")
	fmt.Println("  - 系统指标收集: 每5秒")
	fmt.Println("  - 业务指标模拟: 持续运行")
	fmt.Println("\n按 Ctrl+C 停止服务")
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("\n正在停止监控服务...")
	
	// 停止服务
	metrics.StopSystemCollector()
	metrics.StopPrometheusExporter()
	
	fmt.Println("监控服务已停止")
}