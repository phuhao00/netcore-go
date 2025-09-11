// Package graceful 优雅关闭实现
// Author: NetCore-Go Team
// Created: 2024

package graceful

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/netcore-go/pkg/core"
)

// ShutdownHook 关闭钩子函数类型
type ShutdownHook func(ctx context.Context) error

// ShutdownManager 优雅关闭管理器
type ShutdownManager struct {
	mu           sync.RWMutex
	config       *ShutdownConfig
	hooks        []ShutdownHook
	namedHooks   map[string]ShutdownHook
	running      bool
	shuttingDown bool
	ctx          context.Context
	cancel       context.CancelFunc
	stats        *ShutdownStats
	signalChan   chan os.Signal
}

// ShutdownConfig 关闭配置
type ShutdownConfig struct {
	// 超时配置
	Timeout         time.Duration `json:"timeout"`          // 总超时时间
	GracePeriod     time.Duration `json:"grace_period"`     // 优雅期
	ForceTimeout    time.Duration `json:"force_timeout"`    // 强制超时
	HookTimeout     time.Duration `json:"hook_timeout"`     // 单个钩子超时

	// 信号配置
	Signals         []os.Signal   `json:"-"`                // 监听的信号
	IgnoreSignals   []os.Signal   `json:"-"`                // 忽略的信号

	// 行为配置
	WaitForHooks    bool          `json:"wait_for_hooks"`   // 等待所有钩子完成
	ParallelHooks   bool          `json:"parallel_hooks"`   // 并行执行钩子
	ContinueOnError bool          `json:"continue_on_error"` // 出错时继续
	LogErrors       bool          `json:"log_errors"`       // 记录错误

	// 回调配置
	OnStart         func()        `json:"-"`                // 开始关闭回调
	OnComplete      func()        `json:"-"`                // 完成关闭回调
	OnTimeout       func()        `json:"-"`                // 超时回调
	OnError         func(error)   `json:"-"`                // 错误回调
}

// ShutdownStats 关闭统计信息
type ShutdownStats struct {
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Duration        time.Duration `json:"duration"`
	HooksExecuted   int           `json:"hooks_executed"`
	HooksSucceeded  int           `json:"hooks_succeeded"`
	HooksFailed     int           `json:"hooks_failed"`
	Errors          []string      `json:"errors"`
	TimedOut        bool          `json:"timed_out"`
	ForcedShutdown  bool          `json:"forced_shutdown"`
}

// DefaultShutdownConfig 返回默认关闭配置
func DefaultShutdownConfig() *ShutdownConfig {
	return &ShutdownConfig{
		Timeout:         30 * time.Second,
		GracePeriod:     5 * time.Second,
		ForceTimeout:    10 * time.Second,
		HookTimeout:     15 * time.Second,
		Signals:         []os.Signal{syscall.SIGTERM, syscall.SIGINT},
		IgnoreSignals:   []os.Signal{},
		WaitForHooks:    true,
		ParallelHooks:   false,
		ContinueOnError: true,
		LogErrors:       true,
	}
}

// NewShutdownManager 创建优雅关闭管理器
func NewShutdownManager(config *ShutdownConfig) *ShutdownManager {
	if config == nil {
		config = DefaultShutdownConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ShutdownManager{
		config:     config,
		hooks:      make([]ShutdownHook, 0),
		namedHooks: make(map[string]ShutdownHook),
		ctx:        ctx,
		cancel:     cancel,
		stats:      &ShutdownStats{},
		signalChan: make(chan os.Signal, 1),
	}
}

// AddHook 添加关闭钩子
func (s *ShutdownManager) AddHook(hook ShutdownHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = append(s.hooks, hook)
}

// AddNamedHook 添加命名关闭钩子
func (s *ShutdownManager) AddNamedHook(name string, hook ShutdownHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.namedHooks[name] = hook
}

// RemoveNamedHook 移除命名关闭钩子
func (s *ShutdownManager) RemoveNamedHook(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.namedHooks, name)
}

// Start 启动优雅关闭管理器
func (s *ShutdownManager) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true

	// 注册信号监听
	signal.Notify(s.signalChan, s.config.Signals...)

	// 忽略指定信号
	for _, sig := range s.config.IgnoreSignals {
		signal.Ignore(sig)
	}

	// 启动信号监听协程
	go s.signalListener()
}

// Stop 停止优雅关闭管理器
func (s *ShutdownManager) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	signal.Stop(s.signalChan)
	close(s.signalChan)
	s.cancel()
}

// Shutdown 执行优雅关闭
func (s *ShutdownManager) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return fmt.Errorf("shutdown already in progress")
	}
	s.shuttingDown = true
	s.stats.StartTime = time.Now()
	s.mu.Unlock()

	// 调用开始回调
	if s.config.OnStart != nil {
		s.config.OnStart()
	}

	// 创建超时上下文
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	// 执行关闭钩子
	err := s.executeHooks(shutdownCtx)

	// 更新统计信息
	s.mu.Lock()
	s.stats.EndTime = time.Now()
	s.stats.Duration = s.stats.EndTime.Sub(s.stats.StartTime)
	s.mu.Unlock()

	// 调用完成回调
	if s.config.OnComplete != nil {
		s.config.OnComplete()
	}

	return err
}

// ForceShutdown 强制关闭
func (s *ShutdownManager) ForceShutdown() {
	s.mu.Lock()
	s.stats.ForcedShutdown = true
	s.mu.Unlock()

	s.cancel()

	// 等待强制超时后退出
	go func() {
		time.Sleep(s.config.ForceTimeout)
		os.Exit(1)
	}()
}

// signalListener 信号监听器
func (s *ShutdownManager) signalListener() {
	for {
		select {
		case sig, ok := <-s.signalChan:
			if !ok {
				return
			}

			fmt.Printf("Received signal: %v, starting graceful shutdown...\n", sig)

			// 执行优雅关闭
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
				defer cancel()

				if err := s.Shutdown(ctx); err != nil {
					fmt.Printf("Graceful shutdown failed: %v\n", err)
					if s.config.OnError != nil {
						s.config.OnError(err)
					}
				}

				os.Exit(0)
			}()

			// 如果再次收到信号，强制关闭
			go func() {
				select {
				case <-s.signalChan:
					fmt.Println("Received second signal, forcing shutdown...")
					s.ForceShutdown()
				case <-time.After(s.config.GracePeriod):
					// 优雅期结束
				}
			}()

		case <-s.ctx.Done():
			return
		}
	}
}

// executeHooks 执行关闭钩子
func (s *ShutdownManager) executeHooks(ctx context.Context) error {
	s.mu.RLock()
	hooks := make([]ShutdownHook, len(s.hooks))
	copy(hooks, s.hooks)

	namedHooks := make(map[string]ShutdownHook)
	for name, hook := range s.namedHooks {
		namedHooks[name] = hook
	}
	s.mu.RUnlock()

	allHooks := make([]ShutdownHook, 0, len(hooks)+len(namedHooks))
	allHooks = append(allHooks, hooks...)
	for _, hook := range namedHooks {
		allHooks = append(allHooks, hook)
	}

	if len(allHooks) == 0 {
		return nil
	}

	if s.config.ParallelHooks {
		return s.executeHooksParallel(ctx, allHooks)
	} else {
		return s.executeHooksSequential(ctx, allHooks)
	}
}

// executeHooksSequential 顺序执行钩子
func (s *ShutdownManager) executeHooksSequential(ctx context.Context, hooks []ShutdownHook) error {
	var errors []error

	for i, hook := range hooks {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.stats.TimedOut = true
			s.mu.Unlock()
			if s.config.OnTimeout != nil {
				s.config.OnTimeout()
			}
			return ctx.Err()
		default:
		}

		// 为每个钩子创建超时上下文
		hookCtx, cancel := context.WithTimeout(ctx, s.config.HookTimeout)

		func() {
			defer cancel()
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("hook %d panicked: %v", i, r)
					errors = append(errors, err)
					s.updateStats(false, err)
				}
			}()

			if err := hook(hookCtx); err != nil {
				errors = append(errors, err)
				s.updateStats(false, err)
				if s.config.LogErrors {
					fmt.Printf("Hook %d failed: %v\n", i, err)
				}
				if s.config.OnError != nil {
					s.config.OnError(err)
				}
				if !s.config.ContinueOnError {
					return
				}
			} else {
				s.updateStats(true, nil)
			}
		}()
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown hooks failed: %v", errors)
	}

	return nil
}

// executeHooksParallel 并行执行钩子
func (s *ShutdownManager) executeHooksParallel(ctx context.Context, hooks []ShutdownHook) error {
	var wg sync.WaitGroup
	errorChan := make(chan error, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(index int, h ShutdownHook) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("hook %d panicked: %v", index, r)
					errorChan <- err
					s.updateStats(false, err)
				}
			}()

			// 为每个钩子创建超时上下文
			hookCtx, cancel := context.WithTimeout(ctx, s.config.HookTimeout)
			defer cancel()

			if err := h(hookCtx); err != nil {
				errorChan <- err
				s.updateStats(false, err)
				if s.config.LogErrors {
					fmt.Printf("Hook %d failed: %v\n", index, err)
				}
				if s.config.OnError != nil {
					s.config.OnError(err)
				}
			} else {
				s.updateStats(true, nil)
			}
		}(i, hook)
	}

	// 等待所有钩子完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有钩子完成
	case <-ctx.Done():
		// 超时
		s.mu.Lock()
		s.stats.TimedOut = true
		s.mu.Unlock()
		if s.config.OnTimeout != nil {
			s.config.OnTimeout()
		}
		return ctx.Err()
	}

	// 收集错误
	close(errorChan)
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown hooks failed: %v", errors)
	}

	return nil
}

// updateStats 更新统计信息
func (s *ShutdownManager) updateStats(success bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.HooksExecuted++
	if success {
		s.stats.HooksSucceeded++
	} else {
		s.stats.HooksFailed++
		if err != nil {
			s.stats.Errors = append(s.stats.Errors, err.Error())
		}
	}
}

// GetStats 获取统计信息
func (s *ShutdownManager) GetStats() *ShutdownStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := *s.stats
	return &stats
}

// IsShuttingDown 检查是否正在关闭
func (s *ShutdownManager) IsShuttingDown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shuttingDown
}

// IsRunning 检查是否运行
func (s *ShutdownManager) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Wait 等待关闭完成
func (s *ShutdownManager) Wait() {
	<-s.ctx.Done()
}

// 便利函数

// WaitForShutdown 等待关闭信号并执行优雅关闭
func WaitForShutdown(hooks ...ShutdownHook) error {
	manager := NewShutdownManager(nil)

	for _, hook := range hooks {
		manager.AddHook(hook)
	}

	manager.Start()
	manager.Wait()

	return nil
}

// CreateServerShutdownHook 创建服务器关闭钩子
func CreateServerShutdownHook(server interface{ Shutdown(context.Context) error }, name string) ShutdownHook {
	return func(ctx context.Context) error {
		fmt.Printf("Shutting down %s server...\n", name)
		return server.Shutdown(ctx)
	}
}

// CreateDatabaseShutdownHook 创建数据库关闭钩子
func CreateDatabaseShutdownHook(db interface{ Close() error }, name string) ShutdownHook {
	return func(ctx context.Context) error {
		fmt.Printf("Closing %s database connection...\n", name)
		return db.Close()
	}
}

// CreateCustomShutdownHook 创建自定义关闭钩子
func CreateCustomShutdownHook(name string, fn func(context.Context) error) ShutdownHook {
	return func(ctx context.Context) error {
		fmt.Printf("Executing %s shutdown hook...\n", name)
		return fn(ctx)
	}
}