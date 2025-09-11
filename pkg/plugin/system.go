// Package plugin 插件系统实现
// Author: NetCore-Go Team
// Created: 2024

package plugin

import (
	"context"
	"fmt"
	"net/http"
	"plugin"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// PluginType 插件类型
type PluginType string

const (
	PluginTypeMiddleware PluginType = "middleware"
	PluginTypeHandler    PluginType = "handler"
	PluginTypeService    PluginType = "service"
	PluginTypeFilter     PluginType = "filter"
	PluginTypeTransform  PluginType = "transform"
	PluginTypeAuth       PluginType = "auth"
	PluginTypeLogger     PluginType = "logger"
	PluginTypeMetrics    PluginType = "metrics"
)

// PluginStatus 插件状态
type PluginStatus string

const (
	PluginStatusLoaded    PluginStatus = "loaded"
	PluginStatusActive    PluginStatus = "active"
	PluginStatusInactive  PluginStatus = "inactive"
	PluginStatusError     PluginStatus = "error"
	PluginStatusUnloaded  PluginStatus = "unloaded"
)

// Plugin 插件接口
type Plugin interface {
	Name() string
	Version() string
	Description() string
	Type() PluginType
	Init(config map[string]interface{}) error
	Start() error
	Stop() error
	HealthCheck() error
}

// MiddlewarePlugin 中间件插件接口
type MiddlewarePlugin interface {
	Plugin
	Middleware() func(http.Handler) http.Handler
	Priority() int
}

// HandlerPlugin 处理器插件接口
type HandlerPlugin interface {
	Plugin
	Handler() http.Handler
	Routes() []Route
}

// ServicePlugin 服务插件接口
type ServicePlugin interface {
	Plugin
	Service() interface{}
	Dependencies() []string
}

// Route 路由定义
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// PluginInfo 插件信息
type PluginInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Type        PluginType             `json:"type"`
	Status      PluginStatus           `json:"status"`
	Config      map[string]interface{} `json:"config"`
	FilePath    string                 `json:"file_path"`
	LoadTime    time.Time              `json:"load_time"`
	StartTime   time.Time              `json:"start_time"`
	Error       string                 `json:"error,omitempty"`
	Metrics     *PluginMetrics         `json:"metrics"`
}

// PluginMetrics 插件指标
type PluginMetrics struct {
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	AverageLatency  time.Duration `json:"average_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	MinLatency      time.Duration `json:"min_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
	Uptime          time.Duration `json:"uptime"`
}

// PluginManager 插件管理器
type PluginManager struct {
	mu           sync.RWMutex
	plugins      map[string]*PluginInfo
	instances    map[string]Plugin
	middlewares  []MiddlewarePlugin
	handlers     map[string]HandlerPlugin
	services     map[string]ServicePlugin
	config       *PluginManagerConfig
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	stats        *PluginManagerStats
	hooks        *PluginHooks
}

// PluginManagerConfig 插件管理器配置
type PluginManagerConfig struct {
	// 插件目录
	PluginDir       string   `json:"plugin_dir"`
	PluginPaths     []string `json:"plugin_paths"`
	AutoLoad        bool     `json:"auto_load"`
	AutoStart       bool     `json:"auto_start"`

	// 安全配置
	AllowedTypes    []PluginType `json:"allowed_types"`
	Blacklist       []string     `json:"blacklist"`
	Whitelist       []string     `json:"whitelist"`
	SandboxEnabled  bool         `json:"sandbox_enabled"`

	// 监控配置
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	MetricsEnabled      bool          `json:"metrics_enabled"`

	// 热重载配置
	HotReloadEnabled bool          `json:"hot_reload_enabled"`
	WatchInterval    time.Duration `json:"watch_interval"`

	// 依赖管理
	DependencyResolution bool `json:"dependency_resolution"`
	MaxDependencyDepth   int  `json:"max_dependency_depth"`

	// 超时配置
	LoadTimeout  time.Duration `json:"load_timeout"`
	StartTimeout time.Duration `json:"start_timeout"`
	StopTimeout  time.Duration `json:"stop_timeout"`
}

// PluginManagerStats 插件管理器统计
type PluginManagerStats struct {
	TotalPlugins    int64 `json:"total_plugins"`
	ActivePlugins   int64 `json:"active_plugins"`
	InactivePlugins int64 `json:"inactive_plugins"`
	ErrorPlugins    int64 `json:"error_plugins"`
	LoadedPlugins   int64 `json:"loaded_plugins"`
	StartTime       time.Time `json:"start_time"`
	LastReloadTime  time.Time `json:"last_reload_time"`
}

// PluginHooks 插件钩子
type PluginHooks struct {
	OnLoad   func(plugin Plugin) error
	OnStart  func(plugin Plugin) error
	OnStop   func(plugin Plugin) error
	OnError  func(plugin Plugin, err error)
	OnReload func(plugin Plugin) error
}

// DefaultPluginManagerConfig 返回默认插件管理器配置
func DefaultPluginManagerConfig() *PluginManagerConfig {
	return &PluginManagerConfig{
		PluginDir:               "./plugins",
		AutoLoad:                true,
		AutoStart:               true,
		AllowedTypes:            []PluginType{PluginTypeMiddleware, PluginTypeHandler, PluginTypeService},
		SandboxEnabled:          true,
		HealthCheckEnabled:      true,
		HealthCheckInterval:     30 * time.Second,
		MetricsEnabled:          true,
		HotReloadEnabled:        false,
		WatchInterval:           5 * time.Second,
		DependencyResolution:    true,
		MaxDependencyDepth:      10,
		LoadTimeout:             30 * time.Second,
		StartTimeout:            10 * time.Second,
		StopTimeout:             10 * time.Second,
	}
}

// NewPluginManager 创建插件管理器
func NewPluginManager(config *PluginManagerConfig) *PluginManager {
	if config == nil {
		config = DefaultPluginManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &PluginManager{
		plugins:   make(map[string]*PluginInfo),
		instances: make(map[string]Plugin),
		handlers:  make(map[string]HandlerPlugin),
		services:  make(map[string]ServicePlugin),
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		stats:     &PluginManagerStats{StartTime: time.Now()},
		hooks:     &PluginHooks{},
	}
}

// Start 启动插件管理器
func (pm *PluginManager) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return fmt.Errorf("plugin manager is already running")
	}

	pm.running = true

	// 自动加载插件
	if pm.config.AutoLoad {
		if err := pm.loadAllPlugins(); err != nil {
			return fmt.Errorf("failed to auto-load plugins: %w", err)
		}
	}

	// 自动启动插件
	if pm.config.AutoStart {
		if err := pm.startAllPlugins(); err != nil {
			return fmt.Errorf("failed to auto-start plugins: %w", err)
		}
	}

	// 启动健康检查
	if pm.config.HealthCheckEnabled {
		go pm.healthCheckLoop()
	}

	// 启动热重载监控
	if pm.config.HotReloadEnabled {
		go pm.watchLoop()
	}

	return nil
}

// Stop 停止插件管理器
func (pm *PluginManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return nil
	}

	pm.running = false
	pm.cancel()

	// 停止所有插件
	return pm.stopAllPlugins()
}

// LoadPlugin 加载插件
func (pm *PluginManager) LoadPlugin(filePath string, config map[string]interface{}) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 检查是否已加载
	for _, info := range pm.plugins {
		if info.FilePath == filePath {
			return fmt.Errorf("plugin already loaded: %s", filePath)
		}
	}

	// 加载Go插件
	p, err := plugin.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open plugin %s: %w", filePath, err)
	}

	// 查找插件符号
	symbol, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("plugin %s does not export 'Plugin' symbol: %w", filePath, err)
	}

	// 类型断言
	pluginInstance, ok := symbol.(Plugin)
	if !ok {
		return fmt.Errorf("plugin %s 'Plugin' symbol is not of type Plugin", filePath)
	}

	// 检查插件类型是否允许
	if !pm.isTypeAllowed(pluginInstance.Type()) {
		return fmt.Errorf("plugin type %s is not allowed", pluginInstance.Type())
	}

	// 检查黑白名单
	if !pm.isPluginAllowed(pluginInstance.Name()) {
		return fmt.Errorf("plugin %s is not allowed", pluginInstance.Name())
	}

	// 初始化插件
	if err := pluginInstance.Init(config); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", pluginInstance.Name(), err)
	}

	// 创建插件信息
	pluginID := pm.generatePluginID(pluginInstance.Name())
	pluginInfo := &PluginInfo{
		ID:          pluginID,
		Name:        pluginInstance.Name(),
		Version:     pluginInstance.Version(),
		Description: pluginInstance.Description(),
		Type:        pluginInstance.Type(),
		Status:      PluginStatusLoaded,
		Config:      config,
		FilePath:    filePath,
		LoadTime:    time.Now(),
		Metrics:     &PluginMetrics{},
	}

	// 存储插件
	pm.plugins[pluginID] = pluginInfo
	pm.instances[pluginID] = pluginInstance

	// 根据类型分类存储
	switch pluginInstance.Type() {
	case PluginTypeMiddleware:
		if mw, ok := pluginInstance.(MiddlewarePlugin); ok {
			pm.middlewares = append(pm.middlewares, mw)
			pm.sortMiddlewares()
		}
	case PluginTypeHandler:
		if handler, ok := pluginInstance.(HandlerPlugin); ok {
			pm.handlers[pluginID] = handler
		}
	case PluginTypeService:
		if service, ok := pluginInstance.(ServicePlugin); ok {
			pm.services[pluginID] = service
		}
	}

	// 更新统计
	pm.stats.TotalPlugins++
	pm.stats.LoadedPlugins++

	// 调用钩子
	if pm.hooks.OnLoad != nil {
		if err := pm.hooks.OnLoad(pluginInstance); err != nil {
			pluginInfo.Status = PluginStatusError
			pluginInfo.Error = err.Error()
			if pm.hooks.OnError != nil {
				pm.hooks.OnError(pluginInstance, err)
			}
		}
	}

	return nil
}

// StartPlugin 启动插件
func (pm *PluginManager) StartPlugin(pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pluginInfo, exists := pm.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	if pluginInfo.Status == PluginStatusActive {
		return fmt.Errorf("plugin already active: %s", pluginID)
	}

	pluginInstance := pm.instances[pluginID]

	// 启动插件
	if err := pluginInstance.Start(); err != nil {
		pluginInfo.Status = PluginStatusError
		pluginInfo.Error = err.Error()
		pm.stats.ErrorPlugins++
		if pm.hooks.OnError != nil {
			pm.hooks.OnError(pluginInstance, err)
		}
		return fmt.Errorf("failed to start plugin %s: %w", pluginID, err)
	}

	pluginInfo.Status = PluginStatusActive
	pluginInfo.StartTime = time.Now()
	pluginInfo.Error = ""
	pm.stats.ActivePlugins++

	// 调用钩子
	if pm.hooks.OnStart != nil {
		if err := pm.hooks.OnStart(pluginInstance); err != nil {
			if pm.hooks.OnError != nil {
				pm.hooks.OnError(pluginInstance, err)
			}
		}
	}

	return nil
}

// StopPlugin 停止插件
func (pm *PluginManager) StopPlugin(pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pluginInfo, exists := pm.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	if pluginInfo.Status != PluginStatusActive {
		return fmt.Errorf("plugin not active: %s", pluginID)
	}

	pluginInstance := pm.instances[pluginID]

	// 停止插件
	if err := pluginInstance.Stop(); err != nil {
		pluginInfo.Status = PluginStatusError
		pluginInfo.Error = err.Error()
		if pm.hooks.OnError != nil {
			pm.hooks.OnError(pluginInstance, err)
		}
		return fmt.Errorf("failed to stop plugin %s: %w", pluginID, err)
	}

	pluginInfo.Status = PluginStatusInactive
	pm.stats.ActivePlugins--
	pm.stats.InactivePlugins++

	// 调用钩子
	if pm.hooks.OnStop != nil {
		if err := pm.hooks.OnStop(pluginInstance); err != nil {
			if pm.hooks.OnError != nil {
				pm.hooks.OnError(pluginInstance, err)
			}
		}
	}

	return nil
}

// UnloadPlugin 卸载插件
func (pm *PluginManager) UnloadPlugin(pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pluginInfo, exists := pm.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// 先停止插件
	if pluginInfo.Status == PluginStatusActive {
		if err := pm.stopPluginUnsafe(pluginID); err != nil {
			return err
		}
	}

	// 从分类存储中移除
	switch pluginInfo.Type {
	case PluginTypeMiddleware:
		pm.removeMiddleware(pluginID)
	case PluginTypeHandler:
		delete(pm.handlers, pluginID)
	case PluginTypeService:
		delete(pm.services, pluginID)
	}

	// 移除插件
	delete(pm.plugins, pluginID)
	delete(pm.instances, pluginID)

	// 更新统计
	pm.stats.TotalPlugins--
	pm.stats.LoadedPlugins--

	pluginInfo.Status = PluginStatusUnloaded

	return nil
}

// GetPlugin 获取插件信息
func (pm *PluginManager) GetPlugin(pluginID string) (*PluginInfo, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	plugin, exists := pm.plugins[pluginID]
	return plugin, exists
}

// ListPlugins 列出所有插件
func (pm *PluginManager) ListPlugins() []*PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugins := make([]*PluginInfo, 0, len(pm.plugins))
	for _, plugin := range pm.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// GetMiddlewares 获取所有中间件插件
func (pm *PluginManager) GetMiddlewares() []MiddlewarePlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return append([]MiddlewarePlugin(nil), pm.middlewares...)
}

// GetHandlers 获取所有处理器插件
func (pm *PluginManager) GetHandlers() map[string]HandlerPlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	handlers := make(map[string]HandlerPlugin)
	for id, handler := range pm.handlers {
		handlers[id] = handler
	}
	return handlers
}

// GetServices 获取所有服务插件
func (pm *PluginManager) GetServices() map[string]ServicePlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	services := make(map[string]ServicePlugin)
	for id, service := range pm.services {
		services[id] = service
	}
	return services
}

// GetStats 获取统计信息
func (pm *PluginManager) GetStats() *PluginManagerStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	stats := *pm.stats
	return &stats
}

// SetHooks 设置钩子函数
func (pm *PluginManager) SetHooks(hooks *PluginHooks) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.hooks = hooks
}

// 内部方法

// loadAllPlugins 加载所有插件
func (pm *PluginManager) loadAllPlugins() error {
	// 这里应该实现扫描插件目录并加载所有插件的逻辑
	// 为了简化，这里只是一个占位符
	return nil
}

// startAllPlugins 启动所有插件
func (pm *PluginManager) startAllPlugins() error {
	for pluginID := range pm.plugins {
		if err := pm.startPluginUnsafe(pluginID); err != nil {
			fmt.Printf("Failed to start plugin %s: %v\n", pluginID, err)
		}
	}
	return nil
}

// stopAllPlugins 停止所有插件
func (pm *PluginManager) stopAllPlugins() error {
	for pluginID := range pm.plugins {
		if err := pm.stopPluginUnsafe(pluginID); err != nil {
			fmt.Printf("Failed to stop plugin %s: %v\n", pluginID, err)
		}
	}
	return nil
}

// startPluginUnsafe 启动插件（不加锁）
func (pm *PluginManager) startPluginUnsafe(pluginID string) error {
	pluginInfo := pm.plugins[pluginID]
	pluginInstance := pm.instances[pluginID]

	if pluginInfo.Status == PluginStatusActive {
		return nil
	}

	if err := pluginInstance.Start(); err != nil {
		pluginInfo.Status = PluginStatusError
		pluginInfo.Error = err.Error()
		return err
	}

	pluginInfo.Status = PluginStatusActive
	pluginInfo.StartTime = time.Now()
	pm.stats.ActivePlugins++

	return nil
}

// stopPluginUnsafe 停止插件（不加锁）
func (pm *PluginManager) stopPluginUnsafe(pluginID string) error {
	pluginInfo := pm.plugins[pluginID]
	pluginInstance := pm.instances[pluginID]

	if pluginInfo.Status != PluginStatusActive {
		return nil
	}

	if err := pluginInstance.Stop(); err != nil {
		pluginInfo.Status = PluginStatusError
		pluginInfo.Error = err.Error()
		return err
	}

	pluginInfo.Status = PluginStatusInactive
	pm.stats.ActivePlugins--
	pm.stats.InactivePlugins++

	return nil
}

// isTypeAllowed 检查插件类型是否允许
func (pm *PluginManager) isTypeAllowed(pluginType PluginType) bool {
	if len(pm.config.AllowedTypes) == 0 {
		return true
	}

	for _, allowedType := range pm.config.AllowedTypes {
		if allowedType == pluginType {
			return true
		}
	}
	return false
}

// isPluginAllowed 检查插件是否允许
func (pm *PluginManager) isPluginAllowed(pluginName string) bool {
	// 检查黑名单
	for _, blacklisted := range pm.config.Blacklist {
		if blacklisted == pluginName {
			return false
		}
	}

	// 检查白名单
	if len(pm.config.Whitelist) > 0 {
		for _, whitelisted := range pm.config.Whitelist {
			if whitelisted == pluginName {
				return true
			}
		}
		return false
	}

	return true
}

// generatePluginID 生成插件ID
func (pm *PluginManager) generatePluginID(pluginName string) string {
	baseID := pluginName
	counter := 1

	for {
		id := baseID
		if counter > 1 {
			id = fmt.Sprintf("%s_%d", baseID, counter)
		}

		if _, exists := pm.plugins[id]; !exists {
			return id
		}
		counter++
	}
}

// sortMiddlewares 排序中间件
func (pm *PluginManager) sortMiddlewares() {
	sort.Slice(pm.middlewares, func(i, j int) bool {
		return pm.middlewares[i].Priority() < pm.middlewares[j].Priority()
	})
}

// removeMiddleware 移除中间件
func (pm *PluginManager) removeMiddleware(pluginID string) {
	for i, mw := range pm.middlewares {
		if mw.Name() == pluginID {
			pm.middlewares = append(pm.middlewares[:i], pm.middlewares[i+1:]...)
			break
		}
	}
}

// healthCheckLoop 健康检查循环
func (pm *PluginManager) healthCheckLoop() {
	ticker := time.NewTicker(pm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (pm *PluginManager) performHealthCheck() {
	pm.mu.RLock()
	instances := make(map[string]Plugin)
	for id, instance := range pm.instances {
		instances[id] = instance
	}
	pm.mu.RUnlock()

	for pluginID, instance := range instances {
		if err := instance.HealthCheck(); err != nil {
			pm.mu.Lock()
			if pluginInfo, exists := pm.plugins[pluginID]; exists {
				pluginInfo.Status = PluginStatusError
				pluginInfo.Error = err.Error()
			}
			pm.mu.Unlock()

			if pm.hooks.OnError != nil {
				pm.hooks.OnError(instance, err)
			}
		}
	}
}

// watchLoop 监控循环
func (pm *PluginManager) watchLoop() {
	ticker := time.NewTicker(pm.config.WatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			// 这里应该实现文件系统监控逻辑
			// 检测插件文件变化并重新加载
		}
	}
}

// IsRunning 检查是否运行
func (pm *PluginManager) IsRunning() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.running
}

// 全局插件管理器
var globalPluginManager *PluginManager
var globalPluginManagerOnce sync.Once

// GetGlobalPluginManager 获取全局插件管理器
func GetGlobalPluginManager() *PluginManager {
	globalPluginManagerOnce.Do(func() {
		globalPluginManager = NewPluginManager(nil)
	})
	return globalPluginManager
}

// 便利函数

// LoadPlugin 使用全局管理器加载插件
func LoadPlugin(filePath string, config map[string]interface{}) error {
	return GetGlobalPluginManager().LoadPlugin(filePath, config)
}

// StartPlugin 使用全局管理器启动插件
func StartPlugin(pluginID string) error {
	return GetGlobalPluginManager().StartPlugin(pluginID)
}

// StopPlugin 使用全局管理器停止插件
func StopPlugin(pluginID string) error {
	return GetGlobalPluginManager().StopPlugin(pluginID)
}

// GetMiddlewares 使用全局管理器获取中间件
func GetMiddlewares() []MiddlewarePlugin {
	return GetGlobalPluginManager().GetMiddlewares()
}