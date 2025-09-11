// Package longpoll HTTP长轮询实现
// Author: NetCore-Go Team
// Created: 2024

package longpoll

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/netcore-go/pkg/core"
)

// HTTPLongPollConfig HTTP长轮询配置
type HTTPLongPollConfig struct {
	Manager         *LongPollManager
	DefaultTimeout  time.Duration
	MaxTimeout      time.Duration
	SubscribePath   string
	UnsubscribePath string
	EventsPath      string
	PublishPath     string
	StatsPath       string
	CORSEnabled     bool
	AuthRequired    bool
	AuthValidator   func(token string) (string, error) // 返回clientID
}

// DefaultHTTPLongPollConfig 默认HTTP长轮询配置
func DefaultHTTPLongPollConfig() *HTTPLongPollConfig {
	return &HTTPLongPollConfig{
		Manager:         NewLongPollManager(1000),
		DefaultTimeout:  30 * time.Second,
		MaxTimeout:      5 * time.Minute,
		SubscribePath:   "/longpoll/subscribe",
		UnsubscribePath: "/longpoll/unsubscribe",
		EventsPath:      "/longpoll/events",
		PublishPath:     "/longpoll/publish",
		StatsPath:       "/longpoll/stats",
		CORSEnabled:     true,
		AuthRequired:    false,
	}
}

// HTTPLongPollMiddleware HTTP长轮询中间件
type HTTPLongPollMiddleware struct {
	config *HTTPLongPollConfig
}

// NewHTTPLongPollMiddleware 创建HTTP长轮询中间件
func NewHTTPLongPollMiddleware(config *HTTPLongPollConfig) *HTTPLongPollMiddleware {
	if config == nil {
		config = DefaultHTTPLongPollConfig()
	}
	return &HTTPLongPollMiddleware{
		config: config,
	}
}

// Process 处理HTTP长轮询请求
func (m *HTTPLongPollMiddleware) Process(ctx core.Context, next core.Handler) error {
	// 检查是否是长轮询相关的路径
	path, exists := ctx.Get("request_path")
	if !exists {
		return next(ctx)
	}

	pathStr, ok := path.(string)
	if !ok {
		return next(ctx)
	}

	// 处理长轮询路径
	switch {
	case pathStr == m.config.SubscribePath:
		return m.handleSubscribe(ctx)
	case pathStr == m.config.UnsubscribePath:
		return m.handleUnsubscribe(ctx)
	case pathStr == m.config.EventsPath:
		return m.handleEvents(ctx)
	case pathStr == m.config.PublishPath:
		return m.handlePublish(ctx)
	case pathStr == m.config.StatsPath:
		return m.handleStats(ctx)
	default:
		return next(ctx)
	}
}

// Name 获取中间件名称
func (m *HTTPLongPollMiddleware) Name() string {
	return "http_longpoll"
}

// Priority 获取中间件优先级
func (m *HTTPLongPollMiddleware) Priority() int {
	return 50
}

// handleSubscribe 处理订阅请求
func (m *HTTPLongPollMiddleware) handleSubscribe(ctx core.Context) error {
	// 检查HTTP方法
	method, _ := ctx.Get("request_method")
	if method != "POST" {
		return m.sendError(ctx, 405, "Method not allowed")
	}

	// 获取客户端ID
	clientID, err := m.getClientID(ctx)
	if err != nil {
		return m.sendError(ctx, 401, err.Error())
	}

	// 解析请求体
	var req struct {
		Topics  []string               `json:"topics"`
		Timeout int                    `json:"timeout"` // 秒
		Filters map[string]interface{} `json:"filters,omitempty"`
	}

	if err := m.parseJSONBody(ctx, &req); err != nil {
		return m.sendError(ctx, 400, "Invalid JSON body")
	}

	// 验证参数
	if len(req.Topics) == 0 {
		return m.sendError(ctx, 400, "Topics are required")
	}

	// 设置超时时间
	timeout := m.config.DefaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > m.config.MaxTimeout {
			timeout = m.config.MaxTimeout
		}
	}

	// 创建订阅
	sub := m.config.Manager.Subscribe(clientID, req.Topics, timeout)

	// 添加过滤器
	for key, value := range req.Filters {
		sub.AddFilter(key, value)
	}

	// 返回订阅信息
	response := map[string]interface{}{
		"subscription_id": sub.ID,
		"client_id":       sub.ClientID,
		"topics":          sub.Topics,
		"timeout":         int(sub.Timeout.Seconds()),
		"created_at":      sub.CreatedAt.Unix(),
	}

	return m.sendJSON(ctx, 200, response)
}

// handleUnsubscribe 处理取消订阅请求
func (m *HTTPLongPollMiddleware) handleUnsubscribe(ctx core.Context) error {
	// 检查HTTP方法
	method, _ := ctx.Get("request_method")
	if method != "POST" && method != "DELETE" {
		return m.sendError(ctx, 405, "Method not allowed")
	}

	// 获取客户端ID
	clientID, err := m.getClientID(ctx)
	if err != nil {
		return m.sendError(ctx, 401, err.Error())
	}

	// 解析请求体
	var req struct {
		SubscriptionID string `json:"subscription_id,omitempty"`
		All            bool   `json:"all,omitempty"`
	}

	if err := m.parseJSONBody(ctx, &req); err != nil {
		return m.sendError(ctx, 400, "Invalid JSON body")
	}

	var count int
	if req.All {
		// 取消客户端的所有订阅
		count = m.config.Manager.UnsubscribeClient(clientID)
	} else if req.SubscriptionID != "" {
		// 取消指定订阅
		if m.config.Manager.Unsubscribe(req.SubscriptionID) {
			count = 1
		}
	} else {
		return m.sendError(ctx, 400, "subscription_id or all=true is required")
	}

	response := map[string]interface{}{
		"unsubscribed_count": count,
		"client_id":          clientID,
	}

	return m.sendJSON(ctx, 200, response)
}

// handleEvents 处理获取事件请求
func (m *HTTPLongPollMiddleware) handleEvents(ctx core.Context) error {
	// 检查HTTP方法
	method, _ := ctx.Get("request_method")
	if method != "GET" {
		return m.sendError(ctx, 405, "Method not allowed")
	}

	// 获取查询参数
	subscriptionID := m.getQueryParam(ctx, "subscription_id")
	if subscriptionID == "" {
		return m.sendError(ctx, 400, "subscription_id is required")
	}

	lastEventID := m.getQueryParam(ctx, "last_event_id")
	timeoutStr := m.getQueryParam(ctx, "timeout")

	// 解析超时时间
	timeout := m.config.DefaultTimeout
	if timeoutStr != "" {
		if timeoutSec, err := strconv.Atoi(timeoutStr); err == nil && timeoutSec > 0 {
			timeout = time.Duration(timeoutSec) * time.Second
			if timeout > m.config.MaxTimeout {
				timeout = m.config.MaxTimeout
			}
		}
	}

	// 验证订阅是否存在
	sub, exists := m.config.Manager.GetSubscription(subscriptionID)
	if !exists {
		return m.sendError(ctx, 404, "Subscription not found")
	}

	// 验证客户端权限
	clientID, err := m.getClientID(ctx)
	if err != nil {
		return m.sendError(ctx, 401, err.Error())
	}

	if sub.ClientID != clientID {
		return m.sendError(ctx, 403, "Access denied")
	}

	// 获取事件
	events, err := m.config.Manager.GetEvents(subscriptionID, lastEventID, timeout)
	if err != nil {
		return m.sendError(ctx, 500, err.Error())
	}

	// 返回事件
	response := map[string]interface{}{
		"events":          events,
		"subscription_id": subscriptionID,
		"count":           len(events),
		"timestamp":       time.Now().Unix(),
	}

	if len(events) > 0 {
		response["last_event_id"] = events[len(events)-1].ID
	}

	return m.sendJSON(ctx, 200, response)
}

// handlePublish 处理发布事件请求
func (m *HTTPLongPollMiddleware) handlePublish(ctx core.Context) error {
	// 检查HTTP方法
	method, _ := ctx.Get("request_method")
	if method != "POST" {
		return m.sendError(ctx, 405, "Method not allowed")
	}

	// 获取客户端ID（发布者）
	clientID, err := m.getClientID(ctx)
	if err != nil {
		return m.sendError(ctx, 401, err.Error())
	}

	// 解析请求体
	var req struct {
		Type     string                 `json:"type"`
		Data     interface{}            `json:"data"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
		Targets  struct {
			ClientIDs []string `json:"client_ids,omitempty"`
			Topics    []string `json:"topics,omitempty"`
		} `json:"targets,omitempty"`
	}

	if err := m.parseJSONBody(ctx, &req); err != nil {
		return m.sendError(ctx, 400, "Invalid JSON body")
	}

	// 验证参数
	if req.Type == "" {
		return m.sendError(ctx, 400, "Event type is required")
	}

	// 创建事件
	event := NewEvent(req.Type, req.Data)
	event.SetMetadata("publisher", clientID)

	// 添加自定义元数据
	for key, value := range req.Metadata {
		event.SetMetadata(key, value)
	}

	deliveredCount := 0

	// 发布到指定客户端
	if len(req.Targets.ClientIDs) > 0 {
		for _, targetClientID := range req.Targets.ClientIDs {
			deliveredCount += m.config.Manager.PublishToClient(targetClientID, event)
		}
	} else if len(req.Targets.Topics) > 0 {
		// 发布到指定主题
		for _, topic := range req.Targets.Topics {
			event.Type = topic
			deliveredCount += m.config.Manager.Publish(event)
		}
	} else {
		// 发布到所有匹配的订阅
		deliveredCount = m.config.Manager.Publish(event)
	}

	// 返回发布结果
	response := map[string]interface{}{
		"event_id":        event.ID,
		"delivered_count": deliveredCount,
		"timestamp":       event.Timestamp,
		"publisher":       clientID,
	}

	return m.sendJSON(ctx, 200, response)
}

// handleStats 处理统计信息请求
func (m *HTTPLongPollMiddleware) handleStats(ctx core.Context) error {
	// 检查HTTP方法
	method, _ := ctx.Get("request_method")
	if method != "GET" {
		return m.sendError(ctx, 405, "Method not allowed")
	}

	// 获取统计信息
	stats := m.config.Manager.GetStats()

	return m.sendJSON(ctx, 200, stats)
}

// 辅助方法

// getClientID 获取客户端ID
func (m *HTTPLongPollMiddleware) getClientID(ctx core.Context) (string, error) {
	if !m.config.AuthRequired {
		// 如果不需要认证，使用连接ID作为客户端ID
		return ctx.Connection().ID(), nil
	}

	// 从Authorization头部获取token
	msg := ctx.Message()
	if msg.Type == 0 {
		return "", fmt.Errorf("no message in context")
	}

	auth, _ := msg.GetHeader("Authorization")
	if auth == "" {
		return "", fmt.Errorf("missing authorization header")
	}

	// 解析Bearer token
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", fmt.Errorf("invalid authorization header format")
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return "", fmt.Errorf("missing token")
	}

	// 验证token并获取客户端ID
	if m.config.AuthValidator != nil {
		return m.config.AuthValidator(token)
	}

	return token, nil // 简单情况下直接使用token作为客户端ID
}

// getQueryParam 获取查询参数
func (m *HTTPLongPollMiddleware) getQueryParam(ctx core.Context, key string) string {
	if value, exists := ctx.Get("query_" + key); exists {
		if valueStr, ok := value.(string); ok {
			return valueStr
		}
	}
	return ""
}

// parseJSONBody 解析JSON请求体
func (m *HTTPLongPollMiddleware) parseJSONBody(ctx core.Context, v interface{}) error {
	msg := ctx.Message()
	if msg.Type == 0 {
		return fmt.Errorf("no message in context")
	}

	data := msg.Data
	if len(data) == 0 {
		return fmt.Errorf("empty request body")
	}

	return json.Unmarshal(data, v)
}

// sendJSON 发送JSON响应
func (m *HTTPLongPollMiddleware) sendJSON(ctx core.Context, statusCode int, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ctx.Set("response_status", statusCode)
	ctx.Set("response_data", jsonData)
	ctx.Set("response_content_type", "application/json")

	// 设置CORS头部
	if m.config.CORSEnabled {
		ctx.Set("response_header_Access-Control-Allow-Origin", "*")
		ctx.Set("response_header_Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		ctx.Set("response_header_Access-Control-Allow-Headers", "Content-Type, Authorization")
	}

	return nil
}

// sendError 发送错误响应
func (m *HTTPLongPollMiddleware) sendError(ctx core.Context, statusCode int, message string) error {
	errorResponse := map[string]interface{}{
		"error":     true,
		"message":   message,
		"status":    statusCode,
		"timestamp": time.Now().Unix(),
	}

	return m.sendJSON(ctx, statusCode, errorResponse)
}

// GetManager 获取长轮询管理器
func (m *HTTPLongPollMiddleware) GetManager() *LongPollManager {
	return m.config.Manager
}

// SetAuthValidator 设置认证验证器
func (m *HTTPLongPollMiddleware) SetAuthValidator(validator func(string) (string, error)) {
	m.config.AuthValidator = validator
	m.config.AuthRequired = validator != nil
}

// EnableAuth 启用认证
func (m *HTTPLongPollMiddleware) EnableAuth(validator func(string) (string, error)) {
	m.SetAuthValidator(validator)
}

// DisableAuth 禁用认证
func (m *HTTPLongPollMiddleware) DisableAuth() {
	m.config.AuthRequired = false
	m.config.AuthValidator = nil
}

