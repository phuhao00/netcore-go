package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MetricType 指标类型
type MetricType string

const (
	CounterType   MetricType = "counter"
	GaugeType     MetricType = "gauge"
	HistogramType MetricType = "histogram"
	SummaryType   MetricType = "summary"
)

// Labels 标签映射
type Labels map[string]string

// String 返回标签字符串表示
func (l Labels) String() string {
	if len(l) == 0 {
		return ""
	}
	
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, k, l[k]))
	}
	
	return "{" + strings.Join(pairs, ",") + "}"
}

// Hash 返回标签哈希值
func (l Labels) Hash() string {
	return l.String()
}

// Metric 指标接口
type Metric interface {
	Name() string
	Type() MetricType
	Help() string
	Labels() Labels
	Value() interface{}
	Collect() []*Sample
}

// Sample 指标样本
type Sample struct {
	Name      string
	Labels    Labels
	Value     float64
	Timestamp time.Time
}

// String 返回样本字符串表示
func (s *Sample) String() string {
	labelsStr := s.Labels.String()
	if labelsStr != "" {
		return fmt.Sprintf("%s%s %g %d", s.Name, labelsStr, s.Value, s.Timestamp.UnixMilli())
	}
	return fmt.Sprintf("%s %g %d", s.Name, s.Value, s.Timestamp.UnixMilli())
}

// Counter 计数器指标
type Counter struct {
	mu     sync.RWMutex
	name   string
	help   string
	labels Labels
	value  float64
}

// NewCounter 创建计数器
func NewCounter(name, help string, labels Labels) *Counter {
	return &Counter{
		name:   name,
		help:   help,
		labels: labels,
	}
}

// Name 返回指标名称
func (c *Counter) Name() string {
	return c.name
}

// Type 返回指标类型
func (c *Counter) Type() MetricType {
	return CounterType
}

// Help 返回帮助信息
func (c *Counter) Help() string {
	return c.help
}

// Labels 返回标签
func (c *Counter) Labels() Labels {
	return c.labels
}

// Value 返回当前值
func (c *Counter) Value() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// Inc 增加计数器
func (c *Counter) Inc() {
	c.Add(1)
}

// Add 增加指定值
func (c *Counter) Add(value float64) {
	if value < 0 {
		panic("counter cannot decrease")
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += value
}

// Collect 收集样本
func (c *Counter) Collect() []*Sample {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return []*Sample{
		{
			Name:      c.name,
			Labels:    c.labels,
			Value:     c.value,
			Timestamp: time.Now(),
		},
	}
}

// Gauge 仪表盘指标
type Gauge struct {
	mu     sync.RWMutex
	name   string
	help   string
	labels Labels
	value  float64
}

// NewGauge 创建仪表盘指标
func NewGauge(name, help string, labels Labels) *Gauge {
	return &Gauge{
		name:   name,
		help:   help,
		labels: labels,
	}
}

// Name 返回指标名称
func (g *Gauge) Name() string {
	return g.name
}

// Type 返回指标类型
func (g *Gauge) Type() MetricType {
	return GaugeType
}

// Help 返回帮助信息
func (g *Gauge) Help() string {
	return g.help
}

// Labels 返回标签
func (g *Gauge) Labels() Labels {
	return g.labels
}

// Value 返回当前值
func (g *Gauge) Value() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.value
}

// Set 设置值
func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

// Inc 增加1
func (g *Gauge) Inc() {
	g.Add(1)
}

// Dec 减少1
func (g *Gauge) Dec() {
	g.Add(-1)
}

// Add 增加指定值
func (g *Gauge) Add(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value += value
}

// Sub 减少指定值
func (g *Gauge) Sub(value float64) {
	g.Add(-value)
}

// Collect 收集样本
func (g *Gauge) Collect() []*Sample {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	return []*Sample{
		{
			Name:      g.name,
			Labels:    g.labels,
			Value:     g.value,
			Timestamp: time.Now(),
		},
	}
}

// Histogram 直方图指标
type Histogram struct {
	mu      sync.RWMutex
	name    string
	help    string
	labels  Labels
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

// NewHistogram 创建直方图指标
func NewHistogram(name, help string, labels Labels, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	
	// 确保buckets是排序的
	sort.Float64s(buckets)
	
	return &Histogram{
		name:    name,
		help:    help,
		labels:  labels,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)),
	}
}

// Name 返回指标名称
func (h *Histogram) Name() string {
	return h.name
}

// Type 返回指标类型
func (h *Histogram) Type() MetricType {
	return HistogramType
}

// Help 返回帮助信息
func (h *Histogram) Help() string {
	return h.help
}

// Labels 返回标签
func (h *Histogram) Labels() Labels {
	return h.labels
}

// Value 返回当前值
func (h *Histogram) Value() interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return map[string]interface{}{
		"count":   h.count,
		"sum":     h.sum,
		"buckets": h.buckets,
		"counts":  h.counts,
	}
}

// Observe 观察值
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.sum += value
	h.count++
	
	// 更新bucket计数
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
}

// Collect 收集样本
func (h *Histogram) Collect() []*Sample {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	samples := make([]*Sample, 0, len(h.buckets)+2)
	now := time.Now()
	
	// bucket样本
	cumulativeCount := uint64(0)
	for i, bucket := range h.buckets {
		cumulativeCount += h.counts[i]
		bucketLabels := make(Labels)
		for k, v := range h.labels {
			bucketLabels[k] = v
		}
		bucketLabels["le"] = fmt.Sprintf("%g", bucket)
		
		samples = append(samples, &Sample{
			Name:      h.name + "_bucket",
			Labels:    bucketLabels,
			Value:     float64(cumulativeCount),
			Timestamp: now,
		})
	}
	
	// +Inf bucket
	infLabels := make(Labels)
	for k, v := range h.labels {
		infLabels[k] = v
	}
	infLabels["le"] = "+Inf"
	samples = append(samples, &Sample{
		Name:      h.name + "_bucket",
		Labels:    infLabels,
		Value:     float64(h.count),
		Timestamp: now,
	})
	
	// count样本
	samples = append(samples, &Sample{
		Name:      h.name + "_count",
		Labels:    h.labels,
		Value:     float64(h.count),
		Timestamp: now,
	})
	
	// sum样本
	samples = append(samples, &Sample{
		Name:      h.name + "_sum",
		Labels:    h.labels,
		Value:     h.sum,
		Timestamp: now,
	})
	
	return samples
}

// Summary 摘要指标
type Summary struct {
	mu        sync.RWMutex
	name      string
	help      string
	labels    Labels
	quantiles []float64
	values    []float64
	sum       float64
	count     uint64
	maxAge    time.Duration
	timestamps []time.Time
}

// NewSummary 创建摘要指标
func NewSummary(name, help string, labels Labels, quantiles []float64, maxAge time.Duration) *Summary {
	if len(quantiles) == 0 {
		quantiles = []float64{0.5, 0.9, 0.95, 0.99}
	}
	
	if maxAge <= 0 {
		maxAge = 10 * time.Minute
	}
	
	return &Summary{
		name:      name,
		help:      help,
		labels:    labels,
		quantiles: quantiles,
		maxAge:    maxAge,
	}
}

// Name 返回指标名称
func (s *Summary) Name() string {
	return s.name
}

// Type 返回指标类型
func (s *Summary) Type() MetricType {
	return SummaryType
}

// Help 返回帮助信息
func (s *Summary) Help() string {
	return s.help
}

// Labels 返回标签
func (s *Summary) Labels() Labels {
	return s.labels
}

// Value 返回当前值
func (s *Summary) Value() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"count":     s.count,
		"sum":       s.sum,
		"quantiles": s.quantiles,
		"values":    len(s.values),
	}
}

// Observe 观察值
func (s *Summary) Observe(value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	s.values = append(s.values, value)
	s.timestamps = append(s.timestamps, now)
	s.sum += value
	s.count++
	
	// 清理过期数据
	s.cleanup(now)
}

// cleanup 清理过期数据
func (s *Summary) cleanup(now time.Time) {
	cutoff := now.Add(-s.maxAge)
	
	// 找到第一个未过期的索引
	start := 0
	for i, ts := range s.timestamps {
		if ts.After(cutoff) {
			start = i
			break
		}
	}
	
	if start > 0 {
		// 移除过期数据
		s.values = s.values[start:]
		s.timestamps = s.timestamps[start:]
	}
}

// calculateQuantile 计算分位数
func (s *Summary) calculateQuantile(q float64) float64 {
	if len(s.values) == 0 {
		return 0
	}
	
	// 复制并排序值
	values := make([]float64, len(s.values))
	copy(values, s.values)
	sort.Float64s(values)
	
	// 计算分位数索引
	index := q * float64(len(values)-1)
	lower := int(index)
	upper := lower + 1
	
	if upper >= len(values) {
		return values[len(values)-1]
	}
	
	if lower == upper {
		return values[lower]
	}
	
	// 线性插值
	weight := index - float64(lower)
	return values[lower]*(1-weight) + values[upper]*weight
}

// Collect 收集样本
func (s *Summary) Collect() []*Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// 清理过期数据
	s.cleanup(time.Now())
	
	samples := make([]*Sample, 0, len(s.quantiles)+2)
	now := time.Now()
	
	// 分位数样本
	for _, q := range s.quantiles {
		quantileLabels := make(Labels)
		for k, v := range s.labels {
			quantileLabels[k] = v
		}
		quantileLabels["quantile"] = fmt.Sprintf("%g", q)
		
		samples = append(samples, &Sample{
			Name:      s.name,
			Labels:    quantileLabels,
			Value:     s.calculateQuantile(q),
			Timestamp: now,
		})
	}
	
	// count样本
	samples = append(samples, &Sample{
		Name:      s.name + "_count",
		Labels:    s.labels,
		Value:     float64(s.count),
		Timestamp: now,
	})
	
	// sum样本
	samples = append(samples, &Sample{
		Name:      s.name + "_sum",
		Labels:    s.labels,
		Value:     s.sum,
		Timestamp: now,
	})
	
	return samples
}

// Registry 指标注册表
type Registry struct {
	mu      sync.RWMutex
	metrics map[string]Metric
}

// NewRegistry 创建指标注册表
func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[string]Metric),
	}
}

// Register 注册指标
func (r *Registry) Register(metric Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	key := r.getMetricKey(metric)
	if _, exists := r.metrics[key]; exists {
		return fmt.Errorf("metric already registered: %s", key)
	}
	
	r.metrics[key] = metric
	return nil
}

// Unregister 注销指标
func (r *Registry) Unregister(metric Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	key := r.getMetricKey(metric)
	delete(r.metrics, key)
}

// getMetricKey 获取指标键
func (r *Registry) getMetricKey(metric Metric) string {
	return metric.Name() + metric.Labels().Hash()
}

// Gather 收集所有指标
func (r *Registry) Gather() []*Sample {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var samples []*Sample
	for _, metric := range r.metrics {
		samples = append(samples, metric.Collect()...)
	}
	
	return samples
}

// GetMetrics 获取所有指标
func (r *Registry) GetMetrics() []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	metrics := make([]Metric, 0, len(r.metrics))
	for _, metric := range r.metrics {
		metrics = append(metrics, metric)
	}
	
	return metrics
}

// 默认注册表
var DefaultRegistry = NewRegistry()

// 便捷函数

// RegisterCounter 注册计数器
func RegisterCounter(name, help string, labels Labels) *Counter {
	counter := NewCounter(name, help, labels)
	DefaultRegistry.Register(counter)
	return counter
}

// RegisterGauge 注册仪表盘指标
func RegisterGauge(name, help string, labels Labels) *Gauge {
	gauge := NewGauge(name, help, labels)
	DefaultRegistry.Register(gauge)
	return gauge
}

// RegisterHistogram 注册直方图指标
func RegisterHistogram(name, help string, labels Labels, buckets []float64) *Histogram {
	histogram := NewHistogram(name, help, labels, buckets)
	DefaultRegistry.Register(histogram)
	return histogram
}

// RegisterSummary 注册摘要指标
func RegisterSummary(name, help string, labels Labels, quantiles []float64, maxAge time.Duration) *Summary {
	summary := NewSummary(name, help, labels, quantiles, maxAge)
	DefaultRegistry.Register(summary)
	return summary
}

// Gather 收集默认注册表的所有指标
func Gather() []*Sample {
	return DefaultRegistry.Gather()
}