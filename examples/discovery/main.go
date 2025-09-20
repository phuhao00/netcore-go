// 服务发现示例
package main

import (
	"fmt"
	"log"

	"github.com/phuhao00/netcore-go/pkg/discovery"
	"github.com/phuhao00/netcore-go/pkg/discovery/consul"
	"github.com/phuhao00/netcore-go/pkg/discovery/etcd"
)

func main() {
	fmt.Println("=== NetCore-Go 服务发现示例 ===")

	// 测试Etcd服务发现
	fmt.Println("\n1. 测试Etcd服务发现...")
	testEtcdDiscovery()

	// 测试Consul服务发现
	fmt.Println("\n2. 测试Consul服务发现...")
	testConsulDiscovery()

	fmt.Println("\n=== 所有测试完成 ===")
}

func testEtcdDiscovery() {
	// 创建Etcd配置
	config := etcd.DefaultEtcdConfig()
	config.Endpoints = []string{"127.0.0.1:2379"}

	// 创建Etcd服务发现实例
	etcdDiscovery, err := etcd.NewEtcdDiscovery(config)
	if err != nil {
		log.Printf("创建Etcd服务发现失败: %v", err)
		return
	}

	// 创建测试服务实例
	service := &discovery.ServiceInstance{
		ID:      "test-service-1",
		Name:    "test-service",
		Address: "127.0.0.1",
		Port:    8080,
		Tags:    []string{"api", "v1"},
		Meta: map[string]string{
			"version": "1.0.0",
			"env":     "test",
		},
		Health:   discovery.Healthy,
		Protocol: "http",
	}

	fmt.Printf("Etcd服务发现实例创建成功\n")
	fmt.Printf("配置: %+v\n", config)
	fmt.Printf("测试服务: %s (%s:%d)\n", service.Name, service.Address, service.Port)

	// 获取统计信息
	stats := etcdDiscovery.GetStats()
	fmt.Printf("统计信息: %+v\n", stats)
}

func testConsulDiscovery() {
	// 创建Consul配置
	config := consul.DefaultConsulConfig()
	config.Address = "127.0.0.1:8500"

	// 创建Consul服务发现实例
	consulDiscovery, err := consul.NewConsulDiscovery(config)
	if err != nil {
		log.Printf("创建Consul服务发现失败: %v", err)
		return
	}

	// 创建测试服务实例
	service := &discovery.ServiceInstance{
		ID:      "test-service-2",
		Name:    "test-service",
		Address: "127.0.0.1",
		Port:    8081,
		Tags:    []string{"api", "v2"},
		Meta: map[string]string{
			"version": "2.0.0",
			"env":     "test",
		},
		Health:   discovery.Healthy,
		Protocol: "http",
	}

	fmt.Printf("Consul服务发现实例创建成功\n")
	fmt.Printf("配置: %+v\n", config)
	fmt.Printf("测试服务: %s (%s:%d)\n", service.Name, service.Address, service.Port)

	// 获取统计信息
	stats := consulDiscovery.GetStats()
	fmt.Printf("统计信息: %+v\n", stats)
}
