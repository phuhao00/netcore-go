package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PrometheusExporter Prometheus导出器
type PrometheusExporter struct {
	registry *Registry
	path     string
	server   *http.Server
}

// PrometheusConfig Prometheus配置
type PrometheusConfig struct {
	Registry *Registry // 指标注册表
	Path     string    // 导出路径
	Addr     string    // 监听地址
}

// NewPrometheusExporter 创建Prometheus导出器
func NewPrometheusExporter(config *PrometheusConfig) *PrometheusExporter {
	if config == nil {
		config = &PrometheusConfig{}
	}
	
	if config.Registry == nil {
		config.Registry = DefaultRegistry
	}
	
	if config.Path == "" {
		config.Path = "/metrics"
	}
	
	if config.Addr == "" {
		config.Addr = ":9090"
	}
	
	exporter := &PrometheusExporter{
		registry: config.Registry,
		path:     config.Path,
	}
	
	// 创建HTTP服务器
	mux := http.NewServeMux()
	mux.HandleFunc(config.Path, exporter.metricsHandler)
	mux.HandleFunc("/", exporter.indexHandler)
	
	exporter.server = &http.Server{
		Addr:    config.Addr,
		Handler: mux,
	}
	
	return exporter
}

// Start 启动导出器
func (e *PrometheusExporter) Start() error {
	return e.server.ListenAndServe()
}

// Stop 停止导出器
func (e *PrometheusExporter) Stop() error {
	return e.server.Close()
}

// metricsHandler 处理指标请求
func (e *PrometheusExporter) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	
	// 收集指标
	samples := e.registry.Gather()
	metrics := e.registry.GetMetrics()
	
	// 按指标分组
	metricGroups := make(map[string][]Metric)
	for _, metric := range metrics {
		name := metric.Name()
		metricGroups[name] = append(metricGroups[name], metric)
	}
	
	// 输出指标
	for name, group := range metricGroups {
		if len(group) > 0 {
			// 输出HELP
			fmt.Fprintf(w, "# HELP %s %s\n", name, escapeHelp(group[0].Help()))
			// 输出TYPE
			fmt.Fprintf(w, "# TYPE %s %s\n", name, group[0].Type())
		}
	}
	
	// 输出样本
	for _, sample := range samples {
		e.writeSample(w, sample)
	}
}

// indexHandler 处理首页请求
func (e *PrometheusExporter) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<html>
<head><title>NetCore-Go Metrics</title></head>
<body>
<h1>NetCore-Go Metrics</h1>
<p><a href="%s">Metrics</a></p>
</body>
</html>`, e.path)
}

// writeSample 写入样本
func (e *PrometheusExporter) writeSample(w io.Writer, sample *Sample) {
	labelsStr := ""
	if len(sample.Labels) > 0 {
		labelsStr = sample.Labels.String()
	}
	
	if sample.Timestamp.IsZero() {
		fmt.Fprintf(w, "%s%s %s\n", sample.Name, labelsStr, formatValue(sample.Value))
	} else {
		fmt.Fprintf(w, "%s%s %s %d\n", sample.Name, labelsStr, formatValue(sample.Value), sample.Timestamp.UnixMilli())
	}
}

// formatValue 格式化值
func formatValue(value float64) string {
	if value != value { // NaN
		return "NaN"
	}
	if value > 0 && value == value+1 { // +Inf
		return "+Inf"
	}
	if value < 0 && value == value-1 { // -Inf
		return "-Inf"
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// escapeHelp 转义帮助文本
func escapeHelp(help string) string {
	help = strings.ReplaceAll(help, "\\", "\\\\")
	help = strings.ReplaceAll(help, "\n", "\\n")
	return help
}

// PrometheusHandler 创建Prometheus HTTP处理器
func PrometheusHandler(registry *Registry) http.Handler {
	if registry == nil {
		registry = DefaultRegistry
	}
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		
		// 收集指标
		samples := registry.Gather()
		metrics := registry.GetMetrics()
		
		// 按指标分组
		metricGroups := make(map[string][]Metric)
		for _, metric := range metrics {
			name := metric.Name()
			metricGroups[name] = append(metricGroups[name], metric)
		}
		
		// 输出指标元数据
		for name, group := range metricGroups {
			if len(group) > 0 {
				// 输出HELP
				fmt.Fprintf(w, "# HELP %s %s\n", name, escapeHelp(group[0].Help()))
				// 输出TYPE
				fmt.Fprintf(w, "# TYPE %s %s\n", name, group[0].Type())
			}
		}
		
		// 输出样本
		for _, sample := range samples {
			writeSampleToWriter(w, sample)
		}
	})
}

// writeSampleToWriter 写入样本到Writer
func writeSampleToWriter(w io.Writer, sample *Sample) {
	labelsStr := ""
	if len(sample.Labels) > 0 {
		labelsStr = sample.Labels.String()
	}
	
	if sample.Timestamp.IsZero() {
		fmt.Fprintf(w, "%s%s %s\n", sample.Name, labelsStr, formatValue(sample.Value))
	} else {
		fmt.Fprintf(w, "%s%s %s %d\n", sample.Name, labelsStr, formatValue(sample.Value), sample.Timestamp.UnixMilli())
	}
}

// PrometheusCollector Prometheus收集器接口
type PrometheusCollector interface {
	Describe(chan<- *PrometheusDesc)
	Collect(chan<- *PrometheusSample)
}

// PrometheusDesc Prometheus指标描述
type PrometheusDesc struct {
	Name        string
	Help        string
	Type        MetricType
	LabelNames  []string
	ConstLabels Labels
}

// PrometheusSample Prometheus样本
type PrometheusSample struct {
	Desc      *PrometheusDesc
	Labels    Labels
	Value     float64
	Timestamp time.Time
}

// PrometheusRegistry Prometheus注册表
type PrometheusRegistry struct {
	mu         sync.RWMutex
	collectors map[string]PrometheusCollector
}

// NewPrometheusRegistry 创建Prometheus注册表
func NewPrometheusRegistry() *PrometheusRegistry {
	return &PrometheusRegistry{
		collectors: make(map[string]PrometheusCollector),
	}
}

// Register 注册收集器
func (r *PrometheusRegistry) Register(name string, collector PrometheusCollector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.collectors[name]; exists {
		return fmt.Errorf("collector already registered: %s", name)
	}
	
	r.collectors[name] = collector
	return nil
}

// Unregister 注销收集器
func (r *PrometheusRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.collectors, name)
}

// Gather 收集所有指标
func (r *PrometheusRegistry) Gather() ([]*PrometheusDesc, []*PrometheusSample) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	descChan := make(chan *PrometheusDesc, 100)
	sampleChan := make(chan *PrometheusSample, 1000)
	
	// 收集描述和样本
	go func() {
		defer close(descChan)
		defer close(sampleChan)
		
		for _, collector := range r.collectors {
			collector.Describe(descChan)
			collector.Collect(sampleChan)
		}
	}()
	
	// 收集结果
	var descs []*PrometheusDesc
	var samples []*PrometheusSample
	
	for desc := range descChan {
		descs = append(descs, desc)
	}
	
	for sample := range sampleChan {
		samples = append(samples, sample)
	}
	
	return descs, samples
}

// MetricCollector 指标收集器适配器
type MetricCollector struct {
	metric Metric
}

// NewMetricCollector 创建指标收集器
func NewMetricCollector(metric Metric) *MetricCollector {
	return &MetricCollector{metric: metric}
}

// Describe 描述指标
func (c *MetricCollector) Describe(ch chan<- *PrometheusDesc) {
	labelNames := make([]string, 0, len(c.metric.Labels()))
	for name := range c.metric.Labels() {
		labelNames = append(labelNames, name)
	}
	sort.Strings(labelNames)
	
	ch <- &PrometheusDesc{
		Name:       c.metric.Name(),
		Help:       c.metric.Help(),
		Type:       c.metric.Type(),
		LabelNames: labelNames,
	}
}

// Collect 收集样本
func (c *MetricCollector) Collect(ch chan<- *PrometheusSample) {
	samples := c.metric.Collect()
	for _, sample := range samples {
		ch <- &PrometheusSample{
			Desc: &PrometheusDesc{
				Name: sample.Name,
				Type: c.metric.Type(),
			},
			Labels:    sample.Labels,
			Value:     sample.Value,
			Timestamp: sample.Timestamp,
		}
	}
}

// PrometheusMiddleware HTTP中间件
func PrometheusMiddleware(registry *Registry) func(http.Handler) http.Handler {
	if registry == nil {
		registry = DefaultRegistry
	}
	
	// 注册HTTP指标
	requestCounter := RegisterCounter(
		"http_requests_total",
		"Total number of HTTP requests",
		Labels{"method": "", "path": "", "status": ""},
	)
	
	requestDuration := RegisterHistogram(
		"http_request_duration_seconds",
		"HTTP request duration in seconds",
		Labels{"method": "", "path": ""},
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	)
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// 包装ResponseWriter以捕获状态码
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
			
			// 处理请求
			next.ServeHTTP(wrapped, r)
			
			// 记录指标
			duration := time.Since(start).Seconds()
			
			// 更新指标
			requestCounter.Add(1)
			requestDuration.Observe(duration)
		})
	}
}

// responseWriter 包装ResponseWriter以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// DefaultPrometheusExporter 默认Prometheus导出器
var DefaultPrometheusExporter *PrometheusExporter

// StartPrometheusExporter 启动默认Prometheus导出器
func StartPrometheusExporter(addr string) error {
	if addr == "" {
		addr = ":9090"
	}
	
	DefaultPrometheusExporter = NewPrometheusExporter(&PrometheusConfig{
		Addr: addr,
	})
	
	return DefaultPrometheusExporter.Start()
}

// StopPrometheusExporter 停止默认Prometheus导出器
func StopPrometheusExporter() error {
	if DefaultPrometheusExporter != nil {
		return DefaultPrometheusExporter.Stop()
	}
	return nil
}