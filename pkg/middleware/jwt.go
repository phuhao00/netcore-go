// Package middleware 实现NetCore-Go网络库的扩展中间件
// Author: NetCore-Go Team
// Created: 2024

package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/phuhao00/netcore-go/pkg/core"
)

// JWTClaims JWT声明
type JWTClaims struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Exp      int64     `json:"exp"`
	Iat      int64     `json:"iat"`
	Iss      string    `json:"iss"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey     string
	Issuer        string
	Expiration    time.Duration
	SkipPaths     []string
	TokenLookup   string // "header:Authorization" or "query:token" or "cookie:token"
	TokenHeadName string // "Bearer"
}

// DefaultJWTConfig 默认JWT配置
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		SecretKey:     "netcore-go-secret-key",
		Issuer:        "netcore-go",
		Expiration:    24 * time.Hour,
		SkipPaths:     []string{"/login", "/register", "/health"},
		TokenLookup:   "header:Authorization",
		TokenHeadName: "Bearer",
	}
}

// JWTMiddleware JWT认证中间件
type JWTMiddleware struct {
	config *JWTConfig
}

// NewJWTMiddleware 创建JWT中间件
func NewJWTMiddleware(config *JWTConfig) *JWTMiddleware {
	if config == nil {
		config = DefaultJWTConfig()
	}
	return &JWTMiddleware{
		config: config,
	}
}

// Process 处理JWT认证
func (m *JWTMiddleware) Process(ctx core.Context, next core.Handler) error {
	// 检查是否跳过认证
	if m.shouldSkip(ctx) {
		return next(ctx)
	}

	// 提取token
	token, err := m.extractToken(ctx)
	if err != nil {
		ctx.Set("jwt_error", err.Error())
		ctx.Abort()
		return err
	}

	// 验证token
	claims, err := m.validateToken(token)
	if err != nil {
		ctx.Set("jwt_error", err.Error())
		ctx.Abort()
		return err
	}

	// 将用户信息存储到上下文
	ctx.Set("jwt_claims", claims)
	ctx.Set("user_id", claims.UserID)
	ctx.Set("username", claims.Username)
	ctx.Set("user_role", claims.Role)

	return next(ctx)
}

// Name 获取中间件名称
func (m *JWTMiddleware) Name() string {
	return "jwt"
}

// Priority 获取中间件优先级
func (m *JWTMiddleware) Priority() int {
	return 85
}

// shouldSkip 检查是否应该跳过认证
func (m *JWTMiddleware) shouldSkip(ctx core.Context) bool {
	// 这里需要根据具体的协议类型来判断路径
	// 对于HTTP协议，可以检查路径
	if path, exists := ctx.Get("request_path"); exists {
		if pathStr, ok := path.(string); ok {
			for _, skipPath := range m.config.SkipPaths {
				if pathStr == skipPath {
					return true
				}
			}
		}
	}
	return false
}

// extractToken 提取token
func (m *JWTMiddleware) extractToken(ctx core.Context) (string, error) {
	parts := strings.Split(m.config.TokenLookup, ":")
	if len(parts) != 2 {
		return "", errors.New("invalid token lookup format")
	}

	method, key := parts[0], parts[1]

	switch method {
	case "header":
		return m.extractFromHeader(ctx, key)
	case "query":
		return m.extractFromQuery(ctx, key)
	case "cookie":
		return m.extractFromCookie(ctx, key)
	default:
		return "", errors.New("unsupported token lookup method")
	}
}

// extractFromHeader 从头部提取token
func (m *JWTMiddleware) extractFromHeader(ctx core.Context, key string) (string, error) {
	msg := ctx.Message()
	// 检查消息是否有效
	if len(msg.Data) == 0 && len(msg.Headers) == 0 {
		return "", errors.New("no valid message in context")
	}

	auth, exists := msg.GetHeader(key)
	if !exists || auth == "" {
		return "", errors.New("missing authorization header")
	}

	if m.config.TokenHeadName != "" {
		prefix := m.config.TokenHeadName + " "
		if !strings.HasPrefix(auth, prefix) {
			return "", fmt.Errorf("invalid authorization header format, expected %s", prefix)
		}
		return strings.TrimPrefix(auth, prefix), nil
	}

	return auth, nil
}

// extractFromQuery 从查询参数提取token
func (m *JWTMiddleware) extractFromQuery(ctx core.Context, key string) (string, error) {
	// 这里需要根据具体协议实现
	if token, exists := ctx.Get("query_" + key); exists {
		if tokenStr, ok := token.(string); ok {
			return tokenStr, nil
		}
	}
	return "", errors.New("token not found in query")
}

// extractFromCookie 从Cookie提取token
func (m *JWTMiddleware) extractFromCookie(ctx core.Context, key string) (string, error) {
	// 这里需要根据具体协议实现
	if token, exists := ctx.Get("cookie_" + key); exists {
		if tokenStr, ok := token.(string); ok {
			return tokenStr, nil
		}
	}
	return "", errors.New("token not found in cookie")
}

// validateToken 验证token
func (m *JWTMiddleware) validateToken(tokenString string) (*JWTClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// 解码头部
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid token header")
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid token header format")
	}

	// 检查算法
	if alg, ok := header["alg"].(string); !ok || alg != "HS256" {
		return nil, errors.New("unsupported signing algorithm")
	}

	// 解码载荷
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload")
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid token payload format")
	}

	// 验证签名
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid token signature")
	}

	expectedSignature := m.sign(parts[0] + "." + parts[1])
	if !hmac.Equal(signature, expectedSignature) {
		return nil, errors.New("invalid token signature")
	}

	// 验证过期时间
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	// 验证发行者
	if m.config.Issuer != "" && claims.Iss != m.config.Issuer {
		return nil, errors.New("invalid token issuer")
	}

	return &claims, nil
}

// sign 签名
func (m *JWTMiddleware) sign(data string) []byte {
	h := hmac.New(sha256.New, []byte(m.config.SecretKey))
	h.Write([]byte(data))
	return h.Sum(nil)
}

// GenerateToken 生成JWT token
func (m *JWTMiddleware) GenerateToken(userID, username, role string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		Exp:      now.Add(m.config.Expiration).Unix(),
		Iat:      now.Unix(),
		Iss:      m.config.Issuer,
	}

	// 创建头部
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signature := m.sign(headerEncoded + "." + payloadEncoded)
	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)

	return headerEncoded + "." + payloadEncoded + "." + signatureEncoded, nil
}

