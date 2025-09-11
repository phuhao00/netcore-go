// Package rpc RPC客户端实现
// Author: NetCore-Go Team
// Created: 2024

package rpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/pool"
)

// RPCClient RPC客户端
type RPCClient struct {
	address     string
	conn        *RPCConnection
	codec       Codec
	interceptor Interceptor
	registry    ServiceRegistry
	loadBalancer LoadBalancer
	connPool    *pool.ConnectionPool
	requestID   int64
	mu          sync.RWMutex
	closed      bool
	timeout     time.Duration
	retryCount  int
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Select(instances []*ServiceInstance) (*ServiceInstance, error)
}

// RoundRobinLoadBalancer 轮询负载均衡器
type RoundRobinLoadBalancer struct {
	counter int64
}

// NewRoundRobinLoadBalancer 创建轮询负载均衡器
func NewRoundRobinLoadBalancer() *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{}
}

// Select 选择服务实例
func (lb *RoundRobinLoadBalancer) Select(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no available instances")
	}
	
	index := atomic.AddInt64(&lb.counter, 1) % int64(len(instances))
	return instances[index], nil
}

// RandomLoadBalancer 随机负载均衡器
type RandomLoadBalancer struct{}

// NewRandomLoadBalancer 创建随机负载均衡器
func NewRandomLoadBalancer() *RandomLoadBalancer {
	return &RandomLoadBalancer{}
}

// Select 选择服务实例
func (lb *RandomLoadBalancer) Select(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no available instances")
	}
	
	index := time.Now().UnixNano() % int64(len(instances))
	return instances[index], nil
}

// NewRPCClient 创建RPC客户端
func NewRPCClient(address string, options ...ClientOption) *RPCClient {
	client := &RPCClient{
		address:      address,
		codec:        &JSONCodec{},
		loadBalancer: NewRoundRobinLoadBalancer(),
		timeout:      30 * time.Second,
		retryCount:   3,
	}
	
	// 应用配置选项
	for _, option := range options {
		option(client)
	}
	
	return client
}

// ClientOption 客户端配置选项
type ClientOption func(*RPCClient)

// WithCodec 设置编解码器
func WithCodec(codec Codec) ClientOption {
	return func(client *RPCClient) {
		client.codec = codec
	}
}

// WithInterceptor 设置拦截器
func WithInterceptor(interceptor Interceptor) ClientOption {
	return func(client *RPCClient) {
		client.interceptor = interceptor
	}
}

// WithRegistry 设置服务注册中心
func WithRegistry(registry ServiceRegistry) ClientOption {
	return func(client *RPCClient) {
		client.registry = registry
	}
}

// WithLoadBalancer 设置负载均衡器
func WithLoadBalancer(loadBalancer LoadBalancer) ClientOption {
	return func(client *RPCClient) {
		client.loadBalancer = loadBalancer
	}
}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(client *RPCClient) {
		client.timeout = timeout
	}
}

// WithRetryCount 设置重试次数
func WithRetryCount(retryCount int) ClientOption {
	return func(client *RPCClient) {
		client.retryCount = retryCount
	}
}

// Connect 连接到服务器
func (c *RPCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil && c.conn.IsActive() {
		return nil // 已经连接
	}
	
	conn, err := net.DialTimeout("tcp", c.address, c.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", c.address, err)
	}
	
	c.conn = &RPCConnection{
		conn:   conn,
		id:     generateConnectionID(),
		context: make(map[string]interface{}),
	}
	
	return nil
}

// Call 同步调用RPC方法
func (c *RPCClient) Call(ctx context.Context, serviceName, methodName string, args interface{}, reply interface{}) error {
	return c.CallWithMetadata(ctx, serviceName, methodName, args, reply, nil)
}

// CallWithMetadata 带元数据的同步调用
func (c *RPCClient) CallWithMetadata(ctx context.Context, serviceName, methodName string, args interface{}, reply interface{}, metadata map[string]string) error {
	var lastErr error
	
	// 重试逻辑
	for i := 0; i <= c.retryCount; i++ {
		err := c.doCall(ctx, serviceName, methodName, args, reply, metadata)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
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

// doCall 执行RPC调用
func (c *RPCClient) doCall(ctx context.Context, serviceName, methodName string, args interface{}, reply interface{}, metadata map[string]string) error {
	// 如果配置了服务注册中心，使用服务发现
	if c.registry != nil {
		instances, err := c.registry.Discover(serviceName)
		if err != nil {
			return fmt.Errorf("service discovery failed: %v", err)
		}
		
		instance, err := c.loadBalancer.Select(instances)
		if err != nil {
			return fmt.Errorf("load balancer selection failed: %v", err)
		}
		
		// 更新地址
		c.address = fmt.Sprintf("%s:%d", instance.Address, instance.Port)
	}
	
	// 确保连接
	if err := c.Connect(); err != nil {
		return err
	}
	
	// 生成请求ID
	requestID := fmt.Sprintf("%d", atomic.AddInt64(&c.requestID, 1))
	
	// 创建请求
	request := &RPCRequest{
		ID:       requestID,
		Service:  serviceName,
		Method:   methodName,
		Args:     args,
		Metadata: metadata,
	}
	
	// 应用拦截器
	if c.interceptor != nil {
		ctx = c.interceptor.Before(ctx, request)
	}
	
	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	
	// 发送请求
	if err := c.conn.WriteRequest(request); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	
	// 接收响应
	response, err := c.conn.ReadResponse()
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	// 应用拦截器
	if c.interceptor != nil {
		c.interceptor.After(ctx, request, response)
	}
	
	// 检查响应错误
	if response.Error != "" {
		return fmt.Errorf("RPC error: %s", response.Error)
	}
	
	// 解码响应结果
	if err := c.codec.Decode(response.Result, reply); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}
	
	return nil
}

// AsyncCall 异步调用RPC方法
func (c *RPCClient) AsyncCall(ctx context.Context, serviceName, methodName string, args interface{}, reply interface{}) <-chan error {
	return c.AsyncCallWithMetadata(ctx, serviceName, methodName, args, reply, nil)
}

// AsyncCallWithMetadata 带元数据的异步调用
func (c *RPCClient) AsyncCallWithMetadata(ctx context.Context, serviceName, methodName string, args interface{}, reply interface{}, metadata map[string]string) <-chan error {
	resultChan := make(chan error, 1)
	
	go func() {
		defer close(resultChan)
		err := c.CallWithMetadata(ctx, serviceName, methodName, args, reply, metadata)
		resultChan <- err
	}()
	
	return resultChan
}

// Close 关闭客户端
func (c *RPCClient) Close() error {
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
func (c *RPCClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.conn != nil && c.conn.IsActive()
}

// WriteRequest 写入请求（连接方法）
func (c *RPCConnection) WriteRequest(request *RPCRequest) error {
	// 序列化请求
	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// 写入请求长度
	length := len(requestData)
	lengthBytes := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	
	if _, err := c.conn.Write(lengthBytes); err != nil {
		return err
	}
	
	// 写入请求数据
	_, err = c.conn.Write(requestData)
	return err
}

// ReadResponse 读取响应（连接方法）
func (c *RPCConnection) ReadResponse() (*RPCResponse, error) {
	// 读取响应长度
	lengthBytes := make([]byte, 4)
	if _, err := c.conn.Read(lengthBytes); err != nil {
		return nil, err
	}
	
	length := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])
	if length <= 0 || length > 1024*1024 { // 限制最大1MB
		return nil, fmt.Errorf("invalid response length: %d", length)
	}
	
	// 读取响应数据
	responseData := make([]byte, length)
	if _, err := c.conn.Read(responseData); err != nil {
		return nil, err
	}
	
	// 解析响应
	var response RPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	
	return &response, nil
}



