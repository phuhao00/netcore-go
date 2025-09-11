package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SpanKind 跨度类型
type SpanKind int

const (
	// SpanKindInternal 内部跨度
	SpanKindInternal SpanKind = iota
	// SpanKindServer 服务端跨度
	SpanKindServer
	// SpanKindClient 客户端跨度
	SpanKindClient
	// SpanKindProducer 生产者跨度
	SpanKindProducer
	// SpanKindConsumer 消费者跨度
	SpanKindConsumer
)

// String 返回跨度类型字符串
func (sk SpanKind) String() string {
	switch sk {
	case SpanKindInternal:
		return "internal"
	case SpanKindServer:
		return "server"
	case SpanKindClient:
		return "client"
	case SpanKindProducer:
		return "producer"
	case SpanKindConsumer:
		return "consumer"
	default:
		return "unknown"
	}
}

// SpanStatus 跨度状态
type SpanStatus int

const (
	// StatusUnset 未设置
	StatusUnset SpanStatus = iota
	// StatusOK 成功
	StatusOK
	// StatusError 错误
	StatusError
)

// String 返回状态字符串
func (ss SpanStatus) String() string {
	switch ss {
	case StatusUnset:
		return "unset"
	case StatusOK:
		return "ok"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// TracingConfig 追踪配置
type TracingConfig struct {
	// 启用追踪
	Enabled bool `json:"enabled" yaml:"enabled"`
	// 服务名称
	ServiceName string `json:"service_name" yaml:"service_name"`
	// 服务版本
	ServiceVersion string `json:"service_version" yaml:"service_version"`
	// 环境
	Environment string `json:"environment" yaml:"environment"`
	// 采样率 (0.0-1.0)
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`
	// 最大跨度数量
	MaxSpans int `json:"max_spans" yaml:"max_spans"`
	// 跨度超时时间
	SpanTimeout time.Duration `json:"span_timeout" yaml:"span_timeout"`
	// 批量导出大小
	BatchSize int `json:"batch_size" yaml:"batch_size"`
	// 导出间隔
	ExportInterval time.Duration `json:"export_interval" yaml:"export_interval"`
	// 启用自动仪表化
	AutoInstrumentation bool `json:"auto_instrumentation" yaml:"auto_instrumentation"`
	// 资源属性
	ResourceAttributes map[string]string `json:"resource_attributes" yaml:"resource_attributes"`
}

// DefaultTracingConfig 默认追踪配置
func DefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		Enabled:             true,
		ServiceName:         "netcore-service",
		ServiceVersion:      "1.0.0",
		Environment:         "development",
		SampleRate:          1.0,
		MaxSpans:            10000,
		SpanTimeout:         5 * time.Minute,
		BatchSize:           100,
		ExportInterval:      10 * time.Second,
		AutoInstrumentation: true,
		ResourceAttributes:  make(map[string]string),
	}
}

// TraceID 追踪ID
type TraceID [16]byte

// String 返回追踪ID字符串
func (t TraceID) String() string {
	return hex.EncodeToString(t[:])
}

// IsValid 检查追踪ID是否有效
func (t TraceID) IsValid() bool {
	return t != TraceID{}
}

// SpanID 跨度ID
type SpanID [8]byte

// String 返回跨度ID字符串
func (s SpanID) String() string {
	return hex.EncodeToString(s[:])
}

// IsValid 检查跨度ID是否有效
func (s SpanID) IsValid() bool {
	return s != SpanID{}
}

// SpanContext 跨度上下文
type SpanContext struct {
	// 追踪ID
	TraceID TraceID `json:"trace_id"`
	// 跨度ID
	SpanID SpanID `json:"span_id"`
	// 追踪标志
	TraceFlags byte `json:"trace_flags"`
	// 追踪状态
	TraceState string `json:"trace_state,omitempty"`
	// 是否远程
	Remote bool `json:"remote"`
}

// IsValid 检查跨度上下文是否有效
func (sc SpanContext) IsValid() bool {
	return sc.TraceID.IsValid() && sc.SpanID.IsValid()
}

// IsSampled 检查是否被采样
func (sc SpanContext) IsSampled() bool {
	return sc.TraceFlags&0x01 != 0
}

// Span 跨度接口
type Span interface {
	// End 结束跨度
	End()
	// SetName 设置名称
	SetName(name string)
	// SetStatus 设置状态
	SetStatus(status SpanStatus, description string)
	// SetAttribute 设置属性
	SetAttribute(key string, value interface{})
	// SetAttributes 设置多个属性
	SetAttributes(attributes map[string]interface{})
	// AddEvent 添加事件
	AddEvent(name string, attributes map[string]interface{})
	// RecordError 记录错误
	RecordError(err error)
	// SpanContext 获取跨度上下文
	SpanContext() SpanContext
	// IsRecording 是否正在记录
	IsRecording() bool
}

// Tracer 追踪器接口
type Tracer interface {
	// Start 开始新跨度
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
}

// TracerProvider 追踪器提供者接口
type TracerProvider interface {
	// Tracer 获取追踪器
	Tracer(name string, opts ...TracerOption) Tracer
	// Shutdown 关闭
	Shutdown(ctx context.Context) error
}

// SpanExporter 跨度导出器接口
type SpanExporter interface {
	// ExportSpans 导出跨度
	ExportSpans(ctx context.Context, spans []SpanData) error
	// Shutdown 关闭
	Shutdown(ctx context.Context) error
}

// SpanOption 跨度选项
type SpanOption interface {
	apply(*spanConfig)
}

// TracerOption 追踪器选项
type TracerOption interface {
	apply(*tracerConfig)
}

// spanConfig 跨度配置
type spanConfig struct {
	kind       SpanKind
	attributes map[string]interface{}
	links      []Link
	startTime  time.Time
	parent     SpanContext
}

// tracerConfig 追踪器配置
type tracerConfig struct {
	version     string
	schemaURL   string
	attributes  map[string]interface{}
}

// Link 跨度链接
type Link struct {
	// 跨度上下文
	SpanContext SpanContext `json:"span_context"`
	// 属性
	Attributes map[string]interface{} `json:"attributes"`
}

// Event 事件
type Event struct {
	// 名称
	Name string `json:"name"`
	// 时间戳
	Timestamp time.Time `json:"timestamp"`
	// 属性
	Attributes map[string]interface{} `json:"attributes"`
}

// SpanData 跨度数据
type SpanData struct {
	// 跨度上下文
	SpanContext SpanContext `json:"span_context"`
	// 父跨度ID
	ParentSpanID SpanID `json:"parent_span_id,omitempty"`
	// 名称
	Name string `json:"name"`
	// 跨度类型
	Kind SpanKind `json:"kind"`
	// 开始时间
	StartTime time.Time `json:"start_time"`
	// 结束时间
	EndTime time.Time `json:"end_time"`
	// 状态
	Status SpanStatus `json:"status"`
	// 状态描述
	StatusDescription string `json:"status_description,omitempty"`
	// 属性
	Attributes map[string]interface{} `json:"attributes"`
	// 事件
	Events []Event `json:"events"`
	// 链接
	Links []Link `json:"links"`
	// 资源
	Resource Resource `json:"resource"`
	// 仪表化库
	InstrumentationLibrary InstrumentationLibrary `json:"instrumentation_library"`
}

// Resource 资源
type Resource struct {
	// 属性
	Attributes map[string]interface{} `json:"attributes"`
}

// InstrumentationLibrary 仪表化库
type InstrumentationLibrary struct {
	// 名称
	Name string `json:"name"`
	// 版本
	Version string `json:"version"`
	// Schema URL
	SchemaURL string `json:"schema_url,omitempty"`
}

// span 跨度实现
type span struct {
	mu                     sync.RWMutex
	spanContext            SpanContext
	parentSpanID           SpanID
	name                   string
	kind                   SpanKind
	startTime              time.Time
	endTime                time.Time
	status                 SpanStatus
	statusDescription      string
	attributes             map[string]interface{}
	events                 []Event
	links                  []Link
	resource               Resource
	instrumentationLibrary InstrumentationLibrary
	tracer                 *tracer
	ended                  bool
	recording              bool
}

// End 结束跨度
func (s *span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ended {
		return
	}

	s.ended = true
	s.endTime = time.Now()

	// 导出跨度
	if s.recording && s.tracer != nil {
		s.tracer.addSpan(s.toSpanData())
	}
}

// SetName 设置名称
func (s *span) SetName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ended {
		s.name = name
	}
}

// SetStatus 设置状态
func (s *span) SetStatus(status SpanStatus, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ended {
		s.status = status
		s.statusDescription = description
	}
}

// SetAttribute 设置属性
func (s *span) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ended && s.recording {
		if s.attributes == nil {
			s.attributes = make(map[string]interface{})
		}
		s.attributes[key] = value
	}
}

// SetAttributes 设置多个属性
func (s *span) SetAttributes(attributes map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ended && s.recording {
		if s.attributes == nil {
			s.attributes = make(map[string]interface{})
		}
		for k, v := range attributes {
			s.attributes[k] = v
		}
	}
}

// AddEvent 添加事件
func (s *span) AddEvent(name string, attributes map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ended && s.recording {
		event := Event{
			Name:       name,
			Timestamp:  time.Now(),
			Attributes: attributes,
		}
		s.events = append(s.events, event)
	}
}

// RecordError 记录错误
func (s *span) RecordError(err error) {
	if err == nil {
		return
	}

	s.SetStatus(StatusError, err.Error())
	s.AddEvent("exception", map[string]interface{}{
		"exception.type":    fmt.Sprintf("%T", err),
		"exception.message": err.Error(),
	})
}

// SpanContext 获取跨度上下文
func (s *span) SpanContext() SpanContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spanContext
}

// IsRecording 是否正在记录
func (s *span) IsRecording() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recording && !s.ended
}

// toSpanData 转换为跨度数据
func (s *span) toSpanData() SpanData {
	return SpanData{
		SpanContext:            s.spanContext,
		ParentSpanID:           s.parentSpanID,
		Name:                   s.name,
		Kind:                   s.kind,
		StartTime:              s.startTime,
		EndTime:                s.endTime,
		Status:                 s.status,
		StatusDescription:      s.statusDescription,
		Attributes:             s.attributes,
		Events:                 s.events,
		Links:                  s.links,
		Resource:               s.resource,
		InstrumentationLibrary: s.instrumentationLibrary,
	}
}

// tracer 追踪器实现
type tracer struct {
	name                   string
	version                string
	schemaURL              string
	attributes             map[string]interface{}
	provider               *tracerProvider
	instrumentationLibrary InstrumentationLibrary
}

// Start 开始新跨度
func (t *tracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	config := &spanConfig{
		kind:       SpanKindInternal,
		attributes: make(map[string]interface{}),
		links:      make([]Link, 0),
		startTime:  time.Now(),
	}

	// 应用选项
	for _, opt := range opts {
		opt.apply(config)
	}

	// 获取父跨度上下文
	parentSpanContext := SpanContextFromContext(ctx)
	var traceID TraceID
	var parentSpanID SpanID

	if parentSpanContext.IsValid() {
		traceID = parentSpanContext.TraceID
		parentSpanID = parentSpanContext.SpanID
	} else {
		traceID = generateTraceID()
	}

	// 生成新的跨度ID
	spanID := generateSpanID()

	// 创建跨度上下文
	spanContext := SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: 0x01, // 采样标志
		Remote:     false,
	}

	// 检查采样
	sampled := t.provider.sampler.ShouldSample(spanContext, name, config.kind, config.attributes, config.links)
	if !sampled {
		spanContext.TraceFlags = 0x00
	}

	// 创建跨度
	span := &span{
		spanContext:            spanContext,
		parentSpanID:           parentSpanID,
		name:                   name,
		kind:                   config.kind,
		startTime:              config.startTime,
		status:                 StatusUnset,
		attributes:             config.attributes,
		events:                 make([]Event, 0),
		links:                  config.links,
		resource:               t.provider.resource,
		instrumentationLibrary: t.instrumentationLibrary,
		tracer:                 t,
		recording:              sampled,
	}

	// 将跨度添加到上下文
	newCtx := ContextWithSpan(ctx, span)

	return newCtx, span
}

// addSpan 添加跨度到导出队列
func (t *tracer) addSpan(spanData SpanData) {
	if t.provider != nil && t.provider.processor != nil {
		t.provider.processor.OnEnd(spanData)
	}
}

// tracerProvider 追踪器提供者实现
type tracerProvider struct {
	config    *TracingConfig
	resource  Resource
	sampler   Sampler
	processor SpanProcessor
	tracers   map[string]*tracer
	mu        sync.RWMutex
	running   bool
	cancel    context.CancelFunc
	stats     *TracingStats
}

// TracingStats 追踪统计
type TracingStats struct {
	TotalSpans     int64 `json:"total_spans"`
	SampledSpans   int64 `json:"sampled_spans"`
	DroppedSpans   int64 `json:"dropped_spans"`
	ExportedSpans  int64 `json:"exported_spans"`
	FailedExports  int64 `json:"failed_exports"`
	ActiveSpans    int64 `json:"active_spans"`
	AverageLatency float64 `json:"average_latency_ms"`
}

// NewTracerProvider 创建追踪器提供者
func NewTracerProvider(config *TracingConfig, exporter SpanExporter) *tracerProvider {
	if config == nil {
		config = DefaultTracingConfig()
	}

	// 创建资源
	resource := Resource{
		Attributes: map[string]interface{}{
			"service.name":    config.ServiceName,
			"service.version": config.ServiceVersion,
			"environment":     config.Environment,
			"runtime.name":    "go",
			"runtime.version": runtime.Version(),
		},
	}

	// 添加自定义资源属性
	for k, v := range config.ResourceAttributes {
		resource.Attributes[k] = v
	}

	// 创建采样器
	sampler := NewTraceIDRatioBasedSampler(config.SampleRate)

	// 创建处理器
	processor := NewBatchSpanProcessor(exporter, config)

	return &tracerProvider{
		config:    config,
		resource:  resource,
		sampler:   sampler,
		processor: processor,
		tracers:   make(map[string]*tracer),
		stats:     &TracingStats{},
	}
}

// Tracer 获取追踪器
func (tp *tracerProvider) Tracer(name string, opts ...TracerOption) Tracer {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tracer, exists := tp.tracers[name]; exists {
		return tracer
	}

	config := &tracerConfig{
		attributes: make(map[string]interface{}),
	}

	// 应用选项
	for _, opt := range opts {
		opt.apply(config)
	}

	tracer := &tracer{
		name:       name,
		version:    config.version,
		schemaURL:  config.schemaURL,
		attributes: config.attributes,
		provider:   tp,
		instrumentationLibrary: InstrumentationLibrary{
			Name:      name,
			Version:   config.version,
			SchemaURL: config.schemaURL,
		},
	}

	tp.tracers[name] = tracer
	return tracer
}

// Start 启动追踪器提供者
func (tp *tracerProvider) Start(ctx context.Context) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.running {
		return fmt.Errorf("tracer provider already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	tp.cancel = cancel
	tp.running = true

	// 启动处理器
	if tp.processor != nil {
		if err := tp.processor.Start(ctx); err != nil {
			tp.running = false
			cancel()
			return err
		}
	}

	return nil
}

// Shutdown 关闭追踪器提供者
func (tp *tracerProvider) Shutdown(ctx context.Context) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if !tp.running {
		return fmt.Errorf("tracer provider not running")
	}

	tp.cancel()
	tp.running = false

	// 关闭处理器
	if tp.processor != nil {
		return tp.processor.Shutdown(ctx)
	}

	return nil
}

// GetStats 获取统计信息
func (tp *tracerProvider) GetStats() *TracingStats {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	return &TracingStats{
		TotalSpans:     atomic.LoadInt64(&tp.stats.TotalSpans),
		SampledSpans:   atomic.LoadInt64(&tp.stats.SampledSpans),
		DroppedSpans:   atomic.LoadInt64(&tp.stats.DroppedSpans),
		ExportedSpans:  atomic.LoadInt64(&tp.stats.ExportedSpans),
		FailedExports:  atomic.LoadInt64(&tp.stats.FailedExports),
		ActiveSpans:    atomic.LoadInt64(&tp.stats.ActiveSpans),
		AverageLatency: tp.stats.AverageLatency,
	}
}

// Sampler 采样器接口
type Sampler interface {
	// ShouldSample 是否应该采样
	ShouldSample(spanContext SpanContext, name string, kind SpanKind, attributes map[string]interface{}, links []Link) bool
}

// TraceIDRatioBasedSampler 基于追踪ID比率的采样器
type TraceIDRatioBasedSampler struct {
	ratio float64
}

// NewTraceIDRatioBasedSampler 创建基于追踪ID比率的采样器
func NewTraceIDRatioBasedSampler(ratio float64) *TraceIDRatioBasedSampler {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return &TraceIDRatioBasedSampler{ratio: ratio}
}

// ShouldSample 是否应该采样
func (s *TraceIDRatioBasedSampler) ShouldSample(spanContext SpanContext, name string, kind SpanKind, attributes map[string]interface{}, links []Link) bool {
	if s.ratio == 0 {
		return false
	}
	if s.ratio == 1 {
		return true
	}

	// 使用追踪ID的最后8字节计算采样
	traceIDBytes := spanContext.TraceID[:]
	var value uint64
	for i := 8; i < 16; i++ {
		value = value<<8 + uint64(traceIDBytes[i])
	}

	threshold := uint64(s.ratio * float64(^uint64(0)))
	return value < threshold
}

// SpanProcessor 跨度处理器接口
type SpanProcessor interface {
	// OnStart 跨度开始时调用
	OnStart(span SpanData)
	// OnEnd 跨度结束时调用
	OnEnd(span SpanData)
	// Start 启动处理器
	Start(ctx context.Context) error
	// Shutdown 关闭处理器
	Shutdown(ctx context.Context) error
}

// BatchSpanProcessor 批量跨度处理器
type BatchSpanProcessor struct {
	exporter     SpanExporter
	config       *TracingConfig
	spanQueue    chan SpanData
	batch        []SpanData
	mu           sync.Mutex
	running      bool
	cancel       context.CancelFunc
	stats        *ProcessorStats
}

// ProcessorStats 处理器统计
type ProcessorStats struct {
	QueuedSpans   int64 `json:"queued_spans"`
	ProcessedSpans int64 `json:"processed_spans"`
	DroppedSpans  int64 `json:"dropped_spans"`
	BatchCount    int64 `json:"batch_count"`
}

// NewBatchSpanProcessor 创建批量跨度处理器
func NewBatchSpanProcessor(exporter SpanExporter, config *TracingConfig) *BatchSpanProcessor {
	return &BatchSpanProcessor{
		exporter:  exporter,
		config:    config,
		spanQueue: make(chan SpanData, config.MaxSpans),
		batch:     make([]SpanData, 0, config.BatchSize),
		stats:     &ProcessorStats{},
	}
}

// OnStart 跨度开始时调用
func (bsp *BatchSpanProcessor) OnStart(span SpanData) {
	// 批量处理器在跨度开始时不需要处理
}

// OnEnd 跨度结束时调用
func (bsp *BatchSpanProcessor) OnEnd(span SpanData) {
	select {
	case bsp.spanQueue <- span:
		atomic.AddInt64(&bsp.stats.QueuedSpans, 1)
	default:
		// 队列满了，丢弃跨度
		atomic.AddInt64(&bsp.stats.DroppedSpans, 1)
	}
}

// Start 启动处理器
func (bsp *BatchSpanProcessor) Start(ctx context.Context) error {
	bsp.mu.Lock()
	defer bsp.mu.Unlock()

	if bsp.running {
		return fmt.Errorf("batch span processor already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	bsp.cancel = cancel
	bsp.running = true

	// 启动批量处理循环
	go bsp.batchLoop(ctx)

	return nil
}

// Shutdown 关闭处理器
func (bsp *BatchSpanProcessor) Shutdown(ctx context.Context) error {
	bsp.mu.Lock()
	defer bsp.mu.Unlock()

	if !bsp.running {
		return fmt.Errorf("batch span processor not running")
	}

	bsp.cancel()
	bsp.running = false

	// 导出剩余的跨度
	bsp.exportBatch(ctx)

	// 关闭导出器
	if bsp.exporter != nil {
		return bsp.exporter.Shutdown(ctx)
	}

	return nil
}

// batchLoop 批量处理循环
func (bsp *BatchSpanProcessor) batchLoop(ctx context.Context) {
	ticker := time.NewTicker(bsp.config.ExportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case span := <-bsp.spanQueue:
			bsp.addToBatch(span)
			if len(bsp.batch) >= bsp.config.BatchSize {
				bsp.exportBatch(ctx)
			}
		case <-ticker.C:
			if len(bsp.batch) > 0 {
				bsp.exportBatch(ctx)
			}
		}
	}
}

// addToBatch 添加到批次
func (bsp *BatchSpanProcessor) addToBatch(span SpanData) {
	bsp.mu.Lock()
	defer bsp.mu.Unlock()

	bsp.batch = append(bsp.batch, span)
}

// exportBatch 导出批次
func (bsp *BatchSpanProcessor) exportBatch(ctx context.Context) {
	bsp.mu.Lock()
	if len(bsp.batch) == 0 {
		bsp.mu.Unlock()
		return
	}

	batch := make([]SpanData, len(bsp.batch))
	copy(batch, bsp.batch)
	bsp.batch = bsp.batch[:0]
	bsp.mu.Unlock()

	if bsp.exporter != nil {
		if err := bsp.exporter.ExportSpans(ctx, batch); err != nil {
			// 导出失败，记录统计
			atomic.AddInt64(&bsp.stats.DroppedSpans, int64(len(batch)))
		} else {
			atomic.AddInt64(&bsp.stats.ProcessedSpans, int64(len(batch)))
		}
	}

	atomic.AddInt64(&bsp.stats.BatchCount, 1)
}

// GetStats 获取统计信息
func (bsp *BatchSpanProcessor) GetStats() *ProcessorStats {
	return &ProcessorStats{
		QueuedSpans:    atomic.LoadInt64(&bsp.stats.QueuedSpans),
		ProcessedSpans: atomic.LoadInt64(&bsp.stats.ProcessedSpans),
		DroppedSpans:   atomic.LoadInt64(&bsp.stats.DroppedSpans),
		BatchCount:     atomic.LoadInt64(&bsp.stats.BatchCount),
	}
}

// 上下文相关函数

type spanContextKey struct{}

// ContextWithSpan 将跨度添加到上下文
func ContextWithSpan(ctx context.Context, span Span) context.Context {
	return context.WithValue(ctx, spanContextKey{}, span)
}

// SpanFromContext 从上下文获取跨度
func SpanFromContext(ctx context.Context) Span {
	if span, ok := ctx.Value(spanContextKey{}).(Span); ok {
		return span
	}
	return &noopSpan{}
}

// SpanContextFromContext 从上下文获取跨度上下文
func SpanContextFromContext(ctx context.Context) SpanContext {
	if span := SpanFromContext(ctx); span != nil {
		return span.SpanContext()
	}
	return SpanContext{}
}

// noopSpan 空操作跨度
type noopSpan struct{}

func (n *noopSpan) End()                                                 {}
func (n *noopSpan) SetName(name string)                                 {}
func (n *noopSpan) SetStatus(status SpanStatus, description string)     {}
func (n *noopSpan) SetAttribute(key string, value interface{})          {}
func (n *noopSpan) SetAttributes(attributes map[string]interface{})     {}
func (n *noopSpan) AddEvent(name string, attributes map[string]interface{}) {}
func (n *noopSpan) RecordError(err error)                               {}
func (n *noopSpan) SpanContext() SpanContext                            { return SpanContext{} }
func (n *noopSpan) IsRecording() bool                                   { return false }

// 工具函数

// generateTraceID 生成追踪ID
func generateTraceID() TraceID {
	var traceID TraceID
	rand.Read(traceID[:])
	return traceID
}

// generateSpanID 生成跨度ID
func generateSpanID() SpanID {
	var spanID SpanID
	rand.Read(spanID[:])
	return spanID
}

// 跨度选项实现

type spanKindOption SpanKind

func (sko spanKindOption) apply(config *spanConfig) {
	config.kind = SpanKind(sko)
}

// WithSpanKind 设置跨度类型
func WithSpanKind(kind SpanKind) SpanOption {
	return spanKindOption(kind)
}

type attributesOption map[string]interface{}

func (ao attributesOption) apply(config *spanConfig) {
	for k, v := range ao {
		config.attributes[k] = v
	}
}

// WithAttributes 设置属性
func WithAttributes(attributes map[string]interface{}) SpanOption {
	return attributesOption(attributes)
}

type startTimeOption time.Time

func (sto startTimeOption) apply(config *spanConfig) {
	config.startTime = time.Time(sto)
}

// WithStartTime 设置开始时间
func WithStartTime(startTime time.Time) SpanOption {
	return startTimeOption(startTime)
}

// 追踪器选项实现

type versionOption string

func (vo versionOption) apply(config *tracerConfig) {
	config.version = string(vo)
}

// WithVersion 设置版本
func WithVersion(version string) TracerOption {
	return versionOption(version)
}

// HTTP中间件

// HTTPMiddleware HTTP追踪中间件
func HTTPMiddleware(tracer Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 从HTTP头部提取跨度上下文
			spanContext := extractSpanContextFromHTTP(r)
			ctx := r.Context()
			if spanContext.IsValid() {
				ctx = ContextWithSpan(ctx, &span{spanContext: spanContext})
			}

			// 开始新跨度
			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := tracer.Start(ctx, spanName, WithSpanKind(SpanKindServer))
			defer span.End()

			// 设置HTTP属性
			span.SetAttributes(map[string]interface{}{
				"http.method":      r.Method,
				"http.url":         r.URL.String(),
				"http.scheme":      r.URL.Scheme,
				"http.host":        r.Host,
				"http.target":      r.URL.Path,
				"http.user_agent": r.UserAgent(),
				"http.remote_addr": r.RemoteAddr,
			})

			// 包装响应写入器以捕获状态码
			wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: 200}

			// 将跨度上下文注入HTTP头部
			injectSpanContextToHTTP(span.SpanContext(), w.Header())

			// 调用下一个处理器
			next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

			// 设置响应属性
			span.SetAttribute("http.status_code", wrappedWriter.statusCode)

			// 设置状态
			if wrappedWriter.statusCode >= 400 {
				span.SetStatus(StatusError, fmt.Sprintf("HTTP %d", wrappedWriter.statusCode))
			} else {
				span.SetStatus(StatusOK, "")
			}
		})
	}
}

// responseWriter 响应写入器包装器
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// extractSpanContextFromHTTP 从HTTP头部提取跨度上下文
func extractSpanContextFromHTTP(r *http.Request) SpanContext {
	// 简化实现，实际应该支持标准的追踪头部格式
	traceID := r.Header.Get("X-Trace-Id")
	spanID := r.Header.Get("X-Span-Id")

	if traceID == "" || spanID == "" {
		return SpanContext{}
	}

	traceIDBytes, err := hex.DecodeString(traceID)
	if err != nil || len(traceIDBytes) != 16 {
		return SpanContext{}
	}

	spanIDBytes, err := hex.DecodeString(spanID)
	if err != nil || len(spanIDBytes) != 8 {
		return SpanContext{}
	}

	var traceIDArray TraceID
	var spanIDArray SpanID
	copy(traceIDArray[:], traceIDBytes)
	copy(spanIDArray[:], spanIDBytes)

	return SpanContext{
		TraceID:    traceIDArray,
		SpanID:     spanIDArray,
		TraceFlags: 0x01,
		Remote:     true,
	}
}

// injectSpanContextToHTTP 将跨度上下文注入HTTP头部
func injectSpanContextToHTTP(spanContext SpanContext, header http.Header) {
	if !spanContext.IsValid() {
		return
	}

	header.Set("X-Trace-Id", spanContext.TraceID.String())
	header.Set("X-Span-Id", spanContext.SpanID.String())
}

// 内置导出器

// ConsoleExporter 控制台导出器
type ConsoleExporter struct{}

// NewConsoleExporter 创建控制台导出器
func NewConsoleExporter() *ConsoleExporter {
	return &ConsoleExporter{}
}

// ExportSpans 导出跨度
func (ce *ConsoleExporter) ExportSpans(ctx context.Context, spans []SpanData) error {
	for _, span := range spans {
		fmt.Printf("[TRACE] %s - %s (%s) [%s] %v\n",
			span.SpanContext.TraceID.String(),
			span.Name,
			span.Kind.String(),
			span.Status.String(),
			span.EndTime.Sub(span.StartTime),
		)

		// 打印属性
		if len(span.Attributes) > 0 {
			var attrs []string
			for k, v := range span.Attributes {
				attrs = append(attrs, fmt.Sprintf("%s=%v", k, v))
			}
			sort.Strings(attrs)
			fmt.Printf("  Attributes: %s\n", strings.Join(attrs, ", "))
		}

		// 打印事件
		for _, event := range span.Events {
			fmt.Printf("  Event: %s at %s\n", event.Name, event.Timestamp.Format(time.RFC3339Nano))
		}
	}
	return nil
}

// Shutdown 关闭导出器
func (ce *ConsoleExporter) Shutdown(ctx context.Context) error {
	return nil
}