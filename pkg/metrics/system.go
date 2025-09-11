package metrics

import (
	"os"
	"runtime"
	"sync"
	"time"
)

// SystemCollector 系统指标收集器
type SystemCollector struct {
	mu              sync.RWMutex
	enabled         bool
	collectInterval time.Duration
	stopChan        chan struct{}
	wg              sync.WaitGroup
	
	// CPU指标
	cpuUsage     *Gauge
	goroutines   *Gauge
	cgoCalls     *Counter
	
	// 内存指标
	memoryAlloc      *Gauge
	memoryTotalAlloc *Counter
	memoryHeapAlloc  *Gauge
	memoryHeapSys    *Gauge
	memoryStackSys   *Gauge
	memoryGCSys      *Gauge
	memoryOtherSys   *Gauge
	
	// GC指标
	gcRuns       *Counter
	gcPauseTotal *Counter
	gcPauseLast  *Gauge
	
	// 进程指标
	processStartTime *Gauge
	processUptime    *Gauge
	processPID       *Gauge
	
	// 运行时指标
	runtimeVersion *Gauge
	runtimeArch    *Gauge
	runtimeOS      *Gauge
	
	// 自定义指标
	customMetrics map[string]func() float64
}

// SystemCollectorConfig 系统收集器配置
type SystemCollectorConfig struct {
	Enabled         bool          // 是否启用
	CollectInterval time.Duration // 收集间隔
	Registry        *Registry     // 指标注册表
	Namespace       string        // 命名空间
}

// NewSystemCollector 创建系统指标收集器
func NewSystemCollector(config *SystemCollectorConfig) *SystemCollector {
	if config == nil {
		config = &SystemCollectorConfig{}
	}
	
	if config.CollectInterval <= 0 {
		config.CollectInterval = 15 * time.Second
	}
	
	if config.Registry == nil {
		config.Registry = DefaultRegistry
	}
	
	namespace := config.Namespace
	if namespace != "" {
		namespace += "_"
	}
	
	collector := &SystemCollector{
		enabled:         config.Enabled,
		collectInterval: config.CollectInterval,
		stopChan:        make(chan struct{}),
		customMetrics:   make(map[string]func() float64),
	}
	
	// 初始化CPU指标
	collector.cpuUsage = NewGauge(
		namespace+"cpu_usage_percent",
		"Current CPU usage percentage",
		Labels{},
	)
	collector.goroutines = NewGauge(
		namespace+"goroutines_count",
		"Number of goroutines",
		Labels{},
	)
	collector.cgoCalls = NewCounter(
		namespace+"cgo_calls_total",
		"Total number of cgo calls",
		Labels{},
	)
	
	// 初始化内存指标
	collector.memoryAlloc = NewGauge(
		namespace+"memory_alloc_bytes",
		"Bytes of allocated heap objects",
		Labels{},
	)
	collector.memoryTotalAlloc = NewCounter(
		namespace+"memory_total_alloc_bytes",
		"Cumulative bytes allocated for heap objects",
		Labels{},
	)
	collector.memoryHeapAlloc = NewGauge(
		namespace+"memory_heap_alloc_bytes",
		"Bytes of allocated heap objects",
		Labels{},
	)
	collector.memoryHeapSys = NewGauge(
		namespace+"memory_heap_sys_bytes",
		"Bytes of heap memory obtained from OS",
		Labels{},
	)
	collector.memoryStackSys = NewGauge(
		namespace+"memory_stack_sys_bytes",
		"Bytes of stack memory obtained from OS",
		Labels{},
	)
	collector.memoryGCSys = NewGauge(
		namespace+"memory_gc_sys_bytes",
		"Bytes of memory in garbage collection metadata",
		Labels{},
	)
	collector.memoryOtherSys = NewGauge(
		namespace+"memory_other_sys_bytes",
		"Bytes of memory in miscellaneous off-heap runtime allocations",
		Labels{},
	)
	
	// 初始化GC指标
	collector.gcRuns = NewCounter(
		namespace+"gc_runs_total",
		"Total number of GC runs",
		Labels{},
	)
	collector.gcPauseTotal = NewCounter(
		namespace+"gc_pause_total_seconds",
		"Total GC pause time in seconds",
		Labels{},
	)
	collector.gcPauseLast = NewGauge(
		namespace+"gc_pause_last_seconds",
		"Last GC pause time in seconds",
		Labels{},
	)
	
	// 初始化进程指标
	startTime := time.Now().Unix()
	collector.processStartTime = NewGauge(
		namespace+"process_start_time_seconds",
		"Start time of the process since unix epoch in seconds",
		Labels{},
	)
	collector.processStartTime.Set(float64(startTime))
	
	collector.processUptime = NewGauge(
		namespace+"process_uptime_seconds",
		"Process uptime in seconds",
		Labels{},
	)
	
	collector.processPID = NewGauge(
		namespace+"process_pid",
		"Process ID",
		Labels{},
	)
	collector.processPID.Set(float64(os.Getpid()))
	
	// 初始化运行时指标
	collector.runtimeVersion = NewGauge(
		namespace+"runtime_version_info",
		"Runtime version information",
		Labels{"version": runtime.Version()},
	)
	collector.runtimeVersion.Set(1)
	
	collector.runtimeArch = NewGauge(
		namespace+"runtime_arch_info",
		"Runtime architecture information",
		Labels{"arch": runtime.GOARCH},
	)
	collector.runtimeArch.Set(1)
	
	collector.runtimeOS = NewGauge(
		namespace+"runtime_os_info",
		"Runtime OS information",
		Labels{"os": runtime.GOOS},
	)
	collector.runtimeOS.Set(1)
	
	// 注册指标
	config.Registry.Register(collector.cpuUsage)
	config.Registry.Register(collector.goroutines)
	config.Registry.Register(collector.cgoCalls)
	config.Registry.Register(collector.memoryAlloc)
	config.Registry.Register(collector.memoryTotalAlloc)
	config.Registry.Register(collector.memoryHeapAlloc)
	config.Registry.Register(collector.memoryHeapSys)
	config.Registry.Register(collector.memoryStackSys)
	config.Registry.Register(collector.memoryGCSys)
	config.Registry.Register(collector.memoryOtherSys)
	config.Registry.Register(collector.gcRuns)
	config.Registry.Register(collector.gcPauseTotal)
	config.Registry.Register(collector.gcPauseLast)
	config.Registry.Register(collector.processStartTime)
	config.Registry.Register(collector.processUptime)
	config.Registry.Register(collector.processPID)
	config.Registry.Register(collector.runtimeVersion)
	config.Registry.Register(collector.runtimeArch)
	config.Registry.Register(collector.runtimeOS)
	
	return collector
}

// Start 启动收集器
func (c *SystemCollector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.enabled {
		return
	}
	
	c.enabled = true
	c.wg.Add(1)
	
	go c.collectLoop()
}

// Stop 停止收集器
func (c *SystemCollector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.enabled {
		return
	}
	
	c.enabled = false
	close(c.stopChan)
	c.wg.Wait()
	
	// 重新创建停止通道
	c.stopChan = make(chan struct{})
}

// collectLoop 收集循环
func (c *SystemCollector) collectLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.collectInterval)
	defer ticker.Stop()
	
	// 立即收集一次
	c.collect()
	
	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stopChan:
			return
		}
	}
}

// collect 收集指标
func (c *SystemCollector) collect() {
	c.collectRuntimeMetrics()
	c.collectMemoryMetrics()
	c.collectGCMetrics()
	c.collectProcessMetrics()
	c.collectCustomMetrics()
}

// collectRuntimeMetrics 收集运行时指标
func (c *SystemCollector) collectRuntimeMetrics() {
	// Goroutines数量
	c.goroutines.Set(float64(runtime.NumGoroutine()))
	
	// CGO调用数量
	c.cgoCalls.Add(float64(runtime.NumCgoCall()))
}

// collectMemoryMetrics 收集内存指标
func (c *SystemCollector) collectMemoryMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// 内存分配
	c.memoryAlloc.Set(float64(m.Alloc))
	c.memoryTotalAlloc.Add(float64(m.TotalAlloc))
	c.memoryHeapAlloc.Set(float64(m.HeapAlloc))
	c.memoryHeapSys.Set(float64(m.HeapSys))
	c.memoryStackSys.Set(float64(m.StackSys))
	c.memoryGCSys.Set(float64(m.GCSys))
	c.memoryOtherSys.Set(float64(m.OtherSys))
}

// collectGCMetrics 收集GC指标
func (c *SystemCollector) collectGCMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// GC运行次数
	c.gcRuns.Add(float64(m.NumGC))
	
	// GC暂停时间
	if m.NumGC > 0 {
		// 总暂停时间（纳秒转秒）
		c.gcPauseTotal.Add(float64(m.PauseTotalNs) / 1e9)
		
		// 最后一次暂停时间
		lastPause := m.PauseNs[(m.NumGC+255)%256]
		c.gcPauseLast.Set(float64(lastPause) / 1e9)
	}
}

// collectProcessMetrics 收集进程指标
func (c *SystemCollector) collectProcessMetrics() {
	// 进程运行时间
	startTime := c.processStartTime.Value().(float64)
	uptime := float64(time.Now().Unix()) - startTime
	c.processUptime.Set(uptime)
}

// collectCustomMetrics 收集自定义指标
func (c *SystemCollector) collectCustomMetrics() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	for name, collector := range c.customMetrics {
		if value := collector(); value >= 0 {
			// 这里需要根据指标名称找到对应的指标对象
			// 简化实现，实际使用时需要维护指标映射
			_ = name
			_ = value
		}
	}
}

// AddCustomMetric 添加自定义指标收集器
func (c *SystemCollector) AddCustomMetric(name string, collector func() float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.customMetrics[name] = collector
}

// RemoveCustomMetric 移除自定义指标收集器
func (c *SystemCollector) RemoveCustomMetric(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.customMetrics, name)
}

// GetMetrics 获取所有指标
func (c *SystemCollector) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})
	
	metrics["cpu_usage"] = c.cpuUsage.Value()
	metrics["goroutines"] = c.goroutines.Value()
	metrics["cgo_calls"] = c.cgoCalls.Value()
	metrics["memory_alloc"] = c.memoryAlloc.Value()
	metrics["memory_total_alloc"] = c.memoryTotalAlloc.Value()
	metrics["memory_heap_alloc"] = c.memoryHeapAlloc.Value()
	metrics["memory_heap_sys"] = c.memoryHeapSys.Value()
	metrics["memory_stack_sys"] = c.memoryStackSys.Value()
	metrics["memory_gc_sys"] = c.memoryGCSys.Value()
	metrics["memory_other_sys"] = c.memoryOtherSys.Value()
	metrics["gc_runs"] = c.gcRuns.Value()
	metrics["gc_pause_total"] = c.gcPauseTotal.Value()
	metrics["gc_pause_last"] = c.gcPauseLast.Value()
	metrics["process_start_time"] = c.processStartTime.Value()
	metrics["process_uptime"] = c.processUptime.Value()
	metrics["process_pid"] = c.processPID.Value()
	
	return metrics
}

// NetworkCollector 网络指标收集器
type NetworkCollector struct {
	mu              sync.RWMutex
	enabled         bool
	collectInterval time.Duration
	stopChan        chan struct{}
	wg              sync.WaitGroup
	
	// 网络指标
	connections     *Gauge
	bytesReceived   *Counter
	bytesSent       *Counter
	packetsReceived *Counter
	packetsSent     *Counter
	errorsReceived  *Counter
	errorsSent      *Counter
}

// NewNetworkCollector 创建网络指标收集器
func NewNetworkCollector(registry *Registry, namespace string) *NetworkCollector {
	if registry == nil {
		registry = DefaultRegistry
	}
	
	if namespace != "" {
		namespace += "_"
	}
	
	collector := &NetworkCollector{
		collectInterval: 30 * time.Second,
		stopChan:        make(chan struct{}),
	}
	
	// 初始化网络指标
	collector.connections = NewGauge(
		namespace+"network_connections",
		"Number of network connections",
		Labels{},
	)
	collector.bytesReceived = NewCounter(
		namespace+"network_bytes_received_total",
		"Total bytes received",
		Labels{},
	)
	collector.bytesSent = NewCounter(
		namespace+"network_bytes_sent_total",
		"Total bytes sent",
		Labels{},
	)
	collector.packetsReceived = NewCounter(
		namespace+"network_packets_received_total",
		"Total packets received",
		Labels{},
	)
	collector.packetsSent = NewCounter(
		namespace+"network_packets_sent_total",
		"Total packets sent",
		Labels{},
	)
	collector.errorsReceived = NewCounter(
		namespace+"network_errors_received_total",
		"Total receive errors",
		Labels{},
	)
	collector.errorsSent = NewCounter(
		namespace+"network_errors_sent_total",
		"Total send errors",
		Labels{},
	)
	
	// 注册指标
	registry.Register(collector.connections)
	registry.Register(collector.bytesReceived)
	registry.Register(collector.bytesSent)
	registry.Register(collector.packetsReceived)
	registry.Register(collector.packetsSent)
	registry.Register(collector.errorsReceived)
	registry.Register(collector.errorsSent)
	
	return collector
}

// Start 启动网络收集器
func (c *NetworkCollector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.enabled {
		return
	}
	
	c.enabled = true
	c.wg.Add(1)
	
	go c.collectLoop()
}

// Stop 停止网络收集器
func (c *NetworkCollector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.enabled {
		return
	}
	
	c.enabled = false
	close(c.stopChan)
	c.wg.Wait()
	
	c.stopChan = make(chan struct{})
}

// collectLoop 网络收集循环
func (c *NetworkCollector) collectLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.collectNetworkMetrics()
		case <-c.stopChan:
			return
		}
	}
}

// collectNetworkMetrics 收集网络指标
func (c *NetworkCollector) collectNetworkMetrics() {
	// 这里应该实现实际的网络指标收集逻辑
	// 由于Go标准库没有直接的网络统计API，这里只是示例
	// 实际实现可能需要读取/proc/net/dev等系统文件
	
	// 示例：模拟网络指标
	c.connections.Set(float64(runtime.NumGoroutine() * 2)) // 模拟连接数
}

// 默认系统收集器
var DefaultSystemCollector *SystemCollector

// StartSystemCollector 启动默认系统收集器
func StartSystemCollector(config *SystemCollectorConfig) {
	if DefaultSystemCollector != nil {
		DefaultSystemCollector.Stop()
	}
	
	DefaultSystemCollector = NewSystemCollector(config)
	DefaultSystemCollector.Start()
}

// StopSystemCollector 停止默认系统收集器
func StopSystemCollector() {
	if DefaultSystemCollector != nil {
		DefaultSystemCollector.Stop()
	}
}

// GetSystemMetrics 获取系统指标
func GetSystemMetrics() map[string]interface{} {
	if DefaultSystemCollector != nil {
		return DefaultSystemCollector.GetMetrics()
	}
	return make(map[string]interface{})
}