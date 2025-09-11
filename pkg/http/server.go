// Package http HTTP服务器实现
// Author: NetCore-Go Team
// Created: 2024

package http

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/pool"
)

// HTTPServer HTTP服务器实现
type HTTPServer struct {
	mu          sync.RWMutex
	listener    net.Listener
	connections map[string]*HTTPConnection
	handler     core.MessageHandler
	middlewares []core.Middleware
	config      *core.ServerConfig
	ctx         context.Context
	cancel      context.CancelFunc
	stats       *core.ServerStats
	connPool    *pool.ConnectionPool
	memPool     *pool.MemoryPool
	goroutinePool *pool.GoroutinePool
	router      *Router
	running     int32
}

// NewHTTPServer 创建新的HTTP服务器
func NewHTTPServer(config *core.ServerConfig) *HTTPServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	server := &HTTPServer{
		connections: make(map[string]*HTTPConnection),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		stats: &core.ServerStats{
			StartTime: time.Now(),
		},
		router: NewRouter(),
	}

	// 初始化资源池
	// 资源池初始化将在后续版本中实现
	// if config.EnableConnectionPool {
	//     server.connPool = pool.NewConnectionPool(100, 1000)
	// }
	// if config.EnableMemoryPool {
	//     server.memPool = pool.NewMemoryPool(4096, 100)
	// }
	// if config.EnableGoroutinePool {
	//     server.goroutinePool = pool.NewGoroutinePool(1000)
	// }

	return server
}

// Start 启动HTTP服务器
func (s *HTTPServer) Start(addr string) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("server is already running")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		atomic.StoreInt32(&s.running, 0)
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	fmt.Printf("HTTP Server listening on %s\n", addr)

	// 启动连接接受循环
	go s.acceptLoop()

	// 启动心跳检测
	if s.config.EnableHeartbeat {
		go s.heartbeatLoop()
	}

	return nil
}

// Stop 停止HTTP服务器
func (s *HTTPServer) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return fmt.Errorf("server is not running")
	}

	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有连接
	s.mu.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.connections = make(map[string]*HTTPConnection)
	s.mu.Unlock()

	fmt.Println("HTTP Server stopped")
	return nil
}

// SetHandler 设置消息处理器
func (s *HTTPServer) SetHandler(handler core.MessageHandler) {
	s.handler = handler
}

// SetMiddleware 设置中间件
func (s *HTTPServer) SetMiddleware(middleware ...core.Middleware) {
	s.middlewares = middleware
}

// GetStats 获取服务器统计信息
func (s *HTTPServer) GetStats() *core.ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := *s.stats
	stats.ActiveConnections = int64(len(s.connections))
	stats.Uptime = int64(time.Since(s.stats.StartTime).Seconds())
	return &stats
}

// HandleFunc 注册HTTP处理函数
func (s *HTTPServer) HandleFunc(method, path string, handler HTTPHandlerFunc) {
	s.router.HandleFunc(method, path, handler)
}

// Handle 注册HTTP处理器
func (s *HTTPServer) Handle(method, path string, handler HTTPHandler) {
	s.router.Handle(method, path, handler)
}

// ServeStatic 提供静态文件服务
func (s *HTTPServer) ServeStatic(prefix, dir string) {
	s.router.ServeStatic(prefix, dir)
}

// acceptLoop 连接接受循环
func (s *HTTPServer) acceptLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		rawConn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			s.stats.LastError = err.Error()
			continue
		}

		// 检查连接数限制
		s.mu.RLock()
		connCount := len(s.connections)
		s.mu.RUnlock()

		if connCount >= s.config.MaxConnections {
			rawConn.Close()
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			s.stats.LastError = "max connections exceeded"
			continue
		}

		// 创建HTTP连接
		conn := NewHTTPConnection(rawConn, s)
		s.addConnection(conn)

		// 处理连接
		// if s.goroutinePool != nil {
		//     s.goroutinePool.Submit(func() {
		//         s.handleConnection(conn)
		//     })
		// } else {
			go s.handleConnection(conn)
		// }
	}
}

// handleConnection 处理HTTP连接
func (s *HTTPServer) handleConnection(conn *HTTPConnection) {
	defer s.removeConnection(conn)
	defer conn.Close()

	// 通知连接建立
	if s.handler != nil {
		s.handler.OnConnect(conn)
	}

	reader := bufio.NewReader(conn.rawConn)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 设置读取超时
		if s.config.ReadTimeout > 0 {
			conn.rawConn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		}

		// 解析HTTP请求
		req, err := s.parseHTTPRequest(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			s.stats.LastError = err.Error()
			return
		}

		atomic.AddInt64(&s.stats.MessagesReceived, 1)
		atomic.AddInt64(&s.stats.BytesReceived, int64(len(req.Body)))

		// 处理HTTP请求
		resp := s.handleHTTPRequest(conn, req)

		// 发送HTTP响应
		if err := s.sendHTTPResponse(conn, resp); err != nil {
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			s.stats.LastError = err.Error()
			return
		}

		atomic.AddInt64(&s.stats.MessagesSent, 1)
		atomic.AddInt64(&s.stats.BytesSent, int64(len(resp.Body)))

		// HTTP/1.0 或 Connection: close 则关闭连接
		if req.Version == "HTTP/1.0" || strings.ToLower(req.Headers["Connection"]) == "close" {
			return
		}
	}
}

// parseHTTPRequest 解析HTTP请求
func (s *HTTPServer) parseHTTPRequest(reader *bufio.Reader) (*HTTPRequest, error) {
	// 读取请求行
	requestLine, _, err := reader.ReadLine()
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(requestLine), " ")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid request line")
	}

	req := &HTTPRequest{
		Method:  parts[0],
		Path:    parts[1],
		Version: parts[2],
		Headers: make(map[string]string),
	}

	// 读取请求头
	for {
		headerLine, _, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}

		if len(headerLine) == 0 {
			break // 空行表示头部结束
		}

		headerStr := string(headerLine)
		colonIndex := strings.Index(headerStr, ":")
		if colonIndex == -1 {
			continue
		}

		key := strings.TrimSpace(headerStr[:colonIndex])
		value := strings.TrimSpace(headerStr[colonIndex+1:])
		req.Headers[key] = value
	}

	// 读取请求体
	if contentLengthStr, ok := req.Headers["Content-Length"]; ok {
		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			return nil, fmt.Errorf("invalid content length: %w", err)
		}

		if contentLength > 0 {
			body := make([]byte, contentLength)
			_, err := io.ReadFull(reader, body)
			if err != nil {
				return nil, fmt.Errorf("failed to read body: %w", err)
			}
			req.Body = body
		}
	}

	return req, nil
}

// handleHTTPRequest 处理HTTP请求
func (s *HTTPServer) handleHTTPRequest(conn *HTTPConnection, req *HTTPRequest) *HTTPResponse {
	// 创建HTTP上下文
	ctx := NewHTTPContext(req, conn)

	// 创建默认响应
	resp := &HTTPResponse{
		Version:    "HTTP/1.1",
		StatusCode: 200,
		StatusText: "OK",
		Headers:    make(map[string]string),
		Body:       []byte{},
	}

	// 设置默认头部
	resp.Headers["Server"] = "NetCore-Go/1.0"
	resp.Headers["Date"] = time.Now().UTC().Format(http.TimeFormat)
	resp.Headers["Connection"] = "keep-alive"

	// 执行中间件链
	handler := s.router.Match(req.Method, req.Path)
	if handler == nil {
		resp.StatusCode = 404
		resp.StatusText = "Not Found"
		resp.Body = []byte("404 Not Found")
		resp.Headers["Content-Type"] = "text/plain"
		resp.Headers["Content-Length"] = strconv.Itoa(len(resp.Body))
		return resp
	}

	// 执行处理器
	handler.ServeHTTP(ctx, resp)

	// 设置Content-Length
	if _, ok := resp.Headers["Content-Length"]; !ok {
		resp.Headers["Content-Length"] = strconv.Itoa(len(resp.Body))
	}

	return resp
}

// sendHTTPResponse 发送HTTP响应
func (s *HTTPServer) sendHTTPResponse(conn *HTTPConnection, resp *HTTPResponse) error {
	// 设置写入超时
	if s.config.WriteTimeout > 0 {
		conn.rawConn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
	}

	// 构建响应
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s %d %s\r\n", resp.Version, resp.StatusCode, resp.StatusText))

	// 写入响应头
	for key, value := range resp.Headers {
		response.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	// 空行分隔头部和体
	response.WriteString("\r\n")

	// 发送响应头
	if _, err := conn.rawConn.Write([]byte(response.String())); err != nil {
		return err
	}

	// 发送响应体
	if len(resp.Body) > 0 {
		if _, err := conn.rawConn.Write(resp.Body); err != nil {
			return err
		}
	}

	return nil
}

// addConnection 添加连接
func (s *HTTPServer) addConnection(conn *HTTPConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[conn.ID()] = conn
	atomic.AddInt64(&s.stats.TotalConnections, 1)
}

// removeConnection 移除连接
func (s *HTTPServer) removeConnection(conn *HTTPConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connections, conn.ID())

	// 通知连接断开
	if s.handler != nil {
		s.handler.OnDisconnect(conn, nil)
	}
}

// heartbeatLoop 心跳检测循环
func (s *HTTPServer) heartbeatLoop() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkIdleConnections()
		}
	}
}

// checkIdleConnections 检查空闲连接
func (s *HTTPServer) checkIdleConnections() {
	s.mu.RLock()
	connections := make([]*HTTPConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.RUnlock()

	now := time.Now()
	for _, conn := range connections {
		if now.Sub(conn.lastActive) > s.config.IdleTimeout {
			conn.Close()
		}
	}
}

