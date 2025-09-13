// Package etcd etcd服务发现实现
// Author: NetCore-Go Team
// Created: 2024

package etcd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/api/v3/mvccpb"

	"github.com/netcore-go/pkg/discovery"
)

// EtcdDiscovery etcd服务发现
type EtcdDiscovery struct {
	mu       sync.RWMutex
	client   *v3.Client
	config   *EtcdConfig
	services map[string]*discovery.ServiceInstance
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *EtcdStats
	leases   map[string]v3.LeaseID
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	// etcd连接配置
	Endpoints   []string      `json:"endpoints"`
	DialTimeout time.Duration `json:"dial_timeout"`
	Username    string        `json:"username"`
	Password    string        `json:"password"`

	// TLS配置
	TLSConfig *tls.Config `json:"tls_config"`

	// 服务注册配置
	ServicePrefix   string            `json:"service_prefix"`
	ServiceTTL      time.Duration     `json:"service_ttl"`
	ServiceMeta     map[string]string `json:"service_meta"`
	KeepAliveTime   time.Duration     `json:"keep_alive_time"`
	KeepAliveTimeout time.Duration    `json:"keep_alive_timeout"`

	// 发现配置
	WatchEnabled    bool          `json:"watch_enabled"`
	WatchServices   []string      `json:"watch_services"`
	RefreshInterval time.Duration `json:"refresh_interval"`

	// 重试配置
	MaxRetries    int           `json:"max_retries"`
	RetryInterval time.Duration `json:"retry_interval"`

	// 压缩配置
	AutoCompactionRetention time.Duration `json:"auto_compaction_retention"`
	AutoCompactionMode      string        `json:"auto_compaction_mode"`
}

// EtcdStats etcd统计信息
type EtcdStats struct {
	TotalServices       int64 `json:"total_services"`
	ActiveServices      int64 `json:"active_services"`
	RegistrationCount   int64 `json:"registration_count"`
	DeregistrationCount int64 `json:"deregistration_count"`
	QueryCount          int64 `json:"query_count"`
	WatchCount          int64 `json:"watch_count"`
	ErrorCount          int64 `json:"error_count"`
	LeaseCount          int64 `json:"lease_count"`
	KeepAliveCount      int64 `json:"keep_alive_count"`
	LastUpdateTime      int64 `json:"last_update_time"`
}

// DefaultEtcdConfig 返回默认etcd配置
func DefaultEtcdConfig() *EtcdConfig {
	return &EtcdConfig{
		Endpoints:               []string{"127.0.0.1:2379"},
		DialTimeout:             5 * time.Second,
		ServicePrefix:           "/netcore-go/services",
		ServiceTTL:              30 * time.Second,
		ServiceMeta:             make(map[string]string),
		KeepAliveTime:           10 * time.Second,
		KeepAliveTimeout:        3 * time.Second,
		WatchEnabled:            true,
		RefreshInterval:         30 * time.Second,
		MaxRetries:              3,
		RetryInterval:           5 * time.Second,
		AutoCompactionRetention: 1 * time.Hour,
		AutoCompactionMode:      "periodic",
	}
}

// NewEtcdDiscovery 创建etcd服务发现
func NewEtcdDiscovery(config *EtcdConfig) (*EtcdDiscovery, error) {
	if config == nil {
		config = DefaultEtcdConfig()
	}

	// 创建etcd客户端配置
	clientConfig := v3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: config.DialTimeout,
		Username:    config.Username,
		Password:    config.Password,
	}

	if config.TLSConfig != nil {
		clientConfig.TLS = config.TLSConfig
	}

	// 创建etcd客户端
	client, err := v3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EtcdDiscovery{
		client:   client,
		config:   config,
		services: make(map[string]*discovery.ServiceInstance),
		ctx:      ctx,
		cancel:   cancel,
		stats:    &EtcdStats{},
		leases:   make(map[string]v3.LeaseID),
	}, nil
}

// Start 启动etcd服务发现
func (e *EtcdDiscovery) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("etcd discovery is already running")
	}

	// 测试连接
	if err := e.testConnection(); err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}

	e.running = true

	// 启动服务监控
	if e.config.WatchEnabled {
		go e.watchServices()
	}

	// 启动定期刷新
	go e.refreshLoop()

	return nil
}

// Stop 停止etcd服务发现
func (e *EtcdDiscovery) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.running = false
	e.cancel()

	// 撤销所有租约
	for serviceID, leaseID := range e.leases {
		if _, err := e.client.Revoke(context.Background(), leaseID); err != nil {
			fmt.Printf("Failed to revoke lease for service %s: %v\n", serviceID, err)
		}
	}

	// 关闭客户端
	return e.client.Close()
}

// Register 注册服务
func (e *EtcdDiscovery) Register(service *discovery.ServiceInstance) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("etcd discovery is not running")
	}

	// 创建租约
	leaseResp, err := e.client.Grant(context.Background(), int64(e.config.ServiceTTL.Seconds()))
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to create lease: %w", err)
	}

	// 序列化服务信息
	serviceData, err := json.Marshal(service)
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to marshal service info: %w", err)
	}

	// 构建服务键
	serviceKey := e.buildServiceKey(service.Name, service.ID)

	// 注册服务
	_, err = e.client.Put(context.Background(), serviceKey, string(serviceData), v3.WithLease(leaseResp.ID))
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 启动租约续期
	keepAliveCh, err := e.client.KeepAlive(context.Background(), leaseResp.ID)
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to keep alive lease: %w", err)
	}

	// 处理续期响应
	go func() {
		for ka := range keepAliveCh {
			if ka != nil {
				e.mu.Lock()
				e.stats.KeepAliveCount++
				e.mu.Unlock()
			}
		}
	}()

	e.services[service.ID] = service
	e.leases[service.ID] = leaseResp.ID
	e.stats.RegistrationCount++
	e.stats.TotalServices++
	e.stats.LeaseCount++

	return nil
}

// Deregister 注销服务
func (e *EtcdDiscovery) Deregister(serviceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("etcd discovery is not running")
	}

	service, exists := e.services[serviceID]
	if !exists {
		return fmt.Errorf("service %s not found", serviceID)
	}

	// 构建服务键
	serviceKey := e.buildServiceKey(service.Name, serviceID)

	// 删除服务
	_, err := e.client.Delete(context.Background(), serviceKey)
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	// 撤销租约
	if leaseID, exists := e.leases[serviceID]; exists {
		if _, err := e.client.Revoke(context.Background(), leaseID); err != nil {
			fmt.Printf("Failed to revoke lease for service %s: %v\n", serviceID, err)
		}
		delete(e.leases, serviceID)
		e.stats.LeaseCount--
	}

	delete(e.services, serviceID)
	e.stats.DeregistrationCount++
	e.stats.TotalServices--

	return nil
}

// Discover 发现服务
func (e *EtcdDiscovery) Discover(serviceName string) ([]*discovery.ServiceInstance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.running {
		return nil, fmt.Errorf("etcd discovery is not running")
	}

	// 构建服务前缀
	servicePrefix := e.buildServicePrefix(serviceName)

	// 查询服务
	resp, err := e.client.Get(context.Background(), servicePrefix, v3.WithPrefix())
	if err != nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	e.stats.QueryCount++

	// 解析服务信息
	var services []*discovery.ServiceInstance
	for _, kv := range resp.Kvs {
		var service discovery.ServiceInstance
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			e.stats.ErrorCount++
			continue
		}
		services = append(services, &service)
	}

	return services, nil
}

// Watch 监控服务变化
func (e *EtcdDiscovery) Watch(serviceName string, callback func([]*discovery.ServiceInstance)) error {
	if !e.running {
		return fmt.Errorf("etcd discovery is not running")
	}

	go func() {
		// 构建服务前缀
		servicePrefix := e.buildServicePrefix(serviceName)

		// 创建监控
		watchCh := e.client.Watch(e.ctx, servicePrefix, v3.WithPrefix())

		for watchResp := range watchCh {
			if watchResp.Err() != nil {
				e.mu.Lock()
				e.stats.ErrorCount++
				e.mu.Unlock()
				continue
			}

			e.mu.Lock()
			e.stats.WatchCount++
			e.mu.Unlock()

			// 处理事件
			for _, event := range watchResp.Events {
				switch event.Type {
				case mvccpb.PUT:
					// 服务注册或更新
					var service discovery.ServiceInstance
					if err := json.Unmarshal(event.Kv.Value, &service); err != nil {
						continue
					}
					// 获取当前所有服务并调用回调
					if services, err := e.Discover(serviceName); err == nil {
						callback(services)
					}
				case mvccpb.DELETE:
					// 服务注销
					if services, err := e.Discover(serviceName); err == nil {
						callback(services)
					}
				}
			}
		}
	}()

	return nil
}

// testConnection 测试etcd连接
func (e *EtcdDiscovery) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), e.config.DialTimeout)
	defer cancel()

	_, err := e.client.Status(ctx, e.config.Endpoints[0])
	return err
}

// buildServiceKey 构建服务键
func (e *EtcdDiscovery) buildServiceKey(serviceName, serviceID string) string {
	return path.Join(e.config.ServicePrefix, serviceName, serviceID)
}

// buildServicePrefix 构建服务前缀
func (e *EtcdDiscovery) buildServicePrefix(serviceName string) string {
	return path.Join(e.config.ServicePrefix, serviceName) + "/"
}

// watchServices 监控服务
func (e *EtcdDiscovery) watchServices() {
	for _, serviceName := range e.config.WatchServices {
		go func(name string) {
			e.Watch(name, func(services []*discovery.ServiceInstance) {
				// 更新统计信息
				e.mu.Lock()
				e.stats.LastUpdateTime = time.Now().Unix()
				e.stats.ActiveServices = int64(len(services))
				e.mu.Unlock()
			})
		}(serviceName)
	}
}

// refreshLoop 定期刷新循环
func (e *EtcdDiscovery) refreshLoop() {
	ticker := time.NewTicker(e.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.refreshStats()
		}
	}
}

// refreshStats 刷新统计信息
func (e *EtcdDiscovery) refreshStats() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 获取所有服务
	resp, err := e.client.Get(context.Background(), e.config.ServicePrefix, v3.WithPrefix(), v3.WithCountOnly())
	if err != nil {
		e.stats.ErrorCount++
		return
	}

	// 更新统计信息
	e.stats.TotalServices = resp.Count
	e.stats.LastUpdateTime = time.Now().Unix()
}

// GetStats 获取统计信息
func (e *EtcdDiscovery) GetStats() *EtcdStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// IsRunning 检查是否运行
func (e *EtcdDiscovery) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GetClient 获取etcd客户端
func (e *EtcdDiscovery) GetClient() *v3.Client {
	return e.client
}

// Compact 压缩etcd数据
func (e *EtcdDiscovery) Compact(revision int64) error {
	if !e.running {
		return fmt.Errorf("etcd discovery is not running")
	}

	_, err := e.client.Compact(context.Background(), revision)
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to compact etcd: %w", err)
	}

	return nil
}

// GetAllServices 获取所有服务
func (e *EtcdDiscovery) GetAllServices() (map[string][]*discovery.ServiceInstance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.running {
		return nil, fmt.Errorf("etcd discovery is not running")
	}

	// 获取所有服务
	resp, err := e.client.Get(context.Background(), e.config.ServicePrefix, v3.WithPrefix())
	if err != nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("failed to get all services: %w", err)
	}

	// 按服务名分组
	services := make(map[string][]*discovery.ServiceInstance)
	for _, kv := range resp.Kvs {
		var service discovery.ServiceInstance
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			continue
		}

		// 从键中提取服务名
		key := string(kv.Key)
		parts := strings.Split(strings.TrimPrefix(key, e.config.ServicePrefix+"/"), "/")
		if len(parts) >= 1 {
			serviceName := parts[0]
			services[serviceName] = append(services[serviceName], &service)
		}
	}

	return services, nil
}