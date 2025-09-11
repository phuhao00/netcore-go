// Package main TCP性能测试示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go"
	"github.com/netcore-go/pkg/core"
)

const (
	// 测试参数
	serverAddr    = "localhost:8080"
	concurrency   = 100  // 并发连接数
	messagesPerConn = 1000 // 每个连接发送的消息数
	messageSize   = 1024 // 消息大小（字节）
)

var (
	totalMessages int64
	totalBytes    int64
	errorCount    int64
)

func main() {
	fmt.Println("NetCore-Go TCP Performance Benchmark")
	fmt.Printf("Server: %s\n", serverAddr)
	fmt.Printf("Concurrency: %d\n", concurrency)
	fmt.Printf("Messages per connection: %d\n", messagesPerConn)
	fmt.Printf("Message size: %d bytes\n", messageSize)
	fmt.Println("Starting benchmark...")

	// 准备测试数据
	testData := make([]byte, messageSize)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// 记录开始时间
	startTime := time.Now()

	// 启动并发测试
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			runClient(clientID, testData)
		}(i)
	}

	// 等待所有客户端完成
	wg.Wait()

	// 计算结果
	duration := time.Since(startTime)
	totalMsgs := atomic.LoadInt64(&totalMessages)
	totalBts := atomic.LoadInt64(&totalBytes)
	errors := atomic.LoadInt64(&errorCount)

	// 输出结果
	fmt.Println("\n=== Benchmark Results ===")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Total messages: %d\n", totalMsgs)
	fmt.Printf("Total bytes: %d (%.2f MB)\n", totalBts, float64(totalBts)/(1024*1024))
	fmt.Printf("Errors: %d\n", errors)
	fmt.Printf("Messages/sec: %.2f\n", float64(totalMsgs)/duration.Seconds())
	fmt.Printf("Bytes/sec: %.2f MB/s\n", float64(totalBts)/(1024*1024)/duration.Seconds())
	fmt.Printf("Average latency: %.2f ms\n", float64(duration.Nanoseconds())/float64(totalMsgs)/1000000)
	fmt.Printf("Success rate: %.2f%%\n", float64(totalMsgs-errors)/float64(totalMsgs)*100)
}

func runClient(clientID int, testData []byte) {
	// 连接到服务器
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Printf("Client %d: Failed to connect: %v", clientID, err)
		atomic.AddInt64(&errorCount, int64(messagesPerConn))
		return
	}
	defer conn.Close()

	// 启动接收协程
	var recvCount int64
	recvDone := make(chan bool)
	go func() {
		buffer := make([]byte, messageSize*2)
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				break
			}
			if n > 0 {
				atomic.AddInt64(&recvCount, 1)
				atomic.AddInt64(&totalBytes, int64(n))
				if atomic.LoadInt64(&recvCount) >= int64(messagesPerConn) {
					break
				}
			}
		}
		recvDone <- true
	}()

	// 发送消息
	for i := 0; i < messagesPerConn; i++ {
		_, err := conn.Write(testData)
		if err != nil {
			log.Printf("Client %d: Failed to send message %d: %v", clientID, i, err)
			atomic.AddInt64(&errorCount, 1)
			continue
		}
		atomic.AddInt64(&totalMessages, 1)

		// 可选：添加小延迟以模拟真实场景
		// time.Sleep(time.Microsecond * 100)
	}

	// 等待接收完成或超时
	select {
	case <-recvDone:
		// 接收完成
	case <-time.After(30 * time.Second):
		// 超时
		log.Printf("Client %d: Receive timeout", clientID)
	}
}

