// Package kubernetes Kubernetes服务发现实现
// Author: NetCore-Go Team
// Created: 2024

package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/phuhao00/netcore-go/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock Kubernetes types for when k8s dependencies are not available
type MockKubernetesInterface interface {
	CoreV1() MockCoreV1Interface
}

// MockCoreV1Interface Mock CoreV1接口
type MockCoreV1Interface interface {
	Services(namespace string) MockServiceInterface
	Endpoints(namespace string) MockEndpointsInterface
	Namespaces() MockNamespaceInterface
	Pods(namespace string) MockPodInterface
}

// MockServiceInterface Mock Service接口
type MockServiceInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*corev1.ServiceList, error)
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Service, error)
	Create(ctx context.Context, service *corev1.Service, opts metav1.CreateOptions) (*corev1.Service, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	Watch(ctx context.Context, opts metav1.ListOptions) (MockWatcher, error)
}

// MockEndpointsInterface Mock Endpoints接口
type MockEndpointsInterface interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Endpoints, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (MockWatcher, error)
}

// MockNamespaceInterface Mock Namespace接口
type MockNamespaceInterface interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error)
}

// MockPodInterface Mock Pod接口
type MockPodInterface interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (MockWatcher, error)
}

type MockService struct {
	Name      string
	Namespace string
	ClusterIP string
	Ports     []MockServicePort
	Labels    map[string]string
}

type MockServicePort struct {
	Name     string
	Port     int32
	Protocol string
}

type MockWatcher interface {
	ResultChan() <-chan MockWatchEvent
	Stop()
}

type MockWatchEvent struct {
	Type   string
	Object interface{}
}

type mockKubernetesClient struct{
	coreV1 *mockCoreV1Client
}

func (m *mockKubernetesClient) CoreV1() MockCoreV1Interface {
	if m.coreV1 == nil {
		m.coreV1 = &mockCoreV1Client{}
	}
	return m.coreV1
}

type mockCoreV1Client struct{}

func (m *mockCoreV1Client) Services(namespace string) MockServiceInterface {
	return &mockServiceClient{namespace: namespace}
}

func (m *mockCoreV1Client) Endpoints(namespace string) MockEndpointsInterface {
	return &mockEndpointsClient{namespace: namespace}
}

func (m *mockCoreV1Client) Namespaces() MockNamespaceInterface {
	return &mockNamespaceClient{}
}

func (m *mockCoreV1Client) Pods(namespace string) MockPodInterface {
	return &mockPodClient{namespace: namespace}
}

type mockServiceClient struct {
	namespace string
}

func (m *mockServiceClient) List(ctx context.Context, opts metav1.ListOptions) (*corev1.ServiceList, error) {
	return &corev1.ServiceList{
		Items: []corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-service",
					Namespace: m.namespace,
					Labels:    map[string]string{"app": "example"},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			},
		},
	}, nil
}

func (m *mockServiceClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Service, error) {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
		},
	}, nil
}

func (m *mockServiceClient) Create(ctx context.Context, service *corev1.Service, opts metav1.CreateOptions) (*corev1.Service, error) {
	return service, nil
}

func (m *mockServiceClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return nil
}

func (m *mockServiceClient) Watch(ctx context.Context, opts metav1.ListOptions) (MockWatcher, error) {
	return &mockWatcher{}, nil
}

type mockEndpointsClient struct {
	namespace string
}

func (m *mockEndpointsClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Endpoints, error) {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "10.0.0.1"},
				},
				Ports: []corev1.EndpointPort{
					{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	}, nil
}

func (m *mockEndpointsClient) Watch(ctx context.Context, opts metav1.ListOptions) (MockWatcher, error) {
	return &mockWatcher{}, nil
}

type mockNamespaceClient struct{}

func (m *mockNamespaceClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error) {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, nil
}

type mockPodClient struct {
	namespace string
}

func (m *mockPodClient) Watch(ctx context.Context, opts metav1.ListOptions) (MockWatcher, error) {
	return &mockWatcher{}, nil
}

type mockWatcher struct {
	ch chan MockWatchEvent
}

func (w *mockWatcher) ResultChan() <-chan MockWatchEvent {
	if w.ch == nil {
		w.ch = make(chan MockWatchEvent)
	}
	return w.ch
}

func (w *mockWatcher) Stop() {
	if w.ch != nil {
		close(w.ch)
	}
}

// KubernetesDiscovery Kubernetes服务发现
type KubernetesDiscovery struct {
	mu        sync.RWMutex
	client    MockKubernetesInterface
	config    *KubernetesConfig
	services  map[string]*discovery.ServiceInstance
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	stats     *KubernetesStats
	watchers  map[string]MockWatcher
}

// KubernetesConfig Kubernetes配置
type KubernetesConfig struct {
	// Kubernetes连接配置
	KubeConfig     string `json:"kube_config"`     // kubeconfig文件路径
	InCluster      bool   `json:"in_cluster"`      // 是否在集群内运行
	MasterURL      string `json:"master_url"`     // Kubernetes API服务器URL
	BearerToken    string `json:"bearer_token"`   // 认证令牌
	BearerTokenFile string `json:"bearer_token_file"` // 令牌文件路径

	// 服务发现配置
	Namespace       string            `json:"namespace"`        // 命名空间
	LabelSelector   string            `json:"label_selector"`   // 标签选择器
	FieldSelector   string            `json:"field_selector"`   // 字段选择器
	ServiceTypes    []string          `json:"service_types"`    // 服务类型过滤
	AnnotationKeys  []string          `json:"annotation_keys"`  // 注解键过滤
	PortName        string            `json:"port_name"`        // 端口名称
	ServiceMeta     map[string]string `json:"service_meta"`     // 服务元数据

	// 监控配置
	WatchEnabled     bool          `json:"watch_enabled"`     // 启用监控
	WatchServices    []string      `json:"watch_services"`    // 监控的服务
	WatchEndpoints   bool          `json:"watch_endpoints"`   // 监控端点
	WatchPods        bool          `json:"watch_pods"`        // 监控Pod
	RefreshInterval  time.Duration `json:"refresh_interval"`  // 刷新间隔
	ResyncPeriod     time.Duration `json:"resync_period"`     // 重新同步周期

	// 健康检查配置
	HealthCheckEnabled bool          `json:"health_check_enabled"` // 启用健康检查
	HealthCheckPath    string        `json:"health_check_path"`    // 健康检查路径
	HealthCheckTimeout time.Duration `json:"health_check_timeout"` // 健康检查超时

	// 重试配置
	MaxRetries    int           `json:"max_retries"`    // 最大重试次数
	RetryInterval time.Duration `json:"retry_interval"` // 重试间隔
}

// KubernetesStats Kubernetes统计信息
type KubernetesStats struct {
	TotalServices     int64 `json:"total_services"`     // 总服务数
	HealthyServices   int64 `json:"healthy_services"`   // 健康服务数
	UnhealthyServices int64 `json:"unhealthy_services"` // 不健康服务数
	TotalEndpoints    int64 `json:"total_endpoints"`    // 总端点数
	ActiveEndpoints   int64 `json:"active_endpoints"`   // 活跃端点数
	TotalPods         int64 `json:"total_pods"`         // 总Pod数
	RunningPods       int64 `json:"running_pods"`       // 运行中Pod数
	QueryCount        int64 `json:"query_count"`        // 查询次数
	WatchCount        int64 `json:"watch_count"`        // 监控次数
	ErrorCount        int64 `json:"error_count"`        // 错误次数
	LastUpdateTime    int64 `json:"last_update_time"`   // 最后更新时间
}

// DefaultKubernetesConfig 返回默认Kubernetes配置
func DefaultKubernetesConfig() *KubernetesConfig {
	return &KubernetesConfig{
		InCluster:           false,
		Namespace:           "default",
		ServiceTypes:        []string{"ClusterIP", "NodePort", "LoadBalancer"},
		PortName:            "http",
		ServiceMeta:         make(map[string]string),
		WatchEnabled:        true,
		WatchEndpoints:      true,
		WatchPods:           false,
		RefreshInterval:     30 * time.Second,
		ResyncPeriod:        5 * time.Minute,
		HealthCheckEnabled:  true,
		HealthCheckPath:     "/health",
		HealthCheckTimeout:  5 * time.Second,
		MaxRetries:          3,
		RetryInterval:       5 * time.Second,
	}
}

// NewKubernetesDiscovery 创建Kubernetes服务发现
func NewKubernetesDiscovery(config *KubernetesConfig) (*KubernetesDiscovery, error) {
	if config == nil {
		config = DefaultKubernetesConfig()
	}

	// 创建Kubernetes客户端 (使用mock实现)
	client := &mockKubernetesClient{}

	ctx, cancel := context.WithCancel(context.Background())

	return &KubernetesDiscovery{
		client:   client,
		config:   config,
		services: make(map[string]*discovery.ServiceInstance),
		ctx:      ctx,
		cancel:   cancel,
		stats:    &KubernetesStats{},
		watchers: make(map[string]MockWatcher),
	}, nil
}

// createKubernetesClient 创建Kubernetes客户端 (Mock实现)
// 注意: 这是一个mock实现，用于演示目的
// 在生产环境中，应该使用真实的Kubernetes client-go库
func createKubernetesClient(config *KubernetesConfig) (MockKubernetesInterface, error) {
	// 返回mock客户端
	return &mockKubernetesClient{}, nil
}

// Start 启动Kubernetes服务发现
func (k *KubernetesDiscovery) Start() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.running {
		return fmt.Errorf("Kubernetes discovery is already running")
	}

	// 测试连接
	if err := k.testConnection(); err != nil {
		return fmt.Errorf("failed to connect to Kubernetes: %w", err)
	}

	k.running = true

	// 启动服务监控
	if k.config.WatchEnabled {
		go k.watchServices()
		if k.config.WatchEndpoints {
			go k.watchEndpoints()
		}
		if k.config.WatchPods {
			go k.watchPods()
		}
	}

	// 启动定期刷新
	go k.refreshLoop()

	return nil
}

// Stop 停止Kubernetes服务发现
func (k *KubernetesDiscovery) Stop() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.running {
		return nil
	}

	k.running = false
	k.cancel()

	// 停止所有监控器
	for name, watcher := range k.watchers {
		watcher.Stop()
		delete(k.watchers, name)
	}

	return nil
}

// Register 注册服务（Kubernetes中通过Service资源实现）
func (k *KubernetesDiscovery) Register(service *discovery.ServiceInstance) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.running {
		return fmt.Errorf("Kubernetes discovery is not running")
	}

	// 在Kubernetes中，服务注册通过创建Service资源实现
	// 这里我们创建一个Service对象
	kubeService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        service.Name,
			Namespace:   k.config.Namespace,
			Labels:      convertTagsToLabels(service.Tags),
			Annotations: service.Meta,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": service.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:     k.config.PortName,
					Port:     int32(service.Port),
					Protocol: corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// 创建Service
	_, err := k.client.CoreV1().Services(k.config.Namespace).Create(context.Background(), kubeService, metav1.CreateOptions{})
	if err != nil {
		k.stats.ErrorCount++
		return fmt.Errorf("failed to create Kubernetes service: %w", err)
	}

	k.services[service.ID] = service
	k.stats.TotalServices++

	return nil
}

// Deregister 注销服务
func (k *KubernetesDiscovery) Deregister(serviceID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.running {
		return fmt.Errorf("Kubernetes discovery is not running")
	}

	service, exists := k.services[serviceID]
	if !exists {
		return fmt.Errorf("service %s not found", serviceID)
	}

	// 删除Service
	err := k.client.CoreV1().Services(k.config.Namespace).Delete(context.Background(), service.Name, metav1.DeleteOptions{})
	if err != nil {
		k.stats.ErrorCount++
		return fmt.Errorf("failed to delete Kubernetes service: %w", err)
	}

	delete(k.services, serviceID)
	k.stats.TotalServices--

	return nil
}

// Discover 发现服务
func (k *KubernetesDiscovery) Discover(serviceName string) ([]*discovery.ServiceInstance, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.running {
		return nil, fmt.Errorf("Kubernetes discovery is not running")
	}

	// 构建标签选择器
	labelSelector := k.config.LabelSelector
	if serviceName != "" && labelSelector == "" {
		labelSelector = fmt.Sprintf("app=%s", serviceName)
	}

	// 查询Services
	services, err := k.client.CoreV1().Services(k.config.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: k.config.FieldSelector,
	})
	if err != nil {
		k.stats.ErrorCount++
		return nil, fmt.Errorf("failed to list Kubernetes services: %w", err)
	}

	k.stats.QueryCount++

	// 转换为服务信息
	var result []*discovery.ServiceInstance
	for _, svc := range services.Items {
		// 过滤服务类型
		if !k.isServiceTypeAllowed(string(svc.Spec.Type)) {
			continue
		}

		// 获取端点
		endpoints, err := k.getServiceEndpoints(svc.Name)
		if err != nil {
			continue
		}

		// 为每个端点创建服务信息
		for _, endpoint := range endpoints {
			serviceInfo := &discovery.ServiceInstance{
				ID:       fmt.Sprintf("%s-%s", svc.Name, endpoint.IP),
				Name:     svc.Name,
				Address:  endpoint.IP,
				Port:     endpoint.Port,
				Tags:     convertLabelsToTags(svc.Labels),
				Meta: svc.Annotations,
				Health:  func() discovery.HealthStatus {
					if endpoint.Ready {
						return discovery.Healthy
					}
					return discovery.Unhealthy
				}(),
			}

			// 添加Kubernetes特定元数据
			if serviceInfo.Meta == nil {
				serviceInfo.Meta = make(map[string]string)
			}
			serviceInfo.Meta["kubernetes.namespace"] = svc.Namespace
			serviceInfo.Meta["kubernetes.service_type"] = string(svc.Spec.Type)
			serviceInfo.Meta["kubernetes.cluster_ip"] = svc.Spec.ClusterIP

			result = append(result, serviceInfo)
		}
	}

	return result, nil
}

// Watch 监控服务变化
func (k *KubernetesDiscovery) Watch(serviceName string, callback func([]*discovery.ServiceInstance)) error {
	if !k.running {
		return fmt.Errorf("Kubernetes discovery is not running")
	}

	go func() {
		// 构建监控选项
		labelSelector := k.config.LabelSelector
		if serviceName != "" && labelSelector == "" {
			labelSelector = fmt.Sprintf("app=%s", serviceName)
		}

		// 创建监控器
		watcher, err := k.client.CoreV1().Services(k.config.Namespace).Watch(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: k.config.FieldSelector,
		})
		if err != nil {
			k.mu.Lock()
			k.stats.ErrorCount++
			k.mu.Unlock()
			return
		}

		k.mu.Lock()
		k.watchers[serviceName] = watcher
		k.mu.Unlock()

		// Simple mock implementation - just call callback periodically
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-k.ctx.Done():
				return
			case <-ticker.C:
				// Mock: discover services and call callback
				services, _ := k.Discover(serviceName)
				callback(services)
			}
		}
	}()

	return nil
}

// getServiceEndpoints 获取服务端点
func (k *KubernetesDiscovery) getServiceEndpoints(serviceName string) ([]ServiceEndpoint, error) {
	endpoints, err := k.client.CoreV1().Endpoints(k.config.Namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var result []ServiceEndpoint
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			// 过滤端口名称
			if k.config.PortName != "" && port.Name != k.config.PortName {
				continue
			}

			// 添加就绪地址
			for _, addr := range subset.Addresses {
				result = append(result, ServiceEndpoint{
					IP:    addr.IP,
					Port:  int(port.Port),
					Ready: true,
				})
			}

			// 添加未就绪地址
			for _, addr := range subset.NotReadyAddresses {
				result = append(result, ServiceEndpoint{
					IP:    addr.IP,
					Port:  int(port.Port),
					Ready: false,
				})
			}
		}
	}

	return result, nil
}

// ServiceEndpoint 服务端点
type ServiceEndpoint struct {
	IP    string `json:"ip"`
	Port  int    `json:"port"`
	Ready bool   `json:"ready"`
}

// testConnection 测试Kubernetes连接
func (k *KubernetesDiscovery) testConnection() error {
	_, err := k.client.CoreV1().Namespaces().Get(context.Background(), k.config.Namespace, metav1.GetOptions{})
	return err
}

// isServiceTypeAllowed 检查服务类型是否允许
func (k *KubernetesDiscovery) isServiceTypeAllowed(serviceType string) bool {
	if len(k.config.ServiceTypes) == 0 {
		return true
	}

	for _, allowedType := range k.config.ServiceTypes {
		if allowedType == serviceType {
			return true
		}
	}
	return false
}

// convertTagsToLabels 转换标签到Kubernetes标签
func convertTagsToLabels(tags []string) map[string]string {
	labels := make(map[string]string)
	for i, tag := range tags {
		labels[fmt.Sprintf("tag-%d", i)] = tag
	}
	return labels
}

// convertLabelsToTags 转换Kubernetes标签到标签
func convertLabelsToTags(labels map[string]string) []string {
	var tags []string
	for key, value := range labels {
		tags = append(tags, fmt.Sprintf("%s=%s", key, value))
	}
	return tags
}

// watchServices 监控服务
func (k *KubernetesDiscovery) watchServices() {
	for _, serviceName := range k.config.WatchServices {
		go func(name string) {
			k.Watch(name, func(services []*discovery.ServiceInstance) {
				// 更新统计信息
				k.mu.Lock()
				k.stats.LastUpdateTime = time.Now().Unix()
				k.stats.HealthyServices = 0
				for _, svc := range services {
					if svc.Health == discovery.Healthy {
						k.stats.HealthyServices++
					}
				}
				k.stats.UnhealthyServices = int64(len(services)) - k.stats.HealthyServices
				k.mu.Unlock()
			})
		}(serviceName)
	}
}

// watchEndpoints 监控端点
func (k *KubernetesDiscovery) watchEndpoints() {
	watcher, err := k.client.CoreV1().Endpoints(k.config.Namespace).Watch(context.Background(), metav1.ListOptions{
		LabelSelector: k.config.LabelSelector,
		FieldSelector: k.config.FieldSelector,
	})
	if err != nil {
		k.mu.Lock()
		k.stats.ErrorCount++
		k.mu.Unlock()
		return
	}

	k.mu.Lock()
	k.watchers["endpoints"] = watcher
	k.mu.Unlock()

	for event := range watcher.ResultChan() {
		select {
		case <-k.ctx.Done():
			return
		default:
		}

		k.mu.Lock()
		k.stats.WatchCount++
		k.mu.Unlock()

		// 处理端点变化事件
		if endpoints, ok := event.Object.(*corev1.Endpoints); ok {
			k.updateEndpointStats(endpoints)
		}
	}
}

// watchPods 监控Pod
func (k *KubernetesDiscovery) watchPods() {
	watcher, err := k.client.CoreV1().Pods(k.config.Namespace).Watch(context.Background(), metav1.ListOptions{
		LabelSelector: k.config.LabelSelector,
		FieldSelector: k.config.FieldSelector,
	})
	if err != nil {
		k.mu.Lock()
		k.stats.ErrorCount++
		k.mu.Unlock()
		return
	}

	k.mu.Lock()
	k.watchers["pods"] = watcher
	k.mu.Unlock()

	for event := range watcher.ResultChan() {
		select {
		case <-k.ctx.Done():
			return
		default:
		}

		k.mu.Lock()
		k.stats.WatchCount++
		k.mu.Unlock()

		// 处理Pod变化事件
		if pod, ok := event.Object.(*corev1.Pod); ok {
			k.updatePodStats(pod)
		}
	}
}

// updateEndpointStats 更新端点统计
func (k *KubernetesDiscovery) updateEndpointStats(endpoints *corev1.Endpoints) {
	k.mu.Lock()
	defer k.mu.Unlock()

	var totalEndpoints, activeEndpoints int64
	for _, subset := range endpoints.Subsets {
		totalEndpoints += int64(len(subset.Addresses) + len(subset.NotReadyAddresses))
		activeEndpoints += int64(len(subset.Addresses))
	}

	k.stats.TotalEndpoints = totalEndpoints
	k.stats.ActiveEndpoints = activeEndpoints
	k.stats.LastUpdateTime = time.Now().Unix()
}

// updatePodStats 更新Pod统计
func (k *KubernetesDiscovery) updatePodStats(pod *corev1.Pod) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if pod.Status.Phase == corev1.PodRunning {
		k.stats.RunningPods++
	}
	k.stats.TotalPods++
	k.stats.LastUpdateTime = time.Now().Unix()
}

// refreshLoop 定期刷新循环
func (k *KubernetesDiscovery) refreshLoop() {
	ticker := time.NewTicker(k.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			k.refreshStats()
		}
	}
}

// refreshStats 刷新统计信息
func (k *KubernetesDiscovery) refreshStats() {
	k.mu.Lock()
	defer k.mu.Unlock()

	// 获取所有服务
	services, err := k.client.CoreV1().Services(k.config.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k.stats.ErrorCount++
		return
	}

	// 更新统计信息
	k.stats.TotalServices = int64(len(services.Items))
	k.stats.LastUpdateTime = time.Now().Unix()
}

// GetStats 获取统计信息
func (k *KubernetesDiscovery) GetStats() *KubernetesStats {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.stats
}

// IsRunning 检查是否运行
func (k *KubernetesDiscovery) IsRunning() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.running
}

// GetClient 获取Kubernetes客户端
func (k *KubernetesDiscovery) GetClient() MockKubernetesInterface {
	return k.client
}

// GetNamespace 获取命名空间
func (k *KubernetesDiscovery) GetNamespace() string {
	return k.config.Namespace
}