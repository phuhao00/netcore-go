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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RotationStrategy 日志轮转策略接口
type RotationStrategy interface {
	ShouldRotate(info *FileInfo) bool
	GetName() string
	GetNextFilename(currentFile string) string
}

// FileInfo 文件信息
type FileInfo struct {
	Path         string    // 文件路径
	Size         int64     // 文件大小
	CreatedTime  time.Time // 创建时间
	ModifiedTime time.Time // 修改时间
	LineCount    int64     // 行数
}

// RotationManager 日志轮转管理器
type RotationManager struct {
	mu sync.RWMutex
	
	// 轮转策略
	strategies []RotationStrategy
	
	// 配置
	config RotationManagerConfig
	
	// 统计信息
	stats RotationStats
	
	// 回调函数
	onRotation func(oldFile, newFile string)
	onCleanup  func(files []string)
	onError    func(error)
}

// RotationManagerConfig 轮转管理器配置
type RotationManagerConfig struct {
	MaxBackups     int           `json:"max_backups"`     // 最大备份文件数
	MaxAge         time.Duration `json:"max_age"`         // 最大保存时间
	Compress       bool          `json:"compress"`        // 是否压缩
	CompressDelay  time.Duration `json:"compress_delay"`  // 压缩延迟
	CleanupOnStart bool          `json:"cleanup_on_start"` // 启动时清理
	EnableMetrics  bool          `json:"enable_metrics"`  // 启用指标
}

// RotationStats 轮转统计信息
type RotationStats struct {
	TotalRotations   int64     `json:"total_rotations"`   // 总轮转次数
	LastRotationTime time.Time `json:"last_rotation_time"` // 最后轮转时间
	BackupCount      int       `json:"backup_count"`      // 备份文件数
	CompressedCount  int       `json:"compressed_count"`  // 压缩文件数
	CleanupCount     int64     `json:"cleanup_count"`     // 清理次数
	ErrorCount       int64     `json:"error_count"`       // 错误次数
}

// NewRotationManager 创建轮转管理器
func NewRotationManager(config RotationManagerConfig) *RotationManager {
	// 设置默认值
	if config.MaxBackups <= 0 {
		config.MaxBackups = 10
	}
	if config.MaxAge <= 0 {
		config.MaxAge = 30 * 24 * time.Hour // 30天
	}
	if config.CompressDelay <= 0 {
		config.CompressDelay = 5 * time.Minute
	}
	
	return &RotationManager{
		strategies: make([]RotationStrategy, 0),
		config:     config,
	}
}

// AddStrategy 添加轮转策略
func (rm *RotationManager) AddStrategy(strategy RotationStrategy) {
	rm.mu.Lock()
	rm.strategies = append(rm.strategies, strategy)
	rm.mu.Unlock()
}

// ShouldRotate 检查是否需要轮转
func (rm *RotationManager) ShouldRotate(filePath string) (bool, error) {
	info, err := rm.getFileInfo(filePath)
	if err != nil {
		return false, err
	}
	
	rm.mu.RLock()
	strategies := rm.strategies
	rm.mu.RUnlock()
	
	// 检查所有策略
	for _, strategy := range strategies {
		if strategy.ShouldRotate(info) {
			return true, nil
		}
	}
	
	return false, nil
}

// Rotate 执行日志轮转
func (rm *RotationManager) Rotate(filePath string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 文件不存在，无需轮转
	}
	
	// 生成备份文件名
	backupFile := rm.generateBackupFilename(filePath)
	
	// 重命名文件
	if err := os.Rename(filePath, backupFile); err != nil {
		rm.incrementErrorCount()
		if rm.onError != nil {
			rm.onError(fmt.Errorf("failed to rename file: %w", err))
		}
		return err
	}
	
	// 更新统计信息
	if rm.config.EnableMetrics {
		atomic.AddInt64(&rm.stats.TotalRotations, 1)
		rm.stats.LastRotationTime = time.Now()
	}
	
	// 调用轮转回调
	if rm.onRotation != nil {
		rm.onRotation(filePath, backupFile)
	}
	
	// 异步执行后续任务
	go rm.postRotationTasks(backupFile)
	
	return nil
}

// postRotationTasks 轮转后任务
func (rm *RotationManager) postRotationTasks(backupFile string) {
	// 压缩文件
	if rm.config.Compress {
		time.Sleep(rm.config.CompressDelay)
		if err := rm.compressFile(backupFile); err != nil {
			rm.incrementErrorCount()
			if rm.onError != nil {
				rm.onError(fmt.Errorf("failed to compress file: %w", err))
			}
		}
	}
	
	// 清理旧文件
	if err := rm.cleanup(filepath.Dir(backupFile)); err != nil {
		rm.incrementErrorCount()
		if rm.onError != nil {
			rm.onError(fmt.Errorf("failed to cleanup files: %w", err))
		}
	}
}

// generateBackupFilename 生成备份文件名
func (rm *RotationManager) generateBackupFilename(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// compressFile 压缩文件
func (rm *RotationManager) compressFile(filePath string) error {
	compressedPath := filePath + ".gz"
	
	// 打开源文件
	srcFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	// 创建目标文件
	dstFile, err := os.Create(compressedPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	// 创建gzip写入器
	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()
	
	// 复制数据
	if _, err := io.Copy(gzWriter, srcFile); err != nil {
		return err
	}
	
	// 删除原文件
	if err := os.Remove(filePath); err != nil {
		return err
	}
	
	if rm.config.EnableMetrics {
		rm.stats.CompressedCount++
	}
	
	return nil
}

// cleanup 清理旧文件
func (rm *RotationManager) cleanup(dir string) error {
	files, err := rm.getBackupFiles(dir)
	if err != nil {
		return err
	}
	
	// 按时间排序
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})
	
	var filesToDelete []string
	
	// 删除超过数量限制的文件
	if len(files) > rm.config.MaxBackups {
		for i := rm.config.MaxBackups; i < len(files); i++ {
			filesToDelete = append(filesToDelete, files[i].Name())
		}
	}
	
	// 删除超过时间限制的文件
	cutoff := time.Now().Add(-rm.config.MaxAge)
	for _, file := range files {
		if file.ModTime().Before(cutoff) {
			filesToDelete = append(filesToDelete, file.Name())
		}
	}
	
	// 执行删除
	for _, fileName := range filesToDelete {
		filePath := filepath.Join(dir, fileName)
		if err := os.Remove(filePath); err != nil {
			return err
		}
	}
	
	if len(filesToDelete) > 0 {
		atomic.AddInt64(&rm.stats.CleanupCount, int64(len(filesToDelete)))
		if rm.onCleanup != nil {
			rm.onCleanup(filesToDelete)
		}
	}
	
	return nil
}

// getBackupFiles 获取备份文件列表
func (rm *RotationManager) getBackupFiles(dir string) ([]os.FileInfo, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	var backupFiles []os.FileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		name := file.Name()
		// 检查是否是备份文件（包含时间戳）
		if strings.Contains(name, ".") && (strings.Contains(name, "-") || strings.HasSuffix(name, ".gz")) {
			info, err := file.Info()
			if err == nil {
				backupFiles = append(backupFiles, info)
			}
		}
	}
	
	return backupFiles, nil
}

// getFileInfo 获取文件信息
func (rm *RotationManager) getFileInfo(filePath string) (*FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	
	return &FileInfo{
		Path:         filePath,
		Size:         stat.Size(),
		CreatedTime:  stat.ModTime(), // Go中没有创建时间，使用修改时间
		ModifiedTime: stat.ModTime(),
		LineCount:    0, // 需要单独计算
	}, nil
}

// incrementErrorCount 增加错误计数
func (rm *RotationManager) incrementErrorCount() {
	atomic.AddInt64(&rm.stats.ErrorCount, 1)
}

// SetCallbacks 设置回调函数
func (rm *RotationManager) SetCallbacks(
	onRotation func(oldFile, newFile string),
	onCleanup func(files []string),
	onError func(error),
) {
	rm.mu.Lock()
	rm.onRotation = onRotation
	rm.onCleanup = onCleanup
	rm.onError = onError
	rm.mu.Unlock()
}

// GetStats 获取统计信息
func (rm *RotationManager) GetStats() RotationStats {
	return RotationStats{
		TotalRotations:   atomic.LoadInt64(&rm.stats.TotalRotations),
		LastRotationTime: rm.stats.LastRotationTime,
		BackupCount:      rm.stats.BackupCount,
		CompressedCount:  rm.stats.CompressedCount,
		CleanupCount:     atomic.LoadInt64(&rm.stats.CleanupCount),
		ErrorCount:       atomic.LoadInt64(&rm.stats.ErrorCount),
	}
}

// ResetStats 重置统计信息
func (rm *RotationManager) ResetStats() {
	atomic.StoreInt64(&rm.stats.TotalRotations, 0)
	atomic.StoreInt64(&rm.stats.CleanupCount, 0)
	atomic.StoreInt64(&rm.stats.ErrorCount, 0)
	rm.stats.LastRotationTime = time.Time{}
	rm.stats.BackupCount = 0
	rm.stats.CompressedCount = 0
}

// SizeBasedRotationStrategy 基于大小的轮转策略
type SizeBasedRotationStrategy struct {
	maxSize int64
}

// NewSizeBasedRotationStrategy 创建基于大小的轮转策略
func NewSizeBasedRotationStrategy(maxSize int64) *SizeBasedRotationStrategy {
	return &SizeBasedRotationStrategy{
		maxSize: maxSize,
	}
}

// ShouldRotate 检查是否需要轮转
func (s *SizeBasedRotationStrategy) ShouldRotate(info *FileInfo) bool {
	return info.Size >= s.maxSize
}

// GetName 获取策略名称
func (s *SizeBasedRotationStrategy) GetName() string {
	return "size_based"
}

// GetNextFilename 获取下一个文件名
func (s *SizeBasedRotationStrategy) GetNextFilename(currentFile string) string {
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// TimeBasedRotationStrategy 基于时间的轮转策略
type TimeBasedRotationStrategy struct {
	interval time.Duration
}

// NewTimeBasedRotationStrategy 创建基于时间的轮转策略
func NewTimeBasedRotationStrategy(interval time.Duration) *TimeBasedRotationStrategy {
	return &TimeBasedRotationStrategy{
		interval: interval,
	}
}

// ShouldRotate 检查是否需要轮转
func (t *TimeBasedRotationStrategy) ShouldRotate(info *FileInfo) bool {
	return time.Since(info.ModifiedTime) >= t.interval
}

// GetName 获取策略名称
func (t *TimeBasedRotationStrategy) GetName() string {
	return "time_based"
}

// GetNextFilename 获取下一个文件名
func (t *TimeBasedRotationStrategy) GetNextFilename(currentFile string) string {
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}