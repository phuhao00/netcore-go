// Package main TCP回声服务器示例
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

	"github.com/netcore-go/netcore-go"
	"github.com/netcore-go/netcore-go/pkg/core"
)

// EchoHandler 回声消息处理器
type EchoHandler struct{}

// OnConnect 连接建立时调用
func (h *EchoHandler) OnConnect(conn netcore.Connection) {
	fmt.Printf("[%s] Client connected: %s\n", 
		time.Now().Format("2006-01-02 15:04:05"), 
		conn.RemoteAddr().String())

	// 发送欢迎消息
	welcomeMsg := netcore.NewMessage(netcore.MessageTypeText, []byte("Welcome to NetCore-Go Echo Server!"))
	if err := conn.SendMessage(*welcomeMsg); err != nil {
		fmt.Printf("Failed to send welcome message: %v\n", err)
	}
}

// OnMessage 收到消息时调用
func (h *EchoHandler) OnMessage(conn netcore.Connection, msg netcore.Message) {
	fmt.Printf("[%s] Received from %s: %s\n", 
		time.Now().Format("2006-01-02 15:04:05"), 
		conn.RemoteAddr().String(), 
		string(msg.Data))

	// 回声消息
	echoMsg := netcore.NewMessage(msg.Type, append([]byte("Echo: "), msg.Data...))
	if err := conn.SendMessage(*echoMsg); err != nil {
		fmt.Printf("Failed to send echo message: %v\n", err)
	}
}

// OnDisconnect 连接断开时调用
func (h *EchoHandler) OnDisconnect(conn netcore.Connection, err error) {
	if err != nil {
		fmt.Printf("[%s] Client disconnected with error: %s, error: %v\n", 
			time.Now().Format("2006-01-02 15:04:05"), 
			conn.RemoteAddr().String(), err)
	} else {
		fmt.Printf("[%s] Client disconnected: %s\n", 
			time.Now().Format("2006-01-02 15:04:05"), 
			conn.RemoteAddr().String())
	}
}

func main() {
	// 创建TCP服务器
	server := netcore.NewTCPServer(
		netcore.WithReadBufferSize(4096),
		netcore.WithWriteBufferSize(4096),
		netcore.WithMaxConnections(1000),
		netcore.WithReadTimeout(30*time.Second),
		netcore.WithWriteTimeout(30*time.Second),
		netcore.WithIdleTimeout(300*time.Second),
		netcore.WithHeartbeat(true, 30*time.Second),
		netcore.WithMetrics(true, "/metrics"),
	)

	// 设置消息处理器
	server.SetHandler(&EchoHandler{})

	// 设置中间件
	server.SetMiddleware(
		netcore.RecoveryMiddleware(),
		netcore.LoggingMiddleware(),
		netcore.MetricsMiddleware(),
		netcore.RateLimitMiddleware(100), // 每个连接最多100个请求
	)

	// 启动服务器
	addr := ":8080"
	fmt.Printf("Starting TCP Echo Server on %s...\n", addr)
	if err := server.Start(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	fmt.Printf("TCP Echo Server started successfully on %s\n", addr)
	fmt.Println("Press Ctrl+C to stop the server")

	// 启动统计信息打印goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := server.GetStats()
				fmt.Printf("[Stats] Active: %d, Total: %d, Messages: %d/%d, Bytes: %d/%d, Errors: %d\n",
					stats.ActiveConnections,
					stats.TotalConnections,
					stats.MessagesReceived,
					stats.MessagesSent,
					stats.BytesReceived,
					stats.BytesSent,
					stats.ErrorCount)
			}
		}
	}()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
	fmt.Println("Server stopped successfully")
}