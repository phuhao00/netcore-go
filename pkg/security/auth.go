// Package security 访问控制和权限管理
// Author: NetCore-Go Team
// Created: 2024

package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Permission 权限定义
type Permission string

const (
	// 基础权限
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionDelete Permission = "delete"
	PermissionAdmin  Permission = "admin"

	// 网络权限
	PermissionConnect    Permission = "connect"
	PermissionDisconnect Permission = "disconnect"
	PermissionBroadcast  Permission = "broadcast"

	// 系统权限
	PermissionConfig   Permission = "config"
	PermissionMonitor  Permission = "monitor"
	PermissionShutdown Permission = "shutdown"
)

// Role 角色定义
type Role struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// User 用户定义
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Email        string    `json:"email"`
	Roles        []string  `json:"roles"`
	Enabled      bool      `json:"enabled"`
	LastLogin    time.Time `json:"last_login"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Session 会话定义
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Token     string                 `json:"token"`
	ExpiresAt time.Time              `json:"expires_at"`
	CreatedAt time.Time              `json:"created_at"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// AccessRule 访问规则
type AccessRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        RuleType   `json:"type"`
	Action      RuleAction `json:"action"`
	Conditions  []Condition `json:"conditions"`
	Priority    int        `json:"priority"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// RuleType 规则类型
type RuleType string

const (
	RuleTypeAllow RuleType = "allow"
	RuleTypeDeny  RuleType = "deny"
)

// RuleAction 规则动作
type RuleAction string

const (
	RuleActionAccept RuleAction = "accept"
	RuleActionReject RuleAction = "reject"
	RuleActionLimit  RuleAction = "limit"
)

// Condition 条件定义
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// AuthManager 认证管理器
type AuthManager struct {
	users    map[string]*User
	roles    map[string]*Role
	sessions map[string]*Session
	rules    []*AccessRule
	mu       sync.RWMutex

	// 配置
	sessionTimeout time.Duration
	maxSessions    int
	passwordPolicy *PasswordPolicy

	// 事件回调
	onLogin    func(*User, *Session)
	onLogout   func(*User, *Session)
	onAccess   func(*User, Permission, bool)
	onViolation func(*User, *AccessRule, string)
}

// PasswordPolicy 密码策略
type PasswordPolicy struct {
	MinLength    int  `json:"min_length"`
	RequireUpper bool `json:"require_upper"`
	RequireLower bool `json:"require_lower"`
	RequireDigit bool `json:"require_digit"`
	RequireSymbol bool `json:"require_symbol"`
	MaxAge       time.Duration `json:"max_age"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	SessionTimeout time.Duration   `json:"session_timeout"`
	MaxSessions    int             `json:"max_sessions"`
	PasswordPolicy *PasswordPolicy `json:"password_policy"`
}

// DefaultAuthConfig 返回默认认证配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		SessionTimeout: 24 * time.Hour,
		MaxSessions:    10,
		PasswordPolicy: &PasswordPolicy{
			MinLength:     8,
			RequireUpper:  true,
			RequireLower:  true,
			RequireDigit:  true,
			RequireSymbol: false,
			MaxAge:        90 * 24 * time.Hour, // 90天
		},
	}
}

// NewAuthManager 创建认证管理器
func NewAuthManager(config *AuthConfig) *AuthManager {
	if config == nil {
		config = DefaultAuthConfig()
	}

	am := &AuthManager{
		users:          make(map[string]*User),
		roles:          make(map[string]*Role),
		sessions:       make(map[string]*Session),
		rules:          make([]*AccessRule, 0),
		sessionTimeout: config.SessionTimeout,
		maxSessions:    config.MaxSessions,
		passwordPolicy: config.PasswordPolicy,
	}

	// 创建默认角色
	am.createDefaultRoles()

	return am
}

// createDefaultRoles 创建默认角色
func (am *AuthManager) createDefaultRoles() {
	// 管理员角色
	adminRole := &Role{
		Name:        "admin",
		Description: "系统管理员",
		Permissions: []Permission{
			PermissionRead, PermissionWrite, PermissionDelete, PermissionAdmin,
			PermissionConnect, PermissionDisconnect, PermissionBroadcast,
			PermissionConfig, PermissionMonitor, PermissionShutdown,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	am.roles["admin"] = adminRole

	// 用户角色
	userRole := &Role{
		Name:        "user",
		Description: "普通用户",
		Permissions: []Permission{
			PermissionRead, PermissionConnect,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	am.roles["user"] = userRole

	// 访客角色
	guestRole := &Role{
		Name:        "guest",
		Description: "访客用户",
		Permissions: []Permission{
			PermissionRead,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	am.roles["guest"] = guestRole
}

// CreateUser 创建用户
func (am *AuthManager) CreateUser(username, password, email string, roles []string) (*User, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 检查用户名是否已存在
	for _, user := range am.users {
		if user.Username == username {
			return nil, fmt.Errorf("username already exists")
		}
	}

	// 验证密码策略
	if err := am.validatePassword(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	// 哈希密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 生成用户ID
	userID := am.generateID()

	// 验证角色
	for _, roleName := range roles {
		if _, exists := am.roles[roleName]; !exists {
			return nil, fmt.Errorf("role %s does not exist", roleName)
		}
	}

	user := &User{
		ID:           userID,
		Username:     username,
		PasswordHash: string(passwordHash),
		Email:        email,
		Roles:        roles,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	am.users[userID] = user
	return user, nil
}

// Authenticate 用户认证
func (am *AuthManager) Authenticate(username, password string, clientIP, userAgent string) (*Session, error) {
	am.mu.RLock()
	var user *User
	for _, u := range am.users {
		if u.Username == username {
			user = u
			break
		}
	}
	am.mu.RUnlock()

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.Enabled {
		return nil, fmt.Errorf("user is disabled")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	// 检查会话数量限制
	am.mu.Lock()
	userSessions := 0
	for _, session := range am.sessions {
		if session.UserID == user.ID && session.ExpiresAt.After(time.Now()) {
			userSessions++
		}
	}

	if userSessions >= am.maxSessions {
		am.mu.Unlock()
		return nil, fmt.Errorf("maximum sessions exceeded")
	}

	// 创建会话
	session := &Session{
		ID:        am.generateID(),
		UserID:    user.ID,
		Token:     am.generateToken(),
		ExpiresAt: time.Now().Add(am.sessionTimeout),
		CreatedAt: time.Now(),
		IPAddress: clientIP,
		UserAgent: userAgent,
		Metadata:  make(map[string]interface{}),
	}

	am.sessions[session.ID] = session

	// 更新用户最后登录时间
	user.LastLogin = time.Now()
	user.UpdatedAt = time.Now()
	am.mu.Unlock()

	// 调用登录回调
	if am.onLogin != nil {
		am.onLogin(user, session)
	}

	return session, nil
}

// ValidateSession 验证会话
func (am *AuthManager) ValidateSession(token string) (*User, *Session, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var session *Session
	for _, s := range am.sessions {
		if s.Token == token {
			session = s
			break
		}
	}

	if session == nil {
		return nil, nil, fmt.Errorf("session not found")
	}

	if session.ExpiresAt.Before(time.Now()) {
		return nil, nil, fmt.Errorf("session expired")
	}

	user, exists := am.users[session.UserID]
	if !exists {
		return nil, nil, fmt.Errorf("user not found")
	}

	if !user.Enabled {
		return nil, nil, fmt.Errorf("user is disabled")
	}

	return user, session, nil
}

// CheckPermission 检查权限
func (am *AuthManager) CheckPermission(userID string, permission Permission) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[userID]
	if !exists || !user.Enabled {
		return false
	}

	// 检查用户角色权限
	for _, roleName := range user.Roles {
		if role, exists := am.roles[roleName]; exists {
			for _, perm := range role.Permissions {
				if perm == permission || perm == PermissionAdmin {
					return true
				}
			}
		}
	}

	return false
}

// CheckAccess 检查访问权限
func (am *AuthManager) CheckAccess(ctx context.Context, userID string, resource string, action string, clientIP string) bool {
	am.mu.RLock()
	user := am.users[userID]
	rules := make([]*AccessRule, len(am.rules))
	copy(rules, am.rules)
	am.mu.RUnlock()

	if user == nil || !user.Enabled {
		return false
	}

	// 按优先级排序规则
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority < rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	// 评估访问规则
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if am.evaluateRule(rule, user, resource, action, clientIP) {
			allowed := rule.Type == RuleTypeAllow

			// 调用访问回调
			if am.onAccess != nil {
				am.onAccess(user, Permission(action), allowed)
			}

			// 如果违反规则，调用违规回调
			if !allowed && am.onViolation != nil {
				am.onViolation(user, rule, fmt.Sprintf("Access denied for resource: %s, action: %s", resource, action))
			}

			return allowed
		}
	}

	// 默认拒绝
	return false
}

// evaluateRule 评估规则
func (am *AuthManager) evaluateRule(rule *AccessRule, user *User, resource, action, clientIP string) bool {
	for _, condition := range rule.Conditions {
		if !am.evaluateCondition(condition, user, resource, action, clientIP) {
			return false
		}
	}
	return true
}

// evaluateCondition 评估条件
func (am *AuthManager) evaluateCondition(condition Condition, user *User, resource, action, clientIP string) bool {
	var fieldValue interface{}

	switch condition.Field {
	case "user_id":
		fieldValue = user.ID
	case "username":
		fieldValue = user.Username
	case "roles":
		fieldValue = user.Roles
	case "resource":
		fieldValue = resource
	case "action":
		fieldValue = action
	case "ip":
		fieldValue = clientIP
	case "time":
		fieldValue = time.Now().Format("15:04")
	default:
		return false
	}

	switch condition.Operator {
	case "eq":
		return fieldValue == condition.Value
	case "ne":
		return fieldValue != condition.Value
	case "in":
		if values, ok := condition.Value.([]interface{}); ok {
			for _, v := range values {
				if fieldValue == v {
					return true
				}
			}
		}
		return false
	case "contains":
		if str, ok := fieldValue.(string); ok {
			if substr, ok := condition.Value.(string); ok {
				return strings.Contains(str, substr)
			}
		}
		return false
	case "ip_in_range":
		if cidr, ok := condition.Value.(string); ok {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				return false
			}
			ip := net.ParseIP(clientIP)
			return network.Contains(ip)
		}
		return false
	default:
		return false
	}
}

// Logout 用户登出
func (am *AuthManager) Logout(token string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	var session *Session
	var sessionID string
	for id, s := range am.sessions {
		if s.Token == token {
			session = s
			sessionID = id
			break
		}
	}

	if session == nil {
		return fmt.Errorf("session not found")
	}

	user := am.users[session.UserID]
	delete(am.sessions, sessionID)

	// 调用登出回调
	if am.onLogout != nil && user != nil {
		am.onLogout(user, session)
	}

	return nil
}

// validatePassword 验证密码策略
func (am *AuthManager) validatePassword(password string) error {
	policy := am.passwordPolicy
	if policy == nil {
		return nil
	}

	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters", policy.MinLength)
	}

	if policy.RequireUpper {
		hasUpper := false
		for _, r := range password {
			if r >= 'A' && r <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return fmt.Errorf("password must contain at least one uppercase letter")
		}
	}

	if policy.RequireLower {
		hasLower := false
		for _, r := range password {
			if r >= 'a' && r <= 'z' {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return fmt.Errorf("password must contain at least one lowercase letter")
		}
	}

	if policy.RequireDigit {
		hasDigit := false
		for _, r := range password {
			if r >= '0' && r <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return fmt.Errorf("password must contain at least one digit")
		}
	}

	if policy.RequireSymbol {
		hasSymbol := false
		symbols := "!@#$%^&*()_+-=[]{}|;:,.<>?"
		for _, r := range password {
			for _, s := range symbols {
				if r == s {
					hasSymbol = true
					break
				}
			}
			if hasSymbol {
				break
			}
		}
		if !hasSymbol {
			return fmt.Errorf("password must contain at least one symbol")
		}
	}

	return nil
}

// generateID 生成唯一ID
func (am *AuthManager) generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateToken 生成会话令牌
func (am *AuthManager) generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	hash := sha256.Sum256(b)
	return base64.URLEncoding.EncodeToString(hash[:])
}

// SetEventHandlers 设置事件处理器
func (am *AuthManager) SetEventHandlers(
	onLogin func(*User, *Session),
	onLogout func(*User, *Session),
	onAccess func(*User, Permission, bool),
	onViolation func(*User, *AccessRule, string),
) {
	am.onLogin = onLogin
	am.onLogout = onLogout
	am.onAccess = onAccess
	am.onViolation = onViolation
}

// CleanupExpiredSessions 清理过期会话
func (am *AuthManager) CleanupExpiredSessions() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for id, session := range am.sessions {
		if session.ExpiresAt.Before(now) {
			delete(am.sessions, id)
		}
	}
}

// GetUserStats 获取用户统计信息
func (am *AuthManager) GetUserStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	totalUsers := len(am.users)
	enabledUsers := 0
	activeSessions := 0

	for _, user := range am.users {
		if user.Enabled {
			enabledUsers++
		}
	}

	now := time.Now()
	for _, session := range am.sessions {
		if session.ExpiresAt.After(now) {
			activeSessions++
		}
	}

	return map[string]interface{}{
		"total_users":    totalUsers,
		"enabled_users":  enabledUsers,
		"active_sessions": activeSessions,
		"total_roles":    len(am.roles),
		"total_rules":    len(am.rules),
	}
}