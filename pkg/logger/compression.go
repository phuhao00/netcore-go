// Package logger 定义NetCore-Go网络库的压缩管理系统
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ArchiveOperation 归档操作类型
type ArchiveOperation int

const (
	// OperationCompress 压缩操作
	OperationCompress ArchiveOperation = iota
	// OperationDecompress 解压操作
	OperationDecompress
	// OperationArchive 归档操作
	OperationArchive
)

// String 返回操作类型的字符串表示
func (op ArchiveOperation) String() string {
	switch op {
	case OperationCompress:
		return "compress"
	case OperationDecompress:
		return "decompress"
	case OperationArchive:
		return "archive"
	default:
		return "unknown"
	}
}

// ArchiveTask 归档任务
type ArchiveTask struct {
	ID          string           `json:"id"`
	SourceFile  string           `json:"source_file"`
	TargetFile  string           `json:"target_file"`
	Operation   ArchiveOperation `json:"operation"`
	CreatedTime time.Time        `json:"created_time"`
	Callback    func(error)      `json:"-"`
}

// ArchiveStats 归档统计信息
type ArchiveStats struct {
	TotalFiles      int64   `json:"total_files"`
	CompressedFiles int64   `json:"compressed_files"`
	ArchivedFiles   int64   `json:"archived_files"`
	OriginalSize    int64   `json:"original_size"`
	CompressedSize  int64   `json:"compressed_size"`
	SpaceSaved      int64   `json:"space_saved"`
	CompressionRatio float64 `json:"compression_ratio"`
	QueueLength     int64   `json:"queue_length"`
}

// ArchiveConfig 归档配置
type ArchiveConfig struct {
	Enabled          bool          `json:"enabled"`
	Compression      string        `json:"compression"`       // gzip, lz4, zstd
	CompressionLevel int           `json:"compression_level"` // 压缩级别
	MinFileSize      int64         `json:"min_file_size"`    // 最小文件大小
	MaxWorkers       int           `json:"max_workers"`      // 最大工作协程数
	QueueSize        int           `json:"queue_size"`       // 队列大小
	ArchiveDirectory string        `json:"archive_directory"` // 归档目录
	DeleteOriginal   bool          `json:"delete_original"`  // 是否删除原文件
	EnableMetrics    bool          `json:"enable_metrics"`   // 是否启用指标
}

// Compressor 压缩器接口
type Compressor interface {
	// Compress 压缩文件
	Compress(sourceFile, targetFile string) error
	// Decompress 解压文件
	Decompress(sourceFile, targetFile string) error
	// GetCompressionRatio 获取压缩比
	GetCompressionRatio(originalSize, compressedSize int64) float64
}

// ArchiveManager 归档管理器
type ArchiveManager struct {
	mu          sync.RWMutex
	config      *ArchiveConfig
	compressors map[string]Compressor
	workQueue   chan *ArchiveTask
	stats       *ArchiveStats
	workers     []*ArchiveWorker
	running     int32
	
	// 回调函数
	onCompress   func(string, string, float64)
	onDecompress func(string, string)
	onArchive    func(string, string)
	onError      func(error)
	
	// 控制通道
	stopChan chan struct{}
	done     chan struct{}
}

// ArchiveWorker 归档工作器
type ArchiveWorker struct {
	id      int
	manager *ArchiveManager
	stopChan chan struct{}
	done    chan struct{}
}

// NewArchiveManager 创建新的归档管理器
func NewArchiveManager(config *ArchiveConfig) *ArchiveManager {
	if config == nil {
		config = &ArchiveConfig{
			Enabled:          true,
			Compression:      "gzip",
			CompressionLevel: 6,
			MinFileSize:      1024,
			MaxWorkers:       4,
			QueueSize:        100,
			DeleteOriginal:   false,
			EnableMetrics:    true,
		}
	}
	
	am := &ArchiveManager{
		config:      config,
		compressors: make(map[string]Compressor),
		workQueue:   make(chan *ArchiveTask, config.QueueSize),
		stats:       &ArchiveStats{},
		stopChan:    make(chan struct{}),
		done:        make(chan struct{}),
	}
	
	// 注册默认压缩器
	am.RegisterCompressor("gzip", &GzipCompressor{})
	am.RegisterCompressor("lz4", &LZ4Compressor{})
	am.RegisterCompressor("zstd", &ZstdCompressor{})
	
	return am
}

// RegisterCompressor 注册压缩器
func (am *ArchiveManager) RegisterCompressor(name string, compressor Compressor) {
	am.mu.Lock()
	am.compressors[name] = compressor
	am.mu.Unlock()
}

// Start 启动归档管理器
func (am *ArchiveManager) Start() error {
	if !atomic.CompareAndSwapInt32(&am.running, 0, 1) {
		return fmt.Errorf("archive manager is already running")
	}
	
	// 启动工作器
	for i := 0; i < am.config.MaxWorkers; i++ {
		worker := &ArchiveWorker{
			id:       i,
			manager:  am,
			stopChan: make(chan struct{}),
			done:     make(chan struct{}),
		}
		am.workers = append(am.workers, worker)
		go worker.run()
	}
	
	return nil
}

// Stop 停止归档管理器
func (am *ArchiveManager) Stop() error {
	if !atomic.CompareAndSwapInt32(&am.running, 1, 0) {
		return fmt.Errorf("archive manager is not running")
	}
	
	// 停止所有工作器
	for _, worker := range am.workers {
		close(worker.stopChan)
		<-worker.done
	}
	
	// 清空工作队列
	close(am.workQueue)
	for task := range am.workQueue {
		if task.Callback != nil {
			task.Callback(fmt.Errorf("archive manager stopped"))
		}
	}
	
	am.workers = nil
	return nil
}

// run 工作器运行循环
func (w *ArchiveWorker) run() {
	defer close(w.done)
	
	for {
		select {
		case <-w.stopChan:
			return
		case task := <-w.manager.workQueue:
			if task == nil {
				return
			}
			w.processTask(task)
		}
	}
}

// processTask 处理任务
func (w *ArchiveWorker) processTask(task *ArchiveTask) {
	var err error
	
	// 更新队列长度
	if w.manager.config.EnableMetrics {
		atomic.AddInt64(&w.manager.stats.QueueLength, -1)
	}
	
	switch task.Operation {
	case OperationCompress:
		err = w.manager.compressFile(task.SourceFile, task.TargetFile)
	case OperationDecompress:
		err = w.manager.decompressFile(task.SourceFile, task.TargetFile)
	case OperationArchive:
		err = w.manager.archiveFile(task.SourceFile, task.TargetFile)
	default:
		err = fmt.Errorf("unknown operation: %v", task.Operation)
	}
	
	if err != nil && w.manager.onError != nil {
		w.manager.onError(fmt.Errorf("task %s failed: %w", task.ID, err))
	}
	
	if task.Callback != nil {
		task.Callback(err)
	}
}

// CompressFile 压缩文件
func (am *ArchiveManager) CompressFile(sourceFile, targetFile string, callback func(error)) error {
	if !am.config.Enabled {
		return fmt.Errorf("archive manager is disabled")
	}
	
	// 检查文件状态
	stat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}
	
	if stat.Size() < am.config.MinFileSize {
		return fmt.Errorf("file size %d is below minimum %d", stat.Size(), am.config.MinFileSize)
	}
	
	// 创建压缩任务
	task := &ArchiveTask{
		ID:          fmt.Sprintf("compress_%d", time.Now().UnixNano()),
		SourceFile:  sourceFile,
		TargetFile:  targetFile,
		Operation:   OperationCompress,
		CreatedTime: time.Now(),
		Callback:    callback,
	}
	
	// 添加到工作队列
	select {
	case am.workQueue <- task:
		if am.config.EnableMetrics {
			atomic.AddInt64(&am.stats.QueueLength, 1)
		}
		return nil
	default:
		return fmt.Errorf("work queue is full")
	}
}

// compressFile 压缩文件实现
func (am *ArchiveManager) compressFile(sourceFile, targetFile string) error {
	am.mu.RLock()
	compressor, exists := am.compressors[am.config.Compression]
	am.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("compressor %s not found", am.config.Compression)
	}
	
	// 获取原文件大小
	stat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}
	originalSize := stat.Size()
	
	// 执行压缩
	if err := compressor.Compress(sourceFile, targetFile); err != nil {
		return err
	}
	
	// 获取压缩后文件大小
	stat, err = os.Stat(targetFile)
	if err != nil {
		return err
	}
	compressedSize := stat.Size()
	
	// 计算压缩比
	compressionRatio := compressor.GetCompressionRatio(originalSize, compressedSize)
	
	// 更新统计信息
	if am.config.EnableMetrics {
		atomic.AddInt64(&am.stats.TotalFiles, 1)
		atomic.AddInt64(&am.stats.CompressedFiles, 1)
		atomic.AddInt64(&am.stats.OriginalSize, originalSize)
		atomic.AddInt64(&am.stats.CompressedSize, compressedSize)
		atomic.AddInt64(&am.stats.SpaceSaved, originalSize-compressedSize)
		
		// 更新总体压缩比
		totalFiles := atomic.LoadInt64(&am.stats.CompressedFiles)
		if totalFiles > 0 {
			totalOriginal := atomic.LoadInt64(&am.stats.OriginalSize)
			totalCompressed := atomic.LoadInt64(&am.stats.CompressedSize)
			am.stats.CompressionRatio = float64(totalCompressed) / float64(totalOriginal)
		}
	}
	
	// 删除原文件（如果配置允许）
	if am.config.DeleteOriginal {
		if err := os.Remove(sourceFile); err != nil {
			return fmt.Errorf("failed to delete original file: %w", err)
		}
	}
	
	// 触发压缩回调
	if am.onCompress != nil {
		am.onCompress(sourceFile, targetFile, compressionRatio)
	}
	
	return nil
}

// DecompressFile 解压文件
func (am *ArchiveManager) DecompressFile(sourceFile, targetFile string, callback func(error)) error {
	if !am.config.Enabled {
		return fmt.Errorf("archive manager is disabled")
	}
	
	// 创建解压任务
	task := &ArchiveTask{
		ID:          fmt.Sprintf("decompress_%d", time.Now().UnixNano()),
		SourceFile:  sourceFile,
		TargetFile:  targetFile,
		Operation:   OperationDecompress,
		CreatedTime: time.Now(),
		Callback:    callback,
	}
	
	// 添加到工作队列
	select {
	case am.workQueue <- task:
		if am.config.EnableMetrics {
			atomic.AddInt64(&am.stats.QueueLength, 1)
		}
		return nil
	default:
		return fmt.Errorf("work queue is full")
	}
}

// decompressFile 解压文件实现
func (am *ArchiveManager) decompressFile(sourceFile, targetFile string) error {
	// 根据文件扩展名确定压缩器
	var compressorName string
	switch {
	case strings.HasSuffix(sourceFile, ".gz"):
		compressorName = "gzip"
	case strings.HasSuffix(sourceFile, ".lz4"):
		compressorName = "lz4"
	case strings.HasSuffix(sourceFile, ".zst"):
		compressorName = "zstd"
	default:
		compressorName = am.config.Compression
	}
	
	am.mu.RLock()
	compressor, exists := am.compressors[compressorName]
	am.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("compressor %s not found", compressorName)
	}
	
	// 执行解压
	if err := compressor.Decompress(sourceFile, targetFile); err != nil {
		return err
	}
	
	// 触发解压回调
	if am.onDecompress != nil {
		am.onDecompress(sourceFile, targetFile)
	}
	
	return nil
}

// archiveFile 归档文件
func (am *ArchiveManager) archiveFile(sourceFile, targetFile string) error {
	// 创建归档目录
	if am.config.ArchiveDirectory != "" {
		if err := os.MkdirAll(am.config.ArchiveDirectory, 0755); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}
		
		// 更新目标文件路径
		targetFile = filepath.Join(am.config.ArchiveDirectory, filepath.Base(targetFile))
	}
	
	// 移动文件到归档目录
	if err := os.Rename(sourceFile, targetFile); err != nil {
		return fmt.Errorf("failed to archive file: %w", err)
	}
	
	// 更新统计信息
	if am.config.EnableMetrics {
		atomic.AddInt64(&am.stats.ArchivedFiles, 1)
	}
	
	// 触发归档回调
	if am.onArchive != nil {
		am.onArchive(sourceFile, targetFile)
	}
	
	return nil
}

// SetCallbacks 设置回调函数
func (am *ArchiveManager) SetCallbacks(
	onCompress func(string, string, float64),
	onDecompress func(string, string),
	onArchive func(string, string),
	onError func(error),
) {
	am.mu.Lock()
	am.onCompress = onCompress
	am.onDecompress = onDecompress
	am.onArchive = onArchive
	am.onError = onError
	am.mu.Unlock()
}

// GetStats 获取统计信息
func (am *ArchiveManager) GetStats() *ArchiveStats {
	return am.stats
}

// 压缩器实现

// GzipCompressor Gzip压缩器
type GzipCompressor struct{}

// Compress 压缩文件
func (g *GzipCompressor) Compress(sourceFile, targetFile string) error {
	// 这里应该实现实际的gzip压缩逻辑
	return fmt.Errorf("gzip compression not implemented")
}

// Decompress 解压文件
func (g *GzipCompressor) Decompress(sourceFile, targetFile string) error {
	// 这里应该实现实际的gzip解压逻辑
	return fmt.Errorf("gzip decompression not implemented")
}

// GetCompressionRatio 获取压缩比
func (g *GzipCompressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// LZ4Compressor LZ4压缩器
type LZ4Compressor struct{}

// Compress 压缩文件
func (l *LZ4Compressor) Compress(sourceFile, targetFile string) error {
	return fmt.Errorf("lz4 compression not implemented")
}

// Decompress 解压文件
func (l *LZ4Compressor) Decompress(sourceFile, targetFile string) error {
	return fmt.Errorf("lz4 decompression not implemented")
}

// GetCompressionRatio 获取压缩比
func (l *LZ4Compressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// ZstdCompressor Zstd压缩器
type ZstdCompressor struct{}

// Compress 压缩文件
func (z *ZstdCompressor) Compress(sourceFile, targetFile string) error {
	return fmt.Errorf("zstd compression not implemented")
}

// Decompress 解压文件
func (z *ZstdCompressor) Decompress(sourceFile, targetFile string) error {
	return fmt.Errorf("zstd decompression not implemented")
}

// GetCompressionRatio 获取压缩比
func (z *ZstdCompressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}