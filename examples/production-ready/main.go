// Production Ready Service Example - NetCore-Go
// Demonstrates comprehensive production readiness features
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/netcore-go/pkg/core"
)

// UserService 用户服务示例
type UserService struct {
	logger    *core.Logger
	validator *core.Validator
	errorHandler core.ErrorHandler
	users     map[string]*User
}

// User 用户模型
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Age   int    `json:"age,omitempty"`
}

// APIResponse 统一API响应格式
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Code      string      `json:"code,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// NewUserService 创建用户服务
func NewUserService(logger *core.Logger, validator *core.Validator, errorHandler core.ErrorHandler) *UserService {
	return &UserService{
		logger:       logger,
		validator:    validator,
		errorHandler: errorHandler,
		users:        make(map[string]*User),
	}
}

// setupValidationRules 设置验证规则
func (us *UserService) setupValidationRules() {
	// 用户创建验证规则
	us.validator.AddRule("Name", core.Required)
	us.validator.AddRule("Name", core.NewMinLengthRule(2))
	us.validator.AddRule("Name", core.NewMaxLengthRule(50))
	
	us.validator.AddRule("Email", core.Required)
	us.validator.AddRule("Email", core.URL) // 这里应该是Email验证，简化使用URL
	
	us.validator.AddRule("Age", core.NewRangeRule(0, 150))
}

// CreateUser 创建用户
func (us *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	logger := us.logger.WithContext(ctx).WithField("operation", "create_user")
	logger.Info("Creating new user")
	
	// 验证请求
	result := us.validator.Validate(req)
	if result.HasErrors() {
		logger.WithField("validation_errors", result.GetErrorMessages()).Warn("User creation validation failed")
		return nil, core.NewError(core.ErrCodeInvalidRequest, "Invalid user data").WithDetails(fmt.Sprintf("Validation errors: %v", result.GetErrorMessages()))
	}
	
	// 检查邮箱是否已存在
	for _, user := range us.users {
		if user.Email == req.Email {
			logger.WithField("email", req.Email).Warn("Email already exists")
			return nil, core.NewError(core.ErrCodeInvalidRequest, "Email already exists")
		}
	}
	
	// 创建用户
	user := &User{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	us.users[user.ID] = user
	
	logger.WithFields(map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	}).Info("User created successfully")
	
	return user, nil
}

// GetUser 获取用户
func (us *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	logger := us.logger.WithContext(ctx).WithFields(map[string]interface{}{
		"operation": "get_user",
		"user_id":   userID,
	})
	
	logger.Debug("Getting user")
	
	user, exists := us.users[userID]
	if !exists {
		logger.Warn("User not found")
		return nil, core.NewError(core.ErrCodeNotFound, "User not found")
	}
	
	logger.Debug("User retrieved successfully")
	return user, nil
}

// UpdateUser 更新用户
func (us *UserService) UpdateUser(ctx context.Context, userID string, req *UpdateUserRequest) (*User, error) {
	logger := us.logger.WithContext(ctx).WithFields(map[string]interface{}{
		"operation": "update_user",
		"user_id":   userID,
	})
	
	logger.Info("Updating user")
	
	user, exists := us.users[userID]
	if !exists {
		logger.Warn("User not found for update")
		return nil, core.NewError(core.ErrCodeNotFound, "User not found")
	}
	
	// 更新字段
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		// 检查邮箱是否已被其他用户使用
		for id, existingUser := range us.users {
			if id != userID && existingUser.Email == req.Email {
				logger.WithField("email", req.Email).Warn("Email already exists for another user")
				return nil, core.NewError(core.ErrCodeInvalidRequest, "Email already exists")
			}
		}
		user.Email = req.Email
	}
	if req.Age > 0 {
		user.Age = req.Age
	}
	user.UpdatedAt = time.Now()
	
	logger.Info("User updated successfully")
	return user, nil
}

// DeleteUser 删除用户
func (us *UserService) DeleteUser(ctx context.Context, userID string) error {
	logger := us.logger.WithContext(ctx).WithFields(map[string]interface{}{
		"operation": "delete_user",
		"user_id":   userID,
	})
	
	logger.Info("Deleting user")
	
	if _, exists := us.users[userID]; !exists {
		logger.Warn("User not found for deletion")
		return core.NewError(core.ErrCodeNotFound, "User not found")
	}
	
	delete(us.users, userID)
	logger.Info("User deleted successfully")
	return nil
}

// ListUsers 列出所有用户
func (us *UserService) ListUsers(ctx context.Context) ([]*User, error) {
	logger := us.logger.WithContext(ctx).WithField("operation", "list_users")
	logger.Debug("Listing all users")
	
	users := make([]*User, 0, len(us.users))
	for _, user := range us.users {
		users = append(users, user)
	}
	
	logger.WithField("count", len(users)).Debug("Users listed successfully")
	return users, nil
}

// UserHandler HTTP处理器
type UserHandler struct {
	userService *UserService
	logger      *core.Logger
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userService *UserService, logger *core.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// respondWithJSON 统一JSON响应
func (uh *UserHandler) respondWithJSON(w http.ResponseWriter, statusCode int, success bool, data interface{}, err error, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := APIResponse{
		Success:   success,
		Data:      data,
		Timestamp: time.Now(),
		RequestID: requestID,
	}
	
	if err != nil {
		if netErr, ok := err.(*core.NetCoreError); ok {
			response.Error = netErr.Message
			response.Code = string(netErr.Code)
		} else {
			response.Error = err.Error()
			response.Code = string(core.ErrCodeInternal)
		}
	}
	
	json.NewEncoder(w).Encode(response)
}

// getRequestID 获取请求ID
func (uh *UserHandler) getRequestID(r *http.Request) string {
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		return requestID
	}
	return uuid.New().String()
}

// CreateUser 创建用户处理器
func (uh *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	requestID := uh.getRequestID(r)
	ctx := context.WithValue(r.Context(), "request_id", requestID)
	
	if r.Method != http.MethodPost {
		uh.respondWithJSON(w, http.StatusMethodNotAllowed, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "Method not allowed"), requestID)
		return
	}
	
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		uh.respondWithJSON(w, http.StatusBadRequest, false, nil, 
			core.NewErrorWithCause(core.ErrCodeInvalidRequest, "Invalid JSON", err), requestID)
		return
	}
	
	user, err := uh.userService.CreateUser(ctx, &req)
	if err != nil {
		if netErr, ok := err.(*core.NetCoreError); ok {
			uh.respondWithJSON(w, netErr.HTTPStatusCode(), false, nil, err, requestID)
		} else {
			uh.respondWithJSON(w, http.StatusInternalServerError, false, nil, err, requestID)
		}
		return
	}
	
	uh.respondWithJSON(w, http.StatusCreated, true, user, nil, requestID)
}

// GetUser 获取用户处理器
func (uh *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	requestID := uh.getRequestID(r)
	ctx := context.WithValue(r.Context(), "request_id", requestID)
	
	if r.Method != http.MethodGet {
		uh.respondWithJSON(w, http.StatusMethodNotAllowed, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "Method not allowed"), requestID)
		return
	}
	
	// 从URL路径中提取用户ID（简化实现）
	userID := r.URL.Query().Get("id")
	if userID == "" {
		uh.respondWithJSON(w, http.StatusBadRequest, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "User ID is required"), requestID)
		return
	}
	
	user, err := uh.userService.GetUser(ctx, userID)
	if err != nil {
		if netErr, ok := err.(*core.NetCoreError); ok {
			uh.respondWithJSON(w, netErr.HTTPStatusCode(), false, nil, err, requestID)
		} else {
			uh.respondWithJSON(w, http.StatusInternalServerError, false, nil, err, requestID)
		}
		return
	}
	
	uh.respondWithJSON(w, http.StatusOK, true, user, nil, requestID)
}

// UpdateUser 更新用户处理器
func (uh *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	requestID := uh.getRequestID(r)
	ctx := context.WithValue(r.Context(), "request_id", requestID)
	
	if r.Method != http.MethodPut {
		uh.respondWithJSON(w, http.StatusMethodNotAllowed, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "Method not allowed"), requestID)
		return
	}
	
	userID := r.URL.Query().Get("id")
	if userID == "" {
		uh.respondWithJSON(w, http.StatusBadRequest, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "User ID is required"), requestID)
		return
	}
	
	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		uh.respondWithJSON(w, http.StatusBadRequest, false, nil, 
			core.NewErrorWithCause(core.ErrCodeInvalidRequest, "Invalid JSON", err), requestID)
		return
	}
	
	user, err := uh.userService.UpdateUser(ctx, userID, &req)
	if err != nil {
		if netErr, ok := err.(*core.NetCoreError); ok {
			uh.respondWithJSON(w, netErr.HTTPStatusCode(), false, nil, err, requestID)
		} else {
			uh.respondWithJSON(w, http.StatusInternalServerError, false, nil, err, requestID)
		}
		return
	}
	
	uh.respondWithJSON(w, http.StatusOK, true, user, nil, requestID)
}

// DeleteUser 删除用户处理器
func (uh *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	requestID := uh.getRequestID(r)
	ctx := context.WithValue(r.Context(), "request_id", requestID)
	
	if r.Method != http.MethodDelete {
		uh.respondWithJSON(w, http.StatusMethodNotAllowed, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "Method not allowed"), requestID)
		return
	}
	
	userID := r.URL.Query().Get("id")
	if userID == "" {
		uh.respondWithJSON(w, http.StatusBadRequest, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "User ID is required"), requestID)
		return
	}
	
	err := uh.userService.DeleteUser(ctx, userID)
	if err != nil {
		if netErr, ok := err.(*core.NetCoreError); ok {
			uh.respondWithJSON(w, netErr.HTTPStatusCode(), false, nil, err, requestID)
		} else {
			uh.respondWithJSON(w, http.StatusInternalServerError, false, nil, err, requestID)
		}
		return
	}
	
	uh.respondWithJSON(w, http.StatusNoContent, true, nil, nil, requestID)
}

// ListUsers 列出用户处理器
func (uh *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	requestID := uh.getRequestID(r)
	ctx := context.WithValue(r.Context(), "request_id", requestID)
	
	if r.Method != http.MethodGet {
		uh.respondWithJSON(w, http.StatusMethodNotAllowed, false, nil, 
			core.NewError(core.ErrCodeInvalidRequest, "Method not allowed"), requestID)
		return
	}
	
	users, err := uh.userService.ListUsers(ctx)
	if err != nil {
		if netErr, ok := err.(*core.NetCoreError); ok {
			uh.respondWithJSON(w, netErr.HTTPStatusCode(), false, nil, err, requestID)
		} else {
			uh.respondWithJSON(w, http.StatusInternalServerError, false, nil, err, requestID)
		}
		return
	}
	
	uh.respondWithJSON(w, http.StatusOK, true, map[string]interface{}{
		"users": users,
		"total": len(users),
	}, nil, requestID)
}

func main() {
	// 创建生产环境配置
	config := core.DefaultProductionConfig("user-service")
	config.Environment = "development" // 为了演示，使用开发环境
	config.Logging = core.DevelopmentLoggerConfig("user-service")
	
	// 创建生产环境管理器
	pm, err := core.NewProductionManager(config)
	if err != nil {
		fmt.Printf("Failed to create production manager: %v\n", err)
		os.Exit(1)
	}
	
	// 启动生产环境管理器
	if err := pm.Start(); err != nil {
		fmt.Printf("Failed to start production manager: %v\n", err)
		os.Exit(1)
	}
	
	logger := pm.GetLogger()
	logger.Info("Starting user service")
	
	// 创建用户服务
	userService := NewUserService(logger, pm.GetValidator(), pm.GetErrorHandler())
	userService.setupValidationRules()
	
	// 创建HTTP处理器
	userHandler := NewUserHandler(userService, logger)
	
	// 设置路由
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			userHandler.CreateUser(w, r)
		case http.MethodGet:
			if r.URL.Query().Get("id") != "" {
				userHandler.GetUser(w, r)
			} else {
				userHandler.ListUsers(w, r)
			}
		case http.MethodPut:
			userHandler.UpdateUser(w, r)
		case http.MethodDelete:
			userHandler.DeleteUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	// 状态端点
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pm.GetStatus())
	})
	
	// API文档端点
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "NetCore-Go Production Ready User Service",
			"version": config.Version,
			"environment": config.Environment,
			"features": []string{
				"Comprehensive error handling with error codes",
				"Structured logging with context",
				"Request validation with custom rules",
				"Health checks and monitoring",
				"Performance monitoring",
				"Panic recovery",
				"Metrics collection",
				"Production-ready configuration",
			},
			"endpoints": map[string]string{
				"POST /api/users":     "Create user",
				"GET /api/users":      "List all users",
				"GET /api/users?id=X": "Get user by ID",
				"PUT /api/users?id=X": "Update user",
				"DELETE /api/users?id=X": "Delete user",
				"GET /api/status":     "Service status",
				"GET /health":         "Health check (port 8081)",
				"GET /metrics":        "Metrics (port 8082)",
			},
			"example_requests": map[string]interface{}{
				"create_user": map[string]interface{}{
					"method": "POST",
					"url":    "/api/users",
					"body": map[string]interface{}{
						"name":  "John Doe",
						"email": "john@example.com",
						"age":   30,
					},
				},
				"update_user": map[string]interface{}{
					"method": "PUT",
					"url":    "/api/users?id=USER_ID",
					"body": map[string]interface{}{
						"name": "Jane Doe",
						"age":  25,
					},
				},
			},
		})
	})
	
	// 应用中间件
	handler := pm.LogHTTPRequest(pm.HandleHTTPError(mux))
	
	// 创建HTTP服务器
	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// 启动服务器
	go func() {
		logger.Infof("User service starting on port 8080")
		logger.Infof("Health checks available on port %d", config.HealthCheck.Port)
		logger.Infof("Metrics available on port %d", config.Metrics.Port)
		logger.Info("API Documentation: http://localhost:8080")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorWithErr("Server failed to start", err)
			os.Exit(1)
		}
	}()
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigChan
	logger.Info("Received shutdown signal")
	
	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		logger.ErrorWithErr("Server shutdown failed", err)
	}
	
	if err := pm.Stop(); err != nil {
		logger.ErrorWithErr("Production manager shutdown failed", err)
	}
	
	logger.Info("Service shutdown completed")
}