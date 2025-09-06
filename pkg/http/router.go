// Package http HTTP路由器实现
// Author: NetCore-Go Team
// Created: 2024

package http

import (
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// HTTPHandler HTTP处理器接口
type HTTPHandler interface {
	ServeHTTP(ctx *HTTPContext, resp *HTTPResponse)
}

// HTTPHandlerFunc HTTP处理器函数类型
type HTTPHandlerFunc func(ctx *HTTPContext, resp *HTTPResponse)

// ServeHTTP 实现HTTPHandler接口
func (f HTTPHandlerFunc) ServeHTTP(ctx *HTTPContext, resp *HTTPResponse) {
	f(ctx, resp)
}

// Route 路由定义
type Route struct {
	Method  string
	Path    string
	Handler HTTPHandler
	Params  map[string]string
}

// Router HTTP路由器
type Router struct {
	mu     sync.RWMutex
	routes map[string]map[string]*Route // method -> path -> route
	static map[string]string            // prefix -> directory
}

// NewRouter 创建新的路由器
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]map[string]*Route),
		static: make(map[string]string),
	}
}

// HandleFunc 注册处理函数
func (r *Router) HandleFunc(method, path string, handler HTTPHandlerFunc) {
	r.Handle(method, path, handler)
}

// Handle 注册处理器
func (r *Router) Handle(method, path string, handler HTTPHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes[method] == nil {
		r.routes[method] = make(map[string]*Route)
	}

	r.routes[method][path] = &Route{
		Method:  method,
		Path:    path,
		Handler: handler,
		Params:  make(map[string]string),
	}
}

// ServeStatic 提供静态文件服务
func (r *Router) ServeStatic(prefix, dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.static[prefix] = dir
}

// Match 匹配路由
func (r *Router) Match(method, path string) HTTPHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 首先检查静态文件
	for prefix, dir := range r.static {
		if strings.HasPrefix(path, prefix) {
			return r.createStaticHandler(prefix, dir, path)
		}
	}

	// 检查精确匹配
	if methodRoutes, ok := r.routes[method]; ok {
		if route, ok := methodRoutes[path]; ok {
			return route.Handler
		}

		// 检查参数化路由
		for routePath, route := range methodRoutes {
			if params := r.matchPath(routePath, path); params != nil {
				// 创建带参数的处理器
				return r.createParameterizedHandler(route.Handler, params)
			}
		}
	}

	return nil
}

// matchPath 匹配路径参数
func (r *Router) matchPath(routePath, requestPath string) map[string]string {
	routeSegments := strings.Split(strings.Trim(routePath, "/"), "/")
	requestSegments := strings.Split(strings.Trim(requestPath, "/"), "/")

	if len(routeSegments) != len(requestSegments) {
		return nil
	}

	params := make(map[string]string)
	for i, segment := range routeSegments {
		if strings.HasPrefix(segment, ":") {
			// 参数段
			paramName := segment[1:]
			params[paramName] = requestSegments[i]
		} else if segment != requestSegments[i] {
			// 不匹配
			return nil
		}
	}

	return params
}

// createParameterizedHandler 创建参数化处理器
func (r *Router) createParameterizedHandler(handler HTTPHandler, params map[string]string) HTTPHandler {
	return HTTPHandlerFunc(func(ctx *HTTPContext, resp *HTTPResponse) {
		// 将参数添加到上下文
		for key, value := range params {
			ctx.SetParam(key, value)
		}
		handler.ServeHTTP(ctx, resp)
	})
}

// createStaticHandler 创建静态文件处理器
func (r *Router) createStaticHandler(prefix, dir, requestPath string) HTTPHandler {
	return HTTPHandlerFunc(func(ctx *HTTPContext, resp *HTTPResponse) {
		// 移除前缀获取文件路径
		filePath := strings.TrimPrefix(requestPath, prefix)
		if filePath == "" {
			filePath = "index.html"
		}

		// 构建完整文件路径
		fullPath := filepath.Join(dir, filepath.Clean(filePath))

		// 安全检查：确保文件在指定目录内
		absDir, err := filepath.Abs(dir)
		if err != nil {
			r.sendError(resp, 500, "Internal Server Error")
			return
		}

		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			r.sendError(resp, 500, "Internal Server Error")
			return
		}

		if !strings.HasPrefix(absPath, absDir) {
			r.sendError(resp, 403, "Forbidden")
			return
		}

		// 检查文件是否存在
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				r.sendError(resp, 404, "Not Found")
			} else {
				r.sendError(resp, 500, "Internal Server Error")
			}
			return
		}

		// 如果是目录，尝试查找index.html
		if fileInfo.IsDir() {
			indexPath := filepath.Join(fullPath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				fullPath = indexPath
			} else {
				r.sendError(resp, 403, "Forbidden")
				return
			}
		}

		// 读取文件内容
		content, err := ioutil.ReadFile(fullPath)
		if err != nil {
			r.sendError(resp, 500, "Internal Server Error")
			return
		}

		// 设置Content-Type
		ext := filepath.Ext(fullPath)
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		resp.StatusCode = 200
		resp.StatusText = "OK"
		resp.Headers["Content-Type"] = contentType
		resp.Headers["Content-Length"] = strconv.Itoa(len(content))
		resp.Body = content
	})
}

// sendError 发送错误响应
func (r *Router) sendError(resp *HTTPResponse, statusCode int, statusText string) {
	body := fmt.Sprintf("%d %s", statusCode, statusText)
	resp.StatusCode = statusCode
	resp.StatusText = statusText
	resp.Headers["Content-Type"] = "text/plain"
	resp.Headers["Content-Length"] = strconv.Itoa(len(body))
	resp.Body = []byte(body)
}