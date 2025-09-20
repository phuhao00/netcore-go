// Package http HTTP类型定义
// Author: NetCore-Go Team
// Created: 2024

package http

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/phuhao00/netcore-go/pkg/core"
)

// ServerConfig HTTP服务器配置
type ServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// HTTPMiddlewareWrapper 中间件包装器
type HTTPMiddlewareWrapper struct {
	middleware HTTPMiddleware
	name       string
	priority   int
}

// Name 返回中间件名称
func (w *HTTPMiddlewareWrapper) Name() string {
	return w.name
}

// Priority 返回中间件优先级
func (w *HTTPMiddlewareWrapper) Priority() int {
	return w.priority
}

// Process 处理消息
func (w *HTTPMiddlewareWrapper) Process(ctx core.Context, next core.Handler) error {
	// 这里需要根据实际的core.Middleware接口实现
	return next(ctx)
}

// HTTPRequest HTTP请求结构
type HTTPRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Version string            `json:"version"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
	Query   url.Values        `json:"query"`
}

// HTTPResponse HTTP响应结构
type HTTPResponse struct {
	Version    string            `json:"version"`
	StatusCode int               `json:"status_code"`
	StatusText string            `json:"status_text"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

// HTTPContext HTTP上下文
type HTTPContext struct {
	Request    *HTTPRequest
	Connection *HTTPConnection
	values     map[string]interface{}
	params     map[string]string
}

// NewHTTPContext 创建新的HTTP上下文
func NewHTTPContext(req *HTTPRequest, conn *HTTPConnection) *HTTPContext {
	// 解析查询参数
	if req.Query == nil {
		if idx := strings.Index(req.Path, "?"); idx != -1 {
			queryString := req.Path[idx+1:]
			req.Path = req.Path[:idx]
			var err error
			req.Query, err = url.ParseQuery(queryString)
			if err != nil {
				req.Query = make(url.Values)
			}
		} else {
			req.Query = make(url.Values)
		}
	}

	return &HTTPContext{
		Request:    req,
		Connection: conn,
		values:     make(map[string]interface{}),
		params:     make(map[string]string),
	}
}

// Set 设置上下文值
func (c *HTTPContext) Set(key string, value interface{}) {
	c.values[key] = value
}

// Get 获取上下文值
func (c *HTTPContext) Get(key string) (interface{}, bool) {
	value, exists := c.values[key]
	return value, exists
}

// SetParam 设置路径参数
func (c *HTTPContext) SetParam(key, value string) {
	c.params[key] = value
}

// Param 获取路径参数
func (c *HTTPContext) Param(key string) string {
	return c.params[key]
}

// Query 获取查询参数
func (c *HTTPContext) Query(key string) string {
	return c.Request.Query.Get(key)
}

// QueryDefault 获取查询参数（带默认值）
func (c *HTTPContext) QueryDefault(key, defaultValue string) string {
	value := c.Request.Query.Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Header 获取请求头
func (c *HTTPContext) Header(key string) string {
	return c.Request.Headers[key]
}

// Method 获取请求方法
func (c *HTTPContext) Method() string {
	return c.Request.Method
}

// Path 获取请求路径
func (c *HTTPContext) Path() string {
	return c.Request.Path
}

// Body 获取请求体
func (c *HTTPContext) Body() []byte {
	return c.Request.Body
}

// BindJSON 绑定JSON数据
func (c *HTTPContext) BindJSON(v interface{}) error {
	if len(c.Request.Body) == 0 {
		return fmt.Errorf("empty request body")
	}
	return json.Unmarshal(c.Request.Body, v)
}

// JSON 发送JSON响应
func (c *HTTPContext) JSON(resp *HTTPResponse, statusCode int, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp.StatusCode = statusCode
	resp.StatusText = getStatusText(statusCode)
	resp.Headers["Content-Type"] = "application/json"
	resp.Headers["Content-Length"] = strconv.Itoa(len(body))
	resp.Body = body

	return nil
}

// String 发送字符串响应
func (c *HTTPContext) String(resp *HTTPResponse, statusCode int, text string) {
	body := []byte(text)
	resp.StatusCode = statusCode
	resp.StatusText = getStatusText(statusCode)
	resp.Headers["Content-Type"] = "text/plain; charset=utf-8"
	resp.Headers["Content-Length"] = strconv.Itoa(len(body))
	resp.Body = body
}

// HTML 发送HTML响应
func (c *HTTPContext) HTML(resp *HTTPResponse, statusCode int, html string) {
	body := []byte(html)
	resp.StatusCode = statusCode
	resp.StatusText = getStatusText(statusCode)
	resp.Headers["Content-Type"] = "text/html; charset=utf-8"
	resp.Headers["Content-Length"] = strconv.Itoa(len(body))
	resp.Body = body
}

// Redirect 重定向
func (c *HTTPContext) Redirect(resp *HTTPResponse, statusCode int, location string) {
	resp.StatusCode = statusCode
	resp.StatusText = getStatusText(statusCode)
	resp.Headers["Location"] = location
	resp.Headers["Content-Length"] = "0"
	resp.Body = []byte{}
}

// Error 发送错误响应
func (c *HTTPContext) Error(resp *HTTPResponse, statusCode int, message string) {
	c.String(resp, statusCode, message)
}

// getStatusText 获取状态文本
func getStatusText(statusCode int) string {
	switch statusCode {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 409:
		return "Conflict"
	case 422:
		return "Unprocessable Entity"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		return "Unknown"
	}
}

// generateConnectionID 生成连接ID
func generateConnectionID() string {
	return fmt.Sprintf("http-%d-%d", time.Now().UnixNano(), rand.Int63())
}

// HTTPMiddleware HTTP中间件接口
type HTTPMiddleware interface {
	Handle(ctx *HTTPContext, resp *HTTPResponse, next HTTPHandler)
}

// RouterGroup 路由组
type RouterGroup struct {
	server *HTTPServer
	prefix string
}

// GET 注册GET路由
func (g *RouterGroup) GET(path string, handler HTTPHandlerFunc) {
	fullPath := g.prefix + path
	g.server.router.HandleFunc("GET", fullPath, handler)
}

// POST 注册POST路由
func (g *RouterGroup) POST(path string, handler HTTPHandlerFunc) {
	fullPath := g.prefix + path
	g.server.router.HandleFunc("POST", fullPath, handler)
}

// PUT 注册PUT路由
func (g *RouterGroup) PUT(path string, handler HTTPHandlerFunc) {
	fullPath := g.prefix + path
	g.server.router.HandleFunc("PUT", fullPath, handler)
}

// DELETE 注册DELETE路由
func (g *RouterGroup) DELETE(path string, handler HTTPHandlerFunc) {
	fullPath := g.prefix + path
	g.server.router.HandleFunc("DELETE", fullPath, handler)
}

// PATCH 注册PATCH路由
func (g *RouterGroup) PATCH(path string, handler HTTPHandlerFunc) {
	fullPath := g.prefix + path
	g.server.router.HandleFunc("PATCH", fullPath, handler)
}