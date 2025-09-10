package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/netcore-go/pkg/discovery"
	"github.com/netcore-go/pkg/loadbalancer"
)

func main() {
	fmt.Println("=== NetCore-Go 服务发现和负载均衡示例 ===")
	
	// 示例1: 基本服务注册和发现
	fmt.Println("\n1. 基本服务注册和发现示例")
	basicDiscoveryExample()
	
	// 示例2: 负载均衡算法
	fmt.Println("\n2. 负载均衡算法示例")
	loadBalancingExample()
	
	// 示例3: 服务监听
	fmt.Println("\n3. 服务监听示例")
	serviceWatchExample()
	
	// 示例4: 健康检查
	fmt.Println("\n4. 健康检查示例")
	healthCheckExample()
	
	// 示例5: 服务过滤
	fmt.Println("\n5. 服务过滤示例")
	serviceFilterExample()
	
	// 示例6: 完整的微服务场景
	fmt.Println("\n6. 完整的微服务场景示例")
	microserviceExample()
}

// basicDiscoveryExample 基本服务注册和发现示例
func basicDiscoveryExample() {
	// 创建内存注册中心
	registry := discovery.NewMemoryClient()
	defer registry.Close()
	
	ctx := context.Background()
	
	// 注册多个服务实例
	services := []*discovery.ServiceInstance{
		{
			ID:       "user-service-1",
			Name:     "user-service",
			Address:  "192.168.1.10",
			Port:     8080,
			Tags:     []string{"v1.0", "production"},
			Health:   discovery.Healthy,
			Weight:   100,
			Version:  "1.0.0",
			Region:   "us-east-1",
			Zone:     "us-east-1a",
			Protocol: "http",
		},
		{
			ID:       "user-service-2",
			Name:     "user-service",
			Address:  "192.168.1.11",
			Port:     8080,
			Tags:     []string{"v1.0", "production"},
			Health:   discovery.Healthy,
			Weight:   150,
			Version:  "1.0.0",
			Region:   "us-east-1",
			Zone:     "us-east-1b",
			Protocol: "http",
		},
		{
			ID:       "user-service-3",
			Name:     "user-service",
			Address:  "192.168.1.12",
			Port:     8080,
			Tags:     []string{"v1.1", "beta"},
			Health:   discovery.Unhealthy,
			Weight:   80,
			Version:  "1.1.0",
			Region:   "us-west-1",
			Zone:     "us-west-1a",
			Protocol: "http",
		},
	}
	
	// 注册服务
	for _, service := range services {
		if err := registry.Register(ctx, service); err != nil {
			log.Printf("注册服务失败: %v", err)
			continue
		}
		fmt.Printf("注册服务: %s (%s)\n", service.ID, service.GetEndpoint())
	}
	
	// 发现服务
	instances, err := registry.GetService(ctx, "user-service")
	if err != nil {
		log.Printf("发现服务失败: %v", err)
		return
	}
	
	fmt.Printf("\n发现到 %d 个服务实例:\n", len(instances))
	for _, instance := range instances {
		fmt.Printf("  - %s: %s (健康状态: %s, 权重: %d)\n",
			instance.ID, instance.GetEndpoint(), instance.Health, instance.Weight)
	}
	
	// 获取健康的服务实例
	healthyInstances, err := registry.GetHealthyServices(ctx, "user-service")
	if err != nil {
		log.Printf("获取健康服务失败: %v", err)
		return
	}
	
	fmt.Printf("\n健康的服务实例 (%d 个):\n", len(healthyInstances))
	for _, instance := range healthyInstances {
		fmt.Printf("  - %s: %s\n", instance.ID, instance.GetEndpoint())
	}
	
	// 根据标签获取服务
	prodInstances, err := registry.GetServicesByTag(ctx, "user-service", "production")
	if err != nil {
		log.Printf("根据标签获取服务失败: %v", err)
		return
	}
	
	fmt.Printf("\n生产环境服务实例 (%d 个):\n", len(prodInstances))
	for _, instance := range prodInstances {
		fmt.Printf("  - %s: %s\n", instance.ID, instance.GetEndpoint())
	}
}

// loadBalancingExample 负载均衡算法示例
func loadBalancingExample() {
	// 创建测试服务实例
	instances := []*discovery.ServiceInstance{
		{
			ID:      "service-1",
			Name:    "test-service",
			Address: "192.168.1.10",
			Port:    8080,
			Health:  discovery.Healthy,
			Weight:  100,
		},
		{
			ID:      "service-2",
			Name:    "test-service",
			Address: "192.168.1.11",
			Port:    8080,
			Health:  discovery.Healthy,
			Weight:  200,
		},
		{
			ID:      "service-3",
			Name:    "test-service",
			Address: "192.168.1.12",
			Port:    8080,
			Health:  discovery.Healthy,
			Weight:  50,
		},
	}
	
	ctx := context.Background()
	algorithms := []loadbalancer.Algorithm{
		loadbalancer.RoundRobin,
		loadbalancer.WeightedRoundRobin,
		loadbalancer.Random,
		loadbalancer.WeightedRandom,
	}
	
	// 测试不同的负载均衡算法
	for _, algorithm := range algorithms {
		fmt.Printf("\n%s 算法测试:\n", algorithm)
		
		// 统计每个实例被选中的次数
		counts := make(map[string]int)
		
		// 执行100次选择
		for i := 0; i < 100; i++ {
			selected, err := loadbalancer.Select(ctx, instances, algorithm)
			if err != nil {
				log.Printf("负载均衡选择失败: %v", err)
				continue
			}
			counts[selected.ID]++
		}
		
		// 显示结果
		for _, instance := range instances {
			percentage := float64(counts[instance.ID]) / 100.0 * 100
			fmt.Printf("  %s (权重:%d): %d次 (%.1f%%)\n",
				instance.ID, instance.Weight, counts[instance.ID], percentage)
		}
	}
}

// serviceWatchExample 服务监听示例
func serviceWatchExample() {
	// 创建内存注册中心
	registry := discovery.NewMemoryClient()
	defer registry.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// 创建服务监听器
	watcher, err := registry.Watch(ctx, "watch-service")
	if err != nil {
		log.Printf("创建监听器失败: %v", err)
		return
	}
	defer watcher.Stop()
	
	// 启动事件监听
	eventChan, err := watcher.Watch(ctx, "watch-service")
	if err != nil {
		log.Printf("启动监听失败: %v", err)
		return
	}
	
	// 在后台监听事件
	go func() {
		for {
			select {
			case event := <-eventChan:
				if event != nil {
					fmt.Printf("[事件] %s: %s (%s)\n",
						event.Type, event.Instance.ID, event.Instance.GetEndpoint())
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// 模拟服务注册、更新、注销
	time.Sleep(100 * time.Millisecond)
	
	// 注册服务
	service := &discovery.ServiceInstance{
		ID:      "watch-service-1",
		Name:    "watch-service",
		Address: "192.168.1.100",
		Port:    8080,
		Health:  discovery.Healthy,
	}
	
	fmt.Println("注册服务...")
	registry.Register(ctx, service)
	time.Sleep(500 * time.Millisecond)
	
	// 更新健康状态
	fmt.Println("更新健康状态...")
	registry.SetHealth(ctx, service.ID, discovery.Unhealthy)
	time.Sleep(500 * time.Millisecond)
	
	// 恢复健康状态
	fmt.Println("恢复健康状态...")
	registry.SetHealth(ctx, service.ID, discovery.Healthy)
	time.Sleep(500 * time.Millisecond)
	
	// 注销服务
	fmt.Println("注销服务...")
	registry.Deregister(ctx, service.ID)
	time.Sleep(500 * time.Millisecond)
}

// healthCheckExample 健康检查示例
func healthCheckExample() {
	// 创建内存注册中心
	registry := discovery.NewMemoryClient()
	defer registry.Close()
	
	// 创建健康检查器
	healthChecker := discovery.NewHealthChecker(registry, 2*time.Second)
	defer healthChecker.Stop()
	
	ctx := context.Background()
	
	// 注册服务实例
	service := &discovery.ServiceInstance{
		ID:      "health-service-1",
		Name:    "health-service",
		Address: "192.168.1.200",
		Port:    8080,
		Health:  discovery.Healthy,
	}
	
	registry.Register(ctx, service)
	fmt.Printf("注册服务: %s\n", service.ID)
	
	// 添加HTTP健康检查
	healthCheck := &discovery.HealthCheck{
		Type:     discovery.HealthCheckHTTP,
		URL:      "http://192.168.1.200:8080/health",
		Interval: 5 * time.Second,
		Timeout:  3 * time.Second,
	}
	
	healthChecker.AddCheck(service.ID, healthCheck)
	fmt.Printf("添加健康检查: %s\n", healthCheck.URL)
	
	// 监控健康状态变化
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		
		// 获取当前服务状态
		instances, _ := registry.GetService(ctx, "health-service")
		if len(instances) > 0 {
			fmt.Printf("健康状态检查 #%d: %s\n", i+1, instances[0].Health)
		}
	}
}

// serviceFilterExample 服务过滤示例
func serviceFilterExample() {
	// 创建测试服务实例
	instances := []*discovery.ServiceInstance{
		{
			ID:      "service-1",
			Name:    "api-service",
			Address: "192.168.1.10",
			Port:    8080,
			Tags:    []string{"v1.0", "production"},
			Health:  discovery.Healthy,
			Region:  "us-east-1",
			Zone:    "us-east-1a",
			Version: "1.0.0",
		},
		{
			ID:      "service-2",
			Name:    "api-service",
			Address: "192.168.1.11",
			Port:    8080,
			Tags:    []string{"v1.1", "beta"},
			Health:  discovery.Unhealthy,
			Region:  "us-east-1",
			Zone:    "us-east-1b",
			Version: "1.1.0",
		},
		{
			ID:      "service-3",
			Name:    "api-service",
			Address: "192.168.1.12",
			Port:    8080,
			Tags:    []string{"v1.0", "production"},
			Health:  discovery.Healthy,
			Region:  "us-west-1",
			Zone:    "us-west-1a",
			Version: "1.0.0",
		},
	}
	
	fmt.Printf("原始服务实例 (%d 个):\n", len(instances))
	for _, instance := range instances {
		fmt.Printf("  - %s: %s (健康: %s, 区域: %s, 版本: %s)\n",
			instance.ID, instance.GetEndpoint(), instance.Health, instance.Region, instance.Version)
	}
	
	// 测试不同的过滤器
	filters := []struct {
		name   string
		filter discovery.ServiceFilter
	}{
		{"健康实例", discovery.FilterByHealth(true)},
		{"生产环境", discovery.FilterByTag("production")},
		{"美东区域", discovery.FilterByRegion("us-east-1")},
		{"v1.0版本", discovery.FilterByVersion("1.0.0")},
	}
	
	for _, f := range filters {
		filtered := discovery.ApplyFilters(instances, f.filter)
		fmt.Printf("\n%s过滤结果 (%d 个):\n", f.name, len(filtered))
		for _, instance := range filtered {
			fmt.Printf("  - %s: %s\n", instance.ID, instance.GetEndpoint())
		}
	}
	
	// 组合过滤器
	combined := discovery.ApplyFilters(instances,
		discovery.FilterByHealth(true),
		discovery.FilterByTag("production"),
		discovery.FilterByRegion("us-east-1"),
	)
	
	fmt.Printf("\n组合过滤结果 (%d 个):\n", len(combined))
	for _, instance := range combined {
		fmt.Printf("  - %s: %s\n", instance.ID, instance.GetEndpoint())
	}
}

// microserviceExample 完整的微服务场景示例
func microserviceExample() {
	fmt.Println("启动微服务场景演示...")
	
	// 创建服务注册中心
	registry := discovery.NewMemoryClient()
	defer registry.Close()
	
	// 创建服务管理器
	manager := discovery.NewServiceManager(registry, 5*time.Minute)
	defer manager.Close()
	
	// 创建负载均衡管理器
	lbManager := loadbalancer.NewManager(&loadbalancer.Config{
		Algorithm:   loadbalancer.WeightedRoundRobin,
		HealthCheck: true,
		MaxRetries:  3,
	})
	defer lbManager.Close()
	
	ctx := context.Background()
	
	// 启动模拟服务
	go startMockService("user-service", 8081, registry)
	go startMockService("order-service", 8082, registry)
	go startMockService("payment-service", 8083, registry)
	
	// 等待服务启动
	time.Sleep(2 * time.Second)
	
	// 模拟客户端请求
	for i := 0; i < 10; i++ {
		// 发现用户服务
		userInstances, err := manager.DiscoverService(ctx, "user-service", true)
		if err != nil {
			log.Printf("发现用户服务失败: %v", err)
			continue
		}
		
		if len(userInstances) == 0 {
			log.Println("没有可用的用户服务实例")
			continue
		}
		
		// 负载均衡选择实例
		selected, err := lbManager.Select(ctx, userInstances, loadbalancer.WeightedRoundRobin)
		if err != nil {
			log.Printf("负载均衡选择失败: %v", err)
			continue
		}
		
		// 模拟请求
		start := time.Now()
		success := simulateRequest(selected)
		duration := time.Since(start)
		
		// 更新统计信息
		lbManager.UpdateStats(selected.ID, success, duration)
		
		fmt.Printf("请求 #%d: %s -> %s (耗时: %v, 成功: %v)\n",
			i+1, selected.ID, selected.GetEndpoint(), duration, success)
		
		time.Sleep(500 * time.Millisecond)
	}
	
	// 显示统计信息
	fmt.Println("\n=== 统计信息 ===")
	for _, instance := range userInstances {
		stats := lbManager.GetStats(instance.ID)
		if stats != nil {
			fmt.Printf("%s: 连接数=%d, 成功率=%.2f%%, 平均响应时间=%v\n",
				instance.ID,
				stats.GetConnections(),
				stats.GetSuccessRate()*100,
				stats.GetAvgResponseTime())
		}
	}
}

// startMockService 启动模拟服务
func startMockService(serviceName string, port int, registry discovery.ServiceClient) {
	ctx := context.Background()
	
	// 注册服务实例
	instance := &discovery.ServiceInstance{
		ID:       fmt.Sprintf("%s-%d", serviceName, port),
		Name:     serviceName,
		Address:  "127.0.0.1",
		Port:     port,
		Tags:     []string{"v1.0", "mock"},
		Health:   discovery.Healthy,
		Weight:   100,
		Version:  "1.0.0",
		Protocol: "http",
	}
	
	if err := registry.Register(ctx, instance); err != nil {
		log.Printf("注册服务失败: %v", err)
		return
	}
	
	fmt.Printf("启动模拟服务: %s 在端口 %d\n", serviceName, port)
	
	// 创建HTTP服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 模拟处理时间
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"service": "%s", "instance": "%s", "timestamp": "%s"}`,
			serviceName, instance.ID, time.Now().Format(time.RFC3339))))
	})
	
	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}
	
	// 启动服务器
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器启动失败: %v", err)
		}
	}()
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	// 注销服务
	registry.Deregister(ctx, instance.ID)
	fmt.Printf("停止模拟服务: %s\n", serviceName)
	
	// 关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// simulateRequest 模拟请求
func simulateRequest(instance *discovery.ServiceInstance) bool {
	// 模拟请求处理时间
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
	
	// 90%的成功率
	return rand.Float64() < 0.9
}