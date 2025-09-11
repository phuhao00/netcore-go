// Package core 定义NetCore-Go网络库的配置验证系统
// Author: NetCore-Go Team
// Created: 2024

package core

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(value interface{}) error
	Name() string
	Description() string
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []*ValidationError `json:"errors,omitempty"`
}

// AddError 添加验证错误
func (r *ValidationResult) AddError(field, rule, message string, value interface{}) {
	r.Valid = false
	r.Errors = append(r.Errors, &ValidationError{
		Field:   field,
		Rule:    rule,
		Message: message,
		Value:   value,
	})
}

// HasErrors 检查是否有错误
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetErrorMessages 获取所有错误消息
func (r *ValidationResult) GetErrorMessages() []string {
	messages := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// Validator 配置验证器
type Validator struct {
	rules map[string][]ValidationRule
}

// NewValidator 创建新的验证器
func NewValidator() *Validator {
	return &Validator{
		rules: make(map[string][]ValidationRule),
	}
}

// AddRule 添加验证规则
func (v *Validator) AddRule(field string, rule ValidationRule) {
	v.rules[field] = append(v.rules[field], rule)
}

// Validate 验证配置
func (v *Validator) Validate(config interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true}
	
	val := reflect.ValueOf(config)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		result.AddError("", "type", "configuration must be a struct", config)
		return result
	}
	
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)
		
		// 跳过未导出的字段
		if !fieldValue.CanInterface() {
			continue
		}
		
		fieldName := field.Name
		
		// 检查字段的验证规则
		if rules, exists := v.rules[fieldName]; exists {
			for _, rule := range rules {
				if err := rule.Validate(fieldValue.Interface()); err != nil {
					result.AddError(fieldName, rule.Name(), err.Error(), fieldValue.Interface())
				}
			}
		}
		
		// 递归验证嵌套结构体
		if fieldValue.Kind() == reflect.Struct {
			nestedResult := v.Validate(fieldValue.Interface())
			for _, nestedErr := range nestedResult.Errors {
				nestedErr.Field = fieldName + "." + nestedErr.Field
				result.Errors = append(result.Errors, nestedErr)
				result.Valid = false
			}
		}
	}
	
	return result
}

// 内置验证规则

// RequiredRule 必填验证规则
type RequiredRule struct{}

func (r *RequiredRule) Name() string {
	return "required"
}

func (r *RequiredRule) Description() string {
	return "Field is required and cannot be empty"
}

func (r *RequiredRule) Validate(value interface{}) error {
	if value == nil {
		return fmt.Errorf("field is required")
	}
	
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String:
		if val.String() == "" {
			return fmt.Errorf("field cannot be empty")
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if val.Len() == 0 {
			return fmt.Errorf("field cannot be empty")
		}
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return fmt.Errorf("field is required")
		}
	}
	
	return nil
}

// RangeRule 范围验证规则
type RangeRule struct {
	Min interface{}
	Max interface{}
}

func NewRangeRule(min, max interface{}) *RangeRule {
	return &RangeRule{Min: min, Max: max}
}

func (r *RangeRule) Name() string {
	return "range"
}

func (r *RangeRule) Description() string {
	return fmt.Sprintf("Value must be between %v and %v", r.Min, r.Max)
}

func (r *RangeRule) Validate(value interface{}) error {
	val := reflect.ValueOf(value)
	
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := val.Int()
		min := reflect.ValueOf(r.Min).Int()
		max := reflect.ValueOf(r.Max).Int()
		if v < min || v > max {
			return fmt.Errorf("value %d is not in range [%d, %d]", v, min, max)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := val.Uint()
		min := reflect.ValueOf(r.Min).Uint()
		max := reflect.ValueOf(r.Max).Uint()
		if v < min || v > max {
			return fmt.Errorf("value %d is not in range [%d, %d]", v, min, max)
		}
	case reflect.Float32, reflect.Float64:
		v := val.Float()
		min := reflect.ValueOf(r.Min).Float()
		max := reflect.ValueOf(r.Max).Float()
		if v < min || v > max {
			return fmt.Errorf("value %f is not in range [%f, %f]", v, min, max)
		}
	default:
		return fmt.Errorf("range validation not supported for type %T", value)
	}
	
	return nil
}

// MinLengthRule 最小长度验证规则
type MinLengthRule struct {
	MinLength int
}

func NewMinLengthRule(minLength int) *MinLengthRule {
	return &MinLengthRule{MinLength: minLength}
}

func (r *MinLengthRule) Name() string {
	return "min_length"
}

func (r *MinLengthRule) Description() string {
	return fmt.Sprintf("Minimum length is %d", r.MinLength)
}

func (r *MinLengthRule) Validate(value interface{}) error {
	val := reflect.ValueOf(value)
	
	switch val.Kind() {
	case reflect.String:
		if len(val.String()) < r.MinLength {
			return fmt.Errorf("string length %d is less than minimum %d", len(val.String()), r.MinLength)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if val.Len() < r.MinLength {
			return fmt.Errorf("length %d is less than minimum %d", val.Len(), r.MinLength)
		}
	default:
		return fmt.Errorf("min_length validation not supported for type %T", value)
	}
	
	return nil
}

// MaxLengthRule 最大长度验证规则
type MaxLengthRule struct {
	MaxLength int
}

func NewMaxLengthRule(maxLength int) *MaxLengthRule {
	return &MaxLengthRule{MaxLength: maxLength}
}

func (r *MaxLengthRule) Name() string {
	return "max_length"
}

func (r *MaxLengthRule) Description() string {
	return fmt.Sprintf("Maximum length is %d", r.MaxLength)
}

func (r *MaxLengthRule) Validate(value interface{}) error {
	val := reflect.ValueOf(value)
	
	switch val.Kind() {
	case reflect.String:
		if len(val.String()) > r.MaxLength {
			return fmt.Errorf("string length %d exceeds maximum %d", len(val.String()), r.MaxLength)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if val.Len() > r.MaxLength {
			return fmt.Errorf("length %d exceeds maximum %d", val.Len(), r.MaxLength)
		}
	default:
		return fmt.Errorf("max_length validation not supported for type %T", value)
	}
	
	return nil
}

// RegexRule 正则表达式验证规则
type RegexRule struct {
	Pattern *regexp.Regexp
	Message string
}

func NewRegexRule(pattern, message string) (*RegexRule, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}
	
	return &RegexRule{
		Pattern: regex,
		Message: message,
	}, nil
}

func (r *RegexRule) Name() string {
	return "regex"
}

func (r *RegexRule) Description() string {
	return r.Message
}

func (r *RegexRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("regex validation only supports string values")
	}
	
	if !r.Pattern.MatchString(str) {
		return fmt.Errorf(r.Message)
	}
	
	return nil
}

// URLRule URL验证规则
type URLRule struct{}

func (r *URLRule) Name() string {
	return "url"
}

func (r *URLRule) Description() string {
	return "Value must be a valid URL"
}

func (r *URLRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("URL validation only supports string values")
	}
	
	if str == "" {
		return nil // 空值由Required规则处理
	}
	
	_, err := url.Parse(str)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}
	
	return nil
}

// IPRule IP地址验证规则
type IPRule struct{}

func (r *IPRule) Name() string {
	return "ip"
}

func (r *IPRule) Description() string {
	return "Value must be a valid IP address"
}

func (r *IPRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("IP validation only supports string values")
	}
	
	if str == "" {
		return nil // 空值由Required规则处理
	}
	
	if net.ParseIP(str) == nil {
		return fmt.Errorf("invalid IP address format")
	}
	
	return nil
}

// PortRule 端口验证规则
type PortRule struct{}

func (r *PortRule) Name() string {
	return "port"
}

func (r *PortRule) Description() string {
	return "Value must be a valid port number (1-65535)"
}

func (r *PortRule) Validate(value interface{}) error {
	var port int
	
	switch v := value.(type) {
	case int:
		port = v
	case int32:
		port = int(v)
	case int64:
		port = int(v)
	case string:
		if v == "" {
			return nil // 空值由Required规则处理
		}
		p, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid port number format")
		}
		port = p
	default:
		return fmt.Errorf("port validation not supported for type %T", value)
	}
	
	if port < 1 || port > 65535 {
		return fmt.Errorf("port number %d is not in valid range [1, 65535]", port)
	}
	
	return nil
}

// DurationRule 时间间隔验证规则
type DurationRule struct {
	Min time.Duration
	Max time.Duration
}

func NewDurationRule(min, max time.Duration) *DurationRule {
	return &DurationRule{Min: min, Max: max}
}

func (r *DurationRule) Name() string {
	return "duration"
}

func (r *DurationRule) Description() string {
	return fmt.Sprintf("Duration must be between %v and %v", r.Min, r.Max)
}

func (r *DurationRule) Validate(value interface{}) error {
	var duration time.Duration
	
	switch v := value.(type) {
	case time.Duration:
		duration = v
	case string:
		if v == "" {
			return nil // 空值由Required规则处理
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid duration format: %v", err)
		}
		duration = d
	default:
		return fmt.Errorf("duration validation not supported for type %T", value)
	}
	
	if duration < r.Min || duration > r.Max {
		return fmt.Errorf("duration %v is not in range [%v, %v]", duration, r.Min, r.Max)
	}
	
	return nil
}

// OneOfRule 枚举值验证规则
type OneOfRule struct {
	Values []interface{}
}

func NewOneOfRule(values ...interface{}) *OneOfRule {
	return &OneOfRule{Values: values}
}

func (r *OneOfRule) Name() string {
	return "one_of"
}

func (r *OneOfRule) Description() string {
	valueStrs := make([]string, len(r.Values))
	for i, v := range r.Values {
		valueStrs[i] = fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("Value must be one of: %s", strings.Join(valueStrs, ", "))
}

func (r *OneOfRule) Validate(value interface{}) error {
	for _, allowedValue := range r.Values {
		if reflect.DeepEqual(value, allowedValue) {
			return nil
		}
	}
	
	valueStrs := make([]string, len(r.Values))
	for i, v := range r.Values {
		valueStrs[i] = fmt.Sprintf("%v", v)
	}
	
	return fmt.Errorf("value %v is not one of allowed values: %s", value, strings.Join(valueStrs, ", "))
}

// 预定义验证器实例
var (
	Required    = &RequiredRule{}
	URL         = &URLRule{}
	IP          = &IPRule{}
	Port        = &PortRule{}
)

// 便捷函数

// ValidateServerConfig 验证服务器配置
func ValidateServerConfig(config interface{}) *ValidationResult {
	validator := NewValidator()
	
	// 添加通用服务器配置验证规则
	validator.AddRule("Host", IP)
	validator.AddRule("Port", Port)
	validator.AddRule("Port", NewRangeRule(1, 65535))
	
	return validator.Validate(config)
}

// ValidateNetworkConfig 验证网络配置
func ValidateNetworkConfig(config interface{}) *ValidationResult {
	validator := NewValidator()
	
	// 添加网络配置验证规则
	validator.AddRule("ReadTimeout", NewDurationRule(time.Millisecond, time.Hour))
	validator.AddRule("WriteTimeout", NewDurationRule(time.Millisecond, time.Hour))
	validator.AddRule("IdleTimeout", NewDurationRule(time.Second, 24*time.Hour))
	validator.AddRule("MaxConnections", NewRangeRule(1, 100000))
	
	return validator.Validate(config)
}