// Package main TCP客户端示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	// 连接到服务器
	addr := "localhost:8080"
	fmt.Printf("Connecting to TCP server at %s...\n", addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("Connected to server: %s\n", conn.RemoteAddr().String())
	fmt.Println("Type messages to send to server (type 'quit' to exit):")

	// 启动接收消息的goroutine
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				fmt.Printf("Connection closed by server: %v\n", err)
				return
			}
			if n > 0 {
				fmt.Printf("[%s] Server: %s\n", 
					time.Now().Format("15:04:05"), 
					string(buffer[:n]))
			}
		}
	}()

	// 读取用户输入并发送消息
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		message := strings.TrimSpace(scanner.Text())
		if message == "" {
			continue
		}

		if strings.ToLower(message) == "quit" {
			fmt.Println("Disconnecting...")
			break
		}

		// 发送消息到服务器
		_, err := conn.Write([]byte(message))
		if err != nil {
			fmt.Printf("Failed to send message: %v\n", err)
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}

	fmt.Println("Client disconnected")
}