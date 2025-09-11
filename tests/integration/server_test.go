// Package integration 提供NetCore-Go的集成测试
// Author: NetCore-Go Team
// Created: 2024

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/testing"
)

// TestServerIntegration 服务器集成测试
func TestServerIntegration(t *testing.T) {
	// 创建集成测试套件
	suite := testing.NewIntegrationTestSuite("ServerIntegration")
	
	// 设置测试环境
	if err := suite.SetUp(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}
	defer suite.TearDown()
	
	// 创建测试服务器
	server := createTestServer()
	testServer := httptest.NewServer(server)
	defer testServer.Close()
	
	// 运行集成测试
	t.Run("HealthCheck", func(t *testing.T) {
		testHealthCheck(t, testServer.URL)
	})
	
	t.Run("ErrorHandling", func(t *testing.T) {
		testErrorHandling(t, testServer.URL)
	})
	
	t.Run("RequestValidation", func(t *testing.T) {
		testRequestValidation(t, testServer.URL)
	})
	
	t.Run("Logging", func(t *testing.T) {
		testLogging(t, testServer.URL)
	})
	
	t.Run("Concurrency", func(t *testing.T) {
		testConcurrency(t, testServer.URL)
	})
}

// createTestServer 创建测试服务器
func createTestServer() http.Handler {
	mux := http.NewServeMux()
	
	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   "netcore-go-test",
		})
	})
	
	// 错误处理测试端点
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		errorType := r.URL.Query().Get("type")
		
		var err *core.NetCoreError
		switch errorType {
		case "not_found":
			err = core.NewError(core.ErrCodeNotFound, "Resource not found")
		case "invalid_request":
			err = core.NewError(core.ErrCodeInvalidRequest, "Invalid request parameters")
		case "timeout":
			err = core.NewError(core.ErrCodeTimeout, "Request timeout")
		default:
			err = core.NewError(core.ErrCodeInternal, "Internal server error")
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(err.HTTPStatusCode())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
			"code":  err.Code,
		})
	})
	
	// 验证测试端点
	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid JSON",
			})
			return
		}
		
		// 简单验证
		if name, ok := data["name"].(string); !ok || name == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Name is required",
			})
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Validation successful",
			"data":    data,
		})
	})
	
	// 日志测试端点
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		logger := core.GetGlobalLogger()
		
		level := r.URL.Query().Get("level")
		message := r.URL.Query().Get("message")
		
		if message == "" {
			message = "Test log message"
		}
		
		switch level {
		case "debug":
			logger.Debug(message)
		case "info":
			logger.Info(message)
		case "warn":
			logger.Warn(message)
		case "error":
			logger.Error(message)
		default:
			logger.Info(message)
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Log written",
			"level":   level,
		})
	})
	
	// 并发测试端点
	mux.HandleFunc("/concurrent", func(w http.ResponseWriter, r *http.Request) {
		// 模拟一些处理时间
		time.Sleep(100 * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "Concurrent request processed",
			"timestamp": time.Now().Unix(),
			"goroutine": fmt.Sprintf("%p", &r),
		})
	})
	
	return mux
}

// testHealthCheck 测试健康检查
func testHealthCheck(t *testing.T, baseURL string) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if status, ok := result["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}
	
	if _, ok := result["timestamp"]; !ok {
		t.Error("Expected timestamp in response")
	}
	
	if service, ok := result["service"].(string); !ok || service != "netcore-go-test" {
		t.Errorf("Expected service 'netcore-go-test', got %v", result["service"])
	}
}

// testErrorHandling 测试错误处理
func testErrorHandling(t *testing.T, baseURL string) {
	tests := []struct {
		name           string
		errorType      string
		expectedStatus int
		expectedCode   string
	}{
		{"Not Found", "not_found", http.StatusNotFound, "NOT_FOUND"},
		{"Invalid Request", "invalid_request", http.StatusBadRequest, "INVALID_REQUEST"},
		{"Timeout", "timeout", http.StatusRequestTimeout, "TIMEOUT"},
		{"Internal Error", "internal", http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s/error?type=%s", baseURL, tt.errorType))
			if err != nil {
				t.Fatalf("Error request failed: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
			
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}
			
			if code, ok := result["code"].(string); !ok || code != tt.expectedCode {
				t.Errorf("Expected error code '%s', got %v", tt.expectedCode, result["code"])
			}
			
			if _, ok := result["error"]; !ok {
				t.Error("Expected error message in response")
			}
		})
	}
}

// testRequestValidation 测试请求验证
func testRequestValidation(t *testing.T, baseURL string) {
	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
	}{
		{
			"Valid Request",
			map[string]interface{}{"name": "John Doe", "age": 30},
			http.StatusOK,
		},
		{
			"Missing Name",
			map[string]interface{}{"age": 30},
			http.StatusBadRequest,
		},
		{
			"Empty Name",
			map[string]interface{}{"name": "", "age": 30},
			http.StatusBadRequest,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(tt.payload)
			resp, err := http.Post(
				baseURL+"/validate",
				"application/json",
				bytes.NewBuffer(payloadBytes),
			)
			if err != nil {
				t.Fatalf("Validation request failed: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
			
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode validation response: %v", err)
			}
			
			if tt.expectedStatus == http.StatusOK {
				if message, ok := result["message"].(string); !ok || !strings.Contains(message, "successful") {
					t.Errorf("Expected success message, got %v", result["message"])
				}
			} else {
				if _, ok := result["error"]; !ok {
					t.Error("Expected error message in validation response")
				}
			}
		})
	}
	
	// 测试无效JSON
	t.Run("Invalid JSON", func(t *testing.T) {
		resp, err := http.Post(
			baseURL+"/validate",
			"application/json",
			bytes.NewBuffer([]byte("invalid json")),
		)
		if err != nil {
			t.Fatalf("Invalid JSON request failed: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
		}
	})
}

// testLogging 测试日志功能
func testLogging(t *testing.T, baseURL string) {
	levels := []string{"debug", "info", "warn", "error"}
	
	for _, level := range levels {
		t.Run(fmt.Sprintf("Log Level %s", level), func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s/log?level=%s&message=Test %s message", baseURL, level, level))
			if err != nil {
				t.Fatalf("Log request failed: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for log request, got %d", resp.StatusCode)
			}
			
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode log response: %v", err)
			}
			
			if message, ok := result["message"].(string); !ok || message != "Log written" {
				t.Errorf("Expected message 'Log written', got %v", result["message"])
			}
			
			if resultLevel, ok := result["level"].(string); !ok || resultLevel != level {
				t.Errorf("Expected level '%s', got %v", level, result["level"])
			}
		})
	}
}

// testConcurrency 测试并发处理
func testConcurrency(t *testing.T, baseURL string) {
	concurrency := 10
	requests := 50
	
	resultChan := make(chan error, concurrency*requests)
	
	// 启动并发请求
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < requests; j++ {
				resp, err := http.Get(baseURL + "/concurrent")
				if err != nil {
					resultChan <- err
					continue
				}
				
				if resp.StatusCode != http.StatusOK {
					resultChan <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					resp.Body.Close()
					continue
				}
				
				var result map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					resp.Body.Close()
					resultChan <- err
					continue
				}
				resp.Body.Close()
				
				if message, ok := result["message"].(string); !ok || !strings.Contains(message, "processed") {
					resultChan <- fmt.Errorf("unexpected message: %v", result["message"])
					continue
				}
				
				resultChan <- nil
			}
		}()
	}
	
	// 收集结果
	totalRequests := concurrency * requests
	successCount := 0
	errorCount := 0
	
	for i := 0; i < totalRequests; i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				errorCount++
				t.Logf("Concurrent request error: %v", err)
			} else {
				successCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests to complete")
		}
	}
	
	// 验证结果
	if errorCount > totalRequests/10 { // 允许10%的错误率
		t.Errorf("Too many errors in concurrent requests: %d/%d", errorCount, totalRequests)
	}
	
	if successCount < totalRequests*9/10 { // 至少90%成功
		t.Errorf("Too few successful concurrent requests: %d/%d", successCount, totalRequests)
	}
	
	t.Logf("Concurrent test completed: %d success, %d errors out of %d total requests", 
		successCount, errorCount, totalRequests)
}

// TestDatabaseIntegration 数据库集成测试
func TestDatabaseIntegration(t *testing.T) {
	// 创建集成测试套件
	suite := testing.NewIntegrationTestSuite("DatabaseIntegration")
	
	// 设置测试环境
	if err := suite.SetUp(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}
	defer suite.TearDown()
	
	// 添加模拟数据库
	mockDB := testing.NewMockDatabase()
	suite.AddMockDatabase("primary", mockDB)
	
	// 测试数据库操作
	t.Run("CRUD Operations", func(t *testing.T) {
		testDatabaseCRUD(t, mockDB)
	})
	
	t.Run("Concurrent Access", func(t *testing.T) {
		testDatabaseConcurrency(t, mockDB)
	})
}

// testDatabaseCRUD 测试数据库CRUD操作
func testDatabaseCRUD(t *testing.T, db *testing.MockDatabase) {
	// Create
	db.Set("user:1", map[string]interface{}{
		"id":   1,
		"name": "John Doe",
		"email": "john@example.com",
	})
	
	// Read
	user, exists := db.Get("user:1")
	if !exists {
		t.Error("Expected user to exist")
	}
	
	userData, ok := user.(map[string]interface{})
	if !ok {
		t.Fatal("Expected user data to be a map")
	}
	
	if name, ok := userData["name"].(string); !ok || name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %v", userData["name"])
	}
	
	// Update
	userData["name"] = "Jane Doe"
	db.Set("user:1", userData)
	
	updatedUser, _ := db.Get("user:1")
	updatedUserData := updatedUser.(map[string]interface{})
	if name, ok := updatedUserData["name"].(string); !ok || name != "Jane Doe" {
		t.Errorf("Expected updated name 'Jane Doe', got %v", updatedUserData["name"])
	}
	
	// Delete
	db.Delete("user:1")
	_, exists = db.Get("user:1")
	if exists {
		t.Error("Expected user to be deleted")
	}
}

// testDatabaseConcurrency 测试数据库并发访问
func testDatabaseConcurrency(t *testing.T, db *testing.MockDatabase) {
	concurrency := 10
	operations := 100
	
	resultChan := make(chan error, concurrency*operations)
	
	// 启动并发操作
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("worker:%d:item:%d", workerID, j)
				value := map[string]interface{}{
					"worker": workerID,
					"item":   j,
					"data":   fmt.Sprintf("data-%d-%d", workerID, j),
				}
				
				// Set
				db.Set(key, value)
				
				// Get
				retrieved, exists := db.Get(key)
				if !exists {
					resultChan <- fmt.Errorf("key %s not found after set", key)
					continue
				}
				
				retrievedData, ok := retrieved.(map[string]interface{})
				if !ok {
					resultChan <- fmt.Errorf("invalid data type for key %s", key)
					continue
				}
				
				if retrievedData["worker"] != workerID {
					resultChan <- fmt.Errorf("data corruption for key %s", key)
					continue
				}
				
				resultChan <- nil
			}
		}(i)
	}
	
	// 收集结果
	totalOperations := concurrency * operations
	errorCount := 0
	
	for i := 0; i < totalOperations; i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				errorCount++
				t.Logf("Database concurrent operation error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for database operations to complete")
		}
	}
	
	if errorCount > 0 {
		t.Errorf("Database concurrent operations had %d errors out of %d total", errorCount, totalOperations)
	}
}

// TestCacheIntegration 缓存集成测试
func TestCacheIntegration(t *testing.T) {
	// 创建集成测试套件
	suite := testing.NewIntegrationTestSuite("CacheIntegration")
	
	// 设置测试环境
	if err := suite.SetUp(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}
	defer suite.TearDown()
	
	// 添加模拟缓存
	mockCache := testing.NewMockCache()
	suite.AddMockCache("redis", mockCache)
	
	// 测试缓存操作
	t.Run("Basic Operations", func(t *testing.T) {
		testCacheBasicOperations(t, mockCache)
	})
	
	t.Run("TTL and Expiration", func(t *testing.T) {
		testCacheTTL(t, mockCache)
	})
	
	t.Run("Concurrent Access", func(t *testing.T) {
		testCacheConcurrency(t, mockCache)
	})
}

// testCacheBasicOperations 测试缓存基本操作
func testCacheBasicOperations(t *testing.T, cache *testing.MockCache) {
	// Set and Get
	cache.Set("key1", "value1", time.Hour)
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1', got %v", value)
	}
	
	// Get non-existent key
	_, exists = cache.Get("nonexistent")
	if exists {
		t.Error("Expected nonexistent key to not exist")
	}
	
	// Delete
	cache.Delete("key1")
	_, exists = cache.Get("key1")
	if exists {
		t.Error("Expected key1 to be deleted")
	}
}

// testCacheTTL 测试缓存TTL
func testCacheTTL(t *testing.T, cache *testing.MockCache) {
	// Set with short TTL
	cache.Set("ttl_key", "ttl_value", 100*time.Millisecond)
	
	// Should exist immediately
	value, exists := cache.Get("ttl_key")
	if !exists || value != "ttl_value" {
		t.Error("Expected ttl_key to exist with correct value")
	}
	
	// Wait for expiration
	time.Sleep(150 * time.Millisecond)
	
	// Should be expired
	_, exists = cache.Get("ttl_key")
	if exists {
		t.Error("Expected ttl_key to be expired")
	}
}

// testCacheConcurrency 测试缓存并发访问
func testCacheConcurrency(t *testing.T, cache *testing.MockCache) {
	concurrency := 10
	operations := 50
	
	resultChan := make(chan error, concurrency*operations)
	
	// 启动并发操作
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("cache_worker:%d:item:%d", workerID, j)
				value := fmt.Sprintf("value-%d-%d", workerID, j)
				
				// Set
				cache.Set(key, value, time.Hour)
				
				// Get
				retrieved, exists := cache.Get(key)
				if !exists {
					resultChan <- fmt.Errorf("key %s not found after set", key)
					continue
				}
				
				if retrieved != value {
					resultChan <- fmt.Errorf("value mismatch for key %s: expected %s, got %v", key, value, retrieved)
					continue
				}
				
				resultChan <- nil
			}
		}(i)
	}
	
	// 收集结果
	totalOperations := concurrency * operations
	errorCount := 0
	
	for i := 0; i < totalOperations; i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				errorCount++
				t.Logf("Cache concurrent operation error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for cache operations to complete")
		}
	}
	
	if errorCount > 0 {
		t.Errorf("Cache concurrent operations had %d errors out of %d total", errorCount, totalOperations)
	}
}

// TestLoadTest 负载测试
func TestLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}
	
	// 创建测试服务器
	server := createTestServer()
	testServer := httptest.NewServer(server)
	defer testServer.Close()
	
	// 配置负载测试
	config := &testing.LoadTestConfig{
		Concurrency: 5,
		Duration:    10 * time.Second,
		TargetURL:   testServer.URL + "/health",
		RequestRate: 10, // 10 requests per second per goroutine
	}
	
	// 运行负载测试
	loadTester := testing.NewLoadTester(config)
	result, err := loadTester.Run()
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}
	
	// 验证结果
	if result.ErrorRate > 5.0 { // 允许5%的错误率
		t.Errorf("Error rate too high: %.2f%%", result.ErrorRate)
	}
	
	if result.RequestsPerSec < 10.0 { // 至少10 RPS
		t.Errorf("Request rate too low: %.2f RPS", result.RequestsPerSec)
	}
	
	if result.AverageLatency > 500*time.Millisecond { // 平均延迟不超过500ms
		t.Errorf("Average latency too high: %v", result.AverageLatency)
	}
	
	t.Logf("Load test results: %d total requests, %.2f RPS, %.2f%% error rate, avg latency: %v",
		result.TotalRequests, result.RequestsPerSec, result.ErrorRate, result.AverageLatency)
}