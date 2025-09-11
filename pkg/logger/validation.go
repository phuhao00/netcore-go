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

// Validator `n�?type Validator interface {
	Validate(entry *Entry) error
	GetName() string
	IsEnabled() bool
	SetEnabled(bool)
}

// FieldValidator `n�?type FieldValidator struct {
	mu      sync.RWMutex
	name    string
	enabled bool
	rules   []ValidationRule
	stats   ValidationStats
}

// ValidationRule `n`ntype ValidationRule interface {
	Validate(key string, value interface{}) error
	GetName() string
}

// ValidationStats `n`ntype ValidationStats struct {
	TotalValidations   int64 `json:"total_validations"`   // `n�?	SuccessValidations int64 `json:"success_validations"` // `n
	FailedValidations  int64 `json:"failed_validations"`  // `n
	RuleViolations     map[string]int64 `json:"rule_violations"` // `n
}

// NewFieldValidator `n�?func NewFieldValidator(name string) *FieldValidator {
	return &FieldValidator{
		name:    name,
		enabled: true,
		rules:   make([]ValidationRule, 0),
		stats: ValidationStats{
			RuleViolations: make(map[string]int64),
		},
	}
}

// AddRule `n`nfunc (v *FieldValidator) AddRule(rule ValidationRule) {
	v.mu.Lock()
	v.rules = append(v.rules, rule)
	v.mu.Unlock()
}

// Validate `n`nfunc (v *FieldValidator) Validate(entry *Entry) error {
	if !v.IsEnabled() {
		return nil
	}
	
	atomic.AddInt64(&v.stats.TotalValidations, 1)
	
	v.mu.RLock()
	rules := v.rules
	v.mu.RUnlock()
	
	// `n`nfor key, value := range entry.Fields {
		for _, rule := range rules {
			if err := rule.Validate(key, value); err != nil {
				atomic.AddInt64(&v.stats.FailedValidations, 1)
				
				// `n
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

// GetName `n�?func (v *FieldValidator) GetName() string {
	return v.name
}

// IsEnabled `n�?func (v *FieldValidator) IsEnabled() bool {
	v.mu.RLock()
	enabled := v.enabled
	v.mu.RUnlock()
	return enabled
}

// SetEnabled `n`nfunc (v *FieldValidator) SetEnabled(enabled bool) {
	v.mu.Lock()
	v.enabled = enabled
	v.mu.Unlock()
}

// GetStats `n`nfunc (v *FieldValidator) GetStats() ValidationStats {
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

// ResetStats `n`nfunc (v *FieldValidator) ResetStats() {
	atomic.StoreInt64(&v.stats.TotalValidations, 0)
	atomic.StoreInt64(&v.stats.SuccessValidations, 0)
	atomic.StoreInt64(&v.stats.FailedValidations, 0)
	
	v.mu.Lock()
	v.stats.RuleViolations = make(map[string]int64)
	v.mu.Unlock()
}

// RequiredFieldRule `n`ntype RequiredFieldRule struct {
	name           string
	requiredFields []string
}

// NewRequiredFieldRule `n`nfunc NewRequiredFieldRule(requiredFields []string) *RequiredFieldRule {
	return &RequiredFieldRule{
		name:           "required_field",
		requiredFields: requiredFields,
	}
}

// Validate `n`nfunc (r *RequiredFieldRule) Validate(key string, value interface{}) error {
	// `nEntry`n，`n�?	return nil
}

// ValidateEntry `nEntry
func (r *RequiredFieldRule) ValidateEntry(entry *Entry) error {
	for _, field := range r.requiredFields {
		if _, exists := entry.Fields[field]; !exists {
			return fmt.Errorf("required field '%s' is missing", field)
		}
	}
	return nil
}

// GetName `n`nfunc (r *RequiredFieldRule) GetName() string {
	return r.name
}

// TypeValidationRule `n`ntype TypeValidationRule struct {
	name       string
	fieldTypes map[string]reflect.Type
}

// NewTypeValidationRule `n`nfunc NewTypeValidationRule(fieldTypes map[string]reflect.Type) *TypeValidationRule {
	return &TypeValidationRule{
		name:       "type_validation",
		fieldTypes: fieldTypes,
	}
}

// Validate `n`nfunc (r *TypeValidationRule) Validate(key string, value interface{}) error {
	expectedType, exists := r.fieldTypes[key]
	if !exists {
		return nil // `n
	}
	
	actualType := reflect.TypeOf(value)
	if actualType != expectedType {
		return fmt.Errorf("field '%s' expected type %v, got %v", key, expectedType, actualType)
	}
	
	return nil
}

// GetName `n`nfunc (r *TypeValidationRule) GetName() string {
	return r.name
}

// LengthValidationRule `n`ntype LengthValidationRule struct {
	name      string
	minLength int
	maxLength int
	fields    []string // `n，`n�?}

// NewLengthValidationRule `n`nfunc NewLengthValidationRule(minLength, maxLength int, fields []string) *LengthValidationRule {
	return &LengthValidationRule{
		name:      "length_validation",
		minLength: minLength,
		maxLength: maxLength,
		fields:    fields,
	}
}

// Validate `n`nfunc (r *LengthValidationRule) Validate(key string, value interface{}) error {
	// `n`nif len(r.fields) > 0 {
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
	
	// `n�?	var length int
	switch v := value.(type) {
	case string:
		length = len(v)
	case []byte:
		length = len(v)
	default:
		length = len(fmt.Sprintf("%v", v))
	}
	
	// `n`nif r.minLength > 0 && length < r.minLength {
		return fmt.Errorf("field '%s' length %d is less than minimum %d", key, length, r.minLength)
	}
	
	if r.maxLength > 0 && length > r.maxLength {
		return fmt.Errorf("field '%s' length %d exceeds maximum %d", key, length, r.maxLength)
	}
	
	return nil
}

// GetName `n`nfunc (r *LengthValidationRule) GetName() string {
	return r.name
}

// RegexValidationRule `n�?type RegexValidationRule struct {
	name    string
	pattern *regexp.Regexp
	fields  []string // `n�?}

// NewRegexValidationRule `n�?func NewRegexValidationRule(pattern string, fields []string) (*RegexValidationRule, error) {
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

// Validate `n`nfunc (r *RegexValidationRule) Validate(key string, value interface{}) error {
	// `n
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
	
	// `n
	str := fmt.Sprintf("%v", value)
	
	// `n�?	if !r.pattern.MatchString(str) {
		return fmt.Errorf("field '%s' value '%s' does not match pattern", key, str)
	}
	
	return nil
}

// GetName `n`nfunc (r *RegexValidationRule) GetName() string {
	return r.name
}

// RangeValidationRule `n`ntype RangeValidationRule struct {
	name   string
	min    float64
	max    float64
	fields []string // `n�?}

// NewRangeValidationRule `n`nfunc NewRangeValidationRule(min, max float64, fields []string) *RangeValidationRule {
	return &RangeValidationRule{
		name:   "range_validation",
		min:    min,
		max:    max,
		fields: fields,
	}
}

// Validate `n`nfunc (r *RangeValidationRule) Validate(key string, value interface{}) error {
	// `n
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
	
	// `n�?	var num float64
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
			return fmt.Errorf("field '%s' value '%s' is not a valid number", key, v)
		}
	default:
		return fmt.Errorf("field '%s' value type %T is not numeric", key, value)
	}
	
	// `n`nif num < r.min {
		return fmt.Errorf("field '%s' value %f is less than minimum %f", key, num, r.min)
	}
	
	if num > r.max {
		return fmt.Errorf("field '%s' value %f exceeds maximum %f", key, num, r.max)
	}
	
	return nil
}

// GetName `n`nfunc (r *RangeValidationRule) GetName() string {
	return r.name
}

// EnumValidationRule `n`ntype EnumValidationRule struct {
	name         string
	allowedValues map[string]bool
	fields       []string // `n�?}

// NewEnumValidationRule `n`nfunc NewEnumValidationRule(allowedValues []string, fields []string) *EnumValidationRule {
	valueMap := make(map[string]bool)
	for _, value := range allowedValues {
		valueMap[value] = true
	}
	
	return &EnumValidationRule{
		name:          "enum_validation",
		allowedValues: valueMap,
		fields:        fields,
	}
}

// Validate `n`nfunc (r *EnumValidationRule) Validate(key string, value interface{}) error {
	// `n
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
	
	// `n
	str := fmt.Sprintf("%v", value)
	
	// `n�?	if !r.allowedValues[str] {
		return fmt.Errorf("field '%s' value '%s' is not in allowed values", key, str)
	}
	
	return nil
}

// GetName `n`nfunc (r *EnumValidationRule) GetName() string {
	return r.name
}

// CustomValidationRule `n�?type CustomValidationRule struct {
	name      string
	validator func(key string, value interface{}) error
}

// NewCustomValidationRule `n�?func NewCustomValidationRule(name string, validator func(string, interface{}) error) *CustomValidationRule {
	return &CustomValidationRule{
		name:      name,
		validator: validator,
	}
}

// Validate `n`nfunc (r *CustomValidationRule) Validate(key string, value interface{}) error {
	if r.validator == nil {
		return nil
	}
	return r.validator(key, value)
}

// GetName `n`nfunc (r *CustomValidationRule) GetName() string {
	return r.name
}

// ValidationManager `n�?type ValidationManager struct {
	mu         sync.RWMutex
	validators []Validator
	enabled    bool
	stats      ValidationManagerStats
	
	// `n
	config ValidationManagerConfig
	
	// `n
	onValidationError func(error, *Entry)
}

// ValidationManagerConfig `n�?type ValidationManagerConfig struct {
	Enabled           bool `json:"enabled"`             // `n
	StopOnFirstError  bool `json:"stop_on_first_error"` // `n
	MaxFieldCount     int  `json:"max_field_count"`     // `n�?	MaxValueSize      int  `json:"max_value_size"`      // `n�?	EnableMetrics     bool `json:"enable_metrics"`      // `n
}

// ValidationManagerStats `n�?type ValidationManagerStats struct {
	TotalEntries      int64 `json:"total_entries"`      // `n
	ValidEntries      int64 `json:"valid_entries"`      // `n�?	InvalidEntries    int64 `json:"invalid_entries"`    // `n�?	ValidationErrors  int64 `json:"validation_errors"`  // `n�?}

// NewValidationManager `n�?func NewValidationManager(config ValidationManagerConfig) *ValidationManager {
	// `n�?	if config.MaxFieldCount <= 0 {
		config.MaxFieldCount = 50
	}
	if config.MaxValueSize <= 0 {
		config.MaxValueSize = 1024
	}
	
	return &ValidationManager{
		validators: make([]Validator, 0),
		enabled:    config.Enabled,
		config:     config,
	}
}

// AddValidator `n�?func (vm *ValidationManager) AddValidator(validator Validator) {
	vm.mu.Lock()
	vm.validators = append(vm.validators, validator)
	vm.mu.Unlock()
}

// ValidateEntry `n`nfunc (vm *ValidationManager) ValidateEntry(entry *Entry) error {
	if !vm.enabled {
		return nil
	}
	
	if vm.config.EnableMetrics {
		atomic.AddInt64(&vm.stats.TotalEntries, 1)
	}
	
	// `n`nif err := vm.basicValidation(entry); err != nil {
		if vm.config.EnableMetrics {
			atomic.AddInt64(&vm.stats.InvalidEntries, 1)
			atomic.AddInt64(&vm.stats.ValidationErrors, 1)
		}
		
		if vm.onValidationError != nil {
			vm.onValidationError(err, entry)
		}
		
		return err
	}
	
	// `n
	vm.mu.RLock()
	validators := vm.validators
	stopOnFirstError := vm.config.StopOnFirstError
	vm.mu.RUnlock()
	
	var errors []error
	for _, validator := range validators {
		if !validator.IsEnabled() {
			continue
		}
		
		if err := validator.Validate(entry); err != nil {
			if vm.config.EnableMetrics {
				atomic.AddInt64(&vm.stats.ValidationErrors, 1)
			}
			
			if vm.onValidationError != nil {
				vm.onValidationError(err, entry)
			}
			
			if stopOnFirstError {
				if vm.config.EnableMetrics {
					atomic.AddInt64(&vm.stats.InvalidEntries, 1)
				}
				return err
			}
			
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		if vm.config.EnableMetrics {
			atomic.AddInt64(&vm.stats.InvalidEntries, 1)
		}
		return fmt.Errorf("validation failed with %d errors: %v", len(errors), errors)
	}
	
	if vm.config.EnableMetrics {
		atomic.AddInt64(&vm.stats.ValidEntries, 1)
	}
	
	return nil
}

// basicValidation `n`nfunc (vm *ValidationManager) basicValidation(entry *Entry) error {
	// `n`nif vm.config.MaxFieldCount > 0 && len(entry.Fields) > vm.config.MaxFieldCount {
		return fmt.Errorf("field count %d exceeds maximum %d", len(entry.Fields), vm.config.MaxFieldCount)
	}
	
	// `n�?	if vm.config.MaxValueSize > 0 {
		for key, value := range entry.Fields {
			valueStr := fmt.Sprintf("%v", value)
			if len(valueStr) > vm.config.MaxValueSize {
				return fmt.Errorf("field '%s' value size %d exceeds maximum %d", key, len(valueStr), vm.config.MaxValueSize)
			}
		}
	}
	
	// `n�?	if strings.TrimSpace(entry.Message) == "" {
		return fmt.Errorf("log message cannot be empty")
	}
	
	return nil
}

// SetEnabled `n`nfunc (vm *ValidationManager) SetEnabled(enabled bool) {
	vm.mu.Lock()
	vm.enabled = enabled
	vm.mu.Unlock()
}

// IsEnabled `n�?func (vm *ValidationManager) IsEnabled() bool {
	vm.mu.RLock()
	enabled := vm.enabled
	vm.mu.RUnlock()
	return enabled
}

// SetValidationErrorCallback `n`nfunc (vm *ValidationManager) SetValidationErrorCallback(callback func(error, *Entry)) {
	vm.mu.Lock()
	vm.onValidationError = callback
	vm.mu.Unlock()
}

// GetStats `n`nfunc (vm *ValidationManager) GetStats() ValidationManagerStats {
	return ValidationManagerStats{
		TotalEntries:     atomic.LoadInt64(&vm.stats.TotalEntries),
		ValidEntries:     atomic.LoadInt64(&vm.stats.ValidEntries),
		InvalidEntries:   atomic.LoadInt64(&vm.stats.InvalidEntries),
		ValidationErrors: atomic.LoadInt64(&vm.stats.ValidationErrors),
	}
}

// ResetStats `n`nfunc (vm *ValidationManager) ResetStats() {
	atomic.StoreInt64(&vm.stats.TotalEntries, 0)
	atomic.StoreInt64(&vm.stats.ValidEntries, 0)
	atomic.StoreInt64(&vm.stats.InvalidEntries, 0)
	atomic.StoreInt64(&vm.stats.ValidationErrors, 0)
}

// GetValidationRate `n�?func (vm *ValidationManager) GetValidationRate() float64 {
	total := atomic.LoadInt64(&vm.stats.TotalEntries)
	valid := atomic.LoadInt64(&vm.stats.ValidEntries)
	
	if total == 0 {
		return 0
	}
	
	return float64(valid) / float64(total)
}