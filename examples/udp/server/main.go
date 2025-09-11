// Package main UDP服务器示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	netcore "github.com/netcore-go"
	"github.com/netcore-go/pkg/core"
)

// EchoHandler UDP回声处理器
type EchoHandler struct{}

// OnConnect 连接建立时调用
func (h *EchoHandler) OnConnect(conn netcore.Connection) {
	fmt.Printf("[UDP] Client connected: %s\n", conn.RemoteAddr().String())
}

// OnMessage 收到消息时调用
func (h *EchoHandler) OnMessage(conn netcore.Connection, msg netcore.Message) {
	fmt.Printf("[UDP] Received from %s: %s\n", conn.RemoteAddr().String(), string(msg.Data))
	
	// 回声响应
	response := fmt.Sprintf("Echo: %s", string(msg.Data))
	if err := conn.Send([]byte(response)); err != nil {
		fmt.Printf("[UDP] Failed to send response: %v\n", err)
	}
}

// OnDisconnect 连接断开时调用
func (h *EchoHandler) OnDisconnect(conn netcore.Connection, err error) {
	if err != nil {
		fmt.Printf("[UDP] Client disconnected with error: %s, error: %v\n", conn.RemoteAddr().String(), err)
	} else {
		fmt.Printf("[UDP] Client disconnected: %s\n", conn.RemoteAddr().String())
	}
}

func main() {
	// 创建UDP服务器
	server := netcore.NewUDPServer(
		netcore.WithReadBufferSize(4096),
		netcore.WithWriteBufferSize(4096),
		netcore.WithMaxConnections(1000),
		netcore.WithHeartbeat(true, 30*time.Second),
		netcore.WithIdleTimeout(5*time.Minute),
		netcore.WithMetrics(true, "/metrics"),
	)

	// 设置消息处理器
	server.SetHandler(&EchoHandler{})

	// 设置中间件
	server.SetMiddleware(
		netcore.LoggingMiddleware(),
		netcore.MetricsMiddleware(),
		netcore.RecoveryMiddleware(),
	)

	// 启动服务器
	addr := ":8081"
	fmt.Printf("Starting UDP server on %s...\n", addr)
	if err := server.Start(addr); err != nil {
		log.Fatalf("Failed to start UDP server: %v", err)
	}

	fmt.Printf("UDP server started successfully on %s\n", addr)
	fmt.Println("Server statistics:")
	stats := server.GetStats()
	fmt.Printf("  - Start Time: %s\n", stats.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("  - Max Connections: %d\n", 1000)
	fmt.Printf("  - Heartbeat Enabled: %t\n", true)
	fmt.Printf("  - Metrics Enabled: %t\n", true)

	// 定期打印统计信息
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := server.GetStats()
				fmt.Printf("\n=== Server Statistics ===\n")
				fmt.Printf("Active Connections: %d\n", stats.ActiveConnections)
				fmt.Printf("Total Connections: %d\n", stats.TotalConnections)
				fmt.Printf("Messages Received: %d\n", stats.MessagesReceived)
				fmt.Printf("Messages Sent: %d\n", stats.MessagesSent)
				fmt.Printf("Bytes Received: %d\n", stats.BytesReceived)
				fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
				fmt.Printf("Uptime: %d seconds\n", stats.Uptime)
				fmt.Printf("Error Count: %d\n", stats.ErrorCount)
				if stats.LastError != "" {
					fmt.Printf("Last Error: %s\n", stats.LastError)
				}
				fmt.Printf("========================\n\n")
			}
		}
	}()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down UDP server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	} else {
		fmt.Println("UDP server stopped successfully")
	}

	// 打印最终统计信息
	stats = server.GetStats()
	fmt.Printf("\n=== Final Statistics ===\n")
	fmt.Printf("Total Connections: %d\n", stats.TotalConnections)
	fmt.Printf("Messages Received: %d\n", stats.MessagesReceived)
	fmt.Printf("Messages Sent: %d\n", stats.MessagesSent)
	fmt.Printf("Bytes Received: %d\n", stats.BytesReceived)
	fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
	fmt.Printf("Total Uptime: %d seconds\n", stats.Uptime)
	fmt.Printf("Error Count: %d\n", stats.ErrorCount)
	fmt.Printf("=======================\n")
}

