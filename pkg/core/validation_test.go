// Package core 提供NetCore-Go的核心验证功能测试
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestValidationRule 测试验证规则接口
func TestValidationRule(t *testing.T) {
	t.Run("RequiredRule", func(t *testing.T) {
		rule := NewRequiredRule()
		
		// 测试有效值
		if !rule.Validate("test") {
			t.Error("Expected non-empty string to be valid")
		}
		
		if !rule.Validate(123) {
			t.Error("Expected non-zero number to be valid")
		}
		
		if !rule.Validate(true) {
			t.Error("Expected boolean true to be valid")
		}
		
		// 测试无效值
		if rule.Validate("") {
			t.Error("Expected empty string to be invalid")
		}
		
		if rule.Validate(0) {
			t.Error("Expected zero to be invalid")
		}
		
		if rule.Validate(nil) {
			t.Error("Expected nil to be invalid")
		}
		
		// 测试错误消息
		expectedMsg := "field is required"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("RangeRule", func(t *testing.T) {
		rule := NewRangeRule(10, 100)
		
		// 测试有效值
		if !rule.Validate(50) {
			t.Error("Expected value in range to be valid")
		}
		
		if !rule.Validate(10) {
			t.Error("Expected minimum value to be valid")
		}
		
		if !rule.Validate(100) {
			t.Error("Expected maximum value to be valid")
		}
		
		if !rule.Validate(50.5) {
			t.Error("Expected float in range to be valid")
		}
		
		// 测试无效值
		if rule.Validate(5) {
			t.Error("Expected value below range to be invalid")
		}
		
		if rule.Validate(150) {
			t.Error("Expected value above range to be invalid")
		}
		
		if rule.Validate("not a number") {
			t.Error("Expected non-numeric value to be invalid")
		}
		
		// 测试错误消息
		expectedMsg := "value must be between 10 and 100"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("LengthRule", func(t *testing.T) {
		rule := NewLengthRule(5, 20)
		
		// 测试有效值
		if !rule.Validate("hello") {
			t.Error("Expected string of valid length to be valid")
		}
		
		if !rule.Validate("hello world") {
			t.Error("Expected string within range to be valid")
		}
		
		if !rule.Validate([]int{1, 2, 3, 4, 5}) {
			t.Error("Expected slice of valid length to be valid")
		}
		
		// 测试无效值
		if rule.Validate("hi") {
			t.Error("Expected short string to be invalid")
		}
		
		if rule.Validate("this is a very long string that exceeds the maximum length") {
			t.Error("Expected long string to be invalid")
		}
		
		if rule.Validate(123) {
			t.Error("Expected non-string/slice value to be invalid")
		}
		
		// 测试错误消息
		expectedMsg := "length must be between 5 and 20"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("RegexRule", func(t *testing.T) {
		// 邮箱验证规则
		emailPattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		rule := NewRegexRule(emailPattern)
		
		// 测试有效邮箱
		validEmails := []string{
			"test@example.com",
			"user.name@domain.co.uk",
			"user+tag@example.org",
		}
		
		for _, email := range validEmails {
			if !rule.Validate(email) {
				t.Errorf("Expected valid email '%s' to pass validation", email)
			}
		}
		
		// 测试无效邮箱
		invalidEmails := []string{
			"invalid-email",
			"@example.com",
			"user@",
			"user@.com",
			"user name@example.com",
		}
		
		for _, email := range invalidEmails {
			if rule.Validate(email) {
				t.Errorf("Expected invalid email '%s' to fail validation", email)
			}
		}
		
		// 测试非字符串值
		if rule.Validate(123) {
			t.Error("Expected non-string value to be invalid")
		}
		
		// 测试错误消息
		expectedMsg := "value does not match required pattern"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("URLRule", func(t *testing.T) {
		rule := NewURLRule()
		
		// 测试有效URL
		validURLs := []string{
			"http://example.com",
			"https://www.example.com",
			"https://example.com/path?query=value",
			"ftp://files.example.com",
		}
		
		for _, url := range validURLs {
			if !rule.Validate(url) {
				t.Errorf("Expected valid URL '%s' to pass validation", url)
			}
		}
		
		// 测试无效URL
		invalidURLs := []string{
			"not-a-url",
			"http://",
			"://example.com",
			"example.com", // 缺少协议
		}
		
		for _, url := range invalidURLs {
			if rule.Validate(url) {
				t.Errorf("Expected invalid URL '%s' to fail validation", url)
			}
		}
		
		// 测试错误消息
		expectedMsg := "value must be a valid URL"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("IPRule", func(t *testing.T) {
		rule := NewIPRule()
		
		// 测试有效IP地址
		validIPs := []string{
			"192.168.1.1",
			"10.0.0.1",
			"127.0.0.1",
			"::1",
			"2001:db8::1",
			"fe80::1",
		}
		
		for _, ip := range validIPs {
			if !rule.Validate(ip) {
				t.Errorf("Expected valid IP '%s' to pass validation", ip)
			}
		}
		
		// 测试无效IP地址
		invalidIPs := []string{
			"256.256.256.256",
			"192.168.1",
			"not-an-ip",
			"192.168.1.1.1",
		}
		
		for _, ip := range invalidIPs {
			if rule.Validate(ip) {
				t.Errorf("Expected invalid IP '%s' to fail validation", ip)
			}
		}
		
		// 测试错误消息
		expectedMsg := "value must be a valid IP address"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("PortRule", func(t *testing.T) {
		rule := NewPortRule()
		
		// 测试有效端口
		validPorts := []interface{}{
			80,
			443,
			8080,
			65535,
			"80",
			"443",
		}
		
		for _, port := range validPorts {
			if !rule.Validate(port) {
				t.Errorf("Expected valid port '%v' to pass validation", port)
			}
		}
		
		// 测试无效端口
		invalidPorts := []interface{}{
			0,
			-1,
			65536,
			100000,
			"not-a-port",
			"80.5",
		}
		
		for _, port := range invalidPorts {
			if rule.Validate(port) {
				t.Errorf("Expected invalid port '%v' to fail validation", port)
			}
		}
		
		// 测试错误消息
		expectedMsg := "value must be a valid port number (1-65535)"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("DurationRule", func(t *testing.T) {
		rule := NewDurationRule(time.Second, time.Hour)
		
		// 测试有效持续时间
		validDurations := []interface{}{
			time.Minute,
			time.Second * 30,
			"5m",
			"30s",
			"1h",
		}
		
		for _, duration := range validDurations {
			if !rule.Validate(duration) {
				t.Errorf("Expected valid duration '%v' to pass validation", duration)
			}
		}
		
		// 测试无效持续时间
		invalidDurations := []interface{}{
			time.Millisecond * 500, // 太短
			time.Hour * 2,          // 太长
			"500ms",                // 太短
			"2h",                   // 太长
			"invalid",              // 无效格式
			123,                    // 不是持续时间
		}
		
		for _, duration := range invalidDurations {
			if rule.Validate(duration) {
				t.Errorf("Expected invalid duration '%v' to fail validation", duration)
			}
		}
		
		// 测试错误消息
		expectedMsg := "duration must be between 1s and 1h0m0s"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
	
	t.Run("EnumRule", func(t *testing.T) {
		allowedValues := []interface{}{"red", "green", "blue", 1, 2, 3}
		rule := NewEnumRule(allowedValues)
		
		// 测试有效值
		for _, value := range allowedValues {
			if !rule.Validate(value) {
				t.Errorf("Expected allowed value '%v' to pass validation", value)
			}
		}
		
		// 测试无效值
		invalidValues := []interface{}{"yellow", "purple", 4, 5, "1", "2"}
		
		for _, value := range invalidValues {
			if rule.Validate(value) {
				t.Errorf("Expected disallowed value '%v' to fail validation", value)
			}
		}
		
		// 测试错误消息
		expectedMsg := "value must be one of: red, green, blue, 1, 2, 3"
		if rule.ErrorMessage() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, rule.ErrorMessage())
		}
	})
}

// TestValidationError 测试验证错误
func TestValidationError(t *testing.T) {
	t.Run("NewValidationError", func(t *testing.T) {
		field := "username"
		message := "is required"
		
		err := NewValidationError(field, message)
		
		if err.Field != field {
			t.Errorf("Expected field '%s', got '%s'", field, err.Field)
		}
		
		if err.Message != message {
			t.Errorf("Expected message '%s', got '%s'", message, err.Message)
		}
		
		expectedError := "username: is required"
		if err.Error() != expectedError {
			t.Errorf("Expected error string '%s', got '%s'", expectedError, err.Error())
		}
	})
	
	t.Run("ValidationErrorWithValue", func(t *testing.T) {
		field := "age"
		message := "must be between 18 and 65"
		value := 15
		
		err := NewValidationError(field, message)
		err.Value = value
		
		if err.Value != value {
			t.Errorf("Expected value %v, got %v", value, err.Value)
		}
	})
}

// TestValidationResult 测试验证结果
func TestValidationResult(t *testing.T) {
	t.Run("NewValidationResult", func(t *testing.T) {
		result := NewValidationResult()
		
		if !result.IsValid() {
			t.Error("Expected new validation result to be valid")
		}
		
		if len(result.Errors) != 0 {
			t.Errorf("Expected no errors, got %d", len(result.Errors))
		}
	})
	
	t.Run("AddError", func(t *testing.T) {
		result := NewValidationResult()
		err := NewValidationError("field1", "error message")
		
		result.AddError(err)
		
		if result.IsValid() {
			t.Error("Expected validation result to be invalid after adding error")
		}
		
		if len(result.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(result.Errors))
		}
		
		if result.Errors[0] != err {
			t.Error("Expected added error to match")
		}
	})
	
	t.Run("AddFieldError", func(t *testing.T) {
		result := NewValidationResult()
		field := "username"
		message := "is required"
		
		result.AddFieldError(field, message)
		
		if result.IsValid() {
			t.Error("Expected validation result to be invalid after adding field error")
		}
		
		if len(result.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(result.Errors))
		}
		
		if result.Errors[0].Field != field {
			t.Errorf("Expected field '%s', got '%s'", field, result.Errors[0].Field)
		}
		
		if result.Errors[0].Message != message {
			t.Errorf("Expected message '%s', got '%s'", message, result.Errors[0].Message)
		}
	})
	
	t.Run("GetFieldErrors", func(t *testing.T) {
		result := NewValidationResult()
		field := "email"
		
		result.AddFieldError(field, "is required")
		result.AddFieldError(field, "must be valid email")
		result.AddFieldError("other", "other error")
		
		fieldErrors := result.GetFieldErrors(field)
		
		if len(fieldErrors) != 2 {
			t.Errorf("Expected 2 field errors, got %d", len(fieldErrors))
		}
		
		for _, err := range fieldErrors {
			if err.Field != field {
				t.Errorf("Expected field '%s', got '%s'", field, err.Field)
			}
		}
	})
	
	t.Run("HasFieldError", func(t *testing.T) {
		result := NewValidationResult()
		field := "password"
		
		if result.HasFieldError(field) {
			t.Error("Expected no field error initially")
		}
		
		result.AddFieldError(field, "is too short")
		
		if !result.HasFieldError(field) {
			t.Error("Expected field error after adding")
		}
		
		if result.HasFieldError("other") {
			t.Error("Expected no error for different field")
		}
	})
	
	t.Run("ErrorMessages", func(t *testing.T) {
		result := NewValidationResult()
		
		result.AddFieldError("field1", "error1")
		result.AddFieldError("field2", "error2")
		result.AddFieldError("field1", "error3")
		
		messages := result.ErrorMessages()
		
		if len(messages) != 3 {
			t.Errorf("Expected 3 error messages, got %d", len(messages))
		}
		
		expectedMessages := []string{
			"field1: error1",
			"field2: error2",
			"field1: error3",
		}
		
		for i, expected := range expectedMessages {
			if messages[i] != expected {
				t.