// Package testing 单元测试工具
// Author: NetCore-Go Team
// Created: 2024

package testing

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// UnitTestSuite 单元测试套件
type UnitTestSuite struct {
	Name        string
	Description string
	Tests       []*UnitTest
	Setup       func() error
	Teardown    func() error
	BeforeEach  func() error
	AfterEach   func() error
	Timeout     time.Duration
	Parallel    bool
	Skip        bool
	SkipReason  string
}

// UnitTest 单元测试
type UnitTest struct {
	Name        string
	Description string
	TestFunc    func(*TestContext) error
	Setup       func() error
	Teardown    func() error
	Timeout     time.Duration
	Skip        bool
	SkipReason  string
	Tags        []string
	Expected    interface{}
	Actual      interface{}
	Mocks       map[string]interface{}
	Stubs       map[string]interface{}
}

// TestContext 测试上下文
type TestContext struct {
	T           *testing.T
	Name        string
	Timeout     time.Duration
	Context     context.Context
	Cancel      context.CancelFunc
	Assertions  *Assertions
	Mocks       *MockManager
	Stubs       *StubManager
	Data        map[string]interface{}
	StartTime   time.Time
	CleanupFuncs []func()
}

// Assertions 断言工具
type Assertions struct {
	t *testing.T
}

// MockManager 模拟对象管理器
type MockManager struct {
	mocks map[string]*Mock
}

// Mock 模拟对象
type Mock struct {
	Name        string
	Object      interface{}
	Calls       []*MockCall
	Expectations []*MockExpectation
}

// MockCall 模拟调用
type MockCall struct {
	Method    string
	Args      []interface{}
	Returns   []interface{}
	Timestamp time.Time
}

// MockExpectation 模拟期望
type MockExpectation struct {
	Method      string
	Args        []interface{}
	Returns     []interface{}
	Times       int
	ActualTimes int
	AnyArgs     bool
}

// StubManager 存根管理器
type StubManager struct {
	stubs map[string]*Stub
}

// Stub 存根
type Stub struct {
	Name     string
	Original interface{}
	Replacement interface{}
	Active   bool
}

// TestDataBuilder 测试数据构建器
type TestDataBuilder struct {
	data map[string]interface{}
}

// BenchmarkSuite 基准测试套件
type BenchmarkSuite struct {
	Name        string
	Description string
	Benchmarks  []*Benchmark
	Setup       func() error
	Teardown    func() error
}

// Benchmark 基准测试
type Benchmark struct {
	Name        string
	Description string
	BenchFunc   func(*BenchmarkContext)
	Setup       func() error
	Teardown    func() error
	Iterations  int
	Duration    time.Duration
	MemAllocs   int64
	MemBytes    int64
}

// BenchmarkContext 基准测试上下文
type BenchmarkContext struct {
	B         *testing.B
	Name      string
	N         int
	StartTime time.Time
	Data      map[string]interface{}
}

// NewUnitTestSuite 创建单元测试套件
func NewUnitTestSuite(name, description string) *UnitTestSuite {
	return &UnitTestSuite{
		Name:        name,
		Description: description,
		Tests:       make([]*UnitTest, 0),
		Timeout:     30 * time.Second,
		Parallel:    false,
	}
}

// AddTest 添加测试
func (suite *UnitTestSuite) AddTest(test *UnitTest) {
	suite.Tests = append(suite.Tests, test)
}

// Run 运行测试套件
func (suite *UnitTestSuite) Run(t *testing.T) {
	if suite.Skip {
		t.Skipf("Skipping suite %s: %s", suite.Name, suite.SkipReason)
		return
	}

	t.Run(suite.Name, func(t *testing.T) {
		// 执行套件Setup
		if suite.Setup != nil {
			if err := suite.Setup(); err != nil {
				t.Fatalf("Suite setup failed: %v", err)
			}
		}

		// 执行套件Teardown
		defer func() {
			if suite.Teardown != nil {
				if err := suite.Teardown(); err != nil {
					t.Errorf("Suite teardown failed: %v", err)
				}
			}
		}()

		// 运行测试
		for _, test := range suite.Tests {
			suite.runTest(t, test)
		}
	})
}

// runTest 运行单个测试
func (suite *UnitTestSuite) runTest(t *testing.T, test *UnitTest) {
	if test.Skip {
		t.Run(test.Name, func(t *testing.T) {
			t.Skipf("Skipping test: %s", test.SkipReason)
		})
		return
	}

	t.Run(test.Name, func(t *testing.T) {
		if suite.Parallel {
			t.Parallel()
		}

		// 创建测试上下文
		ctx := suite.createTestContext(t, test)
		defer ctx.cleanup()

		// 执行BeforeEach
		if suite.BeforeEach != nil {
			if err := suite.BeforeEach(); err != nil {
				t.Fatalf("BeforeEach failed: %v", err)
			}
		}

		// 执行测试Setup
		if test.Setup != nil {
			if err := test.Setup(); err != nil {
				t.Fatalf("Test setup failed: %v", err)
			}
		}

		// 执行测试
		if test.TestFunc != nil {
			if err := test.TestFunc(ctx); err != nil {
				t.Errorf("Test failed: %v", err)
			}
		}

		// 执行测试Teardown
		if test.Teardown != nil {
			if err := test.Teardown(); err != nil {
				t.Errorf("Test teardown failed: %v", err)
			}
		}

		// 执行AfterEach
		if suite.AfterEach != nil {
			if err := suite.AfterEach(); err != nil {
				t.Errorf("AfterEach failed: %v", err)
			}
		}
	})
}

// createTestContext 创建测试上下文
func (suite *UnitTestSuite) createTestContext(t *testing.T, test *UnitTest) *TestContext {
	timeout := test.Timeout
	if timeout == 0 {
		timeout = suite.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	testCtx := &TestContext{
		T:           t,
		Name:        test.Name,
		Timeout:     timeout,
		Context:     ctx,
		Cancel:      cancel,
		Assertions:  NewAssertions(t),
		Mocks:       NewMockManager(),
		Stubs:       NewStubManager(),
		Data:        make(map[string]interface{}),
		StartTime:   time.Now(),
		CleanupFuncs: make([]func(), 0),
	}

	return testCtx
}

// cleanup 清理测试上下文
func (ctx *TestContext) cleanup() {
	// 执行清理函数
	for i := len(ctx.CleanupFuncs) - 1; i >= 0; i-- {
		ctx.CleanupFuncs[i]()
	}

	// 清理模拟对象
	ctx.Mocks.CleanupAll()

	// 清理存根
	ctx.Stubs.CleanupAll()

	// 取消上下文
	ctx.Cancel()
}

// AddCleanup 添加清理函数
func (ctx *TestContext) AddCleanup(cleanup func()) {
	ctx.CleanupFuncs = append(ctx.CleanupFuncs, cleanup)
}

// Set 设置测试数据
func (ctx *TestContext) Set(key string, value interface{}) {
	ctx.Data[key] = value
}

// Get 获取测试数据
func (ctx *TestContext) Get(key string) interface{} {
	return ctx.Data[key]
}

// NewAssertions 创建断言工具
func NewAssertions(t *testing.T) *Assertions {
	return &Assertions{t: t}
}

// Equal 断言相等
func (a *Assertions) Equal(expected, actual interface{}, msgAndArgs ...interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		msg := fmt.Sprintf("Expected: %v, Actual: %v", expected, actual)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// NotEqual 断言不相等
func (a *Assertions) NotEqual(expected, actual interface{}, msgAndArgs ...interface{}) {
	if reflect.DeepEqual(expected, actual) {
		msg := fmt.Sprintf("Expected not equal: %v, Actual: %v", expected, actual)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// True 断言为真
func (a *Assertions) True(value bool, msgAndArgs ...interface{}) {
	if !value {
		msg := "Expected true, got false"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// False 断言为假
func (a *Assertions) False(value bool, msgAndArgs ...interface{}) {
	if value {
		msg := "Expected false, got true"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// Nil 断言为nil
func (a *Assertions) Nil(value interface{}, msgAndArgs ...interface{}) {
	if value != nil {
		msg := fmt.Sprintf("Expected nil, got: %v", value)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// NotNil 断言不为nil
func (a *Assertions) NotNil(value interface{}, msgAndArgs ...interface{}) {
	if value == nil {
		msg := "Expected not nil, got nil"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// Error 断言有错误
func (a *Assertions) Error(err error, msgAndArgs ...interface{}) {
	if err == nil {
		msg := "Expected error, got nil"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// NoError 断言无错误
func (a *Assertions) NoError(err error, msgAndArgs ...interface{}) {
	if err != nil {
		msg := fmt.Sprintf("Expected no error, got: %v", err)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		a.t.Error(msg)
	}
}

// Contains 断言包含
func (a *Assertions) Contains(container, item interface{}, msgAndArgs ...interface{}) {
	containerValue := reflect.ValueOf(container)
	itemValue := reflect.ValueOf(item)

	switch containerValue.Kind() {
	case reflect.String:
		if !strings.Contains(containerValue.String(), itemValue.String()) {
			msg := fmt.Sprintf("Expected '%s' to contain '%s'", container, item)
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
			}
			a.t.Error(msg)
		}
	case reflect.Slice, reflect.Array:
		found := false
		for i := 0; i < containerValue.Len(); i++ {
			if reflect.DeepEqual(containerValue.Index(i).Interface(), item) {
				found = true
				break
			}
		}
		if !found {
			msg := fmt.Sprintf("Expected %v to contain %v", container, item)
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
			}
			a.t.Error(msg)
		}
	default:
		a.t.Errorf("Contains assertion not supported for type %T", container)
	}
}

// Panics 断言会panic
func (a *Assertions) Panics(fn func(), msgAndArgs ...interface{}) {
	defer func() {
		if r := recover(); r == nil {
			msg := "Expected function to panic"
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
			}
			a.t.Error(msg)
		}
	}()
	fn()
}

// NotPanics 断言不会panic
func (a *Assertions) NotPanics(fn func(), msgAndArgs ...interface{}) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("Expected function not to panic, but it panicked with: %v", r)
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
			}
			a.t.Error(msg)
		}
	}()
	fn()
}

// NewMockManager 创建模拟对象管理器
func NewMockManager() *MockManager {
	return &MockManager{
		mocks: make(map[string]*Mock),
	}
}

// CreateMock 创建模拟对象
func (mm *MockManager) CreateMock(name string, obj interface{}) *Mock {
	mock := &Mock{
		Name:         name,
		Object:       obj,
		Calls:        make([]*MockCall, 0),
		Expectations: make([]*MockExpectation, 0),
	}
	mm.mocks[name] = mock
	return mock
}

// GetMock 获取模拟对象
func (mm *MockManager) GetMock(name string) *Mock {
	return mm.mocks[name]
}

// CleanupAll 清理所有模拟对象
func (mm *MockManager) CleanupAll() {
	for _, mock := range mm.mocks {
		mock.Calls = nil
		mock.Expectations = nil
	}
	mm.mocks = make(map[string]*Mock)
}

// Expect 设置期望
func (m *Mock) Expect(method string, args ...interface{}) *MockExpectation {
	expectation := &MockExpectation{
		Method:  method,
		Args:    args,
		Times:   1,
		AnyArgs: len(args) == 0,
	}
	m.Expectations = append(m.Expectations, expectation)
	return expectation
}

// Return 设置返回值
func (me *MockExpectation) Return(returns ...interface{}) *MockExpectation {
	me.Returns = returns
	return me
}

// Times 设置调用次数
func (me *MockExpectation) Times(times int) *MockExpectation {
	me.Times = times
	return me
}

// AnyArgs 任意参数
func (me *MockExpectation) AnyArgs() *MockExpectation {
	me.AnyArgs = true
	return me
}

// RecordCall 记录调用
func (m *Mock) RecordCall(method string, args ...interface{}) []interface{} {
	call := &MockCall{
		Method:    method,
		Args:      args,
		Timestamp: time.Now(),
	}

	// 查找匹配的期望
	for _, expectation := range m.Expectations {
		if expectation.Method == method {
			if expectation.AnyArgs || reflect.DeepEqual(expectation.Args, args) {
				expectation.ActualTimes++
				call.Returns = expectation.Returns
				break
			}
		}
	}

	m.Calls = append(m.Calls, call)
	return call.Returns
}

// Verify 验证期望
func (m *Mock) Verify() error {
	for _, expectation := range m.Expectations {
		if expectation.ActualTimes != expectation.Times {
			return fmt.Errorf("method %s expected %d times, called %d times",
				expectation.Method, expectation.Times, expectation.ActualTimes)
		}
	}
	return nil
}

// NewStubManager 创建存根管理器
func NewStubManager() *StubManager {
	return &StubManager{
		stubs: make(map[string]*Stub),
	}
}

// CreateStub 创建存根
func (sm *StubManager) CreateStub(name string, original, replacement interface{}) *Stub {
	stub := &Stub{
		Name:        name,
		Original:    original,
		Replacement: replacement,
		Active:      false,
	}
	sm.stubs[name] = stub
	return stub
}

// Activate 激活存根
func (s *Stub) Activate() {
	s.Active = true
}

// Deactivate 停用存根
func (s *Stub) Deactivate() {
	s.Active = false
}

// CleanupAll 清理所有存根
func (sm *StubManager) CleanupAll() {
	for _, stub := range sm.stubs {
		stub.Deactivate()
	}
	sm.stubs = make(map[string]*Stub)
}

// NewTestDataBuilder 创建测试数据构建器
func NewTestDataBuilder() *TestDataBuilder {
	return &TestDataBuilder{
		data: make(map[string]interface{}),
	}
}

// With 添加数据
func (tdb *TestDataBuilder) With(key string, value interface{}) *TestDataBuilder {
	tdb.data[key] = value
	return tdb
}

// Build 构建数据
func (tdb *TestDataBuilder) Build() map[string]interface{} {
	return tdb.data
}

// NewUnitTest 创建单元测试
func NewUnitTest(name, description string, testFunc func(*TestContext) error) *UnitTest {
	return &UnitTest{
		Name:        name,
		Description: description,
		TestFunc:    testFunc,
		Timeout:     30 * time.Second,
		Tags:        make([]string, 0),
		Mocks:       make(map[string]interface{}),
		Stubs:       make(map[string]interface{}),
	}
}

// WithTimeout 设置超时
func (test *UnitTest) WithTimeout(timeout time.Duration) *UnitTest {
	test.Timeout = timeout
	return test
}

// WithTags 设置标签
func (test *UnitTest) WithTags(tags ...string) *UnitTest {
	test.Tags = append(test.Tags, tags...)
	return test
}

// WithSetup 设置Setup
func (test *UnitTest) WithSetup(setup func() error) *UnitTest {
	test.Setup = setup
	return test
}

// WithTeardown 设置Teardown
func (test *UnitTest) WithTeardown(teardown func() error) *UnitTest {
	test.Teardown = teardown
	return test
}

// Skip 跳过测试
func (test *UnitTest) Skip(reason string) *UnitTest {
	test.Skip = true
	test.SkipReason = reason
	return test
}

// 基准测试相关

// NewBenchmarkSuite 创建基准测试套件
func NewBenchmarkSuite(name, description string) *BenchmarkSuite {
	return &BenchmarkSuite{
		Name:        name,
		Description: description,
		Benchmarks:  make([]*Benchmark, 0),
	}
}

// AddBenchmark 添加基准测试
func (suite *BenchmarkSuite) AddBenchmark(benchmark *Benchmark) {
	suite.Benchmarks = append(suite.Benchmarks, benchmark)
}

// Run 运行基准测试套件
func (suite *BenchmarkSuite) Run(b *testing.B) {
	b.Run(suite.Name, func(b *testing.B) {
		// 执行套件Setup
		if suite.Setup != nil {
			if err := suite.Setup(); err != nil {
				b.Fatalf("Benchmark suite setup failed: %v", err)
			}
		}

		// 执行套件Teardown
		defer func() {
			if suite.Teardown != nil {
				if err := suite.Teardown(); err != nil {
					b.Errorf("Benchmark suite teardown failed: %v", err)
				}
			}
		}()

		// 运行基准测试
		for _, benchmark := range suite.Benchmarks {
			suite.runBenchmark(b, benchmark)
		}
	})
}

// runBenchmark 运行单个基准测试
func (suite *BenchmarkSuite) runBenchmark(b *testing.B, benchmark *Benchmark) {
	b.Run(benchmark.Name, func(b *testing.B) {
		// 执行基准测试Setup
		if benchmark.Setup != nil {
			if err := benchmark.Setup(); err != nil {
				b.Fatalf("Benchmark setup failed: %v", err)
			}
		}

		// 执行基准测试Teardown
		defer func() {
			if benchmark.Teardown != nil {
				if err := benchmark.Teardown(); err != nil {
					b.Errorf("Benchmark teardown failed: %v", err)
				}
			}
		}()

		// 创建基准测试上下文
		ctx := &BenchmarkContext{
			B:         b,
			Name:      benchmark.Name,
			N:         b.N,
			StartTime: time.Now(),
			Data:      make(map[string]interface{}),
		}

		// 重置计时器
		b.ResetTimer()

		// 执行基准测试
		if benchmark.BenchFunc != nil {
			benchmark.BenchFunc(ctx)
		}
	})
}

// NewBenchmark 创建基准测试
func NewBenchmark(name, description string, benchFunc func(*BenchmarkContext)) *Benchmark {
	return &Benchmark{
		Name:        name,
		Description: description,
		BenchFunc:   benchFunc,
	}
}

// 辅助函数

// GetFunctionName 获取函数名
func GetFunctionName(fn interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// IsTestFunction 检查是否为测试函数
func IsTestFunction(name string) bool {
	return strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark")
}

// GenerateTestName 生成测试名称
func GenerateTestName(prefix string, parts ...string) string {
	name := prefix
	for _, part := range parts {
		name += "_" + strings.ReplaceAll(part, " ", "_")
	}
	return name
}