// Package testing 综合测试框架
// Author: NetCore-Go Team
// Created: 2024

package testing

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
)

// TestType 测试类型
type TestType string

const (
	TestTypeUnit        TestType = "unit"
	TestTypeIntegration TestType = "integration"
	TestTypeLoad        TestType = "load"
	TestTypeStress      TestType = "stress"
	TestTypeChaos       TestType = "chaos"
	TestTypeE2E         TestType = "e2e"
	TestTypeSmoke       TestType = "smoke"
	TestTypeRegression  TestType = "regression"
)

// TestStatus 测试状态
type TestStatus string

const (
	TestStatusPending TestStatus = "pending"
	TestStatusRunning TestStatus = "running"
	TestStatusPassed  TestStatus = "passed"
	TestStatusFailed  TestStatus = "failed"
	TestStatusSkipped TestStatus = "skipped"
	TestStatusTimeout TestStatus = "timeout"
)

// TestFramework 测试框架
type TestFramework struct {
	mu       sync.RWMutex
	config   *TestConfig
	suites   map[string]*TestSuite
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *TestStats
	reporter TestReporter
	hooks    *TestHooks
}

// TestConfig 测试配置
type TestConfig struct {
	// 基础配置
	Name            string        `json:"name"`
	Timeout         time.Duration `json:"timeout"`
	Parallel        bool          `json:"parallel"`
	MaxConcurrency  int           `json:"max_concurrency"`
	RetryCount      int           `json:"retry_count"`
	RetryDelay      time.Duration `json:"retry_delay"`

	// 过滤配置
	IncludeTypes    []TestType `json:"include_types"`
	ExcludeTypes    []TestType `json:"exclude_types"`
	IncludePatterns []string   `json:"include_patterns"`
	ExcludePatterns []string   `json:"exclude_patterns"`
	IncludeTags     []string   `json:"include_tags"`
	ExcludeTags     []string   `json:"exclude_tags"`

	// 报告配置
	ReportFormat    string `json:"report_format"`
	ReportOutput    string `json:"report_output"`
	Verbose         bool   `json:"verbose"`
	ShowProgress    bool   `json:"show_progress"`

	// 负载测试配置
	LoadTestConfig  *LoadTestConfig  `json:"load_test_config"`
	ChaosTestConfig *ChaosTestConfig `json:"chaos_test_config"`
}

// LoadTestConfig 负载测试配置
type LoadTestConfig struct {
	Concurrency     int           `json:"concurrency"`
	Duration        time.Duration `json:"duration"`
	RampUpTime      time.Duration `json:"ramp_up_time"`
	RampDownTime    time.Duration `json:"ramp_down_time"`
	RequestsPerSec  int           `json:"requests_per_sec"`
	MaxRequests     int64         `json:"max_requests"`
	ThinkTime       time.Duration `json:"think_time"`
	Thresholds      *Thresholds   `json:"thresholds"`
}

// ChaosTestConfig 混沌测试配置
type ChaosTestConfig struct {
	Enabled         bool                   `json:"enabled"`
	FailureRate     float64                `json:"failure_rate"`
	LatencyInjection *LatencyInjection     `json:"latency_injection"`
	ErrorInjection  *ErrorInjection        `json:"error_injection"`
	ResourceChaos   *ResourceChaos         `json:"resource_chaos"`
	NetworkChaos    *NetworkChaos          `json:"network_chaos"`
	Experiments     []*ChaosExperiment     `json:"experiments"`
}

// Thresholds 阈值配置
type Thresholds struct {
	MaxResponseTime time.Duration `json:"max_response_time"`
	MaxErrorRate    float64       `json:"max_error_rate"`
	MinThroughput   float64       `json:"min_throughput"`
	MaxCPUUsage     float64       `json:"max_cpu_usage"`
	MaxMemoryUsage  float64       `json:"max_memory_usage"`
}

// LatencyInjection 延迟注入
type LatencyInjection struct {
	Enabled     bool          `json:"enabled"`
	Probability float64       `json:"probability"`
	MinDelay    time.Duration `json:"min_delay"`
	MaxDelay    time.Duration `json:"max_delay"`
}

// ErrorInjection 错误注入
type ErrorInjection struct {
	Enabled     bool     `json:"enabled"`
	Probability float64  `json:"probability"`
	ErrorTypes  []string `json:"error_types"`
	StatusCodes []int    `json:"status_codes"`
}

// ResourceChaos 资源混沌
type ResourceChaos struct {
	Enabled        bool    `json:"enabled"`
	CPUStress      bool    `json:"cpu_stress"`
	MemoryStress   bool    `json:"memory_stress"`
	DiskStress     bool    `json:"disk_stress"`
	StressDuration time.Duration `json:"stress_duration"`
	StressLevel    float64 `json:"stress_level"`
}

// NetworkChaos 网络混沌
type NetworkChaos struct {
	Enabled       bool          `json:"enabled"`
	PacketLoss    float64       `json:"packet_loss"`
	Latency       time.Duration `json:"latency"`
	Jitter        time.Duration `json:"jitter"`
	BandwidthLimit int64        `json:"bandwidth_limit"`
	Corruption    float64       `json:"corruption"`
}

// ChaosExperiment 混沌实验
type ChaosExperiment struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Duration    time.Duration `json:"duration"`
	Actions     []ChaosAction `json:"actions"`
	Schedule    string        `json:"schedule"`
}

// ChaosAction 混沌动作
type ChaosAction struct {
	Type       string                 `json:"type"`
	Target     string                 `json:"target"`
	Parameters map[string]interface{} `json:"parameters"`
	Delay      time.Duration          `json:"delay"`
}

// TestSuite 测试套件
type TestSuite struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        TestType    `json:"type"`
	Tags        []string    `json:"tags"`
	Tests       []*TestCase `json:"tests"`
	Setup       func() error `json:"-"`
	Teardown    func() error `json:"-"`
	Timeout     time.Duration `json:"timeout"`
	Parallel    bool        `json:"parallel"`
	Skip        bool        `json:"skip"`
	SkipReason  string      `json:"skip_reason"`
}

// TestCase 测试用例
type TestCase struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        TestType      `json:"type"`
	Tags        []string      `json:"tags"`
	Timeout     time.Duration `json:"timeout"`
	RetryCount  int           `json:"retry_count"`
	Skip        bool          `json:"skip"`
	SkipReason  string        `json:"skip_reason"`
	Setup       func() error  `json:"-"`
	Test        func() error  `json:"-"`
	Teardown    func() error  `json:"-"`
	Expected    interface{}   `json:"expected"`
	Actual      interface{}   `json:"actual"`
	Status      TestStatus    `json:"status"`
	Error       string        `json:"error,omitempty"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Retries     int           `json:"retries"`
}

// TestStats 测试统计
type TestStats struct {
	TotalTests    int64         `json:"total_tests"`
	PassedTests   int64         `json:"passed_tests"`
	FailedTests   int64         `json:"failed_tests"`
	SkippedTests  int64         `json:"skipped_tests"`
	TimeoutTests  int64         `json:"timeout_tests"`
	TotalDuration time.Duration `json:"total_duration"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	SuccessRate   float64       `json:"success_rate"`
	Coverage      float64       `json:"coverage"`
}

// TestReporter 测试报告器接口
type TestReporter interface {
	ReportStart(stats *TestStats)
	ReportSuite(suite *TestSuite, stats *TestStats)
	ReportTest(test *TestCase)
	ReportEnd(stats *TestStats)
	GenerateReport() ([]byte, error)
}

// TestHooks 测试钩子
type TestHooks struct {
	BeforeAll   func() error
	AfterAll    func() error
	BeforeSuite func(suite *TestSuite) error
	AfterSuite  func(suite *TestSuite) error
	BeforeTest  func(test *TestCase) error
	AfterTest   func(test *TestCase) error
	OnFailure   func(test *TestCase, err error)
	OnSuccess   func(test *TestCase)
}

// DefaultTestConfig 返回默认测试配置
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		Name:           "NetCore-Go Tests",
		Timeout:        30 * time.Minute,
		Parallel:       true,
		MaxConcurrency: 10,
		RetryCount:     0,
		RetryDelay:     1 * time.Second,
		ReportFormat:   "json",
		ReportOutput:   "test-results.json",
		Verbose:        false,
		ShowProgress:   true,
		LoadTestConfig: &LoadTestConfig{
			Concurrency:    10,
			Duration:       1 * time.Minute,
			RampUpTime:     10 * time.Second,
			RampDownTime:   10 * time.Second,
			RequestsPerSec: 100,
			Thresholds: &Thresholds{
				MaxResponseTime: 1 * time.Second,
				MaxErrorRate:    0.01,
				MinThroughput:   50,
				MaxCPUUsage:     0.8,
				MaxMemoryUsage:  0.8,
			},
		},
		ChaosTestConfig: &ChaosTestConfig{
			Enabled:     false,
			FailureRate: 0.1,
			LatencyInjection: &LatencyInjection{
				Enabled:     false,
				Probability: 0.1,
				MinDelay:    100 * time.Millisecond,
				MaxDelay:    1 * time.Second,
			},
			ErrorInjection: &ErrorInjection{
				Enabled:     false,
				Probability: 0.05,
				ErrorTypes:  []string{"timeout", "connection_error", "server_error"},
				StatusCodes: []int{500, 502, 503, 504},
			},
			ResourceChaos: &ResourceChaos{
				Enabled:        false,
				CPUStress:      false,
				MemoryStress:   false,
				DiskStress:     false,
				StressDuration: 30 * time.Second,
				StressLevel:    0.7,
			},
			NetworkChaos: &NetworkChaos{
				Enabled:        false,
				PacketLoss:     0.01,
				Latency:        100 * time.Millisecond,
				Jitter:         50 * time.Millisecond,
				BandwidthLimit: 1024 * 1024, // 1MB/s
				Corruption:     0.001,
			},
		},
	}
}

// NewTestFramework 创建测试框架
func NewTestFramework(config *TestConfig) *TestFramework {
	if config == nil {
		config = DefaultTestConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TestFramework{
		config:   config,
		suites:   make(map[string]*TestSuite),
		ctx:      ctx,
		cancel:   cancel,
		stats:    &TestStats{},
		reporter: NewJSONReporter(config.ReportOutput),
		hooks:    &TestHooks{},
	}
}

// AddSuite 添加测试套件
func (tf *TestFramework) AddSuite(suite *TestSuite) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.suites[suite.Name] = suite
}

// RemoveSuite 移除测试套件
func (tf *TestFramework) RemoveSuite(name string) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	delete(tf.suites, name)
}

// SetReporter 设置报告器
func (tf *TestFramework) SetReporter(reporter TestReporter) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.reporter = reporter
}

// SetHooks 设置钩子
func (tf *TestFramework) SetHooks(hooks *TestHooks) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.hooks = hooks
}

// Run 运行测试
func (tf *TestFramework) Run() error {
	tf.mu.Lock()
	if tf.running {
		tf.mu.Unlock()
		return fmt.Errorf("test framework is already running")
	}
	tf.running = true
	tf.mu.Unlock()

	defer func() {
		tf.mu.Lock()
		tf.running = false
		tf.mu.Unlock()
	}()

	// 初始化统计
	tf.stats.StartTime = time.Now()

	// 执行BeforeAll钩子
	if tf.hooks.BeforeAll != nil {
		if err := tf.hooks.BeforeAll(); err != nil {
			return fmt.Errorf("BeforeAll hook failed: %w", err)
		}
	}

	// 开始报告
	tf.reporter.ReportStart(tf.stats)

	// 运行测试套件
	err := tf.runSuites()

	// 完成统计
	tf.stats.EndTime = time.Now()
	tf.stats.TotalDuration = tf.stats.EndTime.Sub(tf.stats.StartTime)
	if tf.stats.TotalTests > 0 {
		tf.stats.SuccessRate = float64(tf.stats.PassedTests) / float64(tf.stats.TotalTests)
	}

	// 执行AfterAll钩子
	if tf.hooks.AfterAll != nil {
		if hookErr := tf.hooks.AfterAll(); hookErr != nil {
			fmt.Printf("AfterAll hook failed: %v\n", hookErr)
		}
	}

	// 结束报告
	tf.reporter.ReportEnd(tf.stats)

	return err
}

// runSuites 运行测试套件
func (tf *TestFramework) runSuites() error {
	tf.mu.RLock()
	suites := make([]*TestSuite, 0, len(tf.suites))
	for _, suite := range tf.suites {
		if tf.shouldRunSuite(suite) {
			suites = append(suites, suite)
		}
	}
	tf.mu.RUnlock()

	if tf.config.Parallel {
		return tf.runSuitesParallel(suites)
	} else {
		return tf.runSuitesSequential(suites)
	}
}

// runSuitesSequential 顺序运行测试套件
func (tf *TestFramework) runSuitesSequential(suites []*TestSuite) error {
	for _, suite := range suites {
		if err := tf.runSuite(suite); err != nil {
			return err
		}
	}
	return nil
}

// runSuitesParallel 并行运行测试套件
func (tf *TestFramework) runSuitesParallel(suites []*TestSuite) error {
	semaphore := make(chan struct{}, tf.config.MaxConcurrency)
	errorChan := make(chan error, len(suites))
	var wg sync.WaitGroup

	for _, suite := range suites {
		wg.Add(1)
		go func(s *TestSuite) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := tf.runSuite(s); err != nil {
				errorChan <- err
			}
		}(suite)
	}

	wg.Wait()
	close(errorChan)

	// 收集错误
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("test suite errors: %v", errors)
	}

	return nil
}

// runSuite 运行测试套件
func (tf *TestFramework) runSuite(suite *TestSuite) error {
	if suite.Skip {
		fmt.Printf("Skipping suite %s: %s\n", suite.Name, suite.SkipReason)
		return nil
	}

	// 执行BeforeSuite钩子
	if tf.hooks.BeforeSuite != nil {
		if err := tf.hooks.BeforeSuite(suite); err != nil {
			return fmt.Errorf("BeforeSuite hook failed for %s: %w", suite.Name, err)
		}
	}

	// 执行套件Setup
	if suite.Setup != nil {
		if err := suite.Setup(); err != nil {
			return fmt.Errorf("suite setup failed for %s: %w", suite.Name, err)
		}
	}

	// 运行测试用例
	var err error
	if suite.Parallel {
		err = tf.runTestsParallel(suite.Tests)
	} else {
		err = tf.runTestsSequential(suite.Tests)
	}

	// 执行套件Teardown
	if suite.Teardown != nil {
		if teardownErr := suite.Teardown(); teardownErr != nil {
			fmt.Printf("Suite teardown failed for %s: %v\n", suite.Name, teardownErr)
		}
	}

	// 执行AfterSuite钩子
	if tf.hooks.AfterSuite != nil {
		if hookErr := tf.hooks.AfterSuite(suite); hookErr != nil {
			fmt.Printf("AfterSuite hook failed for %s: %v\n", suite.Name, hookErr)
		}
	}

	// 报告套件结果
	tf.reporter.ReportSuite(suite, tf.stats)

	return err
}

// runTestsSequential 顺序运行测试用例
func (tf *TestFramework) runTestsSequential(tests []*TestCase) error {
	for _, test := range tests {
		if tf.shouldRunTest(test) {
			tf.runTest(test)
		}
	}
	return nil
}

// runTestsParallel 并行运行测试用例
func (tf *TestFramework) runTestsParallel(tests []*TestCase) error {
	semaphore := make(chan struct{}, tf.config.MaxConcurrency)
	var wg sync.WaitGroup

	for _, test := range tests {
		if tf.shouldRunTest(test) {
			wg.Add(1)
			go func(t *TestCase) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				tf.runTest(t)
			}(test)
		}
	}

	wg.Wait()
	return nil
}

// runTest 运行单个测试用例
func (tf *TestFramework) runTest(test *TestCase) {
	if test.Skip {
		test.Status = TestStatusSkipped
		atomic.AddInt64(&tf.stats.SkippedTests, 1)
		tf.reporter.ReportTest(test)
		return
	}

	test.Status = TestStatusRunning
	test.StartTime = time.Now()
	atomic.AddInt64(&tf.stats.TotalTests, 1)

	// 执行BeforeTest钩子
	if tf.hooks.BeforeTest != nil {
		if err := tf.hooks.BeforeTest(test); err != nil {
			test.Status = TestStatusFailed
			test.Error = fmt.Sprintf("BeforeTest hook failed: %v", err)
			tf.finishTest(test)
			return
		}
	}

	// 执行测试Setup
	if test.Setup != nil {
		if err := test.Setup(); err != nil {
			test.Status = TestStatusFailed
			test.Error = fmt.Sprintf("Test setup failed: %v", err)
			tf.finishTest(test)
			return
		}
	}

	// 执行测试（带重试）
	tf.executeTestWithRetry(test)

	// 执行测试Teardown
	if test.Teardown != nil {
		if err := test.Teardown(); err != nil {
			fmt.Printf("Test teardown failed for %s: %v\n", test.Name, err)
		}
	}

	// 执行AfterTest钩子
	if tf.hooks.AfterTest != nil {
		if err := tf.hooks.AfterTest(test); err != nil {
			fmt.Printf("AfterTest hook failed for %s: %v\n", test.Name, err)
		}
	}

	tf.finishTest(test)
}

// executeTestWithRetry 执行测试（带重试）
func (tf *TestFramework) executeTestWithRetry(test *TestCase) {
	maxRetries := test.RetryCount
	if maxRetries == 0 {
		maxRetries = tf.config.RetryCount
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			test.Retries++
			time.Sleep(tf.config.RetryDelay)
		}

		// 创建超时上下文
		timeout := test.Timeout
		if timeout == 0 {
			timeout = tf.config.Timeout
		}

		ctx, cancel := context.WithTimeout(tf.ctx, timeout)
		done := make(chan error, 1)

		// 在goroutine中执行测试
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("test panicked: %v", r)
				}
			}()
			done <- test.Test()
		}()

		// 等待测试完成或超时
		select {
		case err := <-done:
			cancel()
			if err == nil {
				test.Status = TestStatusPassed
				if tf.hooks.OnSuccess != nil {
					tf.hooks.OnSuccess(test)
				}
				return
			} else {
				test.Error = err.Error()
				if attempt == maxRetries {
					test.Status = TestStatusFailed
					if tf.hooks.OnFailure != nil {
						tf.hooks.OnFailure(test, err)
					}
				}
			}
		case <-ctx.Done():
			cancel()
			test.Status = TestStatusTimeout
			test.Error = "test timeout"
			return
		}
	}
}

// finishTest 完成测试
func (tf *TestFramework) finishTest(test *TestCase) {
	test.EndTime = time.Now()
	test.Duration = test.EndTime.Sub(test.StartTime)

	// 更新统计
	switch test.Status {
	case TestStatusPassed:
		atomic.AddInt64(&tf.stats.PassedTests, 1)
	case TestStatusFailed:
		atomic.AddInt64(&tf.stats.FailedTests, 1)
	case TestStatusSkipped:
		atomic.AddInt64(&tf.stats.SkippedTests, 1)
	case TestStatusTimeout:
		atomic.AddInt64(&tf.stats.TimeoutTests, 1)
	}

	// 报告测试结果
	tf.reporter.ReportTest(test)
}

// shouldRunSuite 检查是否应该运行测试套件
func (tf *TestFramework) shouldRunSuite(suite *TestSuite) bool {
	// 检查类型过滤
	if len(tf.config.IncludeTypes) > 0 {
		found := false
		for _, includeType := range tf.config.IncludeTypes {
			if includeType == suite.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, excludeType := range tf.config.ExcludeTypes {
		if excludeType == suite.Type {
			return false
		}
	}

	// 检查标签过滤
	if len(tf.config.IncludeTags) > 0 {
		found := false
		for _, includeTag := range tf.config.IncludeTags {
			for _, tag := range suite.Tags {
				if tag == includeTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, excludeTag := range tf.config.ExcludeTags {
		for _, tag := range suite.Tags {
			if tag == excludeTag {
				return false
			}
		}
	}

	return true
}

// shouldRunTest 检查是否应该运行测试用例
func (tf *TestFramework) shouldRunTest(test *TestCase) bool {
	// 检查类型过滤
	if len(tf.config.IncludeTypes) > 0 {
		found := false
		for _, includeType := range tf.config.IncludeTypes {
			if includeType == test.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, excludeType := range tf.config.ExcludeTypes {
		if excludeType == test.Type {
			return false
		}
	}

	// 检查标签过滤
	if len(tf.config.IncludeTags) > 0 {
		found := false
		for _, includeTag := range tf.config.IncludeTags {
			for _, tag := range test.Tags {
				if tag == includeTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, excludeTag := range tf.config.ExcludeTags {
		for _, tag := range test.Tags {
			if tag == excludeTag {
				return false
			}
		}
	}

	return true
}

// GetStats 获取统计信息
func (tf *TestFramework) GetStats() *TestStats {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	stats := *tf.stats
	return &stats
}

// IsRunning 检查是否运行
func (tf *TestFramework) IsRunning() bool {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.running
}

// Stop 停止测试框架
func (tf *TestFramework) Stop() {
	tf.cancel()
}

// 混沌工程相关方法

// InjectLatency 注入延迟
func (tf *TestFramework) InjectLatency(ctx context.Context) {
	if !tf.config.ChaosTestConfig.Enabled {
		return
	}

	latencyConfig := tf.config.ChaosTestConfig.LatencyInjection
	if !latencyConfig.Enabled {
		return
	}

	if rand.Float64() < latencyConfig.Probability {
		delay := latencyConfig.MinDelay + time.Duration(rand.Int63n(int64(latencyConfig.MaxDelay-latencyConfig.MinDelay)))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	}
}

// InjectError 注入错误
func (tf *TestFramework) InjectError() error {
	if !tf.config.ChaosTestConfig.Enabled {
		return nil
	}

	errorConfig := tf.config.ChaosTestConfig.ErrorInjection
	if !errorConfig.Enabled {
		return nil
	}

	if rand.Float64() < errorConfig.Probability {
		errorTypes := errorConfig.ErrorTypes
		if len(errorTypes) > 0 {
			errorType := errorTypes[rand.Intn(len(errorTypes))]
			return fmt.Errorf("injected error: %s", errorType)
		}
	}

	return nil
}

// ChaosHTTPMiddleware 混沌HTTP中间件
func (tf *TestFramework) ChaosHTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 注入延迟
			tf.InjectLatency(r.Context())

			// 注入错误
			if err := tf.InjectError(); err != nil {
				errorConfig := tf.config.ChaosTestConfig.ErrorInjection
				if len(errorConfig.StatusCodes) > 0 {
					statusCode := errorConfig.StatusCodes[rand.Intn(len(errorConfig.StatusCodes))]
					http.Error(w, err.Error(), statusCode)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}