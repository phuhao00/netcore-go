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
	"time"
)

// RotationStrategy `n`ntype RotationStrategy interface {
	ShouldRotate(info *FileInfo) bool
	GetName() string
	GetNextFilename(currentFile string) string
}

// FileInfo `n`ntype FileInfo struct {
	Path         string    // `n
	Size         int64     // `n
	CreatedTime  time.Time // `n
	ModifiedTime time.Time // `n
	LineCount    int64     // `n
}

// RotationManager `n�?type RotationManager struct {
	mu sync.RWMutex
	
	// `n
	strategies []RotationStrategy
	
	// `n
	config RotationManagerConfig
	
	// `n
	stats RotationStats
	
	// `n
	onRotation func(oldFile, newFile string)
	onCleanup  func(files []string)
	onError    func(error)
}

// RotationManagerConfig `n�?type RotationManagerConfig struct {
	MaxBackups    int           `json:"max_backups"`    // `n�?	MaxAge        time.Duration `json:"max_age"`        // `n�?	Compress      bool          `json:"compress"`       // `n
	CompressDelay time.Duration `json:"compress_delay"` // `n
	CleanupOnStart bool         `json:"cleanup_on_start"` // `n�?	EnableMetrics bool          `json:"enable_metrics"` // `n
}

// RotationStats `n`ntype RotationStats struct {
	TotalRotations   int64     `json:"total_rotations"`   // `n�?	LastRotationTime time.Time `json:"last_rotation_time"` // `n�?	BackupCount      int       `json:"backup_count"`      // `n
	CompressedCount  int       `json:"compressed_count"`  // `n
	CleanupCount     int64     `json:"cleanup_count"`     // `n
	ErrorCount       int64     `json:"error_count"`       // `n
}

// NewRotationManager `n�?func NewRotationManager(config RotationManagerConfig) *RotationManager {
	// `n�?	if config.MaxBackups <= 0 {
		config.MaxBackups = 10
	}
	if config.MaxAge <= 0 {
		config.MaxAge = 30 * 24 * time.Hour // 30�?	}
	if config.CompressDelay <= 0 {
		config.CompressDelay = 5 * time.Minute
	}
	
	return &RotationManager{
		strategies: make([]RotationStrategy, 0),
		config:     config,
	}
}

// AddStrategy `n`nfunc (rm *RotationManager) AddStrategy(strategy RotationStrategy) {
	rm.mu.Lock()
	rm.strategies = append(rm.strategies, strategy)
	rm.mu.Unlock()
}

// ShouldRotate `n�?func (rm *RotationManager) ShouldRotate(filePath string) (bool, error) {
	info, err := rm.getFileInfo(filePath)
	if err != nil {
		return false, err
	}
	
	rm.mu.RLock()
	strategies := rm.strategies
	rm.mu.RUnlock()
	
	// `n�?	for _, strategy := range strategies {
		if strategy.ShouldRotate(info) {
			return true, nil
		}
	}
	
	return false, nil
}

// Rotate `n`nfunc (rm *RotationManager) Rotate(filePath string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// `n�?	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // `n，`n
	}
	
	// `n�?	backupFile := rm.generateBackupFilename(filePath)
	
	// `n�?	if err := os.Rename(filePath, backupFile); err != nil {
		rm.incrementErrorCount()
		if rm.onError != nil {
			rm.onError(fmt.Errorf("failed to rename file: %w", err))
		}
		return err
	}
	
	// `n`nif rm.config.EnableMetrics {
		rm.stats.TotalRotations++
		rm.stats.LastRotationTime = time.Now()
	}
	
	// `n`nif rm.onRotation != nil {
		rm.onRotation(filePath, backupFile)
	}
	
	// `n�?	go rm.postRotationTasks(backupFile)
	
	return nil
}

// postRotationTasks `n�?func (rm *RotationManager) postRotationTasks(backupFile string) {
	// `n`nif rm.config.Compress {
		time.Sleep(rm.config.CompressDelay)
		if err := rm.compressFile(backupFile); err != nil {
			rm.incrementErrorCount()
			if rm.onError != nil {
				rm.onError(fmt.Errorf("failed to compress file: %w", err))
			}
		}
	}
	
	// `n�?	if err := rm.cleanup(filepath.Dir(backupFile)); err != nil {
		rm.incrementErrorCount()
		if rm.onError != nil {
			rm.onError(fmt.Errorf("failed to cleanup files: %w", err))
		}
	}
}

// generateBackupFilename `n�?func (rm *RotationManager) generateBackupFilename(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// compressFile `n`nfunc (rm *RotationManager) compressFile(filePath string) error {
	compressedPath := filePath + ".gz"
	
	// `n�?	srcFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	// `n
	dstFile, err := os.Create(compressedPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	// `ngzip`n�?	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()
	
	// `n`nif _, err := io.Copy(gzWriter, srcFile); err != nil {
		return err
	}
	
	// `n�?	if err := os.Remove(filePath); err != nil {
		return err
	}
	
	if rm.config.EnableMetrics {
		rm.stats.CompressedCount++
	}
	
	return nil
}

// cleanup `n�?func (rm *RotationManager) cleanup(dir string) error {
	// `n�?	files, err := rm.getBackupFiles(dir)
	if err != nil {
		return err
	}
	
	var filesToDelete []string
	
	// `n�?	if rm.config.MaxBackups > 0 && len(files) > rm.config.MaxBackups {
		// `n（`n�?		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime().After(files[j].ModTime())
		})
		
		// `n�?		for i := rm.config.MaxBackups; i < len(files); i++ {
			filesToDelete = append(filesToDelete, files[i].Name())
		}
	}
	
	// `n�?	if rm.config.MaxAge > 0 {
		cutoff := time.Now().Add(-rm.config.MaxAge)
		for _, file := range files {
			if file.ModTime().Before(cutoff) {
				filesToDelete = append(filesToDelete, file.Name())
			}
		}
	}
	
	// `n`nfor _, filename := range filesToDelete {
		filePath := filepath.Join(dir, filename)
		if err := os.Remove(filePath); err != nil {
			return err
		}
	}
	
	if len(filesToDelete) > 0 {
		if rm.config.EnableMetrics {
			rm.stats.CleanupCount++
		}
		
		// `n`nif rm.onCleanup != nil {
			rm.onCleanup(filesToDelete)
		}
	}
	
	return nil
}

// getBackupFiles `n`nfunc (rm *RotationManager) getBackupFiles(dir string) ([]os.FileInfo, error) {
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
		// `n（`n�?		if strings.Contains(name, "-") && (strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".gz")) {
			info, err := file.Info()
			if err == nil {
				backupFiles = append(backupFiles, info)
			}
		}
	}
	
	return backupFiles, nil
}

// getFileInfo `n`nfunc (rm *RotationManager) getFileInfo(filePath string) (*FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	
	// `n（`n）
	lineCount, _ := rm.countLines(filePath)
	
	return &FileInfo{
		Path:         filePath,
		Size:         stat.Size(),
		CreatedTime:  stat.ModTime(), // `n，`n
		ModifiedTime: stat.ModTime(),
		LineCount:    lineCount,
	}, nil
}

// countLines `n`nfunc (rm *RotationManager) countLines(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	
	var count int64
	buf := make([]byte, 32*1024) // 32KB`n�?	
	for {
		n, err := file.Read(buf)
		if n == 0 {
			break
		}
		
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				count++
			}
		}
		
		if err != nil {
			break
		}
	}
	
	return count, nil
}

// incrementErrorCount `n`nfunc (rm *RotationManager) incrementErrorCount() {
	if rm.config.EnableMetrics {
		rm.stats.ErrorCount++
	}
}

// SetCallbacks `n`nfunc (rm *RotationManager) SetCallbacks(
	onRotation func(string, string),
	onCleanup func([]string),
	onError func(error),
) {
	rm.mu.Lock()
	rm.onRotation = onRotation
	rm.onCleanup = onCleanup
	rm.onError = onError
	rm.mu.Unlock()
}

// GetStats `n`nfunc (rm *RotationManager) GetStats() RotationStats {
	rm.mu.RLock()
	stats := rm.stats
	rm.mu.RUnlock()
	return stats
}

// ResetStats `n`nfunc (rm *RotationManager) ResetStats() {
	rm.mu.Lock()
	rm.stats = RotationStats{}
	rm.mu.Unlock()
}

// SizeBasedRotationStrategy `n�?type SizeBasedRotationStrategy struct {
	name    string
	maxSize int64 // `n（`n�?}

// NewSizeBasedRotationStrategy `n�?func NewSizeBasedRotationStrategy(maxSize int64) *SizeBasedRotationStrategy {
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024 // `n100MB
	}
	
	return &SizeBasedRotationStrategy{
		name:    "size_based",
		maxSize: maxSize,
	}
}

// ShouldRotate `n�?func (s *SizeBasedRotationStrategy) ShouldRotate(info *FileInfo) bool {
	return info.Size >= s.maxSize
}

// GetName `n`nfunc (s *SizeBasedRotationStrategy) GetName() string {
	return s.name
}

// GetNextFilename `n`nfunc (s *SizeBasedRotationStrategy) GetNextFilename(currentFile string) string {
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// TimeBasedRotationStrategy `n�?type TimeBasedRotationStrategy struct {
	name     string
	interval time.Duration // `n
	lastRotation time.Time // `n
	mu       sync.RWMutex
}

// NewTimeBasedRotationStrategy `n�?func NewTimeBasedRotationStrategy(interval time.Duration) *TimeBasedRotationStrategy {
	if interval <= 0 {
		interval = 24 * time.Hour // `n
	}
	
	return &TimeBasedRotationStrategy{
		name:     "time_based",
		interval: interval,
		lastRotation: time.Now(),
	}
}

// ShouldRotate `n�?func (s *TimeBasedRotationStrategy) ShouldRotate(info *FileInfo) bool {
	s.mu.RLock()
	lastRotation := s.lastRotation
	interval := s.interval
	s.mu.RUnlock()
	
	if time.Since(lastRotation) >= interval {
		s.mu.Lock()
		s.lastRotation = time.Now()
		s.mu.Unlock()
		return true
	}
	
	return false
}

// GetName `n`nfunc (s *TimeBasedRotationStrategy) GetName() string {
	return s.name
}

// GetNextFilename `n`nfunc (s *TimeBasedRotationStrategy) GetNextFilename(currentFile string) string {
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// CountBasedRotationStrategy `n�?type CountBasedRotationStrategy struct {
	name     string
	maxLines int64 // `n�?}

// NewCountBasedRotationStrategy `n�?func NewCountBasedRotationStrategy(maxLines int64) *CountBasedRotationStrategy {
	if maxLines <= 0 {
		maxLines = 1000000 // `n100`n
	}
	
	return &CountBasedRotationStrategy{
		name:     "count_based",
		maxLines: maxLines,
	}
}

// ShouldRotate `n�?func (s *CountBasedRotationStrategy) ShouldRotate(info *FileInfo) bool {
	return info.LineCount >= s.maxLines
}

// GetName `n`nfunc (s *CountBasedRotationStrategy) GetName() string {
	return s.name
}

// GetNextFilename `n`nfunc (s *CountBasedRotationStrategy) GetNextFilename(currentFile string) string {
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// CronBasedRotationStrategy `nCron`n`ntype CronBasedRotationStrategy struct {
	name         string
	cronExpr     string
	nextRotation time.Time
	mu           sync.RWMutex
}

// NewCronBasedRotationStrategy `nCron`n�?func NewCronBasedRotationStrategy(cronExpr string) *CronBasedRotationStrategy {
	// `n，`ncron`n�?	nextRotation := calculateNextRotation(cronExpr)
	
	return &CronBasedRotationStrategy{
		name:         "cron_based",
		cronExpr:     cronExpr,
		nextRotation: nextRotation,
	}
}

// calculateNextRotation `n（`n）
func calculateNextRotation(cronExpr string) time.Time {
	now := time.Now()
	
	// `ncron`n，`n`nswitch cronExpr {
	case "@daily", "0 0 * * *":
		// `n`nreturn time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case "@hourly", "0 * * * *":
		// `n�?		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, now.Location())
	case "@weekly", "0 0 * * 0":
		// `n�?		daysUntilSunday := (7 - int(now.Weekday())) % 7
		if daysUntilSunday == 0 {
			daysUntilSunday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()+daysUntilSunday, 0, 0, 0, 0, now.Location())
	default:
		// `n`nreturn time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	}
}

// ShouldRotate `n�?func (s *CronBasedRotationStrategy) ShouldRotate(info *FileInfo) bool {
	s.mu.RLock()
	nextRotation := s.nextRotation
	s.mu.RUnlock()
	
	if time.Now().After(nextRotation) {
		s.mu.Lock()
		s.nextRotation = calculateNextRotation(s.cronExpr)
		s.mu.Unlock()
		return true
	}
	
	return false
}

// GetName `n`nfunc (s *CronBasedRotationStrategy) GetName() string {
	return s.name
}

// GetNextFilename `n`nfunc (s *CronBasedRotationStrategy) GetNextFilename(currentFile string) string {
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}

// CompositeRotationStrategy `n`ntype CompositeRotationStrategy struct {
	name       string
	strategies []RotationStrategy
	mode       CompositeRotationMode
}

// CompositeRotationMode `n`ntype CompositeRotationMode int

const (
	RotationAND CompositeRotationMode = iota // `n�?	RotationOR                               // `n�?)

// NewCompositeRotationStrategy `n`nfunc NewCompositeRotationStrategy(mode CompositeRotationMode, strategies ...RotationStrategy) *CompositeRotationStrategy {
	return &CompositeRotationStrategy{
		name:       "composite",
		strategies: strategies,
		mode:       mode,
	}
}

// ShouldRotate `n�?func (s *CompositeRotationStrategy) ShouldRotate(info *FileInfo) bool {
	switch s.mode {
	case RotationAND:
		for _, strategy := range s.strategies {
			if !strategy.ShouldRotate(info) {
				return false
			}
		}
		return len(s.strategies) > 0
	
	case RotationOR:
		for _, strategy := range s.strategies {
			if strategy.ShouldRotate(info) {
				return true
			}
		}
		return false
	
	default:
		return false
	}
}

// GetName `n`nfunc (s *CompositeRotationStrategy) GetName() string {
	return s.name
}

// GetNextFilename `n`nfunc (s *CompositeRotationStrategy) GetNextFilename(currentFile string) string {
	// `n`nif len(s.strategies) > 0 {
		return s.strategies[0].GetNextFilename(currentFile)
	}
	
	// `n
	dir := filepath.Dir(currentFile)
	base := filepath.Base(currentFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, timestamp, ext))
}