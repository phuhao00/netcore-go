// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

// Examples demonstrates various usage patterns of the logger package.
// This file contains comprehensive examples showing how to use different
// features of the NetCore-Go logging system.
package logger

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// ExampleBasicUsage demonstrates basic logging functionality
func ExampleBasicUsage() {
	// Create a new logger with default configuration
	logger := NewLogger(nil)
	
	// Basic logging at different levels
	logger.Trace("This is a trace message")
	logger.Debug("This is a debug message")
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	
	// Formatted logging
	logger.Infof("User %s logged in at %s", "john", time.Now().Format(time.RFC3339))
	logger.Errorf("Failed to connect to database: %v", fmt.Errorf("connection timeout"))
	
	// Logging with fields
	logger.WithField("user_id", 12345).Info("User action performed")
	logger.WithFields(map[string]interface{}{
		"user_id":   12345,
		"action":    "login",
		"ip_address": "192.168.1.100",
	}).Info("User login successful")
	
	// Logging with error
	err := fmt.Errorf("database connection failed")
	logger.WithError(err).Error("Database operation failed")
	
	// Logging with context
	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	logger.WithContext(ctx).Info("Processing request")
}

// ExampleAdvancedConfiguration demonstrates advanced logger configuration
func ExampleAdvancedConfiguration() {
	// Create file writer
	// fileWriter, err := NewFileWriter(&FileWriterConfig{
	//	Filename:      "app.log",
	//	MaxSize:       10 * 1024 * 1024, // 10MB
	//	MaxAge:        7,                 // 7 days
	//	MaxBackups:    5,
	//	Compress:      true,
	//	BufferSize:    4096,
	//	FlushInterval: 5 * time.Second,
	// })
	// if err != nil {
	//	fmt.Printf("Failed to create file writer: %v\n", err)
	//	return
	// }
	
	// Create network writer
	// networkWriter, err := NewNetworkWriter(&NetworkWriterConfig{
	//	Network: "tcp",
	//	Address: "localhost:9999",
	//	Timeout: 10 * time.Second,
	// })
	// if err != nil {
	//	fmt.Printf("Failed to create network writer: %v\n", err)
	//	return
	// }
	
	// Create multi-writer
	// multiWriter := NewMultiWriter(
	//	&ConsoleWriter{Output: os.Stdout},
	//	fileWriter,
	//	networkWriter,
	// )
	
	// Create logger with advanced configuration
	logger := NewLogger(&Config{
		Level:     DebugLevel,
		Formatter: "json",
		Output:    "file",
		FilePath:  "app.log",
	})
	
	// Add hooks
	metricsHook := NewMetricsHook([]Level{ErrorLevel, FatalLevel, PanicLevel})
	logger.AddHook(metricsHook)
	
	// Use the configured logger
	logger.Info("Application started")
	logger.WithField("component", "database").Error("Connection failed")
	
	// Get metrics
	counters := metricsHook.GetCounters()
	fmt.Printf("Error count: %d\n", counters[ErrorLevel])
}

// ExamplePerformanceMonitoring demonstrates performance monitoring features
func ExamplePerformanceMonitoring() {
	// Performance monitoring is not implemented yet
	// monitor := NewPerformanceMonitor()
	fmt.Println("Performance monitoring example - not implemented yet")
	return
	
	// Create performance monitor
	// monitor := NewPerformanceMonitor()
	
	// Set thresholds
	// monitor.SetThresholds(
	//	100*time.Millisecond, // max write time
	//	10*time.Millisecond,  // max format time
	//	0.01,                 // max error rate (1%)
	//	0.05,                 // max drop rate (5%)
	// )
	
	// Set callbacks
	// monitor.SetCallbacks(
	//	func(duration time.Duration) {
	//		fmt.Printf("Slow write detected: %v\n", duration)
	//	},
	//	func(duration time.Duration) {
	//		fmt.Printf("Slow format detected: %v\n", duration)
	//	},
	//	func(rate float64) {
	//		fmt.Printf("High error rate detected: %.2f%%\n", rate*100)
	//	},
	//	func(rate float64) {
	//		fmt.Printf("High drop rate detected: %.2f%%\n", rate*100)
	//	},
	// )
	
	// Simulate logging operations
	// for i := 0; i < 1000; i++ {
	//	start := time.Now()
	//	
	//	// Simulate write operation
	//	time.Sleep(time.Microsecond * 100)
	//	monitor.RecordWrite(time.Since(start), nil)
	//	
	//	// Simulate format operation
	//	start = time.Now()
	//	time.Sleep(time.Microsecond * 50)
	//	monitor.RecordFormat(time.Since(start), nil)
	//	
	//	// Check thresholds periodically
	//	if i%100 == 0 {
	//		monitor.CheckThresholds()
	//	}
	// }
	
	// Get metrics
	// metrics := monitor.GetMetrics().GetSnapshot()
	// fmt.Printf("Total logs: %d\n", metrics.TotalLogs)
	// fmt.Printf("Average write time: %.2f ms\n", metrics.GetAverageWriteTime()/1e6)
	// fmt.Printf("Average format time: %.2f ms\n", metrics.GetAverageFormatTime()/1e6)
	// fmt.Printf("Logs per second: %.2f\n", metrics.GetLogsPerSecond())
}

// ExampleSampling demonstrates log sampling functionality
func ExampleSampling() {
	// Random sampling (50% sample rate)
	randomSampler := NewRandomSampler(0.5)
	
	// Level-based sampling
	levelRates := map[Level]float64{
		TraceLevel: 0.1, // 10%
		DebugLevel: 0.3, // 30%
		InfoLevel:  0.8, // 80%
		WarnLevel:  1.0, // 100%
		ErrorLevel: 1.0, // 100%
	}
	levelSampler := NewLevelBasedSampler(levelRates)
	
	// Rate limit sampling (100 logs per second) - not implemented
	// rateLimitSampler := NewRateLimitSampler(100, time.Second)
	
	// Hash-based sampling (consistent sampling based on message)
	hashSampler := NewHashBasedSampler(0.3, "message")
	
	// Composite sampling (AND mode - all samplers must accept) - not implemented
	// compositeSampler := NewCompositeSampler(CompositeAND,
	//	randomSampler,
	//	levelSampler,
	// )
	
	// Test sampling
	for i := 0; i < 1000; i++ {
		entry := &Entry{
			Level:   InfoLevel,
			Message: fmt.Sprintf("Test message %d", i),
			Fields:  make(map[string]interface{}),
		}
		
		if randomSampler.Sample(entry) {
			// Log would be processed
		}
		
		if levelSampler.Sample(entry) {
			// Log would be processed
		}
		
		// if rateLimitSampler.Sample(entry) {
		//	// Log would be processed
		// }
		
		if hashSampler.Sample(entry) {
			// Log would be processed
		}
		
		// if compositeSampler.Sample(entry) {
		//	// Log would be processed
		// }
	}
	
	// Get sampling statistics
	randomStats := randomSampler.GetStats()
	fmt.Printf("Random sampler - Effective rate: %.2f%%\n", randomStats.EffectiveRate*100)
	
	levelStats := levelSampler.GetStats()
	fmt.Printf("Level sampler - Effective rate: %.2f%%\n", levelStats.EffectiveRate*100)
	
	// rateStats := rateLimitSampler.GetStats()
	// fmt.Printf("Rate limit sampler - Effective rate: %.2f%%\n", rateStats.EffectiveRate*100)
}

// ExampleValidation demonstrates field validation functionality
func ExampleValidation() {
	// Create field validator
	validator := NewFieldValidator("example_validator")
	
	// Add validation rules
	typeRule := NewTypeValidationRule(map[string]reflect.Type{
		"user_id": reflect.TypeOf(int(0)),
		"email":   reflect.TypeOf(""),
		"age":     reflect.TypeOf(int(0)),
	})
	validator.AddRule(typeRule)
	
	lengthRule := NewLengthValidationRule(1, 100, []string{"username", "email"})
	validator.AddRule(lengthRule)
	
	rangeRule := NewRangeValidationRule(0, 150, []string{"age"})
	validator.AddRule(rangeRule)
	
	regexRule, _ := NewRegexValidationRule(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, []string{"email"})
	validator.AddRule(regexRule)
	
	// enumRule := NewEnumValidationRule([]string{"active", "inactive", "pending"}, []string{"status"})
	// validator.AddRule(enumRule)
	
	// Create validation manager - not implemented
	// validationManager := NewValidationManager(ValidationManagerConfig{
	//	Enabled:          true,
	//	StopOnFirstError: false,
	//	MaxFieldCount:    20,
	//	MaxValueSize:     1024,
	//	EnableMetrics:    true,
	// })
	
	// validationManager.AddValidator(validator)
	
	// Test validation
	validEntry := &Entry{
		Message: "User registration",
		Fields: map[string]interface{}{
			"user_id":  12345,
			"username": "john_doe",
			"email":    "john@example.com",
			"age":      25,
			"status":   "active",
		},
	}
	
	invalidEntry := &Entry{
		Message: "Invalid user data",
		Fields: map[string]interface{}{
			"user_id":  "invalid", // Should be int
			"username": "",        // Too short
			"email":    "invalid-email",
			"age":      -5, // Out of range
			"status":   "unknown",
		},
	}
	
	// Test validation - not implemented
	// if err := validationManager.ValidateEntry(validEntry); err != nil {
	//	fmt.Printf("Validation failed: %v\n", err)
	// } else {
	//	fmt.Println("Valid entry passed validation")
	// }

	// if err := validationManager.ValidateEntry(invalidEntry); err != nil {
	//	fmt.Printf("Validation failed as expected: %v\n", err)
	// }

	// Get validation statistics
	// stats := validationManager.GetStats()
	// fmt.Printf("Total validations: %d\n", stats.TotalEntries)
	// fmt.Printf("Valid entries: %d\n", stats.ValidEntries)
	// fmt.Printf("Invalid entries: %d\n", stats.InvalidEntries)
	// fmt.Printf("Validation rate: %.2f%%\n", validationManager.GetValidationRate()*100)

	fmt.Println("Validation example - not fully implemented yet")
	_ = validEntry
	_ = invalidEntry
}

// ExampleRotation demonstrates log rotation functionality
func ExampleRotation() {
	// Create rotation manager
	rotationManager := NewRotationManager(RotationManagerConfig{
		MaxBackups:     5,
		MaxAge:         7 * 24 * time.Hour, // 7 days
		Compress:       true,
		CompressDelay:  5 * time.Minute,
		CleanupOnStart: true,
		EnableMetrics:  true,
	})
	
	// Add rotation strategies
	sizeStrategy := NewSizeBasedRotationStrategy(10 * 1024 * 1024) // 10MB
	timeStrategy := NewTimeBasedRotationStrategy(24 * time.Hour)   // Daily
	// countStrategy := NewCountBasedRotationStrategy(100000)         // 100k lines - not implemented
	// cronStrategy := NewCronBasedRotationStrategy("@daily")         // Daily at midnight - not implemented
	
	rotationManager.AddStrategy(sizeStrategy)
	rotationManager.AddStrategy(timeStrategy)
	// rotationManager.AddStrategy(countStrategy)
	// rotationManager.AddStrategy(cronStrategy)
	
	// Set callbacks
	rotationManager.SetCallbacks(
		func(oldFile, newFile string) {
			fmt.Printf("File rotated: %s -> %s\n", oldFile, newFile)
		},
		func(files []string) {
			fmt.Printf("Cleaned up %d old files\n", len(files))
		},
		func(err error) {
			fmt.Printf("Rotation error: %v\n", err)
		},
	)
	
	// Check if rotation is needed
	logFile := "app.log"
	shouldRotate, err := rotationManager.ShouldRotate(logFile)
	if err != nil {
		fmt.Printf("Error checking rotation: %v\n", err)
		return
	}
	
	if shouldRotate {
		if err := rotationManager.Rotate(logFile); err != nil {
			fmt.Printf("Error rotating file: %v\n", err)
		} else {
			fmt.Println("File rotated successfully")
		}
	}
	
	// Get rotation statistics
	stats := rotationManager.GetStats()
	fmt.Printf("Total rotations: %d\n", stats.TotalRotations)
	fmt.Printf("Backup files: %d\n", stats.BackupCount)
	fmt.Printf("Compressed files: %d\n", stats.CompressedCount)
}

// ExampleCompression demonstrates compression and archiving functionality
func ExampleCompression() {
	// Create archive manager - not implemented
	// archiveManager := NewArchiveManager(&ArchiveConfig{
	//	Enabled:          true,
	//	Compression:      "gzip",
	//	CompressionLevel: 6,
	//	MinFileSize:      1024,
	//	MaxFileAge:       24 * time.Hour,
	//	ArchiveDirectory: "./archive",
	//	DeleteOriginal:   true,
	//	WorkerCount:      2,
	//	QueueSize:        100,
	//	RetryAttempts:    3,
	//	RetryDelay:       time.Second,
	//	EnableMetrics:    true,
	// })
	
	fmt.Println("Compression example - not implemented yet")
	return
	
	// Set callbacks - not implemented
	// archiveManager.SetCallbacks(
	//	func(original, compressed string, ratio float64) {
	//		fmt.Printf("Compressed %s -> %s (ratio: %.2f)\n", original, compressed, ratio)
	//	},
	//	func(compressed, decompressed string) {
	//		fmt.Printf("Decompressed %s -> %s\n", compressed, decompressed)
	//	},
	//	func(source, archive string) {
	//		fmt.Printf("Archived %s -> %s\n", source, archive)
	//	},
	//	func(err error) {
	//		fmt.Printf("Archive error: %v\n", err)
	//	},
	// )
	
	// Compress files asynchronously - not implemented
	// files := []string{"app.log.1", "app.log.2", "app.log.3"}
	// for _, file := range files {
	//	compressedFile := file + ".gz"
	//	err := archiveManager.CompressFile(file, compressedFile, func(err error) {
	//		if err != nil {
	//			fmt.Printf("Compression failed for %s: %v\n", file, err)
	//		} else {
	//			fmt.Printf("Compression completed for %s\n", file)
	//		}
	//	})
	//	
	//	if err != nil {
	//		fmt.Printf("Failed to queue compression for %s: %v\n", file, err)
	//	}
	// }
	
	// Wait for compression to complete
	// time.Sleep(5 * time.Second)
	
	// Get compression statistics
	// stats := archiveManager.GetStats()
	// fmt.Printf("Total files processed: %d\n", stats.TotalFiles)
	// fmt.Printf("Compressed files: %d\n", stats.CompressedFiles)
	// fmt.Printf("Original size: %d bytes\n", stats.OriginalSize)
	// fmt.Printf("Compressed size: %d bytes\n", stats.CompressedSize)
	// fmt.Printf("Space saved: %d bytes (%.2f%%)\n", stats.SpaceSaved,
	//	float64(stats.SpaceSaved)/float64(stats.OriginalSize)*100)
	// fmt.Printf("Average compression ratio: %.2f\n", stats.CompressionRatio)
	
	// Stop archive manager
	// archiveManager.Stop()
}

// ExampleRecovery demonstrates error handling and recovery functionality
func ExampleRecovery() {
	// Create recovery manager
	recoveryManager := NewRecoveryManager(RecoveryConfig{
		EnablePanicRecovery:     true,
		EnableErrorRetry:        true,
		MaxRetryAttempts:        3,
		RetryInterval:           time.Second,
		CircuitBreakerThreshold: 5,
		RecoveryTimeout:         30 * time.Second,
		EnableMetrics:           true,
	})
	
	// Add error handlers - not implemented
	// errorHandler := NewDefaultErrorHandler(nil)
	// recoveryManager.AddErrorHandler(errorHandler)
	
	// Add recovery strategies - not implemented
	// fileStrategy := NewFileRecoveryStrategy("./recovery")
	// networkStrategy := NewNetworkRecoveryStrategy("http://backup-server/logs")
	
	// recoveryManager.AddStrategy(fileStrategy)
	// recoveryManager.AddStrategy(networkStrategy)
	
	// Set callbacks
	recoveryManager.SetCallbacks(
		func(panicValue interface{}, stackTrace []byte) {
			fmt.Printf("Panic recovered: %v\n", panicValue)
			fmt.Printf("Stack trace: %s\n", string(stackTrace))
		},
		func(err error) {
			fmt.Printf("Error occurred: %v\n", err)
		},
		func(strategy string) {
			fmt.Printf("Recovery successful using strategy: %s\n", strategy)
		},
	)
	
	// Example of panic recovery
	func() {
		defer recoveryManager.HandlePanic()
		
		// Simulate panic
		// panic("Something went wrong!")
	}()
	
	// Example of error handling
	err := fmt.Errorf("database connection failed")
	errorContext := map[string]interface{}{
		"database": "postgres",
		"host":     "localhost",
		"port":     5432,
	}
	recoveryManager.HandleError(err, errorContext)
	
	// Example of retry with backoff - not implemented
	// ctx := context.Background()
	// err = recoveryManager.Retry(ctx, func() error {
	//	// Simulate operation that might fail
	//	return fmt.Errorf("temporary failure")
	// })
	
	// if err != nil {
	//	fmt.Printf("Operation failed after retries: %v\n", err)
	// }
	
	fmt.Println("Recovery example - not fully implemented yet")
	
	// Get recovery statistics
	stats := recoveryManager.GetStats()
	fmt.Printf("Total panics: %d\n", stats.TotalPanics)
	fmt.Printf("Total errors: %d\n", stats.TotalErrors)
	fmt.Printf("Total recoveries: %d\n", stats.TotalRecoveries)
	fmt.Printf("Total retries: %d\n", stats.TotalRetries)
	fmt.Printf("Successful retries: %d\n", stats.SuccessfulRetries)
}

// ExampleDynamicConfiguration demonstrates dynamic configuration management
func ExampleDynamicConfiguration() {
	fmt.Println("Dynamic configuration example - not implemented yet")
	return
	
	// Create configuration manager - not implemented
	// configManager := NewConfigManager()
	
	// Register configuration watcher
	// configManager.Watch("logger.level", func(oldValue, newValue interface{}) {
	//	fmt.Printf("Log level changed from %v to %v\n", oldValue, newValue)
	// })
	
	// Create logger with dynamic configuration
	// logger := NewLogger(&DynamicConfig{
	//	ConfigManager: configManager,
	//	Level:         "info",
	//	Formatter:     "json",
	//	Output:        "console",
	// })
}

// ExampleMemoryOptimization demonstrates object pool usage
func ExampleMemoryOptimization() {
	// Create pool manager
	poolManager := NewPoolManager()
	
	// Use entry pool
	entry := poolManager.GetEntry()
	entry.Level = InfoLevel
	entry.Message = "Test message"
	entry.Fields["key"] = "value"
	
	// Return entry to pool when done
	poolManager.PutEntry(entry)
	
	// Use buffer pool
	buffer := poolManager.GetBuffer()
	buffer.WriteString("Hello, World!")
	
	// Return buffer to pool when done
	poolManager.PutBuffer(buffer)
	
	// Use slice pool
	slice := poolManager.GetSlice()
	slice = append(slice, []byte("data")...)
	
	// Return slice to pool when done
	poolManager.PutSlice(slice)
	
	// Use map pool
	fields := poolManager.GetMap()
	fields["user_id"] = 12345
	fields["action"] = "login"
	
	// Return map to pool when done
	poolManager.PutMap(fields)
	
	// Get pool statistics
	stats := poolManager.GetStats()
	fmt.Printf("Pool statistics: %+v\n", stats)
	// fmt.Printf("Entry gets: %d, puts: %d\n", stats.EntryGets, stats.EntryPuts)
	// fmt.Printf("Buffer gets: %d, puts: %d\n", stats.BufferGets, stats.BufferPuts)
	// fmt.Printf("Slice gets: %d, puts: %d\n", stats.SliceGets, stats.SlicePuts)
	// fmt.Printf("Map gets: %d, puts: %d\n", stats.MapGets, stats.MapPuts)
	
	// Get pool sizes - not implemented
	// sizes := poolManager.GetPoolSizes()
	// fmt.Printf("Pool sizes: %+v\n", sizes)
	
	// Get hit rates - not implemented
	// hitRates := poolManager.GetHitRate()
	// fmt.Printf("Hit rates: %+v\n", hitRates)
	fmt.Println("Pool example completed")
}

// ExampleIntegration demonstrates a complete integration example
func ExampleIntegration() {
	// Create a comprehensive logger setup
	
	// 1. Setup configuration management - not implemented
	// configManager := NewConfigManager("app_config.json")
	// configManager.LoadConfig()
	// config := configManager.GetConfig()
	
	// 2. Setup performance monitoring - not implemented
	// monitor := NewPerformanceMonitor()
	// monitor.SetThresholds(100*time.Millisecond, 10*time.Millisecond, 0.01, 0.05)
	
	// 3. Setup memory optimization - not implemented
	// poolManager := GetGlobalPoolManager()
	poolManager := NewPoolManager()
	
	// 4. Setup sampling - not implemented
	// sampler := NewLevelBasedSampler(map[Level]float64{
	//	TraceLevel: 0.1,
	//	DebugLevel: 0.3,
	//	InfoLevel:  0.8,
	//	WarnLevel:  1.0,
	//	ErrorLevel: 1.0,
	// })
	
	// 5. Setup validation - not implemented
	// validationManager := NewValidationManager(ValidationManagerConfig{
	//	Enabled:       config.Validation.Enabled,
	//	MaxFieldCount: config.Validation.MaxFieldCount,
	//	MaxValueSize:  config.Validation.MaxValueSize,
	//	EnableMetrics: true,
	// })
	
	// 6. Setup rotation - not implemented
	// rotationManager := NewRotationManager(RotationManagerConfig{
	//	MaxBackups:    config.Rotation.MaxBackups,
	//	MaxAge:        config.Rotation.MaxAge,
	//	Compress:      config.Rotation.Compress,
	//	EnableMetrics: true,
	// })
	
	// 7. Setup compression - not implemented
	// archiveManager := NewArchiveManager(&ArchiveConfig{
	//	Enabled:       config.Compression.Enabled,
	//	Compression:   config.Compression.Algorithm,
	//	MinFileSize:   config.Compression.Threshold,
	//	EnableMetrics: true,
	// })
	
	// 8. Setup recovery - not implemented
	// recoveryManager := NewRecoveryManager(RecoveryConfig{
	//	EnablePanicRecovery: true,
	//	EnableErrorRetry:    true,
	//	MaxRetryAttempts:    3,
	//	EnableMetrics:       true,
	// })
	
	// 9. Create writers - not implemented
	// fileWriter, _ := NewFileWriter(&FileWriterConfig{
	//	Filename:   "app.log",
	//	MaxSize:    100 * 1024 * 1024,
	//	MaxBackups: 5,
	//	Compress:   true,
	// })
	
	// consoleWriter := &ConsoleWriter{Output: os.Stdout}
	// multiWriter := NewMultiWriter(consoleWriter, fileWriter)
	
	// 10. Create logger with all components
	logger := NewLogger(&Config{
		Level:     InfoLevel,
		Formatter: "json",
		Output:    "file",
		FilePath:  "app.log",
	})
	
	// 11. Add hooks
	metricsHook := NewMetricsHook([]Level{ErrorLevel, FatalLevel})
	logger.AddHook(metricsHook)
	
	// 12. Use the integrated logger - simplified
	// func() {
	//	defer recoveryManager.HandlePanic()
	//	
	//	// Sample logging operations
	//	for i := 0; i < 100; i++ {
	//		entry := poolManager.GetEntry()
	//		entry.Level = InfoLevel
	//		entry.Message = fmt.Sprintf("Processing request %d", i)
	//		entry.Fields["request_id"] = fmt.Sprintf("req-%d", i)
	//		entry.Fields["user_id"] = 12345 + i
	//		
	//		// Validate entry
	//		if err := validationManager.ValidateEntry(entry); err != nil {
	//			fmt.Printf("Validation failed: %v\n", err)
	//			continue
	//		}
	//		
	//		// Sample entry
	//		if !sampler.Sample(entry) {
	//			poolManager.PutEntry(entry)
	//			continue
	//		}
	//		
	//		// Log entry
	//		logger.WithFields(entry.Fields).Info(entry.Message)
	//		
	//		// Return entry to pool
	//		poolManager.PutEntry(entry)
	//		
	//		// Check rotation
	//		if i%50 == 0 {
	//			if shouldRotate, _ := rotationManager.ShouldRotate("app.log"); shouldRotate {
	//				rotationManager.Rotate("app.log")
	//			}
	//		}
	//	}
	// }()
	
	// Simple logging example
	logger.Info("Integration example started")
	logger.Info("Processing complete")
	
	// 13. Print comprehensive statistics - simplified
	// fmt.Println("=== Performance Statistics ===")
	// perfStats := monitor.GetMetrics().GetSnapshot()
	// fmt.Printf("Total logs: %d\n", perfStats.TotalLogs)
	// fmt.Printf("Logs per second: %.2f\n", perfStats.GetLogsPerSecond())
	
	// fmt.Println("\n=== Sampling Statistics ===")
	// samplingStats := sampler.GetStats()
	// fmt.Printf("Effective sample rate: %.2f%%\n", samplingStats.EffectiveRate*100)
	
	// fmt.Println("\n=== Validation Statistics ===")
	// _ = validationManager.GetStats()
	// fmt.Printf("Validation rate: %.2f%%\n", validationManager.GetValidationRate()*100)
	
	fmt.Println("\n=== Pool Statistics ===")
	poolStats := poolManager.GetStats()
	fmt.Printf("Pool statistics: %+v\n", poolStats)
	
	// fmt.Println("\n=== Recovery Statistics ===")
	// recoveryStats := recoveryManager.GetStats()
	// fmt.Printf("Total recoveries: %d\n", recoveryStats.TotalRecoveries)
	
	// 14. Cleanup - simplified
	// logger.Close()
	// archiveManager.Stop()
	// configManager.StopWatching()
	
	fmt.Println("\nIntegration example completed successfully!")
}





