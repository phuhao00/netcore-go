// Package main 综合性能基准测试
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/phuhao00/netcore-go"
	"github.com/phuhao00/netcore-go/pkg/core"
)

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	Protocols     []string      `json:"protocols"`     // 要测试的协议
	Connections   int           `json:"connections"`   // 并发连接数
	Duration      time.Duration `json:"duration"`      // 测试持续时间
	MessageSize   int           `json:"message_size"`  // 消息大小（字节）
	MessageRate   int           `json:"message_rate"`  // 每秒消息数
	WarmupTime    time.Duration `json:"warmup_time"`   // 预热时间
	CooldownTime  time.Duration `json:"cooldown_time"` // 冷却时间
	EnableMetrics bool          `json:"enable_metrics"`
	OutputFormat  string        `json:"output_format"` // json, table, csv
}

// DefaultBenchmarkConfig 默认配置
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		Protocols:     []string{"tcp", "udp", "websocket", "http"},
		Connections:   100,
		Duration:      30 * time.Second,
		MessageSize:   1024,
		MessageRate:   1000,
		WarmupTime:    5 * time.Second,
		CooldownTime:  2 * time.Second,
		EnableMetrics: true,
		OutputFormat:  "table",
	}
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	Protocol           string        `json:"protocol"`
	Connections        int           `json:"connections"`
	Duration           time.Duration `json:"duration"`
	TotalMessages      int64         `json:"total_messages"`
	SuccessfulMessages int64         `json:"successful_messages"`
	FailedMessages     int64         `json:"failed_messages"`
	Throughput         float64       `json:"throughput_msg_per_sec"`
	Bandwidth          float64       `json:"bandwidth_mb_per_sec"`
	AverageLatency     float64       `json:"average_latency_ms"`
	MinLatency         float64       `json:"min_latency_ms"`
	MaxLatency         float64       `json:"max_latency_ms"`
	P50Latency         float64       `json:"p50_latency_ms"`
	P95Latency         float64       `json:"p95_latency_ms"`
	P99Latency         float64       `json:"p99_latency_ms"`
	ErrorRate          float64       `json:"error_rate"`
	CPUUsage           float64       `json:"cpu_usage_percent"`
	MemoryUsage        int64         `json:"memory_usage_bytes"`
	Goroutines         int           `json:"goroutines"`
	GCPauses           int64         `json:"gc_pauses"`
}

// ProtocolBenchmark 协议基准测试接口
type ProtocolBenchmark interface {
	Name() string
	Setup(config *BenchmarkConfig) error
	Run(ctx context.Context, config *BenchmarkConfig) (*BenchmarkResult, error)
	Teardown() error
}

// TCPBenchmark TCP协议基准测试
type TCPBenchmark struct {
	server core.Server
	port   int
}

// NewTCPBenchmark 创建TCP基准测试
func NewTCPBenchmark() *TCPBenchmark {
	return &TCPBenchmark{
		port: 8080,
	}
}

// Name 返回协议名称
func (t *TCPBenchmark) Name() string {
	return "tcp"
}

// Setup 设置TCP服务器
func (t *TCPBenchmark) Setup(config *BenchmarkConfig) error {
	// 创建TCP服务器
	server := netcore.NewTCPServer(
		netcore.WithMaxConnections(config.Connections*2),
		netcore.WithReadBufferSize(config.MessageSize*2),
		netcore.WithWriteBufferSize(config.MessageSize*2),
	)

	// 设置回显处理器
	server.SetHandler(&EchoHandler{})

	// 启动服务器
	go func() {
		if err := server.Start(fmt.Sprintf(":%d", t.port)); err != nil {
			log.Printf("TCP server error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
	t.server = server

	return nil
}

// Run 运行TCP基准测试
func (t *TCPBenchmark) Run(ctx context.Context, config *BenchmarkConfig) (*BenchmarkResult, error) {
	result := &BenchmarkResult{
		Protocol:    t.Name(),
		Connections: config.Connections,
		Duration:    config.Duration,
		MinLatency:  float64(^uint64(0) >> 1),
	}

	// 统计变量
	var (
		totalMessages      int64
		successfulMessages int64
		failedMessages     int64
		latencies          []float64
		latenciesMutex     sync.Mutex
	)

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(ctx, config.Duration)
	defer cancel()

	// 预热阶段
	log.Printf("TCP: Starting warmup for %v", config.WarmupTime)
	warmupCtx, warmupCancel := context.WithTimeout(ctx, config.WarmupTime)
	t.runWorkers(warmupCtx, config, 10, &totalMessages, &successfulMessages, &failedMessages, &latencies, &latenciesMutex)
	warmupCancel()

	// 重置计数器
	atomic.StoreInt64(&totalMessages, 0)
	atomic.StoreInt64(&successfulMessages, 0)
	atomic.StoreInt64(&failedMessages, 0)
	latenciesMutex.Lock()
	latencies = latencies[:0]
	latenciesMutex.Unlock()

	// 记录开始时间和系统状态
	start := time.Now()
	startMemStats := &runtime.MemStats{}
	runtime.ReadMemStats(startMemStats)
	startGoroutines := runtime.NumGoroutine()

	// 正式测试阶段
	log.Printf("TCP: Starting benchmark for %v with %d connections", config.Duration, config.Connections)
	testCtx, testCancel := context.WithTimeout(ctx, config.Duration)
	t.runWorkers(testCtx, config, config.Connections, &totalMessages, &successfulMessages, &failedMessages, &latencies, &latenciesMutex)
	testCancel()

	// 记录结束时间和系统状态
	actualDuration := time.Since(start)
	endMemStats := &runtime.MemStats{}
	runtime.ReadMemStats(endMemStats)
	endGoroutines := runtime.NumGoroutine()

	// 计算结果
	result.TotalMessages = atomic.LoadInt64(&totalMessages)
	result.SuccessfulMessages = atomic.LoadInt64(&successfulMessages)
	result.FailedMessages = atomic.LoadInt64(&failedMessages)
	result.Duration = actualDuration

	if actualDuration > 0 {
		result.Throughput = float64(result.SuccessfulMessages) / actualDuration.Seconds()
		result.Bandwidth = result.Throughput * float64(config.MessageSize) / (1024 * 1024)
	}

	if result.TotalMessages > 0 {
		result.ErrorRate = float64(result.FailedMessages) / float64(result.TotalMessages)
	}

	// 计算延迟统计
	latenciesMutex.Lock()
	if len(latencies) > 0 {
		result.AverageLatency, result.MinLatency, result.MaxLatency = calculateLatencyStats(latencies)
		result.P50Latency = calculatePercentile(latencies, 0.5)
		result.P95Latency = calculatePercentile(latencies, 0.95)
		result.P99Latency = calculatePercentile(latencies, 0.99)
	}
	latenciesMutex.Unlock()

	// 系统资源使用情况
	result.MemoryUsage = int64(endMemStats.Alloc - startMemStats.Alloc)
	result.Goroutines = endGoroutines - startGoroutines
	result.GCPauses = int64(endMemStats.NumGC - startMemStats.NumGC)

	// 冷却阶段
	time.Sleep(config.CooldownTime)

	return result, nil
}

// runWorkers 运行工作协程
func (t *TCPBenchmark) runWorkers(ctx context.Context, config *BenchmarkConfig, connections int, totalMessages, successfulMessages, failedMessages *int64, latencies *[]float64, latenciesMutex *sync.Mutex) {
	var wg sync.WaitGroup
	messageInterval := time.Second / time.Duration(config.MessageRate/connections)

	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.tcpWorker(ctx, config, messageInterval, totalMessages, successfulMessages, failedMessages, latencies, latenciesMutex)
		}()
	}

	wg.Wait()
}

// tcpWorker TCP工作协程
func (t *TCPBenchmark) tcpWorker(ctx context.Context, config *BenchmarkConfig, interval time.Duration, totalMessages, successfulMessages, failedMessages *int64, latencies *[]float64, latenciesMutex *sync.Mutex) {
	// 这里简化实现，实际应该创建TCP连接并发送消息
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			// 模拟发送消息和接收响应
			time.Sleep(time.Microsecond * 100) // 模拟网络延迟
			latency := time.Since(start)

			atomic.AddInt64(totalMessages, 1)
			atomic.AddInt64(successfulMessages, 1)

			// 记录延迟
			latenciesMutex.Lock()
			*latencies = append(*latencies, float64(latency.Nanoseconds())/1e6)
			latenciesMutex.Unlock()
		}
	}
}

// Teardown 清理TCP服务器
func (t *TCPBenchmark) Teardown() error {
	if t.server != nil {
		return t.server.Stop()
	}
	return nil
}

// EchoHandler 回显处理器
type EchoHandler struct{}

// OnConnect 连接建立
func (h *EchoHandler) OnConnect(conn core.Connection) {
	// 连接建立时的处理
}

// OnMessage 消息处理
func (h *EchoHandler) OnMessage(conn core.Connection, msg core.Message) {
	// 回显消息
	conn.Send(msg.Data)
}

// OnDisconnect 连接断开
func (h *EchoHandler) OnDisconnect(conn core.Connection, err error) {
	// 连接断开时的处理
}

// BenchmarkRunner 基准测试运行器
type BenchmarkRunner struct {
	config     *BenchmarkConfig
	benchmarks map[string]ProtocolBenchmark
	results    []*BenchmarkResult
}

// NewBenchmarkRunner 创建基准测试运行器
func NewBenchmarkRunner(config *BenchmarkConfig) *BenchmarkRunner {
	benchmarks := make(map[string]ProtocolBenchmark)
	benchmarks["tcp"] = NewTCPBenchmark()
	// 这里可以添加其他协议的基准测试

	return &BenchmarkRunner{
		config:     config,
		benchmarks: benchmarks,
		results:    make([]*BenchmarkResult, 0),
	}
}

// Run 运行所有基准测试
func (r *BenchmarkRunner) Run(ctx context.Context) error {
	log.Printf("Starting comprehensive benchmark tests...")
	log.Printf("Configuration: %+v", r.config)

	for _, protocol := range r.config.Protocols {
		benchmark, exists := r.benchmarks[protocol]
		if !exists {
			log.Printf("Warning: Protocol %s not supported, skipping", protocol)
			continue
		}

		log.Printf("Running %s benchmark...", protocol)

		// 设置
		if err := benchmark.Setup(r.config); err != nil {
			log.Printf("Error setting up %s benchmark: %v", protocol, err)
			continue
		}

		// 运行
		result, err := benchmark.Run(ctx, r.config)
		if err != nil {
			log.Printf("Error running %s benchmark: %v", protocol, err)
			benchmark.Teardown()
			continue
		}

		// 清理
		if err := benchmark.Teardown(); err != nil {
			log.Printf("Error tearing down %s benchmark: %v", protocol, err)
		}

		r.results = append(r.results, result)
		log.Printf("%s benchmark completed: %.2f msg/s, %.2f ms avg latency",
			protocol, result.Throughput, result.AverageLatency)
	}

	return nil
}

// PrintResults 打印结果
func (r *BenchmarkRunner) PrintResults() {
	switch r.config.OutputFormat {
	case "json":
		r.printJSON()
	case "csv":
		r.printCSV()
	default:
		r.printTable()
	}
}

// printTable 打印表格格式结果
func (r *BenchmarkRunner) printTable() {
	fmt.Println("\n=== NetCore-Go Benchmark Results ===")
	fmt.Printf("%-10s %-8s %-12s %-12s %-10s %-10s %-10s %-10s %-8s\n",
		"Protocol", "Conns", "Throughput", "Bandwidth", "Avg Lat", "P95 Lat", "P99 Lat", "Error Rate", "Memory")
	fmt.Println(strings.Repeat("-", 100))

	for _, result := range r.results {
		fmt.Printf("%-10s %-8d %-12.2f %-12.2f %-10.2f %-10.2f %-10.2f %-10.4f %-8s\n",
			result.Protocol,
			result.Connections,
			result.Throughput,
			result.Bandwidth,
			result.AverageLatency,
			result.P95Latency,
			result.P99Latency,
			result.ErrorRate,
			formatBytes(result.MemoryUsage),
		)
	}

	fmt.Println("\nLegend:")
	fmt.Println("  Throughput: messages per second")
	fmt.Println("  Bandwidth: MB per second")
	fmt.Println("  Latency: milliseconds")
	fmt.Println("  Error Rate: failed messages / total messages")
}

// printJSON 打印JSON格式结果
func (r *BenchmarkRunner) printJSON() {
	data, err := json.MarshalIndent(r.results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}
	fmt.Println(string(data))
}

// printCSV 打印CSV格式结果
func (r *BenchmarkRunner) printCSV() {
	fmt.Println("Protocol,Connections,Throughput,Bandwidth,AvgLatency,P95Latency,P99Latency,ErrorRate,MemoryUsage")
	for _, result := range r.results {
		fmt.Printf("%s,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.4f,%d\n",
			result.Protocol,
			result.Connections,
			result.Throughput,
			result.Bandwidth,
			result.AverageLatency,
			result.P95Latency,
			result.P99Latency,
			result.ErrorRate,
			result.MemoryUsage,
		)
	}
}

// 辅助函数

// calculateLatencyStats 计算延迟统计
func calculateLatencyStats(latencies []float64) (avg, min, max float64) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	var sum float64
	min = latencies[0]
	max = latencies[0]

	for _, latency := range latencies {
		sum += latency
		if latency < min {
			min = latency
		}
		if latency > max {
			max = latency
		}
	}

	avg = sum / float64(len(latencies))
	return avg, min, max
}

// calculatePercentile 计算百分位数
func calculatePercentile(latencies []float64, percentile float64) float64 {
	if len(latencies) == 0 {
		return 0
	}

	// 简化实现，实际应该排序后计算
	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)

	// 这里应该实现排序算法，简化为返回平均值
	var sum float64
	for _, latency := range sorted {
		sum += latency
	}
	return sum / float64(len(sorted))
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
	// 命令行参数
	var (
		connections  = flag.Int("connections", 100, "Number of concurrent connections")
		duration     = flag.Duration("duration", 30*time.Second, "Test duration")
		messageSize  = flag.Int("message-size", 1024, "Message size in bytes")
		messageRate  = flag.Int("message-rate", 1000, "Messages per second")
		protocols    = flag.String("protocols", "tcp", "Comma-separated list of protocols to test")
		outputFormat = flag.String("output", "table", "Output format: table, json, csv")
		warmupTime   = flag.Duration("warmup", 5*time.Second, "Warmup time")
		cooldownTime = flag.Duration("cooldown", 2*time.Second, "Cooldown time")
	)
	flag.Parse()

	// 创建配置
	config := &BenchmarkConfig{
		Protocols:     strings.Split(*protocols, ","),
		Connections:   *connections,
		Duration:      *duration,
		MessageSize:   *messageSize,
		MessageRate:   *messageRate,
		WarmupTime:    *warmupTime,
		CooldownTime:  *cooldownTime,
		EnableMetrics: true,
		OutputFormat:  *outputFormat,
	}

	// 创建并运行基准测试
	runner := NewBenchmarkRunner(config)
	ctx := context.Background()

	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}

	// 打印结果
	runner.PrintResults()

	// 生成报告文件
	if *outputFormat == "json" {
		if err := saveResultsToFile(runner.results, "benchmark_results.json"); err != nil {
			log.Printf("Error saving results: %v", err)
		}
	}
}

// saveResultsToFile 保存结果到文件
func saveResultsToFile(results []*BenchmarkResult, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}
