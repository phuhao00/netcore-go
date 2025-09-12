// Package core 定义NetCore-Go网络库的生产级日志系统
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	TraceLevel LogLevel = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	case PanicLevel:
		return "PANIC"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel 解析日志级别字符串
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "TRACE":
		return TraceLevel, nil
	case "DEBUG":
		return DebugLevel, nil
	case "INFO":
		return InfoLevel, nil
	case "WARN", "WARNING":
		return WarnLevel, nil
	case "ERROR":
		return ErrorLevel, nil
	case "FATAL":
		return FatalLevel, nil
	case "PANIC":
		return PanicLevel, nil
	default:
		return InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// LogFormat 日志格式
type LogFormat string

const (
	JSONFormat LogFormat = "json"
	TextFormat LogFormat = "text"
)

// LogEntry 日志条目
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	ServiceName string                 `json:"service_name,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	File        string                 `json:"file,omitempty"`
	Line        int                    `json:"line,omitempty"`
	Function    string                 `json:"function,omitempty"`
}

// LoggerConfig 日志器配置
type LoggerConfig struct {
	// 基本配置
	Level       LogLevel  `json:"level"`
	Format      LogFormat `json:"format"`
	ServiceName string    `json:"service_name"`
	Version     string    `json:"version"`
	Environment string    `json:"environment"`

	// 输出配置
	Output      io.Writer `json:"-"`
	ErrorOutput io.Writer `json:"-"`
	FilePath    string    `json:"file_path,omitempty"`
	MaxFileSize int64     `json:"max_file_size"`
	MaxBackups  int       `json:"max_backups"`
	MaxAge      int       `json:"max_age"`
	Compress    bool      `json:"compress"`

	// 功能配置
	EnableStackTrace bool `json:"enable_stack_trace"`
	EnableColors     bool `json:"enable_colors"`
	EnableCaller     bool `json:"enable_caller"`
	EnableTimestamp  bool `json:"enable_timestamp"`
	TimestampFormat  string `json:"timestamp_format"`

	// 性能配置
	BufferSize   int           `json:"buffer_size"`
	FlushTimeout time.Duration `json:"flush_timeout"`
	AsyncLogging bool          `json:"async_logging"`

	// 过滤配置
	IgnoreFields []string `json:"ignore_fields"`
	SanitizeFields []string `json:"sanitize_fields"`
}

// DefaultLoggerConfig 返回默认日志器配置
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:           InfoLevel,
		Format:          JSONFormat,
		ServiceName:     "netcore-go",
		Version:         "1.0.0",
		Environment:     "development",
		Output:          os.Stdout,
		ErrorOutput:     os.Stderr,
		MaxFileSize:     100 * 1024 * 1024, // 100MB
		MaxBackups:      10,
		MaxAge:          30, // 30 days
		Compress:        true,
		EnableStackTrace: true,
		EnableColors:    false,
		EnableCaller:    true,
		EnableTimestamp: true,
		TimestampFormat: time.RFC3339Nano,
		BufferSize:      1024,
		FlushTimeout:    5 * time.Second,
		AsyncLogging:    false,
		IgnoreFields:    []string{"password", "secret", "token"},
		SanitizeFields:  []string{"email", "phone", "ip"},
	}
}

// ProductionLoggerConfig 返回生产环境日志器配置
func ProductionLoggerConfig(serviceName string) *LoggerConfig {
	config := DefaultLoggerConfig()
	config.Level = InfoLevel
	config.Format = JSONFormat
	config.ServiceName = serviceName
	config.Environment = "production"
	config.EnableColors = false
	config.EnableStackTrace = false
	config.AsyncLogging = true
	return config
}

// DevelopmentLoggerConfig 返回开发环境日志器配置
func DevelopmentLoggerConfig(serviceName string) *LoggerConfig {
	config := DefaultLoggerConfig()
	config.Level = DebugLevel
	config.Format = TextFormat
	config.ServiceName = serviceName
	config.Environment = "development"
	config.EnableColors = true
	config.EnableStackTrace = true
	config.AsyncLogging = false
	return config
}

// Logger 生产级日志器
type Logger struct {
	mu       sync.RWMutex
	config   *LoggerConfig
	fields   map[string]interface{}
	writers  []io.Writer
	buffer   chan *LogEntry
	done     chan struct{}
	wg       sync.WaitGroup
	running  bool
}

// NewLogger 创建新的日志器
func NewLogger(config *LoggerConfig) *Logger {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	logger := &Logger{
		config:  config,
		fields:  make(map[string]interface{}),
		writers: []io.Writer{},
		done:    make(chan struct{}),
	}

	// 设置输出
	if config.Output != nil {
		logger.writers = append(logger.writers, config.Output)
	}

	// 设置文件输出
	if config.FilePath != "" {
		if fileWriter, err := logger.createFileWriter(); err == nil {
			logger.writers = append(logger.writers, fileWriter)
		}
	}

	// 启动异步日志处理
	if config.AsyncLogging {
		logger.buffer = make(chan *LogEntry, config.BufferSize)
		logger.startAsyncProcessor()
	}

	return logger
}

// createFileWriter 创建文件写入器
func (l *Logger) createFileWriter() (io.Writer, error) {
	dir := filepath.Dir(l.config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	file, err := os.OpenFile(l.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return file, nil
}

// startAsyncProcessor 启动异步日志处理器
func (l *Logger) startAsyncProcessor() {
	l.running = true
	l.wg.Add(1)

	go func() {
		defer l.wg.Done()
		ticker := time.NewTicker(l.config.FlushTimeout)
		defer ticker.Stop()

		var entries []*LogEntry

		for {
			select {
			case entry := <-l.buffer:
				entries = append(entries, entry)
				if len(entries) >= l.config.BufferSize/2 {
					l.flushEntries(entries)
					entries = entries[:0]
				}

			case <-ticker.C:
				if len(entries) > 0 {
					l.flushEntries(entries)
					entries = entries[:0]
				}

			case <-l.done:
				// 处理剩余的日志条目
				for len(l.buffer) > 0 {
					entries = append(entries, <-l.buffer)
				}
				if len(entries) > 0 {
					l.flushEntries(entries)
				}
				return
			}
		}
	}()
}

// flushEntries 刷新日志条目
func (l *Logger) flushEntries(entries []*LogEntry) {
	for _, entry := range entries {
		l.writeEntry(entry)
	}
}

// Close 关闭日志器
func (l *Logger) Close() error {
	if l.running {
		close(l.done)
		l.wg.Wait()
		l.running = false
	}
	return nil
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = l.sanitizeValue(key, value)
	return &newLogger
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = l.sanitizeValue(k, v)
	}
	return &newLogger
}

// WithError 添加错误信息
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// WithContext 从上下文中提取信息
func (l *Logger) WithContext(ctx context.Context) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// 提取请求ID
	if requestID := ctx.Value("request_id"); requestID != nil {
		newLogger.fields["request_id"] = requestID
	}

	// 提取用户ID
	if userID := ctx.Value("user_id"); userID != nil {
		newLogger.fields["user_id"] = userID
	}

	// 提取跟踪ID
	if traceID := ctx.Value("trace_id"); traceID != nil {
		newLogger.fields["trace_id"] = traceID
	}

	return &newLogger
}

// sanitizeValue 清理敏感值
func (l *Logger) sanitizeValue(key string, value interface{}) interface{} {
	keyLower := strings.ToLower(key)
	
	// 忽略字段
	for _, ignoreField := range l.config.IgnoreFields {
		if strings.Contains(keyLower, strings.ToLower(ignoreField)) {
			return "[IGNORED]"
		}
	}

	// 清理字段
	for _, sanitizeField := range l.config.SanitizeFields {
		if strings.Contains(keyLower, strings.ToLower(sanitizeField)) {
			if str, ok := value.(string); ok && len(str) > 3 {
				return str[:3] + "***"
			}
			return "[SANITIZED]"
		}
	}

	return value
}

// isLevelEnabled 检查日志级别是否启用
func (l *Logger) isLevelEnabled(level LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.config.Level
}

// log 记录日志
func (l *Logger) log(level LogLevel, message string, err error) {
	if !l.isLevelEnabled(level) {
		return
	}

	entry := &LogEntry{
		Timestamp:   time.Now(),
		Level:       level,
		Message:     message,
		Fields:      make(map[string]interface{}),
		ServiceName: l.config.ServiceName,
	}

	// 复制字段
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// 添加错误信息
	if err != nil {
		entry.Error = err.Error()
		if l.config.EnableStackTrace && level >= ErrorLevel {
			entry.StackTrace = l.getStackTrace()
		}
	}

	// 添加调用者信息
	if l.config.EnableCaller {
		if file, line, function := l.getCaller(); file != "" {
			entry.File = file
			entry.Line = line
			entry.Function = function
		}
	}

	// 异步或同步写入
	if l.config.AsyncLogging && l.buffer != nil {
		select {
		case l.buffer <- entry:
		default:
			// 缓冲区满，直接写入
			l.writeEntry(entry)
		}
	} else {
		l.writeEntry(entry)
	}
}

// getCaller 获取调用者信息
func (l *Logger) getCaller() (string, int, string) {
	// 跳过日志框架的调用栈
	for i := 3; i < 10; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		// 跳过日志包内的调用
		if !strings.Contains(file, "logger") && !strings.Contains(file, "logging") {
			fn := runtime.FuncForPC(pc)
			funcName := ""
			if fn != nil {
				funcName = fn.Name()
			}
			return filepath.Base(file), line, funcName
		}
	}
	return "", 0, ""
}

// getStackTrace 获取堆栈跟踪
func (l *Logger) getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// writeEntry 写入日志条目
func (l *Logger) writeEntry(entry *LogEntry) {
	var output []byte
	var err error

	switch l.config.Format {
	case JSONFormat:
		output, err = json.Marshal(entry)
		if err != nil {
			output = []byte(fmt.Sprintf(`{"error":"failed to marshal log entry: %v"}`, err))
		}
		output = append(output, '\n')

	case TextFormat:
		output = []byte(l.formatTextEntry(entry))

	default:
		output = []byte(fmt.Sprintf("[%s] %s: %s\n", entry.Timestamp.Format(time.RFC3339), entry.Level, entry.Message))
	}

	// 写入到所有输出
	for _, writer := range l.writers {
		writer.Write(output)
	}
}

// formatTextEntry 格式化文本日志条目
func (l *Logger) formatTextEntry(entry *LogEntry) string {
	var builder strings.Builder

	// 时间戳
	if l.config.EnableTimestamp {
		builder.WriteString(entry.Timestamp.Format(l.config.TimestampFormat))
		builder.WriteString(" ")
	}

	// 日志级别
	levelStr := entry.Level.String()
	if l.config.EnableColors {
		levelStr = l.colorizeLevel(levelStr, entry.Level)
	}
	builder.WriteString(fmt.Sprintf("[%s]", levelStr))

	// 服务名称
	if entry.ServiceName != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", entry.ServiceName))
	}

	// 调用者信息
	if l.config.EnableCaller && entry.File != "" {
		builder.WriteString(fmt.Sprintf(" %s:%d", entry.File, entry.Line))
	}

	// 消息
	builder.WriteString(fmt.Sprintf(": %s", entry.Message))

	// 字段
	if len(entry.Fields) > 0 {
		builder.WriteString(" {")
		first := true
		for k, v := range entry.Fields {
			if !first {
				builder.WriteString(", ")
			}
			builder.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
		builder.WriteString("}")
	}

	// 错误信息
	if entry.Error != "" {
		builder.WriteString(fmt.Sprintf(" error=%s", entry.Error))
	}

	builder.WriteString("\n")
	return builder.String()
}

// colorizeLevel 为日志级别添加颜色
func (l *Logger) colorizeLevel(level string, logLevel LogLevel) string {
	if !l.config.EnableColors {
		return level
	}

	switch logLevel {
	case TraceLevel:
		return fmt.Sprintf("\033[37m%s\033[0m", level) // 白色
	case DebugLevel:
		return fmt.Sprintf("\033[36m%s\033[0m", level) // 青色
	case InfoLevel:
		return fmt.Sprintf("\033[32m%s\033[0m", level) // 绿色
	case WarnLevel:
		return fmt.Sprintf("\033[33m%s\033[0m", level) // 黄色
	case ErrorLevel:
		return fmt.Sprintf("\033[31m%s\033[0m", level) // 红色
	case FatalLevel:
		return fmt.Sprintf("\033[35m%s\033[0m", level) // 紫色
	case PanicLevel:
		return fmt.Sprintf("\033[41m%s\033[0m", level) // 红色背景
	default:
		return level
	}
}

// 日志方法

// Trace 记录TRACE级别日志
func (l *Logger) Trace(message string) {
	l.log(TraceLevel, message, nil)
}

// Tracef 记录TRACE级别格式化日志
func (l *Logger) Tracef(format string, args ...interface{}) {
	l.log(TraceLevel, fmt.Sprintf(format, args...), nil)
}

// Debug 记录DEBUG级别日志
func (l *Logger) Debug(message string) {
	l.log(DebugLevel, message, nil)
}

// Debugf 记录DEBUG级别格式化日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Info 记录INFO级别日志
func (l *Logger) Info(message string) {
	l.log(InfoLevel, message, nil)
}

// Infof 记录INFO级别格式化日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warn 记录WARN级别日志
func (l *Logger) Warn(message string) {
	l.log(WarnLevel, message, nil)
}

// Warnf 记录WARN级别格式化日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Error 记录ERROR级别日志
func (l *Logger) Error(message string) {
	l.log(ErrorLevel, message, nil)
}

// Errorf 记录ERROR级别格式化日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// ErrorWithErr 记录ERROR级别日志（带错误）
func (l *Logger) ErrorWithErr(message string, err error) {
	l.log(ErrorLevel, message, err)
}

// Fatal 记录FATAL级别日志并退出程序
func (l *Logger) Fatal(message string) {
	l.log(FatalLevel, message, nil)
	l.Close()
	os.Exit(1)
}

// Fatalf 记录FATAL级别格式化日志并退出程序
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
	l.Close()
	os.Exit(1)
}

// Panic 记录PANIC级别日志并panic
func (l *Logger) Panic(message string) {
	l.log(PanicLevel, message, nil)
	panic(message)
}

// Panicf 记录PANIC级别格式化日志并panic
func (l *Logger) Panicf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.log(PanicLevel, msg, nil)
	panic(msg)
}

// 全局日志器
var globalLogger *Logger
var globalLoggerMu sync.RWMutex

// SetGlobalLogger 设置全局日志器
func SetGlobalLogger(logger *Logger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	if globalLogger != nil {
		globalLogger.Close()
	}
	globalLogger = logger
}

// GetGlobalLogger 获取全局日志器
func GetGlobalLogger() *Logger {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	if globalLogger == nil {
		globalLogger = NewLogger(DefaultLoggerConfig())
	}
	return globalLogger
}

// 全局日志函数

// Trace 记录TRACE级别日志
func Trace(message string) {
	GetGlobalLogger().Trace(message)
}

// Tracef 记录TRACE级别格式化日志
func Tracef(format string, args ...interface{}) {
	GetGlobalLogger().Tracef(format, args...)
}

// Debug 记录DEBUG级别日志
func Debug(message string) {
	GetGlobalLogger().Debug(message)
}

// Debugf 记录DEBUG级别格式化日志
func Debugf(format string, args ...interface{}) {
	GetGlobalLogger().Debugf(format, args...)
}

// Info 记录INFO级别日志
func Info(message string) {
	GetGlobalLogger().Info(message)
}

// Infof 记录INFO级别格式化日志
func Infof(format string, args ...interface{}) {
	GetGlobalLogger().Infof(format, args...)
}

// Warn 记录WARN级别日志
func Warn(message string) {
	GetGlobalLogger().Warn(message)
}

// Warnf 记录WARN级别格式化日志
func Warnf(format string, args ...interface{}) {
	GetGlobalLogger().Warnf(format, args...)
}

// Error 记录ERROR级别日志
func Error(message string) {
	GetGlobalLogger().Error(message)
}

// Errorf 记录ERROR级别格式化日志
func Errorf(format string, args ...interface{}) {
	GetGlobalLogger().Errorf(format, args...)
}

// ErrorWithErr 记录ERROR级别日志（带错误）
func ErrorWithErr(message string, err error) {
	GetGlobalLogger().ErrorWithErr(message, err)
}

// Fatal 记录FATAL级别日志并退出程序
func Fatal(message string) {
	GetGlobalLogger().Fatal(message)
}

// Fatalf 记录FATAL级别格式化日志并退出程序
func Fatalf(format string, args ...interface{}) {
	GetGlobalLogger().Fatalf(format, args...)
}

// Panic 记录PANIC级别日志并panic
func Panic(message string) {
	GetGlobalLogger().Panic(message)
}

// Panicf 记录PANIC级别格式化日志并panic
func Panicf(format string, args ...interface{}) {
	GetGlobalLogger().Panicf(format, args...)
}