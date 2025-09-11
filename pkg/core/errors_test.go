// Package core 错误处理系统单元测试
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestErrorCode 测试错误代码
func TestErrorCode(t *testing.T) {
	tests := []struct {
		name string
		code ErrorCode
		want string
	}{
		{"Internal Error", ErrCodeInternal, "INTERNAL_ERROR"},
		{"Invalid Request", ErrCodeInvalidRequest, "INVALID_REQUEST"},
		{"Not Found", ErrCodeNotFound, "NOT_FOUND"},
		{"Timeout", ErrCodeTimeout, "TIMEOUT"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.code) != tt.want {
				t.Errorf("ErrorCode = %v, want %v", tt.code, tt.want)
			}
		})
	}
}

// TestNewError 测试创建新错误
func TestNewError(t *testing.T) {
	err := NewError(ErrCodeInvalidRequest, "Test error message")
	
	if err.Code != ErrCodeInvalidRequest {
		t.Errorf("Expected code %v, got %v", ErrCodeInvalidRequest, err.Code)
	}
	
	if err.Message != "Test error message" {
		t.Errorf("Expected message 'Test error message', got %v", err.Message)
	}
	
	if err.Severity != SeverityMedium {
		t.Errorf("Expected severity %v, got %v", SeverityMedium, err.Severity)
	}
	
	if err.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}
	
	// 检查时间戳是否在合理范围内
	if time.Since(err.Timestamp) > time.Second {
		t.Error("Timestamp should be recent")
	}
}

// TestNewErrorWithCause 测试创建带原因的错误
func TestNewErrorWithCause(t *testing.T) {
	cause := errors.New("original error")
	err := NewErrorWithCause(ErrCodeInternal, "Wrapper error", cause)
	
	if err.Code != ErrCodeInternal {
		t.Errorf("Expected code %v, got %v", ErrCodeInternal, err.Code)
	}
	
	if err.Message != "Wrapper error" {
		t.Errorf("Expected message 'Wrapper error', got %v", err.Message)
	}
	
	if err.Cause != cause {
		t.Errorf("Expected cause to be %v, got %v", cause, err.Cause)
	}
	
	if err.Details != cause.Error() {
		t.Errorf("Expected details to be %v, got %v", cause.Error(), err.Details)
	}
}

// TestNetCoreError_Error 测试错误字符串表示
func TestNetCoreError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetCoreError
		expected string
	}{
		{
			"Error without details",
			&NetCoreError{Code: ErrCodeNotFound, Message: "Resource not found"},
			"NOT_FOUND: Resource not found",
		},
		{
			"Error with details",
			&NetCoreError{Code: ErrCodeInvalidRequest, Message: "Bad request", Details: "Missing required field"},
			"INVALID_REQUEST: Bad request - Missing required field",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("NetCoreError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNetCoreError_Unwrap 测试错误解包
func TestNetCoreError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := NewErrorWithCause(ErrCodeInternal, "Wrapper", cause)
	
	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Expected unwrapped error to be %v, got %v", cause, unwrapped)
	}
	
	// 测试没有原因的错误
	errNoCause := NewError(ErrCodeNotFound, "No cause")
	if errNoCause.Unwrap() != nil {
		t.Error("Expected unwrapped error to be nil for error without cause")
	}
}

// TestNetCoreError_Is 测试错误类型检查
func TestNetCoreError_Is(t *testing.T) {
	err1 := NewError(ErrCodeNotFound, "Error 1")
	err2 := NewError(ErrCodeNotFound, "Error 2")
	err3 := NewError(ErrCodeInternal, "Error 3")
	
	if !err1.Is(err2) {
		t.Error("Expected errors with same code to be equal")
	}
	
	if err1.Is(err3) {
		t.Error("Expected errors with different codes to not be equal")
	}
	
	// 测试与非NetCoreError的比较
	stdErr := errors.New("standard error")
	if err1.Is(stdErr) {
		t.Error("Expected NetCoreError to not equal standard error")
	}
}

// TestNetCoreError_HTTPStatusCode 测试HTTP状态码映射
func TestNetCoreError_HTTPStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		{"Bad Request", ErrCodeInvalidRequest, http.StatusBadRequest},
		{"Unauthorized", ErrCodeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", ErrCodeForbidden, http.StatusForbidden},
		{"Not Found", ErrCodeNotFound, http.StatusNotFound},
		{"Timeout", ErrCodeTimeout, http.StatusRequestTimeout},
		{"Rate Limited", ErrCodeRateLimited, http.StatusTooManyRequests},
		{"Service Unavailable", ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
		{"Internal Error", ErrCodeInternal, http.StatusInternalServerError},
		{"Unknown Code", ErrorCode("UNKNOWN"), http.StatusInternalServerError},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.code, "Test error")
			if got := err.HTTPStatusCode(); got != tt.expected {
				t.Errorf("HTTPStatusCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNetCoreError_WithMethods 测试链式方法
func TestNetCoreError_WithMethods(t *testing.T) {
	err := NewError(ErrCodeInvalidRequest, "Base error")
	
	// 测试WithDetails
	err = err.WithDetails("Additional details")
	if err.Details != "Additional details" {
		t.Errorf("Expected details 'Additional details', got %v", err.Details)
	}
	
	// 测试WithSeverity
	err = err.WithSeverity(SeverityHigh)
	if err.Severity != SeverityHigh {
		t.Errorf("Expected severity %v, got %v", SeverityHigh, err.Severity)
	}
	
	// 测试WithRequestID
	err = err.WithRequestID("req-123")
	if err.RequestID != "req-123" {
		t.Errorf("Expected request ID 'req-123', got %v", err.RequestID)
	}
	
	// 测试WithUserID
	err = err.WithUserID("user-456")
	if err.UserID != "user-456" {
		t.Errorf("Expected user ID 'user-456', got %v", err.UserID)
	}
	
	// 测试WithServiceName
	err = err.WithServiceName("test-service")
	if err.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got %v", err.ServiceName)
	}
	
	// 测试WithOperation
	err = err.WithOperation("test-operation")
	if err.Operation != "test-operation" {
		t.Errorf("Expected operation 'test-operation', got %v", err.Operation)
	}
	
	// 测试WithMetadata
	err = err.WithMetadata("key1", "value1")
	if err.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata key1 to be 'value1', got %v", err.Metadata["key1"])
	}
	
	// 测试WithStackTrace
	err = err.WithStackTrace("stack trace")
	if err.StackTrace != "stack trace" {
		t.Errorf("Expected stack trace 'stack trace', got %v", err.StackTrace)
	}
}

// TestDefaultErrorHandler 测试默认错误处理器
func TestDefaultErrorHandler(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	if handler == nil {
		t.Fatal("Expected handler to be created")
	}
	
	// 测试HandleError
	stdErr := errors.New("standard error")
	netErr := handler.HandleError(stdErr)
	
	if netErr.Code != ErrCodeInternal {
		t.Errorf("Expected code %v, got %v", ErrCodeInternal, netErr.Code)
	}
	
	if netErr.Cause != stdErr {
		t.Errorf("Expected cause to be %v, got %v", stdErr, netErr.Cause)
	}
	
	// 测试已经是NetCoreError的情况
	existingErr := NewError(ErrCodeNotFound, "Not found")
	handledErr := handler.HandleError(existingErr)
	
	if handledErr != existingErr {
		t.Error("Expected existing NetCoreError to be returned as-is")
	}
}

// TestDefaultErrorHandler_ShouldRetry 测试重试逻辑
func TestDefaultErrorHandler_ShouldRetry(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Timeout error", NewError(ErrCodeTimeout, "Timeout"), true},
		{"Connection timeout", NewError(ErrCodeConnectionTimeout, "Connection timeout"), true},
		{"Service unavailable", NewError(ErrCodeServiceUnavailable, "Service unavailable"), true},
		{"Connection failed", NewError(ErrCodeConnectionFailed, "Connection failed"), true},
		{"Connection reset", NewError(ErrCodeConnectionReset, "Connection reset"), true},
		{"Not found error", NewError(ErrCodeNotFound, "Not found"), false},
		{"Invalid request", NewError(ErrCodeInvalidRequest, "Invalid request"), false},
		{"Standard error", errors.New("standard error"), false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handler.ShouldRetry(tt.err); got != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDefaultErrorHandler_GetRetryDelay 测试重试延迟
func TestDefaultErrorHandler_GetRetryDelay(t *testing.T) {
	handler := NewDefaultErrorHandler()
	err := NewError(ErrCodeTimeout, "Timeout")
	
	// 测试指数退避
	delay1 := handler.GetRetryDelay(err, 0)
	delay2 := handler.GetRetryDelay(err, 1)
	delay3 := handler.GetRetryDelay(err, 2)
	
	if delay1 != time.Second {
		t.Errorf("Expected first delay to be 1s, got %v", delay1)
	}
	
	if delay2 != 2*time.Second {
		t.Errorf("Expected second delay to be 2s, got %v", delay2)
	}
	
	if delay3 != 4*time.Second {
		t.Errorf("Expected third delay to be 4s, got %v", delay3)
	}
	
	// 测试超过最大重试次数
	delayMax := handler.GetRetryDelay(err, 10)
	if delayMax != 0 {
		t.Errorf("Expected delay to be 0 when exceeding max retries, got %v", delayMax)
	}
}

// TestIsRetryableError 测试可重试错误检查
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Timeout error", NewError(ErrCodeTimeout, "Timeout"), true},
		{"Connection timeout", NewError(ErrCodeConnectionTimeout, "Connection timeout"), true},
		{"Service unavailable", NewError(ErrCodeServiceUnavailable, "Service unavailable"), true},
		{"Connection failed", NewError(ErrCodeConnectionFailed, "Connection failed"), true},
		{"Connection reset", NewError(ErrCodeConnectionReset, "Connection reset"), true},
		{"Server not running", NewError(ErrCodeServerNotRunning, "Server not running"), true},
		{"Not found error", NewError(ErrCodeNotFound, "Not found"), false},
		{"Invalid request", NewError(ErrCodeInvalidRequest, "Invalid request"), false},
		{"Standard error", errors.New("standard error"), false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.expected {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsTemporaryError 测试临时错误检查
func TestIsTemporaryError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Timeout error", NewError(ErrCodeTimeout, "Timeout"), true},
		{"Connection timeout", NewError(ErrCodeConnectionTimeout, "Connection timeout"), true},
		{"Service unavailable", NewError(ErrCodeServiceUnavailable, "Service unavailable"), true},
		{"Rate limited", NewError(ErrCodeRateLimited, "Rate limited"), true},
		{"No healthy backends", NewError(ErrCodeNoHealthyBackends, "No healthy backends"), true},
		{"Not found error", NewError(ErrCodeNotFound, "Not found"), false},
		{"Invalid request", NewError(ErrCodeInvalidRequest, "Invalid request"), false},
		{"Standard error", errors.New("standard error"), false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTemporaryError(tt.err); got != tt.expected {
				t.Errorf("IsTemporaryError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsCriticalError 测试严重错误检查
func TestIsCriticalError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Critical error", NewError(ErrCodeInternal, "Critical").WithSeverity(SeverityCritical), true},
		{"High severity error", NewError(ErrCodeInternal, "High").WithSeverity(SeverityHigh), false},
		{"Medium severity error", NewError(ErrCodeInternal, "Medium").WithSeverity(SeverityMedium), false},
		{"Standard error", errors.New("standard error"), false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCriticalError(tt.err); got != tt.expected {
				t.Errorf("IsCriticalError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// BenchmarkNewError 性能测试：创建错误
func BenchmarkNewError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewError(ErrCodeInvalidRequest, "Test error")
	}
}

// BenchmarkNewErrorWithCause 性能测试：创建带原因的错误
func BenchmarkNewErrorWithCause(b *testing.B) {
	cause := errors.New("original error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewErrorWithCause(ErrCodeInternal, "Wrapper error", cause)
	}
}

// BenchmarkErrorError 性能测试：错误字符串转换
func BenchmarkErrorError(b *testing.B) {
	err := NewError(ErrCodeInvalidRequest, "Test error").WithDetails("Additional details")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

// BenchmarkErrorHTTPStatusCode 性能测试：HTTP状态码映射
func BenchmarkErrorHTTPStatusCode(b *testing.B) {
	err := NewError(ErrCodeInvalidRequest, "Test error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.HTTPStatusCode()
	}
}