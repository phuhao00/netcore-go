// Package dev 交互式配置向导
// Author: NetCore-Go Team
// Created: 2024

package dev

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ConfigWizard 配置向导
type ConfigWizard struct {
	reader   *bufio.Reader
	projectDir string
	config   *ProjectWizardConfig
	verbose  bool
}

// ProjectWizardConfig 项目向导配置
type ProjectWizardConfig struct {
	// 基本信息
	ProjectName   string `json:"project_name"`
	Description   string `json:"description"`
	Author        string `json:"author"`
	Email         string `json:"email"`
	Version       string `json:"version"`
	License       string `json:"license"`
	GoVersion     string `json:"go_version"`
	ModulePath    string `json:"module_path"`

	// 项目类型
	ProjectType   string   `json:"project_type"`
	Features      []string `json:"features"`
	Architecture  string   `json:"architecture"`

	// 数据库配置
	Database      *DatabaseConfig `json:"database,omitempty"`

	// 缓存配置
	Cache         *CacheConfig    `json:"cache,omitempty"`

	// 认证配置
	Auth          *AuthConfig     `json:"auth,omitempty"`

	// 中间件配置
	Middleware    []string        `json:"middleware"`

	// 部署配置
	Deployment    *DeploymentConfig `json:"deployment,omitempty"`

	// 开发配置
	Development   *DevelopmentConfig `json:"development,omitempty"`

	// 测试配置
	Testing       *TestingConfig   `json:"testing,omitempty"`

	// 监控配置
	Monitoring    *MonitoringConfig `json:"monitoring,omitempty"`

	// 创建时间
	CreatedAt     time.Time       `json:"created_at"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type         string            `json:"type"`
	Host         string            `json:"host,omitempty"`
	Port         int               `json:"port,omitempty"`
	Username     string            `json:"username,omitempty"`
	Password     string            `json:"password,omitempty"`
	Database     string            `json:"database,omitempty"`
	SSLMode      string            `json:"ssl_mode,omitempty"`
	Path         string            `json:"path,omitempty"`
	URI          string            `json:"uri,omitempty"`
	Migrations   bool              `json:"migrations"`
	Seeding      bool              `json:"seeding"`
	Pooling      bool              `json:"pooling"`
	Options      map[string]string `json:"options,omitempty"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Type         string            `json:"type"`
	Host         string            `json:"host,omitempty"`
	Port         int               `json:"port,omitempty"`
	Password     string            `json:"password,omitempty"`
	Database     int               `json:"database,omitempty"`
	Servers      []string          `json:"servers,omitempty"`
	TTL          time.Duration     `json:"ttl"`
	Compression  bool              `json:"compression"`
	Serialization string           `json:"serialization"`
	Options      map[string]string `json:"options,omitempty"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type         string            `json:"type"`
	Secret       string            `json:"secret,omitempty"`
	Expiration   time.Duration     `json:"expiration,omitempty"`
	Issuer       string            `json:"issuer,omitempty"`
	ClientID     string            `json:"client_id,omitempty"`
	ClientSecret string            `json:"client_secret,omitempty"`
	RedirectURL  string            `json:"redirect_url,omitempty"`
	Scopes       []string          `json:"scopes,omitempty"`
	Providers    []string          `json:"providers,omitempty"`
	Options      map[string]string `json:"options,omitempty"`
}

// DeploymentConfig 部署配置
type DeploymentConfig struct {
	Type         string            `json:"type"`
	Registry     string            `json:"registry,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Environments []string          `json:"environments"`
	CI           bool              `json:"ci"`
	CD           bool              `json:"cd"`
	Helm         bool              `json:"helm"`
	Terraform    bool              `json:"terraform"`
	Options      map[string]string `json:"options,omitempty"`
}

// DevelopmentConfig 开发配置
type DevelopmentConfig struct {
	HotReload    bool              `json:"hot_reload"`
	DebugMode    bool              `json:"debug_mode"`
	LiveReload   bool              `json:"live_reload"`
	MockData     bool              `json:"mock_data"`
	APIDoc       bool              `json:"api_doc"`
	CodeGen      bool              `json:"code_gen"`
	Linting      bool              `json:"linting"`
	Formatting   bool              `json:"formatting"`
	Options      map[string]string `json:"options,omitempty"`
}

// TestingConfig 测试配置
type TestingConfig struct {
	Unit         bool              `json:"unit"`
	Integration  bool              `json:"integration"`
	E2E          bool              `json:"e2e"`
	Load         bool              `json:"load"`
	Chaos        bool              `json:"chaos"`
	Coverage     bool              `json:"coverage"`
	Mocking      bool              `json:"mocking"`
	Fixtures     bool              `json:"fixtures"`
	Options      map[string]string `json:"options,omitempty"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Metrics      bool              `json:"metrics"`
	Tracing      bool              `json:"tracing"`
	Logging      bool              `json:"logging"`
	HealthCheck  bool              `json:"health_check"`
	Alerting     bool              `json:"alerting"`
	Dashboard    bool              `json:"dashboard"`
	Prometheus   bool              `json:"prometheus"`
	Jaeger       bool              `json:"jaeger"`
	Grafana      bool              `json:"grafana"`
	Options      map[string]string `json:"options,omitempty"`
}

// WizardStep 向导步骤
type WizardStep struct {
	Name        string
	Description string
	Handler     func(*ConfigWizard) error
	Skippable   bool
}

// NewConfigWizard 创建配置向导
func NewConfigWizard(projectDir string) *ConfigWizard {
	return &ConfigWizard{
		reader:     bufio.NewReader(os.Stdin),
		projectDir: projectDir,
		config: &ProjectWizardConfig{
			CreatedAt: time.Now(),
		},
	}
}

// Run 运行配置向导
func (w *ConfigWizard) Run() (*ProjectWizardConfig, error) {
	fmt.Println("🧙 Welcome to NetCore-Go Configuration Wizard!")
	fmt.Println("This wizard will help you set up your project configuration.")
	fmt.Println()

	steps := []WizardStep{
		{"Basic Information", "Configure basic project information", w.configureBasicInfo, false},
		{"Project Type", "Select project type and features", w.configureProjectType, false},
		{"Database", "Configure database settings", w.configureDatabase, true},
		{"Cache", "Configure caching settings", w.configureCache, true},
		{"Authentication", "Configure authentication settings", w.configureAuth, true},
		{"Middleware", "Select middleware components", w.configureMiddleware, true},
		{"Deployment", "Configure deployment settings", w.configureDeployment, true},
		{"Development", "Configure development settings", w.configureDevelopment, true},
		{"Testing", "Configure testing settings", w.configureTesting, true},
		{"Monitoring", "Configure monitoring settings", w.configureMonitoring, true},
	}

	for i, step := range steps {
		fmt.Printf("📋 Step %d/%d: %s\n", i+1, len(steps), step.Name)
		fmt.Printf("   %s\n", step.Description)
		fmt.Println()

		if step.Skippable {
			if skip := w.askYesNo(fmt.Sprintf("Do you want to configure %s?", step.Name), false); !skip {
				fmt.Println("⏭️  Skipped.")
				fmt.Println()
				continue
			}
		}

		if err := step.Handler(w); err != nil {
			return nil, fmt.Errorf("step %s failed: %w", step.Name, err)
		}

		fmt.Println("✅ Step completed.")
		fmt.Println()
	}

	// 显示配置摘要
	w.showSummary()

	// 确认配置
	if !w.askYesNo("Do you want to save this configuration?", true) {
		fmt.Println("❌ Configuration cancelled.")
		return nil, fmt.Errorf("configuration cancelled by user")
	}

	// 保存配置
	if err := w.saveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("🎉 Configuration completed successfully!")
	return w.config, nil
}

// configureBasicInfo 配置基本信息
func (w *ConfigWizard) configureBasicInfo() error {
	w.config.ProjectName = w.askString("Project name", "my-netcore-app")
	w.config.Description = w.askString("Project description", "A NetCore-Go application")
	w.config.Author = w.askString("Author name", "Developer")
	w.config.Email = w.askString("Author email", "developer@example.com")
	w.config.Version = w.askString("Initial version", "0.1.0")
	w.config.License = w.askChoice("License", []string{"MIT", "Apache-2.0", "GPL-3.0", "BSD-3-Clause", "Unlicense"}, "MIT")
	w.config.GoVersion = w.askString("Go version", "1.21")

	defaultModule := fmt.Sprintf("github.com/%s/%s", strings.ToLower(w.config.Author), w.config.ProjectName)
	w.config.ModulePath = w.askString("Go module path", defaultModule)

	return nil
}

// configureProjectType 配置项目类型
func (w *ConfigWizard) configureProjectType() error {
	projectTypes := []string{"web-api", "microservice", "cli", "library", "full-stack"}
	w.config.ProjectType = w.askChoice("Project type", projectTypes, "web-api")

	architectures := []string{"monolith", "microservices", "serverless", "event-driven"}
	w.config.Architecture = w.askChoice("Architecture pattern", architectures, "monolith")

	availableFeatures := []string{
		"http2", "grpc", "websocket", "graphql",
		"metrics", "tracing", "logging", "health-check",
		"validation", "serialization", "compression",
		"rate-limiting", "cors", "swagger",
		"file-upload", "email", "notifications",
	}

	w.config.Features = w.askMultiChoice("Select features (comma-separated)", availableFeatures)

	return nil
}

// configureDatabase 配置数据库
func (w *ConfigWizard) configureDatabase() error {
	databaseTypes := []string{"postgres", "mysql", "sqlite", "mongodb", "redis"}
	dbType := w.askChoice("Database type", databaseTypes, "postgres")

	w.config.Database = &DatabaseConfig{
		Type: dbType,
	}

	switch dbType {
	case "postgres":
		w.config.Database.Host = w.askString("Host", "localhost")
		w.config.Database.Port = w.askInt("Port", 5432)
		w.config.Database.Username = w.askString("Username", "postgres")
		w.config.Database.Password = w.askString("Password", "password")
		w.config.Database.Database = w.askString("Database name", w.config.ProjectName)
		w.config.Database.SSLMode = w.askChoice("SSL mode", []string{"disable", "require", "verify-full"}, "disable")

	case "mysql":
		w.config.Database.Host = w.askString("Host", "localhost")
		w.config.Database.Port = w.askInt("Port", 3306)
		w.config.Database.Username = w.askString("Username", "root")
		w.config.Database.Password = w.askString("Password", "password")
		w.config.Database.Database = w.askString("Database name", w.config.ProjectName)

	case "sqlite":
		w.config.Database.Path = w.askString("Database file path", "data/app.db")

	case "mongodb":
		w.config.Database.URI = w.askString("MongoDB URI", "mongodb://localhost:27017")
		w.config.Database.Database = w.askString("Database name", w.config.ProjectName)

	case "redis":
		w.config.Database.Host = w.askString("Host", "localhost")
		w.config.Database.Port = w.askInt("Port", 6379)
		w.config.Database.Password = w.askString("Password (optional)", "")
		w.config.Database.Database = w.askInt("Database number", 0)
	}

	w.config.Database.Migrations = w.askYesNo("Enable database migrations?", true)
	w.config.Database.Seeding = w.askYesNo("Enable database seeding?", false)
	w.config.Database.Pooling = w.askYesNo("Enable connection pooling?", true)

	return nil
}

// configureCache 配置缓存
func (w *ConfigWizard) configureCache() error {
	cacheTypes := []string{"redis", "memcached", "in-memory"}
	cacheType := w.askChoice("Cache type", cacheTypes, "redis")

	w.config.Cache = &CacheConfig{
		Type: cacheType,
		TTL:  1 * time.Hour,
	}

	switch cacheType {
	case "redis":
		w.config.Cache.Host = w.askString("Host", "localhost")
		w.config.Cache.Port = w.askInt("Port", 6379)
		w.config.Cache.Password = w.askString("Password (optional)", "")
		w.config.Cache.Database = w.askInt("Database number", 0)

	case "memcached":
		servers := w.askString("Servers (comma-separated)", "localhost:11211")
		w.config.Cache.Servers = strings.Split(servers, ",")
		for i, server := range w.config.Cache.Servers {
			w.config.Cache.Servers[i] = strings.TrimSpace(server)
		}
	}

	ttlHours := w.askInt("Default TTL (hours)", 1)
	w.config.Cache.TTL = time.Duration(ttlHours) * time.Hour
	w.config.Cache.Compression = w.askYesNo("Enable compression?", false)
	w.config.Cache.Serialization = w.askChoice("Serialization format", []string{"json", "msgpack", "gob"}, "json")

	return nil
}

// configureAuth 配置认证
func (w *ConfigWizard) configureAuth() error {
	authTypes := []string{"jwt", "oauth2", "basic", "api-key", "session"}
	authType := w.askChoice("Authentication type", authTypes, "jwt")

	w.config.Auth = &AuthConfig{
		Type: authType,
	}

	switch authType {
	case "jwt":
		w.config.Auth.Secret = w.askString("JWT secret", "your-secret-key-here")
		expHours := w.askInt("Token expiration (hours)", 24)
		w.config.Auth.Expiration = time.Duration(expHours) * time.Hour
		w.config.Auth.Issuer = w.askString("Token issuer", w.config.ProjectName)

	case "oauth2":
		w.config.Auth.ClientID = w.askString("OAuth2 Client ID", "your-client-id")
		w.config.Auth.ClientSecret = w.askString("OAuth2 Client Secret", "your-client-secret")
		w.config.Auth.RedirectURL = w.askString("Redirect URL", "http://localhost:8080/auth/callback")
		scopes := w.askString("Scopes (comma-separated)", "openid,profile,email")
		w.config.Auth.Scopes = strings.Split(scopes, ",")
		for i, scope := range w.config.Auth.Scopes {
			w.config.Auth.Scopes[i] = strings.TrimSpace(scope)
		}
		providers := w.askString("OAuth2 providers (comma-separated)", "google,github")
		w.config.Auth.Providers = strings.Split(providers, ",")
		for i, provider := range w.config.Auth.Providers {
			w.config.Auth.Providers[i] = strings.TrimSpace(provider)
		}
	}

	return nil
}

// configureMiddleware 配置中间件
func (w *ConfigWizard) configureMiddleware() error {
	availableMiddleware := []string{
		"cors", "rate-limiting", "compression", "security-headers",
		"request-id", "logging", "metrics", "tracing",
		"validation", "authentication", "authorization",
		"circuit-breaker", "retry", "timeout",
	}

	w.config.Middleware = w.askMultiChoice("Select middleware (comma-separated)", availableMiddleware)

	return nil
}

// configureDeployment 配置部署
func (w *ConfigWizard) configureDeployment() error {
	deploymentTypes := []string{"docker", "kubernetes", "serverless", "vm", "bare-metal"}
	deploymentType := w.askChoice("Deployment type", deploymentTypes, "docker")

	w.config.Deployment = &DeploymentConfig{
		Type: deploymentType,
	}

	switch deploymentType {
	case "docker":
		w.config.Deployment.Registry = w.askString("Docker registry", "docker.io")

	case "kubernetes":
		w.config.Deployment.Registry = w.askString("Docker registry", "docker.io")
		w.config.Deployment.Namespace = w.askString("Kubernetes namespace", "default")
		w.config.Deployment.Helm = w.askYesNo("Use Helm charts?", true)

	case "serverless":
		provider := w.askChoice("Serverless provider", []string{"aws-lambda", "google-functions", "azure-functions"}, "aws-lambda")
		w.config.Deployment.Options = map[string]string{"provider": provider}
	}

	environments := w.askString("Deployment environments (comma-separated)", "development,staging,production")
	w.config.Deployment.Environments = strings.Split(environments, ",")
	for i, env := range w.config.Deployment.Environments {
		w.config.Deployment.Environments[i] = strings.TrimSpace(env)
	}

	w.config.Deployment.CI = w.askYesNo("Enable CI (Continuous Integration)?", true)
	w.config.Deployment.CD = w.askYesNo("Enable CD (Continuous Deployment)?", false)
	w.config.Deployment.Terraform = w.askYesNo("Use Terraform for infrastructure?", false)

	return nil
}

// configureDevelopment 配置开发
func (w *ConfigWizard) configureDevelopment() error {
	w.config.Development = &DevelopmentConfig{
		HotReload:  w.askYesNo("Enable hot reload?", true),
		DebugMode:  w.askYesNo("Enable debug mode?", true),
		LiveReload: w.askYesNo("Enable live reload?", true),
		MockData:   w.askYesNo("Generate mock data?", false),
		APIDoc:     w.askYesNo("Generate API documentation?", true),
		CodeGen:    w.askYesNo("Enable code generation?", true),
		Linting:    w.askYesNo("Enable code linting?", true),
		Formatting: w.askYesNo("Enable code formatting?", true),
	}

	return nil
}

// configureTesting 配置测试
func (w *ConfigWizard) configureTesting() error {
	w.config.Testing = &TestingConfig{
		Unit:        w.askYesNo("Enable unit tests?", true),
		Integration: w.askYesNo("Enable integration tests?", true),
		E2E:         w.askYesNo("Enable E2E tests?", false),
		Load:        w.askYesNo("Enable load tests?", false),
		Chaos:       w.askYesNo("Enable chaos engineering tests?", false),
		Coverage:    w.askYesNo("Enable test coverage?", true),
		Mocking:     w.askYesNo("Enable test mocking?", true),
		Fixtures:    w.askYesNo("Enable test fixtures?", true),
	}

	return nil
}

// configureMonitoring 配置监控
func (w *ConfigWizard) configureMonitoring() error {
	w.config.Monitoring = &MonitoringConfig{
		Metrics:     w.askYesNo("Enable metrics collection?", true),
		Tracing:     w.askYesNo("Enable distributed tracing?", false),
		Logging:     w.askYesNo("Enable structured logging?", true),
		HealthCheck: w.askYesNo("Enable health checks?", true),
		Alerting:    w.askYesNo("Enable alerting?", false),
		Dashboard:   w.askYesNo("Enable monitoring dashboard?", false),
	}

	if w.config.Monitoring.Metrics {
		w.config.Monitoring.Prometheus = w.askYesNo("Use Prometheus for metrics?", true)
	}

	if w.config.Monitoring.Tracing {
		w.config.Monitoring.Jaeger = w.askYesNo("Use Jaeger for tracing?", true)
	}

	if w.config.Monitoring.Dashboard {
		w.config.Monitoring.Grafana = w.askYesNo("Use Grafana for dashboards?", true)
	}

	return nil
}

// showSummary 显示配置摘要
func (w *ConfigWizard) showSummary() {
	fmt.Println("📋 Configuration Summary:")
	fmt.Println("========================")
	fmt.Printf("Project Name: %s\n", w.config.ProjectName)
	fmt.Printf("Description: %s\n", w.config.Description)
	fmt.Printf("Author: %s <%s>\n", w.config.Author, w.config.Email)
	fmt.Printf("Version: %s\n", w.config.Version)
	fmt.Printf("License: %s\n", w.config.License)
	fmt.Printf("Go Version: %s\n", w.config.GoVersion)
	fmt.Printf("Module Path: %s\n", w.config.ModulePath)
	fmt.Printf("Project Type: %s\n", w.config.ProjectType)
	fmt.Printf("Architecture: %s\n", w.config.Architecture)
	fmt.Printf("Features: %s\n", strings.Join(w.config.Features, ", "))

	if w.config.Database != nil {
		fmt.Printf("Database: %s\n", w.config.Database.Type)
	}

	if w.config.Cache != nil {
		fmt.Printf("Cache: %s\n", w.config.Cache.Type)
	}

	if w.config.Auth != nil {
		fmt.Printf("Authentication: %s\n", w.config.Auth.Type)
	}

	if len(w.config.Middleware) > 0 {
		fmt.Printf("Middleware: %s\n", strings.Join(w.config.Middleware, ", "))
	}

	if w.config.Deployment != nil {
		fmt.Printf("Deployment: %s\n", w.config.Deployment.Type)
	}

	fmt.Println()
}

// saveConfig 保存配置
func (w *ConfigWizard) saveConfig() error {
	configPath := filepath.Join(w.projectDir, ".netcore-go.json")

	configData, err := json.MarshalIndent(w.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	fmt.Printf("💾 Configuration saved to %s\n", configPath)
	return nil
}

// 辅助方法

// askString 询问字符串输入
func (w *ConfigWizard) askString(prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

// askInt 询问整数输入
func (w *ConfigWizard) askInt(prompt string, defaultValue int) int {
	fmt.Printf("%s [%d]: ", prompt, defaultValue)

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("Invalid number, using default: %d\n", defaultValue)
		return defaultValue
	}

	return value
}

// askYesNo 询问是否选择
func (w *ConfigWizard) askYesNo(prompt string, defaultValue bool) bool {
	defaultStr := "n"
	if defaultValue {
		defaultStr = "y"
	}

	fmt.Printf("%s [%s]: ", prompt, defaultStr)

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultValue
	}

	return input == "y" || input == "yes" || input == "true" || input == "1"
}

// askChoice 询问选择
func (w *ConfigWizard) askChoice(prompt string, choices []string, defaultValue string) string {
	fmt.Printf("%s\n", prompt)
	for i, choice := range choices {
		marker := " "
		if choice == defaultValue {
			marker = "*"
		}
		fmt.Printf("  %s %d) %s\n", marker, i+1, choice)
	}
	fmt.Printf("Choice [%s]: ", defaultValue)

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	// 尝试解析为数字
	if index, err := strconv.Atoi(input); err == nil {
		if index >= 1 && index <= len(choices) {
			return choices[index-1]
		}
	}

	// 尝试直接匹配
	for _, choice := range choices {
		if strings.EqualFold(input, choice) {
			return choice
		}
	}

	fmt.Printf("Invalid choice, using default: %s\n", defaultValue)
	return defaultValue
}

// askMultiChoice 询问多选
func (w *ConfigWizard) askMultiChoice(prompt string, choices []string) []string {
	fmt.Printf("%s\n", prompt)
	fmt.Println("Available options:")
	for i, choice := range choices {
		fmt.Printf("  %d) %s\n", i+1, choice)
	}
	fmt.Print("Selection (comma-separated numbers or names): ")

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return []string{}
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	var selected []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 尝试解析为数字
		if index, err := strconv.Atoi(part); err == nil {
			if index >= 1 && index <= len(choices) {
				selected = append(selected, choices[index-1])
				continue
			}
		}

		// 尝试直接匹配
		for _, choice := range choices {
			if strings.EqualFold(part, choice) {
				selected = append(selected, choice)
				break
			}
		}
	}

	return selected
}

// SetVerbose 设置详细模式
func (w *ConfigWizard) SetVerbose(verbose bool) {
	w.verbose = verbose
}

// GetConfig 获取配置
func (w *ConfigWizard) GetConfig() *ProjectWizardConfig {
	return w.config
}

// LoadConfig 加载现有配置
func (w *ConfigWizard) LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, w.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}