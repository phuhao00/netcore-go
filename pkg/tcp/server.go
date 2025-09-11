// Package tcp 实现NetCore-Go网络库的TCP服务器
// Author: NetCore-Go Team
// Created: 2024

package tcp

import (
	"github.com/netcore-go/pkg/core"
)

// TCPServer TCP服务器实现
type TCPServer struct {
	*core.BaseServer
}

// NewTCPServer 创建新的TCP服务器
func NewTCPServer(opts ...core.ServerOption) core.Server {
	return &TCPServer{
		BaseServer: core.NewBaseServer(opts...),
	}
}

// Start 启动TCP服务器
func (s *TCPServer) Start(addr string) error {
	return s.BaseServer.Start(addr)
}

// Stop 停止TCP服务器
func (s *TCPServer) Stop() error {
	return s.BaseServer.Stop()
}

// SetHandler 设置消息处理器
func (s *TCPServer) SetHandler(handler core.MessageHandler) {
	s.BaseServer.SetHandler(handler)
}

// SetMiddleware 设置中间件
func (s *TCPServer) SetMiddleware(middleware ...core.Middleware) {
	s.BaseServer.SetMiddleware(middleware...)
}

// GetStats 获取服务器统计信息
func (s *TCPServer) GetStats() *core.ServerStats {
	return s.BaseServer.GetStats()
}

