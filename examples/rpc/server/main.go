// Package main RPC服务器示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phuhao00/netcore-go/pkg/core"
	"github.com/phuhao00/netcore-go/pkg/rpc"
)

// UserService 用户服务
type UserService struct{}

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

// 模拟用户数据
var users = map[int]*User{
	1: {ID: 1, Name: "Alice", Email: "alice@example.com", Age: 25},
	2: {ID: 2, Name: "Bob", Email: "bob@example.com", Age: 30},
	3: {ID: 3, Name: "Charlie", Email: "charlie@example.com", Age: 35},
}
var nextUserID = 4

// GetUser 获取用户
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest, resp *GetUserResponse) error {
	log.Printf("GetUser called with ID: %d", req.ID)

	user, exists := users[req.ID]
	if !exists {
		return fmt.Errorf("user not found: %d", req.ID)
	}

	resp.User = user
	return nil
}

// CreateUser 创建用户
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest, resp *CreateUserResponse) error {
	log.Printf("CreateUser called: %+v", req)

	user := &User{
		ID:    nextUserID,
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	}

	users[nextUserID] = user
	nextUserID++

	resp.User = user
	return nil
}

// ListUsers 列出用户
func (s *UserService) ListUsers(ctx context.Context, req *ListUsersRequest, resp *ListUsersResponse) error {
	log.Printf("ListUsers called: page=%d, pageSize=%d", req.Page, req.PageSize)

	// 简单的分页逻辑
	allUsers := make([]*User, 0, len(users))
	for _, user := range users {
		allUsers = append(allUsers, user)
	}

	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= len(allUsers) {
		resp.Users = []*User{}
	} else {
		if end > len(allUsers) {
			end = len(allUsers)
		}
		resp.Users = allUsers[start:end]
	}

	resp.Total = len(allUsers)
	return nil
}

// CalculatorService 计算器服务
type CalculatorService struct{}

// AddRequest 加法请求
type AddRequest struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// AddResponse 加法响应
type AddResponse struct {
	Result float64 `json:"result"`
}

// Add 加法运算
func (s *CalculatorService) Add(ctx context.Context, req *AddRequest, resp *AddResponse) error {
	log.Printf("Add called: %f + %f", req.A, req.B)
	resp.Result = req.A + req.B
	return nil
}

// Multiply 乘法运算
func (s *CalculatorService) Multiply(ctx context.Context, req *AddRequest, resp *AddResponse) error {
	log.Printf("Multiply called: %f * %f", req.A, req.B)
	resp.Result = req.A * req.B
	return nil
}

// RPCHandler RPC消息处理器
type RPCHandler struct{}

// OnConnect 连接建立
func (h *RPCHandler) OnConnect(conn core.Connection) {
	log.Printf("RPC client connected: %s", conn.RemoteAddr())
}

// OnMessage 消息处理
func (h *RPCHandler) OnMessage(conn core.Connection, msg core.Message) {
	log.Printf("RPC message received from %s", conn.RemoteAddr())
}

// OnDisconnect 连接断开
func (h *RPCHandler) OnDisconnect(conn core.Connection, err error) {
	if err != nil {
		log.Printf("RPC client disconnected with error: %v", err)
	} else {
		log.Printf("RPC client disconnected: %s", conn.RemoteAddr())
	}
}

func main() {
	log.Println("Starting RPC server example...")

	// 创建RPC服务器
	server := rpc.NewRPCServer(
		core.WithMaxConnections(1000),
		core.WithReadBufferSize(4096),
		core.WithWriteBufferSize(4096),
		core.WithHeartbeat(true, 30*time.Second),
	)

	// 设置消息处理器
	server.SetHandler(&RPCHandler{})

	// 创建服务注册中心
	registry := rpc.NewMemoryRegistry()
	server.SetRegistry(registry)

	// 设置编解码器
	server.SetCodec(rpc.NewJSONCodec())

	// 创建拦截器链
	loggingInterceptor := rpc.NewLoggingInterceptor(nil)
	metricsInterceptor := rpc.NewMetricsInterceptor(nil)
	authInterceptor := rpc.NewAuthInterceptor(
		rpc.NewTokenAuthenticator([]string{"valid-token-123", "admin-token-456"}),
	)

	interceptorChain := rpc.NewChainInterceptor(
		loggingInterceptor,
		metricsInterceptor,
		authInterceptor,
	)
	server.SetInterceptor(interceptorChain)

	// 注册服务
	userService := &UserService{}
	if err := server.RegisterService("UserService", userService); err != nil {
		log.Fatalf("Failed to register UserService: %v", err)
	}

	calculatorService := &CalculatorService{}
	if err := server.RegisterService("CalculatorService", calculatorService); err != nil {
		log.Fatalf("Failed to register CalculatorService: %v", err)
	}

	// 启动服务器
	go func() {
		log.Println("RPC server listening on :8084")
		if err := server.Start(":8084"); err != nil {
			log.Fatalf("Failed to start RPC server: %v", err)
		}
	}()

	// 等待一段时间让服务器启动
	time.Sleep(1 * time.Second)

	// 打印服务器信息
	log.Println("\n=== RPC Server Information ===")
	log.Println("Server Address: localhost:8084")
	log.Println("Registered Services:")
	log.Println("  - UserService")
	log.Println("    - GetUser(id) -> user")
	log.Println("    - CreateUser(name, email, age) -> user")
	log.Println("    - ListUsers(page, pageSize) -> users")
	log.Println("  - CalculatorService")
	log.Println("    - Add(a, b) -> result")
	log.Println("    - Multiply(a, b) -> result")
	log.Println("Authentication: Token required (valid-token-123 or admin-token-456)")
	log.Println("Codec: JSON")
	log.Println("Features: Logging, Metrics, Authentication, Connection Pool")
	log.Println("================================")

	// 定期打印服务器统计信息
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := server.GetStats()
				log.Printf("\n=== Server Stats ===")
				log.Printf("Active Connections: %d", stats.ActiveConnections)
				log.Printf("Total Connections: %d", stats.TotalConnections)
				log.Printf("Messages Received: %d", stats.MessagesReceived)
				log.Printf("Messages Sent: %d", stats.MessagesSent)
				log.Printf("Uptime: %v", time.Since(stats.StartTime))
				if stats.ErrorCount > 0 {
					log.Printf("Error Count: %d", stats.ErrorCount)
					log.Printf("Last Error: %s", stats.LastError)
				}
				log.Printf("===================\n")
			}
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("RPC server is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("\nShutting down RPC server...")

	// 关闭服务器
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	} else {
		log.Println("RPC server stopped successfully")
	}

	// 关闭注册中心
	if err := registry.Close(); err != nil {
		log.Printf("Error closing registry: %v", err)
	}

	log.Println("Goodbye!")
}
