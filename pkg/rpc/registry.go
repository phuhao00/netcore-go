// Package rpc 服务注册发现实现
// Author: NetCore-Go Team
// Created: 2024

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ServiceRegistry 服务注册接口
type ServiceRegistry interface {
	Register(serviceName string, serviceInfo *ServiceInfo) error
	Unregister(serviceName string) error
	Discover(serviceName string) ([]*ServiceInstance, error)
	Watch(serviceName string) (<-chan []*ServiceInstance, error)
	Close() error
}

// ServiceInstance 服务实例
type ServiceInstance struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Metadata map[string]string `json:"metadata"`
	Health   HealthStatus      `json:"health"`
	Weight   int               `json:"weight"`
	Version  string            `json:"version"`
	Tags     []string          `json:"tags"`
	CreateTime time.Time       `json:"create_time"`
	UpdateTime time.Time       `json:"update_time"`
}

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// MemoryRegistry 内存服务注册中心
type MemoryRegistry struct {
	services  map[string][]*ServiceInstance
	watchers  map[string][]chan []*ServiceInstance
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewMemoryRegistry 创建内存服务注册中心
func NewMemoryRegistry() *MemoryRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &MemoryRegistry{
		services: make(map[string][]*ServiceInstance),
		watchers: make(map[string][]chan []*ServiceInstance),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Register 注册服务
func (r *MemoryRegistry) Register(serviceName string, serviceInfo *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	instance := &ServiceInstance{
		ID:         fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()),
		Name:       serviceName,
		Address:    "localhost", // 默认地址
		Port:       8080,        // 默认端口
		Metadata:   make(map[string]string),
		Health:     HealthStatusHealthy,
		Weight:     100,
		Version:    "1.0.0",
		Tags:       []string{},
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	
	// 添加方法信息到元数据
	methods := make([]string, 0, len(serviceInfo.Methods))
	for methodName := range serviceInfo.Methods {
		methods = append(methods, methodName)
	}
	methodsJSON, _ := json.Marshal(methods)
	instance.Metadata["methods"] = string(methodsJSON)
	instance.Metadata["type"] = serviceInfo.Type.String()
	
	r.services[serviceName] = append(r.services[serviceName], instance)
	
	// 通知观察者
	r.notifyWatchers(serviceName)
	
	return nil
}

// Unregister 注销服务
func (r *MemoryRegistry) Unregister(serviceName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.services, serviceName)
	
	// 通知观察者
	r.notifyWatchers(serviceName)
	
	return nil
}

// Discover 发现服务
func (r *MemoryRegistry) Discover(serviceName string) ([]*ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instances, exists := r.services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}
	
	// 过滤健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	for _, instance := range instances {
		if instance.Health == HealthStatusHealthy {
			healthyInstances = append(healthyInstances, instance)
		}
	}
	
	return healthyInstances, nil
}

// Watch 监听服务变化
func (r *MemoryRegistry) Watch(serviceName string) (<-chan []*ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	ch := make(chan []*ServiceInstance, 10)
	r.watchers[serviceName] = append(r.watchers[serviceName], ch)
	
	// 发送当前实例列表
	if instances, exists := r.services[serviceName]; exists {
		select {
		case ch <- instances:
		default:
		}
	}
	
	return ch, nil
}

// notifyWatchers 通知观察者
func (r *MemoryRegistry) notifyWatchers(serviceName string) {
	instances := r.services[serviceName]
	watchers := r.watchers[serviceName]
	
	for _, watcher := range watchers {
		select {
		case watcher <- instances:
		default:
			// 如果通道满了，跳过
		}
	}
}

// Close 关闭注册中心
func (r *MemoryRegistry) Close() error {
	r.cancel()
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// 关闭所有观察者通道
	for _, watchers := range r.watchers {
		for _, watcher := range watchers {
			close(watcher)
		}
	}
	
	r.watchers = make(map[string][]chan []*ServiceInstance)
	r.services = make(map[string][]*ServiceInstance)
	
	return nil
}

// EtcdRegistry Etcd服务注册中心
type EtcdRegistry struct {
	endpoints []string
	prefix    string
	ttl       int64
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewEtcdRegistry 创建Etcd服务注册中心
func NewEtcdRegistry(endpoints []string, prefix string, ttl int64) *EtcdRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &EtcdRegistry{
		endpoints: endpoints,
		prefix:    prefix,
		ttl:       ttl,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Register 注册服务到Etcd
func (r *EtcdRegistry) Register(serviceName string, serviceInfo *ServiceInfo) error {
	// 实现Etcd注册逻辑
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	
	if serviceInfo == nil {
		return fmt.Errorf("service info is required")
	}
	
	// 创建服务实例
	instance := &ServiceInstance{
		ID:       fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()),
		Name:     serviceName,
		Address:  "127.0.0.1", // 默认地址
		Port:     8080,        // 默认端口
		Tags:     []string{"rpc", "etcd"},
		Metadata: make(map[string]string),
		Health:   HealthStatusHealthy,
	}
	
	// 添加方法信息到元数据
	methods := make([]string, 0, len(serviceInfo.Methods))
	for methodName := range serviceInfo.Methods {
		methods = append(methods, methodName)
	}
	methodsJSON, _ := json.Marshal(methods)
	instance.Metadata["methods"] = string(methodsJSON)
	instance.Metadata["type"] = serviceInfo.Type.String()
	instance.Metadata["registry"] = "etcd"
	
	// 序列化实例信息
	instanceData, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal service instance: %w", err)
	}
	
	// 构建Etcd键
	key := fmt.Sprintf("%s/%s/%s", r.prefix, serviceName, instance.ID)
	
	// 模拟Etcd注册过程
	// 实际实现中，这里会调用etcd客户端API
	time.Sleep(100 * time.Millisecond)
	
	// 记录注册信息（模拟）
	fmt.Printf("[EtcdRegistry] Registered service: %s at key: %s, data: %s\n", serviceName, key, string(instanceData))
	
	return nil
}

// Unregister 从Etcd注销服务
func (r *EtcdRegistry) Unregister(serviceName string) error {
	// 实现Etcd注销逻辑
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	
	// 构建Etcd键前缀
	keyPrefix := fmt.Sprintf("%s/%s/", r.prefix, serviceName)
	
	// 模拟Etcd注销过程
	// 实际实现中，这里会调用etcd客户端API删除所有匹配的键
	time.Sleep(50 * time.Millisecond)
	
	// 记录注销信息（模拟）
	fmt.Printf("[EtcdRegistry] Unregistered service: %s with key prefix: %s\n", serviceName, keyPrefix)
	_ = keyPrefix // 避免未使用变量警告
	
	return nil
}

// Discover 从Etcd发现服务
func (r *EtcdRegistry) Discover(serviceName string) ([]*ServiceInstance, error) {
	// 实现Etcd服务发现逻辑
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	
	// 构建Etcd键前缀
	keyPrefix := fmt.Sprintf("%s/%s/", r.prefix, serviceName)
	_ = keyPrefix // 避免未使用变量警告
	
	// 模拟从Etcd获取服务实例
	// 实际实现中，这里会调用etcd客户端API获取所有匹配的键值对
	time.Sleep(50 * time.Millisecond)
	
	// 模拟返回服务实例
	instances := []*ServiceInstance{
		{
			ID:       fmt.Sprintf("%s-etcd-1", serviceName),
			Name:     serviceName,
			Address:  "127.0.0.1",
			Port:     8080,
			Tags:     []string{"rpc", "etcd"},
			Metadata: map[string]string{"registry": "etcd", "type": "grpc"},
			Health:   HealthStatusHealthy,
		},
		{
			ID:       fmt.Sprintf("%s-etcd-2", serviceName),
			Name:     serviceName,
			Address:  "127.0.0.1",
			Port:     8081,
			Tags:     []string{"rpc", "etcd"},
			Metadata: map[string]string{"registry": "etcd", "type": "grpc"},
			Health:   HealthStatusHealthy,
		},
	}
	
	fmt.Printf("[EtcdRegistry] Discovered %d instances for service: %s\n", len(instances), serviceName)
	
	return instances, nil
}

// Watch 监听Etcd服务变化
func (r *EtcdRegistry) Watch(serviceName string) (<-chan []*ServiceInstance, error) {
	// 实现Etcd监听逻辑
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	
	ch := make(chan []*ServiceInstance, 10)
	
	// 启动监听goroutine
	go func() {
		defer close(ch)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		// 发送初始服务列表
		if instances, err := r.Discover(serviceName); err == nil {
			select {
			case ch <- instances:
			case <-r.ctx.Done():
				return
			}
		}
		
		// 定期检查服务变化
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				// 获取最新的服务列表
				if instances, err := r.Discover(serviceName); err == nil {
					select {
					case ch <- instances:
					case <-r.ctx.Done():
						return
					}
				}
			}
		}
	}()
	
	fmt.Printf("[EtcdRegistry] Started watching service: %s\n", serviceName)
	
	return ch, nil
}

// Close 关闭Etcd注册中心
func (r *EtcdRegistry) Close() error {
	r.cancel()
	return nil
}

// ConsulRegistry Consul服务注册中心
type ConsulRegistry struct {
	address string
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewConsulRegistry 创建Consul服务注册中心
func NewConsulRegistry(address string) *ConsulRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConsulRegistry{
		address: address,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Register 注册服务到Consul
func (r *ConsulRegistry) Register(serviceName string, serviceInfo *ServiceInfo) error {
	// 实现Consul注册逻辑
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	
	if serviceInfo == nil {
		return fmt.Errorf("service info is required")
	}
	
	// 创建服务实例
	instance := &ServiceInstance{
		ID:       fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()),
		Name:     serviceName,
		Address:  "127.0.0.1", // 默认地址
		Port:     9090,        // 默认端口
		Tags:     []string{"rpc", "consul"},
		Metadata: make(map[string]string),
		Health:   HealthStatusHealthy,
	}
	
	// 添加方法信息到元数据
	methods := make([]string, 0, len(serviceInfo.Methods))
	for methodName := range serviceInfo.Methods {
		methods = append(methods, methodName)
	}
	methodsJSON, _ := json.Marshal(methods)
	instance.Metadata["methods"] = string(methodsJSON)
	instance.Metadata["type"] = serviceInfo.Type.String()
	instance.Metadata["registry"] = "consul"
	
	// 模拟Consul注册过程
	// 实际实现中，这里会调用Consul API注册服务
	time.Sleep(100 * time.Millisecond)
	
	// 记录注册信息（模拟）
	instanceData, _ := json.Marshal(instance)
	fmt.Printf("[ConsulRegistry] Registered service: %s at %s, data: %s\n", serviceName, r.address, string(instanceData))
	
	return nil
}

// Unregister 从Consul注销服务
func (r *ConsulRegistry) Unregister(serviceName string) error {
	// 实现Consul注销逻辑
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	
	// 模拟Consul注销过程
	// 实际实现中，这里会调用Consul API注销服务
	time.Sleep(50 * time.Millisecond)
	
	// 记录注销信息（模拟）
	fmt.Printf("[ConsulRegistry] Unregistered service: %s from %s\n", serviceName, r.address)
	
	return nil
}

// Discover 从Consul发现服务
func (r *ConsulRegistry) Discover(serviceName string) ([]*ServiceInstance, error) {
	// 实现Consul服务发现逻辑
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	
	// 模拟从Consul获取服务实例
	// 实际实现中，这里会调用Consul API获取健康的服务实例
	time.Sleep(50 * time.Millisecond)
	
	// 模拟返回服务实例
	instances := []*ServiceInstance{
		{
			ID:       fmt.Sprintf("%s-consul-1", serviceName),
			Name:     serviceName,
			Address:  "127.0.0.1",
			Port:     9090,
			Tags:     []string{"rpc", "consul"},
			Metadata: map[string]string{"registry": "consul", "type": "grpc"},
			Health:   HealthStatusHealthy,
		},
		{
			ID:       fmt.Sprintf("%s-consul-2", serviceName),
			Name:     serviceName,
			Address:  "127.0.0.1",
			Port:     9091,
			Tags:     []string{"rpc", "consul"},
			Metadata: map[string]string{"registry": "consul", "type": "grpc"},
			Health:   HealthStatusHealthy,
		},
	}
	
	fmt.Printf("[ConsulRegistry] Discovered %d instances for service: %s\n", len(instances), serviceName)
	
	return instances, nil
}

// Watch 监听Consul服务变化
func (r *ConsulRegistry) Watch(serviceName string) (<-chan []*ServiceInstance, error) {
	// 实现Consul监听逻辑
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	
	ch := make(chan []*ServiceInstance, 10)
	
	// 启动监听goroutine
	go func() {
		defer close(ch)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		// 发送初始服务列表
		if instances, err := r.Discover(serviceName); err == nil {
			select {
			case ch <- instances:
			case <-r.ctx.Done():
				return
			}
		}
		
		// 定期检查服务变化
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				// 获取最新的服务列表
				if instances, err := r.Discover(serviceName); err == nil {
					select {
					case ch <- instances:
					case <-r.ctx.Done():
						return
					}
				}
			}
		}
	}()
	
	fmt.Printf("[ConsulRegistry] Started watching service: %s\n", serviceName)
	
	return ch, nil
}

// Close 关闭Consul注册中心
func (r *ConsulRegistry) Close() error {
	r.cancel()
	return nil
}