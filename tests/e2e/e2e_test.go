// Package e2e 提供NetCore-Go的端到端测试
// Author: NetCore-Go Team
// Created: 2024

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/netcore-go/pkg/testing"
)

// E2ETestSuite 端到端测试套件
type E2ETestSuite struct {
	name        string
	testDir     string
	serverCmd   *exec.Cmd
	serverURL   string
	cleanupFuncs []func()
}

// NewE2ETestSuite 创建新的端到端测试套件
func NewE2ETestSuite(name string) *E2ETestSuite {
	return &E2ETestSuite{
		name:         name,
		serverURL:    "http://localhost:8080",
		cleanupFuncs: make([]func(), 0),
	}
}

// SetUp 设置测试环境
func (s *E2ETestSuite) SetUp() error {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("netcore-e2e-%s-*", s.name))
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	s.testDir = tempDir
	s.addCleanup(func() { os.RemoveAll(tempDir) })
	
	// 创建测试项目结构
	if err := s.createTestProject(); err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	
	// 启动测试服务器
	if err := s.startTestServer(); err != nil {
		return fmt.Errorf("failed to start test server: %w", err)
	}
	
	// 等待服务器启动
	if err := s.waitForServer(); err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}
	
	return nil
}

// TearDown 清理测试环境
func (s *E2ETestSuite) TearDown() {
	// 停止服务器
	if s.serverCmd != nil && s.serverCmd.Process != nil {
		s.serverCmd.Process.Kill()
		s.serverCmd.Wait()
	}
	
	// 执行清理函数
	for i := len(s.cleanupFuncs) - 1; i >= 0; i-- {
		s.cleanupFuncs[i]()
	}
}

// addCleanup 添加清理函数
func (s *E2ETestSuite) addCleanup(cleanup func()) {
	s.cleanupFuncs = append(s.cleanupFuncs, cleanup)
}

// createTestProject 创建测试项目
func (s *E2ETestSuite) createTestProject() error {
	// 创建目录结构
	dirs := []string{
		"cmd/server",
		"pkg/handlers",
		"pkg/models",
		"pkg/services",
		"configs",
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(s.testDir, dir), 0755); err != nil {
			return err
		}
	}
	
	// 创建go.mod
	goMod := `module netcore-e2e-test

go 1.21

require (
	github.com/netcore-go v0.1.0
)
`
	if err := os.WriteFile(filepath.Join(s.testDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}
	
	// 创建主服务器文件
	mainGo := `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netcore-go/pkg/core"
)

type User struct {
	ID    int    ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

type UserService struct {
	users map[int]*User
	nextID int
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[int]*User),
		nextID: 1,
	}
}

func (s *UserService) CreateUser(name, email string) *User {
	user := &User{
		ID:    s.nextID,
		Name:  name,
		Email: email,
	}
	s.users[user.ID] = user
	s.nextID++
	return user
}

func (s *UserService) GetUser(id int) (*User, bool) {
	user, exists := s.users[id]
	return user, exists
}

func (s *UserService) GetAllUsers() []*User {
	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

func (s *UserService) UpdateUser(id int, name, email string) (*User, bool) {
	user, exists := s.users[id]
	if !exists {
		return nil, false
	}
	user.Name = name
	user.Email = email
	return user, true
}

func (s *UserService) DeleteUser(id int) bool {
	_, exists := s.users[id]
	if exists {
		delete(s.users, id)
	}
	return exists
}

type APIResponse struct {
	Success bool        ` + "`json:\"success\"`" + `
	Data    interface{} ` + "`json:\"data,omitempty\"`" + `
	Error   string      ` + "`json:\"error,omitempty\"`" + `
}

type UserHandler struct {
	userService *UserService
	logger      *core.Logger
}

func NewUserHandler(userService *UserService, logger *core.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	case http.MethodPut:
		h.handlePut(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *UserHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/users")
	
	if path == "" || path == "/" {
		// Get all users
		users := h.userService.GetAllUsers()
		h.sendSuccess(w, users)
		return
	}
	
	// Get specific user
	var userID int
	if _, err := fmt.Sscanf(path, "/%d", &userID); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	
	user, exists := h.userService.GetUser(userID)
	if !exists {
		h.sendError(w, http.StatusNotFound, "User not found")
		return
	}
	
	h.sendSuccess(w, user)
}

func (h *UserHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string ` + "`json:\"name\"`" + `
		Email string ` + "`json:\"email\"`" + `
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	
	if req.Name == "" || req.Email == "" {
		h.sendError(w, http.StatusBadRequest, "Name and email are required")
		return
	}
	
	user := h.userService.CreateUser(req.Name, req.Email)
	w.WriteHeader(http.StatusCreated)
	h.sendSuccess(w, user)
}

func (h *UserHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/users")
	
	var userID int
	if _, err := fmt.Sscanf(path, "/%d", &userID); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	
	var req struct {
		Name  string ` + "`json:\"name\"`" + `
		Email string ` + "`json:\"email\"`" + `
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	
	if req.Name == "" || req.Email == "" {
		h.sendError(w, http.StatusBadRequest, "Name and email are required")
		return
	}
	
	user, exists := h.userService.UpdateUser(userID, req.Name, req.Email)
	if !exists {
		h.sendError(w, http.StatusNotFound, "User not found")
		return
	}
	
	h.sendSuccess(w, user)
}

func (h *UserHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/users")
	
	var userID int
	if _, err := fmt.Sscanf(path, "/%d", &userID); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	
	if !h.userService.DeleteUser(userID) {
		h.sendError(w, http.StatusNotFound, "User not found")
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) sendSuccess(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Success: true,
		Data:    data,
	}
	json.NewEncoder(w).Encode(response)
}

func (h *UserHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	// 初始化日志
	logger := core.NewLogger(core.DefaultLoggerConfig())
	
	// 创建服务
	userService := NewUserService()
	userHandler := NewUserHandler(userService, logger)
	
	// 创建路由
	mux := http.NewServeMux()
	mux.Handle("/users", userHandler)
	mux.Handle("/users/", userHandler)
	
	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   "netcore-e2e-test",
		})
	})
	
	// 创建服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	
	// 启动服务器
	go func() {
		logger.Info("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start: " + err.Error())
			os.Exit(1)
		}
	}()
	
	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	// 优雅关闭
	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error: " + err.Error())
	}
	
	logger.Info("Server stopped")
}
`
	if err := os.WriteFile(filepath.Join(s.testDir, "cmd/server/main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}
	
	return nil
}

// startTestServer 启动测试服务器
func (s *E2ETestSuite) startTestServer() error {
	// 构建项目
	buildCmd := exec.Command("go", "build", "-o", "server", "./cmd/server")
	buildCmd.Dir = s.testDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build server: %w", err)
	}
	
	// 启动服务器
	s.serverCmd = exec.Command("./server")
	s.serverCmd.Dir = s.testDir
	s.serverCmd.Stdout = os.Stdout
	s.serverCmd.Stderr = os.Stderr
	
	if err := s.serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	
	return nil
}

// waitForServer 等待服务器启动
func (s *E2ETestSuite) waitForServer() error {
	for i := 0; i < 30; i++ { // 等待最多30秒
		resp, err := http.Get(s.serverURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("server failed to start within timeout")
}

// makeRequest 发送HTTP请求
func (s *E2ETestSuite) makeRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(bodyBytes)
	}
	
	req, err := http.NewRequest(method, s.serverURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// parseResponse 解析响应
func (s *E2ETestSuite) parseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// TestE2EUserAPI 端到端用户API测试
func TestE2EUserAPI(t *testing.T) {
	suite := NewE2ETestSuite("UserAPI")
	
	if err := suite.SetUp(); err != nil {
		t.Fatalf("Failed to set up E2E test suite: %v", err)
	}
	defer suite.TearDown()
	
	t.Run("HealthCheck", func(t *testing.T) {
		testE2EHealthCheck(t, suite)
	})
	
	t.Run("UserCRUD", func(t *testing.T) {
		testE2EUserCRUD(t, suite)
	})
	
	t.Run("ErrorHandling", func(t *testing.T) {
		testE2EErrorHandling(t, suite)
	})
	
	t.Run("ConcurrentOperations", func(t *testing.T) {
		testE2EConcurrentOperations(t, suite)
	})
}

// testE2EHealthCheck 测试健康检查
func testE2EHealthCheck(t *testing.T, suite *E2ETestSuite) {
	resp, err := suite.makeRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := suite.parseResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse health check response: %v", err)
	}
	
	if status, ok := result["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}
	
	if service, ok := result["service"].(string); !ok || service != "netcore-e2e-test" {
		t.Errorf("Expected service 'netcore-e2e-test', got %v", result["service"])
	}
}

// testE2EUserCRUD 测试用户CRUD操作
func testE2EUserCRUD(t *testing.T, suite *E2ETestSuite) {
	// 创建用户
	createReq := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
	}
	
	resp, err := suite.makeRequest("POST", "/users", createReq)
	if err != nil {
		t.Fatalf("Create user request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
	
	var createResult struct {
		Success bool `json:"success"`
		Data    struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"data"`
	}
	
	if err := suite.parseResponse(resp, &createResult); err != nil {
		t.Fatalf("Failed to parse create user response: %v", err)
	}
	
	if !createResult.Success {
		t.Error("Expected success to be true")
	}
	
	if createResult.Data.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %s", createResult.Data.Name)
	}
	
	if createResult.Data.Email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %s", createResult.Data.Email)
	}
	
	userID := createResult.Data.ID
	
	// 获取用户
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/users/%d", userID), nil)
	if err != nil {
		t.Fatalf("Get user request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var getResult struct {
		Success bool `json:"success"`
		Data    struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"data"`
	}
	
	if err := suite.parseResponse(resp, &getResult); err != nil {
		t.Fatalf("Failed to parse get user response: %v", err)
	}
	
	if getResult.Data.ID != userID {
		t.Errorf("Expected user ID %d, got %d", userID, getResult.Data.ID)
	}
	
	// 更新用户
	updateReq := map[string]string{
		"name":  "Jane Doe",
		"email": "jane@example.com",
	}
	
	resp, err = suite.makeRequest("PUT", fmt.Sprintf("/users/%d", userID), updateReq)
	if err != nil {
		t.Fatalf("Update user request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var updateResult struct {
		Success bool `json:"success"`
		Data    struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"data"`
	}
	
	if err := suite.parseResponse(resp, &updateResult); err != nil {
		t.Fatalf("Failed to parse update user response: %v", err)
	}
	
	if updateResult.Data.Name != "Jane Doe" {
		t.Errorf("Expected updated name 'Jane Doe', got %s", updateResult.Data.Name)
	}
	
	if updateResult.Data.Email != "jane@example.com" {
		t.Errorf("Expected updated email 'jane@example.com', got %s", updateResult.Data.Email)
	}
	
	// 获取所有用户
	resp, err = suite.makeRequest("GET", "/users", nil)
	if err != nil {
		t.Fatalf("Get all users request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var getAllResult struct {
		Success bool `json:"success"`
		Data    []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"data"`
	}
	
	if err := suite.parseResponse(resp, &getAllResult); err != nil {
		t.Fatalf("Failed to parse get all users response: %v", err)
	}
	
	if len(getAllResult.Data) != 1 {
		t.Errorf("Expected 1 user, got %d", len(getAllResult.Data))
	}
	
	if len(getAllResult.Data) > 0 && getAllResult.Data[0].Name != "Jane Doe" {
		t.Errorf("Expected first user name 'Jane Doe', got %s", getAllResult.Data[0].Name)
	}
	
	// 删除用户
	resp, err = suite.makeRequest("DELETE", fmt.Sprintf("/users/%d", userID), nil)
	if err != nil {
		t.Fatalf("Delete user request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
	
	// 验证用户已删除
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/users/%d", userID), nil)
	if err != nil {
		t.Fatalf("Get deleted user request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for deleted user, got %d", resp.StatusCode)
	}
}

// testE2EErrorHandling 测试错误处理
func testE2EErrorHandling(t *testing.T, suite *E2ETestSuite) {
	// 测试获取不存在的用户
	resp, err := suite.makeRequest("GET", "/users/999", nil)
	if err != nil {
		t.Fatalf("Get non-existent user request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
	
	var errorResult struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	
	if err := suite.parseResponse(resp, &errorResult); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}
	
	if errorResult.Success {
		t.Error("Expected success to be false for error response")
	}
	
	if errorResult.Error == "" {
		t.Error("Expected error message to be present")
	}
	
	// 测试无效的JSON
	resp, err = suite.makeRequest("POST", "/users", "invalid json")
	if err != nil {
		t.Fatalf("Invalid JSON request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}
	
	// 测试缺少必填字段
	resp, err = suite.makeRequest("POST", "/users", map[string]string{"name": "John"})
	if err != nil {
		t.Fatalf("Missing field request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing field, got %d", resp.StatusCode)
	}
	
	// 测试不支持的HTTP方法
	resp, err = suite.makeRequest("PATCH", "/users", nil)
	if err != nil {
		t.Fatalf("Unsupported method request failed: %v", err)
	}
	
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for unsupported method, got %d", resp.StatusCode)
	}
}

// testE2EConcurrentOperations 测试并发操作
func testE2EConcurrentOperations(t *testing.T, suite *E2ETestSuite) {
	concurrency := 5
	usersPerWorker := 10
	
	resultChan := make(chan error, concurrency*usersPerWorker)
	
	// 启动并发创建用户
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < usersPerWorker; j++ {
				createReq := map[string]string{
					"name":  fmt.Sprintf("User-%d-%d", workerID, j),
					"email": fmt.Sprintf("user%d%d@example.com", workerID, j),
				}
				
				resp, err := suite.makeRequest("POST", "/users", createReq)
				if err != nil {
					resultChan <- err
					continue
				}
				
				if resp.StatusCode != http.StatusCreated {
					resp.Body.Close()
					resultChan <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					continue
				}
				
				var result struct {
					Success bool `json:"success"`
					Data    struct {
						ID    int    `json:"id"`
						Name  string `json:"name"`
						Email string `json:"email"`
					} `json:"data"`
				}
				
				if err := suite.parseResponse(resp, &result); err != nil {
					resultChan <- err
					continue
				}
				
				if !result.Success {
					resultChan <- fmt.Errorf("create user failed for worker %d, user %d", workerID, j)
					continue
				}
				
				expectedName := fmt.Sprintf("User-%d-%d", workerID, j)
				if result.Data.Name != expectedName {
					resultChan <- fmt.Errorf("name mismatch: expected %s, got %s", expectedName, result.Data.Name)
					continue
				}
				
				resultChan <- nil
			}
		}(i)
	}
	
	// 收集结果
	totalOperations := concurrency * usersPerWorker
	successCount := 0
	errorCount := 0
	
	for i := 0; i < totalOperations; i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				errorCount++
				t.Logf("Concurrent operation error: %v", err)
			} else {
				successCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations to complete")
		}
	}
	
	// 验证结果
	if errorCount > totalOperations/10 { // 允许10%的错误率
		t.Errorf("Too many errors in concurrent operations: %d/%d", errorCount, totalOperations)
	}
	
	if successCount < totalOperations*9/10 { // 至少90%成功
		t.Errorf("Too few successful concurrent operations: %d/%d", successCount, totalOperations)
	}
	
	// 验证所有用户都被创建
	resp, err := suite.makeRequest("GET", "/users", nil)
	if err != nil {
		t.Fatalf("Get all users after concurrent creation failed: %v", err)
	}
	
	var getAllResult struct {
		Success bool `json:"success"`
		Data    []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"data"`
	}
	
	if err := suite.parseResponse(resp, &getAllResult); err != nil {
		t.Fatalf("Failed to parse get all users response: %v", err)
	}
	
	if len(getAllResult.Data) != successCount {
		t.Errorf("Expected %d users, got %d", successCount, len(getAllResult.Data))
	}
	
	t.Logf("Concurrent test completed: %d success, %d errors out of %d total operations", 
		successCount, errorCount, totalOperations)
	t.Logf("Total users created: %d", len(getAllResult.Data))
}

// TestE2EPerformance 性能测试
func TestE2EPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	suite := NewE2ETestSuite("Performance")
	
	if err := suite.SetUp(); err != nil {
		t.Fatalf("Failed to set up E2E performance test suite: %v", err)
	}
	defer suite.TearDown()
	
	// 创建一些测试数据
	for i := 0; i < 100; i++ {
		createReq := map[string]string{
			"name":  fmt.Sprintf("PerfUser-%d", i),
			"email": fmt.Sprintf("perfuser%d@example.com", i),
		}
		
		resp, err := suite.makeRequest("POST", "/users", createReq)
		if err != nil {
			t.Fatalf("Failed to create test user %d: %v", i, err)
		}
		resp.Body.Close()
	}
	
	// 性能测试：获取所有用户
	start := time.Now()
	for i := 0; i < 100; i++ {
		resp, err := suite.makeRequest("GET", "/users", nil)
		if err != nil {
			t.Fatalf("Performance test request %d failed: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Performance test request %d returned status %d", i, resp.StatusCode)
		}
		resp.Body.Close()
	}
	duration := time.Since(start)
	
	avgLatency := duration / 100
	requestsPerSec := float64(100) / duration.Seconds()
	
	t.Logf("Performance test results:")
	t.Logf("  Total time: %v", duration)
	t.Logf("  Average latency: %v", avgLatency)
	t.Logf("  Requests per second: %.2f", requestsPerSec)
	
	// 验证性能指标
	if avgLatency > 100*time.Millisecond {
		t.Errorf("Average latency too high: %v (expected < 100ms)", avgLatency)
	}
	
	if requestsPerSec < 10.0 {
		t.Errorf("Request rate too low: %.2f RPS (expected >= 10 RPS)", requestsPerSec)
	}
}

// TestE2EStressTest 压力测试
func TestE2EStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	suite := NewE2ETestSuite("StressTest")
	
	if err := suite.SetUp(); err != nil {
		t.Fatalf("Failed to set up E2E stress test suite: %v", err)
	}
	defer suite.TearDown()
	
	// 配置压力测试
	concurrency := 20
	requestsPerWorker := 50
	testDuration := 30 * time.Second
	
	resultChan := make(chan struct {
		success bool
		latency time.Duration
		error   error
	}, concurrency*requestsPerWorker)
	
	start := time.Now()
	
	// 启动压力测试
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < requestsPerWorker; j++ {
				if time.Since(start) > testDuration {
					break
				}
				
				reqStart := time.Now()
				resp, err := suite.makeRequest("GET", "/health", nil)
				latency := time.Since(reqStart)
				
				if err != nil {
					resultChan <- struct {
						success bool
						latency time.Duration
						error   error
					}{false, latency, err}
					continue
				}
				
				success := resp.StatusCode == http.StatusOK
				resp.Body.Close()
				
				resultChan <- struct {
					success bool
					latency time.Duration
					error   error
				}{success, latency, nil}
				
				// 小延迟避免过度压力
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}
	
	// 收集结果
	var (
		totalRequests   int
		successRequests int
		totalLatency    time.Duration
		maxLatency      time.Duration
		minLatency      = time.Hour // 初始化为很大的值
		errorCount      int
	)
	
	timeout := time.After(testDuration + 10*time.Second)
	for {
		select {
		case result := <-resultChan:
			totalRequests++
			totalLatency += result.latency
			
			if result.latency > maxLatency {
				maxLatency = result.latency
			}
			if result.latency < minLatency {
				minLatency = result.latency
			}
			
			if result.success {
				successRequests++
			} else {
				errorCount++
				if result.error != nil {
					t.Logf("Stress test error: %v", result.error)
				}
			}
			
			// 检查是否所有请求都完成了
			if totalRequests >= concurrency*requestsPerWorker {
				goto done
			}
			
		case <-timeout:
			t.Log("Stress test timeout reached")
			goto done
		}
	}
	
done:
	actualDuration := time.Since(start)
	avgLatency := totalLatency / time.Duration(totalRequests)
	successRate := float64(successRequests) / float64(totalRequests) * 100
	requestsPerSec := float64(totalRequests) / actualDuration.Seconds()
	
	t.Logf("Stress test results:")
	t.Logf("  Duration: %v", actualDuration)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful requests: %d", successRequests)
	t.Logf("  Error count: %d", errorCount)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Requests per second: %.2f", requestsPerSec)
	t.Logf("  Average latency: %v", avgLatency)
	t.Logf("  Min latency: %v", minLatency)
	t.Logf("  Max latency: %v", maxLatency)
	
	// 验证压力测试结果
	if successRate < 95.0 {
		t.Errorf("Success rate too low: %.2f%% (expected >= 95%%)", successRate)
	}
	
	if avgLatency > 200*time.Millisecond {
		t.Errorf("Average latency too high: %v (expected < 200ms)", avgLatency)
	}
	
	if requestsPerSec < 50.0 {
		t.Errorf("Request rate too low: %.2f RPS (expected >= 50 RPS)", requestsPerSec)
	}
}