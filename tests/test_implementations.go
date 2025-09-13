// 测试实现的功能
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/netcore-go/pkg/rpc"
)

func main() {
	fmt.Println("测试MessagePack编解码器...")
	testMessagePackCodec()

	fmt.Println("\n测试Prometheus指标收集器...")
	testPrometheusMetrics()

	fmt.Println("\n所有测试完成!")
}

func testMessagePackCodec() {
	// 创建MessagePack编解码器
	codec := rpc.NewMsgPackCodec()

	// 测试数据
	testData := map[string]interface{}{
		"name":    "test",
		"age":     25,
		"active":  true,
		"scores":  []int{90, 85, 88},
		"created": time.Now(),
	}

	// 编码
	encoded, err := codec.Encode(testData)
	if err != nil {
		log.Fatalf("MessagePack编码失败: %v", err)
	}
	fmt.Printf("编码成功，数据长度: %d bytes\n", len(encoded))

	// 解码
	var decoded map[string]interface{}
	err = codec.Decode(encoded, &decoded)
	if err != nil {
		log.Fatalf("MessagePack解码失败: %v", err)
	}
	fmt.Printf("解码成功: %+v\n", decoded)
}

func testPrometheusMetrics() {
	// 创建Prometheus指标收集器
	metricsCollector := rpc.NewPrometheusMetricsCollector()

	// 测试计数器
	labels := map[string]string{
		"service": "test-service",
		"method":  "test-method",
		"status":  "success",
	}

	metricsCollector.IncCounter("rpc_requests_total", labels)
	fmt.Println("计数器指标记录成功")

	// 测试直方图
	metricsCollector.ObserveHistogram("rpc_request_duration_seconds", 0.123, labels)
	fmt.Println("直方图指标记录成功")

	// 测试仪表盘
	metricsCollector.SetGauge("rpc_active_requests", 5.0, labels)
	fmt.Println("仪表盘指标记录成功")
}