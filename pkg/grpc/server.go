// Package grpc gRPC协议服务器实现
// Author: NetCore-Go Team
// Created: 2024

package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/phuhao00/netcore-go/pkg/core"
	"github.com/phuhao00/netcore-go/pkg/pool"
)

// GRPCServer gRPC服务器实现
type GRPCServer struct {
	core.BaseServer
	grpcServer   *grpc.Server
	services     map[string]interface{}
	serviceMu    sync.RWMutex
	interceptors []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
}

// NewGRPCServer 创建gRPC服务器
func NewGRPCServer(opts ...core.ServerOption) *GRPCServer {
	server := &GRPCServer{
		services: make(map[string]interface{}),
	}
	
	// 应用配置选项
	for _, opt := range opts {
		opt(&server.BaseServer)
	}
	
	// 设置默认配置
	if server.Config.ReadBufferSize == 0 {
		server.Config.ReadBufferSize = 4096
	}
	if server.Config.WriteBufferSize == 0 {
		server.Config.WriteBufferSize = 4096
	}
	if server.Config.MaxConnections == 0 {
		server.Config.MaxConnections = 1000
	}
	
	return server
}

// RegisterService 注册gRPC服务
func (s *GRPCServer) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()
	
	s.services[desc.ServiceName] = impl
	
	// 如果gRPC服务器已创建，直接注册
	if s.grpcServer != nil {
		s.grpcServer.RegisterService(desc, impl)
	}
}

// AddUnaryInterceptor 添加一元拦截器
func (s *GRPCServer) AddUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) {
	s.interceptors = append(s.interceptors, interceptor)
}

// AddStreamInterceptor 添加流拦截器
func (s *GRPCServer) AddStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	s.streamInterceptors = append(s.streamInterceptors, interceptor)
}

// Start 启动gRPC服务器
func (s *GRPCServer) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	
	s.Listener = listener
	s.Running = true
	s.Stats.StartTime = time.Now()
	
	// 启动连接池
	if s.Config.EnableConnectionPool {
		s.ConnPool = pool.NewConnectionPool(pool.PoolConfig{
			InitialSize: 10,
			MaxSize:     s.Config.MaxConnections,
			IdleTimeout: 5 * time.Minute,
		})
	}
	
	// 启动内存池
	if s.Config.EnableMemoryPool {
		s.MemPool = pool.NewMemoryPool(s.Config.ReadBufferSize, 100)
	}
	
	// 启动协程池
	if s.Config.EnableGoroutinePool {
		s.GoroutinePool = pool.NewGoroutinePool(1000, 10000)
	}
	
	// 创建gRPC服务器选项
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.Config.ReadBufferSize),
		grpc.MaxSendMsgSize(s.Config.WriteBufferSize),
		grpc.MaxConcurrentStreams(uint32(s.Config.MaxConnections)),
	}
	
	// 添加拦截器
	if len(s.interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(s.interceptors...))
	}
	if len(s.streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(s.streamInterceptors...))
	}
	
	// 添加默认拦截器
	defaultInterceptors := []grpc.UnaryServerInterceptor{
		s.loggingInterceptor,
		s.metricsInterceptor,
		s.recoveryInterceptor,
	}
	opts = append(opts, grpc.ChainUnaryInterceptor(defaultInterceptors...))
	
	// 创建gRPC服务器
	s.grpcServer = grpc.NewServer(opts...)
	
	// 注册已添加的服务
	s.serviceMu.RLock()
	for serviceName, impl := range s.services {
		// 注意：这里需要服务描述符，实际使用时需要传入正确的ServiceDesc
		// 这里只是示例，实际实现需要根据具体的protobuf生成的代码来注册
		_ = serviceName
		_ = impl
	}
	s.serviceMu.RUnlock()
	
	// 启动服务器
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			s.Stats.ErrorCount++
			s.Stats.LastError = err.Error()
		}
	}()
	
	return nil
}

// Stop 停止gRPC服务器
func (s *GRPCServer) Stop() error {
	s.Running = false
	
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	
	if s.Listener != nil {
		return s.Listener.Close()
	}
	
	return nil
}

// GetStats 获取服务器统计信息
func (s *GRPCServer) GetStats() *core.ServerStats {
	return &s.Stats
}

// SetHandler 设置消息处理器（gRPC不需要）
func (s *GRPCServer) SetHandler(handler core.MessageHandler) {
	s.Handler = handler
}

// SetMiddleware 设置中间件（gRPC使用拦截器）
func (s *GRPCServer) SetMiddleware(middleware ...core.Middleware) {
	s.Middlewares = middleware
}

// loggingInterceptor 日志拦截器
func (s *GRPCServer) loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	
	// 获取元数据
	md, _ := metadata.FromIncomingContext(ctx)
	
	// 记录请求开始
	fmt.Printf("[gRPC] Request started: method=%s, metadata=%v\n", info.FullMethod, md)
	
	// 调用处理器
	resp, err := handler(ctx, req)
	
	// 记录请求结束
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("[gRPC] Request failed: method=%s, duration=%v, error=%v\n", info.FullMethod, duration, err)
		s.Stats.ErrorCount++
		s.Stats.LastError = err.Error()
	} else {
		fmt.Printf("[gRPC] Request completed: method=%s, duration=%v\n", info.FullMethod, duration)
	}
	
	s.Stats.MessagesReceived++
	s.Stats.MessagesSent++
	
	return resp, err
}

// metricsInterceptor 指标拦截器
func (s *GRPCServer) metricsInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	
	// 调用处理器
	resp, err := handler(ctx, req)
	
	// 记录指标
	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}
	
	// TODO: 集成实际的指标收集系统（如Prometheus）
	fmt.Printf("[gRPC Metrics] method=%s, status=%s, duration=%v\n", info.FullMethod, status, duration)
	
	return resp, err
}

// recoveryInterceptor 恢复拦截器
func (s *GRPCServer) recoveryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = status.Errorf(codes.Internal, "panic recovered: %v", r)
			s.Stats.ErrorCount++
			s.Stats.LastError = fmt.Sprintf("panic: %v", r)
			fmt.Printf("[gRPC] Panic recovered: method=%s, panic=%v\n", info.FullMethod, r)
		}
	}()
	
	return handler(ctx, req)
}

// AuthInterceptor 认证拦截器
func AuthInterceptor(validTokens map[string]bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 获取元数据
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}
		
		// 检查授权令牌
		authorization := md.Get("authorization")
		if len(authorization) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization token")
		}
		
		token := authorization[0]
		if !validTokens[token] {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization token")
		}
		
		// 将用户信息添加到上下文
		ctx = metadata.AppendToOutgoingContext(ctx, "user-token", token)
		
		return handler(ctx, req)
	}
}

// RateLimitInterceptor 限流拦截器
func RateLimitInterceptor(maxRequests int, window time.Duration) grpc.UnaryServerInterceptor {
	requestCounts := make(map[string]int)
	lastReset := time.Now()
	mu := sync.Mutex{}
	
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		mu.Lock()
		defer mu.Unlock()
		
		// 重置计数器
		if time.Since(lastReset) > window {
			requestCounts = make(map[string]int)
			lastReset = time.Now()
		}
		
		// 获取客户端标识（这里使用方法名，实际可以使用IP等）
		clientID := info.FullMethod
		
		// 检查限流
		if requestCounts[clientID] >= maxRequests {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		
		requestCounts[clientID]++
		
		return handler(ctx, req)
	}
}