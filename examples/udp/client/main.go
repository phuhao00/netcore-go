// Package main UDP客户端示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	// 连接到UDP服务器
	serverAddr := "localhost:8081"
	fmt.Printf("Connecting to UDP server at %s...\n", serverAddr)

	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Fatalf("Failed to connect to UDP server: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Connected to UDP server at %s\n", serverAddr)
	fmt.Printf("Local address: %s\n", conn.LocalAddr().String())
	fmt.Printf("Remote address: %s\n", conn.RemoteAddr().String())
	fmt.Println("Type messages to send (type 'quit' to exit, 'test' for automated test):")

	// 启动接收goroutine
	go receiveMessages(conn)

	// 读取用户输入并发送消息
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "quit" {
			fmt.Println("Exiting...")
			break
		}

		if input == "test" {
			runAutomatedTest(conn)
			continue
		}

		// 发送消息
		if err := sendMessage(conn, input); err != nil {
			fmt.Printf("Failed to send message: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
}

// receiveMessages 接收服务器消息
func receiveMessages(conn *net.UDPConn) {
	buffer := make([]byte, 4096)
	for {
		// 设置读超时
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		
		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时继续
			}
			fmt.Printf("Error receiving message: %v\n", err)
			return
		}

		if n > 0 {
			message := string(buffer[:n])
			fmt.Printf("\n<< Received: %s\n> ", message)
		}
	}
}

// sendMessage 发送消息到服务器
func sendMessage(conn *net.UDPConn, message string) error {
	data := []byte(message)
	n, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: sent %d bytes, expected %d", n, len(data))
	}

	fmt.Printf(">> Sent: %s\n", message)
	return nil
}

// runAutomatedTest 运行自动化测试
func runAutomatedTest(conn *net.UDPConn) {
	fmt.Println("\n=== Running Automated Test ===")
	
	testMessages := []string{
		"Hello, UDP Server!",
		"This is a test message",
		"UDP is connectionless protocol",
		"NetCore-Go UDP implementation",
		"Performance test message",
	}

	for i, msg := range testMessages {
		fmt.Printf("Test %d/%d: ", i+1, len(testMessages))
		if err := sendMessage(conn, msg); err != nil {
			fmt.Printf("Failed to send test message: %v\n", err)
			continue
		}
		
		// 等待响应
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n=== Automated Test Completed ===")
	fmt.Println("You can continue sending manual messages or type 'quit' to exit.")
}

// performanceTest 性能测试
func performanceTest(conn *net.UDPConn, messageCount int) {
	fmt.Printf("\n=== Performance Test (%d messages) ===\n", messageCount)
	
	start := time.Now()
	message := "Performance test message - this is a sample message for testing UDP throughput"
	
	for i := 0; i < messageCount; i++ {
		testMsg := fmt.Sprintf("%s #%d", message, i+1)
		if err := sendMessage(conn, testMsg); err != nil {
			fmt.Printf("Failed to send message %d: %v\n", i+1, err)
			break
		}
		
		// 小延迟避免过快发送
		if i%100 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}
	
	duration := time.Since(start)
	fmt.Printf("\nPerformance Test Results:\n")
	fmt.Printf("Messages sent: %d\n", messageCount)
	fmt.Printf("Total time: %v\n", duration)
	fmt.Printf("Messages per second: %.2f\n", float64(messageCount)/duration.Seconds())
	fmt.Printf("Average latency: %v\n", duration/time.Duration(messageCount))
	fmt.Println("=== Performance Test Completed ===")
}