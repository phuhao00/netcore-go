// Package testing 负载测试工具
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
)

// LoadTestRunner 负载测试运行器
type LoadTestRunner struct {
	config    *LoadTestConfig
	scenarios []*LoadTestScenario
	results   *LoadTestResults
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	running   bool
}

// LoadTestScenario 负载测试场景
type LoadTestScenario struct {
	Name        string
	Description string
	Weight      float64 // 场景权重 (0-1)
	Requests    []*LoadTestRequest
	Setup       func() error
	Teardown    func() error
	ThinkTime   time.Duration
	Validation  func(*LoadTestResponse) error
}

// LoadTestRequest 负载测试请求
type LoadTestRequest struct {
	Name        string
	Method      string
	URL         string
	Headers     map[string]string
	Body        interface{}
	Timeout     time.Duration
	Weight      float64 // 请求权重
	Validation  func(*LoadTestResponse) error
}

// LoadTestResponse 负载测试响应
type LoadTestResponse struct {
	RequestName   string
	StatusCode    int
	ResponseTime  time.Duration
	ContentLength int64
	Error         error
	Timestamp     time.Time
	Headers       map[string]string
	Body          []byte
}

// LoadTestResults 负载测试结果
type LoadTestResults struct {
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	TotalRequests int64
	SuccessRequests int64
	FailedRequests int64
	TotalBytes    int64
	MinResponseTime time.Duration
	MaxResponseTime time.Duration
	AvgResponseTime time.Duration
	P50ResponseTime time.Duration
	P90ResponseTime time.Duration
	P95ResponseTime time.Duration
	P99ResponseTime time.Duration
	Throughput    float64 // requests per second
	Bandwidth     float64 // bytes per second
	ErrorRate     float64
	StatusCodes   map[int]int64
	Errors        map[string]int64
	ResponseTimes []time.Duration
	TimeSeries    []*TimeSeriesPoint
	ScenarioStats map[string]*ScenarioStats
	mu            sync.RWMutex
}

// TimeSeriesPoint 时间序列点
type TimeSeriesPoint struct {
	Timestamp     time.Time
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	AvgResponseTime time.Duration
	Throughput    float64
	ErrorRate     float64
}

// ScenarioStats 场景统计
type ScenarioStats struct {
	Name          string
	TotalRequests int64
	SuccessRequests int64
	FailedRequests int64
	AvgResponseTime time.Duration
	MinResponseTime time.Duration
	MaxResponseTime time.Duration
	Throughput    float64
	ErrorRate     float64
}

// StressTestConfig 压力测试配置
type StressTestConfig struct {
	StartConcurrency int
	MaxConcurrency   int
	StepSize         int
	StepDuration     time.Duration
	MaxDuration      time.Duration
	FailureThreshold float64 // 失败率阈值
	ResponseTimeThreshold time.Duration // 响应时间阈值
}

// SpikeTestConfig 峰值测试配置
type SpikeTestConfig struct {
	BaseConcurrency  int
	SpikeConcurrency int
	SpikeDuration    time.Duration
	SpikeInterval    time.Duration
	TotalDuration    time.Duration
}

// VolumeTestConfig 容量测试配置
type VolumeTestConfig struct {
	DataSize      int64 // 数据量大小
	Concurrency   int
	Duration      time.Duration
	DataGenerator func() interface{}
}

// NewLoadTestRunner 创建负载测试运行器
func NewLoadTestRunner(config *LoadTestConfig) *LoadTestRunner {
	if config == nil {
		config = &LoadTestConfig{
			Concurrency: 10,
			Duration:    1 * time.Minute,
			RampUp:      10 * time.Second,
			TargetURL:   "http://localhost:8080",
			RequestRate: 100,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &LoadTestRunner{
		config:    config,
		scenarios: make([]*LoadTestScenario, 0),
		results: &LoadTestResults{
			StatusCodes:   make(map[int]int64),
			Errors:        make(map[string]int64),
			ResponseTimes: make([]time.Duration, 0),
			TimeSeries:    make([]*TimeSeriesPoint, 0),
			ScenarioStats: make(map[string]*ScenarioStats),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddScenario 添加测试场景
func (ltr *LoadTestRunner) AddScenario(scenario *LoadTestScenario) {
	ltr.mu.Lock()
	defer ltr.mu.Unlock()
	ltr.scenarios = append(ltr.scenarios, scenario)
}

// Run 运行负载测试
func (ltr *LoadTestRunner) Run() error {
	ltr.mu.Lock()
	if ltr.running {
		ltr.mu.Unlock()
		return fmt.Errorf("load test is already running")
	}
	ltr.running = true
	ltr.mu.Unlock()

	defer func() {
		ltr.mu.Lock()
		ltr.running = false
		ltr.mu.Unlock()
	}()

	ltr.results.StartTime = time.Now()

	// 启动时间序列收集
	go ltr.collectTimeSeries()

	// 执行场景Setup
	for _, scenario := range ltr.scenarios {
		if scenario.Setup != nil {
			if err := scenario.Setup(); err != nil {
				return fmt.Errorf("scenario %s setup failed: %w", scenario.Name, err)
			}
		}
	}

	// 运行负载测试
	err := ltr.runLoadTest()

	// 执行场景Teardown
	for _, scenario := range ltr.scenarios {
		if scenario.Teardown != nil {
			if teardownErr := scenario.Teardown(); teardownErr != nil {
				fmt.Printf("Scenario %s teardown failed: %v\n", scenario.Name, teardownErr)
			}
		}
	}

	ltr.results.EndTime = time.Now()
	ltr.results.Duration = ltr.results.EndTime.Sub(ltr.results.StartTime)
	ltr.calculateStatistics()

	return err
}

// runLoadTest 运行负载测试
func (ltr *LoadTestRunner) runLoadTest() error {
	var wg sync.WaitGroup
	concurrencyChan := make(chan struct{}, ltr.config.Concurrency)
	requestChan := make(chan *LoadTestRequest, 1000)

	// 启动工作协程
	for i := 0; i < ltr.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ltr.worker(concurrencyChan, requestChan)
		}()
	}

	// 启动请求生成器
	go ltr.generateRequests(requestChan)

	// 等待测试完成
	testCtx, testCancel := context.WithTimeout(ltr.ctx, ltr.config.Duration)
	defer testCancel()

	<-testCtx.Done()
	close(requestChan)
	wg.Wait()

	return nil
}

// worker 工作协程
func (ltr *LoadTestRunner) worker(concurrencyChan chan struct{}, requestChan <-chan *LoadTestRequest) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for {
		select {
		case req, ok := <-requestChan:
			if !ok {
				return
			}
			concurrencyChan <- struct{}{}
			ltr.executeRequest(client, req)
			<-concurrencyChan
		case <-ltr.ctx.Done():
			return
		}
	}
}

// generateRequests 生成请求
func (ltr *LoadTestRunner) generateRequests(requestChan chan<- *LoadTestRequest) {
	ticker := time.NewTicker(time.Second / time.Duration(ltr.config.RequestRate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 根据权重选择场景
			scenario := ltr.selectScenario()
			if scenario != nil {
				// 根据权重选择请求
				request := ltr.selectRequest(scenario)
				if request != nil {
					select {
					case requestChan <- request:
					case <-ltr.ctx.Done():
						return
					}
				}
			}
		case <-ltr.ctx.Done():
			return
		}
	}
}

// selectScenario 根据权重选择场景
func (ltr *LoadTestRunner) selectScenario() *LoadTestScenario {
	if len(ltr.scenarios) == 0 {
		return nil
	}

	totalWeight := 0.0
	for _, scenario := range ltr.scenarios {
		totalWeight += scenario.Weight
	}

	if totalWeight == 0 {
		return ltr.scenarios[rand.Intn(len(ltr.scenarios))]
	}

	random := rand.Float64() * totalWeight
	currentWeight := 0.0

	for _, scenario := range ltr.scenarios {
		currentWeight += scenario.Weight
		if random <= currentWeight {
			return scenario
		}
	}

	return ltr.scenarios[len(ltr.scenarios)-1]
}

// selectRequest 根据权重选择请求
func (ltr *LoadTestRunner) selectRequest(scenario *LoadTestScenario) *LoadTestRequest {
	if len(scenario.Requests) == 0 {
		return nil
	}

	totalWeight := 0.0
	for _, request := range scenario.Requests {
		totalWeight += request.Weight
	}

	if totalWeight == 0 {
		return scenario.Requests[rand.Intn(len(scenario.Requests))]
	}

	random := rand.Float64() * totalWeight
	currentWeight := 0.0

	for _, request := range scenario.Requests {
		currentWeight += request.Weight
		if random <= currentWeight {
			return request
		}
	}

	return scenario.Requests[len(scenario.Requests)-1]
}

// executeRequest 执行请求
func (ltr *LoadTestRunner) executeRequest(client *http.Client, request *LoadTestRequest) {
	start := time.Now()
	response := &LoadTestResponse{
		RequestName: request.Name,
		Timestamp:   start,
	}

	// 构建HTTP请求
	req, err := http.NewRequestWithContext(ltr.ctx, request.Method, request.URL, nil)
	if err != nil {
		response.Error = err
		ltr.recordResponse(response)
		return
	}

	// 设置请求头
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	// 执行请求
	resp, err := client.Do(req)
	response.ResponseTime = time.Since(start)

	if err != nil {
		response.Error = err
		ltr.recordResponse(response)
		return
	}
	defer resp.Body.Close()

	response.StatusCode = resp.StatusCode
	response.ContentLength = resp.ContentLength
	response.Headers = make(map[string]string)

	// 复制响应头
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	// 执行验证
	if request.Validation != nil {
		if err := request.Validation(response); err != nil {
			response.Error = err
		}
	}

	ltr.recordResponse(response)
}

// recordResponse 记录响应
func (ltr *LoadTestRunner) recordResponse(response *LoadTestResponse) {
	ltr.results.mu.Lock()
	defer ltr.results.mu.Unlock()

	atomic.AddInt64(&ltr.results.TotalRequests, 1)
	atomic.AddInt64(&ltr.results.TotalBytes, response.ContentLength)

	if response.Error != nil {
		atomic.AddInt64(&ltr.results.FailedRequests, 1)
		ltr.results.Errors[response.Error.Error()]++
	} else {
		atomic.AddInt64(&ltr.results.SuccessRequests, 1)
	}

	ltr.results.StatusCodes[response.StatusCode]++
	ltr.results.ResponseTimes = append(ltr.results.ResponseTimes, response.ResponseTime)

	// 更新最小/最大响应时间
	if ltr.results.MinResponseTime == 0 || response.ResponseTime < ltr.results.MinResponseTime {
		ltr.results.MinResponseTime = response.ResponseTime
	}
	if response.ResponseTime > ltr.results.MaxResponseTime {
		ltr.results.MaxResponseTime = response.ResponseTime
	}
}

// collectTimeSeries 收集时间序列数据
func (ltr *LoadTestRunner) collectTimeSeries() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastRequestCount, lastSuccessCount, lastFailureCount int64

	for {
		select {
		case <-ticker.C:
			currentRequestCount := atomic.LoadInt64(&ltr.results.TotalRequests)
			currentSuccessCount := atomic.LoadInt64(&ltr.results.SuccessRequests)
			currentFailureCount := atomic.LoadInt64(&ltr.results.FailedRequests)

			requestDelta := currentRequestCount - lastRequestCount
			successDelta := currentSuccessCount - lastSuccessCount
			failureDelta := currentFailureCount - lastFailureCount

			point := &TimeSeriesPoint{
				Timestamp:    time.Now(),
				RequestCount: requestDelta,
				SuccessCount: successDelta,
				FailureCount: failureDelta,
				Throughput:   float64(requestDelta),
			}

			if requestDelta > 0 {
				point.ErrorRate = float64(failureDelta) / float64(requestDelta)
			}

			ltr.results.mu.Lock()
			ltr.results.TimeSeries = append(ltr.results.TimeSeries, point)
			ltr.results.mu.Unlock()

			lastRequestCount = currentRequestCount
			lastSuccessCount = currentSuccessCount
			lastFailureCount = currentFailureCount

		case <-ltr.ctx.Done():
			return
		}
	}
}

// calculateStatistics 计算统计信息
func (ltr *LoadTestRunner) calculateStatistics() {
	ltr.results.mu.Lock()
	defer ltr.results.mu.Unlock()

	if ltr.results.TotalRequests == 0 {
		return
	}

	// 计算吞吐量
	if ltr.results.Duration > 0 {
		ltr.results.Throughput = float64(ltr.results.TotalRequests) / ltr.results.Duration.Seconds()
		ltr.results.Bandwidth = float64(ltr.results.TotalBytes) / ltr.results.Duration.Seconds()
	}

	// 计算错误率
	ltr.results.ErrorRate = float64(ltr.results.FailedRequests) / float64(ltr.results.TotalRequests)

	// 计算响应时间统计
	if len(ltr.results.ResponseTimes) > 0 {
		ltr.calculateResponseTimePercentiles()
	}
}

// calculateResponseTimePercentiles 计算响应时间百分位数
func (ltr *LoadTestRunner) calculateResponseTimePercentiles() {
	responseTimes := make([]time.Duration, len(ltr.results.ResponseTimes))
	copy(responseTimes, ltr.results.ResponseTimes)

	// 简单排序（实际应用中可能需要更高效的算法）
	for i := 0; i < len(responseTimes); i++ {
		for j := i + 1; j < len(responseTimes); j++ {
			if responseTimes[i] > responseTimes[j] {
				responseTimes[i], responseTimes[j] = responseTimes[j], responseTimes[i]
			}
		}
	}

	n := len(responseTimes)
	ltr.results.P50ResponseTime = responseTimes[int(float64(n)*0.5)]
	ltr.results.P90ResponseTime = responseTimes[int(float64(n)*0.9)]
	ltr.results.P95ResponseTime = responseTimes[int(float64(n)*0.95)]
	ltr.results.P99ResponseTime = responseTimes[int(float64(n)*0.99)]

	// 计算平均响应时间
	var total time.Duration
	for _, rt := range responseTimes {
		total += rt
	}
	ltr.results.AvgResponseTime = total / time.Duration(n)
}

// GetResults 获取测试结果
func (ltr *LoadTestRunner) GetResults() *LoadTestResults {
	ltr.results.mu.RLock()
	defer ltr.results.mu.RUnlock()
	return ltr.results
}

// IsRunning 检查是否运行中
func (ltr *LoadTestRunner) IsRunning() bool {
	ltr.mu.RLock()
	defer ltr.mu.RUnlock()
	return ltr.running
}

// Stop 停止负载测试
func (ltr *LoadTestRunner) Stop() {
	ltr.cancel()
}

// RunStressTest 运行压力测试
func (ltr *LoadTestRunner) RunStressTest(config *StressTestConfig) error {
	if config == nil {
		return fmt.Errorf("stress test config is required")
	}

	currentConcurrency := config.StartConcurrency
	startTime := time.Now()

	for currentConcurrency <= config.MaxConcurrency {
		// 检查是否超过最大持续时间
		if time.Since(startTime) > config.MaxDuration {
			break
		}

		fmt.Printf("Running stress test with concurrency: %d\n", currentConcurrency)

		// 更新并发数
		ltr.config.Concurrency = currentConcurrency

		// 运行测试
		testCtx, testCancel := context.WithTimeout(ltr.ctx, config.StepDuration)
		ltr.ctx = testCtx

		err := ltr.runLoadTest()
		testCancel()

		if err != nil {
			return fmt.Errorf("stress test failed at concurrency %d: %w", currentConcurrency, err)
		}

		// 检查失败阈值
		if ltr.results.ErrorRate > config.FailureThreshold {
			fmt.Printf("Stress test failed: error rate %.2f%% exceeds threshold %.2f%%\n",
				ltr.results.ErrorRate*100, config.FailureThreshold*100)
			break
		}

		// 检查响应时间阈值
		if ltr.results.AvgResponseTime > config.ResponseTimeThreshold {
			fmt.Printf("Stress test failed: avg response time %v exceeds threshold %v\n",
				ltr.results.AvgResponseTime, config.ResponseTimeThreshold)
			break
		}

		currentConcurrency += config.StepSize
	}

	return nil
}

// RunSpikeTest 运行峰值测试
func (ltr *LoadTestRunner) RunSpikeTest(config *SpikeTestConfig) error {
	if config == nil {
		return fmt.Errorf("spike test config is required")
	}

	startTime := time.Now()
	nextSpikeTime := startTime.Add(config.SpikeInterval)

	for time.Since(startTime) < config.TotalDuration {
		currentTime := time.Now()

		// 检查是否需要触发峰值
		if currentTime.After(nextSpikeTime) {
			fmt.Printf("Triggering spike: concurrency %d -> %d\n",
				config.BaseConcurrency, config.SpikeConcurrency)

			// 峰值期间
			ltr.config.Concurrency = config.SpikeConcurrency
			spikeCtx, spikeCancel := context.WithTimeout(ltr.ctx, config.SpikeDuration)
			ltr.ctx = spikeCtx
			ltr.runLoadTest()
			spikeCancel()

			// 恢复基础并发
			ltr.config.Concurrency = config.BaseConcurrency
			nextSpikeTime = currentTime.Add(config.SpikeInterval)
		} else {
			// 基础负载期间
			ltr.config.Concurrency = config.BaseConcurrency
			baseCtx, baseCancel := context.WithTimeout(ltr.ctx, time.Second)
			ltr.ctx = baseCtx
			ltr.runLoadTest()
			baseCancel()
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// GenerateReport 生成负载测试报告
func (ltr *LoadTestRunner) GenerateReport() string {
	results := ltr.GetResults()

	report := fmt.Sprintf(`
=== Load Test Report ===
Duration: %v
Total Requests: %d
Successful Requests: %d
Failed Requests: %d
Throughput: %.2f req/s
Bandwidth: %.2f bytes/s
Error Rate: %.2f%%

Response Times:
  Min: %v
  Max: %v
  Avg: %v
  P50: %v
  P90: %v
  P95: %v
  P99: %v

Status Codes:
`,
		results.Duration,
		results.TotalRequests,
		results.SuccessRequests,
		results.FailedRequests,
		results.Throughput,
		results.Bandwidth,
		results.ErrorRate*100,
		results.MinResponseTime,
		results.MaxResponseTime,
		results.AvgResponseTime,
		results.P50ResponseTime,
		results.P90ResponseTime,
		results.P95ResponseTime,
		results.P99ResponseTime,
	)

	for code, count := range results.StatusCodes {
		report += fmt.Sprintf("  %d: %d\n", code, count)
	}

	if len(results.Errors) > 0 {
		report += "\nErrors:\n"
		for err, count := range results.Errors {
			report += fmt.Sprintf("  %s: %d\n", err, count)
		}
	}

	return report
}

// 辅助函数

// NewLoadTestScenario 创建负载测试场景
func NewLoadTestScenario(name, description string) *LoadTestScenario {
	return &LoadTestScenario{
		Name:        name,
		Description: description,
		Weight:      1.0,
		Requests:    make([]*LoadTestRequest, 0),
		ThinkTime:   0,
	}
}

// AddRequest 添加请求
func (scenario *LoadTestScenario) AddRequest(request *LoadTestRequest) {
	scenario.Requests = append(scenario.Requests, request)
}

// NewLoadTestRequest 创建负载测试请求
func NewLoadTestRequest(name, method, url string) *LoadTestRequest {
	return &LoadTestRequest{
		Name:    name,
		Method:  method,
		URL:     url,
		Headers: make(map[string]string),
		Timeout: 30 * time.Second,
		Weight:  1.0,
	}
}

// WithHeader 添加请求头
func (req *LoadTestRequest) WithHeader(key, value string) *LoadTestRequest {
	req.Headers[key] = value
	return req
}

// WithBody 设置请求体
func (req *LoadTestRequest) WithBody(body interface{}) *LoadTestRequest {
	req.Body = body
	return req
}

// WithWeight 设置权重
func (req *LoadTestRequest) WithWeight(weight float64) *LoadTestRequest {
	req.Weight = weight
	return req
}

// WithValidation 设置验证函数
func (req *LoadTestRequest) WithValidation(validation func(*LoadTestResponse) error) *LoadTestRequest {
	req.Validation = validation
	return req
}