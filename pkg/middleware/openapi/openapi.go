// Package openapi OpenAPI/Swagger文档中间件
// Author: NetCore-Go Team
// Created: 2024

package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

// OpenAPIMiddleware OpenAPI文档中间件
type OpenAPIMiddleware struct {
	mu       sync.RWMutex
	config   *OpenAPIConfig
	spec     *OpenAPISpec
	routes   map[string]*RouteInfo
	running  bool
}

// OpenAPIConfig OpenAPI配置
type OpenAPIConfig struct {
	// 基础信息
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`

	// 服务器信息
	Servers []Server `json:"servers,omitempty"`

	// 路径配置
	BasePath    string `json:"base_path"`
	DocsPath    string `json:"docs_path"`
	SpecPath    string `json:"spec_path"`
	UIPath      string `json:"ui_path"`

	// 安全配置
	Security []SecurityRequirement `json:"security,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"security_schemes,omitempty"`

	// 生成配置
	AutoGenerate    bool     `json:"auto_generate"`
	IncludePaths    []string `json:"include_paths"`
	ExcludePaths    []string `json:"exclude_paths"`
	IncludeMethods  []string `json:"include_methods"`
	ExcludeMethods  []string `json:"exclude_methods"`

	// UI配置
	SwaggerUIEnabled bool   `json:"swagger_ui_enabled"`
	SwaggerUIPath    string `json:"swagger_ui_path"`
	ReDocEnabled     bool   `json:"redoc_enabled"`
	ReDocPath        string `json:"redoc_path"`

	// 其他配置
	PrettyJSON      bool `json:"pretty_json"`
	CacheEnabled    bool `json:"cache_enabled"`
	CacheTTL        time.Duration `json:"cache_ttl"`
}

// Contact 联系信息
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License 许可证信息
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server 服务器信息
type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]*ServerVariable `json:"variables,omitempty"`
}

// ServerVariable 服务器变量
type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// SecurityRequirement 安全要求
type SecurityRequirement map[string][]string

// SecurityScheme 安全方案
type SecurityScheme struct {
	Type             string            `json:"type"`
	Description      string            `json:"description,omitempty"`
	Name             string            `json:"name,omitempty"`
	In               string            `json:"in,omitempty"`
	Scheme           string            `json:"scheme,omitempty"`
	BearerFormat     string            `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlows       `json:"flows,omitempty"`
	OpenIdConnectUrl string            `json:"openIdConnectUrl,omitempty"`
}

// OAuthFlows OAuth流程
type OAuthFlows struct {
	Implicit          *OAuthFlow `json:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
}

// OAuthFlow OAuth流程详情
type OAuthFlow struct {
	AuthorizationUrl string            `json:"authorizationUrl,omitempty"`
	TokenUrl         string            `json:"tokenUrl,omitempty"`
	RefreshUrl       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// OpenAPISpec OpenAPI规范
type OpenAPISpec struct {
	OpenAPI      string                 `json:"openapi"`
	Info         *Info                  `json:"info"`
	Servers      []Server               `json:"servers,omitempty"`
	Paths        map[string]*PathItem   `json:"paths"`
	Components   *Components            `json:"components,omitempty"`
	Security     []SecurityRequirement  `json:"security,omitempty"`
	Tags         []*Tag                 `json:"tags,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
}

// Info 信息对象
type Info struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Version        string   `json:"version"`
}

// PathItem 路径项
type PathItem struct {
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	Get         *Operation `json:"get,omitempty"`
	Put         *Operation `json:"put,omitempty"`
	Post        *Operation `json:"post,omitempty"`
	Delete      *Operation `json:"delete,omitempty"`
	Options     *Operation `json:"options,omitempty"`
	Head        *Operation `json:"head,omitempty"`
	Patch       *Operation `json:"patch,omitempty"`
	Trace       *Operation `json:"trace,omitempty"`
	Servers     []Server   `json:"servers,omitempty"`
	Parameters  []*Parameter `json:"parameters,omitempty"`
}

// Operation 操作对象
type Operation struct {
	Tags         []string              `json:"tags,omitempty"`
	Summary      string                `json:"summary,omitempty"`
	Description  string                `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
	OperationId  string                `json:"operationId,omitempty"`
	Parameters   []*Parameter          `json:"parameters,omitempty"`
	RequestBody  *RequestBody          `json:"requestBody,omitempty"`
	Responses    map[string]*Response  `json:"responses"`
	Callbacks    map[string]*Callback  `json:"callbacks,omitempty"`
	Deprecated   bool                  `json:"deprecated,omitempty"`
	Security     []SecurityRequirement `json:"security,omitempty"`
	Servers      []Server              `json:"servers,omitempty"`
}

// Parameter 参数对象
type Parameter struct {
	Name            string      `json:"name"`
	In              string      `json:"in"`
	Description     string      `json:"description,omitempty"`
	Required        bool        `json:"required,omitempty"`
	Deprecated      bool        `json:"deprecated,omitempty"`
	AllowEmptyValue bool        `json:"allowEmptyValue,omitempty"`
	Style           string      `json:"style,omitempty"`
	Explode         bool        `json:"explode,omitempty"`
	AllowReserved   bool        `json:"allowReserved,omitempty"`
	Schema          *Schema     `json:"schema,omitempty"`
	Example         interface{} `json:"example,omitempty"`
	Examples        map[string]*Example `json:"examples,omitempty"`
}

// RequestBody 请求体对象
type RequestBody struct {
	Description string                `json:"description,omitempty"`
	Content     map[string]*MediaType `json:"content"`
	Required    bool                  `json:"required,omitempty"`
}

// Response 响应对象
type Response struct {
	Description string                `json:"description"`
	Headers     map[string]*Header    `json:"headers,omitempty"`
	Content     map[string]*MediaType `json:"content,omitempty"`
	Links       map[string]*Link      `json:"links,omitempty"`
}

// MediaType 媒体类型对象
type MediaType struct {
	Schema   *Schema             `json:"schema,omitempty"`
	Example  interface{}         `json:"example,omitempty"`
	Examples map[string]*Example `json:"examples,omitempty"`
	Encoding map[string]*Encoding `json:"encoding,omitempty"`
}

// Schema 模式对象
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Default              interface{}        `json:"default,omitempty"`
	Example              interface{}        `json:"example,omitempty"`
	Enum                 []interface{}      `json:"enum,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	ExclusiveMinimum     bool               `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum     bool               `json:"exclusiveMaximum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	MinItems             *int               `json:"minItems,omitempty"`
	MaxItems             *int               `json:"maxItems,omitempty"`
	UniqueItems          bool               `json:"uniqueItems,omitempty"`
	MinProperties        *int               `json:"minProperties,omitempty"`
	MaxProperties        *int               `json:"maxProperties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	AdditionalProperties interface{}        `json:"additionalProperties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AllOf                []*Schema          `json:"allOf,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	AnyOf                []*Schema          `json:"anyOf,omitempty"`
	Not                  *Schema            `json:"not,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	ReadOnly             bool               `json:"readOnly,omitempty"`
	WriteOnly            bool               `json:"writeOnly,omitempty"`
	Deprecated           bool               `json:"deprecated,omitempty"`
}

// Components 组件对象
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	Responses       map[string]*Response       `json:"responses,omitempty"`
	Parameters      map[string]*Parameter      `json:"parameters,omitempty"`
	Examples        map[string]*Example        `json:"examples,omitempty"`
	RequestBodies   map[string]*RequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]*Header         `json:"headers,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]*Link           `json:"links,omitempty"`
	Callbacks       map[string]*Callback       `json:"callbacks,omitempty"`
}

// Example 示例对象
type Example struct {
	Summary       string      `json:"summary,omitempty"`
	Description   string      `json:"description,omitempty"`
	Value         interface{} `json:"value,omitempty"`
	ExternalValue string      `json:"externalValue,omitempty"`
}

// Header 头部对象
type Header struct {
	Description     string      `json:"description,omitempty"`
	Required        bool        `json:"required,omitempty"`
	Deprecated      bool        `json:"deprecated,omitempty"`
	AllowEmptyValue bool        `json:"allowEmptyValue,omitempty"`
	Style           string      `json:"style,omitempty"`
	Explode         bool        `json:"explode,omitempty"`
	AllowReserved   bool        `json:"allowReserved,omitempty"`
	Schema          *Schema     `json:"schema,omitempty"`
	Example         interface{} `json:"example,omitempty"`
	Examples        map[string]*Example `json:"examples,omitempty"`
}

// Link 链接对象
type Link struct {
	OperationRef string                 `json:"operationRef,omitempty"`
	OperationId  string                 `json:"operationId,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	RequestBody  interface{}            `json:"requestBody,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Server       *Server                `json:"server,omitempty"`
}

// Callback 回调对象
type Callback map[string]*PathItem

// Encoding 编码对象
type Encoding struct {
	ContentType   string             `json:"contentType,omitempty"`
	Headers       map[string]*Header `json:"headers,omitempty"`
	Style         string             `json:"style,omitempty"`
	Explode       bool               `json:"explode,omitempty"`
	AllowReserved bool               `json:"allowReserved,omitempty"`
}

// Tag 标签对象
type Tag struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
}

// ExternalDocumentation 外部文档对象
type ExternalDocumentation struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// RouteInfo 路由信息
type RouteInfo struct {
	Method      string
	Path        string
	Handler     interface{}
	Summary     string
	Description string
	Tags        []string
	Parameters  []*Parameter
	RequestBody *RequestBody
	Responses   map[string]*Response
	Security    []SecurityRequirement
	Deprecated  bool
}

// DefaultOpenAPIConfig 返回默认OpenAPI配置
func DefaultOpenAPIConfig() *OpenAPIConfig {
	return &OpenAPIConfig{
		Title:       "NetCore-Go API",
		Description: "High-performance network server API documentation",
		Version:     "1.0.0",
		Contact: &Contact{
			Name:  "NetCore-Go Team",
			Email: "team@netcore-go.dev",
			URL:   "https://netcore-go.dev",
		},
		License: &License{
			Name: "MIT",
			URL:  "https://opensource.org/licenses/MIT",
		},
		Servers: []Server{
			{
				URL:         "http://localhost:8080",
				Description: "Development server",
			},
		},
		BasePath:         "/api",
		DocsPath:         "/docs",
		SpecPath:         "/openapi.json",
		UIPath:           "/swagger",
		AutoGenerate:     true,
		IncludeMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		SwaggerUIEnabled: true,
		SwaggerUIPath:    "/swagger-ui",
		ReDocEnabled:     true,
		ReDocPath:        "/redoc",
		PrettyJSON:       true,
		CacheEnabled:     true,
		CacheTTL:         5 * time.Minute,
		SecuritySchemes: map[string]*SecurityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "JWT Bearer token authentication",
			},
			"apiKey": {
				Type:        "apiKey",
				In:          "header",
				Name:        "X-API-Key",
				Description: "API key authentication",
			},
		},
	}
}

// NewOpenAPIMiddleware 创建OpenAPI中间件
func NewOpenAPIMiddleware(config *OpenAPIConfig) *OpenAPIMiddleware {
	if config == nil {
		config = DefaultOpenAPIConfig()
	}

	return &OpenAPIMiddleware{
		config: config,
		spec:   &OpenAPISpec{},
		routes: make(map[string]*RouteInfo),
	}
}

// Start 启动中间件
func (o *OpenAPIMiddleware) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running {
		return fmt.Errorf("OpenAPI middleware is already running")
	}

	// 初始化OpenAPI规范
	o.initializeSpec()

	o.running = true
	return nil
}

// Stop 停止中间件
func (o *OpenAPIMiddleware) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.running {
		return nil
	}

	o.running = false
	return nil
}

// Middleware 返回HTTP中间件函数
func (o *OpenAPIMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否是文档相关路径
			if o.handleDocsPaths(w, r) {
				return
			}

			// 如果启用自动生成，记录路由信息
			if o.config.AutoGenerate {
				o.recordRoute(r)
			}

			// 继续处理请求
			next.ServeHTTP(w, r)
		})
	}
}

// handleDocsPaths 处理文档相关路径
func (o *OpenAPIMiddleware) handleDocsPaths(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case o.config.SpecPath:
		o.handleSpec(w, r)
		return true
	case o.config.SwaggerUIPath:
		if o.config.SwaggerUIEnabled {
			o.handleSwaggerUI(w, r)
			return true
		}
	case o.config.ReDocPath:
		if o.config.ReDocEnabled {
			o.handleReDoc(w, r)
			return true
		}
	case o.config.DocsPath:
		o.handleDocs(w, r)
		return true
	}
	return false
}

// handleSpec 处理OpenAPI规范请求
func (o *OpenAPIMiddleware) handleSpec(w http.ResponseWriter, r *http.Request) {
	o.mu.RLock()
	spec := o.generateSpec()
	o.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var data []byte
	var err error

	if o.config.PrettyJSON {
		data, err = json.MarshalIndent(spec, "", "  ")
	} else {
		data, err = json.Marshal(spec)
	}

	if err != nil {
		http.Error(w, "Failed to generate OpenAPI spec", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

// handleSwaggerUI 处理Swagger UI请求
func (o *OpenAPIMiddleware) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s - Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui.css" />
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin:0; background: #fafafa; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: '%s',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`, o.config.Title, o.config.SpecPath)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// handleReDoc 处理ReDoc请求
func (o *OpenAPIMiddleware) handleReDoc(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s - ReDoc</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
    <style>
        body { margin: 0; padding: 0; }
    </style>
</head>
<body>
    <redoc spec-url='%s'></redoc>
    <script src="https://cdn.jsdelivr.net/npm/redoc@2.0.0/bundles/redoc.standalone.js"></script>
</body>
</html>`, o.config.Title, o.config.SpecPath)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// handleDocs 处理文档首页请求
func (o *OpenAPIMiddleware) handleDocs(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s - API Documentation</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007acc; padding-bottom: 10px; }
        .links { margin: 30px 0; }
        .link { display: inline-block; margin: 10px 15px 10px 0; padding: 12px 24px; background: #007acc; color: white; text-decoration: none; border-radius: 4px; transition: background 0.3s; }
        .link:hover { background: #005a9e; }
        .description { margin: 20px 0; line-height: 1.6; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s</h1>
        <p class="description">%s</p>
        <div class="links">
            <a href="%s" class="link">OpenAPI Specification</a>
            %s
            %s
        </div>
        <p><strong>Version:</strong> %s</p>
        %s
    </div>
</body>
</html>`,
		o.config.Title,
		o.config.Title,
		o.config.Description,
		o.config.SpecPath,
		o.getSwaggerUILink(),
		o.getReDocLink(),
		o.config.Version,
		o.getContactInfo())

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// getSwaggerUILink 获取Swagger UI链接
func (o *OpenAPIMiddleware) getSwaggerUILink() string {
	if o.config.SwaggerUIEnabled {
		return fmt.Sprintf(`<a href="%s" class="link">Swagger UI</a>`, o.config.SwaggerUIPath)
	}
	return ""
}

// getReDocLink 获取ReDoc链接
func (o *OpenAPIMiddleware) getReDocLink() string {
	if o.config.ReDocEnabled {
		return fmt.Sprintf(`<a href="%s" class="link">ReDoc</a>`, o.config.ReDocPath)
	}
	return ""
}

// getContactInfo 获取联系信息
func (o *OpenAPIMiddleware) getContactInfo() string {
	if o.config.Contact != nil {
		contact := "<p><strong>Contact:</strong> "
		if o.config.Contact.Name != "" {
			contact += o.config.Contact.Name
		}
		if o.config.Contact.Email != "" {
			contact += fmt.Sprintf(" (<a href=\"mailto:%s\">%s</a>)", o.config.Contact.Email, o.config.Contact.Email)
		}
		if o.config.Contact.URL != "" {
			contact += fmt.Sprintf(" - <a href=\"%s\" target=\"_blank\">%s</a>", o.config.Contact.URL, o.config.Contact.URL)
		}
		contact += "</p>"
		return contact
	}
	return ""
}

// recordRoute 记录路由信息
func (o *OpenAPIMiddleware) recordRoute(r *http.Request) {
	if !o.shouldIncludeRoute(r) {
		return
	}

	routeKey := fmt.Sprintf("%s %s", r.Method, r.URL.Path)

	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.routes[routeKey]; !exists {
		o.routes[routeKey] = &RouteInfo{
			Method: r.Method,
			Path:   r.URL.Path,
			Responses: map[string]*Response{
				"200": {
					Description: "Successful response",
				},
			},
		}
	}
}

// shouldIncludeRoute 检查是否应该包含路由
func (o *OpenAPIMiddleware) shouldIncludeRoute(r *http.Request) bool {
	// 检查方法
	if len(o.config.IncludeMethods) > 0 {
		found := false
		for _, method := range o.config.IncludeMethods {
			if method == r.Method {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, method := range o.config.ExcludeMethods {
		if method == r.Method {
			return false
		}
	}

	// 检查路径
	for _, excludePath := range o.config.ExcludePaths {
		if strings.HasPrefix(r.URL.Path, excludePath) {
			return false
		}
	}

	if len(o.config.IncludePaths) > 0 {
		found := false
		for _, includePath := range o.config.IncludePaths {
			if strings.HasPrefix(r.URL.Path, includePath) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// initializeSpec 初始化OpenAPI规范
func (o *OpenAPIMiddleware) initializeSpec() {
	o.spec = &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: &Info{
			Title:       o.config.Title,
			Description: o.config.Description,
			Version:     o.config.Version,
			Contact:     o.config.Contact,
			License:     o.config.License,
		},
		Servers:  o.config.Servers,
		Paths:    make(map[string]*PathItem),
		Security: o.config.Security,
	}

	if len(o.config.SecuritySchemes) > 0 {
		o.spec.Components = &Components{
			SecuritySchemes: o.config.SecuritySchemes,
		}
	}
}

// generateSpec 生成OpenAPI规范
func (o *OpenAPIMiddleware) generateSpec() *OpenAPISpec {
	spec := *o.spec
	spec.Paths = make(map[string]*PathItem)

	// 从记录的路由生成路径
	for _, route := range o.routes {
		pathItem, exists := spec.Paths[route.Path]
		if !exists {
			pathItem = &PathItem{}
			spec.Paths[route.Path] = pathItem
		}

		operation := &Operation{
			Summary:     route.Summary,
			Description: route.Description,
			Tags:        route.Tags,
			Parameters:  route.Parameters,
			RequestBody: route.RequestBody,
			Responses:   route.Responses,
			Security:    route.Security,
			Deprecated:  route.Deprecated,
		}

		switch strings.ToUpper(route.Method) {
		case "GET":
			pathItem.Get = operation
		case "POST":
			pathItem.Post = operation
		case "PUT":
			pathItem.Put = operation
		case "DELETE":
			pathItem.Delete = operation
		case "PATCH":
			pathItem.Patch = operation
		case "OPTIONS":
			pathItem.Options = operation
		case "HEAD":
			pathItem.Head = operation
		case "TRACE":
			pathItem.Trace = operation
		}
	}

	return &spec
}

// AddRoute 手动添加路由
func (o *OpenAPIMiddleware) AddRoute(route *RouteInfo) {
	o.mu.Lock()
	defer o.mu.Unlock()

	routeKey := fmt.Sprintf("%s %s", route.Method, route.Path)
	o.routes[routeKey] = route
}

// RemoveRoute 移除路由
func (o *OpenAPIMiddleware) RemoveRoute(method, path string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	routeKey := fmt.Sprintf("%s %s", method, path)
	delete(o.routes, routeKey)
}

// GetRoutes 获取所有路由
func (o *OpenAPIMiddleware) GetRoutes() map[string]*RouteInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	routes := make(map[string]*RouteInfo)
	for key, route := range o.routes {
		routes[key] = route
	}
	return routes
}

// UpdateConfig 更新配置
func (o *OpenAPIMiddleware) UpdateConfig(config *OpenAPIConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.config = config
	o.initializeSpec()
}

// GetSpec 获取OpenAPI规范
func (o *OpenAPIMiddleware) GetSpec() *OpenAPISpec {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.generateSpec()
}

// IsRunning 检查是否运行
func (o *OpenAPIMiddleware) IsRunning() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.running
}

// 辅助函数

// SchemaFromType 从Go类型生成Schema
func SchemaFromType(t reflect.Type) *Schema {
	schema := &Schema{}

	switch t.Kind() {
	case reflect.String:
		schema.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema.Type = "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema.Type = "integer"
	case reflect.Float32, reflect.Float64:
		schema.Type = "number"
	case reflect.Bool:
		schema.Type = "boolean"
	case reflect.Array, reflect.Slice:
		schema.Type = "array"
		schema.Items = SchemaFromType(t.Elem())
	case reflect.Map:
		schema.Type = "object"
		schema.AdditionalProperties = SchemaFromType(t.Elem())
	case reflect.Struct:
		schema.Type = "object"
		schema.Properties = make(map[string]*Schema)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.IsExported() {
				fieldName := field.Name
				if jsonTag := field.Tag.Get("json"); jsonTag != "" {
					parts := strings.Split(jsonTag, ",")
					if parts[0] != "" && parts[0] != "-" {
						fieldName = parts[0]
					}
				}
				schema.Properties[fieldName] = SchemaFromType(field.Type)
			}
		}
	case reflect.Ptr:
		return SchemaFromType(t.Elem())
	default:
		schema.Type = "string"
	}

	return schema
}

// CreateJSONResponse 创建JSON响应
func CreateJSONResponse(description string, schema *Schema) *Response {
	return &Response{
		Description: description,
		Content: map[string]*MediaType{
			"application/json": {
				Schema: schema,
			},
		},
	}
}

// CreateJSONRequestBody 创建JSON请求体
func CreateJSONRequestBody(description string, schema *Schema, required bool) *RequestBody {
	return &RequestBody{
		Description: description,
		Required:    required,
		Content: map[string]*MediaType{
			"application/json": {
				Schema: schema,
			},
		},
	}
}