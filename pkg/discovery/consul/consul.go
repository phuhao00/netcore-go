// Package consul Consul服务发现实现
// Author: NetCore-Go Team
// Created: 2024

package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/phuhao00/netcore-go/pkg/discovery"
)

// ConsulDiscovery Consul服务发现
type ConsulDiscovery struct {
	mu       sync.RWMutex
	client   *api.Client
	config   *ConsulConfig
	services map[string]*discovery.ServiceInfo
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *ConsulStats
}

// ConsulConfig Consul配置
type ConsulConfig struct {
	// Consul连接配置
	Address    string `json:"address"`
	Scheme     string `json:"scheme"`
	Datacenter string `json:"datacenter"`
	Token      string `json:"token"`
	TokenFile  string `json:"token_file"`

	// TLS配置
	TLSConfig *api.TLSConfig `json:"tls_config"`

	// 服务注册配置
	ServiceName        string            `json:"service_name"`
	ServiceID          string            `json:"service_id"`
	ServiceTags        []string          `json:"service_tags"`
	ServiceMeta        map[string]string `json:"service_meta"`
	ServiceAddress     string            `json:"service_address"`
	ServicePort        int               `json:"service_port"`
	EnableTagOverride  bool              `json:"enable_tag_override"`

	// 健康检查配置
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckHTTP     string        `json:"health_check_http"`
	HealthCheckTCP      string        `json:"health_check_tcp"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`
	HealthCheckTTL      time.Duration `json:"health_check_ttl"`

	// 发现配置
	WatchEnabled     bool              `json:"watch_enabled"`
	WatchServices    []string          `json:"watch_services"`
	QueryOptions     *api.QueryOptions `json:"-"`
	RefreshInterval  time.Duration     `json:"refresh_interval"`

	// 重试配置
	MaxRetries    int           `json:"max_retries"`
	RetryInterval time.Duration `json:"retry_interval"`
}

// ConsulStats Consul统计信息
type ConsulStats struct {
	TotalServices       int64 `json:"total_services"`
	HealthyServices     int64 `json:"healthy_services"`
	UnhealthyServices   int64 `json:"unhealthy_services"`
	RegistrationCount   int64 `json:"registration_count"`
	DeregistrationCount int64 `json:"deregistration_count"`
	QueryCount          int64 `json:"query_count"`
	WatchCount          int64 `json:"watch_count"`
	ErrorCount          int64 `json:"error_count"`
	LastUpdateTime      int64 `json:"last_update_time"`
}

// DefaultConsulConfig 返回默认Consul配置
func DefaultConsulConfig() *ConsulConfig {
	return &ConsulConfig{
		Address:             "127.0.0.1:8500",
		Scheme:              "http",
		Datacenter:          "dc1",
		ServiceTags:         []string{"netcore-go"},
		ServiceMeta:         make(map[string]string),
		EnableTagOverride:   false,
		HealthCheckEnabled:  true,
		HealthCheckInterval: 10 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		HealthCheckTTL:      30 * time.Second,
		WatchEnabled:        true,
		RefreshInterval:     30 * time.Second,
		MaxRetries:          3,
		RetryInterval:       5 * time.Second,
	}
}

// NewConsulDiscovery 创建Consul服务发现
func NewConsulDiscovery(config *ConsulConfig) (*ConsulDiscovery, error) {
	if config == nil {
		config = DefaultConsulConfig()
	}

	// 创建Consul客户端配置
	clientConfig := api.DefaultConfig()
	clientConfig.Address = config.Address
	clientConfig.Scheme = config.Scheme
	clientConfig.Datacenter = config.Datacenter
	clientConfig.Token = config.Token
	clientConfig.TokenFile = config.TokenFile

	if config.TLSConfig != nil {
		clientConfig.TLSConfig = *config.TLSConfig
	}

	// 创建Consul客户端
	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ConsulDiscovery{
		client:   client,
		config:   config,
		services: make(map[string]*discovery.ServiceInfo),
		ctx:      ctx,
		cancel:   cancel,
		stats:    &ConsulStats{},
	}, nil
}

// Start 启动Consul服务发现
func (c *ConsulDiscovery) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("Consul discovery is already running")
	}

	// 测试连接
	if err := c.testConnection(); err != nil {
		return fmt.Errorf("failed to connect to Consul: %w", err)
	}

	c.running = true

	// 启动服务监控
	if c.config.WatchEnabled {
		go c.watchServices()
	}

	// 启动定期刷新
	go c.refreshLoop()

	return nil
}

// Stop 停止Consul服务发现
func (c *ConsulDiscovery) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.running = false
	c.cancel()

	return nil
}

// Register 注册服务
func (c *ConsulDiscovery) Register(service *discovery.ServiceInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return fmt.Errorf("Consul discovery is not running")
	}

	// 创建服务注册信息
	registration := &api.AgentServiceRegistration{
		ID:                service.ID,
		Name:              service.Name,
		Tags:              append(c.config.ServiceTags, service.Tags...),
		Address:           service.Address,
		Port:              service.Port,
		Meta:              service.Meta,
		EnableTagOverride: c.config.EnableTagOverride,
	}

	// 添加健康检查
	if c.config.HealthCheckEnabled {
		registration.Check = c.createHealthCheck(service)
	}

	// 注册服务
	if err := c.client.Agent().ServiceRegister(registration); err != nil {
		c.stats.ErrorCount++
		return fmt.Errorf("failed to register service: %w", err)
	}

	c.services[service.ID] = service
	c.stats.RegistrationCount++
	c.stats.TotalServices++

	return nil
}

// Deregister 注销服务
func (c *ConsulDiscovery) Deregister(serviceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return fmt.Errorf("Consul discovery is not running")
	}

	// 注销服务
	if err := c.client.Agent().ServiceDeregister(serviceID); err != nil {
		c.stats.ErrorCount++
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	delete(c.services, serviceID)
	c.stats.DeregistrationCount++
	c.stats.TotalServices--

	return nil
}

// Discover 发现服务
func (c *ConsulDiscovery) Discover(serviceName string) ([]*discovery.ServiceInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.running {
		return nil, fmt.Errorf("Consul discovery is not running")
	}

	// 查询服务
	services, _, err := c.client.Health().Service(serviceName, "", true, c.config.QueryOptions)
	if err != nil {
		c.stats.ErrorCount++
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	c.stats.QueryCount++

	// 转换服务信息
	var result []*discovery.ServiceInfo
	for _, service := range services {
		serviceInfo := &discovery.ServiceInfo{
			ID:       service.Service.ID,
			Name:     service.Service.Service,
			Address:  service.Service.Address,
			Port:     service.Service.Port,
			Tags:     service.Service.Tags,
			Meta:   service.Service.Meta,
			Health: discovery.Healthy, // 只返回健康的服务
		}

		// 设置节点信息
		if service.Node != nil {
			if serviceInfo.Address == "" {
				serviceInfo.Address = service.Node.Address
			}
			if serviceInfo.Meta == nil {
				serviceInfo.Meta = make(map[string]string)
			}
			serviceInfo.Meta["node_id"] = service.Node.ID
			serviceInfo.Meta["node_name"] = service.Node.Node
		}

		result = append(result, serviceInfo)
	}

	return result, nil
}

// Watch 监控服务变化
func (c *ConsulDiscovery) Watch(serviceName string, callback func([]*discovery.ServiceInfo)) error {
	if !c.running {
		return fmt.Errorf("Consul discovery is not running")
	}

	go func() {
		var lastIndex uint64
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			// 创建查询选项
			queryOpts := &api.QueryOptions{
				WaitIndex: lastIndex,
				WaitTime:  30 * time.Second,
			}

			// 查询服务
			services, meta, err := c.client.Health().Service(serviceName, "", true, queryOpts)
			if err != nil {
				c.stats.ErrorCount++
				time.Sleep(c.config.RetryInterval)
				continue
			}

			// 检查是否有变化
			if meta.LastIndex != lastIndex {
				lastIndex = meta.LastIndex
				c.stats.WatchCount++

				// 转换服务信息
				var serviceInfos []*discovery.ServiceInfo
				for _, service := range services {
					serviceInfo := &discovery.ServiceInfo{
						ID:       service.Service.ID,
						Name:     service.Service.Service,
						Address:  service.Service.Address,
						Port:     service.Service.Port,
						Tags:     service.Service.Tags,
						Meta:   service.Service.Meta,
					Health: discovery.Healthy,
					}

					if service.Node != nil && serviceInfo.Address == "" {
						serviceInfo.Address = service.Node.Address
					}

					serviceInfos = append(serviceInfos, serviceInfo)
				}

				// 调用回调函数
				callback(serviceInfos)
			}
		}
	}()

	return nil
}

// testConnection 测试Consul连接
func (c *ConsulDiscovery) testConnection() error {
	_, err := c.client.Status().Leader()
	return err
}

// createHealthCheck 创建健康检查
func (c *ConsulDiscovery) createHealthCheck(service *discovery.ServiceInfo) *api.AgentServiceCheck {
	check := &api.AgentServiceCheck{
		Interval: c.config.HealthCheckInterval.String(),
		Timeout:  c.config.HealthCheckTimeout.String(),
	}

	// HTTP健康检查
	if c.config.HealthCheckHTTP != "" {
		check.HTTP = c.config.HealthCheckHTTP
	} else {
		// 默认HTTP健康检查
		scheme := "http"
		if service.Protocol == "https" {
			scheme = "https"
		}
		check.HTTP = fmt.Sprintf("%s://%s:%d/health", scheme, service.Address, service.Port)
	}

	// TCP健康检查
	if c.config.HealthCheckTCP != "" {
		check.TCP = c.config.HealthCheckTCP
	} else if check.HTTP == "" {
		check.TCP = net.JoinHostPort(service.Address, strconv.Itoa(service.Port))
	}

	// TTL健康检查
	if c.config.HealthCheckTTL > 0 {
		check.TTL = c.config.HealthCheckTTL.String()
	}

	return check
}

// watchServices 监控服务
func (c *ConsulDiscovery) watchServices() {
	for _, serviceName := range c.config.WatchServices {
		go func(name string) {
			c.Watch(name, func(services []*discovery.ServiceInfo) {
				// 更新统计信息
				c.mu.Lock()
				c.stats.LastUpdateTime = time.Now().Unix()
				c.mu.Unlock()
			})
		}(serviceName)
	}
}

// refreshLoop 定期刷新循环
func (c *ConsulDiscovery) refreshLoop() {
	ticker := time.NewTicker(c.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.refreshStats()
		}
	}
}

// refreshStats 刷新统计信息
func (c *ConsulDiscovery) refreshStats() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 获取所有服务
	services, err := c.client.Agent().Services()
	if err != nil {
		c.stats.ErrorCount++
		return
	}

	// 更新统计信息
	c.stats.TotalServices = int64(len(services))
	c.stats.LastUpdateTime = time.Now().Unix()

	// 检查服务健康状态
	healthyCount := int64(0)
	for _, service := range services {
		checks, _, err := c.client.Health().Checks(service.Service, nil)
		if err != nil {
			continue
		}

		isHealthy := true
		for _, check := range checks {
			if check.Status != api.HealthPassing {
				isHealthy = false
				break
			}
		}

		if isHealthy {
			healthyCount++
		}
	}

	c.stats.HealthyServices = healthyCount
	c.stats.UnhealthyServices = c.stats.TotalServices - healthyCount
}

// GetStats 获取统计信息
func (c *ConsulDiscovery) GetStats() *ConsulStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// IsRunning 检查是否运行
func (c *ConsulDiscovery) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// GetClient 获取Consul客户端
func (c *ConsulDiscovery) GetClient() *api.Client {
	return c.client
}