// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// DynamicConfig `n�?type DynamicConfig struct {
	mu sync.RWMutex
	
	// `n
	Level      Level  `json:"level"`       // `n`nFormatter  string `json:"formatter"`   // `n
	Caller     bool   `json:"caller"`      // `n�?	SkipCaller int    `json:"skip_caller"` // `n
	
	// `n
	BufferSize      int           `json:"buffer_size"`      // `n�?	FlushInterval   time.Duration `json:"flush_interval"`   // `n
	MaxConcurrency  int           `json:"max_concurrency"`  // `n
	EnableMetrics   bool          `json:"enable_metrics"`   // `n
	EnableSampling  bool          `json:"enable_sampling"`  // `n
	SampleRate      float64       `json:"sample_rate"`      // `n�?	
	// `n
	Outputs []OutputConfig `json:"outputs"` // `n
	
	// `n
	Rotation RotationConfig `json:"rotation"` // `n
	
	// `n
	Hooks []HookConfig `json:"hooks"` // `n
	
	// `n
	Validation ValidationConfig `json:"validation"` // `n
	
	// `n
	Compression CompressionConfig `json:"compression"` // `n
	
	// `n
	Version   string    `json:"version"`    // `n
	UpdatedAt time.Time `json:"updated_at"` // `n
}

// OutputConfig `n`ntype OutputConfig struct {
	Type   string                 `json:"type"`   // `n: console, file, network, http
	Config map[string]interface{} `json:"config"` // `n
}

// RotationConfig `n`ntype RotationConfig struct {
	Enabled    bool          `json:"enabled"`     // `n
	MaxSize    int64         `json:"max_size"`    // `n�?`n)
	MaxAge     time.Duration `json:"max_age"`     // `n�?	MaxBackups int           `json:"max_backups"` // `n�?	Compress   bool          `json:"compress"`    // `n
	Schedule   string        `json:"schedule"`    // `n(cron`n)
}

// HookConfig `n`ntype HookConfig struct {
	Type    string                 `json:"type"`    // `n: slack, email, file, metrics, callback
	Levels  []Level                `json:"levels"`  // `n
	Enabled bool                   `json:"enabled"` // `n
	Config  map[string]interface{} `json:"config"`  // `n
}

// ValidationConfig `n`ntype ValidationConfig struct {
	Enabled        bool              `json:"enabled"`         // `n
	RequiredFields []string          `json:"required_fields"` // `n
	FieldTypes     map[string]string `json:"field_types"`    // `n
	MaxFieldCount  int               `json:"max_field_count"` // `n�?	MaxValueSize   int               `json:"max_value_size"`  // `n�?}

// CompressionConfig `n`ntype CompressionConfig struct {
	Enabled   bool   `json:"enabled"`   // `n
	Algorithm string `json:"algorithm"` // `n: gzip, lz4, zstd
	Level     int    `json:"level"`     // `n
	Threshold int64  `json:"threshold"` // `n�?`n)
}

// ConfigManager `n�?type ConfigManager struct {
	mu       sync.RWMutex
	config   *DynamicConfig
	filePath string
	watching int32 // `n，`n�?	
	// `n
	onConfigChange func(*DynamicConfig, *DynamicConfig) // `n
	onError        func(error)                         // `n
	
	// `n
	stopChan chan struct{}
	done     chan struct{}
}

// NewConfigManager `n�?func NewConfigManager(filePath string) *ConfigManager {
	return &ConfigManager{
		filePath: filePath,
		config:   getDefaultConfig(),
		stopChan: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// getDefaultConfig `n`nfunc getDefaultConfig() *DynamicConfig {
	return &DynamicConfig{
		Level:          InfoLevel,
		Formatter:      "text",
		Caller:         false,
		SkipCaller:     0,
		BufferSize:     4096,
		FlushInterval:  5 * time.Second,
		MaxConcurrency: 100,
		EnableMetrics:  true,
		EnableSampling: false,
		SampleRate:     1.0,
		Outputs: []OutputConfig{
			{
				Type: "console",
				Config: map[string]interface{}{
					"output": "stdout",
				},
			},
		},
		Rotation: RotationConfig{
			Enabled:    false,
			MaxSize:    100 * 1024 * 1024, // 100MB
			MaxAge:     30 * 24 * time.Hour, // 30�?			MaxBackups: 10,
			Compress:   true,
		},
		Validation: ValidationConfig{
			Enabled:       false,
			MaxFieldCount: 50,
			MaxValueSize:  1024,
		},
		Compression: CompressionConfig{
			Enabled:   false,
			Algorithm: "gzip",
			Level:     6,
			Threshold: 1024,
		},
		Version:   "1.0.0",
		UpdatedAt: time.Now(),
	}
}

// LoadConfig `n`nfunc (cm *ConfigManager) LoadConfig() error {
	if cm.filePath == "" {
		return nil // `n
	}
	
	data, err := ioutil.ReadFile(cm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// `n，`n`nreturn cm.SaveConfig()
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config DynamicConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// `n`nif err := cm.validateConfig(&config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	
	cm.mu.Lock()
	oldConfig := cm.config
	cm.config = &config
	cm.mu.Unlock()
	
	// `n`nif cm.onConfigChange != nil {
		cm.onConfigChange(oldConfig, &config)
	}
	
	return nil
}

// SaveConfig `n`nfunc (cm *ConfigManager) SaveConfig() error {
	if cm.filePath == "" {
		return fmt.Errorf("config file path not set")
	}
	
	cm.mu.RLock()
	config := *cm.config
	cm.mu.RUnlock()
	
	config.UpdatedAt = time.Now()
	
	data, err := json.MarshalIndent(&config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// `n
	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// `n
	tempFile := cm.filePath + ".tmp"
	if err := ioutil.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}
	
	if err := os.Rename(tempFile, cm.filePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename config file: %w", err)
	}
	
	return nil
}

// GetConfig `n`nfunc (cm *ConfigManager) GetConfig() *DynamicConfig {
	cm.mu.RLock()
	config := *cm.config
	cm.mu.RUnlock()
	return &config
}

// UpdateConfig `n`nfunc (cm *ConfigManager) UpdateConfig(updater func(*DynamicConfig)) error {
	cm.mu.Lock()
	oldConfig := *cm.config
	newConfig := *cm.config
	updater(&newConfig)
	
	// `n�?	if err := cm.validateConfig(&newConfig); err != nil {
		cm.mu.Unlock()
		return fmt.Errorf("invalid config update: %w", err)
	}
	
	newConfig.UpdatedAt = time.Now()
	cm.config = &newConfig
	cm.mu.Unlock()
	
	// `n`nif cm.onConfigChange != nil {
		cm.onConfigChange(&oldConfig, &newConfig)
	}
	
	// `n�?	if err := cm.SaveConfig(); err != nil && cm.onError != nil {
		cm.onError(fmt.Errorf("failed to save config: %w", err))
	}
	
	return nil
}

// SetCallbacks `n`nfunc (cm *ConfigManager) SetCallbacks(
	onConfigChange func(*DynamicConfig, *DynamicConfig),
	onError func(error),
) {
	cm.mu.Lock()
	cm.onConfigChange = onConfigChange
	cm.onError = onError
	cm.mu.Unlock()
}

// StartWatching `n�?func (cm *ConfigManager) StartWatching() error {
	if cm.filePath == "" {
		return fmt.Errorf("config file path not set")
	}
	
	if !atomic.CompareAndSwapInt32(&cm.watching, 0, 1) {
		return fmt.Errorf("already watching")
	}
	
	go cm.watchLoop()
	return nil
}

// StopWatching `n`nfunc (cm *ConfigManager) StopWatching() {
	if atomic.CompareAndSwapInt32(&cm.watching, 1, 0) {
		close(cm.stopChan)
		<-cm.done
		
		// `n
		cm.stopChan = make(chan struct{})
		cm.done = make(chan struct{})
	}
}

// watchLoop `n`nfunc (cm *ConfigManager) watchLoop() {
	defer close(cm.done)
	
	ticker := time.NewTicker(1 * time.Second) // `n�?	defer ticker.Stop()
	
	var lastModTime time.Time
	if info, err := os.Stat(cm.filePath); err == nil {
		lastModTime = info.ModTime()
	}
	
	for {
		select {
		case <-cm.stopChan:
			return
		case <-ticker.C:
			info, err := os.Stat(cm.filePath)
			if err != nil {
				if cm.onError != nil {
					cm.onError(fmt.Errorf("failed to stat config file: %w", err))
				}
				continue
			}
			
			if info.ModTime().After(lastModTime) {
				lastModTime = info.ModTime()
				
				// `n
				time.Sleep(100 * time.Millisecond)
				
				if err := cm.LoadConfig(); err != nil && cm.onError != nil {
					cm.onError(fmt.Errorf("failed to reload config: %w", err))
				}
			}
		}
	}
}

// validateConfig `n`nfunc (cm *ConfigManager) validateConfig(config *DynamicConfig) error {
	// `n`nif config.Level < TraceLevel || config.Level > PanicLevel {
		return fmt.Errorf("invalid log level: %d", config.Level)
	}
	
	// `n
	validFormatters := map[string]bool{
		"text":   true,
		"json":   true,
		"logfmt": true,
	}
	if !validFormatters[config.Formatter] {
		return fmt.Errorf("invalid formatter: %s", config.Formatter)
	}
	
	// `n�?	if config.BufferSize < 0 {
		return fmt.Errorf("buffer size cannot be negative: %d", config.BufferSize)
	}
	
	// `n`nif config.FlushInterval < 0 {
		return fmt.Errorf("flush interval cannot be negative: %v", config.FlushInterval)
	}
	
	// `n�?	if config.SampleRate < 0 || config.SampleRate > 1 {
		return fmt.Errorf("sample rate must be between 0 and 1: %f", config.SampleRate)
	}
	
	// `n`nif len(config.Outputs) == 0 {
		return fmt.Errorf("at least one output must be configured")
	}
	
	validOutputTypes := map[string]bool{
		"console": true,
		"file":    true,
		"network": true,
		"http":    true,
	}
	
	for i, output := range config.Outputs {
		if !validOutputTypes[output.Type] {
			return fmt.Errorf("invalid output type at index %d: %s", i, output.Type)
		}
	}
	
	// `n`nif config.Rotation.MaxSize < 0 {
		return fmt.Errorf("rotation max size cannot be negative: %d", config.Rotation.MaxSize)
	}
	
	if config.Rotation.MaxBackups < 0 {
		return fmt.Errorf("rotation max backups cannot be negative: %d", config.Rotation.MaxBackups)
	}
	
	// `n
	validAlgorithms := map[string]bool{
		"gzip": true,
		"lz4":  true,
		"zstd": true,
	}
	if config.Compression.Enabled && !validAlgorithms[config.Compression.Algorithm] {
		return fmt.Errorf("invalid compression algorithm: %s", config.Compression.Algorithm)
	}
	
	return nil
}

// IsWatching `n�?func (cm *ConfigManager) IsWatching() bool {
	return atomic.LoadInt32(&cm.watching) == 1
}

// ReloadConfig `n`nfunc (cm *ConfigManager) ReloadConfig() error {
	return cm.LoadConfig()
}

// GetConfigVersion `n`nfunc (cm *ConfigManager) GetConfigVersion() string {
	cm.mu.RLock()
	version := cm.config.Version
	cm.mu.RUnlock()
	return version
}

// GetLastUpdated `n�?func (cm *ConfigManager) GetLastUpdated() time.Time {
	cm.mu.RLock()
	updatedAt := cm.config.UpdatedAt
	cm.mu.RUnlock()
	return updatedAt
}