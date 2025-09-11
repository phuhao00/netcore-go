// NetCore-Go CLI Templates
// Author: NetCore-Go Team
// Created: 2024

package main

// goModTemplate go.mod文件模板
const goModTemplate = `module {{.Project.ModulePath}}

go {{.Project.GoVersion}}

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.17.0
	github.com/sirupsen/logrus v1.9.3
	github.com/prometheus/client_golang v1.17.0
	github.com/stretchr/testify v1.8.4
{{- if eq .Project.Database "postgres"}}
	github.com/lib/pq v1.10.9
	gorm.io/driver/postgres v1.5.4
	gorm.io/gorm v1.25.5
{{- end}}
{{- if eq .Project.Database "mysql"}}
	github.com/go-sql-driver/mysql v1.7.1
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.5
{{- end}}
{{- if eq .Project.Database "sqlite"}}
	gorm.io/driver/sqlite v1.5.4
	gorm.io/gorm v1.25.5
{{- end}}
{{- if eq .Project.Database "mongodb"}}
	go.mongodb.org/mongo-driver v1.12.1
{{- end}}
{{- if eq .Project.Cache "redis"}}
	github.com/redis/go-redis/v9 v9.3.0
{{- end}}
{{- if eq .Project.Auth "jwt"}}
	github.com/golang-jwt/jwt/v5 v5.2.0
{{- end}}
{{- if contains .Project.Features "grpc"}}
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
{{- end}}
{{- if contains .Project.Features "websocket"}}
	github.com/gorilla/websocket v1.5.1
{{- end}}
{{- if contains .Project.Features "swagger"}}
	github.com/swaggo/gin-swagger v1.6.0
	github.com/swaggo/files v1.0.1
	github.com/swaggo/swag v1.16.2
{{- end}}
)
`

// mainGoTemplate main.go文件模板
const mainGoTemplate = `// {{.Project.Name}} - {{.Project.Description}}
// Author: {{.Project.Author}} <{{.Project.Email}}>
// Created: {{.Timestamp}}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/sirupsen/logrus"
{{- if contains .Project.Features "swagger"}}
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
{{- end}}
{{- if contains .Project.Features "metrics"}}
	"github.com/prometheus/client_golang/prometheus/promhttp"
{{- end}}
)

// @title {{.Project.Name}} API
// @version {{.Project.Version}}
// @description {{.Project.Description}}
// @contact.name {{.Project.Author}}
// @contact.email {{.Project.Email}}
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// 初始化配置
	initConfig()

	// 初始化日志
	initLogger()

	// 创建Gin引擎
	r := gin.New()

	// 添加中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
{{- if contains .Project.Features "metrics"}}
	r.Use(prometheusMiddleware())
{{- end}}

	// 设置路由
	setupRoutes(r)

	// 启动服务器
	port := viper.GetString("server.port")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// 优雅关闭
	go func() {
		logrus.Infof("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down server...")

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Fatalf("Server forced to shutdown: %v", err)
	}

	logrus.Info("Server exited")
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("log.level", "info")

	if err := viper.ReadInConfig(); err != nil {
		logrus.Warnf("Config file not found: %v", err)
	}

	viper.AutomaticEnv()
}

func initLogger() {
	level := viper.GetString("log.level")
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}

	logrus.SetLevel(logLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func setupRoutes(r *gin.Engine) {
	// 健康检查
	r.GET("/health", healthCheck)

{{- if contains .Project.Features "metrics"}}
	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
{{- end}}

{{- if contains .Project.Features "swagger"}}
	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
{{- end}}

	// API路由组
	api := r.Group("/api/v1")
	{
		api.GET("/ping", ping)
		// 在这里添加更多API路由
	}
}

// healthCheck 健康检查端点
// @Summary Health check
// @Description Check if the service is healthy
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"service":   "{{.Project.Name}}",
		"version":   "{{.Project.Version}}",
	})
}

// ping Ping端点
// @Summary Ping
// @Description Ping endpoint
// @Tags ping
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ping [get]
func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"time":    time.Now().Unix(),
	})
}

{{- if contains .Project.Features "metrics"}}
func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 实现Prometheus指标收集
		c.Next()
	}
}
{{- end}}
`

// readmeTemplate README.md文件模板
const readmeTemplate = `# {{.Project.Name}}

{{.Project.Description}}

## Features

{{- range .Project.Features}}
- {{.}}
{{- end}}

## Quick Start

### Prerequisites

- Go {{.Project.GoVersion}} or later
{{- if ne .Project.Database ""}}
- {{.Project.Database | title}} database
{{- end}}
{{- if ne .Project.Cache ""}}
- {{.Project.Cache | title}} cache
{{- end}}

### Installation

1. Clone the repository:
   \`\`\`bash
   git clone <repository-url>
   cd {{.Project.Name}}
   \`\`\`

2. Install dependencies:
   \`\`\`bash
   go mod tidy
   \`\`\`

3. Copy and configure the environment:
   \`\`\`bash
   cp configs/config.yaml.example configs/config.yaml
   \`\`\`

4. Run the application:
   \`\`\`bash
   go run cmd/main.go
   \`\`\`

### Using NetCore-Go CLI

```bash
# Start development server with hot reload
netcore-cli dev

# Generate code components
netcore-cli generate handler User
netcore-cli generate model Product --fields="name:string,price:float64"

# Build for production
netcore-cli build --docker --tag={{.Project.Name}}:latest

# Deploy to Kubernetes
netcore-cli deploy kubernetes
```

## API Documentation

{{- if contains .Project.Features "swagger"}}
Once the server is running, you can access the API documentation at:
- Swagger UI: http://localhost:8080/swagger/index.html
{{- end}}

## Configuration

The application can be configured using the \`configs/config.yaml\` file or environment variables.

### Environment Variables

- \`SERVER_PORT\`: Server port (default: 8080)
- \`LOG_LEVEL\`: Log level (default: info)
{{- if ne .Project.Database ""}}
- \`DATABASE_URL\`: Database connection URL
{{- end}}
{{- if ne .Project.Cache ""}}
- \`CACHE_URL\`: Cache connection URL
{{- end}}

## Development

### Project Structure

```
{{.Project.Name}}/
├── cmd/                 # Application entrypoints
├── internal/            # Private application code
│   ├── handlers/        # HTTP handlers
│   ├── services/        # Business logic
│   ├── models/          # Data models
│   └── middleware/      # Custom middleware
├── pkg/                 # Public library code
├── api/                 # API definitions
├── web/                 # Web assets
├── configs/             # Configuration files
├── scripts/             # Build and deployment scripts
├── docs/                # Documentation
├── tests/               # Test files
├── deployments/         # Deployment configurations
└── examples/            # Example code
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./tests/...
```

### Building

```bash
# Build binary
go build -o bin/{{.Project.Name}} cmd/main.go

# Build Docker image
docker build -t {{.Project.Name}}:latest .

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build -o bin/{{.Project.Name}}-linux-amd64 cmd/main.go
```

## Deployment

### Docker

```bash
# Build and run with Docker
docker build -t {{.Project.Name}} .
docker run -p 8080:8080 {{.Project.Name}}
```

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/
```

## Contributing

1. Fork the repository
2. Create your feature branch (\`git checkout -b feature/amazing-feature\`)
3. Commit your changes (\`git commit -m 'Add some amazing feature'\`)
4. Push to the branch (\`git push origin feature/amazing-feature\`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

{{.Project.Author}} <{{.Project.Email}}>

## Acknowledgments

- Built with [NetCore-Go](https://github.com/netcore-go/netcore-go)
- Powered by [Gin](https://github.com/gin-gonic/gin)
`

// dockerfileTemplate Dockerfile模板
const dockerfileTemplate = `# Build stage
FROM golang:{{.Project.GoVersion}}-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /root/

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/configs ./configs

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]
`

// gitignoreTemplate .gitignore文件模板
const gitignoreTemplate = `# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/

# Test binary, built with \`go test -c\`
*.test

# Output of the go coverage tool
*.out
*.coverage

# Dependency directories
vendor/

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS generated files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db

# Application specific
logs/
*.log
tmp/
temp/

# Configuration files with secrets
*.env
*.env.local
*.env.production
configs/config.yaml
!configs/config.yaml.example

# Database files
*.db
*.sqlite
*.sqlite3

# Docker
.dockerignore

# Kubernetes secrets
secrets/

# Test coverage
coverage.html
coverage.xml

# Build artifacts
dist/
build/
`

// makefileTemplate Makefile模板
const makefileTemplate = `.PHONY: build run test clean docker help

# Variables
APP_NAME={{.Project.Name}}
VERSION={{.Project.Version}}
GO_VERSION={{.Project.GoVersion}}
BUILD_DIR=bin
DOCKER_IMAGE=$(APP_NAME):$(VERSION)

# Default target
all: build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "-X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME) cmd/main.go

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	@go run cmd/main.go

# Run with hot reload
dev:
	@echo "Starting development server..."
	@netcore-cli dev

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@go test -tags=integration -v ./tests/...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Build Docker image
docker:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 $(DOCKER_IMAGE)

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@swag init -g cmd/main.go

# Generate code
generate:
	@echo "Generating code..."
	@go generate ./...

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod download

# Cross-compile for multiple platforms
build-all:
	@echo "Cross-compiling for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 cmd/main.go
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 cmd/main.go
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe cmd/main.go

# Deploy to Kubernetes
deploy-k8s:
	@echo "Deploying to Kubernetes..."
	@kubectl apply -f deployments/kubernetes/

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  dev           - Start development server with hot reload"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  test-integration - Run integration tests"
	@echo "  lint          - Lint code"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker        - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  swagger       - Generate Swagger documentation"
	@echo "  generate      - Generate code"
	@echo "  install       - Install dependencies"
	@echo "  build-all     - Cross-compile for multiple platforms"
	@echo "  deploy-k8s    - Deploy to Kubernetes"
	@echo "  help          - Show this help message"
`

// configTemplate 配置文件模板
const configTemplate = `# {{.Project.Name}} Configuration
# Author: {{.Project.Author}}
# Created: {{.Timestamp}}

server:
  port: 8080
  mode: debug  # debug, release, test
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s

log:
  level: info  # debug, info, warn, error
  format: json  # json, text
  output: stdout  # stdout, stderr, file
  file: logs/app.log

{{- if ne .Project.Database ""}}
database:
  type: {{.Project.Database}}
  {{- if eq .Project.Database "postgres"}}
  host: localhost
  port: 5432
  user: postgres
  password: password
  dbname: {{.Project.Name}}
  sslmode: disable
  {{- else if eq .Project.Database "mysql"}}
  host: localhost
  port: 3306
  user: root
  password: password
  dbname: {{.Project.Name}}
  charset: utf8mb4
  {{- else if eq .Project.Database "sqlite"}}
  path: data/{{.Project.Name}}.db
  {{- else if eq .Project.Database "mongodb"}}
  uri: mongodb://localhost:27017
  database: {{.Project.Name}}
  {{- end}}
  max_open_conns: 100
  max_idle_conns: 10
  conn_max_lifetime: 1h
{{- end}}

{{- if ne .Project.Cache ""}}
cache:
  type: {{.Project.Cache}}
  {{- if eq .Project.Cache "redis"}}
  addr: localhost:6379
  password: ""
  db: 0
  {{- else if eq .Project.Cache "memcached"}}
  servers:
    - localhost:11211
  {{- end}}
  ttl: 1h
{{- end}}

{{- if ne .Project.Auth ""}}
auth:
  type: {{.Project.Auth}}
  {{- if eq .Project.Auth "jwt"}}
  secret: your-secret-key-here
  expires: 24h
  issuer: {{.Project.Name}}
  {{- else if eq .Project.Auth "oauth2"}}
  client_id: your-client-id
  client_secret: your-client-secret
  redirect_url: http://localhost:8080/auth/callback
  {{- end}}
{{- end}}

{{- if contains .Project.Features "metrics"}}
metrics:
  enabled: true
  path: /metrics
  namespace: {{.Project.Name}}
{{- end}}

{{- if contains .Project.Features "tracing"}}
tracing:
  enabled: true
  jaeger:
    endpoint: http://localhost:14268/api/traces
    service_name: {{.Project.Name}}
{{- end}}

{{- if contains .Project.Features "swagger"}}
swagger:
  enabled: true
  path: /swagger
  title: {{.Project.Name}} API
  version: {{.Project.Version}}
{{- end}}

cors:
  enabled: true
  allowed_origins:
    - "*"
  allowed_methods:
    - GET
    - POST
    - PUT
    - DELETE
    - OPTIONS
  allowed_headers:
    - "*"
  max_age: 12h

rate_limit:
  enabled: true
  requests_per_minute: 100
  burst: 10
`

// 代码生成模板

// handlerTemplate 处理器模板
const handlerTemplate = `// {{.Package}} {{.Name}} handler
// Generated by NetCore-Go CLI

package {{.Package}}

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// {{.Name}}Handler {{.Name}} 处理器
type {{.Name}}Handler struct {
	// 添加依赖项
}

// New{{.Name}}Handler 创建 {{.Name}} 处理器
func New{{.Name}}Handler() *{{.Name}}Handler {
	return &{{.Name}}Handler{}
}

// Get{{.Name}} 获取{{.Name}}
// @Summary Get {{.Name}}
// @Description Get {{.Name}} by ID
// @Tags {{.LowerName}}
// @Accept json
// @Produce json
// @Param id path int true "{{.Name}} ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /{{.LowerName}}/{id} [get]
func (h *{{.Name}}Handler) Get{{.Name}}(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// TODO: 实现获取{{.Name}}的逻辑
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "{{.Name}} retrieved successfully",
	})
}

// Create{{.Name}} 创建{{.Name}}
// @Summary Create {{.Name}}
// @Description Create a new {{.Name}}
// @Tags {{.LowerName}}
// @Accept json
// @Produce json
// @Param {{.LowerName}} body map[string]interface{} true "{{.Name}} data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /{{.LowerName}} [post]
func (h *{{.Name}}Handler) Create{{.Name}}(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: 实现创建{{.Name}}的逻辑
	c.JSON(http.StatusCreated, gin.H{
		"message": "{{.Name}} created successfully",
		"data":    req,
	})
}

// Update{{.Name}} 更新{{.Name}}
// @Summary Update {{.Name}}
// @Description Update {{.Name}} by ID
// @Tags {{.LowerName}}
// @Accept json
// @Produce json
// @Param id path int true "{{.Name}} ID"
// @Param {{.LowerName}} body map[string]interface{} true "{{.Name}} data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /{{.LowerName}}/{id} [put]
func (h *{{.Name}}Handler) Update{{.Name}}(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: 实现更新{{.Name}}的逻辑
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "{{.Name}} updated successfully",
		"data":    req,
	})
}

// Delete{{.Name}} 删除{{.Name}}
// @Summary Delete {{.Name}}
// @Description Delete {{.Name}} by ID
// @Tags {{.LowerName}}
// @Accept json
// @Produce json
// @Param id path int true "{{.Name}} ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /{{.LowerName}}/{id} [delete]
func (h *{{.Name}}Handler) Delete{{.Name}}(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// TODO: 实现删除{{.Name}}的逻辑
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "{{.Name}} deleted successfully",
	})
}

// List{{.Name}}s 列出{{.Name}}s
// @Summary List {{.Name}}s
// @Description Get list of {{.Name}}s
// @Tags {{.LowerName}}
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Router /{{.LowerName}} [get]
func (h *{{.Name}}Handler) List{{.Name}}s(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// TODO: 实现列出{{.Name}}s的逻辑
	c.JSON(http.StatusOK, gin.H{
		"data": []map[string]interface{}{},
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": 0,
		},
	})
}
`

// modelTemplate 模型模板
const modelTemplate = `// {{.Package}} {{.Name}} model
// Generated by NetCore-Go CLI

package {{.Package}}

import (
	"time"
	"gorm.io/gorm"
)

// {{.Name}} {{.Name}} 模型
type {{.Name}} struct {
	ID        uint           \`json:"id" gorm:"primarykey"\`
	CreatedAt time.Time      \`json:"created_at"\`
	UpdatedAt time.Time      \`json:"updated_at"\`
	DeletedAt gorm.DeletedAt \`json:"deleted_at,omitempty" gorm:"index"\`
	
	// TODO: 添加字段
	{{- range .Fields}}
	// {{.}}
	{{- end}}
}

// TableName 返回表名
func ({{.Name}}) TableName() string {
	return "{{.LowerName}}s"
}

{{- if .CRUD}}
// {{.Name}}Repository {{.Name}} 仓库接口
type {{.Name}}Repository interface {
	Create({{.LowerName}} *{{.Name}}) error
	GetByID(id uint) (*{{.Name}}, error)
	Update({{.LowerName}} *{{.Name}}) error
	Delete(id uint) error
	List(offset, limit int) ([]*{{.Name}}, int64, error)
}

// {{.LowerName}}Repository {{.Name}} 仓库实现
type {{.LowerName}}Repository struct {
	db *gorm.DB
}

// New{{.Name}}Repository 创建 {{.Name}} 仓库
func New{{.Name}}Repository(db *gorm.DB) {{.Name}}Repository {
	return &{{.LowerName}}Repository{db: db}
}

// Create 创建{{.Name}}
func (r *{{.LowerName}}Repository) Create({{.LowerName}} *{{.Name}}) error {
	return r.db.Create({{.LowerName}}).Error
}

// GetByID 根据ID获取{{.Name}}
func (r *{{.LowerName}}Repository) GetByID(id uint) (*{{.Name}}, error) {
	var {{.LowerName}} {{.Name}}
	err := r.db.First(&{{.LowerName}}, id).Error
	if err != nil {
		return nil, err
	}
	return &{{.LowerName}}, nil
}

// Update 更新{{.Name}}
func (r *{{.LowerName}}Repository) Update({{.LowerName}} *{{.Name}}) error {
	return r.db.Save({{.LowerName}}).Error
}

// Delete 删除{{.Name}}
func (r *{{.LowerName}}Repository) Delete(id uint) error {
	return r.db.Delete(&{{.Name}}{}, id).Error
}

// List 列出{{.Name}}s
func (r *{{.LowerName}}Repository) List(offset, limit int) ([]*{{.Name}}, int64, error) {
	var {{.LowerName}}s []*{{.Name}}
	var total int64
	
	err := r.db.Model(&{{.Name}}{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	
	err = r.db.Offset(offset).Limit(limit).Find(&{{.LowerName}}s).Error
	if err != nil {
		return nil, 0, err
	}
	
	return {{.LowerName}}s, total, nil
}
{{- end}}
`

// serviceTemplate 服务模板
const serviceTemplate = `// {{.Package}} {{.Name}} service
// Generated by NetCore-Go CLI

package {{.Package}}

import (
	"context"
	"fmt"
)

// {{.Name}}Service {{.Name}} 服务接口
type {{.Name}}Service interface {
	// TODO: 定义服务方法
	Process(ctx context.Context, data interface{}) (interface{}, error)
}

// {{.LowerName}}Service {{.Name}} 服务实现
type {{.LowerName}}Service struct {
	// TODO: 添加依赖项
}

// New{{.Name}}Service 创建 {{.Name}} 服务
func New{{.Name}}Service() {{.Name}}Service {
	return &{{.LowerName}}Service{}
}

// Process 处理业务逻辑
func (s *{{.LowerName}}Service) Process(ctx context.Context, data interface{}) (interface{}, error) {
	// TODO: 实现业务逻辑
	return nil, fmt.Errorf("{{.Name}}Service.Process not implemented")
}
`

// middlewareTemplate 中间件模板
const middlewareTemplate = `// {{.Package}} {{.Name}} middleware
// Generated by NetCore-Go CLI

package {{.Package}}

import (
	"github.com/gin-gonic/gin"
)

// {{.Name}} {{.Name}} 中间件
func {{.Name}}() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 实现中间件逻辑
		
		// 继续处理请求
		c.Next()
		
		// 请求处理完成后的逻辑
	}
}
`

// testTemplate 测试模板
const testTemplate = `// {{.Package}} {{.Name}} tests
// Generated by NetCore-Go CLI

package {{.Package}}

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func Test{{.Name}}(t *testing.T) {
	// TODO: 实现测试
	t.Run("should pass", func(t *testing.T) {
		assert.True(t, true)
	})
}

func Test{{.Name}}_Integration(t *testing.T) {
	// TODO: 实现集成测试
	t.Run("integration test", func(t *testing.T) {
		t.Skip("Integration test not implemented")
	})
}

func Benchmark{{.Name}}(b *testing.B) {
	// TODO: 实现基准测试
	for i := 0; i < b.N; i++ {
		// 基准测试代码
	}
}
`