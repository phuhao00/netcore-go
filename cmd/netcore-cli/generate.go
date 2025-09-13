// NetCore-Go CLI Code Generation Commands
// Enhanced code generation capabilities
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// GenerateData 生成数据
type GenerateData struct {
	Name        string
	NameLower   string
	TableName   string
	Package     string
	Project     *ProjectConfig
	Fields      []Field
	Imports     []string
	Options     map[string]interface{}
}

// Field 字段定义
type Field struct {
	Name     string
	Type     string
	Tag      string
	Comment  string
	Required bool
}

var (
	templateRegistry *TemplateRegistry
)

func init() {
	templateRegistry = NewTemplateRegistry()
}

// generate 命令 - 代码生成
var generateCmd = &cobra.Command{
	Use:   "generate [type] [name]",
	Short: "Generate code from templates",
	Long: `Generate code from built-in or custom templates.

Available types:
  service     - Generate service layer code
  handler     - Generate HTTP handlers
  model       - Generate data models
  repository  - Generate repository layer
  middleware  - Generate middleware
  config      - Generate configuration
  test        - Generate test files
  docker      - Generate Docker files
  k8s         - Generate Kubernetes manifests
  makefile    - Generate Makefile
  ci          - Generate CI/CD configurations

Examples:
  netcore-cli generate service User
  netcore-cli generate handler User --with-validation
  netcore-cli generate model Product --fields="name:string,price:float64"
  netcore-cli generate repository User --database=postgres
  netcore-cli generate docker --multi-stage
  netcore-cli generate k8s --namespace=production`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runGenerate,
}

func init() {
	// 通用标志
	generateCmd.Flags().StringP("output", "o", ".", "output directory")
	generateCmd.Flags().StringP("package", "p", "", "package name")
	generateCmd.Flags().StringSlice("fields", []string{}, "model fields (name:type:tag)")
	generateCmd.Flags().StringSlice("imports", []string{}, "additional imports")
	generateCmd.Flags().Bool("force", false, "overwrite existing files")
	generateCmd.Flags().Bool("dry-run", false, "show what would be generated without creating files")
	
	// 特定类型标志
	generateCmd.Flags().String("database", "postgres", "database type for repository")
	generateCmd.Flags().String("cache", "redis", "cache type")
	generateCmd.Flags().Bool("with-validation", false, "include validation")
	generateCmd.Flags().Bool("with-swagger", false, "include Swagger annotations")
	generateCmd.Flags().Bool("with-tests", false, "generate test files")
	generateCmd.Flags().Bool("with-mocks", false, "generate mock files")
	
	// Docker标志
	generateCmd.Flags().Bool("multi-stage", true, "use multi-stage Docker build")
	generateCmd.Flags().String("base-image", "alpine", "base Docker image")
	
	// Kubernetes标志
	generateCmd.Flags().String("namespace", "default", "Kubernetes namespace")
	generateCmd.Flags().Int32("replicas", 3, "number of replicas")
	generateCmd.Flags().StringSlice("ports", []string{"8080"}, "service ports")
	
	// CI/CD标志
	generateCmd.Flags().String("ci-type", "github", "CI/CD type (github, gitlab, jenkins)")
	generateCmd.Flags().StringSlice("stages", []string{"test", "build", "deploy"}, "CI/CD stages")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	generateType := args[0]
	var name string
	if len(args) > 1 {
		name = args[1]
	}
	
	outputDir, _ := cmd.Flags().GetString("output")
	packageName, _ := cmd.Flags().GetString("package")
	fields, _ := cmd.Flags().GetStringSlice("fields")
	imports, _ := cmd.Flags().GetStringSlice("imports")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	
	// 加载项目配置
	projectConfig, err := loadProjectConfig(outputDir)
	if err != nil {
		// 如果没有项目配置，创建默认配置
		projectConfig = &ProjectConfig{
			Name:       "netcore-project",
			Author:     cliConfig.DefaultAuthor,
			Email:      cliConfig.DefaultEmail,
			GoVersion:  cliConfig.DefaultGoVersion,
			ModulePath: "github.com/example/netcore-project",
		}
	}
	
	// 创建生成数据
	generateData := &GenerateData{
		Name:      name,
		NameLower: strings.ToLower(name),
		TableName: toSnakeCase(name),
		Package:   packageName,
		Project:   projectConfig,
		Fields:    parseFields(fields),
		Imports:   imports,
		Options:   make(map[string]interface{}),
	}
	
	// 设置选项
	setGenerateOptions(cmd, generateData)
	
	// 根据类型生成代码
	switch generateType {
	case "service":
		return generateService(generateData, outputDir, force, dryRun)
	case "handler":
		return generateHandler(generateData, outputDir, force, dryRun)
	case "model":
		return generateModel(generateData, outputDir, force, dryRun)
	case "repository":
		return generateRepository(generateData, outputDir, force, dryRun)
	case "middleware":
		return generateMiddleware(generateData, outputDir, force, dryRun)
	case "config":
		return generateConfig(generateData, outputDir, force, dryRun)
	case "test":
		return generateTest(generateData, outputDir, force, dryRun)
	case "docker":
		return generateDocker(generateData, outputDir, force, dryRun)
	case "k8s":
		return generateK8s(generateData, outputDir, force, dryRun)
	case "makefile":
		return generateMakefile(generateData, outputDir, force, dryRun)
	case "ci":
		return generateCI(generateData, outputDir, force, dryRun)
	default:
		return fmt.Errorf("unknown generate type: %s", generateType)
	}
}

// loadProjectConfig 加载项目配置
func loadProjectConfig(dir string) (*ProjectConfig, error) {
	configPath := filepath.Join(dir, "netcore.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project config not found")
	}
	
	// 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// 解析JSON配置
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// 创建项目配置结构
	projectConfig := &ProjectConfig{
		Name:        getString(config, "name", ""),
		Version:     getString(config, "version", "1.0.0"),
		Description: getString(config, "description", ""),
		Author:      getString(config, "author", ""),
		License:     getString(config, "license", "MIT"),
		GoVersion:   getString(config, "go_version", "1.21"),
		ModulePath:  getString(config, "module_path", ""),
	}
	
	// 解析数据库配置
	if dbConfig, ok := config["database"].(map[string]interface{}); ok {
		projectConfig.Database = &DatabaseConfig{
			Type:     getString(dbConfig, "type", "sqlite"),
			Host:     getString(dbConfig, "host", "localhost"),
			Port:     getInt(dbConfig, "port", 5432),
			Name:     getString(dbConfig, "name", "app"),
			User:     getString(dbConfig, "user", ""),
			Password: getString(dbConfig, "password", ""),
			SSLMode:  getString(dbConfig, "ssl_mode", "disable"),
		}
	}
	
	// 解析服务器配置
	if serverConfig, ok := config["server"].(map[string]interface{}); ok {
		projectConfig.Server = &ServerConfig{
			Host:         getString(serverConfig, "host", "localhost"),
			Port:         getInt(serverConfig, "port", 8080),
			ReadTimeout:  getDuration(serverConfig, "read_timeout", "30s"),
			WriteTimeout: getDuration(serverConfig, "write_timeout", "30s"),
			IdleTimeout:  getDuration(serverConfig, "idle_timeout", "120s"),
		}
	}
	
	// 解析功能配置
	if features, ok := config["features"].(map[string]interface{}); ok {
		projectConfig.Features = &FeatureConfig{
			Auth:       getBool(features, "auth", false),
			Logging:    getBool(features, "logging", true),
			Metrics:    getBool(features, "metrics", false),
			Tracing:    getBool(features, "tracing", false),
			Swagger:    getBool(features, "swagger", false),
			Docker:     getBool(features, "docker", false),
			Kubernetes: getBool(features, "kubernetes", false),
		}
	}
	
	return projectConfig, nil
}

// parseFields 解析字段定义
func parseFields(fieldStrs []string) []Field {
	fields := make([]Field, 0, len(fieldStrs))
	
	for _, fieldStr := range fieldStrs {
		parts := strings.Split(fieldStr, ":")
		if len(parts) < 2 {
			continue
		}
		
		field := Field{
			Name: parts[0],
			Type: parts[1],
		}
		
		if len(parts) > 2 {
			field.Tag = parts[2]
		}
		
		fields = append(fields, field)
	}
	
	return fields
}

// setGenerateOptions 设置生成选项
func setGenerateOptions(cmd *cobra.Command, data *GenerateData) {
	if database, _ := cmd.Flags().GetString("database"); database != "" {
		data.Options["database"] = database
	}
	
	if cache, _ := cmd.Flags().GetString("cache"); cache != "" {
		data.Options["cache"] = cache
	}
	
	if withValidation, _ := cmd.Flags().GetBool("with-validation"); withValidation {
		data.Options["with_validation"] = true
	}
	
	if withSwagger, _ := cmd.Flags().GetBool("with-swagger"); withSwagger {
		data.Options["with_swagger"] = true
	}
	
	if withTests, _ := cmd.Flags().GetBool("with-tests"); withTests {
		data.Options["with_tests"] = true
	}
	
	if withMocks, _ := cmd.Flags().GetBool("with-mocks"); withMocks {
		data.Options["with_mocks"] = true
	}
	
	if multiStage, _ := cmd.Flags().GetBool("multi-stage"); multiStage {
		data.Options["multi_stage"] = true
	}
	
	if baseImage, _ := cmd.Flags().GetString("base-image"); baseImage != "" {
		data.Options["base_image"] = baseImage
	}
	
	if namespace, _ := cmd.Flags().GetString("namespace"); namespace != "" {
		data.Options["namespace"] = namespace
	}
	
	if replicas, _ := cmd.Flags().GetInt32("replicas"); replicas > 0 {
		data.Options["replicas"] = replicas
	}
	
	if ports, _ := cmd.Flags().GetStringSlice("ports"); len(ports) > 0 {
		data.Options["ports"] = ports
	}
	
	if ciType, _ := cmd.Flags().GetString("ci-type"); ciType != "" {
		data.Options["ci_type"] = ciType
	}
	
	if stages, _ := cmd.Flags().GetStringSlice("stages"); len(stages) > 0 {
		data.Options["stages"] = stages
	}
}

// generateService 生成服务代码
func generateService(data *GenerateData, outputDir string, force, dryRun bool) error {
	if data.Name == "" {
		return fmt.Errorf("service name is required")
	}
	
	templateName := "rest-service"
	if data.Options["type"] == "grpc" {
		templateName = "grpc-service"
	}
	
	if dryRun {
		fmt.Printf("Would generate service: %s using template: %s\n", data.Name, templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate service: %w", err)
	}
	
	fmt.Printf("✅ Generated service: %s\n", data.Name)
	
	// 生成相关测试文件
	if data.Options["with_tests"] == true {
		if err := generateServiceTest(data, outputDir, force, dryRun); err != nil {
			fmt.Printf("⚠️  Failed to generate service tests: %v\n", err)
		}
	}
	
	return nil
}

// generateHandler 生成处理器代码
func generateHandler(data *GenerateData, outputDir string, force, dryRun bool) error {
	if data.Name == "" {
		return fmt.Errorf("handler name is required")
	}
	
	templateName := "http-handler"
	if data.Options["type"] == "websocket" {
		templateName = "websocket-handler"
	}
	
	if dryRun {
		fmt.Printf("Would generate handler: %s using template: %s\n", data.Name, templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate handler: %w", err)
	}
	
	fmt.Printf("✅ Generated handler: %s\n", data.Name)
	return nil
}

// generateModel 生成模型代码
func generateModel(data *GenerateData, outputDir string, force, dryRun bool) error {
	if data.Name == "" {
		return fmt.Errorf("model name is required")
	}
	
	templateName := "entity-model"
	if data.Options["type"] == "dto" {
		templateName = "dto-model"
	}
	
	if dryRun {
		fmt.Printf("Would generate model: %s using template: %s\n", data.Name, templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}
	
	fmt.Printf("✅ Generated model: %s\n", data.Name)
	return nil
}

// generateRepository 生成仓储代码
func generateRepository(data *GenerateData, outputDir string, force, dryRun bool) error {
	if data.Name == "" {
		return fmt.Errorf("repository name is required")
	}
	
	templateName := "gorm-repository"
	if database, ok := data.Options["database"].(string); ok {
		switch database {
		case "mongodb":
			templateName = "mongo-repository"
		}
	}
	
	if dryRun {
		fmt.Printf("Would generate repository: %s using template: %s\n", data.Name, templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate repository: %w", err)
	}
	
	fmt.Printf("✅ Generated repository: %s\n", data.Name)
	return nil
}

// generateMiddleware 生成中间件代码
func generateMiddleware(data *GenerateData, outputDir string, force, dryRun bool) error {
	if data.Name == "" {
		return fmt.Errorf("middleware name is required")
	}
	
	// 根据名称选择模板
	var templateName string
	switch strings.ToLower(data.Name) {
	case "auth", "authentication":
		templateName = "auth-middleware"
	case "cors":
		templateName = "cors-middleware"
	case "ratelimit", "rate-limit":
		templateName = "rate-limit-middleware"
	default:
		templateName = "auth-middleware" // 默认模板
	}
	
	if dryRun {
		fmt.Printf("Would generate middleware: %s using template: %s\n", data.Name, templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate middleware: %w", err)
	}
	
	fmt.Printf("✅ Generated middleware: %s\n", data.Name)
	return nil
}

// generateConfig 生成配置代码
func generateConfig(data *GenerateData, outputDir string, force, dryRun bool) error {
	templateName := "app-config"
	
	if dryRun {
		fmt.Printf("Would generate config using template: %s\n", templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}
	
	fmt.Printf("✅ Generated config\n")
	return nil
}

// generateTest 生成测试代码
func generateTest(data *GenerateData, outputDir string, force, dryRun bool) error {
	if data.Name == "" {
		return fmt.Errorf("test name is required")
	}
	
	templateName := "unit-test"
	if data.Options["type"] == "integration" {
		templateName = "integration-test"
	}
	
	if dryRun {
		fmt.Printf("Would generate test: %s using template: %s\n", data.Name, templateName)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate test: %w", err)
	}
	
	fmt.Printf("✅ Generated test: %s\n", data.Name)
	return nil
}

// generateServiceTest 生成服务测试
func generateServiceTest(data *GenerateData, outputDir string, force, dryRun bool) error {
	testData := *data
	testData.Package = "service"
	return generateTest(&testData, outputDir, force, dryRun)
}

// generateDocker 生成Docker文件
func generateDocker(data *GenerateData, outputDir string, force, dryRun bool) error {
	if dryRun {
		fmt.Printf("Would generate Docker files\n")
		return nil
	}
	
	// 生成Dockerfile
	if err := templateRegistry.GenerateFromTemplate("dockerfile", data, outputDir); err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}
	
	// 生成docker-compose.yml
	if err := templateRegistry.GenerateFromTemplate("docker-compose", data, outputDir); err != nil {
		return fmt.Errorf("failed to generate docker-compose.yml: %w", err)
	}
	
	fmt.Printf("✅ Generated Docker files\n")
	return nil
}

// generateK8s 生成Kubernetes清单
func generateK8s(data *GenerateData, outputDir string, force, dryRun bool) error {
	if dryRun {
		fmt.Printf("Would generate Kubernetes manifests\n")
		return nil
	}
	
	// 生成Deployment
	if err := templateRegistry.GenerateFromTemplate("k8s-deployment", data, outputDir); err != nil {
		return fmt.Errorf("failed to generate Kubernetes deployment: %w", err)
	}
	
	// 生成Service
	if err := templateRegistry.GenerateFromTemplate("k8s-service", data, outputDir); err != nil {
		return fmt.Errorf("failed to generate Kubernetes service: %w", err)
	}
	
	fmt.Printf("✅ Generated Kubernetes manifests\n")
	return nil
}

// generateMakefile 生成Makefile
func generateMakefile(data *GenerateData, outputDir string, force, dryRun bool) error {
	if dryRun {
		fmt.Printf("Would generate Makefile\n")
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate("makefile", data, outputDir); err != nil {
		return fmt.Errorf("failed to generate Makefile: %w", err)
	}
	
	fmt.Printf("✅ Generated Makefile\n")
	return nil
}

// generateCI 生成CI/CD配置
func generateCI(data *GenerateData, outputDir string, force, dryRun bool) error {
	ciType := "github"
	if ct, ok := data.Options["ci_type"].(string); ok {
		ciType = ct
	}
	
	var templateName string
	switch ciType {
	case "github":
		templateName = "github-actions"
	case "gitlab":
		templateName = "gitlab-ci"
	default:
		return fmt.Errorf("unsupported CI type: %s", ciType)
	}
	
	if dryRun {
		fmt.Printf("Would generate %s CI configuration\n", ciType)
		return nil
	}
	
	if err := templateRegistry.GenerateFromTemplate(templateName, data, outputDir); err != nil {
		return fmt.Errorf("failed to generate CI configuration: %w", err)
	}
	
	fmt.Printf("✅ Generated %s CI configuration\n", ciType)
	return nil
}

// toSnakeCase 转换为蛇形命名
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// 配置解析辅助函数
func getString(config map[string]interface{}, key, defaultValue string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return defaultValue
}

func getInt(config map[string]interface{}, key string, defaultValue int) int {
	if value, ok := config[key].(float64); ok {
		return int(value)
	}
	if value, ok := config[key].(int); ok {
		return value
	}
	return defaultValue
}

func getBool(config map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return defaultValue
}

func getDuration(config map[string]interface{}, key, defaultValue string) time.Duration {
	if value, ok := config[key].(string); ok {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	if duration, err := time.ParseDuration(defaultValue); err == nil {
		return duration
	}
	return 30 * time.Second
}