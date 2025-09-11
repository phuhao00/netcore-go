// NetCore-Go CLI Templates
// Enhanced templates for code generation
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateType 模板类型
type TemplateType string

const (
	TemplateTypeService    TemplateType = "service"
	TemplateTypeHandler    TemplateType = "handler"
	TemplateTypeModel      TemplateType = "model"
	TemplateTypeRepository TemplateType = "repository"
	TemplateTypeMiddleware TemplateType = "middleware"
	TemplateTypeConfig     TemplateType = "config"
	TemplateTypeTest       TemplateType = "test"
	TemplateTypeDocker     TemplateType = "docker"
	TemplateTypeK8s        TemplateType = "k8s"
	TemplateTypeMakefile   TemplateType = "makefile"
	TemplateTypeCI         TemplateType = "ci"
)

// Template 模板定义
type Template struct {
	Name        string
	Type        TemplateType
	Description string
	Content     string
	FilePath    string
	Dependencies []string
}

// TemplateRegistry 模板注册表
type TemplateRegistry struct {
	templates map[string]*Template
}

// NewTemplateRegistry 创建模板注册表
func NewTemplateRegistry() *TemplateRegistry {
	registry := &TemplateRegistry{
		templates: make(map[string]*Template),
	}
	
	// 注册内置模板
	registry.registerBuiltinTemplates()
	
	return registry
}

// registerBuiltinTemplates 注册内置模板
func (tr *TemplateRegistry) registerBuiltinTemplates() {
	// 服务模板
	tr.Register(&Template{
		Name:        "rest-service",
		Type:        TemplateTypeService,
		Description: "RESTful API service with CRUD operations",
		Content:     restServiceTemplate,
		FilePath:    "pkg/service/{{.Name}}_service.go",
		Dependencies: []string{"model", "repository"},
	})
	
	tr.Register(&Template{
		Name:        "grpc-service",
		Type:        TemplateTypeService,
		Description: "gRPC service implementation",
		Content:     grpcServiceTemplate,
		FilePath:    "pkg/service/{{.Name}}_grpc_service.go",
		Dependencies: []string{"proto"},
	})
	
	// 处理器模板
	tr.Register(&Template{
		Name:        "http-handler",
		Type:        TemplateTypeHandler,
		Description: "HTTP request handler",
		Content:     httpHandlerTemplate,
		FilePath:    "pkg/handler/{{.Name}}_handler.go",
		Dependencies: []string{"service"},
	})
	
	tr.Register(&Template{
		Name:        "websocket-handler",
		Type:        TemplateTypeHandler,
		Description: "WebSocket connection handler",
		Content:     websocketHandlerTemplate,
		FilePath:    "pkg/handler/{{.Name}}_ws_handler.go",
	})
	
	// 模型模板
	tr.Register(&Template{
		Name:        "entity-model",
		Type:        TemplateTypeModel,
		Description: "Database entity model",
		Content:     entityModelTemplate,
		FilePath:    "pkg/model/{{.Name}}.go",
	})
	
	tr.Register(&Template{
		Name:        "dto-model",
		Type:        TemplateTypeModel,
		Description: "Data Transfer Object model",
		Content:     dtoModelTemplate,
		FilePath:    "pkg/dto/{{.Name}}_dto.go",
	})
	
	// 仓储模板
	tr.Register(&Template{
		Name:        "gorm-repository",
		Type:        TemplateTypeRepository,
		Description: "GORM-based repository implementation",
		Content:     gormRepositoryTemplate,
		FilePath:    "pkg/repository/{{.Name}}_repository.go",
		Dependencies: []string{"model"},
	})
	
	tr.Register(&Template{
		Name:        "mongo-repository",
		Type:        TemplateTypeRepository,
		Description: "MongoDB repository implementation",
		Content:     mongoRepositoryTemplate,
		FilePath:    "pkg/repository/{{.Name}}_mongo_repository.go",
		Dependencies: []string{"model"},
	})
	
	// 中间件模板
	tr.Register(&Template{
		Name:        "auth-middleware",
		Type:        TemplateTypeMiddleware,
		Description: "Authentication middleware",
		Content:     authMiddlewareTemplate,
		FilePath:    "pkg/middleware/auth.go",
	})
	
	tr.Register(&Template{
		Name:        "cors-middleware",
		Type:        TemplateTypeMiddleware,
		Description: "CORS middleware",
		Content:     corsMiddlewareTemplate,
		FilePath:    "pkg/middleware/cors.go",
	})
	
	tr.Register(&Template{
		Name:        "rate-limit-middleware",
		Type:        TemplateTypeMiddleware,
		Description: "Rate limiting middleware",
		Content:     rateLimitMiddlewareTemplate,
		FilePath:    "pkg/middleware/rate_limit.go",
	})
	
	// 配置模板
	tr.Register(&Template{
		Name:        "app-config",
		Type:        TemplateTypeConfig,
		Description: "Application configuration",
		Content:     appConfigTemplate,
		FilePath:    "pkg/config/config.go",
	})
	
	// 测试模板
	tr.Register(&Template{
		Name:        "unit-test",
		Type:        TemplateTypeTest,
		Description: "Unit test template",
		Content:     unitTestTemplate,
		FilePath:    "pkg/{{.Package}}/{{.Name}}_test.go",
	})
	
	tr.Register(&Template{
		Name:        "integration-test",
		Type:        TemplateTypeTest,
		Description: "Integration test template",
		Content:     integrationTestTemplate,
		FilePath:    "tests/integration/{{.Name}}_test.go",
	})
	
	// 部署模板
	tr.Register(&Template{
		Name:        "dockerfile",
		Type:        TemplateTypeDocker,
		Description: "Multi-stage Dockerfile",
		Content:     dockerfileTemplate,
		FilePath:    "Dockerfile",
	})
	
	tr.Register(&Template{
		Name:        "docker-compose",
		Type:        TemplateTypeDocker,
		Description: "Docker Compose configuration",
		Content:     dockerComposeTemplate,
		FilePath:    "docker-compose.yml",
	})
	
	tr.Register(&Template{
		Name:        "k8s-deployment",
		Type:        TemplateTypeK8s,
		Description: "Kubernetes deployment manifest",
		Content:     k8sDeploymentTemplate,
		FilePath:    "deployments/k8s/deployment.yaml",
	})
	
	tr.Register(&Template{
		Name:        "k8s-service",
		Type:        TemplateTypeK8s,
		Description: "Kubernetes service manifest",
		Content:     k8sServiceTemplate,
		FilePath:    "deployments/k8s/service.yaml",
	})
	
	// 构建模板
	tr.Register(&Template{
		Name:        "makefile",
		Type:        TemplateTypeMakefile,
		Description: "Project Makefile",
		Content:     makefileTemplate,
		FilePath:    "Makefile",
	})
	
	// CI/CD模板
	tr.Register(&Template{
		Name:        "github-actions",
		Type:        TemplateTypeCI,
		Description: "GitHub Actions workflow",
		Content:     githubActionsTemplate,
		FilePath:    ".github/workflows/ci.yml",
	})
	
	tr.Register(&Template{
		Name:        "gitlab-ci",
		Type:        TemplateTypeCI,
		Description: "GitLab CI configuration",
		Content:     gitlabCITemplate,
		FilePath:    ".gitlab-ci.yml",
	})
}

// Register 注册模板
func (tr *TemplateRegistry) Register(template *Template) {
	tr.templates[template.Name] = template
}

// Get 获取模板
func (tr *TemplateRegistry) Get(name string) (*Template, bool) {
	template, exists := tr.templates[name]
	return template, exists
}

// List 列出所有模板
func (tr *TemplateRegistry) List() []*Template {
	templates := make([]*Template, 0, len(tr.templates))
	for _, template := range tr.templates {
		templates = append(templates, template)
	}
	return templates
}

// ListByType 按类型列出模板
func (tr *TemplateRegistry) ListByType(templateType TemplateType) []*Template {
	templates := make([]*Template, 0)
	for _, template := range tr.templates {
		if template.Type == templateType {
			templates = append(templates, template)
		}
	}
	return templates
}

// GenerateFromTemplate 从模板生成文件
func (tr *TemplateRegistry) GenerateFromTemplate(templateName string, data interface{}, outputDir string) error {
	template, exists := tr.Get(templateName)
	if !exists {
		return fmt.Errorf("template '%s' not found", templateName)
	}
	
	// 解析文件路径模板
	filePathTmpl, err := template.Parse("filepath", template.FilePath)
	if err != nil {
		return fmt.Errorf("failed to parse file path template: %w", err)
	}
	
	var filePathBuf strings.Builder
	if err := filePathTmpl.Execute(&filePathBuf, data); err != nil {
		return fmt.Errorf("failed to execute file path template: %w", err)
	}
	
	filePath := filepath.Join(outputDir, filePathBuf.String())
	
	// 创建目录
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// 生成文件内容
	contentTmpl, err := template.Parse("content", template.Content)
	if err != nil {
		return fmt.Errorf("failed to parse content template: %w", err)
	}
	
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	if err := contentTmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute content template: %w", err)
	}
	
	return nil
}

// Parse 解析模板
func (t *Template) Parse(name, content string) (*template.Template, error) {
	return template.New(name).Parse(content)
}

// 模板内容定义

// REST服务模板
const restServiceTemplate = `// {{.Name}} Service
// Generated by NetCore-Go CLI
// Author: {{.Project.Author}}
// Created: {{.Timestamp}}

package service

import (
	"context"
	"fmt"

	"{{.Project.ModulePath}}/pkg/model"
	"{{.Project.ModulePath}}/pkg/repository"
)

// {{.Name}}Service {{.Name}}服务接口
type {{.Name}}Service interface {
	Create(ctx context.Context, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error)
	GetByID(ctx context.Context, id string) (*model.{{.Name}}, error)
	Update(ctx context.Context, id string, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*model.{{.Name}}, error)
}

// {{.NameLower}}Service {{.Name}}服务实现
type {{.NameLower}}Service struct {
	repo repository.{{.Name}}Repository
}

// New{{.Name}}Service 创建{{.Name}}服务
func New{{.Name}}Service(repo repository.{{.Name}}Repository) {{.Name}}Service {
	return &{{.NameLower}}Service{
		repo: repo,
	}
}

// Create 创建{{.Name}}
func (s *{{.NameLower}}Service) Create(ctx context.Context, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error) {
	if {{.NameLower}} == nil {
		return nil, fmt.Errorf("{{.NameLower}} cannot be nil")
	}
	
	// 业务逻辑验证
	if err := s.validate{{.Name}}({{.NameLower}}); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	
	return s.repo.Create(ctx, {{.NameLower}})
}

// GetByID 根据ID获取{{.Name}}
func (s *{{.NameLower}}Service) GetByID(ctx context.Context, id string) (*model.{{.Name}}, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}
	
	return s.repo.GetByID(ctx, id)
}

// Update 更新{{.Name}}
func (s *{{.NameLower}}Service) Update(ctx context.Context, id string, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}
	if {{.NameLower}} == nil {
		return nil, fmt.Errorf("{{.NameLower}} cannot be nil")
	}
	
	// 检查是否存在
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing {{.NameLower}}: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("{{.NameLower}} not found")
	}
	
	// 业务逻辑验证
	if err := s.validate{{.Name}}({{.NameLower}}); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	
	return s.repo.Update(ctx, id, {{.NameLower}})
}

// Delete 删除{{.Name}}
func (s *{{.NameLower}}Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	
	// 检查是否存在
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get existing {{.NameLower}}: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("{{.NameLower}} not found")
	}
	
	return s.repo.Delete(ctx, id)
}

// List 列出{{.Name}}
func (s *{{.NameLower}}Service) List(ctx context.Context, limit, offset int) ([]*model.{{.Name}}, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	
	return s.repo.List(ctx, limit, offset)
}

// validate{{.Name}} 验证{{.Name}}数据
func (s *{{.NameLower}}Service) validate{{.Name}}({{.NameLower}} *model.{{.Name}}) error {
	// TODO: 添加业务逻辑验证
	return nil
}
`

// HTTP处理器模板
const httpHandlerTemplate = `// {{.Name}} HTTP Handler
// Generated by NetCore-Go CLI
// Author: {{.Project.Author}}
// Created: {{.Timestamp}}

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"{{.Project.ModulePath}}/pkg/service"
	"{{.Project.ModulePath}}/pkg/model"
)

// {{.Name}}Handler {{.Name}}HTTP处理器
type {{.Name}}Handler struct {
	service service.{{.Name}}Service
}

// New{{.Name}}Handler 创建{{.Name}}处理器
func New{{.Name}}Handler(service service.{{.Name}}Service) *{{.Name}}Handler {
	return &{{.Name}}Handler{
		service: service,
	}
}

// RegisterRoutes 注册路由
func (h *{{.Name}}Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/{{.NameLower}}s", h.Create{{.Name}}).Methods("POST")
	router.HandleFunc("/{{.NameLower}}s/{id}", h.Get{{.Name}}).Methods("GET")
	router.HandleFunc("/{{.NameLower}}s/{id}", h.Update{{.Name}}).Methods("PUT")
	router.HandleFunc("/{{.NameLower}}s/{id}", h.Delete{{.Name}}).Methods("DELETE")
	router.HandleFunc("/{{.NameLower}}s", h.List{{.Name}}s).Methods("GET")
}

// Create{{.Name}} 创建{{.Name}}
func (h *{{.Name}}Handler) Create{{.Name}}(w http.ResponseWriter, r *http.Request) {
	var {{.NameLower}} model.{{.Name}}
	if err := json.NewDecoder(r.Body).Decode(&{{.NameLower}}); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	created, err := h.service.Create(r.Context(), &{{.NameLower}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// Get{{.Name}} 获取{{.Name}}
func (h *{{.Name}}Handler) Get{{.Name}}(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	{{.NameLower}}, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode({{.NameLower}})
}

// Update{{.Name}} 更新{{.Name}}
func (h *{{.Name}}Handler) Update{{.Name}}(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	var {{.NameLower}} model.{{.Name}}
	if err := json.NewDecoder(r.Body).Decode(&{{.NameLower}}); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	updated, err := h.service.Update(r.Context(), id, &{{.NameLower}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// Delete{{.Name}} 删除{{.Name}}
func (h *{{.Name}}Handler) Delete{{.Name}}(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	if err := h.service.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// List{{.Name}}s 列出{{.Name}}
func (h *{{.Name}}Handler) List{{.Name}}s(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	
	if limit <= 0 {
		limit = 10
	}
	
	{{.NameLower}}s, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": {{.NameLower}}s,
		"total": len({{.NameLower}}s),
	})
}
`

// 实体模型模板
const entityModelTemplate = `// {{.Name}} Model
// Generated by NetCore-Go CLI
// Author: {{.Project.Author}}
// Created: {{.Timestamp}}

package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// {{.Name}} {{.Name}}实体
type {{.Name}} struct {
	ID        string    ` + "`json:\"id\" gorm:\"primaryKey;type:uuid;default:gen_random_uuid()\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\" gorm:\"autoCreateTime\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\" gorm:\"autoUpdateTime\"`" + `
	DeletedAt gorm.DeletedAt ` + "`json:\"-\" gorm:\"index\"`" + `
	
	// TODO: 添加具体字段
	Name        string ` + "`json:\"name\" gorm:\"not null\"`" + `
	Description string ` + "`json:\"description\"`" + `
	Status      string ` + "`json:\"status\" gorm:\"default:'active'\"`" + `
}

// BeforeCreate GORM钩子：创建前
func ({{.NameLower}} *{{.Name}}) BeforeCreate(tx *gorm.DB) error {
	if {{.NameLower}}.ID == "" {
		{{.NameLower}}.ID = uuid.New().String()
	}
	return nil
}

// TableName 表名
func ({{.Name}}) TableName() string {
	return "{{.TableName}}"
}

// IsActive 检查是否激活
func ({{.NameLower}} *{{.Name}}) IsActive() bool {
	return {{.NameLower}}.Status == "active"
}

// Activate 激活
func ({{.NameLower}} *{{.Name}}) Activate() {
	{{.NameLower}}.Status = "active"
}

// Deactivate 停用
func ({{.NameLower}} *{{.Name}}) Deactivate() {
	{{.NameLower}}.Status = "inactive"
}
`

// GORM仓储模板
const gormRepositoryTemplate = `// {{.Name}} Repository
// Generated by NetCore-Go CLI
// Author: {{.Project.Author}}
// Created: {{.Timestamp}}

package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"{{.Project.ModulePath}}/pkg/model"
)

// {{.Name}}Repository {{.Name}}仓储接口
type {{.Name}}Repository interface {
	Create(ctx context.Context, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error)
	GetByID(ctx context.Context, id string) (*model.{{.Name}}, error)
	Update(ctx context.Context, id string, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*model.{{.Name}}, error)
	Count(ctx context.Context) (int64, error)
}

// {{.NameLower}}Repository {{.Name}}仓储实现
type {{.NameLower}}Repository struct {
	db *gorm.DB
}

// New{{.Name}}Repository 创建{{.Name}}仓储
func New{{.Name}}Repository(db *gorm.DB) {{.Name}}Repository {
	return &{{.NameLower}}Repository{
		db: db,
	}
}

// Create 创建{{.Name}}
func (r *{{.NameLower}}Repository) Create(ctx context.Context, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error) {
	if err := r.db.WithContext(ctx).Create({{.NameLower}}).Error; err != nil {
		return nil, fmt.Errorf("failed to create {{.NameLower}}: %w", err)
	}
	return {{.NameLower}}, nil
}

// GetByID 根据ID获取{{.Name}}
func (r *{{.NameLower}}Repository) GetByID(ctx context.Context, id string) (*model.{{.Name}}, error) {
	var {{.NameLower}} model.{{.Name}}
	if err := r.db.WithContext(ctx).First(&{{.NameLower}}, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get {{.NameLower}}: %w", err)
	}
	return &{{.NameLower}}, nil
}

// Update 更新{{.Name}}
func (r *{{.NameLower}}Repository) Update(ctx context.Context, id string, {{.NameLower}} *model.{{.Name}}) (*model.{{.Name}}, error) {
	if err := r.db.WithContext(ctx).Model(&model.{{.Name}}{}).Where("id = ?", id).Updates({{.NameLower}}).Error; err != nil {
		return nil, fmt.Errorf("failed to update {{.NameLower}}: %w", err)
	}
	
	// 返回更新后的数据
	return r.GetByID(ctx, id)
}

// Delete 删除{{.Name}}
func (r *{{.NameLower}}Repository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.{{.Name}}{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete {{.NameLower}}: %w", err)
	}
	return nil
}

// List 列出{{.Name}}
func (r *{{.NameLower}}Repository) List(ctx context.Context, limit, offset int) ([]*model.{{.Name}}, error) {
	var {{.NameLower}}s []*model.{{.Name}}
	if err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&{{.NameLower}}s).Error; err != nil {
		return nil, fmt.Errorf("failed to list {{.NameLower}}s: %w", err)
	}
	return {{.NameLower}}s, nil
}

// Count 统计{{.Name}}数量
func (r *{{.NameLower}}Repository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.{{.Name}}{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count {{.NameLower}}s: %w", err)
	}
	return count, nil
}
`

// 其他模板内容...
const grpcServiceTemplate = `// gRPC Service template content here`
const websocketHandlerTemplate = `// WebSocket Handler template content here`
const dtoModelTemplate = `// DTO Model template content here`
const mongoRepositoryTemplate = `// MongoDB Repository template content here`
const authMiddlewareTemplate = `// Auth Middleware template content here`
const corsMiddlewareTemplate = `// CORS Middleware template content here`
const rateLimitMiddlewareTemplate = `// Rate Limit Middleware template content here`
const appConfigTemplate = `// App Config template content here`
const unitTestTemplate = `// Unit Test template content here`
const integrationTestTemplate = `// Integration Test template content here`
const dockerfileTemplate = `// Dockerfile template content here`
const dockerComposeTemplate = `// Docker Compose template content here`
const k8sDeploymentTemplate = `// K8s Deployment template content here`
const k8sServiceTemplate = `// K8s Service template content here`
const makefileTemplate = `// Makefile template content here`
const githubActionsTemplate = `// GitHub Actions template content here`
const gitlabCITemplate = `// GitLab CI template content here`
