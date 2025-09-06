// Package main HTTP性能测试示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/netcore-go"
	"github.com/netcore-go/netcore-go/pkg/core"
	netcorehttp "github.com/netcore-go/netcore-go/pkg/http"
)

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	TotalRequests    int64         `json:"total_requests"`
	SuccessRequests  int64         `json:"success_requests"`
	FailedRequests   int64         `json:"failed_requests"`
	TotalDuration    time.Duration `json:"total_duration"`
	AvgResponseTime  time.Duration `json:"avg_response_time"`
	MinResponseTime  time.Duration `json:"min_response_time"`
	MaxResponseTime  time.Duration `json:"max_response_time"`
	RequestsPerSec   float64       `json:"requests_per_sec"`
	ThroughputMBps   float64       `json:"throughput_mbps"`
	TotalBytes       int64         `json:"total_bytes"`
}

// HTTPBenchmarkHandler HTTP基准测试处理器
type HTTPBenchmarkHandler struct {
	requestCount int64
	bytesSent    int64
}

// OnConnect 连接建立时调用
func (h *HTTPBenchmarkHandler) OnConnect(conn core.Connection) {
	// HTTP基准测试不需要特殊处理
}

// OnMessage 收到消息时调用（HTTP不使用此方法）
func (h *HTTPBenchmarkHandler) OnMessage(conn core.Connection, msg core.Message) {
	// HTTP协议不使用此方法
}

// OnDisconnect 连接断开时调用
func (h *HTTPBenchmarkHandler) OnDisconnect(conn core.Connection, err error) {
	// HTTP基准测试不需要特殊处理
}

// setupBenchmarkRoutes 设置基准测试路由
func (h *HTTPBenchmarkHandler) setupBenchmarkRoutes(server *netcorehttp.HTTPServer) {
	// 简单的Hello World响应
	server.HandleFunc("GET", "/hello", func(ctx *netcorehttp.HTTPContext, resp *netcorehttp.HTTPResponse) {
		atomic.AddInt64(&h.requestCount, 1)
		message := "Hello, World!"
		atomic.AddInt64(&h.bytesSent, int64(len(message)))
		ctx.String(resp, 200, message)
	})

	// JSON响应测试
	server.HandleFunc("GET", "/json", func(ctx *netcorehttp.HTTPContext, resp *netcorehttp.HTTPResponse) {
		atomic.AddInt64(&h.requestCount, 1)
		data := map[string]interface{}{
			"message": "Hello, World!",
			"timestamp": time.Now().Unix(),
			"status": "success",
		}
		ctx.JSON(resp, 200, data)
		atomic.AddInt64(&h.bytesSent, int64(len(resp.Body)))
	})

	// POST请求测试
	server.HandleFunc("POST", "/echo", func(ctx *netcorehttp.HTTPContext, resp *netcorehttp.HTTPResponse) {
		atomic.AddInt64(&h.requestCount, 1)
		body := ctx.Body()
		atomic.AddInt64(&h.bytesSent, int64(len(body)))
		resp.StatusCode = 200
		resp.StatusText = "OK"
		resp.Headers["Content-Type"] = "application/octet-stream"
		resp.Body = body
	})

	// 大数据响应测试
	server.HandleFunc("GET", "/large", func(ctx *netcorehttp.HTTPContext, resp *netcorehttp.HTTPResponse) {
		atomic.AddInt64(&h.requestCount, 1)
		// 生成1KB的数据
		data := make([]byte, 1024)
		for i := range data {
			data[i] = byte('A' + (i % 26))
		}
		atomic.AddInt64(&h.bytesSent, int64(len(data)))
		resp.StatusCode = 200
		resp.StatusText = "OK"
		resp.Headers["Content-Type"] = "text/plain"
		resp.Body = data
	})

	// 统计信息
	server.HandleFunc("GET", "/stats", func(ctx *netcorehttp.HTTPContext, resp *netcorehttp.HTTPResponse) {
		stats := server.GetStats()
		data := map[string]interface{}{
			"requests_handled": atomic.LoadInt64(&h.requestCount),
			"bytes_sent": atomic.LoadInt64(&h.bytesSent),
			"active_connections": stats.ActiveConnections,
			"total_connections": stats.TotalConnections,
			"uptime_seconds": stats.Uptime,
		}
		ctx.JSON(resp, 200, data)
	})
}

// runBenchmark 运行基准测试
func runBenchmark(url string, concurrency int, duration time.Duration, requestBody []byte) *BenchmarkResult {
	fmt.Printf("Running benchmark: %s\n", url)
	fmt.Printf("Concurrency: %d, Duration: %v\n", concurrency, duration)

	var (
		totalRequests   int64
		successRequests int64
		failedRequests  int64
		totalBytes      int64
		minTime         int64 = int64(time.Hour)
		maxTime         int64
		totalTime       int64
	)

	startTime := time.Now()
	endTime := startTime.Add(duration)

	var wg sync.WaitGroup
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 启动并发goroutine
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for time.Now().Before(endTime) {
				reqStart := time.Now()

				var resp *http.Response
				var err error

				if requestBody != nil {
					resp, err = client.Post(url, "application/json", bytes.NewReader(requestBody))
				} else {
					resp, err = client.Get(url)
				}

				reqDuration := time.Since(reqStart)
				atomic.AddInt64(&totalRequests, 1)
				atomic.AddInt64(&totalTime, int64(reqDuration))

				// 更新最小和最大响应时间
				for {
					oldMin := atomic.LoadInt64(&minTime)
					if int64(reqDuration) >= oldMin {
						break
					}
					if atomic.CompareAndSwapInt64(&minTime, oldMin, int64(reqDuration)) {
						break
					}
				}

				for {
					oldMax := atomic.LoadInt64(&maxTime)
					if int64(reqDuration) <= oldMax {
						break
					}
					if atomic.CompareAndSwapInt64(&maxTime, oldMax, int64(reqDuration)) {
						break
					}
				}

				if err != nil {
					atomic.AddInt64(&failedRequests, 1)
					continue
				}

				if resp.StatusCode == 200 {
					atomic.AddInt64(&successRequests, 1)
					// 读取响应体以计算字节数
					body, _ := io.ReadAll(resp.Body)
					atomic.AddInt64(&totalBytes, int64(len(body)))
				} else {
					atomic.AddInt64(&failedRequests, 1)
				}

				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	actualDuration := time.Since(startTime)

	// 计算结果
	totalReqs := atomic.LoadInt64(&totalRequests)
	successReqs := atomic.LoadInt64(&successRequests)
	failedReqs := atomic.LoadInt64(&failedRequests)
	totalBytesTransferred := atomic.LoadInt64(&totalBytes)
	totalTimeNs := atomic.LoadInt64(&totalTime)

	var avgResponseTime time.Duration
	if totalReqs > 0 {
		avgResponseTime = time.Duration(totalTimeNs / totalReqs)
	}

	requestsPerSec := float64(totalReqs) / actualDuration.Seconds()
	throughputMBps := float64(totalBytesTransferred) / (1024 * 1024) / actualDuration.Seconds()

	return &BenchmarkResult{
		TotalRequests:   totalReqs,
		SuccessRequests: successReqs,
		FailedRequests:  failedReqs,
		TotalDuration:   actualDuration,
		AvgResponseTime: avgResponseTime,
		MinResponseTime: time.Duration(atomic.LoadInt64(&minTime)),
		MaxResponseTime: time.Duration(atomic.LoadInt64(&maxTime)),
		RequestsPerSec:  requestsPerSec,
		ThroughputMBps:  throughputMBps,
		TotalBytes:      totalBytesTransferred,
	}
}

// printResults 打印测试结果
func printResults(name string, result *BenchmarkResult) {
	fmt.Printf("\n=== %s ===\n", name)
	fmt.Printf("Total Requests:     %d\n", result.TotalRequests)
	fmt.Printf("Success Requests:   %d\n", result.SuccessRequests)
	fmt.Printf("Failed Requests:    %d\n", result.FailedRequests)
	fmt.Printf("Success Rate:       %.2f%%\n", float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("Total Duration:     %v\n", result.TotalDuration)
	fmt.Printf("Requests/sec:       %.2f\n", result.RequestsPerSec)
	fmt.Printf("Throughput:         %.2f MB/s\n", result.ThroughputMBps)
	fmt.Printf("Avg Response Time:  %v\n", result.AvgResponseTime)
	fmt.Printf("Min Response Time:  %v\n", result.MinResponseTime)
	fmt.Printf("Max Response Time:  %v\n", result.MaxResponseTime)
	fmt.Printf("Total Bytes:        %d\n", result.TotalBytes)
}

func main() {
	fmt.Println("NetCore-Go HTTP Server Benchmark")
	fmt.Println("=================================")

	// 创建HTTP服务器
	server := netcore.NewHTTPServer(
		netcore.WithReadBufferSize(8192),
		netcore.WithWriteBufferSize(8192),
		netcore.WithMaxConnections(2000),
		netcore.WithConnectionPool(true),
		netcore.WithMemoryPool(true),
		netcore.WithGoroutinePool(true),
	)

	// 创建处理器
	handler := &HTTPBenchmarkHandler{}
	server.SetHandler(handler)

	// 设置路由
	if httpServer, ok := server.(*netcorehttp.HTTPServer); ok {
		handler.setupBenchmarkRoutes(httpServer)
	}

	// 启动服务器
	go func() {
		if err := server.Start(":8084"); err != nil {
			fmt.Printf("Failed to start server: %v\n", err)
			return
		}
	}()

	// 等待服务器启动
	time.Sleep(2 * time.Second)
	fmt.Println("HTTP Server started on :8084")

	// 基准测试配置
	concurrency := 100
	duration := 30 * time.Second

	// 测试1: 简单GET请求
	result1 := runBenchmark("http://localhost:8084/hello", concurrency, duration, nil)
	printResults("Simple GET Request (/hello)", result1)

	// 测试2: JSON响应
	result2 := runBenchmark("http://localhost:8084/json", concurrency, duration, nil)
	printResults("JSON Response (/json)", result2)

	// 测试3: POST请求
	postData, _ := json.Marshal(map[string]string{"message": "Hello, World!"})
	result3 := runBenchmark("http://localhost:8084/echo", concurrency, duration, postData)
	printResults("POST Request (/echo)", result3)

	// 测试4: 大数据响应
	result4 := runBenchmark("http://localhost:8084/large", concurrency, duration, nil)
	printResults("Large Response (/large)", result4)

	// 打印服务器统计信息
	fmt.Println("\n=== Server Statistics ===")
	stats := server.GetStats()
	fmt.Printf("Active Connections:  %d\n", stats.ActiveConnections)
	fmt.Printf("Total Connections:   %d\n", stats.TotalConnections)
	fmt.Printf("Messages Received:   %d\n", stats.MessagesReceived)
	fmt.Printf("Messages Sent:       %d\n", stats.MessagesSent)
	fmt.Printf("Bytes Received:      %d\n", stats.BytesReceived)
	fmt.Printf("Bytes Sent:          %d\n", stats.BytesSent)
	fmt.Printf("Uptime:              %d seconds\n", stats.Uptime)
	fmt.Printf("Error Count:         %d\n", stats.ErrorCount)

	// 停止服务器
	fmt.Println("\nStopping server...")
	server.Stop()
	fmt.Println("Benchmark completed!")
}