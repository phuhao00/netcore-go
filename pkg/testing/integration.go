// Package testing 集成测试工具
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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// IntegrationTestSuite 集成测试套件
type IntegrationTestSuite struct {
	Name        string
	Description string
	BaseURL     string
	Client      *http.Client
	Server      *httptest.Server
	App         *core.App
	Setup       func() error
	Teardown    func() error
	Tests       []*IntegrationTest
	Context     context.Context
	Cancel      context.CancelFunc
	mu          sync.RWMutex
}

// IntegrationTest 集成测试
type IntegrationTest struct {
	Name        string
	Description string
	Steps       []*TestStep
	Timeout     time.Duration
	RetryCount  int
	Setup       func() error
	Teardown    func() error
	Validation  func(*TestResult) error
}

// TestStep 测试步骤
type TestStep struct {
	Name        string
	Description string
	Action      StepAction
	Request     *HTTPRequest
	Response    *HTTPResponse
	Delay       time.Duration
	Validation  func(*StepResult) error
	ContinueOnError bool
}

// StepAction 步骤动作
type StepAction string

const (
	ActionHTTPRequest StepAction = "http_request"
	ActionDelay       StepAction = "delay"
	ActionValidation  StepAction = "validation"
	ActionSetup       StepAction = "setup"
	ActionTeardown    StepAction = "teardown"
	ActionCustom      StepAction = "custom"
)

// HTTPRequest HTTP请求
type HTTPRequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"query_params"`
	Body        interface{}       `json:"body"`
	Timeout     time.Duration     `json:"timeout"`
	FollowRedirects bool          `json:"follow_redirects"`
}

// HTTPResponse HTTP响应
type HTTPResponse struct {
	StatusCode      int               `json:"status_code"`
	Headers         map[string]string `json:"headers"`
	Body            interface{}       `json:"body"`
	ResponseTime    time.Duration     `json:"response_time"`
	ContentLength   int64             `json:"content_length"`
	ContentType     string            `json:"content_type"`
}

// TestResult 测试结果
type TestResult struct {
	TestName    string        `json:"test_name"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	Steps       []*StepResult `json:"steps"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Retries     int           `json:"retries"`
}

// StepResult 步骤结果
type StepResult struct {
	StepName     string        `json:"step_name"`
	Action       StepAction    `json:"action"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	Request      *HTTPRequest  `json:"request,omitempty"`
	Response     *HTTPResponse `json:"response,omitempty"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	ValidationErrors []string  `json:"validation_errors,omitempty"`
}

// E2ETestRunner 端到端测试运行器
type E2ETestRunner struct {
	Suites      []*IntegrationTestSuite
	Config      *E2EConfig
	Results     []*SuiteResult
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// E2EConfig 端到端测试配置
type E2EConfig struct {
	BaseURL         string        `json:"base_url"`
	Timeout         time.Duration `json:"timeout"`
	RetryCount      int           `json:"retry_count"`
	RetryDelay      time.Duration `json:"retry_delay"`
	Parallel        bool          `json:"parallel"`
	MaxConcurrency  int           `json:"max_concurrency"`
	ReportFormat    string        `json:"report_format"`
	ReportOutput    string        `json:"report_output"`
	ScreenshotDir   string        `json:"screenshot_dir"`
	VideoRecording  bool          `json:"video_recording"`
	BrowserConfig   *BrowserConfig `json:"browser_config,omitempty"`
}

// BrowserConfig 浏览器配置
type BrowserConfig struct {
	Browser     string            `json:"browser"`
	Headless    bool              `json:"headless"`
	WindowSize  string            `json:"window_size"`
	UserAgent   string            `json:"user_agent"`
	Options     map[string]interface{} `json:"options"`
}

// SuiteResult 套件结果
type SuiteResult struct {
	SuiteName   string        `json:"suite_name"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	Tests       []*TestResult `json:"tests"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Stats       *TestStats    `json:"stats"`
}

// NewIntegrationTestSuite 创建集成测试套件
func NewIntegrationTestSuite(name, description string) *IntegrationTestSuite {
	ctx, cancel := context.WithCancel(context.Background())
	return &IntegrationTestSuite{
		Name:        name,
		Description: description,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Tests:   make([]*IntegrationTest, 0),
		Context: ctx,
		Cancel:  cancel,
	}
}

// WithApp 设置应用
func (its *IntegrationTestSuite) WithApp(app *core.App) *IntegrationTestSuite {
	its.App = app
	return its
}

// WithServer 设置测试服务器
func (its *IntegrationTestSuite) WithServer(server *httptest.Server) *IntegrationTestSuite {
	its.Server = server
	its.BaseURL = server.URL
	return its
}

// WithBaseURL 设置基础URL
func (its *IntegrationTestSuite) WithBaseURL(baseURL string) *IntegrationTestSuite {
	its.BaseURL = baseURL
	return its
}

// WithClient 设置HTTP客户端
func (its *IntegrationTestSuite) WithClient(client *http.Client) *IntegrationTestSuite {
	its.Client = client
	return its
}

// AddTest 添加测试
func (its *IntegrationTestSuite) AddTest(test *IntegrationTest) {
	its.mu.Lock()
	defer its.mu.Unlock()
	its.Tests = append(its.Tests, test)
}

// Run 运行测试套件
func (its *IntegrationTestSuite) Run() (*SuiteResult, error) {
	result := &SuiteResult{
		SuiteName: its.Name,
		StartTime: time.Now(),
		Tests:     make([]*TestResult, 0),
		Stats:     &TestStats{},
	}

	// 执行Setup
	if its.Setup != nil {
		if err := its.Setup(); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Suite setup failed: %v", err)
			return result, err
		}
	}

	// 运行测试
	for _, test := range its.Tests {
		testResult := its.runTest(test)
		result.Tests = append(result.Tests, testResult)
		result.Stats.TotalTests++
		if testResult.Success {
			result.Stats.PassedTests++
		} else {
			result.Stats.FailedTests++
		}
	}

	// 执行Teardown
	if its.Teardown != nil {
		if err := its.Teardown(); err != nil {
			fmt.Printf("Suite teardown failed: %v\n", err)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = result.Stats.FailedTests == 0
	if result.Stats.TotalTests > 0 {
		result.Stats.SuccessRate = float64(result.Stats.PassedTests) / float64(result.Stats.TotalTests)
	}

	return result, nil
}

// runTest 运行单个测试
func (its *IntegrationTestSuite) runTest(test *IntegrationTest) *TestResult {
	result := &TestResult{
		TestName:  test.Name,
		StartTime: time.Now(),
		Steps:     make([]*StepResult, 0),
	}

	// 执行测试Setup
	if test.Setup != nil {
		if err := test.Setup(); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Test setup failed: %v", err)
			return result
		}
	}

	// 执行测试步骤
	for _, step := range test.Steps {
		stepResult := its.runStep(step)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Success && !step.ContinueOnError {
			result.Success = false
			result.Error = fmt.Sprintf("Step '%s' failed: %s", step.Name, stepResult.Error)
			break
		}
	}

	// 执行测试Teardown
	if test.Teardown != nil {
		if err := test.Teardown(); err != nil {
			fmt.Printf("Test teardown failed: %v\n", err)
		}
	}

	// 执行验证
	if test.Validation != nil {
		if err := test.Validation(result); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Test validation failed: %v", err)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 如果没有明确失败，则认为成功
	if result.Error == "" {
		result.Success = true
	}

	return result
}

// runStep 运行测试步骤
func (its *IntegrationTestSuite) runStep(step *TestStep) *StepResult {
	result := &StepResult{
		StepName:  step.Name,
		Action:    step.Action,
		StartTime: time.Now(),
	}

	// 添加延迟
	if step.Delay > 0 {
		time.Sleep(step.Delay)
	}

	// 执行步骤动作
	switch step.Action {
	case ActionHTTPRequest:
		its.executeHTTPRequest(step, result)
	case ActionDelay:
		time.Sleep(step.Delay)
		result.Success = true
	case ActionValidation:
		if step.Validation != nil {
			if err := step.Validation(result); err != nil {
				result.Success = false
				result.Error = err.Error()
			} else {
				result.Success = true
			}
		} else {
			result.Success = true
		}
	default:
		result.Success = true
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// executeHTTPRequest 执行HTTP请求
func (its *IntegrationTestSuite) executeHTTPRequest(step *TestStep, result *StepResult) {
	req := step.Request
	if req == nil {
		result.Success = false
		result.Error = "HTTP request is nil"
		return
	}

	result.Request = req

	// 构建URL
	requestURL := req.URL
	if !strings.HasPrefix(requestURL, "http") {
		requestURL = its.BaseURL + requestURL
	}

	// 添加查询参数
	if len(req.QueryParams) > 0 {
		u, err := url.Parse(requestURL)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Invalid URL: %v", err)
			return
		}
		q := u.Query()
		for key, value := range req.QueryParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
		requestURL = u.String()
	}

	// 构建请求体
	var body io.Reader
	if req.Body != nil {
		switch v := req.Body.(type) {
		case string:
			body = strings.NewReader(v)
		case []byte:
			body = bytes.NewReader(v)
		default:
			jsonData, err := json.Marshal(req.Body)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to marshal request body: %v", err)
				return
			}
			body = bytes.NewReader(jsonData)
		}
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(its.Context, req.Method, requestURL, body)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create HTTP request: %v", err)
		return
	}

	// 设置请求头
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// 设置默认Content-Type
	if httpReq.Header.Get("Content-Type") == "" && req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// 执行请求
	start := time.Now()
	resp, err := its.Client.Do(httpReq)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("HTTP request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to read response body: %v", err)
		return
	}

	// 构建响应对象
	httpResp := &HTTPResponse{
		StatusCode:    resp.StatusCode,
		Headers:       make(map[string]string),
		ResponseTime:  time.Since(start),
		ContentLength: resp.ContentLength,
		ContentType:   resp.Header.Get("Content-Type"),
	}

	// 复制响应头
	for key, values := range resp.Header {
		if len(values) > 0 {
			httpResp.Headers[key] = values[0]
		}
	}

	// 解析响应体
	if strings.Contains(httpResp.ContentType, "application/json") {
		var jsonBody interface{}
		if err := json.Unmarshal(respBody, &jsonBody); err == nil {
			httpResp.Body = jsonBody
		} else {
			httpResp.Body = string(respBody)
		}
	} else {
		httpResp.Body = string(respBody)
	}

	result.Response = httpResp

	// 验证响应
	if step.Response != nil {
		if step.Response.StatusCode != 0 && httpResp.StatusCode != step.Response.StatusCode {
			result.Success = false
			result.Error = fmt.Sprintf("Expected status code %d, got %d", step.Response.StatusCode, httpResp.StatusCode)
			return
		}
	}

	// 执行自定义验证
	if step.Validation != nil {
		if err := step.Validation(result); err != nil {
			result.Success = false
			result.Error = err.Error()
			result.ValidationErrors = append(result.ValidationErrors, err.Error())
			return
		}
	}

	result.Success = true
}

// NewE2ETestRunner 创建端到端测试运行器
func NewE2ETestRunner(config *E2EConfig) *E2ETestRunner {
	if config == nil {
		config = &E2EConfig{
			Timeout:        30 * time.Minute,
			RetryCount:     0,
			RetryDelay:     1 * time.Second,
			Parallel:       false,
			MaxConcurrency: 5,
			ReportFormat:   "json",
			ReportOutput:   "e2e-results.json",
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &E2ETestRunner{
		Suites:  make([]*IntegrationTestSuite, 0),
		Config:  config,
		Results: make([]*SuiteResult, 0),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// AddSuite 添加测试套件
func (runner *E2ETestRunner) AddSuite(suite *IntegrationTestSuite) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	runner.Suites = append(runner.Suites, suite)
}

// Run 运行所有测试套件
func (runner *E2ETestRunner) Run() error {
	if runner.Config.Parallel {
		return runner.runParallel()
	} else {
		return runner.runSequential()
	}
}

// runSequential 顺序运行
func (runner *E2ETestRunner) runSequential() error {
	for _, suite := range runner.Suites {
		result, err := suite.Run()
		if err != nil {
			fmt.Printf("Suite %s failed: %v\n", suite.Name, err)
		}
		runner.mu.Lock()
		runner.Results = append(runner.Results, result)
		runner.mu.Unlock()
	}
	return nil
}

// runParallel 并行运行
func (runner *E2ETestRunner) runParallel() error {
	semaphore := make(chan struct{}, runner.Config.MaxConcurrency)
	var wg sync.WaitGroup

	for _, suite := range runner.Suites {
		wg.Add(1)
		go func(s *IntegrationTestSuite) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := s.Run()
			if err != nil {
				fmt.Printf("Suite %s failed: %v\n", s.Name, err)
			}
			runner.mu.Lock()
			runner.Results = append(runner.Results, result)
			runner.mu.Unlock()
		}(suite)
	}

	wg.Wait()
	return nil
}

// GetResults 获取测试结果
func (runner *E2ETestRunner) GetResults() []*SuiteResult {
	runner.mu.RLock()
	defer runner.mu.RUnlock()
	results := make([]*SuiteResult, len(runner.Results))
	copy(results, runner.Results)
	return results
}

// Stop 停止测试运行器
func (runner *E2ETestRunner) Stop() {
	runner.cancel()
}

// 辅助函数

// NewHTTPRequest 创建HTTP请求
func NewHTTPRequest(method, url string) *HTTPRequest {
	return &HTTPRequest{
		Method:          method,
		URL:             url,
		Headers:         make(map[string]string),
		QueryParams:     make(map[string]string),
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	}
}

// WithHeader 添加请求头
func (req *HTTPRequest) WithHeader(key, value string) *HTTPRequest {
	req.Headers[key] = value
	return req
}

// WithQueryParam 添加查询参数
func (req *HTTPRequest) WithQueryParam(key, value string) *HTTPRequest {
	req.QueryParams[key] = value
	return req
}

// WithBody 设置请求体
func (req *HTTPRequest) WithBody(body interface{}) *HTTPRequest {
	req.Body = body
	return req
}

// WithTimeout 设置超时
func (req *HTTPRequest) WithTimeout(timeout time.Duration) *HTTPRequest {
	req.Timeout = timeout
	return req
}

// NewTestStep 创建测试步骤
func NewTestStep(name string, action StepAction) *TestStep {
	return &TestStep{
		Name:   name,
		Action: action,
	}
}

// WithRequest 设置请求
func (step *TestStep) WithRequest(request *HTTPRequest) *TestStep {
	step.Request = request
	return step
}

// WithExpectedResponse 设置期望响应
func (step *TestStep) WithExpectedResponse(response *HTTPResponse) *TestStep {
	step.Response = response
	return step
}

// WithDelay 设置延迟
func (step *TestStep) WithDelay(delay time.Duration) *TestStep {
	step.Delay = delay
	return step
}

// WithValidation 设置验证函数
func (step *TestStep) WithValidation(validation func(*StepResult) error) *TestStep {
	step.Validation = validation
	return step
}

// ContinueOnError 设置错误时继续
func (step *TestStep) ContinueOnError() *TestStep {
	step.ContinueOnError = true
	return step
}