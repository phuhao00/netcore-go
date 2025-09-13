package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"github.com/BurntSushi/toml"
)

// ConfigFormat 配置文件格式
type ConfigFormat string

const (
	FormatJSON ConfigFormat = "json"
	FormatYAML ConfigFormat = "yaml"
	FormatTOML ConfigFormat = "toml"
	FormatENV  ConfigFormat = "env"
)

// Config 配置管理器
type Config struct {
	mu       sync.RWMutex
	data     map[string]interface{}
	format   ConfigFormat
	filePath string
	watchers []ConfigWatcher
	defaults map[string]interface{}
}

// ConfigWatcher 配置变更监听器
type ConfigWatcher interface {
	OnConfigChange(key string, oldValue, newValue interface{})
}

// ConfigOptions 配置选项
type ConfigOptions struct {
	Format   ConfigFormat
	FilePath string
	Defaults map[string]interface{}
	AutoReload bool
}

// NewConfig 创建新的配置管理器
func NewConfig(opts *ConfigOptions) *Config {
	if opts == nil {
		opts = &ConfigOptions{}
	}
	
	c := &Config{
		data:     make(map[string]interface{}),
		format:   opts.Format,
		filePath: opts.FilePath,
		defaults: opts.Defaults,
	}
	
	// 如果没有指定格式，根据文件扩展名推断
	if c.format == "" && c.filePath != "" {
		c.format = detectFormat(c.filePath)
	}
	
	// 设置默认值
	if c.defaults != nil {
		for k, v := range c.defaults {
			c.data[k] = v
		}
	}
	
	return c
}

// LoadFromFile 从文件加载配置
func (c *Config) LoadFromFile(filePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if filePath != "" {
		c.filePath = filePath
		c.format = detectFormat(filePath)
	}
	
	if c.filePath == "" {
		return fmt.Errorf("file path is required")
	}
	
	data, err := ioutil.ReadFile(c.filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	return c.parseData(data)
}

// LoadFromBytes 从字节数据加载配置
func (c *Config) LoadFromBytes(data []byte, format ConfigFormat) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.format = format
	return c.parseData(data)
}

// LoadFromEnv 从环境变量加载配置
func (c *Config) LoadFromEnv(prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := parts[0]
		value := parts[1]
		
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		
		// 移除前缀
		if prefix != "" {
			key = strings.TrimPrefix(key, prefix)
			key = strings.TrimPrefix(key, "_")
		}
		
		// 转换为小写并替换下划线为点
		key = strings.ToLower(key)
		key = strings.ReplaceAll(key, "_", ".")
		
		c.data[key] = parseEnvValue(value)
	}
	
	return nil
}

// SaveToFile 保存配置到文件
func (c *Config) SaveToFile(filePath string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if filePath != "" {
		c.filePath = filePath
		c.format = detectFormat(filePath)
	}
	
	if c.filePath == "" {
		return fmt.Errorf("file path is required")
	}
	
	data, err := c.marshalData()
	if err != nil {
		return fmt.Errorf("failed to marshal config data: %w", err)
	}
	
	// 确保目录存在
	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	return ioutil.WriteFile(c.filePath, data, 0644)
}

// Get 获取配置值
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.getValue(key)
}

// GetString 获取字符串配置值
func (c *Config) GetString(key string) string {
	value := c.Get(key)
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

// GetInt 获取整数配置值
func (c *Config) GetInt(key string) int {
	value := c.Get(key)
	if value == nil {
		return 0
	}
	
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}

// GetBool 获取布尔配置值
func (c *Config) GetBool(key string) bool {
	value := c.Get(key)
	if value == nil {
		return false
	}
	
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.ToLower(v) == "true" || v == "1"
	case int:
		return v != 0
	}
	return false
}

// GetDuration 获取时间间隔配置值
func (c *Config) GetDuration(key string) time.Duration {
	value := c.Get(key)
	if value == nil {
		return 0
	}
	
	switch v := value.(type) {
	case time.Duration:
		return v
	case string:
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	case int64:
		return time.Duration(v)
	}
	return 0
}

// Set 设置配置值
func (c *Config) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	oldValue := c.getValue(key)
	c.setValue(key, value)
	
	// 通知监听器
	for _, watcher := range c.watchers {
		watcher.OnConfigChange(key, oldValue, value)
	}
}

// AddWatcher 添加配置变更监听器
func (c *Config) AddWatcher(watcher ConfigWatcher) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.watchers = append(c.watchers, watcher)
}

// GetAll 获取所有配置
func (c *Config) GetAll() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]interface{})
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

// parseData 解析配置数据
func (c *Config) parseData(data []byte) error {
	switch c.format {
	case FormatJSON:
		return json.Unmarshal(data, &c.data)
	case FormatYAML:
		return yaml.Unmarshal(data, &c.data)
	case FormatTOML:
		return toml.Unmarshal(data, &c.data)
	default:
		return fmt.Errorf("unsupported config format: %s", c.format)
	}
}

// marshalData 序列化配置数据
func (c *Config) marshalData() ([]byte, error) {
	switch c.format {
	case FormatJSON:
		return json.MarshalIndent(c.data, "", "  ")
	case FormatYAML:
		return yaml.Marshal(c.data)
	case FormatTOML:
		buf := new(strings.Builder)
		encoder := toml.NewEncoder(buf)
		if err := encoder.Encode(c.data); err != nil {
			return nil, err
		}
		return []byte(buf.String()), nil
	default:
		return nil, fmt.Errorf("unsupported config format: %s", c.format)
	}
}

// getValue 获取嵌套键值
func (c *Config) getValue(key string) interface{} {
	keys := strings.Split(key, ".")
	current := c.data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			return current[k]
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

// setValue 设置嵌套键值
func (c *Config) setValue(key string, value interface{}) {
	keys := strings.Split(key, ".")
	current := c.data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			current[k] = value
			return
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			next = make(map[string]interface{})
			current[k] = next
			current = next
		}
	}
}

// detectFormat 根据文件扩展名检测格式
func detectFormat(filePath string) ConfigFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".toml":
		return FormatTOML
	default:
		return FormatJSON
	}
}

// parseEnvValue 解析环境变量值
func parseEnvValue(value string) interface{} {
	// 尝试解析为布尔值
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	
	// 尝试解析为整数
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	
	// 尝试解析为浮点数
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	
	// 返回字符串
	return value
}

// Unmarshal 将配置解析到结构体
func (c *Config) Unmarshal(v interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := json.Marshal(c.data)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, v)
}

// UnmarshalKey 将指定键的配置解析到结构体
func (c *Config) UnmarshalKey(key string, v interface{}) error {
	value := c.Get(key)
	if value == nil {
		return fmt.Errorf("key %s not found", key)
	}
	
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, v)
}

// Merge 合并另一个配置
func (c *Config) Merge(other *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	other.mu.RLock()
	defer other.mu.RUnlock()
	
	c.mergeMap(c.data, other.data)
}

// mergeMap 递归合并map
func (c *Config) mergeMap(dst, src map[string]interface{}) {
	for k, v := range src {
		if dstVal, exists := dst[k]; exists {
			if dstMap, ok := dstVal.(map[string]interface{}); ok {
				if srcMap, ok := v.(map[string]interface{}); ok {
					c.mergeMap(dstMap, srcMap)
					continue
				}
			}
		}
		dst[k] = v
	}
}

// Clone 克隆配置
func (c *Config) Clone() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	newConfig := &Config{
		data:     make(map[string]interface{}),
		format:   c.format,
		filePath: c.filePath,
		defaults: make(map[string]interface{}),
	}
	
	// 深拷贝数据
	data, _ := json.Marshal(c.data)
	json.Unmarshal(data, &newConfig.data)
	
	// 拷贝默认值
	for k, v := range c.defaults {
		newConfig.defaults[k] = v
	}
	
	return newConfig
}