package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"
)

// SlackHook Slack钩子
type SlackHook struct {
	mu       sync.Mutex
	webhookURL string
	username   string
	channel    string
	levels     []Level
	client     *http.Client
}

// SlackHookConfig Slack钩子配置
type SlackHookConfig struct {
	WebhookURL string        // Webhook URL
	Username   string        // 用户名
	Channel    string        // 频道
	Levels     []Level       // 触发级别
	Timeout    time.Duration // 超时时间
}

// NewSlackHook 创建Slack钩子
func NewSlackHook(config *SlackHookConfig) *SlackHook {
	if config == nil {
		config = &SlackHookConfig{}
	}
	
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	
	if len(config.Levels) == 0 {
		config.Levels = []Level{ErrorLevel, FatalLevel, PanicLevel}
	}
	
	return &SlackHook{
		webhookURL: config.WebhookURL,
		username:   config.Username,
		channel:    config.Channel,
		levels:     config.Levels,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Levels 返回触发级别
func (h *SlackHook) Levels() []Level {
	return h.levels
}

// Fire 触发钩子
func (h *SlackHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.webhookURL == "" {
		return fmt.Errorf("slack webhook URL is not configured")
	}
	
	// 构建Slack消息
	message := h.buildSlackMessage(entry)
	
	// 发送消息
	return h.sendMessage(message)
}

// buildSlackMessage 构建Slack消息
func (h *SlackHook) buildSlackMessage(entry *Entry) map[string]interface{} {
	color := h.getLevelColor(entry.Level)
	
	attachment := map[string]interface{}{
		"color":     color,
		"title":     fmt.Sprintf("%s Log", entry.Level.String()),
		"text":      entry.Message,
		"timestamp": entry.Time.Unix(),
		"fields":    []map[string]interface{}{},
	}
	
	// 添加字段
	fields := attachment["fields"].([]map[string]interface{})
	
	if entry.Error != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Error",
			"value": entry.Error,
			"short": false,
		})
	}
	
	if entry.Caller != nil {
		fields = append(fields, map[string]interface{}{
			"title": "Location",
			"value": fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line),
			"short": true,
		})
	}
	
	for k, v := range entry.Fields {
		fields = append(fields, map[string]interface{}{
			"title": k,
			"value": fmt.Sprintf("%v", v),
			"short": true,
		})
	}
	
	attachment["fields"] = fields
	
	message := map[string]interface{}{
		"attachments": []map[string]interface{}{attachment},
	}
	
	if h.username != "" {
		message["username"] = h.username
	}
	
	if h.channel != "" {
		message["channel"] = h.channel
	}
	
	return message
}

// getLevelColor 获取级别颜色
func (h *SlackHook) getLevelColor(level Level) string {
	switch level {
	case DebugLevel, TraceLevel:
		return "#9E9E9E" // 灰色
	case InfoLevel:
		return "#2196F3" // 蓝色
	case WarnLevel:
		return "#FF9800" // 橙色
	case ErrorLevel:
		return "#F44336" // 红色
	case FatalLevel, PanicLevel:
		return "#9C27B0" // 紫色
	default:
		return "#607D8B" // 蓝灰色
	}
}

// sendMessage 发送消息
func (h *SlackHook) sendMessage(message map[string]interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}
	
	resp, err := h.client.Post(h.webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status: %d", resp.StatusCode)
	}
	
	return nil
}

// EmailHook 邮件钩子
type EmailHook struct {
	mu       sync.Mutex
	smtpHost string
	smtpPort int
	username string
	password string
	from     string
	to       []string
	subject  string
	levels   []Level
}

// EmailHookConfig 邮件钩子配置
type EmailHookConfig struct {
	SMTPHost string   // SMTP主机
	SMTPPort int      // SMTP端口
	Username string   // 用户名
	Password string   // 密码
	From     string   // 发件人
	To       []string // 收件人
	Subject  string   // 主题
	Levels   []Level  // 触发级别
}

// NewEmailHook 创建邮件钩子
func NewEmailHook(config *EmailHookConfig) *EmailHook {
	if config == nil {
		config = &EmailHookConfig{}
	}
	
	if len(config.Levels) == 0 {
		config.Levels = []Level{ErrorLevel, FatalLevel, PanicLevel}
	}
	
	if config.Subject == "" {
		config.Subject = "Application Log Alert"
	}
	
	return &EmailHook{
		smtpHost: config.SMTPHost,
		smtpPort: config.SMTPPort,
		username: config.Username,
		password: config.Password,
		from:     config.From,
		to:       config.To,
		subject:  config.Subject,
		levels:   config.Levels,
	}
}

// Levels 返回触发级别
func (h *EmailHook) Levels() []Level {
	return h.levels
}

// Fire 触发钩子
func (h *EmailHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.smtpHost == "" || len(h.to) == 0 {
		return fmt.Errorf("email configuration is incomplete")
	}
	
	// 构建邮件内容
	body := h.buildEmailBody(entry)
	
	// 发送邮件
	return h.sendEmail(body)
}

// buildEmailBody 构建邮件内容
func (h *EmailHook) buildEmailBody(entry *Entry) string {
	var buf bytes.Buffer
	
	buf.WriteString(fmt.Sprintf("Time: %s\n", entry.Time.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("Level: %s\n", entry.Level.String()))
	buf.WriteString(fmt.Sprintf("Message: %s\n", entry.Message))
	
	if entry.Error != "" {
		buf.WriteString(fmt.Sprintf("Error: %s\n", entry.Error))
	}
	
	if entry.Caller != nil {
		buf.WriteString(fmt.Sprintf("Location: %s:%d\n", entry.Caller.File, entry.Caller.Line))
		buf.WriteString(fmt.Sprintf("Function: %s\n", entry.Caller.Function))
	}
	
	if len(entry.Fields) > 0 {
		buf.WriteString("\nFields:\n")
		for k, v := range entry.Fields {
			buf.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
	}
	
	return buf.String()
}

// sendEmail 发送邮件
func (h *EmailHook) sendEmail(body string) error {
	auth := smtp.PlainAuth("", h.username, h.password, h.smtpHost)
	
	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s",
		strings.Join(h.to, ","), h.subject, body)
	
	addr := fmt.Sprintf("%s:%d", h.smtpHost, h.smtpPort)
	return smtp.SendMail(addr, auth, h.from, h.to, []byte(msg))
}

// FileHook 文件钩子
type FileHook struct {
	mu       sync.Mutex
	filename string
	levels   []Level
	file     *os.File
}

// FileHookConfig 文件钩子配置
type FileHookConfig struct {
	Filename string  // 文件名
	Levels   []Level // 触发级别
}

// NewFileHook 创建文件钩子
func NewFileHook(config *FileHookConfig) (*FileHook, error) {
	if config == nil {
		config = &FileHookConfig{}
	}
	
	if config.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	
	if len(config.Levels) == 0 {
		config.Levels = []Level{ErrorLevel, FatalLevel, PanicLevel}
	}
	
	file, err := os.OpenFile(config.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open hook file: %w", err)
	}
	
	return &FileHook{
		filename: config.Filename,
		levels:   config.Levels,
		file:     file,
	}, nil
}

// Levels 返回触发级别
func (h *FileHook) Levels() []Level {
	return h.levels
}

// Fire 触发钩子
func (h *FileHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 格式化日志条目
	formatter := &JSONFormatter{}
	data, err := formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}
	
	// 写入文件
	_, err = h.file.Write(data)
	return err
}

// Close 关闭钩子
func (h *FileHook) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.file != nil {
		return h.file.Close()
	}
	return nil
}

// MetricsHook 指标钩子
type MetricsHook struct {
	mu      sync.Mutex
	levels  []Level
	counters map[Level]int64
}

// NewMetricsHook 创建指标钩子
func NewMetricsHook(levels []Level) *MetricsHook {
	if len(levels) == 0 {
		levels = []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, FatalLevel, PanicLevel}
	}
	
	return &MetricsHook{
		levels:   levels,
		counters: make(map[Level]int64),
	}
}

// Levels 返回触发级别
func (h *MetricsHook) Levels() []Level {
	return h.levels
}

// Fire 触发钩子
func (h *MetricsHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.counters[entry.Level]++
	return nil
}

// GetCounters 获取计数器
func (h *MetricsHook) GetCounters() map[Level]int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	result := make(map[Level]int64)
	for k, v := range h.counters {
		result[k] = v
	}
	return result
}

// Reset 重置计数器
func (h *MetricsHook) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for k := range h.counters {
		h.counters[k] = 0
	}
}

// SyslogHook Syslog钩子
type SyslogHook struct {
	mu     sync.Mutex
	levels []Level
	writer Writer
}

// NewSyslogHook 创建Syslog钩子
func NewSyslogHook(levels []Level, writer Writer) *SyslogHook {
	if len(levels) == 0 {
		levels = []Level{InfoLevel, WarnLevel, ErrorLevel, FatalLevel, PanicLevel}
	}
	
	return &SyslogHook{
		levels: levels,
		writer: writer,
	}
}

// Levels 返回触发级别
func (h *SyslogHook) Levels() []Level {
	return h.levels
}

// Fire 触发钩子
func (h *SyslogHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 格式化为syslog格式
	message := fmt.Sprintf("<%d>%s %s[%d]: %s",
		h.getSyslogPriority(entry.Level),
		entry.Time.Format(time.RFC3339),
		"netcore-go",
		os.Getpid(),
		entry.Message)
	
	return h.writer.Write([]byte(message + "\n"))
}

// getSyslogPriority 获取syslog优先级
func (h *SyslogHook) getSyslogPriority(level Level) int {
	// Facility: 16 (local0), Severity根据日志级别
	facility := 16 << 3
	
	switch level {
	case TraceLevel, DebugLevel:
		return facility + 7 // Debug
	case InfoLevel:
		return facility + 6 // Info
	case WarnLevel:
		return facility + 4 // Warning
	case ErrorLevel:
		return facility + 3 // Error
	case FatalLevel:
		return facility + 2 // Critical
	case PanicLevel:
		return facility + 0 // Emergency
	default:
		return facility + 6 // Info
	}
}

// CallbackHook 回调钩子
type CallbackHook struct {
	levels   []Level
	callback func(*Entry) error
}

// NewCallbackHook 创建回调钩子
func NewCallbackHook(levels []Level, callback func(*Entry) error) *CallbackHook {
	return &CallbackHook{
		levels:   levels,
		callback: callback,
	}
}

// Levels 返回触发级别
func (h *CallbackHook) Levels() []Level {
	return h.levels
}

// Fire 触发钩子
func (h *CallbackHook) Fire(entry *Entry) error {
	if h.callback != nil {
		return h.callback(entry)
	}
	return nil
}

// ConditionalHook 条件钩子
type ConditionalHook struct {
	hook      Hook
	condition func(*Entry) bool
}

// NewConditionalHook 创建条件钩子
func NewConditionalHook(hook Hook, condition func(*Entry) bool) *ConditionalHook {
	return &ConditionalHook{
		hook:      hook,
		condition: condition,
	}
}

// Levels 返回触发级别
func (h *ConditionalHook) Levels() []Level {
	return h.hook.Levels()
}

// Fire 触发钩子
func (h *ConditionalHook) Fire(entry *Entry) error {
	if h.condition != nil && !h.condition(entry) {
		return nil
	}
	return h.hook.Fire(entry)
}

// RateLimitHook 限流钩子
type RateLimitHook struct {
	mu       sync.Mutex
	hook     Hook
	interval time.Duration
	lastFire map[Level]time.Time
}

// NewRateLimitHook 创建限流钩子
func NewRateLimitHook(hook Hook, interval time.Duration) *RateLimitHook {
	return &RateLimitHook{
		hook:     hook,
		interval: interval,
		lastFire: make(map[Level]time.Time),
	}
}

// Levels 返回触发级别
func (h *RateLimitHook) Levels() []Level {
	return h.hook.Levels()
}

// Fire 触发钩子
func (h *RateLimitHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	last, exists := h.lastFire[entry.Level]
	if exists && time.Since(last) < h.interval {
		return nil // 跳过，还在限流期内
	}
	
	h.lastFire[entry.Level] = time.Now()
	return h.hook.Fire(entry)
}