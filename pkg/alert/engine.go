package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	// LevelInfo 信息级别
	LevelInfo AlertLevel = "info"
	// LevelWarning 警告级别
	LevelWarning AlertLevel = "warning"
	// LevelError 错误级别
	LevelError AlertLevel = "error"
	// LevelCritical 严重级别
	LevelCritical AlertLevel = "critical"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	// StatusFiring 触发中
	StatusFiring AlertStatus = "firing"
	// StatusResolved 已解决
	StatusResolved AlertStatus = "resolved"
	// StatusSuppressed 已抑制
	StatusSuppressed AlertStatus = "suppressed"
	// StatusAcknowledged 已确认
	StatusAcknowledged AlertStatus = "acknowledged"
)

// ComparisonOperator 比较操作符
type ComparisonOperator string

const (
	// OpEqual 等于
	OpEqual ComparisonOperator = "eq"
	// OpNotEqual 不等于
	OpNotEqual ComparisonOperator = "ne"
	// OpGreaterThan 大于
	OpGreaterThan ComparisonOperator = "gt"
	// OpGreaterThanOrEqual 大于等于
	OpGreaterThanOrEqual ComparisonOperator = "gte"
	// OpLessThan 小于
	OpLessThan ComparisonOperator = "lt"
	// OpLessThanOrEqual 小于等于
	OpLessThanOrEqual ComparisonOperator = "lte"
	// OpContains 包含
	OpContains ComparisonOperator = "contains"
	// OpNotContains 不包含
	OpNotContains ComparisonOperator = "not_contains"
	// OpRegex 正则匹配
	OpRegex ComparisonOperator = "regex"
	// OpNotRegex 正则不匹配
	OpNotRegex ComparisonOperator = "not_regex"
)

// AlertEngineConfig 告警引擎配置
type AlertEngineConfig struct {
	// 启用告警引擎
	Enabled bool `json:"enabled" yaml:"enabled"`
	// 评估间隔
	EvaluationInterval time.Duration `json:"evaluation_interval" yaml:"evaluation_interval"`
	// 最大并发评估数
	MaxConcurrentEvaluations int `json:"max_concurrent_evaluations" yaml:"max_concurrent_evaluations"`
	// 告警历史保留时间
	AlertHistoryRetention time.Duration `json:"alert_history_retention" yaml:"alert_history_retention"`
	// 默认通知渠道
	DefaultNotifiers []string `json:"default_notifiers" yaml:"default_notifiers"`
	// 启用告警抑制
	SuppressionEnabled bool `json:"suppression_enabled" yaml:"suppression_enabled"`
	// 启用告警分组
	GroupingEnabled bool `json:"grouping_enabled" yaml:"grouping_enabled"`
	// 分组等待时间
	GroupWait time.Duration `json:"group_wait" yaml:"group_wait"`
	// 分组间隔
	GroupInterval time.Duration `json:"group_interval" yaml:"group_interval"`
	// 重复间隔
	RepeatInterval time.Duration `json:"repeat_interval" yaml:"repeat_interval"`
}

// DefaultAlertEngineConfig 默认告警引擎配置
func DefaultAlertEngineConfig() *AlertEngineConfig {
	return &AlertEngineConfig{
		Enabled:                   true,
		EvaluationInterval:        30 * time.Second,
		MaxConcurrentEvaluations:  10,
		AlertHistoryRetention:     7 * 24 * time.Hour, // 7天
		DefaultNotifiers:          []string{"console"},
		SuppressionEnabled:        true,
		GroupingEnabled:           true,
		GroupWait:                 10 * time.Second,
		GroupInterval:             5 * time.Minute,
		RepeatInterval:            4 * time.Hour,
	}
}

// AlertRule 告警规则
type AlertRule struct {
	// 规则ID
	ID string `json:"id" yaml:"id"`
	// 规则名称
	Name string `json:"name" yaml:"name"`
	// 描述
	Description string `json:"description" yaml:"description"`
	// 表达式
	Expression string `json:"expression" yaml:"expression"`
	// 条件
	Conditions []AlertCondition `json:"conditions" yaml:"conditions"`
	// 告警级别
	Level AlertLevel `json:"level" yaml:"level"`
	// 持续时间
	Duration time.Duration `json:"duration" yaml:"duration"`
	// 标签
	Labels map[string]string `json:"labels" yaml:"labels"`
	// 注解
	Annotations map[string]string `json:"annotations" yaml:"annotations"`
	// 通知渠道
	Notifiers []string `json:"notifiers" yaml:"notifiers"`
	// 启用状态
	Enabled bool `json:"enabled" yaml:"enabled"`
	// 创建时间
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	// 最后评估时间
	LastEvaluatedAt time.Time `json:"last_evaluated_at" yaml:"last_evaluated_at"`
	// 触发次数
	TriggerCount int64 `json:"trigger_count" yaml:"trigger_count"`
}

// AlertCondition 告警条件
type AlertCondition struct {
	// 指标名称
	Metric string `json:"metric" yaml:"metric"`
	// 操作符
	Operator ComparisonOperator `json:"operator" yaml:"operator"`
	// 阈值
	Threshold interface{} `json:"threshold" yaml:"threshold"`
	// 时间窗口
	TimeWindow time.Duration `json:"time_window" yaml:"time_window"`
	// 聚合函数
	Aggregation string `json:"aggregation" yaml:"aggregation"`
	// 标签过滤器
	LabelFilters map[string]string `json:"label_filters" yaml:"label_filters"`
}

// Alert 告警实例
type Alert struct {
	// 告警ID
	ID string `json:"id"`
	// 规则ID
	RuleID string `json:"rule_id"`
	// 规则名称
	RuleName string `json:"rule_name"`
	// 告警级别
	Level AlertLevel `json:"level"`
	// 状态
	Status AlertStatus `json:"status"`
	// 消息
	Message string `json:"message"`
	// 详细信息
	Details map[string]interface{} `json:"details"`
	// 标签
	Labels map[string]string `json:"labels"`
	// 注解
	Annotations map[string]string `json:"annotations"`
	// 触发时间
	FiredAt time.Time `json:"fired_at"`
	// 解决时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// 确认时间
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	// 确认用户
	AcknowledgedBy string `json:"acknowledged_by,omitempty"`
	// 通知状态
	NotificationStatus map[string]NotificationStatus `json:"notification_status"`
	// 分组键
	GroupKey string `json:"group_key"`
	// 指纹
	Fingerprint string `json:"fingerprint"`
}

// NotificationStatus 通知状态
type NotificationStatus struct {
	// 状态
	Status string `json:"status"`
	// 发送时间
	SentAt time.Time `json:"sent_at"`
	// 错误信息
	Error string `json:"error,omitempty"`
	// 重试次数
	RetryCount int `json:"retry_count"`
}

// MetricValue 指标值
type MetricValue struct {
	// 指标名称
	Name string `json:"name"`
	// 值
	Value interface{} `json:"value"`
	// 标签
	Labels map[string]string `json:"labels"`
	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// MetricProvider 指标提供者接口
type MetricProvider interface {
	// GetMetric 获取指标值
	GetMetric(name string, labels map[string]string, timeWindow time.Duration) ([]MetricValue, error)
}

// Notifier 通知器接口
type Notifier interface {
	// Name 通知器名称
	Name() string
	// Send 发送通知
	Send(ctx context.Context, alert Alert) error
}

// AlertEngine 告警引擎
type AlertEngine struct {
	config          *AlertEngineConfig
	rules           map[string]*AlertRule
	alerts          map[string]*Alert
	notifiers       map[string]Notifier
	metricProvider  MetricProvider
	mu              sync.RWMutex
	running         bool
	cancel          context.CancelFunc
	stats           *AlertEngineStats
	alertHistory    []*Alert
	suppressionRules []SuppressionRule
	groupedAlerts   map[string][]*Alert
}

// AlertEngineStats 告警引擎统计
type AlertEngineStats struct {
	TotalRules        int64     `json:"total_rules"`
	ActiveRules       int64     `json:"active_rules"`
	TotalAlerts       int64     `json:"total_alerts"`
	FiringAlerts      int64     `json:"firing_alerts"`
	ResolvedAlerts    int64     `json:"resolved_alerts"`
	SuppressedAlerts  int64     `json:"suppressed_alerts"`
	TotalEvaluations  int64     `json:"total_evaluations"`
	FailedEvaluations int64     `json:"failed_evaluations"`
	LastEvaluationTime time.Time `json:"last_evaluation_time"`
	AverageEvaluationTime float64 `json:"average_evaluation_time_ms"`
}

// SuppressionRule 抑制规则
type SuppressionRule struct {
	// 规则ID
	ID string `json:"id"`
	// 源匹配器
	SourceMatchers []LabelMatcher `json:"source_matchers"`
	// 目标匹配器
	TargetMatchers []LabelMatcher `json:"target_matchers"`
	// 相等标签
	Equal []string `json:"equal"`
	// 启用状态
	Enabled bool `json:"enabled"`
}

// LabelMatcher 标签匹配器
type LabelMatcher struct {
	// 标签名
	Name string `json:"name"`
	// 值
	Value string `json:"value"`
	// 操作符
	Operator ComparisonOperator `json:"operator"`
}

// NewAlertEngine 创建告警引擎
func NewAlertEngine(config *AlertEngineConfig, metricProvider MetricProvider) *AlertEngine {
	if config == nil {
		config = DefaultAlertEngineConfig()
	}

	return &AlertEngine{
		config:           config,
		rules:            make(map[string]*AlertRule),
		alerts:           make(map[string]*Alert),
		notifiers:        make(map[string]Notifier),
		metricProvider:   metricProvider,
		stats:            &AlertEngineStats{},
		alertHistory:     make([]*Alert, 0),
		suppressionRules: make([]SuppressionRule, 0),
		groupedAlerts:    make(map[string][]*Alert),
	}
}

// RegisterNotifier 注册通知器
func (ae *AlertEngine) RegisterNotifier(notifier Notifier) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.notifiers[notifier.Name()] = notifier
}

// AddRule 添加告警规则
func (ae *AlertEngine) AddRule(rule *AlertRule) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	// 验证规则
	if err := ae.validateRule(rule); err != nil {
		return err
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	ae.rules[rule.ID] = rule
	atomic.AddInt64(&ae.stats.TotalRules, 1)

	if rule.Enabled {
		atomic.AddInt64(&ae.stats.ActiveRules, 1)
	}

	return nil
}

// RemoveRule 移除告警规则
func (ae *AlertEngine) RemoveRule(ruleID string) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	rule, exists := ae.rules[ruleID]
	if !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	delete(ae.rules, ruleID)
	atomic.AddInt64(&ae.stats.TotalRules, -1)

	if rule.Enabled {
		atomic.AddInt64(&ae.stats.ActiveRules, -1)
	}

	// 解决相关的告警
	for alertID, alert := range ae.alerts {
		if alert.RuleID == ruleID {
			ae.resolveAlert(alertID, "Rule removed")
		}
	}

	return nil
}

// UpdateRule 更新告警规则
func (ae *AlertEngine) UpdateRule(rule *AlertRule) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	oldRule, exists := ae.rules[rule.ID]
	if !exists {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	// 验证规则
	if err := ae.validateRule(rule); err != nil {
		return err
	}

	// 更新启用状态统计
	if oldRule.Enabled && !rule.Enabled {
		atomic.AddInt64(&ae.stats.ActiveRules, -1)
	} else if !oldRule.Enabled && rule.Enabled {
		atomic.AddInt64(&ae.stats.ActiveRules, 1)
	}

	rule.CreatedAt = oldRule.CreatedAt
	rule.UpdatedAt = time.Now()
	rule.TriggerCount = oldRule.TriggerCount

	ae.rules[rule.ID] = rule

	return nil
}

// validateRule 验证规则
func (ae *AlertEngine) validateRule(rule *AlertRule) error {
	if len(rule.Conditions) == 0 && rule.Expression == "" {
		return fmt.Errorf("rule must have either conditions or expression")
	}

	// 验证条件
	for i, condition := range rule.Conditions {
		if condition.Metric == "" {
			return fmt.Errorf("condition %d: metric name cannot be empty", i)
		}

		if condition.Threshold == nil {
			return fmt.Errorf("condition %d: threshold cannot be nil", i)
		}
	}

	// 验证通知器
	for _, notifierName := range rule.Notifiers {
		if _, exists := ae.notifiers[notifierName]; !exists {
			return fmt.Errorf("notifier not found: %s", notifierName)
		}
	}

	return nil
}

// Start 启动告警引擎
func (ae *AlertEngine) Start(ctx context.Context) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	if ae.running {
		return fmt.Errorf("alert engine already running")
	}

	if !ae.config.Enabled {
		return fmt.Errorf("alert engine is disabled")
	}

	ctx, cancel := context.WithCancel(ctx)
	ae.cancel = cancel
	ae.running = true

	// 启动评估循环
	go ae.evaluationLoop(ctx)

	// 启动清理循环
	go ae.cleanupLoop(ctx)

	// 启动通知循环
	if ae.config.GroupingEnabled {
		go ae.notificationLoop(ctx)
	}

	return nil
}

// Stop 停止告警引擎
func (ae *AlertEngine) Stop() error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	if !ae.running {
		return fmt.Errorf("alert engine not running")
	}

	ae.cancel()
	ae.running = false

	return nil
}

// evaluationLoop 评估循环
func (ae *AlertEngine) evaluationLoop(ctx context.Context) {
	ticker := time.NewTicker(ae.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ae.evaluateRules(ctx)
		}
	}
}

// evaluateRules 评估所有规则
func (ae *AlertEngine) evaluateRules(ctx context.Context) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		ae.updateEvaluationStats(duration)
	}()

	ae.mu.RLock()
	rules := make([]*AlertRule, 0, len(ae.rules))
	for _, rule := range ae.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	ae.mu.RUnlock()

	// 限制并发评估数
	semaphore := make(chan struct{}, ae.config.MaxConcurrentEvaluations)
	var wg sync.WaitGroup

	for _, rule := range rules {
		wg.Add(1)
		go func(r *AlertRule) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ae.evaluateRule(ctx, r)
		}(rule)
	}

	wg.Wait()
}

// evaluateRule 评估单个规则
func (ae *AlertEngine) evaluateRule(ctx context.Context, rule *AlertRule) {
	atomic.AddInt64(&ae.stats.TotalEvaluations, 1)

	// 更新最后评估时间
	ae.mu.Lock()
	rule.LastEvaluatedAt = time.Now()
	ae.mu.Unlock()

	var shouldFire bool
	var alertMessage string
	var alertDetails map[string]interface{}

	if rule.Expression != "" {
		// 使用表达式评估
		result, err := ae.evaluateExpression(ctx, rule.Expression)
		if err != nil {
			atomic.AddInt64(&ae.stats.FailedEvaluations, 1)
			return
		}
		shouldFire = result
		alertMessage = fmt.Sprintf("Expression '%s' evaluated to true", rule.Expression)
	} else {
		// 使用条件评估
		result, message, details, err := ae.evaluateConditions(ctx, rule.Conditions)
		if err != nil {
			atomic.AddInt64(&ae.stats.FailedEvaluations, 1)
			return
		}
		shouldFire = result
		alertMessage = message
		alertDetails = details
	}

	// 生成告警指纹
	fingerprint := ae.generateFingerprint(rule, rule.Labels)

	ae.mu.Lock()
	existingAlert, exists := ae.alerts[fingerprint]
	ae.mu.Unlock()

	if shouldFire {
		if !exists {
			// 创建新告警
			alert := &Alert{
				ID:                 ae.generateAlertID(),
				RuleID:             rule.ID,
				RuleName:           rule.Name,
				Level:              rule.Level,
				Status:             StatusFiring,
				Message:            alertMessage,
				Details:            alertDetails,
				Labels:             rule.Labels,
				Annotations:        rule.Annotations,
				FiredAt:            time.Now(),
				NotificationStatus: make(map[string]NotificationStatus),
				GroupKey:           ae.generateGroupKey(rule.Labels),
				Fingerprint:        fingerprint,
			}

			ae.mu.Lock()
			ae.alerts[fingerprint] = alert
			atomic.AddInt64(&rule.TriggerCount, 1)
			atomic.AddInt64(&ae.stats.TotalAlerts, 1)
			atomic.AddInt64(&ae.stats.FiringAlerts, 1)
			ae.mu.Unlock()

			// 检查抑制
			if ae.config.SuppressionEnabled && ae.isAlertSuppressed(alert) {
				alert.Status = StatusSuppressed
				atomic.AddInt64(&ae.stats.SuppressedAlerts, 1)
				atomic.AddInt64(&ae.stats.FiringAlerts, -1)
			} else {
				// 发送通知
				ae.sendNotification(alert, rule.Notifiers)
			}
		}
	} else {
		if exists && existingAlert.Status == StatusFiring {
			// 解决告警
			ae.resolveAlert(fingerprint, "Condition no longer met")
		}
	}
}

// evaluateExpression 评估表达式
func (ae *AlertEngine) evaluateExpression(ctx context.Context, expression string) (bool, error) {
	// 简化实现，实际应该支持复杂的表达式解析
	// 这里只是示例
	return strings.Contains(expression, "true"), nil
}

// evaluateConditions 评估条件
func (ae *AlertEngine) evaluateConditions(ctx context.Context, conditions []AlertCondition) (bool, string, map[string]interface{}, error) {
	if len(conditions) == 0 {
		return false, "", nil, fmt.Errorf("no conditions to evaluate")
	}

	var messages []string
	details := make(map[string]interface{})
	allConditionsMet := true

	for i, condition := range conditions {
		met, message, conditionDetails, err := ae.evaluateCondition(ctx, condition)
		if err != nil {
			return false, "", nil, err
		}

		if !met {
			allConditionsMet = false
		}

		messages = append(messages, message)
		details[fmt.Sprintf("condition_%d", i)] = conditionDetails
	}

	return allConditionsMet, strings.Join(messages, "; "), details, nil
}

// evaluateCondition 评估单个条件
func (ae *AlertEngine) evaluateCondition(ctx context.Context, condition AlertCondition) (bool, string, map[string]interface{}, error) {
	if ae.metricProvider == nil {
		return false, "", nil, fmt.Errorf("metric provider not configured")
	}

	// 获取指标值
	metrics, err := ae.metricProvider.GetMetric(condition.Metric, condition.LabelFilters, condition.TimeWindow)
	if err != nil {
		return false, "", nil, err
	}

	if len(metrics) == 0 {
		return false, "No metric data available", nil, nil
	}

	// 聚合指标值
	aggregatedValue, err := ae.aggregateMetrics(metrics, condition.Aggregation)
	if err != nil {
		return false, "", nil, err
	}

	// 比较值
	met, err := ae.compareValues(aggregatedValue, condition.Operator, condition.Threshold)
	if err != nil {
		return false, "", nil, err
	}

	message := fmt.Sprintf("%s %s %v (actual: %v)", condition.Metric, condition.Operator, condition.Threshold, aggregatedValue)
	details := map[string]interface{}{
		"metric":           condition.Metric,
		"operator":         condition.Operator,
		"threshold":        condition.Threshold,
		"actual_value":     aggregatedValue,
		"aggregation":      condition.Aggregation,
		"time_window":      condition.TimeWindow,
		"condition_met":    met,
		"metric_count":     len(metrics),
	}

	return met, message, details, nil
}

// aggregateMetrics 聚合指标
func (ae *AlertEngine) aggregateMetrics(metrics []MetricValue, aggregation string) (interface{}, error) {
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no metrics to aggregate")
	}

	switch strings.ToLower(aggregation) {
	case "avg", "average":
		return ae.calculateAverage(metrics)
	case "sum":
		return ae.calculateSum(metrics)
	case "min":
		return ae.calculateMin(metrics)
	case "max":
		return ae.calculateMax(metrics)
	case "count":
		return len(metrics), nil
	case "last", "":
		return metrics[len(metrics)-1].Value, nil
	default:
		return nil, fmt.Errorf("unsupported aggregation: %s", aggregation)
	}
}

// calculateAverage 计算平均值
func (ae *AlertEngine) calculateAverage(metrics []MetricValue) (float64, error) {
	sum, err := ae.calculateSum(metrics)
	if err != nil {
		return 0, err
	}
	return sum / float64(len(metrics)), nil
}

// calculateSum 计算总和
func (ae *AlertEngine) calculateSum(metrics []MetricValue) (float64, error) {
	var sum float64
	for _, metric := range metrics {
		val, err := ae.convertToFloat64(metric.Value)
		if err != nil {
			return 0, err
		}
		sum += val
	}
	return sum, nil
}

// calculateMin 计算最小值
func (ae *AlertEngine) calculateMin(metrics []MetricValue) (float64, error) {
	if len(metrics) == 0 {
		return 0, fmt.Errorf("no metrics")
	}

	min, err := ae.convertToFloat64(metrics[0].Value)
	if err != nil {
		return 0, err
	}

	for _, metric := range metrics[1:] {
		val, err := ae.convertToFloat64(metric.Value)
		if err != nil {
			return 0, err
		}
		if val < min {
			min = val
		}
	}
	return min, nil
}

// calculateMax 计算最大值
func (ae *AlertEngine) calculateMax(metrics []MetricValue) (float64, error) {
	if len(metrics) == 0 {
		return 0, fmt.Errorf("no metrics")
	}

	max, err := ae.convertToFloat64(metrics[0].Value)
	if err != nil {
		return 0, err
	}

	for _, metric := range metrics[1:] {
		val, err := ae.convertToFloat64(metric.Value)
		if err != nil {
			return 0, err
		}
		if val > max {
			max = val
		}
	}
	return max, nil
}

// convertToFloat64 转换为float64
func (ae *AlertEngine) convertToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// compareValues 比较值
func (ae *AlertEngine) compareValues(actual interface{}, operator ComparisonOperator, threshold interface{}) (bool, error) {
	switch operator {
	case OpEqual:
		return ae.isEqual(actual, threshold), nil
	case OpNotEqual:
		return !ae.isEqual(actual, threshold), nil
	case OpGreaterThan:
		return ae.isGreaterThan(actual, threshold)
	case OpGreaterThanOrEqual:
		return ae.isGreaterThanOrEqual(actual, threshold)
	case OpLessThan:
		return ae.isLessThan(actual, threshold)
	case OpLessThanOrEqual:
		return ae.isLessThanOrEqual(actual, threshold)
	case OpContains:
		return ae.contains(actual, threshold), nil
	case OpNotContains:
		return !ae.contains(actual, threshold), nil
	case OpRegex:
		return ae.matchesRegex(actual, threshold)
	case OpNotRegex:
		matches, err := ae.matchesRegex(actual, threshold)
		return !matches, err
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// isEqual 判断相等
func (ae *AlertEngine) isEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// isGreaterThan 判断大于
func (ae *AlertEngine) isGreaterThan(a, b interface{}) (bool, error) {
	valA, err := ae.convertToFloat64(a)
	if err != nil {
		return false, err
	}
	valB, err := ae.convertToFloat64(b)
	if err != nil {
		return false, err
	}
	return valA > valB, nil
}

// isGreaterThanOrEqual 判断大于等于
func (ae *AlertEngine) isGreaterThanOrEqual(a, b interface{}) (bool, error) {
	valA, err := ae.convertToFloat64(a)
	if err != nil {
		return false, err
	}
	valB, err := ae.convertToFloat64(b)
	if err != nil {
		return false, err
	}
	return valA >= valB, nil
}

// isLessThan 判断小于
func (ae *AlertEngine) isLessThan(a, b interface{}) (bool, error) {
	valA, err := ae.convertToFloat64(a)
	if err != nil {
		return false, err
	}
	valB, err := ae.convertToFloat64(b)
	if err != nil {
		return false, err
	}
	return valA < valB, nil
}

// isLessThanOrEqual 判断小于等于
func (ae *AlertEngine) isLessThanOrEqual(a, b interface{}) (bool, error) {
	valA, err := ae.convertToFloat64(a)
	if err != nil {
		return false, err
	}
	valB, err := ae.convertToFloat64(b)
	if err != nil {
		return false, err
	}
	return valA <= valB, nil
}

// contains 判断包含
func (ae *AlertEngine) contains(a, b interface{}) bool {
	strA := fmt.Sprintf("%v", a)
	strB := fmt.Sprintf("%v", b)
	return strings.Contains(strA, strB)
}

// matchesRegex 判断正则匹配
func (ae *AlertEngine) matchesRegex(value, pattern interface{}) (bool, error) {
	strValue := fmt.Sprintf("%v", value)
	strPattern := fmt.Sprintf("%v", pattern)

	regex, err := regexp.Compile(strPattern)
	if err != nil {
		return false, err
	}

	return regex.MatchString(strValue), nil
}

// generateFingerprint 生成告警指纹
func (ae *AlertEngine) generateFingerprint(rule *AlertRule, labels map[string]string) string {
	// 简化实现，实际应该使用更复杂的哈希算法
	var parts []string
	parts = append(parts, rule.ID)

	// 按键排序标签
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	return strings.Join(parts, "|")
}

// generateAlertID 生成告警ID
func (ae *AlertEngine) generateAlertID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}

// generateGroupKey 生成分组键
func (ae *AlertEngine) generateGroupKey(labels map[string]string) string {
	// 简化实现，实际应该根据配置的分组标签生成
	if service, ok := labels["service"]; ok {
		return service
	}
	if instance, ok := labels["instance"]; ok {
		return instance
	}
	return "default"
}

// isAlertSuppressed 检查告警是否被抑制
func (ae *AlertEngine) isAlertSuppressed(alert *Alert) bool {
	for _, rule := range ae.suppressionRules {
		if !rule.Enabled {
			continue
		}

		// 检查是否匹配抑制规则
		if ae.matchesSuppressionRule(alert, rule) {
			return true
		}
	}
	return false
}

// matchesSuppressionRule 检查是否匹配抑制规则
func (ae *AlertEngine) matchesSuppressionRule(alert *Alert, rule SuppressionRule) bool {
	// 简化实现
	return false
}

// resolveAlert 解决告警
func (ae *AlertEngine) resolveAlert(fingerprint, reason string) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	alert, exists := ae.alerts[fingerprint]
	if !exists || alert.Status != StatusFiring {
		return
	}

	now := time.Now()
	alert.Status = StatusResolved
	alert.ResolvedAt = &now
	alert.Message = reason

	// 移动到历史记录
	ae.alertHistory = append(ae.alertHistory, alert)
	delete(ae.alerts, fingerprint)

	atomic.AddInt64(&ae.stats.FiringAlerts, -1)
	atomic.AddInt64(&ae.stats.ResolvedAlerts, 1)

	// 发送解决通知
	ae.sendResolutionNotification(alert)
}

// sendNotification 发送通知
func (ae *AlertEngine) sendNotification(alert *Alert, notifiers []string) {
	if len(notifiers) == 0 {
		notifiers = ae.config.DefaultNotifiers
	}

	if ae.config.GroupingEnabled {
		// 添加到分组
		ae.addToGroup(alert)
	} else {
		// 立即发送
		ae.sendImmediateNotification(alert, notifiers)
	}
}

// addToGroup 添加到分组
func (ae *AlertEngine) addToGroup(alert *Alert) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	groupKey := alert.GroupKey
	ae.groupedAlerts[groupKey] = append(ae.groupedAlerts[groupKey], alert)
}

// sendImmediateNotification 立即发送通知
func (ae *AlertEngine) sendImmediateNotification(alert *Alert, notifiers []string) {
	for _, notifierName := range notifiers {
		notifier, exists := ae.notifiers[notifierName]
		if !exists {
			continue
		}

		go func(n Notifier, a *Alert) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := n.Send(ctx, *a)
			status := NotificationStatus{
				SentAt: time.Now(),
			}

			if err != nil {
				status.Status = "failed"
				status.Error = err.Error()
			} else {
				status.Status = "sent"
			}

			ae.mu.Lock()
			a.NotificationStatus[n.Name()] = status
			ae.mu.Unlock()
		}(notifier, alert)
	}
}

// sendResolutionNotification 发送解决通知
func (ae *AlertEngine) sendResolutionNotification(alert *Alert) {
	// 实现解决通知逻辑
}

// notificationLoop 通知循环
func (ae *AlertEngine) notificationLoop(ctx context.Context) {
	groupTicker := time.NewTicker(ae.config.GroupInterval)
	defer groupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-groupTicker.C:
			ae.processGroupedAlerts()
		}
	}
}

// processGroupedAlerts 处理分组告警
func (ae *AlertEngine) processGroupedAlerts() {
	ae.mu.Lock()
	groups := make(map[string][]*Alert)
	for k, v := range ae.groupedAlerts {
		groups[k] = v
		delete(ae.groupedAlerts, k)
	}
	ae.mu.Unlock()

	for groupKey, alerts := range groups {
		if len(alerts) > 0 {
			ae.sendGroupedNotification(groupKey, alerts)
		}
	}
}

// sendGroupedNotification 发送分组通知
func (ae *AlertEngine) sendGroupedNotification(groupKey string, alerts []*Alert) {
	// 实现分组通知逻辑
}

// cleanupLoop 清理循环
func (ae *AlertEngine) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ae.cleanupHistory()
		}
	}
}

// cleanupHistory 清理历史记录
func (ae *AlertEngine) cleanupHistory() {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	cutoff := time.Now().Add(-ae.config.AlertHistoryRetention)
	var validHistory []*Alert

	for _, alert := range ae.alertHistory {
		if alert.FiredAt.After(cutoff) {
			validHistory = append(validHistory, alert)
		}
	}

	ae.alertHistory = validHistory
}

// updateEvaluationStats 更新评估统计
func (ae *AlertEngine) updateEvaluationStats(duration time.Duration) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.stats.LastEvaluationTime = time.Now()

	// 更新平均评估时间
	totalEvaluations := atomic.LoadInt64(&ae.stats.TotalEvaluations)
	currentAvg := ae.stats.AverageEvaluationTime
	newAvg := (currentAvg*float64(totalEvaluations-1) + float64(duration.Nanoseconds())/1e6) / float64(totalEvaluations)
	ae.stats.AverageEvaluationTime = newAvg
}

// GetStats 获取统计信息
func (ae *AlertEngine) GetStats() *AlertEngineStats {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	return &AlertEngineStats{
		TotalRules:            atomic.LoadInt64(&ae.stats.TotalRules),
		ActiveRules:           atomic.LoadInt64(&ae.stats.ActiveRules),
		TotalAlerts:           atomic.LoadInt64(&ae.stats.TotalAlerts),
		FiringAlerts:          atomic.LoadInt64(&ae.stats.FiringAlerts),
		ResolvedAlerts:        atomic.LoadInt64(&ae.stats.ResolvedAlerts),
		SuppressedAlerts:      atomic.LoadInt64(&ae.stats.SuppressedAlerts),
		TotalEvaluations:      atomic.LoadInt64(&ae.stats.TotalEvaluations),
		FailedEvaluations:     atomic.LoadInt64(&ae.stats.FailedEvaluations),
		LastEvaluationTime:    ae.stats.LastEvaluationTime,
		AverageEvaluationTime: ae.stats.AverageEvaluationTime,
	}
}

// GetAlerts 获取告警列表
func (ae *AlertEngine) GetAlerts(status AlertStatus) []*Alert {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	var alerts []*Alert
	for _, alert := range ae.alerts {
		if status == "" || alert.Status == status {
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// GetRules 获取规则列表
func (ae *AlertEngine) GetRules() []*AlertRule {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(ae.rules))
	for _, rule := range ae.rules {
		rules = append(rules, rule)
	}

	return rules
}

// AcknowledgeAlert 确认告警
func (ae *AlertEngine) AcknowledgeAlert(alertID, user string) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	for _, alert := range ae.alerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Status = StatusAcknowledged
			alert.AcknowledgedAt = &now
			alert.AcknowledgedBy = user
			return nil
		}
	}

	return fmt.Errorf("alert not found: %s", alertID)
}

// 内置通知器实现

// ConsoleNotifier 控制台通知器
type ConsoleNotifier struct{}

// Name 返回通知器名称
func (c *ConsoleNotifier) Name() string {
	return "console"
}

// Send 发送通知
func (c *ConsoleNotifier) Send(ctx context.Context, alert Alert) error {
	alertJSON, _ := json.MarshalIndent(alert, "", "  ")
	fmt.Printf("[ALERT] %s\n%s\n", alert.Level, string(alertJSON))
	return nil
}

// NewConsoleNotifier 创建控制台通知器
func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{}
}