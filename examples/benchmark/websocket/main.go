// Package main WebSocket性能测试示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	netcore "github.com/netcore-go/netcore-go"
)

// BenchmarkHandler WebSocket性能测试处理器
type BenchmarkHandler struct {
	messageCount int64
	byteCount    int64
	startTime    time.Time
	mu           sync.RWMutex
	connections  map[string]netcore.Connection
}

// NewBenchmarkHandler 创建性能测试处理器
func NewBenchmarkHandler() *BenchmarkHandler {
	return &BenchmarkHandler{
		startTime:   time.Now(),
		connections: make(map[string]netcore.Connection),
	}
}

// OnConnect 连接建立时调用
func (h *BenchmarkHandler) OnConnect(conn netcore.Connection) {
	h.mu.Lock()
	h.connections[conn.ID()] = conn
	h.mu.Unlock()
	
	fmt.Printf("Client connected: %s (Total: %d)\n", conn.RemoteAddr(), len(h.connections))
}

// OnMessage 收到消息时调用
func (h *BenchmarkHandler) OnMessage(conn netcore.Connection, msg netcore.Message) {
	atomic.AddInt64(&h.messageCount, 1)
	atomic.AddInt64(&h.byteCount, int64(len(msg.Data)))
	
	// 简单回声，用于性能测试
	response := netcore.NewMessage(msg.Type, msg.Data)
	conn.SendMessage(*response)
}

// OnDisconnect 连接断开时调用
func (h *BenchmarkHandler) OnDisconnect(conn netcore.Connection, err error) {
	h.mu.Lock()
	delete(h.connections, conn.ID())
	h.mu.Unlock()
	
	if err != nil {
		fmt.Printf("Client disconnected with error %s: %v (Total: %d)\n", 
			conn.RemoteAddr(), err, len(h.connections))
	} else {
		fmt.Printf("Client disconnected: %s (Total: %d)\n", 
			conn.RemoteAddr(), len(h.connections))
	}
}

// GetStats 获取统计信息
func (h *BenchmarkHandler) GetStats() (int64, int64, int, time.Duration) {
	h.mu.RLock()
	connCount := len(h.connections)
	h.mu.RUnlock()
	
	msgCount := atomic.LoadInt64(&h.messageCount)
	byteCount := atomic.LoadInt64(&h.byteCount)
	duration := time.Since(h.startTime)
	
	return msgCount, byteCount, connCount, duration
}

// Broadcast 广播消息到所有连接
func (h *BenchmarkHandler) Broadcast(msg netcore.Message) {
	h.mu.RLock()
	connections := make([]netcore.Connection, 0, len(h.connections))
	for _, conn := range h.connections {
		connections = append(connections, conn)
	}
	h.mu.RUnlock()
	
	for _, conn := range connections {
		if conn.IsActive() {
			conn.SendMessage(msg)
		}
	}
}

func main() {
	fmt.Println("Starting NetCore-Go WebSocket Benchmark Server...")
	
	// 创建性能测试处理器
	handler := NewBenchmarkHandler()
	
	// 创建WebSocket服务器（高性能配置）
	server := netcore.NewWebSocketServer(
		netcore.WithReadBufferSize(8192),
		netcore.WithWriteBufferSize(8192),
		netcore.WithMaxConnections(10000),
		netcore.WithHeartbeat(true, 60*time.Second),
		netcore.WithIdleTimeout(10*time.Minute),
		netcore.WithConnectionPool(true),
		netcore.WithMemoryPool(true),
		netcore.WithGoroutinePool(true),
	)
	
	// 设置消息处理器
	server.SetHandler(handler)
	
	// 启动服务器
	go func() {
		fmt.Println("WebSocket benchmark server listening on :8083")
		if err := server.Start(":8083"); err != nil {
			log.Fatalf("Failed to start WebSocket server: %v", err)
		}
	}()
	
	// 启动性能统计
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				msgCount, byteCount, connCount, duration := handler.GetStats()
				serverStats := server.GetStats()
				
				// 计算性能指标
				seconds := duration.Seconds()
				msgPerSec := float64(msgCount) / seconds
				bytesPerSec := float64(byteCount) / seconds
				mbPerSec := bytesPerSec / (1024 * 1024)
				
				fmt.Printf("\n=== WebSocket Performance Stats ===\n")
				fmt.Printf("Runtime: %.1fs\n", seconds)
				fmt.Printf("Active Connections: %d\n", connCount)
				fmt.Printf("Total Connections: %d\n", serverStats.TotalConnections)
				fmt.Printf("Messages: %d (%.1f msg/s)\n", msgCount, msgPerSec)
				fmt.Printf("Bytes: %d (%.2f MB/s)\n", byteCount, mbPerSec)
				fmt.Printf("Server Messages: %d/%d\n", serverStats.MessagesReceived, serverStats.MessagesSent)
				fmt.Printf("Server Bytes: %d/%d\n", serverStats.BytesReceived, serverStats.BytesSent)
				fmt.Printf("Errors: %d\n", serverStats.ErrorCount)
				if serverStats.LastError != "" {
					fmt.Printf("Last Error: %s\n", serverStats.LastError)
				}
				fmt.Printf("================================\n")
			}
		}
	}()
	
	// 启动广播测试
	go func() {
		time.Sleep(10 * time.Second) // 等待连接建立
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// 广播测试消息
				broadcastMsg := netcore.NewMessage(
					netcore.MessageTypeText,
					[]byte(fmt.Sprintf("Broadcast message at %s", time.Now().Format("15:04:05"))),
				)
				handler.Broadcast(*broadcastMsg)
				fmt.Println("Broadcast message sent to all connections")
			}
		}
	}()
	
	fmt.Println("WebSocket benchmark server started successfully!")
	fmt.Println("Performance test instructions:")
	fmt.Println("1. Use WebSocket client tools to connect to ws://localhost:8083")
	fmt.Println("2. Send messages to test echo performance")
	fmt.Println("3. Monitor the performance statistics printed every 5 seconds")
	fmt.Println("4. Test with multiple concurrent connections for load testing")
	fmt.Println("")
	fmt.Println("Recommended test scenarios:")
	fmt.Println("- Single connection high-frequency messaging")
	fmt.Println("- Multiple connections concurrent messaging")
	fmt.Println("- Large message size testing")
	fmt.Println("- Connection stability testing")
	fmt.Println("")
	fmt.Println("Press Ctrl+C to stop the server")
	
	// 保持服务器运行
	select {}
}