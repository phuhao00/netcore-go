package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/netcore-go/pkg/logger"
)

func main() {
	fmt.Println("=== NetCore-Go 日志系统示例 ===")
	
	// 示例1: 基本日志使用
	fmt.Println("\n1. 基本日志使用示例")
	basicLoggingExample()
	
	// 示例2: 结构化日志
	fmt.Println("\n2. 结构化日志示例")
	structuredLoggingExample()
	
	// 示例3: 不同格式化器
	fmt.Println("\n3. 不同格式化器示例")
	formatterExample()
	
	// 示例4: 文件日志
	fmt.Println("\n4. 文件日志示例")
	fileLoggingExample()
	
	// 示例5: 日志钩子
	fmt.Println("\n5. 日志钩子示例")
	hookExample()
	
	// 示例6: 多重写入器
	fmt.Println("\n6. 多重写入器示例")
	multiWriterExample()
	
	// 示例7: 上下文日志
	fmt.Println("\n7. 上下文日志示例")
	contextLoggingExample()
	
	// 示例8: 自定义日志器
	fmt.Println("\n8. 自定义日志器示例")
	customLoggerExample()
	
	// 示例9: 性能测试
	fmt.Println("\n9. 性能测试示例")
	performanceExample()
}

// basicLoggingExample 基本日志使用示例
func basicLoggingExample() {
	// 使用全局日志器
	logger.SetGlobalLogger(logger.NewLogger(&logger.Config{
		Level:     logger.DebugLevel,
		Formatter: "text",
		Output:    "console",
	}))
	
	// 不同级别的日志
	logger.Trace("这是一条TRACE日志")
	logger.Debug("这是一条DEBUG日志")
	logger.Info("这是一条INFO日志")
	logger.Warn("这是一条WARN日志")
	logger.Error("这是一条ERROR日志")
	
	// 格式化日志
	logger.Infof("用户 %s 登录成功，IP: %s", "admin", "192.168.1.100")
	logger.Warnf("磁盘使用率达到 %d%%", 85)
	
	// 带错误的日志
	err := errors.New("数据库连接失败")
	logger.ErrorWithErr("操作失败", err)
}

// structuredLoggingExample 结构化日志示例
func structuredLoggingExample() {
	// 创建带字段的日志器
	log := logger.WithFields(map[string]interface{}{
		"service": "user-service",
		"version": "1.0.0",
	})
	
	// 添加更多字段
	log = log.WithField("request_id", "req-12345")
	log = log.WithField("user_id", 1001)
	
	log.Info("用户请求处理开始")
	
	// 模拟处理过程
	time.Sleep(100 * time.Millisecond)
	
	log.WithField("duration", "100ms").Info("用户请求处理完成")
	
	// 错误处理
	err := errors.New("权限不足")
	log.WithError(err).Error("用户操作失败")
}

// formatterExample 不同格式化器示例
func formatterExample() {
	// JSON格式化器
	fmt.Println("JSON格式:")
	jsonLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "json",
		Output:    "console",
	})
	
	jsonLogger.WithFields(map[string]interface{}{
		"user_id": 1001,
		"action":  "login",
	}).Info("用户登录")
	
	// Logfmt格式化器
	fmt.Println("\nLogfmt格式:")
	logfmtLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "text",
		Output:    "console",
	})
	
	logfmtLogger.WithFields(map[string]interface{}{
		"user_id": 1001,
		"action":  "logout",
	}).Info("用户登出")
	
	// 自定义格式化器
	fmt.Println("\n自定义格式:")
	customLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "text",
		Output:    "console",
	})
	
	customLogger.Info("自定义格式日志")
}

// fileLoggingExample 文件日志示例
func fileLoggingExample() {
	// 创建文件写入器
	fileWriter, err := logger.NewFileWriter(&logger.FileWriterConfig{
		Filename:   "logs/app.log",
		MaxSize:    1024 * 1024, // 1MB
		MaxAge:     7,           // 7天
		MaxBackups: 5,           // 5个备份
		Compress:   true,        // 压缩备份
	})
	if err != nil {
		fmt.Printf("创建文件写入器失败: %v\n", err)
		return
	}
	defer fileWriter.Close()
	
	// 创建文件日志器
	fileLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "json",
		Output:    "file",
		FilePath:  "logs/app.log",
	})
	
	// 写入日志到文件
	for i := 0; i < 10; i++ {
		fileLogger.WithField("iteration", i).Infof("文件日志测试 #%d", i)
	}
	
	fmt.Println("日志已写入到 logs/app.log 文件")
}

// hookExample 日志钩子示例
func hookExample() {
	// 创建指标钩子
	metricsHook := logger.NewMetricsHook([]logger.Level{
		logger.InfoLevel, logger.WarnLevel, logger.ErrorLevel,
	})
	
	// 创建文件钩子（用于错误日志）
	errorFileHook, err := logger.NewFileHook(&logger.FileHookConfig{
		Filename: "logs/errors.log",
		Levels:   []logger.Level{logger.ErrorLevel, logger.FatalLevel},
	})
	if err != nil {
		fmt.Printf("创建文件钩子失败: %v\n", err)
		return
	}
	defer errorFileHook.Close()
	
	// 创建回调钩子 - 暂时注释
	// callbackHook := logger.NewCallbackHook(
	//	[]logger.Level{logger.ErrorLevel},
	//	func(entry *logger.Entry) error {
	//		fmt.Printf("[钩子回调] 检测到错误日志: %s\n", entry.Message)
	//		return nil
	//	},
	// )
	
	// 创建带钩子的日志器
	hookLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "text",
		Output:    "console",
	})
	
	// 生成一些日志
	hookLogger.Info("正常信息日志")
	hookLogger.Warn("警告日志")
	hookLogger.Error("错误日志")
	hookLogger.Info("另一条信息日志")
	hookLogger.Error("另一条错误日志")
	
	// 显示指标统计
	counters := metricsHook.GetCounters()
	fmt.Println("\n日志统计:")
	for level, count := range counters {
		if count > 0 {
			fmt.Printf("  %s: %d\n", level.String(), count)
		}
	}
}

// multiWriterExample 多重写入器示例
func multiWriterExample() {
	// 创建控制台写入器
	// consoleWriter := &logger.ConsoleWriter{Output: os.Stdout}
	
	// 创建文件写入器
	fileWriter, err := logger.NewFileWriter(&logger.FileWriterConfig{
		Filename: "logs/multi.log",
		MaxSize:  1024 * 1024,
	})
	if err != nil {
		fmt.Printf("创建文件写入器失败: %v\n", err)
		return
	}
	defer fileWriter.Close()
	
	// 创建多重写入器 - 暂时注释
	// multiWriter := logger.NewMultiWriter(consoleWriter, fileWriter)
	
	// 创建多重写入器日志器
	multiLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "text",
		Output:    "console",
	})
	
	// 同时输出到控制台和文件
	multiLogger.Info("这条日志会同时输出到控制台和文件")
	multiLogger.Warn("多重写入器测试")
	
	fmt.Println("日志已同时写入控制台和 logs/multi.log 文件")
}

// contextLoggingExample 上下文日志示例
func contextLoggingExample() {
	// 创建带上下文的context
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-67890")
	ctx = context.WithValue(ctx, "user_id", 2001)
	ctx = context.WithValue(ctx, "trace_id", "trace-abcdef")
	
	// 从上下文创建日志器
	ctxLogger := logger.WithContext(ctx)
	
	ctxLogger.Info("开始处理用户请求")
	
	// 模拟业务处理
	processUserRequest(ctx)
	
	ctxLogger.Info("用户请求处理完成")
}

// processUserRequest 模拟处理用户请求
func processUserRequest(ctx context.Context) {
	log := logger.WithContext(ctx).WithField("function", "processUserRequest")
	
	log.Debug("验证用户权限")
	time.Sleep(50 * time.Millisecond)
	
	log.Debug("查询用户数据")
	time.Sleep(100 * time.Millisecond)
	
	log.Info("用户数据处理完成")
}

// customLoggerExample 自定义日志器示例
func customLoggerExample() {
	// 创建自定义文本格式化器 - 暂时注释
	// customTextFormatter := &logger.TextFormatter{
	//	TimestampFormat:        "2006-01-02 15:04:05.000",
	//	DisableColors:          false,
	//	FullTimestamp:          true,
	//	PadLevelText:           true,
	//	QuoteEmptyFields:       true,
	//	DisableLevelTruncation: true,
	//	FieldMap: map[string]string{
	//		"msg":   "message",
	//		"level": "severity",
	//	},
	//	CallerPrettyfier: func(caller *logger.CallerInfo) (string, string) {
	//		return caller.Function, fmt.Sprintf("%s:%d", caller.File, caller.Line)
	//	},
	// }
	
	// 创建自定义JSON格式化器
	customJSONFormatter := &logger.JSONFormatter{
		TimestampFormat:   time.RFC3339Nano,
		DisableTimestamp:  false,
		DisableHTMLEscape: true,
		PrettyPrint:       true,
		FieldMap: map[string]string{
			"msg":   "message",
			"level": "severity",
			"time":  "timestamp",
		},
	}
	
	// 创建自定义日志器
	customLogger := logger.NewLogger(&logger.Config{
		Level:     logger.DebugLevel,
		Formatter: "text",
		Output:    "console",
	})
	
	fmt.Println("自定义文本格式:")
	customLogger.WithField("component", "auth").Info("用户认证成功")
	
	// 切换到JSON格式
	customLogger.SetFormatter(customJSONFormatter)
	fmt.Println("\n自定义JSON格式:")
	customLogger.WithField("component", "database").Warn("数据库连接池使用率较高")
}

// performanceExample 性能测试示例
func performanceExample() {
	// 创建高性能日志器（禁用调用者信息）
	perfLogger := logger.NewLogger(&logger.Config{
		Level:     logger.InfoLevel,
		Formatter: "json",
		Output:    "console",
	})
	
	// 性能测试
	start := time.Now()
	count := 10000
	
	for i := 0; i < count; i++ {
		perfLogger.WithFields(map[string]interface{}{
			"iteration": i,
			"batch":     i / 1000,
		}).Info("性能测试日志")
	}
	
	duration := time.Since(start)
	fmt.Printf("写入 %d 条日志耗时: %v\n", count, duration)
	fmt.Printf("平均每条日志耗时: %v\n", duration/time.Duration(count))
	fmt.Printf("每秒处理日志数: %.0f\n", float64(count)/duration.Seconds())
	
	// 测试不同级别的性能影响
	fmt.Println("\n测试日志级别过滤性能:")
	perfLogger.SetLevel(logger.ErrorLevel) // 只记录ERROR及以上级别
	
	start = time.Now()
	for i := 0; i < count; i++ {
		// 这些日志会被过滤掉，不会实际处理
		perfLogger.Debug("这条DEBUG日志会被过滤")
		perfLogger.Info("这条INFO日志会被过滤")
		perfLogger.Warn("这条WARN日志会被过滤")
	}
	duration = time.Since(start)
	fmt.Printf("过滤 %d 条日志耗时: %v\n", count*3, duration)
}