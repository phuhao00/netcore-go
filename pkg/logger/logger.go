package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// String 返回日志级别字符串
func (l Level) String() string {
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

// ParseLevel 解析日志级别字符串
func ParseLevel(level string) (Level, error) {
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
		return InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// Entry 日志条目
type Entry struct {
	Time    time.Time              `json:"time"`
	Level   Level                  `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
	Caller  *CallerInfo            `json:"caller,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// CallerInfo 调用者信息
type CallerInfo struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// Formatter 日志格式化器接口
type Formatter interface {
	Format(entry *Entry) ([]byte, error)
}

// Writer 日志写入器接口
type Writer interface {
	Write(data []byte) error
	Close() error
}

// Hook 日志钩子接口
type Hook interface {
	Levels() []Level
	Fire(entry *Entry) error
}

// Logger 日志器
type Logger struct {
	mu        sync.RWMutex
	level     Level
	formatter Formatter
	writers   []Writer
	hooks     []Hook
	fields    map[string]interface{}
	caller    bool
	skipCaller int
}

// NewLogger 创建新的日志器
func NewLogger(config *Config) *Logger {
	if config == nil {
		config = &Config{}
	}
	
	logger := &Logger{
		level:      config.Level,
		formatter:  nil, // Will be set based on config.Formatter string
		writers:    nil, // Will be set based on config.Output string
		hooks:      nil,
		fields:     make(map[string]interface{}),
		caller:     false,
		skipCaller: 0,
	}
	
	// 设置格式化器
	switch config.Formatter {
	case "json":
		logger.formatter = &JSONFormatter{}
	case "text":
		logger.formatter = &TextFormatter{}
	default:
		logger.formatter = &TextFormatter{}
	}
	
	// 设置写入器
	switch config.Output {
	case "file":
		if config.FilePath != "" {
			if fileWriter, err := NewFileWriter(&FileWriterConfig{
				Filename: config.FilePath,
				MaxSize:  config.MaxSize,
			}); err == nil {
				logger.writers = []Writer{fileWriter}
			} else {
				logger.writers = []Writer{&ConsoleWriter{Output: os.Stdout}}
			}
		} else {
			logger.writers = []Writer{&ConsoleWriter{Output: os.Stdout}}
		}
	case "console":
		logger.writers = []Writer{&ConsoleWriter{Output: os.Stdout}}
	default:
		logger.writers = []Writer{&ConsoleWriter{Output: os.Stdout}}
	}
	
	// 字段初始化完成
	// config.Fields field does not exist in current Config struct
	
	return logger
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel 获取日志级别
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetFormatter 设置格式化器
func (l *Logger) SetFormatter(formatter Formatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = formatter
}

// AddWriter 添加写入器
func (l *Logger) AddWriter(writer Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writers = append(l.writers, writer)
}

// AddHook 添加钩子
func (l *Logger) AddHook(hook Hook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(map[string]interface{}{key: value})
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.RLock()
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	l.mu.RUnlock()
	
	for k, v := range fields {
		newFields[k] = v
	}
	
	return &Logger{
		level:      l.level,
		formatter:  l.formatter,
		writers:    l.writers,
		hooks:      l.hooks,
		fields:     newFields,
		caller:     l.caller,
		skipCaller: l.skipCaller,
	}
}

// WithError 添加错误字段
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// WithContext 从上下文添加字段
func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := make(map[string]interface{})
	
	// 从上下文中提取常见字段
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields["request_id"] = requestID
	}
	if userID := ctx.Value("user_id"); userID != nil {
		fields["user_id"] = userID
	}
	if traceID := ctx.Value("trace_id"); traceID != nil {
		fields["trace_id"] = traceID
	}
	
	if len(fields) == 0 {
		return l
	}
	
	return l.WithFields(fields)
}

// IsLevelEnabled 检查日志级别是否启用
func (l *Logger) IsLevelEnabled(level Level) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.level
}

// log 记录日志
func (l *Logger) log(level Level, msg string, err error) {
	if !l.IsLevelEnabled(level) {
		return
	}
	
	entry := &Entry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Fields:  make(map[string]interface{}),
	}
	
	// 复制字段
	l.mu.RLock()
	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	caller := l.caller
	skipCaller := l.skipCaller
	l.mu.RUnlock()
	
	// 添加错误信息
	if err != nil {
		entry.Error = err.Error()
	}
	
	// 添加调用者信息
	if caller {
		entry.Caller = l.getCaller(skipCaller + 3)
	}
	
	// 执行钩子
	l.mu.RLock()
	hooks := l.hooks
	l.mu.RUnlock()
	
	for _, hook := range hooks {
		for _, hookLevel := range hook.Levels() {
			if hookLevel == level {
				if err := hook.Fire(entry); err != nil {
					fmt.Fprintf(os.Stderr, "Hook error: %v\n", err)
				}
				break
			}
		}
	}
	
	// 格式化日志
	l.mu.RLock()
	formatter := l.formatter
	writers := l.writers
	l.mu.RUnlock()
	
	data, err := formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Format error: %v\n", err)
		return
	}
	
	// 写入日志
	for _, writer := range writers {
		if err := writer.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "Write error: %v\n", err)
		}
	}
}

// getCaller 获取调用者信息
func (l *Logger) getCaller(skip int) *CallerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}
	
	func_ := runtime.FuncForPC(pc)
	if func_ == nil {
		return nil
	}
	
	return &CallerInfo{
		File:     filepath.Base(file),
		Line:     line,
		Function: func_.Name(),
	}
}

// Trace 记录TRACE级别日志
func (l *Logger) Trace(msg string) {
	l.log(TraceLevel, msg, nil)
}

// Tracef 记录TRACE级别格式化日志
func (l *Logger) Tracef(format string, args ...interface{}) {
	l.log(TraceLevel, fmt.Sprintf(format, args...), nil)
}

// Debug 记录DEBUG级别日志
func (l *Logger) Debug(msg string) {
	l.log(DebugLevel, msg, nil)
}

// Debugf 记录DEBUG级别格式化日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Info 记录INFO级别日志
func (l *Logger) Info(msg string) {
	l.log(InfoLevel, msg, nil)
}

// Infof 记录INFO级别格式化日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warn 记录WARN级别日志
func (l *Logger) Warn(msg string) {
	l.log(WarnLevel, msg, nil)
}

// Warnf 记录WARN级别格式化日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Error 记录ERROR级别日志
func (l *Logger) Error(msg string) {
	l.log(ErrorLevel, msg, nil)
}

// Errorf 记录ERROR级别格式化日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// ErrorWithErr 记录ERROR级别日志（带错误）
func (l *Logger) ErrorWithErr(msg string, err error) {
	l.log(ErrorLevel, msg, err)
}

// Fatal 记录FATAL级别日志并退出程序
func (l *Logger) Fatal(msg string) {
	l.log(FatalLevel, msg, nil)
	os.Exit(1)
}

// Fatalf 记录FATAL级别格式化日志并退出程序
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// Panic 记录PANIC级别日志并panic
func (l *Logger) Panic(msg string) {
	l.log(PanicLevel, msg, nil)
	panic(msg)
}

// Panicf 记录PANIC级别格式化日志并panic
func (l *Logger) Panicf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.log(PanicLevel, msg, nil)
	panic(msg)
}

// Close 关闭日志器
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	var errs []error
	for _, writer := range l.writers {
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// 全局日志器
var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// SetGlobalLogger 设置全局日志器
func SetGlobalLogger(logger *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = logger
}

// GetGlobalLogger 获取全局日志器
func GetGlobalLogger() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalLogger == nil {
		globalLogger = NewLogger(nil)
	}
	return globalLogger
}

// 全局日志函数

// Trace 记录TRACE级别日志
func Trace(msg string) {
	GetGlobalLogger().Trace(msg)
}

// Tracef 记录TRACE级别格式化日志
func Tracef(format string, args ...interface{}) {
	GetGlobalLogger().Tracef(format, args...)
}

// Debug 记录DEBUG级别日志
func Debug(msg string) {
	GetGlobalLogger().Debug(msg)
}

// Debugf 记录DEBUG级别格式化日志
func Debugf(format string, args ...interface{}) {
	GetGlobalLogger().Debugf(format, args...)
}

// Info 记录INFO级别日志
func Info(msg string) {
	GetGlobalLogger().Info(msg)
}

// Infof 记录INFO级别格式化日志
func Infof(format string, args ...interface{}) {
	GetGlobalLogger().Infof(format, args...)
}

// Warn 记录WARN级别日志
func Warn(msg string) {
	GetGlobalLogger().Warn(msg)
}

// Warnf 记录WARN级别格式化日志
func Warnf(format string, args ...interface{}) {
	GetGlobalLogger().Warnf(format, args...)
}

// Error 记录ERROR级别日志
func Error(msg string) {
	GetGlobalLogger().Error(msg)
}

// Errorf 记录ERROR级别格式化日志
func Errorf(format string, args ...interface{}) {
	GetGlobalLogger().Errorf(format, args...)
}

// ErrorWithErr 记录ERROR级别日志（带错误）
func ErrorWithErr(msg string, err error) {
	GetGlobalLogger().ErrorWithErr(msg, err)
}

// Fatal 记录FATAL级别日志并退出程序
func Fatal(msg string) {
	GetGlobalLogger().Fatal(msg)
}

// Fatalf 记录FATAL级别格式化日志并退出程序
func Fatalf(format string, args ...interface{}) {
	GetGlobalLogger().Fatalf(format, args...)
}

// Panic 记录PANIC级别日志并panic
func Panic(msg string) {
	GetGlobalLogger().Panic(msg)
}

// Panicf 记录PANIC级别格式化日志并panic
func Panicf(format string, args ...interface{}) {
	GetGlobalLogger().Panicf(format, args...)
}

// WithField 添加字段
func WithField(key string, value interface{}) *Logger {
	return GetGlobalLogger().WithField(key, value)
}

// WithFields 添加多个字段
func WithFields(fields map[string]interface{}) *Logger {
	return GetGlobalLogger().WithFields(fields)
}

// WithError 添加错误字段
func WithError(err error) *Logger {
	return GetGlobalLogger().WithError(err)
}

// WithContext 从上下文添加字段
func WithContext(ctx context.Context) *Logger {
	return GetGlobalLogger().WithContext(ctx)
}