// Package main 演示NetCore-Go测试框架的使用
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	testing "github.com/phuhao00/netcore-go/pkg/testing"
)

func main() {
	fmt.Println("🧪 NetCore-Go Testing Framework Example")

	// 创建单元测试套件
	unitSuite := testing.NewUnitTestSuite("Example Unit Tests")
	runUnitTests(unitSuite)

	// 创建集成测试套件
	integrationSuite := testing.NewIntegrationTestSuite("Example Integration Tests")
	runIntegrationTests(integrationSuite)

	// 创建端到端测试套件
	e2eSuite := testing.NewE2ETestSuite("Example E2E Tests", "http://localhost:8080")
	runE2ETests(e2eSuite)

	fmt.Println("✅ All tests completed!")
}

// runUnitTests 运行单元测试
func runUnitTests(suite *testing.UnitTestSuite) {
	fmt.Println("\n📋 Running Unit Tests...")

	if err := suite.SetUp(); err != nil {
		log.Printf("Failed to setup unit tests: %v", err)
		return
	}
	defer suite.TearDown()

	// 添加清理函数示例
	suite.AddCleanup(func() error {
		fmt.Println("🧹 Cleaning up unit test resources...")
		return nil
	})

	fmt.Printf("✅ Unit test suite '%s' completed\n", suite.Name())
}

// runIntegrationTests 运行集成测试
func runIntegrationTests(suite *testing.IntegrationTestSuite) {
	fmt.Println("\n🔗 Running Integration Tests...")

	if err := suite.SetUp(); err != nil {
		log.Printf("Failed to setup integration tests: %v", err)
		return
	}
	defer suite.TearDown()

	// 添加模拟数据库
	mockDB := testing.NewMockDatabase()
	mockDB.Set("user:1", map[string]interface{}{
		"id":    1,
		"name":  "John Doe",
		"email": "john@example.com",
	})
	suite.AddMockDatabase("users", mockDB)

	// 添加模拟缓存
	mockCache := testing.NewMockCache()
	mockCache.Set("session:abc123", "user:1", 30*time.Minute)
	suite.AddMockCache("sessions", mockCache)

	// 测试模拟数据库
	if db := suite.GetMockDatabase("users"); db != nil {
		if user, exists := db.Get("user:1"); exists {
			fmt.Printf("📊 Retrieved user from mock database: %+v\n", user)
		}
	}

	// 测试模拟缓存
	if cache := suite.GetMockCache("sessions"); cache != nil {
		if session, exists := cache.Get("session:abc123"); exists {
			fmt.Printf("💾 Retrieved session from mock cache: %v\n", session)
		}
	}

	fmt.Printf("✅ Integration test suite '%s' completed\n", suite.Name())
	fmt.Printf("🌐 Test server URL: %s\n", suite.GetServerURL())
}

// runE2ETests 运行端到端测试
func runE2ETests(suite *testing.E2ETestSuite) {
	fmt.Println("\n🎯 Running E2E Tests...")

	if err := suite.SetUp(); err != nil {
		log.Printf("Failed to setup E2E tests: %v", err)
		return
	}
	defer suite.TearDown()

	// 创建测试场景
	scenario := testing.TestScenario{
		Name:        "API CRUD Operations",
		Description: "Test complete CRUD operations on API endpoints",
		Steps: []testing.TestStep{
			{
				Name:   "Create User",
				Action: "POST",
				Parameters: map[string]interface{}{
					"path": "/api/users",
					"body": map[string]interface{}{
						"name":  "Alice Smith",
						"email": "alice@example.com",
						"age":   28,
					},
					"headers": map[string]string{
						"Content-Type":  "application/json",
						"Authorization": "Bearer test-token",
					},
				},
				Validation: func(result interface{}) error {
					if resp, ok := result.(map[string]interface{}); ok {
						if statusCode, ok := resp["status_code"].(int); ok {
							if statusCode != http.StatusCreated {
								return fmt.Errorf("expected status 201, got %d", statusCode)
							}
						}
					}
					return nil
				},
			},
			{
				Name:   "Get User",
				Action: "GET",
				Parameters: map[string]interface{}{
					"path": "/api/users/1",
					"headers": map[string]string{
						"Authorization": "Bearer test-token",
					},
				},
				Validation: func(result interface{}) error {
					if resp, ok := result.(map[string]interface{}); ok {
						if statusCode, ok := resp["status_code"].(int); ok {
							if statusCode != http.StatusOK {
								return fmt.Errorf("expected status 200, got %d", statusCode)
							}
						}
					}
					return nil
				},
			},
			{
				Name:   "Update User",
				Action: "PUT",
				Parameters: map[string]interface{}{
					"path": "/api/users/1",
					"body": map[string]interface{}{
						"name":  "Alice Johnson",
						"email": "alice.johnson@example.com",
						"age":   29,
					},
					"headers": map[string]string{
						"Content-Type":  "application/json",
						"Authorization": "Bearer test-token",
					},
				},
				Validation: func(result interface{}) error {
					if resp, ok := result.(map[string]interface{}); ok {
						if statusCode, ok := resp["status_code"].(int); ok {
							if statusCode != http.StatusOK {
								return fmt.Errorf("expected status 200, got %d", statusCode)
							}
						}
					}
					return nil
				},
			},
			{
				Name:   "Wait Before Delete",
				Action: "WAIT",
				Parameters: map[string]interface{}{
					"duration": 1 * time.Second,
				},
			},
			{
				Name:   "Delete User",
				Action: "DELETE",
				Parameters: map[string]interface{}{
					"path": "/api/users/1",
					"headers": map[string]string{
						"Authorization": "Bearer test-token",
					},
				},
				Validation: func(result interface{}) error {
					if resp, ok := result.(map[string]interface{}); ok {
						if statusCode, ok := resp["status_code"].(int); ok {
							if statusCode != http.StatusNoContent && statusCode != http.StatusOK {
								return fmt.Errorf("expected status 200 or 204, got %d", statusCode)
							}
						}
					}
					return nil
				},
			},
		},
	}

	// 添加测试场景
	suite.AddScenario(scenario)

	// 运行测试场景（注意：这里只是演示，实际运行需要真实的API服务器）
	fmt.Printf("📝 Added test scenario: %s\n", scenario.Name)
	fmt.Printf("📊 Scenario has %d test steps\n", len(scenario.Steps))

	// 演示各种HTTP方法的参数格式
	fmt.Println("\n📋 HTTP Methods Usage Examples:")
	fmt.Println("POST: Creates resources with request body")
	fmt.Println("GET: Retrieves resources")
	fmt.Println("PUT: Updates entire resources with request body")
	fmt.Println("DELETE: Removes resources")
	fmt.Println("WAIT: Adds delays between operations")

	fmt.Printf("✅ E2E test suite '%s' completed\n", suite.Name())
}
