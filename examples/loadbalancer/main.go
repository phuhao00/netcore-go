package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go"
	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/heartbeat"
	"github.com/netcore-go/pkg/middleware"
	"github.com/netcore-go/pkg/pool"
)

// LoadBalancingAlgorithm 负载均衡算法
type LoadBalancingAlgorithm int

const (
	RoundRobin LoadBalancingAlgorithm = iota
	WeightedRoundRobin
	LeastConnections
	WeightedLeastConnections
	IPHash
	Random
)

// BackendServer 后端服务器
type BackendServer struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	Weight      int       `json:"weight"`
	Connections int64     `json:"connections"`
	Healthy     bool      `json:"healthy"`
	LastCheck   time.Time `json:"last_check"`
	ResponseTime int64    `json:"response_time"` // microseconds
	TotalRequests int64   `json:"total_requests"`
	FailedRequests int64  `json:"failed_requests"`
	Server      *netcore.Server `json:"-"`
	mu          sync.RWMutex    `json:"-"`
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	servers     []*BackendServer
	algorithm   LoadBalancingAlgorithm
	currentIndex int64
	heartbeat   *heartbeat.Detector
	stats       *LoadBalancerStats
	mu          sync.RWMutex
}

// LoadBalancerStats 负载均衡器统计
type LoadBalancerStats struct {
	TotalRequests    int64 `json:"total_requests"`
	SuccessfulRequests int64 `json:"successful_requests"`
	FailedRequests   int64 `json:"failed_requests"`
	AverageResponseTime int64 `json:"average_response_time"`
	ActiveConnections int64 `json:"active_connections"`
	HealthyServers   int   `json:"healthy_servers"`
	TotalServers     int   `json:"total_servers"`
	LastUpdateTime   int64 `json:"last_update_time"`
	mu               sync.RWMutex
}

// ProxyServer 代理服务器
type ProxyServer struct {
	server       *netcore.Server
	loadBalancer *LoadBalancer
	pool         *pool.ConnectionPool
	stats        *ProxyStats
}

// ProxyStats 代理统计
type ProxyStats struct {
	TotalConnections int64 `json:"total_connections"`
	ActiveConnections int64 `json:"active_connections"`
	BytesTransferred int64 `json:"bytes_transferred"`
	LastUpdateTime   int64 `json:"last_update_time"`
	mu               sync.RWMutex
}

// RequestMessage 请求消息
type RequestMessage struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Data      interface{}            `json:"data"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// ResponseMessage 响应消息
type ResponseMessage struct {
	ID         string                 `json:"id"`
	RequestID  string                 `json:"request_id"`
	Type       string                 `json:"type"`
	Data       interface{}            `json:"data"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ServerID   string                 `json:"server_id"`
	StatusCode int                    `json:"status_code"`
	Timestamp  int64                  `json:"timestamp"`
}

// NewBackendServer 创建后端服务器
func NewBackendServer(id, address string, weight int) *BackendServer {
	return &BackendServer{
		ID:          id,
		Address:     address,
		Weight:      weight,
		Connections: 0,
		Healthy:     true,
		LastCheck:   time.Now(),
	}
}

// Start 启动后端服务器
func (bs *BackendServer) Start() error {
	config := &netcore.Config{
		Network:     "tcp",
		Address:     bs.Address,
		MaxClients:  100,
		BufferSize:  4096,
		ReadTimeout: time.Second * 30,
	}

	server, err := netcore.NewServer(config)
	if err != nil {
		return fmt.Errorf("创建服务器失败: %v", err)
	}

	bs.Server = server

	// 设置处理器
	server.SetMessageHandler(bs.handleMessage)
	server.SetConnectHandler(bs.handleConnect)
	server.SetDisconnectHandler(bs.handleDisconnect)

	log.Printf("后端服务器 %s 启动在 %s", bs.ID, bs.Address)
	return server.Start()
}

// handleConnect 处理连接
func (bs *BackendServer) handleConnect(conn core.Connection) {
	atomic.AddInt64(&bs.Connections, 1)
	log.Printf("后端服务器 %s: 新连接 %s (总连接: %d)", bs.ID, conn.RemoteAddr(), bs.Connections)
}

// handleDisconnect 处理断开连接
func (bs *BackendServer) handleDisconnect(conn core.Connection) {
	atomic.AddInt64(&bs.Connections, -1)
	log.Printf("后端服务器 %s: 连接断开 %s (总连接: %d)", bs.ID, conn.RemoteAddr(), bs.Connections)
}

// handleMessage 处理消息
func (bs *BackendServer) handleMessage(conn core.Connection, data []byte) {
	start := time.Now()

	var req RequestMessage
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("后端服务器 %s: 解析请求失败: %v", bs.ID, err)
		return
	}

	// 更新统计
	atomic.AddInt64(&bs.TotalRequests, 1)

	// 模拟处理请求
	processingTime := time.Duration(rand.Intn(100)+50) * time.Millisecond
	time.Sleep(processingTime)

	// 创建响应
	resp := ResponseMessage{
		ID:        fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		RequestID: req.ID,
		Type:      "response",
		Data: map[string]interface{}{
			"message":        fmt.Sprintf("Hello from server %s", bs.ID),
			"request_data":   req.Data,
			"processing_time": processingTime.String(),
		},
		ServerID:   bs.ID,
		StatusCode: 200,
		Timestamp:  time.Now().Unix(),
	}

	// 发送响应
	respData, err := json.Marshal(resp)
	if err != nil {
		log.Printf("后端服务器 %s: 序列化响应失败: %v", bs.ID, err)
		atomic.AddInt64(&bs.FailedRequests, 1)
		return
	}

	conn.Write(respData)

	// 更新响应时间
	responseTime := time.Since(start).Microseconds()
	atomic.StoreInt64(&bs.ResponseTime, responseTime)

	log.Printf("后端服务器 %s: 处理请求 %s (耗时: %v)", bs.ID, req.ID, time.Since(start))
}

// Stop 停止后端服务器
func (bs *BackendServer) Stop() error {
	if bs.Server != nil {
		return bs.Server.Stop()
	}
	return nil
}

// GetStats 获取服务器统计
func (bs *BackendServer) GetStats() map[string]interface{} {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	return map[string]interface{}{
		"id":              bs.ID,
		"address":         bs.Address,
		"weight":          bs.Weight,
		"connections":     atomic.LoadInt64(&bs.Connections),
		"healthy":         bs.Healthy,
		"last_check":      bs.LastCheck,
		"response_time":   atomic.LoadInt64(&bs.ResponseTime),
		"total_requests":  atomic.LoadInt64(&bs.TotalRequests),
		"failed_requests": atomic.LoadInt64(&bs.FailedRequests),
	}
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(algorithm LoadBalancingAlgorithm) *LoadBalancer {
	lb := &LoadBalancer{
		servers:   make([]*BackendServer, 0),
		algorithm: algorithm,
		stats:     &LoadBalancerStats{},
	}

	// 创建健康检查
	heartbeatConfig := &heartbeat.Config{
		Interval:    time.Second * 10,
		Timeout:     time.Second * 5,
		MaxRetries:  3,
		Type:        heartbeat.TypePing,
		AutoRestart: true,
	}
	lb.heartbeat = heartbeat.NewDetector(heartbeatConfig)

	return lb
}

// AddServer 添加服务器
func (lb *LoadBalancer) AddServer(server *BackendServer) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.servers = append(lb.servers, server)
	log.Printf("添加后端服务器: %s (%s)", server.ID, server.Address)
}

// RemoveServer 移除服务器
func (lb *LoadBalancer) RemoveServer(serverID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i, server := range lb.servers {
		if server.ID == serverID {
			lb.servers = append(lb.servers[:i], lb.servers[i+1:]...)
			log.Printf("移除后端服务器: %s", serverID)
			break
		}
	}
}

// SelectServer 选择服务器
func (lb *LoadBalancer) SelectServer(clientAddr string) *BackendServer {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// 过滤健康的服务器
	healthyServers := make([]*BackendServer, 0)
	for _, server := range lb.servers {
		if server.Healthy {
			healthyServers = append(healthyServers, server)
		}
	}

	if len(healthyServers) == 0 {
		return nil
	}

	switch lb.algorithm {
	case RoundRobin:
		return lb.roundRobin(healthyServers)
	case WeightedRoundRobin:
		return lb.weightedRoundRobin(healthyServers)
	case LeastConnections:
		return lb.leastConnections(healthyServers)
	case WeightedLeastConnections:
		return lb.weightedLeastConnections(healthyServers)
	case IPHash:
		return lb.ipHash(healthyServers, clientAddr)
	case Random:
		return lb.random(healthyServers)
	default:
		return lb.roundRobin(healthyServers)
	}
}

// roundRobin 轮询算法
func (lb *LoadBalancer) roundRobin(servers []*BackendServer) *BackendServer {
	index := atomic.AddInt64(&lb.currentIndex, 1) % int64(len(servers))
	return servers[index]
}

// weightedRoundRobin 加权轮询算法
func (lb *LoadBalancer) weightedRoundRobin(servers []*BackendServer) *BackendServer {
	totalWeight := 0
	for _, server := range servers {
		totalWeight += server.Weight
	}

	if totalWeight == 0 {
		return lb.roundRobin(servers)
	}

	index := atomic.AddInt64(&lb.currentIndex, 1) % int64(totalWeight)
	currentWeight := int64(0)

	for _, server := range servers {
		currentWeight += int64(server.Weight)
		if index < currentWeight {
			return server
		}
	}

	return servers[0]
}

// leastConnections 最少连接算法
func (lb *LoadBalancer) leastConnections(servers []*BackendServer) *BackendServer {
	minConnections := int64(^uint64(0) >> 1) // max int64
	var selectedServer *BackendServer

	for _, server := range servers {
		connections := atomic.LoadInt64(&server.Connections)
		if connections < minConnections {
			minConnections = connections
			selectedServer = server
		}
	}

	return selectedServer
}

// weightedLeastConnections 加权最少连接算法
func (lb *LoadBalancer) weightedLeastConnections(servers []*BackendServer) *BackendServer {
	minRatio := float64(^uint64(0) >> 1) // max float64
	var selectedServer *BackendServer

	for _, server := range servers {
		connections := atomic.LoadInt64(&server.Connections)
		weight := float64(server.Weight)
		if weight == 0 {
			weight = 1
		}
		ratio := float64(connections) / weight

		if ratio < minRatio {
			minRatio = ratio
			selectedServer = server
		}
	}

	return selectedServer
}

// ipHash IP哈希算法
func (lb *LoadBalancer) ipHash(servers []*BackendServer, clientAddr string) *BackendServer {
	hash := 0
	for _, char := range clientAddr {
		hash += int(char)
	}
	index := hash % len(servers)
	return servers[index]
}

// random 随机算法
func (lb *LoadBalancer) random(servers []*BackendServer) *BackendServer {
	index := rand.Intn(len(servers))
	return servers[index]
}

// StartHealthCheck 启动健康检查
func (lb *LoadBalancer) StartHealthCheck() {
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()

		for range ticker.C {
			lb.performHealthCheck()
		}
	}()
}

// performHealthCheck 执行健康检查
func (lb *LoadBalancer) performHealthCheck() {
	lb.mu.RLock()
	servers := make([]*BackendServer, len(lb.servers))
	copy(servers, lb.servers)
	lb.mu.RUnlock()

	for _, server := range servers {
		go func(s *BackendServer) {
			start := time.Now()
			healthy := lb.checkServerHealth(s)
			checkDuration := time.Since(start)

			s.mu.Lock()
			s.Healthy = healthy
			s.LastCheck = time.Now()
			if healthy {
				atomic.StoreInt64(&s.ResponseTime, checkDuration.Microseconds())
			}
			s.mu.Unlock()

			if !healthy {
				log.Printf("服务器 %s 健康检查失败", s.ID)
			}
		}(server)
	}
}

// checkServerHealth 检查服务器健康状态
func (lb *LoadBalancer) checkServerHealth(server *BackendServer) bool {
	// 简单的TCP连接检查
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// 这里可以实现更复杂的健康检查逻辑
	// 比如发送特定的健康检查请求
	return true // 简化实现，总是返回健康
}

// GetStats 获取负载均衡器统计
func (lb *LoadBalancer) GetStats() *LoadBalancerStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	healthyCount := 0
	totalConnections := int64(0)
	totalRequests := int64(0)
	totalFailedRequests := int64(0)
	totalResponseTime := int64(0)
	responseTimeCount := 0

	for _, server := range lb.servers {
		if server.Healthy {
			healthyCount++
		}
		totalConnections += atomic.LoadInt64(&server.Connections)
		totalRequests += atomic.LoadInt64(&server.TotalRequests)
		totalFailedRequests += atomic.LoadInt64(&server.FailedRequests)

		responseTime := atomic.LoadInt64(&server.ResponseTime)
		if responseTime > 0 {
			totalResponseTime += responseTime
			responseTimeCount++
		}
	}

	averageResponseTime := int64(0)
	if responseTimeCount > 0 {
		averageResponseTime = totalResponseTime / int64(responseTimeCount)
	}

	lb.stats.mu.Lock()
	lb.stats.TotalRequests = totalRequests
	lb.stats.SuccessfulRequests = totalRequests - totalFailedRequests
	lb.stats.FailedRequests = totalFailedRequests
	lb.stats.AverageResponseTime = averageResponseTime
	lb.stats.ActiveConnections = totalConnections
	lb.stats.HealthyServers = healthyCount
	lb.stats.TotalServers = len(lb.servers)
	lb.stats.LastUpdateTime = time.Now().Unix()
	lb.stats.mu.Unlock()

	return &LoadBalancerStats{
		TotalRequests:       lb.stats.TotalRequests,
		SuccessfulRequests:  lb.stats.SuccessfulRequests,
		FailedRequests:      lb.stats.FailedRequests,
		AverageResponseTime: lb.stats.AverageResponseTime,
		ActiveConnections:   lb.stats.ActiveConnections,
		HealthyServers:      lb.stats.HealthyServers,
		TotalServers:        lb.stats.TotalServers,
		LastUpdateTime:      lb.stats.LastUpdateTime,
	}
}

// NewProxyServer 创建代理服务器
func NewProxyServer(loadBalancer *LoadBalancer) *ProxyServer {
	ps := &ProxyServer{
		loadBalancer: loadBalancer,
		stats:        &ProxyStats{},
	}

	// 创建连接池
	ps.pool = pool.NewConnectionPool(&pool.Config{
		MaxConnections:    1000,
		MaxIdleTime:       time.Minute * 5,
		CleanupInterval:   time.Minute,
		HealthCheckPeriod: time.Second * 30,
	})

	return ps
}

// Start 启动代理服务器
func (ps *ProxyServer) Start(addr string) error {
	config := &netcore.Config{
		Network:     "tcp",
		Address:     addr,
		MaxClients:  1000,
		BufferSize:  4096,
		ReadTimeout: time.Second * 30,
	}

	server, err := netcore.NewServer(config)
	if err != nil {
		return fmt.Errorf("创建代理服务器失败: %v", err)
	}

	ps.server = server

	// 添加中间件
	middlewareManager := middleware.NewManager()
	middlewareManager.RegisterWebAPIPreset()

	// 设置处理器
	server.SetMessageHandler(ps.handleMessage)
	server.SetConnectHandler(ps.handleConnect)
	server.SetDisconnectHandler(ps.handleDisconnect)

	// 启动统计更新
	go ps.updateStats()

	log.Printf("代理服务器启动在 %s", addr)
	return server.Start()
}

// handleConnect 处理连接
func (ps *ProxyServer) handleConnect(conn core.Connection) {
	atomic.AddInt64(&ps.stats.TotalConnections, 1)
	atomic.AddInt64(&ps.stats.ActiveConnections, 1)

	// 添加到连接池
	ps.pool.AddConnection(conn)

	log.Printf("代理服务器: 新连接 %s (活跃连接: %d)", 
		conn.RemoteAddr(), atomic.LoadInt64(&ps.stats.ActiveConnections))
}

// handleDisconnect 处理断开连接
func (ps *ProxyServer) handleDisconnect(conn core.Connection) {
	atomic.AddInt64(&ps.stats.ActiveConnections, -1)

	// 从连接池移除
	ps.pool.RemoveConnection(conn.ID())

	log.Printf("代理服务器: 连接断开 %s (活跃连接: %d)", 
		conn.RemoteAddr(), atomic.LoadInt64(&ps.stats.ActiveConnections))
}

// handleMessage 处理消息
func (ps *ProxyServer) handleMessage(conn core.Connection, data []byte) {
	// 选择后端服务器
	backendServer := ps.loadBalancer.SelectServer(conn.RemoteAddr())
	if backendServer == nil {
		ps.sendError(conn, "没有可用的后端服务器")
		return
	}

	// 转发请求到后端服务器
	ps.forwardRequest(conn, backendServer, data)

	// 更新统计
	atomic.AddInt64(&ps.stats.BytesTransferred, int64(len(data)))
}

// forwardRequest 转发请求
func (ps *ProxyServer) forwardRequest(clientConn core.Connection, backendServer *BackendServer, data []byte) {
	// 这里简化实现，直接调用后端服务器的处理函数
	// 在实际实现中，应该建立到后端服务器的连接并转发数据

	var req RequestMessage
	if err := json.Unmarshal(data, &req); err != nil {
		ps.sendError(clientConn, "解析请求失败")
		return
	}

	// 添加代理信息
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["X-Forwarded-For"] = clientConn.RemoteAddr()
	req.Headers["X-Proxy-Server"] = "NetCore-LoadBalancer"

	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["backend_server"] = backendServer.ID
	req.Metadata["proxy_timestamp"] = time.Now().Unix()

	// 模拟转发到后端服务器
	start := time.Now()

	// 创建模拟响应
	resp := ResponseMessage{
		ID:        fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		RequestID: req.ID,
		Type:      "response",
		Data: map[string]interface{}{
			"message":      fmt.Sprintf("Hello from backend server %s via proxy", backendServer.ID),
			"request_data": req.Data,
			"backend_server": backendServer.ID,
		},
		Headers: map[string]string{
			"X-Backend-Server": backendServer.ID,
			"X-Response-Time":  time.Since(start).String(),
		},
		ServerID:   backendServer.ID,
		StatusCode: 200,
		Timestamp:  time.Now().Unix(),
	}

	// 发送响应给客户端
	respData, err := json.Marshal(resp)
	if err != nil {
		ps.sendError(clientConn, "序列化响应失败")
		return
	}

	clientConn.Write(respData)

	// 更新后端服务器统计
	atomic.AddInt64(&backendServer.TotalRequests, 1)
	atomic.StoreInt64(&backendServer.ResponseTime, time.Since(start).Microseconds())

	log.Printf("代理转发: 请求 %s -> 后端服务器 %s (耗时: %v)", 
		req.ID, backendServer.ID, time.Since(start))
}

// sendError 发送错误响应
func (ps *ProxyServer) sendError(conn core.Connection, errorMsg string) {
	errorResp := ResponseMessage{
		ID:         fmt.Sprintf("error_%d", time.Now().UnixNano()),
		Type:       "error",
		Data:       errorMsg,
		StatusCode: 500,
		Timestamp:  time.Now().Unix(),
	}

	data, _ := json.Marshal(errorResp)
	conn.Write(data)
}

// updateStats 更新统计信息
func (ps *ProxyServer) updateStats() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for range ticker.C {
		ps.stats.mu.Lock()
		ps.stats.LastUpdateTime = time.Now().Unix()
		ps.stats.mu.Unlock()

		log.Printf("代理统计 - 总连接: %d, 活跃连接: %d, 传输字节: %d", 
			atomic.LoadInt64(&ps.stats.TotalConnections),
			atomic.LoadInt64(&ps.stats.ActiveConnections),
			atomic.LoadInt64(&ps.stats.BytesTransferred))
	}
}

// GetStats 获取代理统计
func (ps *ProxyServer) GetStats() *ProxyStats {
	ps.stats.mu.RLock()
	defer ps.stats.mu.RUnlock()

	return &ProxyStats{
		TotalConnections:  atomic.LoadInt64(&ps.stats.TotalConnections),
		ActiveConnections: atomic.LoadInt64(&ps.stats.ActiveConnections),
		BytesTransferred:  atomic.LoadInt64(&ps.stats.BytesTransferred),
		LastUpdateTime:    ps.stats.LastUpdateTime,
	}
}

// Stop 停止代理服务器
func (ps *ProxyServer) Stop() error {
	if ps.pool != nil {
		ps.pool.Close()
	}

	if ps.server != nil {
		return ps.server.Stop()
	}

	return nil
}

func main() {
	// 创建负载均衡器
	loadBalancer := NewLoadBalancer(WeightedRoundRobin)

	// 创建后端服务器
	backendServers := []*BackendServer{
		NewBackendServer("backend-1", ":10001", 3),
		NewBackendServer("backend-2", ":10002", 2),
		NewBackendServer("backend-3", ":10003", 1),
	}

	// 启动后端服务器
	for _, server := range backendServers {
		loadBalancer.AddServer(server)
		go func(s *BackendServer) {
			if err := s.Start(); err != nil {
				log.Printf("启动后端服务器 %s 失败: %v", s.ID, err)
			}
		}(server)
	}

	// 启动健康检查
	loadBalancer.StartHealthCheck()

	// 创建代理服务器
	proxyServer := NewProxyServer(loadBalancer)

	// 启动HTTP API服务器
	go func() {
		http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			stats := map[string]interface{}{
				"proxy":         proxyServer.GetStats(),
				"load_balancer": loadBalancer.GetStats(),
			}
			json.NewEncoder(w).Encode(stats)
		})

		http.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			loadBalancer.mu.RLock()
			serverStats := make([]map[string]interface{}, 0, len(loadBalancer.servers))
			for _, server := range loadBalancer.servers {
				serverStats = append(serverStats, server.GetStats())
			}
			loadBalancer.mu.RUnlock()
			json.NewEncoder(w).Encode(serverStats)
		})

		http.HandleFunc("/algorithm", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				var req struct {
					Algorithm string `json:"algorithm"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "Invalid request", http.StatusBadRequest)
					return
				}

				var algorithm LoadBalancingAlgorithm
				switch req.Algorithm {
				case "round_robin":
					algorithm = RoundRobin
				case "weighted_round_robin":
					algorithm = WeightedRoundRobin
				case "least_connections":
					algorithm = LeastConnections
				case "weighted_least_connections":
					algorithm = WeightedLeastConnections
				case "ip_hash":
					algorithm = IPHash
				case "random":
					algorithm = Random
				default:
					http.Error(w, "Unknown algorithm", http.StatusBadRequest)
					return
				}

				loadBalancer.mu.Lock()
				loadBalancer.algorithm = algorithm
				loadBalancer.mu.Unlock()

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			} else {
				w.Header().Set("Content-Type", "application/json")
				algorithmNames := map[LoadBalancingAlgorithm]string{
					RoundRobin:               "round_robin",
					WeightedRoundRobin:       "weighted_round_robin",
					LeastConnections:         "least_connections",
					WeightedLeastConnections: "weighted_least_connections",
					IPHash:                   "ip_hash",
					Random:                   "random",
				}
				currentAlgorithm := algorithmNames[loadBalancer.algorithm]
				json.NewEncoder(w).Encode(map[string]string{"current_algorithm": currentAlgorithm})
			}
		})

		log.Println("HTTP API服务器启动在 :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 等待后端服务器启动
	time.Sleep(time.Second * 2)

	// 启动代理服务器
	if err := proxyServer.Start(":9996"); err != nil {
		log.Fatalf("启动代理服务器失败: %v", err)
	}
}


