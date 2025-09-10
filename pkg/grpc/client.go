// Package grpc gRPC客户端实现
// Author: NetCore-Go Team
// Created: 2024

package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCClient gRPC客户端
type GRPCClient struct {
	conn         *grpc.ClientConn
	address      string
	timeout      time.Duration
	retryCount   int
	interceptors []grpc.UnaryClientInterceptor
	streamInterceptors []grpc.StreamClientInterceptor
	mu           sync.RWMutex
	closed       bool
	metadata     map[string]string
}

// NewGRPCClient 创建gRPC客户端
func NewGRPCClient(address string, options ...ClientOption) *GRPCClient {
	client := &GRPCClient{
		address:    address,
		timeout:    30 * time.Second,
		retryCount: 3,
		metadata:   make(map[string]string),
	}
	
	// 应用配置选项
	for _, option := range options {
		option(client)
	}
	
	return client
}

// ClientOption 客户端配置选项
type ClientOption func(*GRPCClient)

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(client *GRPCClient) {
		client.timeout = timeout
	}
}

// WithRetryCount 设置重试次数
func WithRetryCount(retryCount int) ClientOption {
	return func(client *GRPCClient) {
		client.retryCount = retryCount
	}
}

// WithUnaryInterceptor 添加一元拦截器
func WithUnaryInterceptor(interceptor grpc.UnaryClientInterceptor) ClientOption {
	return func(client *GRPCClient) {
		client.interceptors = append(client.interceptors, interceptor)
	}
}

// WithStreamInterceptor 添加流拦截器
func WithStreamInterceptor(interceptor grpc.StreamClientInterceptor) ClientOption {
	return func(client *GRPCClient) {
		client.streamInterceptors = append(client.streamInterceptors, interceptor)
	}
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]string) ClientOption {
	return func(client *GRPCClient) {
		client.metadata = metadata
	}
}

// Connect 连接到gRPC服务器
func (c *GRPCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		return nil // 已经连接
	}
	
	// 创建连接选项
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(c.timeout),
	}
	
	// 添加拦截器
	if len(c.interceptors) > 0 {
		opts = append(opts, grpc.WithChainUnaryInterceptor(c.interceptors...))
	}
	if len(c.streamInterceptors) > 0 {
		opts = append(opts, grpc.WithChainStreamInterceptor(c.streamInterceptors...))
	}
	
	// 添加默认拦截器
	defaultInterceptors := []grpc.UnaryClientInterceptor{
		c.loggingInterceptor,
		c.retryInterceptor,
		c.metadataInterceptor,
	}
	opts = append(opts, grpc.WithChainUnaryInterceptor(defaultInterceptors...))
	
	// 建立连接
	conn, err := grpc.Dial(c.address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", c.address, err)
	}
	
	c.conn = conn
	return nil
}

// GetConnection 获取gRPC连接
func (c *GRPCClient) GetConnection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// Close 关闭客户端
func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	
	if c.conn != nil {
		return c.conn.Close()
	}
	
	return nil
}

// IsConnected 检查是否已连接
func (c *GRPCClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn == nil {
		return false
	}
	
	state := c.conn.GetState()
	return state == grpc.Ready || state == grpc.Idle
}

// SetMetadata 设置请求元数据
func (c *GRPCClient) SetMetadata(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata 获取元数据
func (c *GRPCClient) GetMetadata() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]string)
	for k, v := range c.metadata {
		result[k] = v
	}
	return result
}

// loggingInterceptor 日志拦截器
func (c *GRPCClient) loggingInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	start := time.Now()
	
	fmt.Printf("[gRPC Client] Request started: method=%s\n", method)
	
	err := invoker(ctx, method, req, reply, cc, opts...)
	
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("[gRPC Client] Request failed: method=%s, duration=%v, error=%v\n", method, duration, err)
	} else {
		fmt.Printf("[gRPC Client] Request completed: method=%s, duration=%v\n", method, duration)
	}
	
	return err
}

// retryInterceptor 重试拦截器
func (c *GRPCClient) retryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var lastErr error
	
	for i := 0; i <= c.retryCount; i++ {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// 检查是否应该重试
		if !shouldRetry(err) {
			break
		}
		
		// 如果是最后一次重试，直接返回错误
		if i == c.retryCount {
			break
		}
		
		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * time.Second):
			// 继续重试
		}
	}
	
	return lastErr
}

// metadataInterceptor 元数据拦截器
func (c *GRPCClient) metadataInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	// 添加客户端元数据
	md := metadata.New(c.GetMetadata())
	ctx = metadata.NewOutgoingContext(ctx, md)
	
	return invoker(ctx, method, req, reply, cc, opts...)
}

// shouldRetry 判断是否应该重试
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	
	// 获取gRPC状态码
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	
	// 只对特定错误码进行重试
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

// AuthClientInterceptor 认证客户端拦截器
func AuthClientInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 添加认证令牌到元数据
		md := metadata.Pairs("authorization", token)
		ctx = metadata.NewOutgoingContext(ctx, md)
		
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// MetricsClientInterceptor 指标客户端拦截器
func MetricsClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		
		err := invoker(ctx, method, req, reply, cc, opts...)
		
		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}
		
		// TODO: 集成实际的指标收集系统
		fmt.Printf("[gRPC Client Metrics] method=%s, status=%s, duration=%v\n", method, status, duration)
		
		return err
	}
}

// TimeoutClientInterceptor 超时客户端拦截器
func TimeoutClientInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 设置超时
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// CircuitBreakerClientInterceptor 熔断器客户端拦截器
func CircuitBreakerClientInterceptor(failureThreshold int, resetTimeout time.Duration) grpc.UnaryClientInterceptor {
	type circuitBreaker struct {
		failureCount int
		lastFailure  time.Time
		state        string // "closed", "open", "half-open"
		mu           sync.Mutex
	}
	
	cb := &circuitBreaker{state: "closed"}
	
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		
		// 检查熔断器状态
		switch cb.state {
		case "open":
			// 检查是否可以进入半开状态
			if time.Since(cb.lastFailure) > resetTimeout {
				cb.state = "half-open"
			} else {
				return status.Errorf(codes.Unavailable, "circuit breaker is open")
			}
		case "half-open":
			// 半开状态，允许一个请求通过
		}
		
		// 执行请求
		err := invoker(ctx, method, req, reply, cc, opts...)
		
		if err != nil {
			cb.failureCount++
			cb.lastFailure = time.Now()
			
			if cb.failureCount >= failureThreshold {
				cb.state = "open"
			}
		} else {
			// 成功，重置计数器
			cb.failureCount = 0
			cb.state = "closed"
		}
		
		return err
	}
}