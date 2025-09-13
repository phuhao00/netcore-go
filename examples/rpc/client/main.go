// Package main RPC客户端示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"log"
	"time"

	"github.com/netcore-go/pkg/rpc"
)

// User 用户结构
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// GetUserRequest 获取用户请求
type GetUserRequest struct {
	ID int `json:"id"`
}

// GetUserResponse 获取用户响应
type GetUserResponse struct {
	User *User `json:"user"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// CreateUserResponse 创建用户响应
type CreateUserResponse struct {
	User *User `json:"user"`
}

// ListUsersRequest 列出用户请求
type ListUsersRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// ListUsersResponse 列出用户响应
type ListUsersResponse struct {
	Users []*User `json:"users"`
	Total int     `json:"total"`
}

// AddRequest 加法请求
type AddRequest struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// AddResponse 加法响应
type AddResponse struct {
	Result float64 `json:"result"`
}

func main() {
	log.Println("Starting RPC client example...")
	
	// 创建RPC客户端
	client := rpc.NewRPCClient("localhost:8084",
		rpc.WithCodec(rpc.NewJSONCodec()),
		rpc.WithTimeout(10*time.Second),
		rpc.WithRetryCount(3),
		rpc.WithInterceptor(rpc.NewLoggingInterceptor(nil)),
	)
	
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Error closing client: %v", err)
		}
	}()
	
	// 连接到服务器
	log.Println("Connecting to RPC server...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	log.Println("Connected to RPC server successfully")
	
	// 创建带认证的上下文
	metadata := map[string]string{
		"authorization": "valid-token-123",
	}
	
	ctx := context.Background()
	
	log.Println("\n=== Testing UserService ===")
	
	// 测试获取用户
	log.Println("\n1. Testing GetUser...")
	getUserReq := &GetUserRequest{ID: 1}
	getUserResp := &GetUserResponse{}
	
	if err := client.CallWithMetadata(ctx, "UserService", "GetUser", getUserReq, getUserResp, metadata); err != nil {
		log.Printf("GetUser failed: %v", err)
	} else {
		log.Printf("GetUser success: %+v", getUserResp.User)
	}
	
	// 测试创建用户
	log.Println("\n2. Testing CreateUser...")
	createUserReq := &CreateUserRequest{
		Name:  "David",
		Email: "david@example.com",
		Age:   28,
	}
	createUserResp := &CreateUserResponse{}
	
	if err := client.CallWithMetadata(ctx, "UserService", "CreateUser", createUserReq, createUserResp, metadata); err != nil {
		log.Printf("CreateUser failed: %v", err)
	} else {
		log.Printf("CreateUser success: %+v", createUserResp.User)
	}
	
	// 测试列出用户
	log.Println("\n3. Testing ListUsers...")
	listUsersReq := &ListUsersRequest{
		Page:     1,
		PageSize: 10,
	}
	listUsersResp := &ListUsersResponse{}
	
	if err := client.CallWithMetadata(ctx, "UserService", "ListUsers", listUsersReq, listUsersResp, metadata); err != nil {
		log.Printf("ListUsers failed: %v", err)
	} else {
		log.Printf("ListUsers success: Total=%d, Users:", listUsersResp.Total)
		for _, user := range listUsersResp.Users {
			log.Printf("  - %+v", user)
		}
	}
	
	log.Println("\n=== Testing CalculatorService ===")
	
	// 测试加法
	log.Println("\n4. Testing Add...")
	addReq := &AddRequest{A: 10.5, B: 20.3}
	addResp := &AddResponse{}
	
	if err := client.CallWithMetadata(ctx, "CalculatorService", "Add", addReq, addResp, metadata); err != nil {
		log.Printf("Add failed: %v", err)
	} else {
		log.Printf("Add success: %f + %f = %f", addReq.A, addReq.B, addResp.Result)
	}
	
	// 测试乘法
	log.Println("\n5. Testing Multiply...")
	multiplyReq := &AddRequest{A: 5.5, B: 4.0}
	multiplyResp := &AddResponse{}
	
	if err := client.CallWithMetadata(ctx, "CalculatorService", "Multiply", multiplyReq, multiplyResp, metadata); err != nil {
		log.Printf("Multiply failed: %v", err)
	} else {
		log.Printf("Multiply success: %f * %f = %f", multiplyReq.A, multiplyReq.B, multiplyResp.Result)
	}
	
	log.Println("\n=== Testing Error Cases ===")
	
	// 测试不存在的用户
	log.Println("\n6. Testing GetUser with non-existent ID...")
	getUserReq2 := &GetUserRequest{ID: 999}
	getUserResp2 := &GetUserResponse{}
	
	if err := client.CallWithMetadata(ctx, "UserService", "GetUser", getUserReq2, getUserResp2, metadata); err != nil {
		log.Printf("GetUser failed as expected: %v", err)
	} else {
		log.Printf("GetUser unexpectedly succeeded: %+v", getUserResp2.User)
	}
	
	// 测试无效认证
	log.Println("\n7. Testing with invalid authentication...")
	invalidMetadata := map[string]string{
		"authorization": "invalid-token",
	}
	
	getUserReq3 := &GetUserRequest{ID: 1}
	getUserResp3 := &GetUserResponse{}
	
	if err := client.CallWithMetadata(ctx, "UserService", "GetUser", getUserReq3, getUserResp3, invalidMetadata); err != nil {
		log.Printf("GetUser failed as expected (invalid auth): %v", err)
	} else {
		log.Printf("GetUser unexpectedly succeeded with invalid auth: %+v", getUserResp3.User)
	}
	
	// 测试不存在的服务
	log.Println("\n8. Testing non-existent service...")
	if err := client.CallWithMetadata(ctx, "NonExistentService", "SomeMethod", map[string]interface{}{}, map[string]interface{}{}, metadata); err != nil {
		log.Printf("NonExistentService failed as expected: %v", err)
	} else {
		log.Printf("NonExistentService unexpectedly succeeded")
	}
	
	log.Println("\n=== Testing Async Calls ===")
	
	// 测试异步调用
	log.Println("\n9. Testing async calls...")
	
	// 启动多个异步调用
	asyncCalls := make([]<-chan error, 0)
	
	for i := 0; i < 3; i++ {
		addReqAsync := &AddRequest{A: float64(i * 10), B: float64(i * 5)}
		addRespAsync := &AddResponse{}
		
		resultChan := client.AsyncCallWithMetadata(ctx, "CalculatorService", "Add", addReqAsync, addRespAsync, metadata)
		asyncCalls = append(asyncCalls, resultChan)
		
		log.Printf("Started async call %d: %f + %f", i+1, addReqAsync.A, addReqAsync.B)
	}
	
	// 等待所有异步调用完成
	for i, resultChan := range asyncCalls {
		select {
		case err := <-resultChan:
			if err != nil {
				log.Printf("Async call %d failed: %v", i+1, err)
			} else {
				log.Printf("Async call %d completed successfully", i+1)
			}
		case <-time.After(5 * time.Second):
			log.Printf("Async call %d timed out", i+1)
		}
	}
	
	log.Println("\n=== Performance Test ===")
	
	// 性能测试
	log.Println("\n10. Running performance test...")
	startTime := time.Now()
	successCount := 0
	errorCount := 0
	testCount := 100
	
	for i := 0; i < testCount; i++ {
		perfReq := &AddRequest{A: float64(i), B: float64(i * 2)}
		perfResp := &AddResponse{}
		
		if err := client.CallWithMetadata(ctx, "CalculatorService", "Add", perfReq, perfResp, metadata); err != nil {
			errorCount++
		} else {
			successCount++
		}
	}
	
	duration := time.Since(startTime)
	log.Printf("Performance test completed:")
	log.Printf("  Total calls: %d", testCount)
	log.Printf("  Successful: %d", successCount)
	log.Printf("  Failed: %d", errorCount)
	log.Printf("  Duration: %v", duration)
	log.Printf("  Average latency: %v", duration/time.Duration(testCount))
	log.Printf("  QPS: %.2f", float64(testCount)/duration.Seconds())
	
	log.Println("\n=== RPC Client Test Completed ===")
	log.Println("All tests finished successfully!")
}

