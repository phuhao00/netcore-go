// Package main WebSocket服务器示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netcore-go"
)

// WebSocketEchoHandler WebSocket回声处理器
type WebSocketEchoHandler struct{}

// OnConnect 连接建立时调用
func (h *WebSocketEchoHandler) OnConnect(conn netcore.Connection) {
	fmt.Printf("[WebSocket] Client connected: %s\n", conn.RemoteAddr())
	
	// 发送欢迎消息
	welcomeMsg := netcore.NewMessage(netcore.MessageTypeText, []byte("Welcome to NetCore-Go WebSocket Server!"))
	if err := conn.SendMessage(*welcomeMsg); err != nil {
		fmt.Printf("Failed to send welcome message: %v\n", err)
	}
}

// OnMessage 收到消息时调用
func (h *WebSocketEchoHandler) OnMessage(conn netcore.Connection, msg netcore.Message) {
	fmt.Printf("[WebSocket] Received %s message from %s: %s\n", 
		msg.Type.String(), conn.RemoteAddr(), string(msg.Data))
	
	// 回声消息
	response := netcore.NewMessage(msg.Type, append([]byte("Echo: "), msg.Data...))
	if err := conn.SendMessage(*response); err != nil {
		fmt.Printf("Failed to send echo message: %v\n", err)
	}
}

// OnDisconnect 连接断开时调用
func (h *WebSocketEchoHandler) OnDisconnect(conn netcore.Connection, err error) {
	if err != nil {
		fmt.Printf("[WebSocket] Client disconnected with error %s: %v\n", conn.RemoteAddr(), err)
	} else {
		fmt.Printf("[WebSocket] Client disconnected: %s\n", conn.RemoteAddr())
	}
}

func main() {
	fmt.Println("Starting NetCore-Go WebSocket Server Example...")
	
	// 创建WebSocket服务器
	server := netcore.NewWebSocketServer(
		netcore.WithReadBufferSize(4096),
		netcore.WithWriteBufferSize(4096),
		netcore.WithMaxConnections(1000),
		netcore.WithHeartbeat(true, 30*time.Second),
		netcore.WithIdleTimeout(5*time.Minute),
		netcore.WithConnectionPool(true),
		netcore.WithMemoryPool(true),
		netcore.WithGoroutinePool(true),
	)
	
	// 设置消息处理器
	server.SetHandler(&WebSocketEchoHandler{})
	
	// 启动服务器
	go func() {
		fmt.Println("WebSocket server listening on :8082")
		if err := server.Start(":8082"); err != nil {
			log.Fatalf("Failed to start WebSocket server: %v", err)
		}
	}()
	
	// 启动统计信息打印
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
					stats.ErrorCount,
				)
			}
		}
	}()
	
	fmt.Println("WebSocket server started successfully!")
	fmt.Println("You can test it by opening the HTML client in examples/websocket/client/index.html")
	fmt.Println("Press Ctrl+C to stop the server")
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("\nShutting down WebSocket server...")
	if err := server.Stop(); err != nil {
		fmt.Printf("Error stopping server: %v\n", err)
	} else {
		fmt.Println("WebSocket server stopped successfully")
	}
}

