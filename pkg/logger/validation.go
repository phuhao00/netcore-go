// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Validator 验证器接口
type Validator interface {
	Validate(entry *Entry) error
	GetName() string
	IsEnabled() bool
	SetEnabled(bool)
}

// FieldValidator 字段验证器
type FieldValidator struct {
	mu      sync.RWMutex
	name    string
	enabled bool
	rules   []ValidationRule
	stats   ValidationStats
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(key string, value interface{}) error
	GetName() string
}

// ValidationStats 验证统计信息
type ValidationStats struct {
	TotalValidations   int64            `json:"total_validations"`   // 总验证次数
	SuccessValidations int64            `json:"success_validations"` // 成功验证次数
	FailedValidations  int64            `json:"failed_validations"`  // 失败验证次数
	RuleViolations     map[string]int64 `json:"rule_violations"`     // 规则违反次数
}

// NewFieldValidator 创建字段验证器
func NewFieldValidator(name string) *FieldValidator {
	return &FieldValidator{
		name:    name,
		enabled: true,
		rules:   make([]ValidationRule, 0),
		stats: ValidationStats{
			RuleViolations: make(map[string]int64),
		},
	}
}

// AddRule 添加验证规则
func (v *FieldValidator) AddRule(rule ValidationRule) {
	v.mu.Lock()
	v.rules = append(v.rules, rule)
	v.mu.Unlock()
}

// Validate 验证日志条目
func (v *FieldValidator) Validate(entry *Entry) error {
	if !v.IsEnabled() {
		return nil
	}
	
	atomic.AddInt64(&v.stats.TotalValidations, 1)
	
	v.mu.RLock()
	rules := v.rules
	v.mu.RUnlock()
	
	// 验证所有字段
	for key, value := range entry.Fields {
		for _, rule := range rules {
			if err := rule.Validate(key, value); err != nil {
				atomic.AddInt64(&v.stats.FailedValidations, 1)
				
				// 记录规则违反
				v.mu.Lock()
				v.stats.RuleViolations[rule.GetName()]++
				v.mu.Unlock()
				
				return fmt.Errorf("field validation failed for key '%s': %w", key, err)
			}
		}
	}
	
	atomic.AddInt64(&v.stats.SuccessValidations, 1)
	return nil
}

// GetName 获取验证器名称
func (v *FieldValidator) GetName() string {
	return v.name
}

// IsEnabled 检查是否启用
func (v *FieldValidator) IsEnabled() bool {
	v.mu.RLock()
	enabled := v.enabled
	v.mu.RUnlock()
	return enabled
}

// SetEnabled 设置启用状态
func (v *FieldValidator) SetEnabled(enabled bool) {
	v.mu.Lock()
	v.enabled = enabled
	v.mu.Unlock()
}

// GetStats 获取统计信息
func (v *FieldValidator) GetStats() ValidationStats {
	v.mu.RLock()
	ruleViolations := make(map[string]int64)
	for k, v := range v.stats.RuleViolations {
		ruleViolations[k] = v
	}
	v.mu.RUnlock()
	
	return ValidationStats{
		TotalValidations:   atomic.LoadInt64(&v.stats.TotalValidations),
		SuccessValidations: atomic.LoadInt64(&v.stats.SuccessValidations),
		FailedValidations:  atomic.LoadInt64(&v.stats.FailedValidations),
		RuleViolations:     ruleViolations,
	}
}

// ResetStats 重置统计信息
func (v *FieldValidator) ResetStats() {
	atomic.StoreInt64(&v.stats.TotalValidations, 0)
	atomic.StoreInt64(&v.stats.SuccessValidations, 0)
	atomic.StoreInt64(&v.stats.FailedValidations, 0)
	
	v.mu.Lock()
	v.stats.RuleViolations = make(map[string]int64)
	v.mu.Unlock()
}

// RequiredFieldRule 必需字段规则
type RequiredFieldRule struct {
	name           string
	requiredFields []string
}

// NewRequiredFieldRule 创建必需字段规则
func NewRequiredFieldRule(requiredFields []string) *RequiredFieldRule {
	return &RequiredFieldRule{
		name:           "required_field",
		requiredFields: requiredFields,
	}
}

// Validate 验证字段
func (r *RequiredFieldRule) Validate(key string, value interface{}) error {
	// 这个规则需要在Entry级别验证，这里返回nil
	return nil
}

// ValidateEntry 验证Entry
func (r *RequiredFieldRule) ValidateEntry(entry *Entry) error {
	for _, field := range r.requiredFields {
		if _, exists := entry.Fields[field]; !exists {
			return fmt.Errorf("required field '%s' is missing", field)
		}
	}
	return nil
}

// GetName 获取规则名称
func (r *RequiredFieldRule) GetName() string {
	return r.name
}

// TypeValidationRule 类型验证规则
type TypeValidationRule struct {
	name       string
	fieldTypes map[string]reflect.Type
}

// NewTypeValidationRule 创建类型验证规则
func NewTypeValidationRule(fieldTypes map[string]reflect.Type) *TypeValidationRule {
	return &TypeValidationRule{
		name:       "type_validation",
		fieldTypes: fieldTypes,
	}
}

// Validate 验证字段类型
func (r *TypeValidationRule) Validate(key string, value interface{}) error {
	expectedType, exists := r.fieldTypes[key]
	if !exists {
		return nil // 不在验证范围内
	}
	
	actualType := reflect.TypeOf(value)
	if actualType != expectedType {
		return fmt.Errorf("field '%s' expected type %v, got %v", key, expectedType, actualType)
	}
	
	return nil
}

// GetName 获取规则名称
func (r *TypeValidationRule) GetName() string {
	return r.name
}

// LengthValidationRule 长度验证规则
type LengthValidationRule struct {
	name      string
	minLength int
	maxLength int
	fields    []string // 需要验证的字段，为空则验证所有字符串字段
}

// NewLengthValidationRule 创建长度验证规则
func NewLengthValidationRule(minLength, maxLength int, fields []string) *LengthValidationRule {
	return &LengthValidationRule{
		name:      "length_validation",
		minLength: minLength,
		maxLength: maxLength,
		fields:    fields,
	}
}

// Validate 验证字段长度
func (r *LengthValidationRule) Validate(key string, value interface{}) error {
	// 检查是否需要验证此字段
	if len(r.fields) > 0 {
		found := false
		for _, field := range r.fields {
			if field == key {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	
	// 只验证字符串类型
	str, ok := value.(string)
	if !ok {
		return nil
	}
	
	length := len(str)
	if r.minLength > 0 && length < r.minLength {
		return fmt.Errorf("field '%s' length %d is less than minimum %d", key, length, r.minLength)
	}
	
	if r.maxLength > 0 && length > r.maxLength {
		return fmt.Errorf("field '%s' length %d exceeds maximum %d", key, length, r.maxLength)
	}
	
	return nil
}

// GetName 获取规则名称
func (r *LengthValidationRule) GetName() string {
	return r.name
}

// RegexValidationRule 正则表达式验证规则
type RegexValidationRule struct {
	name    string
	pattern *regexp.Regexp
	fields  []string
}

// NewRegexValidationRule 创建正则表达式验证规则
func NewRegexValidationRule(pattern string, fields []string) (*RegexValidationRule, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	return &RegexValidationRule{
		name:    "regex_validation",
		pattern: regex,
		fields:  fields,
	}, nil
}

// Validate 验证字段格式
func (r *RegexValidationRule) Validate(key string, value interface{}) error {
	// 检查是否需要验证此字段
	if len(r.fields) > 0 {
		found := false
		for _, field := range r.fields {
			if field == key {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	
	// 只验证字符串类型
	str, ok := value.(string)
	if !ok {
		return nil
	}
	
	if !r.pattern.MatchString(str) {
		return fmt.Errorf("field '%s' value '%s' does not match pattern", key, str)
	}
	
	return nil
}

// GetName 获取规则名称
func (r *RegexValidationRule) GetName() string {
	return r.name
}

// RangeValidationRule 范围验证规则
type RangeValidationRule struct {
	name   string
	min    float64
	max    float64
	fields []string
}

// NewRangeValidationRule 创建范围验证规则
func NewRangeValidationRule(min, max float64, fields []string) *RangeValidationRule {
	return &RangeValidationRule{
		name:   "range_validation",
		min:    min,
		max:    max,
		fields: fields,
	}
}

// Validate 验证数值范围
func (r *RangeValidationRule) Validate(key string, value interface{}) error {
	// 检查是否需要验证此字段
	if len(r.fields) > 0 {
		found := false
		for _, field := range r.fields {
			if field == key {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	
	// 转换为数值
	var num float64
	var err error
	
	switch v := value.(type) {
	case int:
		num = float64(v)
	case int32:
		num = float64(v)
	case int64:
		num = float64(v)
	case float32:
		num = float64(v)
	case float64:
		num = v
	case string:
		num, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return nil // 不是数值，跳过验证
		}
	default:
		return nil // 不支持的类型，跳过验证
	}
	
	if num < r.min {
		return fmt.Errorf("field '%s' value %f is less than minimum %f", key, num, r.min)
	}
	
	if num > r.max {
		return fmt.Errorf("field '%s' value %f exceeds maximum %f", key, num, r.max)
	}
	
	return nil
}

// GetName 获取规则名称
func (r *RangeValidationRule) GetName() string {
	return r.name
}

// BlacklistValidationRule 黑名单验证规则
type BlacklistValidationRule struct {
	name      string
	blacklist map[string]bool
	fields    []string
}

// NewBlacklistValidationRule 创建黑名单验证规则
func NewBlacklistValidationRule(blacklist []string, fields []string) *BlacklistValidationRule {
	blacklistMap := make(map[string]bool)
	for _, item := range blacklist {
		blacklistMap[strings.ToLower(item)] = true
	}
	
	return &BlacklistValidationRule{
		name:      "blacklist_validation",
		blacklist: blacklistMap,
		fields:    fields,
	}
}

// Validate 验证黑名单
func (r *BlacklistValidationRule) Validate(key string, value interface{}) error {
	// 检查是否需要验证此字段
	if len(r.fields) > 0 {
		found := false
		for _, field := range r.fields {
			if field == key {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	
	// 只验证字符串类型
	str, ok := value.(string)
	if !ok {
		return nil
	}
	
	if r.blacklist[strings.ToLower(str)] {
		return fmt.Errorf("field '%s' contains blacklisted value '%s'", key, str)
	}
	
	return nil
}

// GetName 获取规则名称
func (r *BlacklistValidationRule) GetName() string {
	return r.name
}

// CompositeValidator 复合验证器
type CompositeValidator struct {
	mu         sync.RWMutex
	name       string
	enabled    bool
	validators []Validator
	stats      ValidationStats
}

// NewCompositeValidator 创建复合验证器
func NewCompositeValidator(name string) *CompositeValidator {
	return &CompositeValidator{
		name:       name,
		enabled:    true,
		validators: make([]Validator, 0),
		stats: ValidationStats{
			RuleViolations: make(map[string]int64),
		},
	}
}

// AddValidator 添加验证器
func (v *CompositeValidator) AddValidator(validator Validator) {
	v.mu.Lock()
	v.validators = append(v.validators, validator)
	v.mu.Unlock()
}

// Validate 验证日志条目
func (v *CompositeValidator) Validate(entry *Entry) error {
	if !v.IsEnabled() {
		return nil
	}
	
	atomic.AddInt64(&v.stats.TotalValidations, 1)
	
	v.mu.RLock()
	validators := v.validators
	v.mu.RUnlock()
	
	// 执行所有验证器
	for _, validator := range validators {
		if err := validator.Validate(entry); err != nil {
			atomic.AddInt64(&v.stats.FailedValidations, 1)
			
			// 记录验证器违反
			v.mu.Lock()
			v.stats.RuleViolations[validator.GetName()]++
			v.mu.Unlock()
			
			return fmt.Errorf("validator '%s' failed: %w", validator.GetName(), err)
		}
	}
	
	atomic.AddInt64(&v.stats.SuccessValidations, 1)
	return nil
}

// GetName 获取验证器名称
func (v *CompositeValidator) GetName() string {
	return v.name
}

// IsEnabled 检查是否启用
func (v *CompositeValidator) IsEnabled() bool {
	v.mu.RLock()
	enabled := v.enabled
	v.mu.RUnlock()
	return enabled
}

// SetEnabled 设置启用状态
func (v *CompositeValidator) SetEnabled(enabled bool) {
	v.mu.Lock()
	v.enabled = enabled
	v.mu.Unlock()
}

// GetStats 获取统计信息
func (v *CompositeValidator) GetStats() ValidationStats {
	v.mu.RLock()
	ruleViolations := make(map[string]int64)
	for k, v := range v.stats.RuleViolations {
		ruleViolations[k] = v
	}
	v.mu.RUnlock()
	
	return ValidationStats{
		TotalValidations:   atomic.LoadInt64(&v.stats.TotalValidations),
		SuccessValidations: atomic.LoadInt64(&v.stats.SuccessValidations),
		FailedValidations:  atomic.LoadInt64(&v.stats.FailedValidations),
		RuleViolations:     ruleViolations,
	}
}

// ResetStats 重置统计信息
func (v *CompositeValidator) ResetStats() {
	atomic.StoreInt64(&v.stats.TotalValidations, 0)
	atomic.StoreInt64(&v.stats.SuccessValidations, 0)
	atomic.StoreInt64(&v.stats.FailedValidations, 0)
	
	v.mu.Lock()
	v.stats.RuleViolations = make(map[string]int64)
	v.mu.Unlock()
}