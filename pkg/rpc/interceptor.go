// Package rpc 拦截器实现
// Author: NetCore-Go Team
// Created: 2024

package rpc

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Interceptor 拦截器接口
type Interceptor interface {
	Before(ctx context.Context, request *RPCRequest) context.Context
	After(ctx context.Context, request *RPCRequest, response *RPCResponse)
}

// LoggingInterceptor 日志拦截器
type LoggingInterceptor struct {
	logger Logger
}

// Logger 日志接口
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// DefaultLogger 默认日志实现
type DefaultLogger struct{}

// Info 信息日志
func (l *DefaultLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}

// Error 错误日志
func (l *DefaultLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}

// Debug 调试日志
func (l *DefaultLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

// NewLoggingInterceptor 创建日志拦截器
func NewLoggingInterceptor(logger Logger) *LoggingInterceptor {
	if logger == nil {
		logger = &DefaultLogger{}
	}
	return &LoggingInterceptor{
		logger: logger,
	}
}

// Before 请求前处理
func (i *LoggingInterceptor) Before(ctx context.Context, request *RPCRequest) context.Context {
	startTime := time.Now()
	ctx = context.WithValue(ctx, "start_time", startTime)
	
	i.logger.Info("RPC request started",
		"id", request.ID,
		"service", request.Service,
		"method", request.Method,
		"timestamp", startTime,
	)
	
	return ctx
}

// After 请求后处理
func (i *LoggingInterceptor) After(ctx context.Context, request *RPCRequest, response *RPCResponse) {
	startTime, ok := ctx.Value("start_time").(time.Time)
	if !ok {
		startTime = time.Now()
	}
	
	duration := time.Since(startTime)
	
	if response.Error != "" {
		i.logger.Error("RPC request failed",
			"id", request.ID,
			"service", request.Service,
			"method", request.Method,
			"error", response.Error,
			"duration", duration,
		)
	} else {
		i.logger.Info("RPC request completed",
			"id", request.ID,
			"service", request.Service,
			"method", request.Method,
			"duration", duration,
		)
	}
}

// MetricsInterceptor 指标拦截器
type MetricsInterceptor struct {
	metrics MetricsCollector
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// DefaultMetricsCollector 默认指标收集器
type DefaultMetricsCollector struct{}

// IncCounter 增加计数器
func (m *DefaultMetricsCollector) IncCounter(name string, labels map[string]string) {
	// TODO: 实现指标收集
	log.Printf("[METRICS] Counter %s incremented with labels %v", name, labels)
}

// ObserveHistogram 观察直方图
func (m *DefaultMetricsCollector) ObserveHistogram(name string, value float64, labels map[string]string) {
	// TODO: 实现指标收集
	log.Printf("[METRICS] Histogram %s observed value %f with labels %v", name, value, labels)
}

// SetGauge 设置仪表盘
func (m *DefaultMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	// TODO: 实现指标收集
	log.Printf("[METRICS] Gauge %s set to %f with labels %v", name, value, labels)
}

// NewMetricsInterceptor 创建指标拦截器
func NewMetricsInterceptor(metrics MetricsCollector) *MetricsInterceptor {
	if metrics == nil {
		metrics = &DefaultMetricsCollector{}
	}
	return &MetricsInterceptor{
		metrics: metrics,
	}
}

// Before 请求前处理
func (i *MetricsInterceptor) Before(ctx context.Context, request *RPCRequest) context.Context {
	labels := map[string]string{
		"service": request.Service,
		"method":  request.Method,
	}
	
	i.metrics.IncCounter("rpc_requests_total", labels)
	return context.WithValue(ctx, "metrics_labels", labels)
}

// After 请求后处理
func (i *MetricsInterceptor) After(ctx context.Context, request *RPCRequest, response *RPCResponse) {
	labels, ok := ctx.Value("metrics_labels").(map[string]string)
	if !ok {
		labels = map[string]string{
			"service": request.Service,
			"method":  request.Method,
		}
	}
	
	// 记录响应状态
	if response.Error != "" {
		labels["status"] = "error"
		i.metrics.IncCounter("rpc_requests_failed_total", labels)
	} else {
		labels["status"] = "success"
		i.metrics.IncCounter("rpc_requests_success_total", labels)
	}
	
	// 记录请求耗时
	if startTime, ok := ctx.Value("start_time").(time.Time); ok {
		duration := time.Since(startTime).Seconds()
		i.metrics.ObserveHistogram("rpc_request_duration_seconds", duration, labels)
	}
}

// AuthInterceptor 认证拦截器
type AuthInterceptor struct {
	authenticator Authenticator
}

// Authenticator 认证器接口
type Authenticator interface {
	Authenticate(ctx context.Context, metadata map[string]string) (context.Context, error)
}

// TokenAuthenticator Token认证器
type TokenAuthenticator struct {
	validTokens map[string]bool
}

// NewTokenAuthenticator 创建Token认证器
func NewTokenAuthenticator(validTokens []string) *TokenAuthenticator {
	tokens := make(map[string]bool)
	for _, token := range validTokens {
		tokens[token] = true
	}
	return &TokenAuthenticator{
		validTokens: tokens,
	}
}

// Authenticate 认证
func (a *TokenAuthenticator) Authenticate(ctx context.Context, metadata map[string]string) (context.Context, error) {
	token, exists := metadata["authorization"]
	if !exists {
		return ctx, fmt.Errorf("missing authorization token")
	}
	
	if !a.validTokens[token] {
		return ctx, fmt.Errorf("invalid authorization token")
	}
	
	return context.WithValue(ctx, "user_token", token), nil
}

// NewAuthInterceptor 创建认证拦截器
func NewAuthInterceptor(authenticator Authenticator) *AuthInterceptor {
	return &AuthInterceptor{
		authenticator: authenticator,
	}
}

// Before 请求前处理
func (i *AuthInterceptor) Before(ctx context.Context, request *RPCRequest) context.Context {
	if i.authenticator != nil {
		authCtx, err := i.authenticator.Authenticate(ctx, request.Metadata)
		if err != nil {
			// 认证失败，在上下文中标记错误
			return context.WithValue(ctx, "auth_error", err)
		}
		return authCtx
	}
	return ctx
}

// After 请求后处理
func (i *AuthInterceptor) After(ctx context.Context, request *RPCRequest, response *RPCResponse) {
	if authError, ok := ctx.Value("auth_error").(error); ok {
		response.Error = authError.Error()
		response.Result = nil
	}
}

// ChainInterceptor 拦截器链
type ChainInterceptor struct {
	interceptors []Interceptor
}

// NewChainInterceptor 创建拦截器链
func NewChainInterceptor(interceptors ...Interceptor) *ChainInterceptor {
	return &ChainInterceptor{
		interceptors: interceptors,
	}
}

// Before 请求前处理
func (c *ChainInterceptor) Before(ctx context.Context, request *RPCRequest) context.Context {
	for _, interceptor := range c.interceptors {
		ctx = interceptor.Before(ctx, request)
	}
	return ctx
}

// After 请求后处理
func (c *ChainInterceptor) After(ctx context.Context, request *RPCRequest, response *RPCResponse) {
	// 逆序执行After方法
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		c.interceptors[i].After(ctx, request, response)
	}
}