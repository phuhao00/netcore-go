// Package rpc RPC协议服务器实现
// Author: NetCore-Go Team
// Created: 2024

package rpc

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/netcore-go/pkg/core"
)

// RPCServer RPC服务器实现
type RPCServer struct {
	core.BaseServer
	services    map[string]*ServiceInfo
	serviceMu   sync.RWMutex
	registry    ServiceRegistry
	codec       Codec
	interceptor Interceptor
	listener    net.Listener
	running     bool
	handler     core.MessageHandler
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name     string
	Methods  map[string]*MethodInfo
	Receiver reflect.Value
	Type     reflect.Type
}

// MethodInfo 方法信息
type MethodInfo struct {
	Method   reflect.Method
	ArgType  reflect.Type
	ReplyType reflect.Type
}

// RPCRequest RPC请求
type RPCRequest struct {
	ID       string      `json:"id"`
	Service  string      `json:"service"`
	Method   string      `json:"method"`
	Args     interface{} `json:"args"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RPCResponse RPC响应
type RPCResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewRPCServer 创建RPC服务器
func NewRPCServer(opts ...core.ServerOption) *RPCServer {
	server := &RPCServer{
		BaseServer: *core.NewBaseServer(),
		services:   make(map[string]*ServiceInfo),
		codec:      &JSONCodec{},
	}
	
	// 应用配置选项
	for _, opt := range opts {
		opt(server.BaseServer.GetConfig())
	}
	
	// 设置默认配置
	config := server.BaseServer.GetConfig()
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = 4096
	}
	if config.WriteBufferSize == 0 {
		config.WriteBufferSize = 4096
	}
	if config.MaxConnections == 0 {
		config.MaxConnections = 1000
	}
	
	return server
}

// RegisterService 注册服务
func (s *RPCServer) RegisterService(name string, service interface{}) error {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()
	
	serviceType := reflect.TypeOf(service)
	serviceValue := reflect.ValueOf(service)
	
	if serviceType.Kind() != reflect.Ptr {
		return fmt.Errorf("service must be a pointer")
	}
	
	serviceInfo := &ServiceInfo{
		Name:     name,
		Methods:  make(map[string]*MethodInfo),
		Receiver: serviceValue,
		Type:     serviceType,
	}
	
	// 扫描方法
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		methodType := method.Type
		
		// 检查方法签名：func(receiver, context.Context, *args, *reply) error
		if methodType.NumIn() != 4 || methodType.NumOut() != 1 {
			continue
		}
		
		// 检查参数类型
		ctxType := methodType.In(1)
		argType := methodType.In(2)
		replyType := methodType.In(3)
		errorType := methodType.Out(0)
		
		if !ctxType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			continue
		}
		
		if argType.Kind() != reflect.Ptr || replyType.Kind() != reflect.Ptr {
			continue
		}
		
		if !errorType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			continue
		}
		
		serviceInfo.Methods[method.Name] = &MethodInfo{
			Method:    method,
			ArgType:   argType.Elem(),
			ReplyType: replyType.Elem(),
		}
	}
	
	s.services[name] = serviceInfo
	
	// 注册到服务注册中心
	if s.registry != nil {
		return s.registry.Register(name, serviceInfo)
	}
	
	return nil
}

// SetRegistry 设置服务注册中心
func (s *RPCServer) SetRegistry(registry ServiceRegistry) {
	s.registry = registry
}

// SetCodec 设置编解码器
func (s *RPCServer) SetCodec(codec Codec) {
	s.codec = codec
}

// SetInterceptor 设置拦截器
func (s *RPCServer) SetInterceptor(interceptor Interceptor) {
	s.interceptor = interceptor
}

// Start 启动RPC服务器
func (s *RPCServer) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	
	s.listener = listener
	s.running = true
	stats := s.BaseServer.GetStats()
	stats.StartTime = time.Now()
	
	// 启动连接池（暂时注释掉）
	// if s.config.EnableConnectionPool {
	//	s.ConnPool = pool.NewConnectionPool(pool.PoolConfig{
	//		InitialSize: 10,
	//		MaxSize:     s.config.MaxConnections,
	//		IdleTimeout: 5 * time.Minute,
	//	})
	// }
	
	// 启动内存池（暂时注释掉）
	// if s.config.EnableMemoryPool {
	//	s.MemPool = pool.NewMemoryPool(s.config.ReadBufferSize, 100)
	// }
	
	// 启动协程池（暂时注释掉）
	// if s.config.EnableGoroutinePool {
	//	s.GoroutinePool = pool.NewGoroutinePool(1000, 10000)
	// }
	
	go s.acceptLoop()
	
	return nil
}

// acceptLoop 接受连接循环
func (s *RPCServer) acceptLoop() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				stats := s.BaseServer.GetStats()
				stats.ErrorCount++
				stats.LastError = err.Error()
			}
			continue
		}
		
		// 检查连接数限制
		stats := s.BaseServer.GetStats()
		config := s.BaseServer.GetConfig()
		if stats.ActiveConnections >= int64(config.MaxConnections) {
			conn.Close()
			continue
		}
		
		// 创建RPC连接
		rpcConn := NewRPCConnection(conn, s)
		
		// 使用协程池处理连接（暂时注释掉）
		// if s.GoroutinePool != nil {
		//	s.GoroutinePool.Submit(func() {
		//		s.handleConnection(rpcConn)
		//	})
		// } else {
			go s.handleConnection(rpcConn)
		// }
	}
}

// handleConnection 处理RPC连接
func (s *RPCServer) handleConnection(conn *RPCConnection) {
	defer func() {
		if r := recover(); r != nil {
			stats := s.BaseServer.GetStats()
			stats.ErrorCount++
			stats.LastError = fmt.Sprintf("panic: %v", r)
		}
		conn.Close()
	}()
	
	stats := s.BaseServer.GetStats()
	stats.ActiveConnections++
	stats.TotalConnections++
	defer func() {
		stats.ActiveConnections--
	}()
	
	// 调用连接处理器
	if s.handler != nil {
		s.handler.OnConnect(conn)
		defer s.handler.OnDisconnect(conn, nil)
	}
	
	// 处理RPC请求
	for {
		request, err := conn.ReadRequest()
		if err != nil {
			if s.handler != nil {
				s.handler.OnDisconnect(conn, err)
			}
			break
		}
		
		stats := s.BaseServer.GetStats()
		stats.MessagesReceived++
		
		// 处理请求
		response := s.handleRequest(conn, request)
		
		// 发送响应
		if err := conn.WriteResponse(response); err != nil {
			stats.ErrorCount++
			stats.LastError = err.Error()
			break
		}
		
		stats.MessagesSent++
	}
}

// handleRequest 处理RPC请求
func (s *RPCServer) handleRequest(conn *RPCConnection, request *RPCRequest) *RPCResponse {
	response := &RPCResponse{
		ID: request.ID,
		Metadata: make(map[string]string),
	}
	
	// 查找服务
	s.serviceMu.RLock()
	serviceInfo, exists := s.services[request.Service]
	s.serviceMu.RUnlock()
	
	if !exists {
		response.Error = fmt.Sprintf("service not found: %s", request.Service)
		return response
	}
	
	// 查找方法
	methodInfo, exists := serviceInfo.Methods[request.Method]
	if !exists {
		response.Error = fmt.Sprintf("method not found: %s.%s", request.Service, request.Method)
		return response
	}
	
	// 创建参数和返回值
	argValue := reflect.New(methodInfo.ArgType)
	replyValue := reflect.New(methodInfo.ReplyType)
	
	// 解码参数
	if err := s.codec.Decode(request.Args, argValue.Interface()); err != nil {
		response.Error = fmt.Sprintf("failed to decode args: %v", err)
		return response
	}
	
	// 创建上下文
	ctx := context.WithValue(context.Background(), "connection", conn)
	ctx = context.WithValue(ctx, "metadata", request.Metadata)
	
	// 应用拦截器
	if s.interceptor != nil {
		ctx = s.interceptor.Before(ctx, request)
	}
	
	// 调用方法
	args := []reflect.Value{
		serviceInfo.Receiver,
		reflect.ValueOf(ctx),
		argValue,
		replyValue,
	}
	
	results := methodInfo.Method.Func.Call(args)
	
	// 检查错误
	if !results[0].IsNil() {
		err := results[0].Interface().(error)
		response.Error = err.Error()
	} else {
		response.Result = replyValue.Interface()
	}
	
	// 应用拦截器
	if s.interceptor != nil {
		s.interceptor.After(ctx, request, response)
	}
	
	return response
}

// Stop 停止RPC服务器
func (s *RPCServer) Stop() error {
	s.running = false
	
	if s.listener != nil {
		return s.listener.Close()
	}
	
	return nil
}

// GetStats 获取服务器统计信息
func (s *RPCServer) GetStats() *core.ServerStats {
	return s.BaseServer.GetStats()
}

// SetHandler 设置消息处理器
func (s *RPCServer) SetHandler(handler core.MessageHandler) {
	s.handler = handler
}

