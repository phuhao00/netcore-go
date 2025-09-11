// Package main HTTP服务器示例
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
	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/http"
)

// User 用户结构
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// HTTPHandler HTTP处理器
type HTTPHandler struct {
	users []User
}

// NewHTTPHandler 创建HTTP处理器
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		users: []User{
			{ID: 1, Name: "Alice", Email: "alice@example.com"},
			{ID: 2, Name: "Bob", Email: "bob@example.com"},
			{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
		},
	}
}

// OnConnect 连接建立时调用
func (h *HTTPHandler) OnConnect(conn core.Connection) {
	fmt.Printf("HTTP connection established: %s\n", conn.RemoteAddr())
}

// OnMessage 收到消息时调用（HTTP不使用此方法）
func (h *HTTPHandler) OnMessage(conn core.Connection, msg core.Message) {
	// HTTP协议不使用此方法
}

// OnDisconnect 连接断开时调用
func (h *HTTPHandler) OnDisconnect(conn core.Connection, err error) {
	fmt.Printf("HTTP connection closed: %s\n", conn.RemoteAddr())
}

// setupRoutes 设置路由
func (h *HTTPHandler) setupRoutes(server *http.HTTPServer) {
	// 首页
	server.HandleFunc("GET", "/", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		html := `
<!DOCTYPE html>
<html>
<head>
    <title>NetCore-Go HTTP Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .method { color: #fff; padding: 3px 8px; border-radius: 3px; font-size: 12px; }
        .get { background: #61affe; }
        .post { background: #49cc90; }
        .put { background: #fca130; }
        .delete { background: #f93e3e; }
    </style>
</head>
<body>
    <div class="container">
        <h1>NetCore-Go HTTP Server</h1>
        <p>Welcome to NetCore-Go HTTP Server! This is a high-performance HTTP server built with Go.</p>
        
        <h2>Available Endpoints:</h2>
        
        <div class="endpoint">
            <span class="method get">GET</span>
            <strong>/api/users</strong> - Get all users
        </div>
        
        <div class="endpoint">
            <span class="method get">GET</span>
            <strong>/api/users/:id</strong> - Get user by ID
        </div>
        
        <div class="endpoint">
            <span class="method post">POST</span>
            <strong>/api/users</strong> - Create new user
        </div>
        
        <div class="endpoint">
            <span class="method put">PUT</span>
            <strong>/api/users/:id</strong> - Update user
        </div>
        
        <div class="endpoint">
            <span class="method delete">DELETE</span>
            <strong>/api/users/:id</strong> - Delete user
        </div>
        
        <div class="endpoint">
            <span class="method get">GET</span>
            <strong>/api/health</strong> - Health check
        </div>
        
        <h2>Test the API:</h2>
        <p>You can test the API using curl or any HTTP client:</p>
        <pre>
# Get all users
curl http://localhost:8083/api/users

# Get user by ID
curl http://localhost:8083/api/users/1

# Create new user
curl -X POST http://localhost:8083/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"David","email":"david@example.com"}'

# Health check
curl http://localhost:8083/api/health
        </pre>
    </div>
</body>
</html>`
		ctx.HTML(resp, 200, html)
	})

	// API路由
	// 获取所有用户
	server.HandleFunc("GET", "/api/users", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		ctx.JSON(resp, 200, map[string]interface{}{
			"success": true,
			"data":    h.users,
			"total":   len(h.users),
		})
	})

	// 根据ID获取用户
	server.HandleFunc("GET", "/api/users/:id", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		id := ctx.Param("id")
		for _, user := range h.users {
			if fmt.Sprintf("%d", user.ID) == id {
				ctx.JSON(resp, 200, map[string]interface{}{
					"success": true,
					"data":    user,
				})
				return
			}
		}
		ctx.JSON(resp, 404, map[string]interface{}{
			"success": false,
			"error":   "User not found",
		})
	})

	// 创建新用户
	server.HandleFunc("POST", "/api/users", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		var newUser User
		if err := ctx.BindJSON(&newUser); err != nil {
			ctx.JSON(resp, 400, map[string]interface{}{
				"success": false,
				"error":   "Invalid JSON data",
			})
			return
		}

		// 分配新ID
		maxID := 0
		for _, user := range h.users {
			if user.ID > maxID {
				maxID = user.ID
			}
		}
		newUser.ID = maxID + 1

		// 添加到用户列表
		h.users = append(h.users, newUser)

		ctx.JSON(resp, 201, map[string]interface{}{
			"success": true,
			"data":    newUser,
			"message": "User created successfully",
		})
	})

	// 更新用户
	server.HandleFunc("PUT", "/api/users/:id", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		id := ctx.Param("id")
		for i, user := range h.users {
			if fmt.Sprintf("%d", user.ID) == id {
				var updatedUser User
				if err := ctx.BindJSON(&updatedUser); err != nil {
					ctx.JSON(resp, 400, map[string]interface{}{
						"success": false,
						"error":   "Invalid JSON data",
					})
					return
				}

				updatedUser.ID = user.ID
				h.users[i] = updatedUser

				ctx.JSON(resp, 200, map[string]interface{}{
					"success": true,
					"data":    updatedUser,
					"message": "User updated successfully",
				})
				return
			}
		}
		ctx.JSON(resp, 404, map[string]interface{}{
			"success": false,
			"error":   "User not found",
		})
	})

	// 删除用户
	server.HandleFunc("DELETE", "/api/users/:id", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		id := ctx.Param("id")
		for i, user := range h.users {
			if fmt.Sprintf("%d", user.ID) == id {
				// 删除用户
				h.users = append(h.users[:i], h.users[i+1:]...)
				ctx.JSON(resp, 200, map[string]interface{}{
					"success": true,
					"message": "User deleted successfully",
				})
				return
			}
		}
		ctx.JSON(resp, 404, map[string]interface{}{
			"success": false,
			"error":   "User not found",
		})
	})

	// 健康检查
	server.HandleFunc("GET", "/api/health", func(ctx *http.HTTPContext, resp *http.HTTPResponse) {
		stats := server.GetStats()
		ctx.JSON(resp, 200, map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"uptime":    stats.Uptime,
			"connections": map[string]interface{}{
				"active": stats.ActiveConnections,
				"total":  stats.TotalConnections,
			},
			"messages": map[string]interface{}{
				"received": stats.MessagesReceived,
				"sent":     stats.MessagesSent,
			},
			"bytes": map[string]interface{}{
				"received": stats.BytesReceived,
				"sent":     stats.BytesSent,
			},
		})
	})

	// 静态文件服务（如果需要）
	// server.ServeStatic("/static/", "./static")
}

func main() {
	fmt.Println("Starting NetCore-Go HTTP Server...")

	// 创建HTTP服务器
	server := netcore.NewHTTPServer(
		netcore.WithReadBufferSize(8192),
		netcore.WithWriteBufferSize(8192),
		netcore.WithMaxConnections(1000),
		netcore.WithReadTimeout(30*time.Second),
		netcore.WithWriteTimeout(30*time.Second),
		netcore.WithIdleTimeout(5*time.Minute),
		netcore.WithHeartbeat(true, 30*time.Second),
		netcore.WithConnectionPool(true),
		netcore.WithMemoryPool(true),
		netcore.WithGoroutinePool(true),
	)

	// 创建处理器
	handler := NewHTTPHandler()
	server.SetHandler(handler)

	// 设置路由
	if httpServer, ok := server.(*http.HTTPServer); ok {
		handler.setupRoutes(httpServer)
	}

	// 启动服务器
	go func() {
		if err := server.Start(":8083"); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	fmt.Println("HTTP Server started on :8083")
	fmt.Println("Visit http://localhost:8083 to see the web interface")
	fmt.Println("API endpoints available at http://localhost:8083/api/")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down HTTP server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
	fmt.Println("HTTP Server stopped")
}

