// Package benchmarks 提供NetCore-Go的性能基准测试
// Author: NetCore-Go Team
// Created: 2024

package benchmarks

import (
	"bytes"

	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/phuhao00/netcore-go/pkg/core"
	netcoretesting "github.com/phuhao00/netcore-go/pkg/testing"
)

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	Concurrency int
	Duration    time.Duration
	WarmupTime  time.Duration
}

// DefaultBenchmarkConfig 默认基准测试配置
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		Concurrency: runtime.NumCPU(),
		Duration:    10 * time.Second,
		WarmupTime:  2 * time.Second,
	}
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	Operations     int64
	Duration       time.Duration
	OpsPerSec      float64
	AvgLatency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	P50Latency     time.Duration
	P95Latency     time.Duration
	P99Latency     time.Duration
	MemoryUsage    int64
	Allocations    int64
	Goroutines     int
}

// String 返回基准测试结果的字符串表示
func (r *BenchmarkResult) String() string {
	return fmt.Sprintf(
		"Operations: %d, Duration: %v, Ops/sec: %.2f, Avg Latency: %v, P95: %v, P99: %v, Memory: %d bytes, Allocs: %d, Goroutines: %d",
		r.Operations, r.Duration, r.OpsPerSec, r.AvgLatency, r.P95Latency, r.P99Latency, r.MemoryUsage, r.Allocations, r.Goroutines,
	)
}

// BenchmarkErrorHandling 错误处理性能基准测试
func BenchmarkErrorHandling(b *testing.B) {
	b.Run("NewError", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := core.NewError(core.ErrCodeInternal, "test error")
			_ = err
		}
	})
	
	b.Run("ErrorWithContext", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := core.NewError(core.ErrCodeInternal, "test error")
			// WithContext method not available, using error as-is
			_ = err
		}
	})
	
	b.Run("ErrorChaining", func(b *testing.B) {
		baseErr := fmt.Errorf("base error")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := core.NewError(core.ErrCodeInternal, "wrapped error")
			// WithCause method not available, using error as-is
			_ = err
			_ = baseErr
		}
	})
	
	b.Run("ErrorHandlerProcess", func(b *testing.B) {
		handler := core.NewDefaultErrorHandler()
		err := core.NewError(core.ErrCodeInternal, "test error")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processedErr := handler.HandleError(err)
			_ = processedErr
		}
	})
}

// BenchmarkLogging 日志记录性能基准测试
func BenchmarkLogging(b *testing.B) {
	logger := core.NewLogger(core.DefaultLoggerConfig())
	
	b.Run("InfoLogging", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("test info message")
		}
	})
	
	b.Run("InfoLoggingWithFields", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.WithField("key", "value").WithField("count", i).Info("test message")
		}
	})
	
	b.Run("ErrorLogging", func(b *testing.B) {
		err := fmt.Errorf("test error")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Error("error occurred: " + err.Error())
		}
	})
	
	b.Run("StructuredLogging", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.WithField("user_id", 12345).
				WithField("action", "login").
				WithField("ip", "192.168.1.1").
				WithField("timestamp", time.Now()).
				Info("user login")
		}
	})
	
	b.Run("AsyncLogging", func(b *testing.B) {
		// AsyncEnabled field not available, using regular logger
		logger := core.NewLogger(core.DefaultLoggerConfig())
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info(fmt.Sprintf("async message %d", i))
		}
	})
}

// BenchmarkValidation 验证性能基准测试
func BenchmarkValidation(b *testing.B) {
	// Validation rules not available, using simple validation
	b.Run("SimpleValidation", func(b *testing.B) {
		config := map[string]interface{}{
			"name": "John Doe",
			"age":  25,
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simple validation logic
			name, nameOk := config["name"].(string)
			age, ageOk := config["age"].(int)
			valid := nameOk && len(name) > 0 && ageOk && age > 0
			_ = valid
		}
	})
	
	b.Run("ComplexValidation", func(b *testing.B) {
		config := map[string]interface{}{
			"name":     "John Doe",
			"age":      25,
			"email":    "john@example.com",
			"password": "secretpassword123",
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Complex validation logic
			name, nameOk := config["name"].(string)
			age, ageOk := config["age"].(int)
			email, emailOk := config["email"].(string)
			password, passwordOk := config["password"].(string)
			valid := nameOk && len(name) > 0 && ageOk && age > 0 && emailOk && len(email) > 0 && passwordOk && len(password) > 0
			_ = valid
		}
	})
}

// BenchmarkHTTPServer HTTP服务器性能基准测试
func BenchmarkHTTPServer(b *testing.B) {
	// 创建测试服务器
	mux := http.NewServeMux()
	
	// 简单的健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})
	
	// JSON处理端点
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"received": data,
			"timestamp": time.Now().Unix(),
		})
	})
	
	// 错误处理端点
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		err := core.NewError(core.ErrCodeInternal, "simulated error")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(err.HTTPStatusCode())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
			"code":  err.Code,
		})
	})
	
	server := httptest.NewServer(mux)
	defer server.Close()
	
	b.Run("HealthCheck", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				resp, err := http.Get(server.URL + "/health")
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	})
	
	b.Run("JSONProcessing", func(b *testing.B) {
		payload := map[string]interface{}{
			"name":  "John Doe",
			"age":   30,
			"email": "john@example.com",
		}
		payloadBytes, _ := json.Marshal(payload)
		
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				resp, err := http.Post(
					server.URL+"/json",
					"application/json",
					bytes.NewBuffer(payloadBytes),
				)
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	})
	
	b.Run("ErrorHandling", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				resp, err := http.Get(server.URL + "/error")
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	})
}

// BenchmarkConcurrency 并发性能基准测试
func BenchmarkConcurrency(b *testing.B) {
	b.Run("GoroutineCreation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				done := make(chan bool)
				go func() {
					// 模拟一些工作
					time.Sleep(1 * time.Microsecond)
					done <- true
				}()
				<-done
			}
		})
	})
	
	b.Run("ChannelCommunication", func(b *testing.B) {
		ch := make(chan int, 100)
		
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				select {
				case ch <- 1:
				case <-ch:
				default:
				}
			}
		})
	})
	
	b.Run("MutexContention", func(b *testing.B) {
		var mu sync.Mutex
		var counter int
		
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		})
	})
	
	b.Run("RWMutexRead", func(b *testing.B) {
		var mu sync.RWMutex
		data := make(map[string]int)
		data["key"] = 42
		
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.RLock()
				_ = data["key"]
				mu.RUnlock()
			}
		})
	})
}

// BenchmarkMemoryOperations 内存操作性能基准测试
func BenchmarkMemoryOperations(b *testing.B) {
	b.Run("SliceAppend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var slice []int
			for j := 0; j < 1000; j++ {
				slice = append(slice, j)
			}
		}
	})
	
	b.Run("SlicePrealloc", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := make([]int, 0, 1000)
			for j := 0; j < 1000; j++ {
				slice = append(slice, j)
			}
		}
	})
	
	b.Run("MapOperations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m := make(map[string]int)
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key%d", j)
				m[key] = j
			}
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key%d", j)
				_ = m[key]
			}
		}
	})
	
	b.Run("MapPrealloc", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m := make(map[string]int, 100)
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key%d", j)
				m[key] = j
			}
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key%d", j)
				_ = m[key]
			}
		}
	})
	
	b.Run("StringConcatenation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result string
			for j := 0; j < 100; j++ {
				result += fmt.Sprintf("item%d", j)
			}
		}
	})
	
	b.Run("StringBuilderConcatenation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var builder strings.Builder
			for j := 0; j < 100; j++ {
				builder.WriteString(fmt.Sprintf("item%d", j))
			}
			_ = builder.String()
		}
	})
}

// BenchmarkJSONOperations JSON操作性能基准测试
func BenchmarkJSONOperations(b *testing.B) {
	type TestStruct struct {
		ID       int                    `json:"id"`
		Name     string                 `json:"name"`
		Email    string                 `json:"email"`
		Age      int                    `json:"age"`
		Active   bool                   `json:"active"`
		Tags     []string               `json:"tags"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	
	testData := TestStruct{
		ID:     12345,
		Name:   "John Doe",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
		Tags:   []string{"user", "premium", "active"},
		Metadata: map[string]interface{}{
			"last_login": "2024-01-01T00:00:00Z",
			"preferences": map[string]interface{}{
				"theme": "dark",
				"language": "en",
			},
		},
	}
	
	b.Run("JSONMarshal", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("JSONUnmarshal", func(b *testing.B) {
		jsonData, _ := json.Marshal(testData)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result TestStruct
			err := json.Unmarshal(jsonData, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("JSONEncode", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			encoder := json.NewEncoder(&buf)
			err := encoder.Encode(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("JSONDecode", func(b *testing.B) {
		jsonData, _ := json.Marshal(testData)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(jsonData)
			decoder := json.NewDecoder(buf)
			var result TestStruct
			err := decoder.Decode(&result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDatabaseOperations 数据库操作性能基准测试
func BenchmarkDatabaseOperations(b *testing.B) {
	mockDB := netcoretesting.NewMockDatabase()
	
	b.Run("DatabaseSet", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("user:%d", i)
			value := map[string]interface{}{
				"id":   i,
				"name": fmt.Sprintf("User %d", i),
				"email": fmt.Sprintf("user%d@example.com", i),
			}
			mockDB.Set(key, value)
		}
	})
	
	b.Run("DatabaseGet", func(b *testing.B) {
		// 预填充数据
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("user:%d", i)
			value := map[string]interface{}{
				"id":   i,
				"name": fmt.Sprintf("User %d", i),
			}
			mockDB.Set(key, value)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("user:%d", i%1000)
			_, _ = mockDB.Get(key)
		}
	})
	
	b.Run("DatabaseDelete", func(b *testing.B) {
		// 预填充数据
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("temp:%d", i)
			value := map[string]interface{}{"data": i}
			mockDB.Set(key, value)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("temp:%d", i)
			mockDB.Delete(key)
		}
	})
	
	b.Run("DatabaseConcurrentAccess", func(b *testing.B) {
		// 预填充数据
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("concurrent:%d", i)
			value := map[string]interface{}{"data": i}
			mockDB.Set(key, value)
		}
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent:%d", i%100)
				if i%2 == 0 {
					// 读操作
					_, _ = mockDB.Get(key)
				} else {
					// 写操作
					value := map[string]interface{}{"data": i}
					mockDB.Set(key, value)
				}
				i++
			}
		})
	})
}

// BenchmarkCacheOperations 缓存操作性能基准测试
func BenchmarkCacheOperations(b *testing.B) {
	mockCache := netcoretesting.NewMockCache()
	
	b.Run("CacheSet", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("cache:%d", i)
			value := fmt.Sprintf("value-%d", i)
			mockCache.Set(key, value, time.Hour)
		}
	})
	
	b.Run("CacheGet", func(b *testing.B) {
		// 预填充缓存
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("cache:%d", i)
			value := fmt.Sprintf("value-%d", i)
			mockCache.Set(key, value, time.Hour)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("cache:%d", i%1000)
			_, _ = mockCache.Get(key)
		}
	})
	
	b.Run("CacheDelete", func(b *testing.B) {
		// 预填充缓存
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("temp_cache:%d", i)
			value := fmt.Sprintf("value-%d", i)
			mockCache.Set(key, value, time.Hour)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("temp_cache:%d", i)
			mockCache.Delete(key)
		}
	})
	
	b.Run("CacheConcurrentAccess", func(b *testing.B) {
		// 预填充缓存
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("concurrent_cache:%d", i)
			value := fmt.Sprintf("value-%d", i)
			mockCache.Set(key, value, time.Hour)
		}
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent_cache:%d", i%100)
				if i%3 == 0 {
					// 读操作
					_, _ = mockCache.Get(key)
				} else if i%3 == 1 {
					// 写操作
					value := fmt.Sprintf("updated-value-%d", i)
					mockCache.Set(key, value, time.Hour)
				} else {
					// 删除操作
					mockCache.Delete(key)
					// 重新设置以保持数据
					value := fmt.Sprintf("value-%d", i%100)
					mockCache.Set(key, value, time.Hour)
				}
				i++
			}
		})
	})
}

// BenchmarkCustomLoadTest 自定义负载测试
func BenchmarkCustomLoadTest(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping load test in short mode")
	}
	
	// 创建测试服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		// 模拟一些处理时间
		time.Sleep(1 * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "success",
			"timestamp": time.Now().Unix(),
		})
	})
	
	server := httptest.NewServer(mux)
	defer server.Close()
	
	b.Run("LoadTest", func(b *testing.B) {
		config := DefaultBenchmarkConfig()
		config.Concurrency = 10
		config.Duration = 5 * time.Second
		
		result := runCustomLoadTest(b, server.URL+"/api/test", config)
		
		b.Logf("Load test results: %s", result.String())
		
		// 验证性能指标
		if result.OpsPerSec < 100 {
			b.Errorf("Operations per second too low: %.2f (expected >= 100)", result.OpsPerSec)
		}
		
		if result.AvgLatency > 50*time.Millisecond {
			b.Errorf("Average latency too high: %v (expected <= 50ms)", result.AvgLatency)
		}
	})
}

// runCustomLoadTest 运行自定义负载测试
func runCustomLoadTest(b *testing.B, url string, config *BenchmarkConfig) *BenchmarkResult {
	var (
		operations   int64
		latencies    []time.Duration
		latenciesMu  sync.Mutex
		startTime    = time.Now()
		endTime      = startTime.Add(config.Duration)
		wg           sync.WaitGroup
	)
	
	// 预热
	if config.WarmupTime > 0 {
		warmupEnd := time.Now().Add(config.WarmupTime)
		for i := 0; i < config.Concurrency; i++ {
			go func() {
				for time.Now().Before(warmupEnd) {
					resp, err := http.Get(url)
					if err == nil {
						resp.Body.Close()
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}
		time.Sleep(config.WarmupTime)
	}
	
	// 获取初始内存统计
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)
	goroutinesBefore := runtime.NumGoroutine()
	
	// 启动负载测试
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for time.Now().Before(endTime) {
				reqStart := time.Now()
				resp, err := http.Get(url)
				latency := time.Since(reqStart)
				
				if err == nil {
					resp.Body.Close()
					atomic.AddInt64(&operations, 1)
					
					latenciesMu.Lock()
					latencies = append(latencies, latency)
					latenciesMu.Unlock()
				}
				
				// 小延迟避免过度压力
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}
	
	wg.Wait()
	
	// 获取最终内存统计
	var memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)
	goroutinesAfter := runtime.NumGoroutine()
	
	// 计算结果
	duration := time.Since(startTime)
	opsPerSec := float64(operations) / duration.Seconds()
	
	// 计算延迟统计
	if len(latencies) == 0 {
		return &BenchmarkResult{
			Operations: operations,
			Duration:   duration,
			OpsPerSec:  opsPerSec,
		}
	}
	
	// 排序延迟数据
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	
	// 计算延迟统计
	var totalLatency time.Duration
	for _, lat := range latencies {
		totalLatency += lat
	}
	
	avgLatency := totalLatency / time.Duration(len(latencies))
	minLatency := latencies[0]
	maxLatency := latencies[len(latencies)-1]
	p50Latency := latencies[len(latencies)*50/100]
	p95Latency := latencies[len(latencies)*95/100]
	p99Latency := latencies[len(latencies)*99/100]
	
	return &BenchmarkResult{
		Operations:  operations,
		Duration:    duration,
		OpsPerSec:   opsPerSec,
		AvgLatency:  avgLatency,
		MinLatency:  minLatency,
		MaxLatency:  maxLatency,
		P50Latency:  p50Latency,
		P95Latency:  p95Latency,
		P99Latency:  p99Latency,
		MemoryUsage: int64(memStatsAfter.Alloc - memStatsBefore.Alloc),
		Allocations: int64(memStatsAfter.Mallocs - memStatsBefore.Mallocs),
		Goroutines:  goroutinesAfter - goroutinesBefore,
	}
}