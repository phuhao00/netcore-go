// Package pool 实现NetCore-Go网络库的协程池
// Author: NetCore-Go Team
// Created: 2024

package pool

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Task 任务接口
type Task interface {
	// Execute 执行任务
	Execute() error
}

// TaskFunc 任务函数类型
type TaskFunc func() error

// Execute 执行任务函数
func (f TaskFunc) Execute() error {
	return f()
}

// GoroutinePool 协程池接口
type GoroutinePool interface {
	// Submit 提交任务
	Submit(task Task) error
	// SubmitFunc 提交任务函数
	SubmitFunc(fn func() error) error
	// Start 启动协程池
	Start() error
	// Stop 停止协程池
	Stop() error
	// Stats 获取统计信息
	Stats() *GoroutinePoolStats
}

// WorkerPool 工作协程池实现
type WorkerPool struct {
	minWorkers    int
	maxWorkers    int
	maxIdleTime   time.Duration
	taskQueue     chan Task
	queueSize     int
	workers       map[int]*worker
	workerID      int32
	running       int32
	activeWorkers int32
	stats         *GoroutinePoolStats
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
}

// WorkerPoolConfig 工作池配置
type WorkerPoolConfig struct {
	// MinWorkers 最小工作协程数
	MinWorkers int
	// MaxWorkers 最大工作协程数
	MaxWorkers int
	// QueueSize 任务队列大小
	QueueSize int
	// MaxIdleTime 最大空闲时间
	MaxIdleTime time.Duration
}

// DefaultWorkerPoolConfig 返回默认工作池配置
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		MinWorkers:  runtime.NumCPU(),
		MaxWorkers:  runtime.NumCPU() * 2,
		QueueSize:   1000,
		MaxIdleTime: 60 * time.Second,
	}
}

// NewWorkerPool 创建新的工作协程池
func NewWorkerPool(config *WorkerPoolConfig) *WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		minWorkers:  config.MinWorkers,
		maxWorkers:  config.MaxWorkers,
		maxIdleTime: config.MaxIdleTime,
		queueSize:   config.QueueSize,
		taskQueue:   make(chan Task, config.QueueSize),
		workers:     make(map[int]*worker),
		stats:       NewGoroutinePoolStats(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动协程池
func (p *WorkerPool) Start() error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return nil // 已经启动
	}

	// 启动最小数量的工作协程
	for i := 0; i < p.minWorkers; i++ {
		p.addWorker()
	}

	// 启动监控协程
	p.wg.Add(1)
	go p.monitor()

	return nil
}

// Stop 停止协程池
func (p *WorkerPool) Stop() error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return nil // 已经停止
	}

	// 取消上下文
	p.cancel()

	// 关闭任务队列
	close(p.taskQueue)

	// 等待所有协程结束
	p.wg.Wait()

	return nil
}

// Submit 提交任务
func (p *WorkerPool) Submit(task Task) error {
	if atomic.LoadInt32(&p.running) == 0 {
		return ErrPoolStopped
	}

	select {
	case p.taskQueue <- task:
		atomic.AddInt64(&p.stats.TasksSubmitted, 1)
		return nil
	default:
		atomic.AddInt64(&p.stats.TasksRejected, 1)
		return ErrQueueFull
	}
}

// SubmitFunc 提交任务函数
func (p *WorkerPool) SubmitFunc(fn func() error) error {
	return p.Submit(TaskFunc(fn))
}

// Stats 获取统计信息
func (p *WorkerPool) Stats() *GoroutinePoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &GoroutinePoolStats{
		ActiveWorkers:   atomic.LoadInt32(&p.activeWorkers),
		TotalWorkers:    int32(len(p.workers)),
		TasksSubmitted:  atomic.LoadInt64(&p.stats.TasksSubmitted),
		TasksCompleted:  atomic.LoadInt64(&p.stats.TasksCompleted),
		TasksRejected:   atomic.LoadInt64(&p.stats.TasksRejected),
		TasksFailed:     atomic.LoadInt64(&p.stats.TasksFailed),
		QueueSize:       int32(len(p.taskQueue)),
		MaxQueueSize:    int32(p.queueSize),
	}
}

// addWorker 添加工作协程
func (p *WorkerPool) addWorker() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) >= p.maxWorkers {
		return
	}

	id := int(atomic.AddInt32(&p.workerID, 1))
	w := &worker{
		id:       id,
		pool:     p,
		lastUsed: time.Now(),
	}

	p.workers[id] = w
	p.wg.Add(1)
	go w.run()
}

// removeWorker 移除工作协程
func (p *WorkerPool) removeWorker(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.workers, id)
}

// monitor 监控协程
func (p *WorkerPool) monitor() {
	defer p.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.adjustWorkers()
		}
	}
}

// adjustWorkers 调整工作协程数量
func (p *WorkerPool) adjustWorkers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	queueLen := len(p.taskQueue)
	workerCount := len(p.workers)

	// 如果队列积压严重且未达到最大工作协程数，增加工作协程
	if queueLen > p.queueSize/2 && workerCount < p.maxWorkers {
		p.addWorker()
	}

	// 清理空闲时间过长的工作协程
	now := time.Now()
	for id, w := range p.workers {
		if workerCount > p.minWorkers && now.Sub(w.lastUsed) > p.maxIdleTime {
			w.stop()
			delete(p.workers, id)
			workerCount--
		}
	}
}

// worker 工作协程
type worker struct {
	id       int
	pool     *WorkerPool
	lastUsed time.Time
	stopped  int32
}

// run 运行工作协程
func (w *worker) run() {
	defer func() {
		w.pool.wg.Done()
		w.pool.removeWorker(w.id)
	}()

	for {
		select {
		case <-w.pool.ctx.Done():
			return
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				return
			}

			if atomic.LoadInt32(&w.stopped) == 1 {
				return
			}

			atomic.AddInt32(&w.pool.activeWorkers, 1)
			w.lastUsed = time.Now()

			// 执行任务
			if err := task.Execute(); err != nil {
				atomic.AddInt64(&w.pool.stats.TasksFailed, 1)
			} else {
				atomic.AddInt64(&w.pool.stats.TasksCompleted, 1)
			}

			atomic.AddInt32(&w.pool.activeWorkers, -1)
		}
	}
}

// stop 停止工作协程
func (w *worker) stop() {
	atomic.StoreInt32(&w.stopped, 1)
}

// GoroutinePoolStats 协程池统计信息
type GoroutinePoolStats struct {
	// ActiveWorkers 活跃工作协程数
	ActiveWorkers int32 `json:"active_workers"`
	// TotalWorkers 总工作协程数
	TotalWorkers int32 `json:"total_workers"`
	// TasksSubmitted 提交的任务数
	TasksSubmitted int64 `json:"tasks_submitted"`
	// TasksCompleted 完成的任务数
	TasksCompleted int64 `json:"tasks_completed"`
	// TasksRejected 拒绝的任务数
	TasksRejected int64 `json:"tasks_rejected"`
	// TasksFailed 失败的任务数
	TasksFailed int64 `json:"tasks_failed"`
	// QueueSize 当前队列大小
	QueueSize int32 `json:"queue_size"`
	// MaxQueueSize 最大队列大小
	MaxQueueSize int32 `json:"max_queue_size"`
}

// NewGoroutinePoolStats 创建新的协程池统计信息
func NewGoroutinePoolStats() *GoroutinePoolStats {
	return &GoroutinePoolStats{}
}

// 错误定义
var (
	// ErrPoolStopped 协程池已停止错误
	ErrPoolStopped = fmt.Errorf("goroutine pool is stopped")
	// ErrQueueFull 队列已满错误
	ErrQueueFull = fmt.Errorf("task queue is full")
)

// 默认协程池实例
var DefaultGoroutinePool = NewWorkerPool(DefaultWorkerPoolConfig())

// SubmitTask 提交任务到默认协程池
func SubmitTask(task Task) error {
	return DefaultGoroutinePool.Submit(task)
}

// SubmitTaskFunc 提交任务函数到默认协程池
func SubmitTaskFunc(fn func() error) error {
	return DefaultGoroutinePool.SubmitFunc(fn)
}

// StartDefaultPool 启动默认协程池
func StartDefaultPool() error {
	return DefaultGoroutinePool.Start()
}

// StopDefaultPool 停止默认协程池
func StopDefaultPool() error {
	return DefaultGoroutinePool.Stop()
}