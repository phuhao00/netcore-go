// Package main UDP性能测试示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/udp"
)

// BenchmarkHandler UDP性能测试处理器
type BenchmarkHandler struct {
	messageCount int64
	byteCount    int64
	startTime    time.Time
	mu           sync.RWMutex
}

// OnConnect 连接建立时调用
func (h *BenchmarkHandler) OnConnect(conn netcore.Connection) {
	// 性能测试中不打印连接信息，避免影响性能
}

// OnMessage 收到消息时调用
func (h *BenchmarkHandler) OnMessage(conn netcore.Connection, msg netcore.Message) {
	atomic.AddInt64(&h.messageCount, 1)
	atomic.AddInt64(&h.byteCount, int64(len(msg.Data)))
	
	// 简单回声，不做复杂处理
	conn.Send(msg.Data)
}

// OnDisconnect 连接断开时调用
func (h *BenchmarkHandler) OnDisconnect(conn netcore.Connection, err error) {
	// 性能测试中不处理断开连接
}

// GetStats 获取统计信息
func (h *BenchmarkHandler) GetStats() (int64, int64, time.Duration) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return atomic.LoadInt64(&h.messageCount), atomic.LoadInt64(&h.byteCount), time.Since(h.startTime)
}

// Reset 重置统计信息
func (h *BenchmarkHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	atomic.StoreInt64(&h.messageCount, 0)
	atomic.StoreInt64(&h.byteCount, 0)
	h.startTime = time.Now()
}

func main() {
	fmt.Println("NetCore-Go UDP Performance Benchmark")
	fmt.Println("=====================================")

	// 运行不同场景的性能测试
	runBenchmarkSuite()
}

// runBenchmarkSuite 运行完整的性能测试套件
func runBenchmarkSuite() {
	testCases := []struct {
		name        string
		clientCount int
		messageSize int
		duration    time.Duration
	}{
		{"Small Messages (64B)", 10, 64, 30 * time.Second},
		{"Medium Messages (1KB)", 10, 1024, 30 * time.Second},
		{"Large Messages (4KB)", 10, 4096, 30 * time.Second},
		{"High Concurrency (100 clients)", 100, 512, 30 * time.Second},
		{"Extreme Load (500 clients)", 500, 256, 20 * time.Second},
	}

	for i, tc := range testCases {
		fmt.Printf("\n[Test %d/%d] %s\n", i+1, len(testCases), tc.name)
		fmt.Printf("Clients: %d, Message Size: %d bytes, Duration: %v\n", 
			tc.clientCount, tc.messageSize, tc.duration)
		fmt.Println(strings.Repeat("-", 60))
		
		result := runBenchmark(tc.clientCount, tc.messageSize, tc.duration)
		printBenchmarkResult(result)
		
		// 测试间隔
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=====================================")
	fmt.Println("All benchmark tests completed!")
}

// BenchmarkResult 性能测试结果
type BenchmarkResult struct {
	ClientCount     int
	MessageSize     int
	Duration        time.Duration
	TotalMessages   int64
	TotalBytes      int64
	MessagesPerSec  float64
	MBytesPerSec    float64
	AvgLatency      time.Duration
	ServerStats     interface{}
}

// runBenchmark 运行单个性能测试
func runBenchmark(clientCount, messageSize int, duration time.Duration) *BenchmarkResult {
	// 创建性能测试处理器
	handler := &BenchmarkHandler{}
	handler.Reset()

	// 创建UDP服务器
	server := udp.NewServer()
	server.SetReadBufferSize(8192)
	server.SetWriteBufferSize(8192)

	server.SetHandler(handler)

	// 启动服务器
	addr := ":8082"
	if err := server.Start(addr); err != nil {
		log.Fatalf("Failed to start UDP server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建测试消息
	message := make([]byte, messageSize)
	for i := range message {
		message[i] = byte(i % 256)
	}

	// 启动客户端
	var wg sync.WaitGroup
	var totalMessages int64
	startTime := time.Now()

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			msgCount := runUDPClient(addr, message, duration)
			atomic.AddInt64(&totalMessages, msgCount)
		}(i)
	}

	// 等待所有客户端完成
	wg.Wait()
	actualDuration := time.Since(startTime)

	// 获取服务器统计信息
	var serverStats interface{} = nil
	_, handlerBytes, _ := handler.GetStats()

	// 计算性能指标
	messagesPerSec := float64(totalMessages) / actualDuration.Seconds()
	mBytesPerSec := float64(handlerBytes) / (1024 * 1024) / actualDuration.Seconds()
	avgLatency := actualDuration / time.Duration(totalMessages)

	return &BenchmarkResult{
		ClientCount:     clientCount,
		MessageSize:     messageSize,
		Duration:        actualDuration,
		TotalMessages:   totalMessages,
		TotalBytes:      handlerBytes,
		MessagesPerSec:  messagesPerSec,
		MBytesPerSec:    mBytesPerSec,
		AvgLatency:      avgLatency,
		ServerStats:     serverStats,
	}
}

// runUDPClient 运行UDP客户端
func runUDPClient(serverAddr string, message []byte, duration time.Duration) int64 {
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return 0
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return 0
	}
	defer conn.Close()

	// 设置缓冲区大小
	conn.SetReadBuffer(8192)
	conn.SetWriteBuffer(8192)

	var messageCount int64
	startTime := time.Now()
	buffer := make([]byte, len(message))

	for time.Since(startTime) < duration {
		// 发送消息
		if _, err := conn.Write(message); err != nil {
			continue
		}

		// 接收响应
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if _, err := conn.Read(buffer); err != nil {
			continue
		}

		messageCount++
	}

	return messageCount
}

// printBenchmarkResult 打印性能测试结果
func printBenchmarkResult(result *BenchmarkResult) {
	fmt.Printf("Results:\n")
	fmt.Printf("  Total Messages: %d\n", result.TotalMessages)
	fmt.Printf("  Total Bytes: %d (%.2f MB)\n", result.TotalBytes, float64(result.TotalBytes)/(1024*1024))
	fmt.Printf("  Messages/sec: %.2f\n", result.MessagesPerSec)
	fmt.Printf("  Throughput: %.2f MB/s\n", result.MBytesPerSec)
	fmt.Printf("  Avg Latency: %v\n", result.AvgLatency)
	fmt.Printf("  Actual Duration: %v\n", result.Duration)
	
	if result.ServerStats != nil {
		fmt.Printf("\nServer Stats:\n")
		fmt.Printf("  Active Connections: %d\n", result.ServerStats.ActiveConnections)
		fmt.Printf("  Total Connections: %d\n", result.ServerStats.TotalConnections)
		fmt.Printf("  Messages Received: %d\n", result.ServerStats.MessagesReceived)
		fmt.Printf("  Messages Sent: %d\n", result.ServerStats.MessagesSent)
		fmt.Printf("  Error Count: %d\n", result.ServerStats.ErrorCount)
	}
}

