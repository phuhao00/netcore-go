package logger

import (
	"time"
)

// Config 日志配置
type Config struct {
	Level     Level         `json:"level"`
	Formatter string        `json:"formatter"`
	Output    string        `json:"output"`
	FilePath  string        `json:"file_path"`
	MaxSize   int64         `json:"max_size"`
	MaxAge    time.Duration `json:"max_age"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:     InfoLevel,
		Formatter: "text",
		Output:    "console",
		MaxSize:   100 * 1024 * 1024, // 100MB
		MaxAge:    30 * 24 * time.Hour, // 30天
	}
}