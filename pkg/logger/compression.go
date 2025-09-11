// Package logger provides high-performance logging functionality for NetCore-Go
// Author: NetCore-Go Team
// Created: 2024

package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Compressor `n�?type Compressor interface {
	Compress(src, dst string) error
	Decompress(src, dst string) error
	GetName() string
	GetExtension() string
	GetCompressionRatio(originalSize, compressedSize int64) float64
}

// ArchiveManager `n�?type ArchiveManager struct {
	mu sync.RWMutex
	
	// `n�?	compressors map[string]Compressor
	
	// `n
	config ArchiveConfig
	
	// `n
	stats ArchiveStats
	
	// `n
	workQueue chan *ArchiveTask
	workers   int
	stopChan  chan struct{}
	done      chan struct{}
	
	// `n
	onCompress   func(string, string, float64) // `n�? `n, `n�?	onDecompress func(string, string)          // `n, `n
	onArchive    func(string, string)          // `n�? `n
	onError      func(error)
}

// ArchiveConfig `n`ntype ArchiveConfig struct {
	Enabled           bool          `json:"enabled"`            // `n
	Compression       string        `json:"compression"`        // `n: gzip, lz4, zstd
	CompressionLevel  int           `json:"compression_level"`  // `n
	MinFileSize       int64         `json:"min_file_size"`      // `n（`n�?	MaxFileAge        time.Duration `json:"max_file_age"`       // `n�?	ArchiveDirectory  string        `json:"archive_directory"`  // `n
	DeleteOriginal    bool          `json:"delete_original"`    // `n�?	WorkerCount       int           `json:"worker_count"`       // `n�?	QueueSize         int           `json:"queue_size"`         // `n
	BatchSize         int           `json:"batch_size"`         // `n�?	RetryAttempts     int           `json:"retry_attempts"`     // `n
	RetryDelay        time.Duration `json:"retry_delay"`        // `n
	EnableMetrics     bool          `json:"enable_metrics"`     // `n
}

// ArchiveStats `n`ntype ArchiveStats struct {
	TotalFiles        int64   `json:"total_files"`        // `n
	CompressedFiles   int64   `json:"compressed_files"`   // `n�?	ArchivedFiles     int64   `json:"archived_files"`     // `n�?	FailedFiles       int64   `json:"failed_files"`       // `n�?	OriginalSize      int64   `json:"original_size"`      // `n
	CompressedSize    int64   `json:"compressed_size"`    // `n�?	SpaceSaved        int64   `json:"space_saved"`        // `n
	CompressionRatio  float64 `json:"compression_ratio"`  // `n�?	ProcessingTime    int64   `json:"processing_time_ns"` // `n（`n）
	QueueLength       int64   `json:"queue_length"`       // `n
	ActiveWorkers     int64   `json:"active_workers"`     // `n
}

// ArchiveTask `n`ntype ArchiveTask struct {
	ID          string
	SourceFile  string
	TargetFile  string
	Operation   ArchiveOperation
	RetryCount  int
	CreatedTime time.Time
	Callback    func(error)
}

// ArchiveOperation `n`ntype ArchiveOperation int

const (
	OperationCompress ArchiveOperation = iota
	OperationDecompress
	OperationArchive
)

// String `n�?func (op ArchiveOperation) String() string {
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

// NewArchiveManager `n�?func NewArchiveManager(config ArchiveConfig) *ArchiveManager {
	// `n�?	if config.CompressionLevel <= 0 {
		config.CompressionLevel = 6
	}
	if config.MinFileSize <= 0 {
		config.MinFileSize = 1024 // 1KB
	}
	if config.MaxFileAge <= 0 {
		config.MaxFileAge = 24 * time.Hour
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = 2
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = time.Second
	}
	if config.Compression == "" {
		config.Compression = "gzip"
	}
	
	am := &ArchiveManager{
		compressors: make(map[string]Compressor),
		config:      config,
		workQueue:   make(chan *ArchiveTask, config.QueueSize),
		workers:     config.WorkerCount,
		stopChan:    make(chan struct{}),
		done:        make(chan struct{}),
	}
	
	// `n�?	am.RegisterCompressor(NewGzipCompressor(config.CompressionLevel))
	am.RegisterCompressor(NewLZ4Compressor())
	am.RegisterCompressor(NewZstdCompressor(config.CompressionLevel))
	
	// `n`nif config.Enabled {
		am.Start()
	}
	
	return am
}

// RegisterCompressor `n�?func (am *ArchiveManager) RegisterCompressor(compressor Compressor) {
	am.mu.Lock()
	am.compressors[compressor.GetName()] = compressor
	am.mu.Unlock()
}

// Start `n�?func (am *ArchiveManager) Start() {
	for i := 0; i < am.workers; i++ {
		go am.worker(i)
	}
}

// Stop `n�?func (am *ArchiveManager) Stop() {
	close(am.stopChan)
	<-am.done
}

// worker `n`nfunc (am *ArchiveManager) worker(id int) {
	defer func() {
		if id == 0 { // `nworker`ndone`n
			close(am.done)
		}
	}()
	
	for {
		select {
		case <-am.stopChan:
			return
		case task := <-am.workQueue:
			if task != nil {
				atomic.AddInt64(&am.stats.ActiveWorkers, 1)
				am.processTask(task)
				atomic.AddInt64(&am.stats.ActiveWorkers, -1)
			}
		}
	}
}

// processTask `n`nfunc (am *ArchiveManager) processTask(task *ArchiveTask) {
	start := time.Now()
	var err error
	
	switch task.Operation {
	case OperationCompress:
		err = am.compressFile(task.SourceFile, task.TargetFile)
	case OperationDecompress:
		err = am.decompressFile(task.SourceFile, task.TargetFile)
	case OperationArchive:
		err = am.archiveFile(task.SourceFile, task.TargetFile)
	}
	
	// `n`nif am.config.EnableMetrics {
		atomic.AddInt64(&am.stats.ProcessingTime, time.Since(start).Nanoseconds())
	}
	
	// `n�?	if err != nil {
		if task.RetryCount < am.config.RetryAttempts {
			task.RetryCount++
			time.Sleep(am.config.RetryDelay)
			
			// `n
			select {
			case am.workQueue <- task:
			default:
				// `n，`n`nif am.config.EnableMetrics {
					atomic.AddInt64(&am.stats.FailedFiles, 1)
				}
				if am.onError != nil {
					am.onError(fmt.Errorf("task queue full, dropping task: %s", task.ID))
				}
			}
			return
		}
		
		// `n`nif am.config.EnableMetrics {
			atomic.AddInt64(&am.stats.FailedFiles, 1)
		}
		if am.onError != nil {
			am.onError(fmt.Errorf("task failed after %d retries: %s, error: %w", am.config.RetryAttempts, task.ID, err))
		}
	}
	
	// `n`nif task.Callback != nil {
		task.Callback(err)
	}
}

// CompressFile `n（`n）
func (am *ArchiveManager) CompressFile(sourceFile, targetFile string, callback func(error)) error {
	if !am.config.Enabled {
		return fmt.Errorf("archive manager is disabled")
	}
	
	// `n�?	stat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}
	
	if stat.Size() < am.config.MinFileSize {
		return fmt.Errorf("file size %d is below minimum %d", stat.Size(), am.config.MinFileSize)
	}
	
	// `n
	task := &ArchiveTask{
		ID:          fmt.Sprintf("compress_%d", time.Now().UnixNano()),
		SourceFile:  sourceFile,
		TargetFile:  targetFile,
		Operation:   OperationCompress,
		CreatedTime: time.Now(),
		Callback:    callback,
	}
	
	// `n
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

// compressFile `n（`n）
func (am *ArchiveManager) compressFile(sourceFile, targetFile string) error {
	am.mu.RLock()
	compressor, exists := am.compressors[am.config.Compression]
	am.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("compressor %s not found", am.config.Compression)
	}
	
	// `n�?	stat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}
	originalSize := stat.Size()
	
	// `n`nif err := compressor.Compress(sourceFile, targetFile); err != nil {
		return err
	}
	
	// `n�?	stat, err = os.Stat(targetFile)
	if err != nil {
		return err
	}
	compressedSize := stat.Size()
	
	// `n�?	compressionRatio := compressor.GetCompressionRatio(originalSize, compressedSize)
	
	// `n`nif am.config.EnableMetrics {
		atomic.AddInt64(&am.stats.TotalFiles, 1)
		atomic.AddInt64(&am.stats.CompressedFiles, 1)
		atomic.AddInt64(&am.stats.OriginalSize, originalSize)
		atomic.AddInt64(&am.stats.CompressedSize, compressedSize)
		atomic.AddInt64(&am.stats.SpaceSaved, originalSize-compressedSize)
		
		// `n�?		totalFiles := atomic.LoadInt64(&am.stats.CompressedFiles)
		if totalFiles > 0 {
			totalOriginal := atomic.LoadInt64(&am.stats.OriginalSize)
			totalCompressed := atomic.LoadInt64(&am.stats.CompressedSize)
			am.stats.CompressionRatio = float64(totalCompressed) / float64(totalOriginal)
		}
	}
	
	// `n�?	if am.config.DeleteOriginal {
		if err := os.Remove(sourceFile); err != nil {
			return fmt.Errorf("failed to delete original file: %w", err)
		}
	}
	
	// `n`nif am.onCompress != nil {
		am.onCompress(sourceFile, targetFile, compressionRatio)
	}
	
	return nil
}

// DecompressFile `n（`n）
func (am *ArchiveManager) DecompressFile(sourceFile, targetFile string, callback func(error)) error {
	if !am.config.Enabled {
		return fmt.Errorf("archive manager is disabled")
	}
	
	// `n
	task := &ArchiveTask{
		ID:          fmt.Sprintf("decompress_%d", time.Now().UnixNano()),
		SourceFile:  sourceFile,
		TargetFile:  targetFile,
		Operation:   OperationDecompress,
		CreatedTime: time.Now(),
		Callback:    callback,
	}
	
	// `n
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

// decompressFile `n（`n）
func (am *ArchiveManager) decompressFile(sourceFile, targetFile string) error {
	// `n`nvar compressorName string
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
	
	// `n`nif err := compressor.Decompress(sourceFile, targetFile); err != nil {
		return err
	}
	
	// `n`nif am.onDecompress != nil {
		am.onDecompress(sourceFile, targetFile)
	}
	
	return nil
}

// archiveFile `n`nfunc (am *ArchiveManager) archiveFile(sourceFile, targetFile string) error {
	// `n`nif am.config.ArchiveDirectory != "" {
		if err := os.MkdirAll(am.config.ArchiveDirectory, 0755); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}
		
		// `n
		targetFile = filepath.Join(am.config.ArchiveDirectory, filepath.Base(targetFile))
	}
	
	// `n�?	if err := os.Rename(sourceFile, targetFile); err != nil {
		return fmt.Errorf("failed to archive file: %w", err)
	}
	
	// `n`nif am.config.EnableMetrics {
		atomic.AddInt64(&am.stats.ArchivedFiles, 1)
	}
	
	// `n`nif am.onArchive != nil {
		am.onArchive(sourceFile, targetFile)
	}
	
	return nil
}

// SetCallbacks `n`nfunc (am *ArchiveManager) SetCallbacks(
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

// GetStats `n`nfunc (am *ArchiveManager) GetStats() ArchiveStats {
	am.mu.RLock()
	stats := ArchiveStats{
		TotalFiles:       atomic.LoadInt64(&am.stats.TotalFiles),
		CompressedFiles:  atomic.LoadInt64(&am.stats.CompressedFiles),
		ArchivedFiles:    atomic.LoadInt64(&am.stats.ArchivedFiles),
		FailedFiles:      atomic.LoadInt64(&am.stats.FailedFiles),
		OriginalSize:     atomic.LoadInt64(&am.stats.OriginalSize),
		CompressedSize:   atomic.LoadInt64(&am.stats.CompressedSize),
		SpaceSaved:       atomic.LoadInt64(&am.stats.SpaceSaved),
		CompressionRatio: am.stats.CompressionRatio,
		ProcessingTime:   atomic.LoadInt64(&am.stats.ProcessingTime),
		QueueLength:      atomic.LoadInt64(&am.stats.QueueLength),
		ActiveWorkers:    atomic.LoadInt64(&am.stats.ActiveWorkers),
	}
	am.mu.RUnlock()
	return stats
}

// ResetStats `n`nfunc (am *ArchiveManager) ResetStats() {
	atomic.StoreInt64(&am.stats.TotalFiles, 0)
	atomic.StoreInt64(&am.stats.CompressedFiles, 0)
	atomic.StoreInt64(&am.stats.ArchivedFiles, 0)
	atomic.StoreInt64(&am.stats.FailedFiles, 0)
	atomic.StoreInt64(&am.stats.OriginalSize, 0)
	atomic.StoreInt64(&am.stats.CompressedSize, 0)
	atomic.StoreInt64(&am.stats.SpaceSaved, 0)
	atomic.StoreInt64(&am.stats.ProcessingTime, 0)
	atomic.StoreInt64(&am.stats.QueueLength, 0)
	
	am.mu.Lock()
	am.stats.CompressionRatio = 0
	am.mu.Unlock()
}

// GzipCompressor Gzip`n�?type GzipCompressor struct {
	name  string
	level int
}

// NewGzipCompressor `nGzip`n�?func NewGzipCompressor(level int) *GzipCompressor {
	if level < gzip.DefaultCompression || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	
	return &GzipCompressor{
		name:  "gzip",
		level: level,
	}
}

// Compress `n`nfunc (c *GzipCompressor) Compress(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	gzWriter, err := gzip.NewWriterLevel(dstFile, c.level)
	if err != nil {
		return err
	}
	defer gzWriter.Close()
	
	_, err = io.Copy(gzWriter, srcFile)
	return err
}

// Decompress `n`nfunc (c *GzipCompressor) Decompress(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	gzReader, err := gzip.NewReader(srcFile)
	if err != nil {
		return err
	}
	defer gzReader.Close()
	
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	_, err = io.Copy(dstFile, gzReader)
	return err
}

// GetName `n�?func (c *GzipCompressor) GetName() string {
	return c.name
}

// GetExtension `n�?func (c *GzipCompressor) GetExtension() string {
	return ".gz"
}

// GetCompressionRatio `n�?func (c *GzipCompressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// LZ4Compressor LZ4`n（`n）
type LZ4Compressor struct {
	name string
}

// NewLZ4Compressor `nLZ4`n�?func NewLZ4Compressor() *LZ4Compressor {
	return &LZ4Compressor{
		name: "lz4",
	}
}

// Compress `n（`n，`nLZ4`n）
func (c *LZ4Compressor) Compress(src, dst string) error {
	// `nLZ4`n�?	// `n，`ngzip`n
	gzipCompressor := NewGzipCompressor(gzip.DefaultCompression)
	return gzipCompressor.Compress(src, dst+".gz")
}

// Decompress `n（`n）
func (c *LZ4Compressor) Decompress(src, dst string) error {
	// `nLZ4`n�?	// `n，`ngzip`n
	gzipCompressor := NewGzipCompressor(gzip.DefaultCompression)
	return gzipCompressor.Decompress(src, dst)
}

// GetName `n�?func (c *LZ4Compressor) GetName() string {
	return c.name
}

// GetExtension `n�?func (c *LZ4Compressor) GetExtension() string {
	return ".lz4"
}

// GetCompressionRatio `n�?func (c *LZ4Compressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// ZstdCompressor Zstd`n（`n）
type ZstdCompressor struct {
	name  string
	level int
}

// NewZstdCompressor `nZstd`n�?func NewZstdCompressor(level int) *ZstdCompressor {
	return &ZstdCompressor{
		name:  "zstd",
		level: level,
	}
}

// Compress `n（`n，`nZstd`n）
func (c *ZstdCompressor) Compress(src, dst string) error {
	// `nZstd`n�?	// `n，`ngzip`n
	gzipCompressor := NewGzipCompressor(c.level)
	return gzipCompressor.Compress(src, dst+".gz")
}

// Decompress `n（`n）
func (c *ZstdCompressor) Decompress(src, dst string) error {
	// `nZstd`n�?	// `n，`ngzip`n
	gzipCompressor := NewGzipCompressor(c.level)
	return gzipCompressor.Decompress(src, dst)
}

// GetName `n�?func (c *ZstdCompressor) GetName() string {
	return c.name
}

// GetExtension `n�?func (c *ZstdCompressor) GetExtension() string {
	return ".zst"
}

// GetCompressionRatio `n�?func (c *ZstdCompressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}






