// Package deployment 蓝绿部署实现
// Author: NetCore-Go Team
// Created: 2024

package deployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/phuhao00/netcore-go/pkg/health"
)

// DeploymentColor 部署颜色
type DeploymentColor string

const (
	Blue  DeploymentColor = "blue"
	Green DeploymentColor = "green"
)

func (d DeploymentColor) String() string {
	return string(d)
}

// Other 返回另一种颜色
func (d DeploymentColor) Other() DeploymentColor {
	if d == Blue {
		return Green
	}
	return Blue
}

// DeploymentStatus 部署状态
type DeploymentStatus string

const (
	StatusIdle       DeploymentStatus = "idle"
	StatusDeploying  DeploymentStatus = "deploying"
	StatusTesting    DeploymentStatus = "testing"
	StatusSwitching  DeploymentStatus = "switching"
	StatusCompleted  DeploymentStatus = "completed"
	StatusFailed     DeploymentStatus = "failed"
	StatusRollingBack DeploymentStatus = "rolling_back"
)

// BlueGreenManager 蓝绿部署管理器
type BlueGreenManager struct {
	mu             sync.RWMutex
	config         *BlueGreenConfig
	active         DeploymentColor
	status         DeploymentStatus
	deployments    map[DeploymentColor]*Deployment
	loadBalancer   LoadBalancer
	healthChecker  health.HealthChecker
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
	stats          *DeploymentStats
	history        []*DeploymentRecord
}

// BlueGreenConfig 蓝绿部署配置
type BlueGreenConfig struct {
	// 基础配置
	ServiceName     string        `json:"service_name"`
	Namespace       string        `json:"namespace"`
	InitialColor    DeploymentColor `json:"initial_color"`

	// 部署配置
	DeployTimeout   time.Duration `json:"deploy_timeout"`
	HealthTimeout   time.Duration `json:"health_timeout"`
	SwitchTimeout   time.Duration `json:"switch_timeout"`
	RollbackTimeout time.Duration `json:"rollback_timeout"`

	// 健康检查配置
	HealthCheckPath     string        `json:"health_check_path"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckRetries  int           `json:"health_check_retries"`
	MinHealthyInstances int           `json:"min_healthy_instances"`

	// 流量切换配置
	TrafficSwitchMode   TrafficSwitchMode `json:"traffic_switch_mode"`
	CanaryPercentage    int               `json:"canary_percentage"`
	CanaryDuration      time.Duration     `json:"canary_duration"`
	GradualSwitchSteps  []int             `json:"gradual_switch_steps"`
	GradualSwitchDelay  time.Duration     `json:"gradual_switch_delay"`

	// 自动回滚配置
	AutoRollback        bool          `json:"auto_rollback"`
	RollbackThreshold   float64       `json:"rollback_threshold"`
	RollbackMetrics     []string      `json:"rollback_metrics"`
	RollbackCheckPeriod time.Duration `json:"rollback_check_period"`

	// 通知配置
	Notifications       *NotificationConfig `json:"notifications"`

	// 钩子配置
	PreDeployHooks      []string `json:"pre_deploy_hooks"`
	PostDeployHooks     []string `json:"post_deploy_hooks"`
	PreSwitchHooks      []string `json:"pre_switch_hooks"`
	PostSwitchHooks     []string `json:"post_switch_hooks"`
	RollbackHooks       []string `json:"rollback_hooks"`
}

// TrafficSwitchMode 流量切换模式
type TrafficSwitchMode string

const (
	SwitchModeInstant   TrafficSwitchMode = "instant"
	SwitchModeCanary    TrafficSwitchMode = "canary"
	SwitchModeGradual   TrafficSwitchMode = "gradual"
)

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled   bool     `json:"enabled"`
	Webhooks  []string `json:"webhooks"`
	Slack     *SlackConfig `json:"slack"`
	Email     *EmailConfig `json:"email"`
}

// SlackConfig Slack配置
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel"`
	Username   string `json:"username"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	From         string   `json:"from"`
	To           []string `json:"to"`
}

// Deployment 部署实例
type Deployment struct {
	Color       DeploymentColor `json:"color"`
	Version     string          `json:"version"`
	Image       string          `json:"image"`
	Replicas    int             `json:"replicas"`
	Healthy     bool            `json:"healthy"`
	Endpoints   []string        `json:"endpoints"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	SwitchTraffic(from, to DeploymentColor, percentage int) error
	GetTrafficDistribution() (map[DeploymentColor]int, error)
	HealthCheck(color DeploymentColor) (bool, error)
}

// DeploymentStats 部署统计
type DeploymentStats struct {
	TotalDeployments    int64         `json:"total_deployments"`
	SuccessfulDeployments int64       `json:"successful_deployments"`
	FailedDeployments   int64         `json:"failed_deployments"`
	Rollbacks           int64         `json:"rollbacks"`
	AverageDeployTime   time.Duration `json:"average_deploy_time"`
	LastDeployment      time.Time     `json:"last_deployment"`
	Uptime              time.Duration `json:"uptime"`
}

// DeploymentRecord 部署记录
type DeploymentRecord struct {
	ID          string            `json:"id"`
	Color       DeploymentColor   `json:"color"`
	Version     string            `json:"version"`
	Image       string            `json:"image"`
	Status      DeploymentStatus  `json:"status"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	Duration    time.Duration     `json:"duration"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

// DefaultBlueGreenConfig 返回默认蓝绿部署配置
func DefaultBlueGreenConfig() *BlueGreenConfig {
	return &BlueGreenConfig{
		ServiceName:         "netcore-go",
		Namespace:           "default",
		InitialColor:        Blue,
		DeployTimeout:       10 * time.Minute,
		HealthTimeout:       5 * time.Minute,
		SwitchTimeout:       2 * time.Minute,
		RollbackTimeout:     5 * time.Minute,
		HealthCheckPath:     "/health",
		HealthCheckInterval: 10 * time.Second,
		HealthCheckRetries:  3,
		MinHealthyInstances: 1,
		TrafficSwitchMode:   SwitchModeInstant,
		CanaryPercentage:    10,
		CanaryDuration:      5 * time.Minute,
		GradualSwitchSteps:  []int{10, 25, 50, 75, 100},
		GradualSwitchDelay:  1 * time.Minute,
		AutoRollback:        true,
		RollbackThreshold:   0.95,
		RollbackMetrics:     []string{"error_rate", "response_time"},
		RollbackCheckPeriod: 30 * time.Second,
		Notifications: &NotificationConfig{
			Enabled: false,
		},
	}
}

// NewBlueGreenManager 创建蓝绿部署管理器
func NewBlueGreenManager(config *BlueGreenConfig, lb LoadBalancer) *BlueGreenManager {
	if config == nil {
		config = DefaultBlueGreenConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &BlueGreenManager{
		config:       config,
		active:       config.InitialColor,
		status:       StatusIdle,
		deployments:  make(map[DeploymentColor]*Deployment),
		loadBalancer: lb,
		ctx:          ctx,
		cancel:       cancel,
		stats:        &DeploymentStats{},
		history:      make([]*DeploymentRecord, 0),
	}
}

// Start 启动蓝绿部署管理器
func (bg *BlueGreenManager) Start() error {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	if bg.running {
		return fmt.Errorf("blue-green manager is already running")
	}

	bg.running = true

	// 初始化部署
	bg.deployments[bg.active] = &Deployment{
		Color:     bg.active,
		Version:   "initial",
		Healthy:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 启动监控
	if bg.config.AutoRollback {
		go bg.monitorDeployment()
	}

	return nil
}

// Stop 停止蓝绿部署管理器
func (bg *BlueGreenManager) Stop() error {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	if !bg.running {
		return nil
	}

	bg.running = false
	bg.cancel()

	return nil
}

// Deploy 执行蓝绿部署
func (bg *BlueGreenManager) Deploy(ctx context.Context, version, image string, metadata map[string]string) error {
	bg.mu.Lock()
	if bg.status != StatusIdle {
		bg.mu.Unlock()
		return fmt.Errorf("deployment already in progress: %s", bg.status)
	}
	bg.status = StatusDeploying
	bg.mu.Unlock()

	// 创建部署记录
	record := &DeploymentRecord{
		ID:        fmt.Sprintf("%s-%d", version, time.Now().Unix()),
		Color:     bg.active.Other(),
		Version:   version,
		Image:     image,
		Status:    StatusDeploying,
		StartTime: time.Now(),
		Metadata:  metadata,
	}

	defer func() {
		record.EndTime = time.Now()
		record.Duration = record.EndTime.Sub(record.StartTime)
		bg.addDeploymentRecord(record)
		bg.updateStats(record)
	}()

	// 执行部署步骤
	if err := bg.executeDeployment(ctx, record); err != nil {
		record.Status = StatusFailed
		record.Error = err.Error()
		bg.setStatus(StatusFailed)
		
		// 发送失败通知
		bg.sendNotification("Deployment Failed", fmt.Sprintf("Deployment %s failed: %v", record.ID, err))
		
		return err
	}

	record.Status = StatusCompleted
	bg.setStatus(StatusCompleted)

	// 发送成功通知
	bg.sendNotification("Deployment Completed", fmt.Sprintf("Deployment %s completed successfully", record.ID))

	return nil
}

// executeDeployment 执行部署
func (bg *BlueGreenManager) executeDeployment(ctx context.Context, record *DeploymentRecord) error {
	// 创建超时上下文
	deployCtx, cancel := context.WithTimeout(ctx, bg.config.DeployTimeout)
	defer cancel()

	// 1. 执行预部署钩子
	if err := bg.executeHooks(deployCtx, bg.config.PreDeployHooks, "pre-deploy"); err != nil {
		return fmt.Errorf("pre-deploy hooks failed: %w", err)
	}

	// 2. 部署到非活跃环境
	inactiveColor := bg.active.Other()
	if err := bg.deployToEnvironment(deployCtx, inactiveColor, record); err != nil {
		return fmt.Errorf("deployment to %s environment failed: %w", inactiveColor, err)
	}

	// 3. 健康检查
	bg.setStatus(StatusTesting)
	if err := bg.waitForHealthy(deployCtx, inactiveColor); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// 4. 执行后部署钩子
	if err := bg.executeHooks(deployCtx, bg.config.PostDeployHooks, "post-deploy"); err != nil {
		return fmt.Errorf("post-deploy hooks failed: %w", err)
	}

	// 5. 流量切换
	bg.setStatus(StatusSwitching)
	if err := bg.switchTraffic(deployCtx, inactiveColor); err != nil {
		return fmt.Errorf("traffic switch failed: %w", err)
	}

	// 6. 更新活跃环境
	bg.mu.Lock()
	bg.active = inactiveColor
	bg.status = StatusIdle
	bg.mu.Unlock()

	return nil
}

// deployToEnvironment 部署到指定环境
func (bg *BlueGreenManager) deployToEnvironment(ctx context.Context, color DeploymentColor, record *DeploymentRecord) error {
	// 创建新部署
	deployment := &Deployment{
		Color:     color,
		Version:   record.Version,
		Image:     record.Image,
		Replicas:  bg.config.MinHealthyInstances,
		Healthy:   false,
		Metadata:  record.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 这里应该实现实际的部署逻辑
	// 例如：创建Kubernetes Deployment、Service等资源
	fmt.Printf("Deploying %s:%s to %s environment...\n", record.Image, record.Version, color)

	// 模拟部署过程
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second): // 模拟部署时间
		// 部署完成
	}

	bg.mu.Lock()
	bg.deployments[color] = deployment
	bg.mu.Unlock()

	return nil
}

// waitForHealthy 等待环境健康
func (bg *BlueGreenManager) waitForHealthy(ctx context.Context, color DeploymentColor) error {
	healthCtx, cancel := context.WithTimeout(ctx, bg.config.HealthTimeout)
	defer cancel()

	ticker := time.NewTicker(bg.config.HealthCheckInterval)
	defer ticker.Stop()

	retries := 0
	for {
		select {
		case <-healthCtx.Done():
			return healthCtx.Err()
		case <-ticker.C:
			healthy, err := bg.loadBalancer.HealthCheck(color)
			if err != nil {
				retries++
				if retries >= bg.config.HealthCheckRetries {
					return fmt.Errorf("health check failed after %d retries: %w", retries, err)
				}
				continue
			}

			if healthy {
				bg.mu.Lock()
				if deployment, exists := bg.deployments[color]; exists {
					deployment.Healthy = true
					deployment.UpdatedAt = time.Now()
				}
				bg.mu.Unlock()
				return nil
			}

			retries++
			if retries >= bg.config.HealthCheckRetries {
				return fmt.Errorf("environment %s is not healthy after %d checks", color, retries)
			}
		}
	}
}

// switchTraffic 切换流量
func (bg *BlueGreenManager) switchTraffic(ctx context.Context, to DeploymentColor) error {
	switchCtx, cancel := context.WithTimeout(ctx, bg.config.SwitchTimeout)
	defer cancel()

	// 执行预切换钩子
	if err := bg.executeHooks(switchCtx, bg.config.PreSwitchHooks, "pre-switch"); err != nil {
		return fmt.Errorf("pre-switch hooks failed: %w", err)
	}

	switch bg.config.TrafficSwitchMode {
	case SwitchModeInstant:
		err := bg.loadBalancer.SwitchTraffic(bg.active, to, 100)
		if err != nil {
			return fmt.Errorf("instant traffic switch failed: %w", err)
		}

	case SwitchModeCanary:
		if err := bg.canarySwitch(switchCtx, to); err != nil {
			return fmt.Errorf("canary switch failed: %w", err)
		}

	case SwitchModeGradual:
		if err := bg.gradualSwitch(switchCtx, to); err != nil {
			return fmt.Errorf("gradual switch failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown traffic switch mode: %s", bg.config.TrafficSwitchMode)
	}

	// 执行后切换钩子
	if err := bg.executeHooks(switchCtx, bg.config.PostSwitchHooks, "post-switch"); err != nil {
		return fmt.Errorf("post-switch hooks failed: %w", err)
	}

	return nil
}

// canarySwitch 金丝雀切换
func (bg *BlueGreenManager) canarySwitch(ctx context.Context, to DeploymentColor) error {
	// 先切换少量流量
	if err := bg.loadBalancer.SwitchTraffic(bg.active, to, bg.config.CanaryPercentage); err != nil {
		return err
	}

	// 等待金丝雀期间
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(bg.config.CanaryDuration):
		// 金丝雀期间结束，检查指标
		if bg.config.AutoRollback {
			if shouldRollback, err := bg.checkRollbackConditions(to); err != nil {
				return fmt.Errorf("rollback check failed: %w", err)
			} else if shouldRollback {
				return fmt.Errorf("canary metrics indicate rollback needed")
			}
		}
	}

	// 切换全部流量
	return bg.loadBalancer.SwitchTraffic(bg.active, to, 100)
}

// gradualSwitch 渐进式切换
func (bg *BlueGreenManager) gradualSwitch(ctx context.Context, to DeploymentColor) error {
	for _, percentage := range bg.config.GradualSwitchSteps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := bg.loadBalancer.SwitchTraffic(bg.active, to, percentage); err != nil {
			return fmt.Errorf("failed to switch %d%% traffic: %w", percentage, err)
		}

		// 等待观察期
		if percentage < 100 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(bg.config.GradualSwitchDelay):
				// 检查是否需要回滚
				if bg.config.AutoRollback {
					if shouldRollback, err := bg.checkRollbackConditions(to); err != nil {
						return fmt.Errorf("rollback check failed: %w", err)
					} else if shouldRollback {
						return fmt.Errorf("metrics indicate rollback needed at %d%%", percentage)
					}
				}
			}
		}
	}

	return nil
}

// Rollback 回滚部署
func (bg *BlueGreenManager) Rollback(ctx context.Context) error {
	bg.mu.Lock()
	if bg.status != StatusIdle && bg.status != StatusFailed {
		bg.mu.Unlock()
		return fmt.Errorf("cannot rollback during %s", bg.status)
	}
	bg.status = StatusRollingBack
	bg.mu.Unlock()

	defer func() {
		bg.setStatus(StatusIdle)
	}()

	rollbackCtx, cancel := context.WithTimeout(ctx, bg.config.RollbackTimeout)
	defer cancel()

	// 执行回滚钩子
	if err := bg.executeHooks(rollbackCtx, bg.config.RollbackHooks, "rollback"); err != nil {
		return fmt.Errorf("rollback hooks failed: %w", err)
	}

	// 切换回原环境
	previousColor := bg.active.Other()
	if err := bg.loadBalancer.SwitchTraffic(bg.active, previousColor, 100); err != nil {
		return fmt.Errorf("rollback traffic switch failed: %w", err)
	}

	bg.mu.Lock()
	bg.active = previousColor
	bg.stats.Rollbacks++
	bg.mu.Unlock()

	// 发送回滚通知
	bg.sendNotification("Rollback Completed", "Deployment has been rolled back successfully")

	return nil
}

// executeHooks 执行钩子
func (bg *BlueGreenManager) executeHooks(ctx context.Context, hooks []string, phase string) error {
	for _, hook := range hooks {
		fmt.Printf("Executing %s hook: %s\n", phase, hook)
		// 这里应该实现实际的钩子执行逻辑
		// 例如：执行脚本、调用API等
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second): // 模拟钩子执行时间
			// 钩子执行完成
		}
	}
	return nil
}

// checkRollbackConditions 检查回滚条件
func (bg *BlueGreenManager) checkRollbackConditions(color DeploymentColor) (bool, error) {
	// 这里应该实现实际的指标检查逻辑
	// 例如：检查错误率、响应时间等指标
	fmt.Printf("Checking rollback conditions for %s environment...\n", color)
	
	// 模拟指标检查
	// 在实际实现中，这里会查询监控系统的指标
	errorRate := 0.02 // 2% 错误率
	threshold := 1.0 - bg.config.RollbackThreshold // 5% 阈值
	
	return errorRate > threshold, nil
}

// monitorDeployment 监控部署
func (bg *BlueGreenManager) monitorDeployment() {
	ticker := time.NewTicker(bg.config.RollbackCheckPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-bg.ctx.Done():
			return
		case <-ticker.C:
			bg.mu.RLock()
			active := bg.active
			status := bg.status
			bg.mu.RUnlock()

			if status == StatusIdle && bg.config.AutoRollback {
				if shouldRollback, err := bg.checkRollbackConditions(active); err != nil {
					fmt.Printf("Rollback check error: %v\n", err)
				} else if shouldRollback {
					fmt.Println("Auto-rollback triggered")
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), bg.config.RollbackTimeout)
						defer cancel()
						if err := bg.Rollback(ctx); err != nil {
							fmt.Printf("Auto-rollback failed: %v\n", err)
						}
					}()
				}
			}
		}
	}
}

// sendNotification 发送通知
func (bg *BlueGreenManager) sendNotification(title, message string) {
	if !bg.config.Notifications.Enabled {
		return
	}

	// 这里应该实现实际的通知发送逻辑
	fmt.Printf("Notification: %s - %s\n", title, message)
}

// 辅助方法

func (bg *BlueGreenManager) setStatus(status DeploymentStatus) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.status = status
}

func (bg *BlueGreenManager) addDeploymentRecord(record *DeploymentRecord) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.history = append(bg.history, record)
	
	// 保持历史记录数量限制
	if len(bg.history) > 100 {
		bg.history = bg.history[1:]
	}
}

func (bg *BlueGreenManager) updateStats(record *DeploymentRecord) {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	bg.stats.TotalDeployments++
	bg.stats.LastDeployment = record.StartTime

	if record.Status == StatusCompleted {
		bg.stats.SuccessfulDeployments++
	} else {
		bg.stats.FailedDeployments++
	}

	// 更新平均部署时间
	if bg.stats.TotalDeployments == 1 {
		bg.stats.AverageDeployTime = record.Duration
	} else {
		bg.stats.AverageDeployTime = (bg.stats.AverageDeployTime + record.Duration) / 2
	}
}

// 公共方法

// GetStatus 获取当前状态
func (bg *BlueGreenManager) GetStatus() DeploymentStatus {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.status
}

// GetActiveColor 获取当前活跃颜色
func (bg *BlueGreenManager) GetActiveColor() DeploymentColor {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.active
}

// GetDeployments 获取部署信息
func (bg *BlueGreenManager) GetDeployments() map[DeploymentColor]*Deployment {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	deployments := make(map[DeploymentColor]*Deployment)
	for color, deployment := range bg.deployments {
		deployments[color] = deployment
	}
	return deployments
}

// GetStats 获取统计信息
func (bg *BlueGreenManager) GetStats() *DeploymentStats {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	stats := *bg.stats
	return &stats
}

// GetHistory 获取部署历史
func (bg *BlueGreenManager) GetHistory() []*DeploymentRecord {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	history := make([]*DeploymentRecord, len(bg.history))
	copy(history, bg.history)
	return history
}

// IsRunning 检查是否运行
func (bg *BlueGreenManager) IsRunning() bool {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.running
}