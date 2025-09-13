// NetCore-Go CLI Tool
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

// Version 版本信息
var (
	Version   = "1.0.0"
	BuildTime = "2024-01-01T00:00:00Z"
	Commit    = "dev"
)

// ProjectConfig 项目配置
type ProjectConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Email       string            `json:"email"`
	Version     string            `json:"version"`
	License     string            `json:"license"`
	GoVersion   string            `json:"go_version"`
	ModulePath  string            `json:"module_path"`
	Features    *FeatureConfig    `json:"features"`
	Database    *DatabaseConfig   `json:"database"`
	Server      *ServerConfig     `json:"server"`
	Cache       string            `json:"cache"`
	Auth        string            `json:"auth"`
	Middleware  []string          `json:"middleware"`
	Plugins     []string          `json:"plugins"`
	Deployment  string            `json:"deployment"`
	Environment map[string]string `json:"environment"`
	CreatedAt   time.Time         `json:"created_at"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `json:"host"`
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// FeatureConfig 功能配置
type FeatureConfig struct {
	Auth       bool `json:"auth"`
	Logging    bool `json:"logging"`
	Metrics    bool `json:"metrics"`
	Tracing    bool `json:"tracing"`
	Swagger    bool `json:"swagger"`
	Docker     bool `json:"docker"`
	Kubernetes bool `json:"kubernetes"`
}

// TemplateData 模板数据
type TemplateData struct {
	Project     *ProjectConfig
	CurrentYear int
	Timestamp   string
}

// CLIConfig CLI配置
type CLIConfig struct {
	DefaultAuthor    string `json:"default_author"`
	DefaultEmail     string `json:"default_email"`
	DefaultGoVersion string `json:"default_go_version"`
	TemplatesDir     string `json:"templates_dir"`
	OutputDir        string `json:"output_dir"`
	Verbose          bool   `json:"verbose"`
}

var (
	cliConfig *CLIConfig
	rootCmd   *cobra.Command
)

func init() {
	cliConfig = &CLIConfig{
		DefaultAuthor:    "NetCore-Go Developer",
		DefaultEmail:     "developer@netcore-go.com",
		DefaultGoVersion: "1.21",
		TemplatesDir:     "templates",
		OutputDir:        ".",
		Verbose:          false,
	}

	rootCmd = &cobra.Command{
		Use:   "netcore-cli",
		Short: "NetCore-Go CLI Tool",
		Long: `NetCore-Go CLI Tool is a command-line interface for creating and managing NetCore-Go projects.

It provides scaffolding, code generation, and project management capabilities.`,
		Version: fmt.Sprintf("%s (built at %s, commit %s)", Version, BuildTime, Commit),
	}

	// 添加子命令
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(deployCmd)

	// 全局标志
	rootCmd.PersistentFlags().BoolVarP(&cliConfig.Verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&cliConfig.OutputDir, "output", ".", "output directory")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// new 命令 - 创建新项目
var newCmd = &cobra.Command{
	Use:   "new [project-name]",
	Short: "Create a new NetCore-Go project",
	Long:  `Create a new NetCore-Go project with the specified name and configuration.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNewProject,
}

func init() {
	newCmd.Flags().StringP("author", "a", "", "project author")
	newCmd.Flags().StringP("email", "e", "", "author email")
	newCmd.Flags().StringP("description", "d", "", "project description")
	newCmd.Flags().StringP("module", "m", "", "Go module path")
	newCmd.Flags().StringSliceP("features", "f", []string{}, "features to include")
	newCmd.Flags().String("database", "", "database type (postgres, mysql, sqlite, mongodb)")
	newCmd.Flags().String("cache", "", "cache type (redis, memcached, in-memory)")
	newCmd.Flags().String("auth", "", "authentication type (jwt, oauth2, basic)")
	newCmd.Flags().StringSlice("middleware", []string{}, "middleware to include")
	newCmd.Flags().StringSlice("plugins", []string{}, "plugins to include")
	newCmd.Flags().String("deployment", "", "deployment target (docker, kubernetes, serverless)")
	newCmd.Flags().BoolP("interactive", "i", false, "interactive mode")
}

func runNewProject(cmd *cobra.Command, args []string) error {
	var projectName string
	if len(args) > 0 {
		projectName = args[0]
	}

	interactive, _ := cmd.Flags().GetBool("interactive")

	config := &ProjectConfig{
		CreatedAt: time.Now(),
		Version:   "0.1.0",
		GoVersion: cliConfig.DefaultGoVersion,
	}

	if interactive {
		if err := runInteractiveSetup(config, projectName); err != nil {
			return fmt.Errorf("interactive setup failed: %w", err)
		}
	} else {
		if err := setupFromFlags(cmd, config, projectName); err != nil {
			return fmt.Errorf("setup from flags failed: %w", err)
		}
	}

	if config.Name == "" {
		return fmt.Errorf("project name is required")
	}

	projectDir := filepath.Join(cliConfig.OutputDir, config.Name)
	if err := createProjectStructure(projectDir, config); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	fmt.Printf("✅ Project '%s' created successfully in %s\n", config.Name, projectDir)
	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", config.Name)
	fmt.Println("  go mod tidy")
	fmt.Println("  netcore-cli dev")

	return nil
}

func runInteractiveSetup(config *ProjectConfig, initialName string) error {
	reader := bufio.NewReader(os.Stdin)

	// 项目名称
	if initialName != "" {
		config.Name = initialName
	} else {
		fmt.Print("Project name: ")
		name, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		config.Name = strings.TrimSpace(name)
	}

	// 项目描述
	fmt.Print("Project description: ")
	desc, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.Description = strings.TrimSpace(desc)

	// 作者信息
	fmt.Printf("Author [%s]: ", cliConfig.DefaultAuthor)
	author, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	author = strings.TrimSpace(author)
	if author == "" {
		author = cliConfig.DefaultAuthor
	}
	config.Author = author

	// 邮箱
	fmt.Printf("Email [%s]: ", cliConfig.DefaultEmail)
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		email = cliConfig.DefaultEmail
	}
	config.Email = email

	// 模块路径
	defaultModule := fmt.Sprintf("github.com/%s/%s", strings.ToLower(config.Author), config.Name)
	fmt.Printf("Go module path [%s]: ", defaultModule)
	modulePath, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		modulePath = defaultModule
	}
	config.ModulePath = modulePath

	// 功能选择
	fmt.Println("\nSelect features (comma-separated):")
	fmt.Println("  - http2: HTTP/2 support")
	fmt.Println("  - grpc: gRPC support")
	fmt.Println("  - websocket: WebSocket support")
	fmt.Println("  - metrics: Metrics and monitoring")
	fmt.Println("  - tracing: Distributed tracing")
	fmt.Println("  - logging: Structured logging")
	fmt.Println("  - validation: Request validation")
	fmt.Println("  - swagger: OpenAPI/Swagger documentation")
	fmt.Print("Features: ")
	featuresStr, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	featuresStr = strings.TrimSpace(featuresStr)
	if featuresStr != "" {
		features := strings.Split(featuresStr, ",")
		for i, feature := range features {
			features[i] = strings.TrimSpace(feature)
		}
		// 这里需要根据实际需求设置Features字段
		config.Features = &FeatureConfig{}
		// 可以根据features切片设置具体的功能开关
	}

	// 数据库选择
	fmt.Println("\nDatabase (postgres/mysql/sqlite/mongodb/none): ")
	db, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.Database = &DatabaseConfig{
			Type: strings.TrimSpace(db),
		}

	// 缓存选择
	fmt.Println("Cache (redis/memcached/in-memory/none): ")
	cache, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.Cache = strings.TrimSpace(cache)

	// 认证选择
	fmt.Println("Authentication (jwt/oauth2/basic/none): ")
	auth, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.Auth = strings.TrimSpace(auth)

	return nil
}

func setupFromFlags(cmd *cobra.Command, config *ProjectConfig, projectName string) error {
	config.Name = projectName

	if author, _ := cmd.Flags().GetString("author"); author != "" {
		config.Author = author
	} else {
		config.Author = cliConfig.DefaultAuthor
	}

	if email, _ := cmd.Flags().GetString("email"); email != "" {
		config.Email = email
	} else {
		config.Email = cliConfig.DefaultEmail
	}

	if desc, _ := cmd.Flags().GetString("description"); desc != "" {
		config.Description = desc
	}

	if module, _ := cmd.Flags().GetString("module"); module != "" {
		config.ModulePath = module
	} else {
		config.ModulePath = fmt.Sprintf("github.com/%s/%s", strings.ToLower(config.Author), config.Name)
	}

	if features, _ := cmd.Flags().GetStringSlice("features"); len(features) > 0 {
		config.Features = &FeatureConfig{}
		// 根据features设置具体功能
	}

	if db, _ := cmd.Flags().GetString("database"); db != "" {
		config.Database = &DatabaseConfig{
			Type: db,
		}
	}

	if cache, _ := cmd.Flags().GetString("cache"); cache != "" {
		config.Cache = cache
	}

	if auth, _ := cmd.Flags().GetString("auth"); auth != "" {
		config.Auth = auth
	}

	if middleware, _ := cmd.Flags().GetStringSlice("middleware"); len(middleware) > 0 {
		config.Middleware = middleware
	}

	if plugins, _ := cmd.Flags().GetStringSlice("plugins"); len(plugins) > 0 {
		config.Plugins = plugins
	}

	if deployment, _ := cmd.Flags().GetString("deployment"); deployment != "" {
		config.Deployment = deployment
	}

	return nil
}

func createProjectStructure(projectDir string, config *ProjectConfig) error {
	// 创建项目目录
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	// 创建目录结构
	dirs := []string{
		"cmd",
		"pkg",
		"internal",
		"api",
		"web",
		"configs",
		"scripts",
		"docs",
		"tests",
		"deployments",
		"examples",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0755); err != nil {
			return err
		}
	}

	// 生成文件
	if err := generateProjectFiles(projectDir, config); err != nil {
		return err
	}

	return nil
}

func generateProjectFiles(projectDir string, config *ProjectConfig) error {
	templateData := &TemplateData{
		Project:     config,
		CurrentYear: time.Now().Year(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// 生成 go.mod
	if err := generateFile(projectDir, "go.mod", goModTemplate, templateData); err != nil {
		return err
	}

	// 生成 main.go
	if err := generateFile(projectDir, "cmd/main.go", mainGoTemplate, templateData); err != nil {
		return err
	}

	// 生成 README.md
	if err := generateFile(projectDir, "README.md", readmeTemplate, templateData); err != nil {
		return err
	}

	// 生成 Dockerfile
	if err := generateFile(projectDir, "Dockerfile", dockerfileTemplate, templateData); err != nil {
		return err
	}

	// 生成 .gitignore
	if err := generateFile(projectDir, ".gitignore", gitignoreTemplate, templateData); err != nil {
		return err
	}

	// 生成 Makefile
	if err := generateFile(projectDir, "Makefile", makefileTemplate, templateData); err != nil {
		return err
	}

	// 生成配置文件
	if err := generateFile(projectDir, "configs/config.yaml", configTemplate, templateData); err != nil {
		return err
	}

	// 生成项目配置
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(projectDir, ".netcore-go.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return err
	}

	return nil
}

func generateFile(projectDir, filename, templateStr string, data *TemplateData) error {
	tmpl, err := template.New(filename).Parse(templateStr)
	if err != nil {
		return err
	}

	filePath := filepath.Join(projectDir, filename)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// Generate functions are implemented in generate.go

func generateFromTemplate(filename, templateStr string, data interface{}) error {
	tmpl, err := template.New(filename).Parse(templateStr)
	if err != nil {
		return err
	}

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return err
	}

	fmt.Printf("✅ Generated %s\n", filename)
	return nil
}

// config 命令 - 配置管理
var configCmd = &cobra.Command{
	Use:   "config [action]",
	Short: "Manage CLI configuration",
	Long:  `Manage CLI configuration settings.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfig,
}

func runConfig(cmd *cobra.Command, args []string) error {
	action := args[0]

	switch action {
	case "init":
		return initConfig()
	case "show":
		return showConfig()
	case "set":
		return setConfig(cmd)
	default:
		return fmt.Errorf("unknown config action: %s", action)
	}
}

func initConfig() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	netcoreDir := filepath.Join(configDir, "netcore-go")
	if err := os.MkdirAll(netcoreDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(netcoreDir, "config.json")
	configData, err := json.MarshalIndent(cliConfig, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		return err
	}

	fmt.Printf("✅ Configuration initialized at %s\n", configFile)
	return nil
}

func showConfig() error {
	configData, err := json.MarshalIndent(cliConfig, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(configData))
	return nil
}

func setConfig(cmd *cobra.Command) error {
	// 实现配置设置逻辑
	fmt.Println("Config set functionality not implemented yet")
	return nil
}

// template 命令 - 模板管理
var templateCmd = &cobra.Command{
	Use:   "template [action]",
	Short: "Manage project templates",
	Long:  `Manage project templates for scaffolding.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplate,
}

func runTemplate(cmd *cobra.Command, args []string) error {
	action := args[0]

	switch action {
	case "list":
		return listTemplates()
	case "create":
		return createTemplate(cmd)
	case "update":
		return updateTemplate(cmd)
	default:
		return fmt.Errorf("unknown template action: %s", action)
	}
}

func listTemplates() error {
	fmt.Println("Available templates:")
	fmt.Println("  - basic: Basic NetCore-Go project")
	fmt.Println("  - api: REST API project")
	fmt.Println("  - microservice: Microservice project")
	fmt.Println("  - web: Web application project")
	return nil
}

func createTemplate(cmd *cobra.Command) error {
	fmt.Println("Template creation functionality not implemented yet")
	return nil
}

func updateTemplate(cmd *cobra.Command) error {
	fmt.Println("Template update functionality not implemented yet")
	return nil
}

// Dev command is implemented in dev.go

// build 命令 - 构建项目
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the project",
	Long:  `Build the project for production deployment.`,
	RunE:  runBuild,
}

func init() {
	buildCmd.Flags().StringP("output", "o", "bin/app", "output binary path")
	buildCmd.Flags().StringP("target", "t", "", "build target (linux, windows, darwin)")
	buildCmd.Flags().BoolP("docker", "d", false, "build Docker image")
	buildCmd.Flags().String("tag", "latest", "Docker image tag")
}

func runBuild(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	target, _ := cmd.Flags().GetString("target")
	docker, _ := cmd.Flags().GetBool("docker")
	tag, _ := cmd.Flags().GetString("tag")

	if docker {
		fmt.Printf("🐳 Building Docker image with tag: %s\n", tag)
		// 实现Docker构建逻辑
		fmt.Println("Docker build functionality not implemented yet")
	} else {
		fmt.Printf("🔨 Building binary to: %s\n", output)
		if target != "" {
			fmt.Printf("🎯 Target platform: %s\n", target)
		}
		// 实现Go构建逻辑
		fmt.Println("Binary build functionality not implemented yet")
	}

	return nil
}

// Deploy command is implemented in deploy.go

// Template definitions
const (
	goModTemplate = `module {{.Project.Name}}

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.16.0
)
`

	mainGoTemplate = `package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from {{.Project.Name}}!",
		})
	})

	fmt.Println("Starting {{.Project.Name}} server...")
	log.Fatal(r.Run(":8080"))
}
`

	readmeTemplate = `# {{.Project.Name}}

{{.Project.Description}}

## Author
{{.Project.Author}}

## License
{{.Project.License}}

## Getting Started

` + "`" + `bash
go run cmd/main.go
` + "`" + `
`

	dockerfileTemplate = `FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
`

	gitignoreTemplate = `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/

# Test binary
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# IDE
.vscode/
.idea/

# OS
.DS_Store
Thumbs.db
`

	makefileTemplate = `# {{.Project.Name}} Makefile

.PHONY: build run test clean

build:
	go build -o bin/{{.Project.Name}} cmd/main.go

run:
	go run cmd/main.go

test:
	go test ./...

clean:
	rm -rf bin/
`

	configTemplate = `# {{.Project.Name}} Configuration

server:
  host: "0.0.0.0"
  port: 8080

log:
  level: "info"
  format: "json"
`
)