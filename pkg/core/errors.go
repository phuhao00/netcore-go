// Package core 定义NetCore-Go网络库的错误处理系统
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"fmt"
	"net/http"
	"time"
)

// ErrorCode 错误代码类型
type ErrorCode string

// 系统错误代码定义
const (
	// 通用错误代码
	ErrCodeInternal        ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidRequest  ErrorCode = "INVALID_REQUEST"
	ErrCodeInvalidConfig   ErrorCode = "INVALID_CONFIG"
	ErrCodeTimeout         ErrorCode = "TIMEOUT"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden       ErrorCode = "FORBIDDEN"
	ErrCodeRateLimited     ErrorCode = "RATE_LIMITED"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"

	// 连接相关错误代码
	ErrCodeConnectionFailed   ErrorCode = "CONNECTION_FAILED"
	ErrCodeConnectionClosed   ErrorCode = "CONNECTION_CLOSED"
	ErrCodeConnectionTimeout  ErrorCode = "CONNECTION_TIMEOUT"
	ErrCodeConnectionRefused  ErrorCode = "CONNECTION_REFUSED"
	ErrCodeConnectionReset    ErrorCode = "CONNECTION_RESET"
	ErrCodeTLSHandshakeFailed ErrorCode = "TLS_HANDSHAKE_FAILED"

	// 服务器相关错误代码
	ErrCodeServerNotRunning   ErrorCode = "SERVER_NOT_RUNNING"
	ErrCodeServerRunning      ErrorCode = "SERVER_ALREADY_RUNNING"
	ErrCodeServerStartFailed  ErrorCode = "SERVER_START_FAILED"
	ErrCodeServerStopFailed   ErrorCode = "SERVER_STOP_FAILED"
	ErrCodePortInUse          ErrorCode = "PORT_IN_USE"

	// 协议相关错误代码
	ErrCodeProtocolError      ErrorCode = "PROTOCOL_ERROR"
	ErrCodeInvalidMessage     ErrorCode = "INVALID_MESSAGE"
	ErrCodeMessageTooLarge    ErrorCode = "MESSAGE_TOO_LARGE"
	ErrCodeUnsupportedProtocol ErrorCode = "UNSUPPORTED_PROTOCOL"

	// 服务发现相关错误代码
	ErrCodeServiceNotFound    ErrorCode = "SERVICE_NOT_FOUND"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeDiscoveryFailed    ErrorCode = "DISCOVERY_FAILED"
	ErrCodeRegistrationFailed ErrorCode = "REGISTRATION_FAILED"

	// 负载均衡相关错误代码
	ErrCodeNoHealthyBackends  ErrorCode = "NO_HEALTHY_BACKENDS"
	ErrCodeLoadBalancerFailed ErrorCode = "LOAD_BALANCER_FAILED"

	// 数据库相关错误代码
	ErrCodeDatabaseConnection ErrorCode = "DATABASE_CONNECTION_ERROR"
	ErrCodeDatabaseQuery      ErrorCode = "DATABASE_QUERY_ERROR"
	ErrCodeDatabaseTransaction ErrorCode = "DATABASE_TRANSACTION_ERROR"

	// 缓存相关错误代码
	ErrCodeCacheConnection    ErrorCode = "CACHE_CONNECTION_ERROR"
	ErrCodeCacheOperation     ErrorCode = "CACHE_OPERATION_ERROR"

	// 认证授权相关错误代码
	ErrCodeInvalidToken       ErrorCode = "INVALID_TOKEN"
	ErrCodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrCodePermissionDenied   ErrorCode = "PERMISSION_DENIED"
)

// ErrorSeverity 错误严重程度
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "LOW"
	SeverityMedium   ErrorSeverity = "MEDIUM"
	SeverityHigh     ErrorSeverity = "HIGH"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

// NetCoreError 自定义错误类型
type NetCoreError struct {
	Code        ErrorCode     `json:"code"`
	Message     string        `json:"message"`
	Details     string        `json:"details,omitempty"`
	Severity    ErrorSeverity `json:"severity"`
	Timestamp   time.Time     `json:"timestamp"`
	RequestID   string        `json:"request_id,omitempty"`
	UserID      string        `json:"user_id,omitempty"`
	ServiceName string        `json:"service_name,omitempty"`
	Operation   string        `json:"operation,omitempty"`
	Cause       error         `json:"-"` // 原始错误，不序列化
	StackTrace  string        `json:"stack_trace,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Error 实现error接口
func (e *NetCoreError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s - %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 返回原始错误
func (e *NetCoreError) Unwrap() error {
	return e.Cause
}

// Is 检查错误类型
func (e *NetCoreError) Is(target error) bool {
	if targetErr, ok := target.(*NetCoreError); ok {
		return e.Code == targetErr.Code
	}
	return false
}

// HTTPStatusCode 返回对应的HTTP状态码
func (e *NetCoreError) HTTPStatusCode() int {
	switch e.Code {
	case ErrCodeInvalidRequest, ErrCodeInvalidMessage, ErrCodeMessageTooLarge:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidToken, ErrCodeTokenExpired, ErrCodeInvalidCredentials:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodePermissionDenied:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeServiceNotFound:
		return http.StatusNotFound
	case ErrCodeTimeout, ErrCodeConnectionTimeout:
		return http.StatusRequestTimeout
	case ErrCodeRateLimited:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable, ErrCodeServerNotRunning, ErrCodeNoHealthyBackends:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// NewError 创建新的NetCore错误
func NewError(code ErrorCode, message string) *NetCoreError {
	return &NetCoreError{
		Code:      code,
		Message:   message,
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// NewErrorWithCause 创建带原因的NetCore错误
func NewErrorWithCause(code ErrorCode, message string, cause error) *NetCoreError {
	err := NewError(code, message)
	err.Cause = cause
	if cause != nil {
		err.Details = cause.Error()
	}
	return err
}

// WithDetails 添加错误详情
func (e *NetCoreError) WithDetails(details string) *NetCoreError {
	e.Details = details
	return e
}

// WithSeverity 设置错误严重程度
func (e *NetCoreError) WithSeverity(severity ErrorSeverity) *NetCoreError {
	e.Severity = severity
	return e
}

// WithRequestID 设置请求ID
func (e *NetCoreError) WithRequestID(requestID string) *NetCoreError {
	e.RequestID = requestID
	return e
}

// WithUserID 设置用户ID
func (e *NetCoreError) WithUserID(userID string) *NetCoreError {
	e.UserID = userID
	return e
}

// WithServiceName 设置服务名称
func (e *NetCoreError) WithServiceName(serviceName string) *NetCoreError {
	e.ServiceName = serviceName
	return e
}

// WithOperation 设置操作名称
func (e *NetCoreError) WithOperation(operation string) *NetCoreError {
	e.Operation = operation
	return e
}

// WithMetadata 添加元数据
func (e *NetCoreError) WithMetadata(key string, value interface{}) *NetCoreError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// WithStackTrace 添加堆栈跟踪
func (e *NetCoreError) WithStackTrace(stackTrace string) *NetCoreError {
	e.StackTrace = stackTrace
	return e
}

// 预定义错误实例
var (
	// 通用错误
	ErrInternal        = NewError(ErrCodeInternal, "Internal server error")
	ErrInvalidRequest  = NewError(ErrCodeInvalidRequest, "Invalid request")
	ErrInvalidConfig   = NewError(ErrCodeInvalidConfig, "Invalid configuration")
	ErrTimeout         = NewError(ErrCodeTimeout, "Operation timeout")
	ErrNotFound        = NewError(ErrCodeNotFound, "Resource not found")
	ErrUnauthorized    = NewError(ErrCodeUnauthorized, "Unauthorized access")
	ErrForbidden       = NewError(ErrCodeForbidden, "Access forbidden")
	ErrRateLimited     = NewError(ErrCodeRateLimited, "Rate limit exceeded")
	ErrServiceUnavailable = NewError(ErrCodeServiceUnavailable, "Service unavailable")

	// 连接相关错误
	ErrConnectionFailed   = NewError(ErrCodeConnectionFailed, "Connection failed")
	ErrConnectionClosed   = NewError(ErrCodeConnectionClosed, "Connection closed")
	ErrConnectionTimeout  = NewError(ErrCodeConnectionTimeout, "Connection timeout")
	ErrConnectionRefused  = NewError(ErrCodeConnectionRefused, "Connection refused")
	ErrConnectionReset    = NewError(ErrCodeConnectionReset, "Connection reset")
	ErrTLSHandshakeFailed = NewError(ErrCodeTLSHandshakeFailed, "TLS handshake failed")

	// 服务器相关错误
	ErrServerNotRunning  = NewError(ErrCodeServerNotRunning, "Server is not running")
	ErrServerRunning     = NewError(ErrCodeServerRunning, "Server is already running")
	ErrServerStartFailed = NewError(ErrCodeServerStartFailed, "Failed to start server")
	ErrServerStopFailed  = NewError(ErrCodeServerStopFailed, "Failed to stop server")
	ErrPortInUse         = NewError(ErrCodePortInUse, "Port is already in use")

	// 协议相关错误
	ErrProtocolError      = NewError(ErrCodeProtocolError, "Protocol error")
	ErrInvalidMessage     = NewError(ErrCodeInvalidMessage, "Invalid message format")
	ErrMessageTooLarge    = NewError(ErrCodeMessageTooLarge, "Message too large")
	ErrUnsupportedProtocol = NewError(ErrCodeUnsupportedProtocol, "Unsupported protocol")

	// 服务发现相关错误
	ErrServiceNotFound    = NewError(ErrCodeServiceNotFound, "Service not found")
	ErrDiscoveryFailed    = NewError(ErrCodeDiscoveryFailed, "Service discovery failed")
	ErrRegistrationFailed = NewError(ErrCodeRegistrationFailed, "Service registration failed")

	// 负载均衡相关错误
	ErrNoHealthyBackends  = NewError(ErrCodeNoHealthyBackends, "No healthy backends available")
	ErrLoadBalancerFailed = NewError(ErrCodeLoadBalancerFailed, "Load balancer operation failed")

	// 数据库相关错误
	ErrDatabaseConnection = NewError(ErrCodeDatabaseConnection, "Database connection error")
	ErrDatabaseQuery      = NewError(ErrCodeDatabaseQuery, "Database query error")
	ErrDatabaseTransaction = NewError(ErrCodeDatabaseTransaction, "Database transaction error")

	// 缓存相关错误
	ErrCacheConnection = NewError(ErrCodeCacheConnection, "Cache connection error")
	ErrCacheOperation  = NewError(ErrCodeCacheOperation, "Cache operation error")

	// 认证授权相关错误
	ErrInvalidToken       = NewError(ErrCodeInvalidToken, "Invalid token")
	ErrTokenExpired       = NewError(ErrCodeTokenExpired, "Token expired")
	ErrInvalidCredentials = NewError(ErrCodeInvalidCredentials, "Invalid credentials")
	ErrPermissionDenied   = NewError(ErrCodePermissionDenied, "Permission denied")
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	HandleError(err error) *NetCoreError
	ShouldRetry(err error) bool
	GetRetryDelay(err error, attempt int) time.Duration
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	maxRetries   int
	baseDelay    time.Duration
	maxDelay     time.Duration
	retryableErrors map[ErrorCode]bool
}

// NewDefaultErrorHandler 创建默认错误处理器
func NewDefaultErrorHandler() *DefaultErrorHandler {
	return &DefaultErrorHandler{
		maxRetries:  3,
		baseDelay:   time.Second,
		maxDelay:    time.Minute,
		retryableErrors: map[ErrorCode]bool{
			ErrCodeTimeout:         true,
			ErrCodeConnectionTimeout: true,
			ErrCodeServiceUnavailable: true,
			ErrCodeConnectionFailed: true,
			ErrCodeConnectionReset:  true,
		},
	}
}

// HandleError 处理错误
func (h *DefaultErrorHandler) HandleError(err error) *NetCoreError {
	if netErr, ok := err.(*NetCoreError); ok {
		return netErr
	}
	
	// 转换标准错误为NetCore错误
	return NewErrorWithCause(ErrCodeInternal, "Unexpected error", err)
}

// ShouldRetry 判断是否应该重试
func (h *DefaultErrorHandler) ShouldRetry(err error) bool {
	if netErr, ok := err.(*NetCoreError); ok {
		return h.retryableErrors[netErr.Code]
	}
	return false
}

// GetRetryDelay 获取重试延迟
func (h *DefaultErrorHandler) GetRetryDelay(err error, attempt int) time.Duration {
	if attempt >= h.maxRetries {
		return 0
	}
	
	// 指数退避算法
	delay := h.baseDelay * time.Duration(1<<uint(attempt))
	if delay > h.maxDelay {
		delay = h.maxDelay
	}
	return delay
}

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	if netErr, ok := err.(*NetCoreError); ok {
		retryableCodes := []ErrorCode{
			ErrCodeTimeout,
			ErrCodeConnectionTimeout,
			ErrCodeServiceUnavailable,
			ErrCodeConnectionFailed,
			ErrCodeConnectionReset,
			ErrCodeServerNotRunning,
		}
		
		for _, code := range retryableCodes {
			if netErr.Code == code {
				return true
			}
		}
	}
	return false
}

// IsTemporaryError 检查错误是否是临时性的
func IsTemporaryError(err error) bool {
	if netErr, ok := err.(*NetCoreError); ok {
		temporaryCodes := []ErrorCode{
			ErrCodeTimeout,
			ErrCodeConnectionTimeout,
			ErrCodeServiceUnavailable,
			ErrCodeRateLimited,
			ErrCodeNoHealthyBackends,
		}
		
		for _, code := range temporaryCodes {
			if netErr.Code == code {
				return true
			}
		}
	}
	return false
}

// IsCriticalError 检查错误是否是严重错误
func IsCriticalError(err error) bool {
	if netErr, ok := err.(*NetCoreError); ok {
		return netErr.Severity == SeverityCritical
	}
	return false
}