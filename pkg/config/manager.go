package config

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Manager 全局配置管理器
type Manager struct {
	mu            sync.RWMutex
	configs       map[string]*Config
	defaultConfig *Config
}

var (
	globalManager *Manager
	once          sync.Once
)

// GetManager 获取全局配置管理器
func GetManager() *Manager {
	once.Do(func() {
		globalManager = &Manager{
			configs: make(map[string]*Config),
		}
	})
	return globalManager
}

// Register 注册配置实例
func (m *Manager) Register(name string, config *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.configs[name] = config
	
	// 如果是第一个注册的配置，设为默认
	if m.defaultConfig == nil {
		m.defaultConfig = config
	}
}

// Get 获取指定名称的配置实例
func (m *Manager) Get(name string) *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.configs[name]
}

// Default 获取默认配置实例
func (m *Manager) Default() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.defaultConfig
}

// SetDefault 设置默认配置实例
func (m *Manager) SetDefault(config *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.defaultConfig = config
}

// LoadConfig 加载配置文件
func (m *Manager) LoadConfig(name, filePath string, opts *ConfigOptions) (*Config, error) {
	if opts == nil {
		opts = &ConfigOptions{}
	}
	opts.FilePath = filePath
	
	config := NewConfig(opts)
	if err := config.LoadFromFile(filePath); err != nil {
		return nil, err
	}
	
	m.Register(name, config)
	return config, nil
}

// LoadConfigFromEnv 从环境变量加载配置
func (m *Manager) LoadConfigFromEnv(name, prefix string) (*Config, error) {
	config := NewConfig(&ConfigOptions{
		Format: FormatENV,
	})
	
	if err := config.LoadFromEnv(prefix); err != nil {
		return nil, err
	}
	
	m.Register(name, config)
	return config, nil
}

// ConfigBuilder 配置构建器
type ConfigBuilder struct {
	config *Config
	err    error
}

// NewBuilder 创建配置构建器
func NewBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: NewConfig(nil),
	}
}

// WithFile 添加配置文件
func (b *ConfigBuilder) WithFile(filePath string) *ConfigBuilder {
	if b.err != nil {
		return b
	}
	
	if err := b.config.LoadFromFile(filePath); err != nil {
		b.err = err
	}
	return b
}

// WithEnv 添加环境变量配置
func (b *ConfigBuilder) WithEnv(prefix string) *ConfigBuilder {
	if b.err != nil {
		return b
	}
	
	envConfig := NewConfig(&ConfigOptions{Format: FormatENV})
	if err := envConfig.LoadFromEnv(prefix); err != nil {
		b.err = err
		return b
	}
	
	b.config.Merge(envConfig)
	return b
}

// WithDefaults 添加默认值
func (b *ConfigBuilder) WithDefaults(defaults map[string]interface{}) *ConfigBuilder {
	if b.err != nil {
		return b
	}
	
	for k, v := range defaults {
		if b.config.Get(k) == nil {
			b.config.Set(k, v)
		}
	}
	return b
}

// WithWatcher 添加配置监听器
func (b *ConfigBuilder) WithWatcher(watcher ConfigWatcher) *ConfigBuilder {
	if b.err != nil {
		return b
	}
	
	b.config.AddWatcher(watcher)
	return b
}

// Build 构建配置实例
func (b *ConfigBuilder) Build() (*Config, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.config, nil
}

// MustBuild 构建配置实例（panic on error）
func (b *ConfigBuilder) MustBuild() *Config {
	config, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build config: %v", err))
	}
	return config
}

// 便捷函数

// Load 加载配置文件
func Load(filePath string, opts *ConfigOptions) (*Config, error) {
	return GetManager().LoadConfig("default", filePath, opts)
}

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv(prefix string) (*Config, error) {
	return GetManager().LoadConfigFromEnv("default", prefix)
}

// Get 获取默认配置的值
func Get(key string) interface{} {
	if config := GetManager().Default(); config != nil {
		return config.Get(key)
	}
	return nil
}

// GetString 获取默认配置的字符串值
func GetString(key string) string {
	if config := GetManager().Default(); config != nil {
		return config.GetString(key)
	}
	return ""
}

// GetInt 获取默认配置的整数值
func GetInt(key string) int {
	if config := GetManager().Default(); config != nil {
		return config.GetInt(key)
	}
	return 0
}

// GetBool 获取默认配置的布尔值
func GetBool(key string) bool {
	if config := GetManager().Default(); config != nil {
		return config.GetBool(key)
	}
	return false
}

// GetDuration 获取默认配置的时间间隔值
func GetDuration(key string) time.Duration {
	if config := GetManager().Default(); config != nil {
		return config.GetDuration(key)
	}
	return 0
}

// Set 设置默认配置的值
func Set(key string, value interface{}) {
	if config := GetManager().Default(); config != nil {
		config.Set(key, value)
	}
}

// ConfigProfile 配置档案
type ConfigProfile struct {
	Name        string
	Description string
	Files       []string
	EnvPrefix   string
	Defaults    map[string]interface{}
}

// ProfileManager 配置档案管理器
type ProfileManager struct {
	mu       sync.RWMutex
	profiles map[string]*ConfigProfile
	active   string
}

// NewProfileManager 创建配置档案管理器
func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		profiles: make(map[string]*ConfigProfile),
	}
}

// RegisterProfile 注册配置档案
func (pm *ProfileManager) RegisterProfile(profile *ConfigProfile) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.profiles[profile.Name] = profile
}

// LoadProfile 加载配置档案
func (pm *ProfileManager) LoadProfile(name string) (*Config, error) {
	pm.mu.RLock()
	profile, exists := pm.profiles[name]
	pm.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("profile %s not found", name)
	}
	
	builder := NewBuilder()
	
	// 加载配置文件
	for _, file := range profile.Files {
		if _, err := os.Stat(file); err == nil {
			builder = builder.WithFile(file)
		}
	}
	
	// 加载环境变量
	if profile.EnvPrefix != "" {
		builder = builder.WithEnv(profile.EnvPrefix)
	}
	
	// 设置默认值
	if profile.Defaults != nil {
		builder = builder.WithDefaults(profile.Defaults)
	}
	
	config, err := builder.Build()
	if err != nil {
		return nil, err
	}
	
	pm.mu.Lock()
	pm.active = name
	pm.mu.Unlock()
	
	// 注册到全局管理器
	GetManager().Register(name, config)
	GetManager().SetDefault(config)
	
	return config, nil
}

// GetActiveProfile 获取当前活跃的配置档案名称
func (pm *ProfileManager) GetActiveProfile() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	return pm.active
}

// AutoDetectProfile 自动检测配置档案
func (pm *ProfileManager) AutoDetectProfile() (*Config, error) {
	// 检测环境变量
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = "development"
	}
	
	// 尝试加载对应的配置档案
	if config, err := pm.LoadProfile(env); err == nil {
		return config, nil
	}
	
	// 回退到默认配置
	return pm.LoadProfile("default")
}

// 预定义配置档案

// RegisterDefaultProfiles 注册默认配置档案
func (pm *ProfileManager) RegisterDefaultProfiles() {
	// 开发环境
	pm.RegisterProfile(&ConfigProfile{
		Name:        "development",
		Description: "Development environment configuration",
		Files: []string{
			"config/config.yaml",
			"config/development.yaml",
			"config.yaml",
			"development.yaml",
		},
		EnvPrefix: "APP",
		Defaults: map[string]interface{}{
			"debug":     true,
			"log.level": "debug",
			"server.port": 8080,
		},
	})
	
	// 测试环境
	pm.RegisterProfile(&ConfigProfile{
		Name:        "testing",
		Description: "Testing environment configuration",
		Files: []string{
			"config/config.yaml",
			"config/testing.yaml",
			"config.yaml",
			"testing.yaml",
		},
		EnvPrefix: "APP",
		Defaults: map[string]interface{}{
			"debug":     false,
			"log.level": "info",
			"server.port": 8081,
		},
	})
	
	// 生产环境
	pm.RegisterProfile(&ConfigProfile{
		Name:        "production",
		Description: "Production environment configuration",
		Files: []string{
			"config/config.yaml",
			"config/production.yaml",
			"config.yaml",
			"production.yaml",
		},
		EnvPrefix: "APP",
		Defaults: map[string]interface{}{
			"debug":     false,
			"log.level": "warn",
			"server.port": 80,
		},
	})
	
	// 默认配置
	pm.RegisterProfile(&ConfigProfile{
		Name:        "default",
		Description: "Default configuration",
		Files: []string{
			"config.yaml",
			"config.json",
			"config.toml",
		},
		EnvPrefix: "APP",
		Defaults: map[string]interface{}{
			"debug":       true,
			"log.level":   "info",
			"server.port": 8080,
			"server.host": "localhost",
		},
	})
}

// 全局配置档案管理器
var globalProfileManager *ProfileManager

// GetProfileManager 获取全局配置档案管理器
func GetProfileManager() *ProfileManager {
	if globalProfileManager == nil {
		globalProfileManager = NewProfileManager()
		globalProfileManager.RegisterDefaultProfiles()
	}
	return globalProfileManager
}

// LoadProfileConfig 加载配置档案
func LoadProfileConfig(profile string) (*Config, error) {
	return GetProfileManager().LoadProfile(profile)
}

// AutoLoadConfig 自动加载配置
func AutoLoadConfig() (*Config, error) {
	return GetProfileManager().AutoDetectProfile()
}