// Package testing 提供NetCore-Go的测试框架
// Author: NetCore-Go Team
// Created: 2024

package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/netcore-go/pkg/core"
)

// TestSuite 测试套件接口
type TestSuite interface {
	SetUp() error
	TearDown() error
	Name() string
}

// UnitTestSuite 单元测试套件
type UnitTestSuite struct {
	name    string
	logger  *core.Logger
	cleanup []func() error
}

// NewUnitTestSuite 创建单元测试套件
func NewUnitTestSuite(name string) *UnitTestSuite {
	return &UnitTestSuite{
		name:    name,
		logger:  core.NewLogger(core.DefaultLoggerConfig()),
		cleanup: make([]func() error, 0),
	}
}

// Name 返回测试套件名称
func (uts *UnitTestSuite) Name() string {
	return uts.name
}

// SetUp 设置测试环境
func (uts *UnitTestSuite) SetUp() error {
	uts.logger.Info(fmt.Sprintf("Setting up unit test suite: %s", uts.name))
	return nil
}

// TearDown 清理测试环境
func (uts *UnitTestSuite) TearDown() error {
	uts.logger.Info(fmt.Sprintf("Tearing down unit test suite: %s", uts.name))
	
	// 执行清理函数
	for _, cleanupFunc := range uts.cleanup {
		if err := cleanupFunc(); err != nil {
			uts.logger.ErrorWithErr("Cleanup function failed", err)
		}
	}
	
	return nil
}

// AddCleanup 添加清理函数
func (uts *UnitTestSuite) AddCleanup(cleanup func() error) {
	uts.cleanup = append(uts.cleanup, cleanup)
}

// IntegrationTestSuite 集成测试套件
type IntegrationTestSuite struct {
	name       string
	logger     *core.Logger
	server     *httptest.Server
	cleanup    []func() error
	databases  map[string]*MockDatabase
	caches     map[string]*MockCache
	services   map[string]interface{}
}

// NewIntegrationTestSuite 创建集成测试套件
func NewIntegrationTestSuite(name string) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		name:      name,
		logger:    core.NewLogger(core.DefaultLoggerConfig()),
		cleanup:   make([]func() error, 0),
		databases: make(map[string]*MockDatabase),
		caches:    make(map[string]*MockCache),
		services:  make(map[string]interface{}),
	}
}

// Name 返回测试套件名称
func (its *IntegrationTestSuite) Name() string {
	return its.name
}

// SetUp 设置集成测试环境
func (its *IntegrationTestSuite) SetUp() error {
	its.logger.Info(fmt.Sprintf("Setting up integration test suite: %s", its.name))
	
	// 启动测试服务器
	its.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Integration Test Server"))
	}))
	
	its.logger.Info(fmt.Sprintf("Test server started at: %s", its.server.URL))
	return nil
}

// TearDown 清理集成测试环境
func (its *IntegrationTestSuite) TearDown() error {
	its.logger.Info(fmt.Sprintf("Tearing down integration test suite: %s", its.name))
	
	// 关闭测试服务器
	if its.server != nil {
		its.server.Close()
	}
	
	// 执行清理函数
	for _, cleanupFunc := range its.cleanup {
		if err := cleanupFunc(); err != nil {
			its.logger.ErrorWithErr("Cleanup function failed", err)
		}
	}
	
	return nil
}

// GetServerURL 获取测试服务器URL
func (its *IntegrationTestSuite) GetServerURL() string {
	if its.server != nil {
		return its.server.URL
	}
	return ""
}

// AddMockDatabase 添加模拟数据库
func (its *IntegrationTestSuite) AddMockDatabase(name string, db *MockDatabase) {
	its.databases[name] = db
}

// GetMockDatabase 获取模拟数据库
func (its *IntegrationTestSuite) GetMockDatabase(name string) *MockDatabase {
	return its.databases[name]
}

// AddMockCache 添加模拟缓存
func (its *IntegrationTestSuite) AddMockCache(name string, cache *MockCache) {
	its.caches[name] = cache
}

// GetMockCache 获取模拟缓存
func (its *IntegrationTestSuite) GetMockCache(name string) *MockCache {
	return its.caches[name]
}

// E2ETestSuite 端到端测试套件
type E2ETestSuite struct {
	name        string
	logger      *core.Logger
	baseURL     string
	client      *http.Client
	cleanup     []func() error
	testData    map[string]interface{}
	scenarios   []TestScenario
}

// TestScenario 测试场景
type TestScenario struct {
	Name        string
	Description string
	Steps       []TestStep
	Expected    interface{}
}

// TestStep 测试步骤
type TestStep struct {
	Name        string
	Action      string
	Parameters  map[string]interface{}
	Expected    interface{}
	Validation  func(interface{}) error
}

// NewE2ETestSuite 创建端到端测试套件
func NewE2ETestSuite(name, baseURL string) *E2ETestSuite {
	return &E2ETestSuite{
		name:     name,
		logger:   core.NewLogger(core.DefaultLoggerConfig()),
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 30 * time.Second},
		cleanup:  make([]func() error, 0),
		testData: make(map[string]interface{}),
		scenarios: make([]TestScenario, 0),
	}
}

// GetScenarios 获取所有测试场景
func (e2e *E2ETestSuite) GetScenarios() []TestScenario {
	return e2e.scenarios
}

// RunAllScenarios 运行所有测试场景
func (e2e *E2ETestSuite) RunAllScenarios() error {
	for _, scenario := range e2e.scenarios {
		if err := e2e.RunScenario(scenario); err != nil {
			return err
		}
	}
	return nil
}

// RunScenario 运行指定的测试场景
func (e2e *E2ETestSuite) RunScenario(scenario TestScenario) error {
	e2e.logger.Info(fmt.Sprintf("Running scenario: %s", scenario.Name))
	
	for i, step := range scenario.Steps {
		e2e.logger.Info(fmt.Sprintf("Executing step %d: %s", i+1, step.Name))
		
		result, err := e2e.executeStep(step)
		if err != nil {
			e2e.logger.ErrorWithErr(fmt.Sprintf("Step %d failed: %s", i+1, step.Name), err)
			return fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
		}
		
		// 执行验证
		if step.Validation != nil {
			if err := step.Validation(result); err != nil {
				e2e.logger.ErrorWithErr(fmt.Sprintf("Step %d validation failed: %s", i+1, step.Name), err)
				return fmt.Errorf("step %d (%s) validation failed: %w", i+1, step.Name, err)
			}
		}
		
		e2e.logger.Info(fmt.Sprintf("Step %d completed: %s", i+1, step.Name))
	}
	
	e2e.logger.Info(fmt.Sprintf("Scenario completed: %s", scenario.Name))
	return nil
}

// Name 返回测试套件名称
func (e2e *E2ETestSuite) Name() string {
	return e2e.name
}

// SetUp 设置端到端测试环境
func (e2e *E2ETestSuite) SetUp() error {
	e2e.logger.Info(fmt.Sprintf("Setting up E2E test suite: %s", e2e.name))
	e2e.logger.Info(fmt.Sprintf("Base URL: %s", e2e.baseURL))
	
	// 等待服务启动
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for service to start")
		default:
			resp, err := e2e.client.Get(e2e.baseURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				e2e.logger.Info("Service is ready for E2E testing")
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// TearDown 清理端到端测试环境
func (e2e *E2ETestSuite) TearDown() error {
	e2e.logger.Info(fmt.Sprintf("Tearing down E2E test suite: %s", e2e.name))
	
	// 执行清理函数
	for _, cleanupFunc := range e2e.cleanup {
		if err := cleanupFunc(); err != nil {
			e2e.logger.ErrorWithErr("Cleanup function failed", err)
		}
	}
	
	return nil
}

// AddScenario 添加测试场景
func (e2e *E2ETestSuite) AddScenario(scenario TestScenario) {
	e2e.scenarios = append(e2e.scenarios, scenario)
}

// RunScenarios 运行所有测试场景
func (e2e *E2ETestSuite) RunScenarios(t *testing.T) {
	for _, scenario := range e2e.scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			e2e.runScenario(t, scenario)
		})
	}
}

// runScenario 运行单个测试场景
func (e2e *E2ETestSuite) runScenario(t *testing.T, scenario TestScenario) {
	e2e.logger.Info(fmt.Sprintf("Running scenario: %s", scenario.Name))
	
	for i, step := range scenario.Steps {
		e2e.logger.Info(fmt.Sprintf("Executing step %d: %s", i+1, step.Name))
		
		result, err := e2e.executeStep(step)
		if err != nil {
			t.Fatalf("Step %d failed: %v", i+1, err)
		}
		
		if step.Validation != nil {
			if err := step.Validation(result); err != nil {
				t.Fatalf("Step %d validation failed: %v", i+1, err)
			}
		}
	}
	
	e2e.logger.Info(fmt.Sprintf("Scenario completed: %s", scenario.Name))
}

// executeStep 执行测试步骤
func (e2e *E2ETestSuite) executeStep(step TestStep) (interface{}, error) {
	switch step.Action {
	case "GET":
		return e2e.executeHTTPGet(step)
	case "POST":
		return e2e.executeHTTPPost(step)
	case "PUT":
		return e2e.executeHTTPPut(step)
	case "DELETE":
		return e2e.executeHTTPDelete(step)
	case "WAIT":
		return e2e.executeWait(step)
	default:
		return nil, fmt.Errorf("unknown action: %s", step.Action)
	}
}

// executeHTTPGet 执行HTTP GET请求
func (e2e *E2ETestSuite) executeHTTPGet(step TestStep) (interface{}, error) {
	url := e2e.baseURL + step.Parameters["path"].(string)
	resp, err := e2e.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"response":    resp,
	}, nil
}

// executeHTTPPost 执行HTTP POST请求
func (e2e *E2ETestSuite) executeHTTPPost(step TestStep) (interface{}, error) {
	url := e2e.baseURL + step.Parameters["path"].(string)
	
	// 获取请求体数据
	var body io.Reader
	if bodyData, exists := step.Parameters["body"]; exists {
		if bodyStr, ok := bodyData.(string); ok {
			body = strings.NewReader(bodyStr)
		} else if bodyBytes, ok := bodyData.([]byte); ok {
			body = bytes.NewReader(bodyBytes)
		} else {
			// 尝试JSON序列化
			jsonData, err := json.Marshal(bodyData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(jsonData)
		}
	}
	
	// 创建POST请求
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	
	// 设置请求头
	if headers, exists := step.Parameters["headers"]; exists {
		if headerMap, ok := headers.(map[string]string); ok {
			for key, value := range headerMap {
				req.Header.Set(key, value)
			}
		}
	}
	
	// 如果没有设置Content-Type且有body，默认设置为application/json
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	
	// 执行请求
	resp, err := e2e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
		"response":    resp,
	}, nil
}

// executeHTTPPut 执行HTTP PUT请求
func (e2e *E2ETestSuite) executeHTTPPut(step TestStep) (interface{}, error) {
	url := e2e.baseURL + step.Parameters["path"].(string)
	
	// 获取请求体数据
	var body io.Reader
	if bodyData, exists := step.Parameters["body"]; exists {
		if bodyStr, ok := bodyData.(string); ok {
			body = strings.NewReader(bodyStr)
		} else if bodyBytes, ok := bodyData.([]byte); ok {
			body = bytes.NewReader(bodyBytes)
		} else {
			// 尝试JSON序列化
			jsonData, err := json.Marshal(bodyData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(jsonData)
		}
	}
	
	// 创建PUT请求
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	
	// 设置请求头
	if headers, exists := step.Parameters["headers"]; exists {
		if headerMap, ok := headers.(map[string]string); ok {
			for key, value := range headerMap {
				req.Header.Set(key, value)
			}
		}
	}
	
	// 如果没有设置Content-Type且有body，默认设置为application/json
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	
	// 执行请求
	resp, err := e2e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PUT request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
		"response":    resp,
	}, nil
}

// executeHTTPDelete 执行HTTP DELETE请求
func (e2e *E2ETestSuite) executeHTTPDelete(step TestStep) (interface{}, error) {
	url := e2e.baseURL + step.Parameters["path"].(string)
	
	// 创建DELETE请求
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}
	
	// 设置请求头
	if headers, exists := step.Parameters["headers"]; exists {
		if headerMap, ok := headers.(map[string]string); ok {
			for key, value := range headerMap {
				req.Header.Set(key, value)
			}
		}
	}
	
	// 执行请求
	resp, err := e2e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DELETE request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
		"response":    resp,
	}, nil
}

// executeWait 执行等待操作
func (e2e *E2ETestSuite) executeWait(step TestStep) (interface{}, error) {
	duration := step.Parameters["duration"].(time.Duration)
	time.Sleep(duration)
	return nil, nil
}

// MockDatabase 模拟数据库
type MockDatabase struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewMockDatabase 创建模拟数据库
func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		data: make(map[string]interface{}),
	}
}

// Set 设置数据
func (db *MockDatabase) Set(key string, value interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = value
}

// Get 获取数据
func (db *MockDatabase) Get(key string) (interface{}, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	value, exists := db.data[key]
	return value, exists
}

// Delete 删除数据
func (db *MockDatabase) Delete(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, key)
}

// Clear 清空数据
func (db *MockDatabase) Clear() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data = make(map[string]interface{})
}

// MockCache 模拟缓存
type MockCache struct {
	mu   sync.RWMutex
	data map[string]CacheItem
}

// CacheItem 缓存项
type CacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
}

// NewMockCache 创建模拟缓存
func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]CacheItem),
	}
}

// Set 设置缓存
func (c *MockCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	item := CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	c.data[key] = item
}

// Get 获取缓存
func (c *MockCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.data[key]
	if !exists {
		return nil, false
	}
	
	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		delete(c.data, key)
		return nil, false
	}
	
	return item.Value, true
}

// Delete 删除缓存
func (c *MockCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear 清空缓存
func (c *MockCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]CacheItem)
}

// TestRunner 测试运行器
type TestRunner struct {
	suites []TestSuite
	logger *core.Logger
}

// NewTestRunner 创建测试运行器
func NewTestRunner() *TestRunner {
	return &TestRunner{
		suites: make([]TestSuite, 0),
		logger: core.NewLogger(core.DefaultLoggerConfig()),
	}
}

// AddSuite 添加测试套件
func (tr *TestRunner) AddSuite(suite TestSuite) {
	tr.suites = append(tr.suites, suite)
}

// RunAll 运行所有测试套件
func (tr *TestRunner) RunAll() error {
	tr.logger.Info("Starting test runner")
	
	for _, suite := range tr.suites {
		tr.logger.Info(fmt.Sprintf("Running test suite: %s", suite.Name()))
		
		if err := suite.SetUp(); err != nil {
			tr.logger.ErrorWithErr(fmt.Sprintf("Failed to set up suite: %s", suite.Name()), err)
			continue
		}
		
		// 这里应该运行实际的测试
		// 由于这是框架代码，实际测试由具体的测试文件执行
		
		if err := suite.TearDown(); err != nil {
			tr.logger.ErrorWithErr(fmt.Sprintf("Failed to tear down suite: %s", suite.Name()), err)
		}
		
		tr.logger.Info(fmt.Sprintf("Completed test suite: %s", suite.Name()))
	}
	
	tr.logger.Info("Test runner completed")
	return nil
}

// BenchmarkSuite 性能测试套件
type BenchmarkSuite struct {
	name      string
	logger    *core.Logger
	benchmarks map[string]func(*testing.B)
}

// NewBenchmarkSuite 创建性能测试套件
func NewBenchmarkSuite(name string) *BenchmarkSuite {
	return &BenchmarkSuite{
		name:       name,
		logger:     core.NewLogger(core.DefaultLoggerConfig()),
		benchmarks: make(map[string]func(*testing.B)),
	}
}

// AddBenchmark 添加性能测试
func (bs *BenchmarkSuite) AddBenchmark(name string, benchmark func(*testing.B)) {
	bs.benchmarks[name] = benchmark
}

// RunBenchmarks 运行性能测试
func (bs *BenchmarkSuite) RunBenchmarks(t *testing.T) {
	for name, benchmark := range bs.benchmarks {
		t.Run(name, func(t *testing.T) {
			result := testing.Benchmark(benchmark)
			bs.logger.Info(fmt.Sprintf("Benchmark %s: %s", name, result.String()))
		})
	}
}

// LoadTestConfig 负载测试配置
type LoadTestConfig struct {
	Concurrency int           `json:"concurrency"`
	Duration    time.Duration `json:"duration"`
	RampUp      time.Duration `json:"ramp_up"`
	TargetURL   string        `json:"target_url"`
	RequestRate int           `json:"request_rate"`
}

// LoadTestResult 负载测试结果
type LoadTestResult struct {
	TotalRequests    int64         `json:"total_requests"`
	SuccessfulReqs   int64         `json:"successful_requests"`
	FailedReqs       int64         `json:"failed_requests"`
	AverageLatency   time.Duration `json:"average_latency"`
	MinLatency       time.Duration `json:"min_latency"`
	MaxLatency       time.Duration `json:"max_latency"`
	RequestsPerSec   float64       `json:"requests_per_second"`
	ErrorRate        float64       `json:"error_rate"`
}

// LoadTester 负载测试器
type LoadTester struct {
	config *LoadTestConfig
	logger *core.Logger
	client *http.Client
}

// NewLoadTester 创建负载测试器
func NewLoadTester(config *LoadTestConfig) *LoadTester {
	return &LoadTester{
		config: config,
		logger: core.NewLogger(core.DefaultLoggerConfig()),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Run 运行负载测试
func (lt *LoadTester) Run() (*LoadTestResult, error) {
	lt.logger.Info("Starting load test")
	lt.logger.Info(fmt.Sprintf("Target: %s, Concurrency: %d, Duration: %s", 
		lt.config.TargetURL, lt.config.Concurrency, lt.config.Duration))
	
	result := &LoadTestResult{}
	startTime := time.Now()
	endTime := startTime.Add(lt.config.Duration)
	
	// 启动并发请求
	var wg sync.WaitGroup
	resultsChan := make(chan RequestResult, lt.config.Concurrency*100)
	
	for i := 0; i < lt.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lt.runWorker(endTime, resultsChan)
		}()
	}
	
	// 等待所有工作协程完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// 收集结果
	var latencies []time.Duration
	for reqResult := range resultsChan {
		result.TotalRequests++
		if reqResult.Success {
			result.SuccessfulReqs++
			latencies = append(latencies, reqResult.Latency)
		} else {
			result.FailedReqs++
		}
	}
	
	// 计算统计信息
	if len(latencies) > 0 {
		result.MinLatency = latencies[0]
		result.MaxLatency = latencies[0]
		totalLatency := time.Duration(0)
		
		for _, latency := range latencies {
			totalLatency += latency
			if latency < result.MinLatency {
				result.MinLatency = latency
			}
			if latency > result.MaxLatency {
				result.MaxLatency = latency
			}
		}
		
		result.AverageLatency = totalLatency / time.Duration(len(latencies))
	}
	
	totalDuration := time.Since(startTime)
	result.RequestsPerSec = float64(result.TotalRequests) / totalDuration.Seconds()
	result.ErrorRate = float64(result.FailedReqs) / float64(result.TotalRequests) * 100
	
	lt.logger.Info("Load test completed")
	lt.logger.Info(fmt.Sprintf("Results: %d total, %d success, %d failed, %.2f req/s, %.2f%% error rate",
		result.TotalRequests, result.SuccessfulReqs, result.FailedReqs, 
		result.RequestsPerSec, result.ErrorRate))
	
	return result, nil
}

// RequestResult 请求结果
type RequestResult struct {
	Success bool
	Latency time.Duration
	Error   error
}

// runWorker 运行工作协程
func (lt *LoadTester) runWorker(endTime time.Time, results chan<- RequestResult) {
	for time.Now().Before(endTime) {
		startTime := time.Now()
		
		resp, err := lt.client.Get(lt.config.TargetURL)
		latency := time.Since(startTime)
		
		result := RequestResult{
			Latency: latency,
		}
		
		if err != nil {
			result.Success = false
			result.Error = err
		} else {
			resp.Body.Close()
			result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
		}
		
		results <- result
		
		// 控制请求速率
		if lt.config.RequestRate > 0 {
			time.Sleep(time.Second / time.Duration(lt.config.RequestRate))
		}
	}
}