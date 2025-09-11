// Package security 安全审计日志功能
// Author: NetCore-Go Team
// Created: 2024

package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEventType 审计事件类型
type AuditEventType string

const (
	// 认证事件
	EventTypeLogin          AuditEventType = "login"
	EventTypeLogout         AuditEventType = "logout"
	EventTypeLoginFailed    AuditEventType = "login_failed"
	EventTypeSessionExpired AuditEventType = "session_expired"

	// 授权事件
	EventTypeAccessGranted AuditEventType = "access_granted"
	EventTypeAccessDenied  AuditEventType = "access_denied"
	EventTypePermissionChanged AuditEventType = "permission_changed"

	// 系统事件
	EventTypeConfigChanged AuditEventType = "config_changed"
	EventTypeSystemStart   AuditEventType = "system_start"
	EventTypeSystemStop    AuditEventType = "system_stop"
	EventTypeSystemError   AuditEventType = "system_error"

	// 网络事件
	EventTypeConnectionOpen  AuditEventType = "connection_open"
	EventTypeConnectionClose AuditEventType = "connection_close"
	EventTypeDataTransfer    AuditEventType = "data_transfer"

	// 安全事件
	EventTypeSecurityViolation AuditEventType = "security_violation"
	EventTypeSuspiciousActivity AuditEventType = "suspicious_activity"
	EventTypeAttackDetected    AuditEventType = "attack_detected"
)

// AuditLevel 审计级别
type AuditLevel int

const (
	AuditLevelInfo AuditLevel = iota
	AuditLevelWarn
	AuditLevelError
	AuditLevelCritical
)

// String 返回审计级别字符串
func (al AuditLevel) String() string {
	switch al {
	case AuditLevelInfo:
		return "INFO"
	case AuditLevelWarn:
		return "WARN"
	case AuditLevelError:
		return "ERROR"
	case AuditLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// AuditEvent 审计事件
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   AuditEventType         `json:"event_type"`
	Level       AuditLevel             `json:"level"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	SourceIP    string                 `json:"source_ip,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Result      string                 `json:"result,omitempty"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	RiskScore   int                    `json:"risk_score,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

// AuditLogger 审计日志记录器接口
type AuditLogger interface {
	Log(event *AuditEvent) error
	Query(filter *AuditFilter) ([]*AuditEvent, error)
	Close() error
}

// AuditFilter 审计查询过滤器
type AuditFilter struct {
	StartTime   *time.Time       `json:"start_time,omitempty"`
	EndTime     *time.Time       `json:"end_time,omitempty"`
	EventTypes  []AuditEventType `json:"event_types,omitempty"`
	Levels      []AuditLevel     `json:"levels,omitempty"`
	UserID      string           `json:"user_id,omitempty"`
	Username    string           `json:"username,omitempty"`
	SourceIP    string           `json:"source_ip,omitempty"`
	Resource    string           `json:"resource,omitempty"`
	Action      string           `json:"action,omitempty"`
	MinRiskScore int             `json:"min_risk_score,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Limit       int              `json:"limit,omitempty"`
	Offset      int              `json:"offset,omitempty"`
}

// FileAuditLogger 文件审计日志记录器
type FileAuditLogger struct {
	filePath   string
	file       *os.File
	mu         sync.Mutex
	maxSize    int64
	maxFiles   int
	compress   bool
	encrypt    bool
	encryptKey []byte
}

// FileAuditConfig 文件审计配置
type FileAuditConfig struct {
	FilePath   string `json:"file_path"`
	MaxSize    int64  `json:"max_size"`    // 最大文件大小（字节）
	MaxFiles   int    `json:"max_files"`   // 最大文件数量
	Compress   bool   `json:"compress"`    // 是否压缩
	Encrypt    bool   `json:"encrypt"`     // 是否加密
	EncryptKey string `json:"encrypt_key"` // 加密密钥
}

// NewFileAuditLogger 创建文件审计日志记录器
func NewFileAuditLogger(config *FileAuditConfig) (*FileAuditLogger, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// 创建目录
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 打开文件
	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	logger := &FileAuditLogger{
		filePath: config.FilePath,
		file:     file,
		maxSize:  config.MaxSize,
		maxFiles: config.MaxFiles,
		compress: config.Compress,
		encrypt:  config.Encrypt,
	}

	// 设置加密密钥
	if config.Encrypt && config.EncryptKey != "" {
		logger.encryptKey = []byte(config.EncryptKey)
	}

	return logger, nil
}

// Log 记录审计事件
func (fal *FileAuditLogger) Log(event *AuditEvent) error {
	fal.mu.Lock()
	defer fal.mu.Unlock()

	// 检查文件大小并轮转
	if err := fal.rotateIfNeeded(); err != nil {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	// 序列化事件
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// 加密数据（如果启用）
	if fal.encrypt && len(fal.encryptKey) > 0 {
		// 这里可以实现加密逻辑
		// data = encrypt(data, fal.encryptKey)
	}

	// 写入文件
	data = append(data, '\n')
	if _, err := fal.file.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	// 强制刷新到磁盘
	return fal.file.Sync()
}

// Query 查询审计事件（简单实现）
func (fal *FileAuditLogger) Query(filter *AuditFilter) ([]*AuditEvent, error) {
	// 注意：这是一个简单的实现，生产环境建议使用数据库
	return nil, fmt.Errorf("query not implemented for file logger")
}

// Close 关闭审计日志记录器
func (fal *FileAuditLogger) Close() error {
	fal.mu.Lock()
	defer fal.mu.Unlock()

	if fal.file != nil {
		return fal.file.Close()
	}
	return nil
}

// rotateIfNeeded 检查并轮转日志文件
func (fal *FileAuditLogger) rotateIfNeeded() error {
	if fal.maxSize <= 0 {
		return nil
	}

	// 获取文件信息
	info, err := fal.file.Stat()
	if err != nil {
		return err
	}

	// 检查文件大小
	if info.Size() < fal.maxSize {
		return nil
	}

	// 关闭当前文件
	fal.file.Close()

	// 轮转文件
	if err := fal.rotateFiles(); err != nil {
		return err
	}

	// 打开新文件
	file, err := os.OpenFile(fal.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	fal.file = file
	return nil
}

// rotateFiles 轮转文件
func (fal *FileAuditLogger) rotateFiles() error {
	// 删除最旧的文件
	if fal.maxFiles > 0 {
		oldestFile := fmt.Sprintf("%s.%d", fal.filePath, fal.maxFiles)
		os.Remove(oldestFile)
	}

	// 重命名现有文件
	for i := fal.maxFiles - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", fal.filePath, i)
		newName := fmt.Sprintf("%s.%d", fal.filePath, i+1)
		os.Rename(oldName, newName)
	}

	// 重命名当前文件
	return os.Rename(fal.filePath, fal.filePath+".1")
}

// AuditManager 审计管理器
type AuditManager struct {
	loggers    []AuditLogger
	mu         sync.RWMutex
	buffer     []*AuditEvent
	bufferSize int
	flushInterval time.Duration
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// 风险评分规则
	riskRules map[AuditEventType]int

	// 事件过滤器
	filters []func(*AuditEvent) bool

	// 告警回调
	onAlert func(*AuditEvent)
}

// AuditManagerConfig 审计管理器配置
type AuditManagerConfig struct {
	BufferSize    int           `json:"buffer_size"`
	FlushInterval time.Duration `json:"flush_interval"`
	Loggers       []AuditLogger `json:"-"`
}

// NewAuditManager 创建审计管理器
func NewAuditManager(config *AuditManagerConfig) *AuditManager {
	if config == nil {
		config = &AuditManagerConfig{
			BufferSize:    1000,
			FlushInterval: 5 * time.Second,
		}
	}

	am := &AuditManager{
		loggers:       config.Loggers,
		buffer:        make([]*AuditEvent, 0, config.BufferSize),
		bufferSize:    config.BufferSize,
		flushInterval: config.FlushInterval,
		stopChan:      make(chan struct{}),
		riskRules:     make(map[AuditEventType]int),
		filters:       make([]func(*AuditEvent) bool, 0),
	}

	// 设置默认风险评分规则
	am.setDefaultRiskRules()

	// 启动刷新协程
	am.wg.Add(1)
	go am.flushLoop()

	return am
}

// setDefaultRiskRules 设置默认风险评分规则
func (am *AuditManager) setDefaultRiskRules() {
	am.riskRules[EventTypeLoginFailed] = 30
	am.riskRules[EventTypeAccessDenied] = 20
	am.riskRules[EventTypeSecurityViolation] = 80
	am.riskRules[EventTypeSuspiciousActivity] = 60
	am.riskRules[EventTypeAttackDetected] = 100
	am.riskRules[EventTypeSystemError] = 40
}

// AddLogger 添加审计日志记录器
func (am *AuditManager) AddLogger(logger AuditLogger) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.loggers = append(am.loggers, logger)
}

// LogEvent 记录审计事件
func (am *AuditManager) LogEvent(ctx context.Context, eventType AuditEventType, level AuditLevel, message string, details map[string]interface{}) {
	event := &AuditEvent{
		ID:        am.generateEventID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Level:     level,
		Message:   message,
		Details:   details,
	}

	// 从上下文中提取信息
	if userID, ok := ctx.Value("user_id").(string); ok {
		event.UserID = userID
	}
	if username, ok := ctx.Value("username").(string); ok {
		event.Username = username
	}
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		event.SessionID = sessionID
	}
	if sourceIP, ok := ctx.Value("source_ip").(string); ok {
		event.SourceIP = sourceIP
	}
	if userAgent, ok := ctx.Value("user_agent").(string); ok {
		event.UserAgent = userAgent
	}

	// 计算风险评分
	if score, exists := am.riskRules[eventType]; exists {
		event.RiskScore = score
	}

	// 应用过滤器
	for _, filter := range am.filters {
		if !filter(event) {
			return // 事件被过滤
		}
	}

	// 检查是否需要告警
	if event.RiskScore >= 70 && am.onAlert != nil {
		am.onAlert(event)
	}

	am.addToBuffer(event)
}

// LogAuth 记录认证事件
func (am *AuditManager) LogAuth(ctx context.Context, eventType AuditEventType, userID, username, sourceIP string, success bool, details map[string]interface{}) {
	level := AuditLevelInfo
	if !success {
		level = AuditLevelWarn
	}

	event := &AuditEvent{
		ID:        am.generateEventID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Level:     level,
		UserID:    userID,
		Username:  username,
		SourceIP:  sourceIP,
		Result:    fmt.Sprintf("%t", success),
		Details:   details,
	}

	if success {
		event.Message = fmt.Sprintf("User %s authentication successful", username)
	} else {
		event.Message = fmt.Sprintf("User %s authentication failed", username)
		event.RiskScore = 30
	}

	am.addToBuffer(event)
}

// LogAccess 记录访问事件
func (am *AuditManager) LogAccess(ctx context.Context, userID, username, resource, action, sourceIP string, granted bool, details map[string]interface{}) {
	eventType := EventTypeAccessGranted
	level := AuditLevelInfo
	if !granted {
		eventType = EventTypeAccessDenied
		level = AuditLevelWarn
	}

	event := &AuditEvent{
		ID:        am.generateEventID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Level:     level,
		UserID:    userID,
		Username:  username,
		SourceIP:  sourceIP,
		Resource:  resource,
		Action:    action,
		Result:    fmt.Sprintf("%t", granted),
		Details:   details,
	}

	if granted {
		event.Message = fmt.Sprintf("User %s accessed %s with action %s", username, resource, action)
	} else {
		event.Message = fmt.Sprintf("User %s denied access to %s with action %s", username, resource, action)
		event.RiskScore = 20
	}

	am.addToBuffer(event)
}

// LogSecurity 记录安全事件
func (am *AuditManager) LogSecurity(ctx context.Context, eventType AuditEventType, sourceIP, message string, riskScore int, details map[string]interface{}) {
	level := AuditLevelWarn
	if riskScore >= 80 {
		level = AuditLevelCritical
	} else if riskScore >= 50 {
		level = AuditLevelError
	}

	event := &AuditEvent{
		ID:        am.generateEventID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Level:     level,
		SourceIP:  sourceIP,
		Message:   message,
		RiskScore: riskScore,
		Details:   details,
	}

	// 添加安全标签
	event.Tags = []string{"security", "threat"}

	// 检查是否需要立即告警
	if riskScore >= 70 && am.onAlert != nil {
		am.onAlert(event)
	}

	am.addToBuffer(event)
}

// addToBuffer 添加事件到缓冲区
func (am *AuditManager) addToBuffer(event *AuditEvent) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.buffer = append(am.buffer, event)

	// 如果缓冲区满了，立即刷新
	if len(am.buffer) >= am.bufferSize {
		am.flushBuffer()
	}
}

// flushLoop 刷新循环
func (am *AuditManager) flushLoop() {
	defer am.wg.Done()

	ticker := time.NewTicker(am.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-am.stopChan:
			am.flushBuffer() // 最后一次刷新
			return
		case <-ticker.C:
			am.mu.Lock()
			am.flushBuffer()
			am.mu.Unlock()
		}
	}
}

// flushBuffer 刷新缓冲区
func (am *AuditManager) flushBuffer() {
	if len(am.buffer) == 0 {
		return
	}

	// 复制缓冲区
	events := make([]*AuditEvent, len(am.buffer))
	copy(events, am.buffer)
	am.buffer = am.buffer[:0] // 清空缓冲区

	// 写入所有日志记录器
	for _, logger := range am.loggers {
		for _, event := range events {
			if err := logger.Log(event); err != nil {
				// 记录错误，但不停止处理
				fmt.Printf("Failed to log audit event: %v\n", err)
			}
		}
	}
}

// AddFilter 添加事件过滤器
func (am *AuditManager) AddFilter(filter func(*AuditEvent) bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.filters = append(am.filters, filter)
}

// SetAlertHandler 设置告警处理器
func (am *AuditManager) SetAlertHandler(handler func(*AuditEvent)) {
	am.onAlert = handler
}

// SetRiskRule 设置风险评分规则
func (am *AuditManager) SetRiskRule(eventType AuditEventType, score int) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.riskRules[eventType] = score
}

// Close 关闭审计管理器
func (am *AuditManager) Close() error {
	close(am.stopChan)
	am.wg.Wait()

	// 关闭所有日志记录器
	for _, logger := range am.loggers {
		if err := logger.Close(); err != nil {
			return err
		}
	}

	return nil
}

// generateEventID 生成事件ID
func (am *AuditManager) generateEventID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), len(am.buffer))
}

// GetStats 获取审计统计信息
func (am *AuditManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return map[string]interface{}{
		"buffer_size":     len(am.buffer),
		"max_buffer_size": am.bufferSize,
		"loggers_count":   len(am.loggers),
		"filters_count":   len(am.filters),
		"risk_rules_count": len(am.riskRules),
	}
}