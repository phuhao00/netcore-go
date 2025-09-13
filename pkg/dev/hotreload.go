// Package dev 热重载开发服务器
// Author: NetCore-Go Team
// Created: 2024

package dev

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// HotReloadServer 热重载服务器
type HotReloadServer struct {
	config    *HotReloadConfig
	watcher   *fsnotify.Watcher
	process   *os.Process
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	lastBuild time.Time
	buildMu   sync.Mutex
	logger    *log.Logger
	stats     *ReloadStats
}

// HotReloadConfig 热重载配置
type HotReloadConfig struct {
	// 基础配置
	ProjectDir    string        `json:"project_dir"`
	BuildCommand  string        `json:"build_command"`
	RunCommand    string        `json:"run_command"`
	BinaryPath    string        `json:"binary_path"`
	Port          int           `json:"port"`
	Host          string        `json:"host"`
	BuildTimeout  time.Duration `json:"build_timeout"`
	StartupDelay  time.Duration `json:"startup_delay"`
	ShutdownDelay time.Duration `json:"shutdown_delay"`

	// 文件监控配置
	WatchDirs     []string `json:"watch_dirs"`
	WatchExts     []string `json:"watch_exts"`
	IgnorePatterns []string `json:"ignore_patterns"`
	IgnoreDirs    []string `json:"ignore_dirs"`
	IgnoreFiles   []string `json:"ignore_files"`

	// 构建配置
	BuildArgs     []string          `json:"build_args"`
	BuildEnv      map[string]string `json:"build_env"`
	RunArgs       []string          `json:"run_args"`
	RunEnv        map[string]string `json:"run_env"`
	PreBuildHook  string            `json:"pre_build_hook"`
	PostBuildHook string            `json:"post_build_hook"`
	PreRunHook    string            `json:"pre_run_hook"`
	PostRunHook   string            `json:"post_run_hook"`

	// 高级配置
	DebounceDelay time.Duration `json:"debounce_delay"`
	Verbose       bool          `json:"verbose"`
	Quiet         bool          `json:"quiet"`
	ColorOutput   bool          `json:"color_output"`
	ShowBuildTime bool          `json:"show_build_time"`
	AutoRestart   bool          `json:"auto_restart"`
	KillSignal    string        `json:"kill_signal"`
	KillTimeout   time.Duration `json:"kill_timeout"`
}

// ReloadStats 重载统计
type ReloadStats struct {
	StartTime     time.Time     `json:"start_time"`
	TotalReloads  int64         `json:"total_reloads"`
	SuccessReloads int64        `json:"success_reloads"`
	FailedReloads int64         `json:"failed_reloads"`
	TotalBuildTime time.Duration `json:"total_build_time"`
	AvgBuildTime  time.Duration `json:"avg_build_time"`
	LastReload    time.Time     `json:"last_reload"`
	LastError     string        `json:"last_error,omitempty"`
	FileChanges   int64         `json:"file_changes"`
	mu            sync.RWMutex
}

// FileChangeEvent 文件变更事件
type FileChangeEvent struct {
	Path      string
	Op        fsnotify.Op
	Timestamp time.Time
}

// BuildResult 构建结果
type BuildResult struct {
	Success   bool
	Duration  time.Duration
	Output    string
	Error     error
	Timestamp time.Time
}

// DefaultHotReloadConfig 返回默认热重载配置
func DefaultHotReloadConfig() *HotReloadConfig {
	return &HotReloadConfig{
		ProjectDir:    ".",
		BuildCommand:  "go build -o bin/app cmd/main.go",
		RunCommand:    "./bin/app",
		BinaryPath:    "bin/app",
		Port:          8080,
		Host:          "localhost",
		BuildTimeout:  30 * time.Second,
		StartupDelay:  1 * time.Second,
		ShutdownDelay: 5 * time.Second,
		WatchDirs:     []string{"."},
		WatchExts:     []string{".go", ".yaml", ".yml", ".json", ".toml"},
		IgnorePatterns: []string{
			`\.git/.*`,
			`\.vscode/.*`,
			`\.idea/.*`,
			`node_modules/.*`,
			`vendor/.*`,
			`bin/.*`,
			`tmp/.*`,
			`logs/.*`,
			`\.(log|tmp|swp|swo)$`,
		},
		IgnoreDirs:    []string{".git", ".vscode", ".idea", "node_modules", "vendor", "bin", "tmp", "logs"},
		IgnoreFiles:   []string{".DS_Store", "Thumbs.db"},
		BuildArgs:     []string{},
		BuildEnv:      make(map[string]string),
		RunArgs:       []string{},
		RunEnv:        make(map[string]string),
		DebounceDelay: 500 * time.Millisecond,
		Verbose:       false,
		Quiet:         false,
		ColorOutput:   true,
		ShowBuildTime: true,
		AutoRestart:   true,
		KillSignal:    "SIGTERM",
		KillTimeout:   10 * time.Second,
	}
}

// NewHotReloadServer 创建热重载服务器
func NewHotReloadServer(config *HotReloadConfig) (*HotReloadServer, error) {
	if config == nil {
		config = DefaultHotReloadConfig()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := log.New(os.Stdout, "[HotReload] ", log.LstdFlags)
	if config.Quiet {
		logger.SetOutput(io.Discard)
	}

	return &HotReloadServer{
		config:  config,
		watcher: watcher,
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		stats: &ReloadStats{
			StartTime: time.Now(),
		},
	}, nil
}

// Start 启动热重载服务器
func (hrs *HotReloadServer) Start() error {
	hrs.mu.Lock()
	if hrs.running {
		hrs.mu.Unlock()
		return fmt.Errorf("hot reload server is already running")
	}
	hrs.running = true
	hrs.mu.Unlock()

	hrs.logger.Println("🚀 Starting hot reload server...")

	// 添加监控目录
	if err := hrs.setupWatcher(); err != nil {
		return fmt.Errorf("failed to setup file watcher: %w", err)
	}

	// 初始构建
	if err := hrs.initialBuild(); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	// 启动文件监控
	go hrs.watchFiles()

	// 启动应用
	if err := hrs.startApp(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	hrs.logger.Printf("✅ Hot reload server started on %s:%d", hrs.config.Host, hrs.config.Port)
	hrs.logger.Println("📁 Watching for file changes...")

	return nil
}

// Stop 停止热重载服务器
func (hrs *HotReloadServer) Stop() error {
	hrs.mu.Lock()
	defer hrs.mu.Unlock()

	if !hrs.running {
		return nil
	}

	hrs.logger.Println("🛑 Stopping hot reload server...")

	// 停止应用
	if err := hrs.stopApp(); err != nil {
		hrs.logger.Printf("Error stopping application: %v", err)
	}

	// 关闭文件监控
	if err := hrs.watcher.Close(); err != nil {
		hrs.logger.Printf("Error closing file watcher: %v", err)
	}

	// 取消上下文
	hrs.cancel()

	hrs.running = false
	hrs.logger.Println("✅ Hot reload server stopped")

	// 打印统计信息
	hrs.printStats()

	return nil
}

// setupWatcher 设置文件监控
func (hrs *HotReloadServer) setupWatcher() error {
	for _, dir := range hrs.config.WatchDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
		}

		err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 跳过忽略的目录和文件
			if hrs.shouldIgnore(path, info) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// 只监控目录
			if info.IsDir() {
				if err := hrs.watcher.Add(path); err != nil {
					hrs.logger.Printf("Warning: failed to watch directory %s: %v", path, err)
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", dir, err)
		}
	}

	return nil
}

// shouldIgnore 检查是否应该忽略文件或目录
func (hrs *HotReloadServer) shouldIgnore(path string, info os.FileInfo) bool {
	name := info.Name()
	relPath, _ := filepath.Rel(hrs.config.ProjectDir, path)

	// 检查忽略的目录
	if info.IsDir() {
		for _, ignoreDir := range hrs.config.IgnoreDirs {
			if name == ignoreDir {
				return true
			}
		}
	}

	// 检查忽略的文件
	for _, ignoreFile := range hrs.config.IgnoreFiles {
		if name == ignoreFile {
			return true
		}
	}

	// 检查忽略的模式
	for _, pattern := range hrs.config.IgnorePatterns {
		matched, err := regexp.MatchString(pattern, relPath)
		if err == nil && matched {
			return true
		}
	}

	// 检查文件扩展名
	if !info.IsDir() {
		ext := filepath.Ext(name)
		if len(hrs.config.WatchExts) > 0 {
			found := false
			for _, watchExt := range hrs.config.WatchExts {
				if ext == watchExt {
					found = true
					break
				}
			}
			if !found {
				return true
			}
		}
	}

	return false
}

// initialBuild 初始构建
func (hrs *HotReloadServer) initialBuild() error {
	hrs.logger.Println("🔨 Initial build...")
	result := hrs.build()
	if !result.Success {
		return fmt.Errorf("initial build failed: %w", result.Error)
	}
	hrs.logger.Printf("✅ Initial build completed in %v", result.Duration)
	return nil
}

// watchFiles 监控文件变化
func (hrs *HotReloadServer) watchFiles() {
	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	var pendingEvents []FileChangeEvent

	for {
		select {
		case event, ok := <-hrs.watcher.Events:
			if !ok {
				return
			}

			// 检查是否应该忽略此事件
			if hrs.shouldIgnoreEvent(event) {
				continue
			}

			// 添加到待处理事件
			changeEvent := FileChangeEvent{
				Path:      event.Name,
				Op:        event.Op,
				Timestamp: time.Now(),
			}
			pendingEvents = append(pendingEvents, changeEvent)

			// 重置防抖计时器
			debounceTimer.Reset(hrs.config.DebounceDelay)

			if hrs.config.Verbose {
				hrs.logger.Printf("📝 File changed: %s (%s)", event.Name, event.Op)
			}

		case err, ok := <-hrs.watcher.Errors:
			if !ok {
				return
			}
			hrs.logger.Printf("❌ Watcher error: %v", err)

		case <-debounceTimer.C:
			if len(pendingEvents) > 0 {
				hrs.handleFileChanges(pendingEvents)
				pendingEvents = nil
			}

		case <-hrs.ctx.Done():
			return
		}
	}
}

// shouldIgnoreEvent 检查是否应该忽略事件
func (hrs *HotReloadServer) shouldIgnoreEvent(event fsnotify.Event) bool {
	// 忽略临时文件和备份文件
	name := filepath.Base(event.Name)
	if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".swp") {
		return true
	}
	if strings.HasSuffix(name, "~") {
		return true
	}
	if strings.HasSuffix(name, ".tmp") {
		return true
	}

	// 检查文件信息
	info, err := os.Stat(event.Name)
	if err != nil {
		// 文件可能已被删除，不忽略删除事件
		if event.Op&fsnotify.Remove != 0 {
			return false
		}
		return true
	}

	return hrs.shouldIgnore(event.Name, info)
}

// handleFileChanges 处理文件变化
func (hrs *HotReloadServer) handleFileChanges(events []FileChangeEvent) {
	hrs.stats.mu.Lock()
	hrs.stats.FileChanges += int64(len(events))
	hrs.stats.mu.Unlock()

	if hrs.config.Verbose {
		hrs.logger.Printf("🔄 Processing %d file changes", len(events))
		for _, event := range events {
			hrs.logger.Printf("  - %s (%s)", event.Path, event.Op)
		}
	}

	// 重新构建和重启
	if err := hrs.rebuild(); err != nil {
		hrs.logger.Printf("❌ Rebuild failed: %v", err)
		hrs.stats.mu.Lock()
		hrs.stats.FailedReloads++
		hrs.stats.LastError = err.Error()
		hrs.stats.mu.Unlock()
	} else {
		hrs.stats.mu.Lock()
		hrs.stats.SuccessReloads++
		hrs.stats.LastReload = time.Now()
		hrs.stats.mu.Unlock()
	}

	hrs.stats.mu.Lock()
	hrs.stats.TotalReloads++
	hrs.stats.mu.Unlock()
}

// rebuild 重新构建和重启应用
func (hrs *HotReloadServer) rebuild() error {
	hrs.buildMu.Lock()
	defer hrs.buildMu.Unlock()

	// 防止频繁构建
	if time.Since(hrs.lastBuild) < 100*time.Millisecond {
		return nil
	}

	hrs.logger.Println("🔨 Rebuilding...")

	// 停止当前应用
	if err := hrs.stopApp(); err != nil {
		hrs.logger.Printf("Warning: failed to stop app: %v", err)
	}

	// 构建
	result := hrs.build()
	hrs.lastBuild = time.Now()

	hrs.stats.mu.Lock()
	hrs.stats.TotalBuildTime += result.Duration
	if hrs.stats.TotalReloads > 0 {
		hrs.stats.AvgBuildTime = hrs.stats.TotalBuildTime / time.Duration(hrs.stats.TotalReloads)
	}
	hrs.stats.mu.Unlock()

	if !result.Success {
		hrs.logger.Printf("❌ Build failed in %v: %v", result.Duration, result.Error)
		if result.Output != "" {
			hrs.logger.Printf("Build output:\n%s", result.Output)
		}
		return result.Error
	}

	if hrs.config.ShowBuildTime {
		hrs.logger.Printf("✅ Build completed in %v", result.Duration)
	}

	// 启动应用
	if err := hrs.startApp(); err != nil {
		hrs.logger.Printf("❌ Failed to start app: %v", err)
		return err
	}

	hrs.logger.Println("🚀 Application restarted")
	return nil
}

// build 构建应用
func (hrs *HotReloadServer) build() *BuildResult {
	start := time.Now()
	result := &BuildResult{
		Timestamp: start,
	}

	// 执行预构建钩子
	if hrs.config.PreBuildHook != "" {
		if err := hrs.runHook(hrs.config.PreBuildHook); err != nil {
			hrs.logger.Printf("Warning: pre-build hook failed: %v", err)
		}
	}

	// 解析构建命令
	parts := strings.Fields(hrs.config.BuildCommand)
	if len(parts) == 0 {
		result.Error = fmt.Errorf("empty build command")
		result.Duration = time.Since(start)
		return result
	}

	// 创建构建命令
	ctx, cancel := context.WithTimeout(hrs.ctx, hrs.config.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = hrs.config.ProjectDir

	// 设置环境变量
	cmd.Env = os.Environ()
	for key, value := range hrs.config.BuildEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// 添加构建参数
	if len(hrs.config.BuildArgs) > 0 {
		cmd.Args = append(cmd.Args, hrs.config.BuildArgs...)
	}

	// 执行构建
	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)

	if err != nil {
		result.Error = err
		result.Success = false
	} else {
		result.Success = true
	}

	// 执行后构建钩子
	if hrs.config.PostBuildHook != "" {
		if err := hrs.runHook(hrs.config.PostBuildHook); err != nil {
			hrs.logger.Printf("Warning: post-build hook failed: %v", err)
		}
	}

	return result
}

// startApp 启动应用
func (hrs *HotReloadServer) startApp() error {
	// 执行预运行钩子
	if hrs.config.PreRunHook != "" {
		if err := hrs.runHook(hrs.config.PreRunHook); err != nil {
			hrs.logger.Printf("Warning: pre-run hook failed: %v", err)
		}
	}

	// 解析运行命令
	parts := strings.Fields(hrs.config.RunCommand)
	if len(parts) == 0 {
		return fmt.Errorf("empty run command")
	}

	// 创建运行命令
	cmd := exec.CommandContext(hrs.ctx, parts[0], parts[1:]...)
	cmd.Dir = hrs.config.ProjectDir

	// 设置环境变量
	cmd.Env = os.Environ()
	for key, value := range hrs.config.RunEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// 添加运行参数
	if len(hrs.config.RunArgs) > 0 {
		cmd.Args = append(cmd.Args, hrs.config.RunArgs...)
	}

	// 设置输出
	if hrs.config.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 创建日志管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// 启动日志处理
		go hrs.handleAppOutput(stdout, "[APP] ")
		go hrs.handleAppOutput(stderr, "[APP ERROR] ")
	}

	// 启动应用
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	hrs.mu.Lock()
	hrs.process = cmd.Process
	hrs.mu.Unlock()

	// 等待启动延迟
	if hrs.config.StartupDelay > 0 {
		time.Sleep(hrs.config.StartupDelay)
	}

	// 执行后运行钩子
	if hrs.config.PostRunHook != "" {
		if err := hrs.runHook(hrs.config.PostRunHook); err != nil {
			hrs.logger.Printf("Warning: post-run hook failed: %v", err)
		}
	}

	return nil
}

// stopApp 停止应用
func (hrs *HotReloadServer) stopApp() error {
	hrs.mu.Lock()
	process := hrs.process
	hrs.process = nil
	hrs.mu.Unlock()

	if process == nil {
		return nil
	}

	// 发送终止信号
	var sig os.Signal = syscall.SIGTERM
	switch hrs.config.KillSignal {
	case "SIGINT":
		sig = syscall.SIGINT
	case "SIGKILL":
		sig = syscall.SIGKILL
	}

	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal to process: %w", err)
	}

	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(hrs.config.KillTimeout):
		// 强制杀死进程
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		<-done // 等待进程真正退出
		return nil
	}
}

// handleAppOutput 处理应用输出
func (hrs *HotReloadServer) handleAppOutput(reader io.Reader, prefix string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if !hrs.config.Quiet {
			hrs.logger.Printf("%s%s", prefix, line)
		}
	}
}

// runHook 运行钩子命令
func (hrs *HotReloadServer) runHook(hook string) error {
	parts := strings.Fields(hook)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.CommandContext(hrs.ctx, parts[0], parts[1:]...)
	cmd.Dir = hrs.config.ProjectDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook command failed: %w, output: %s", err, string(output))
	}

	return nil
}

// GetStats 获取统计信息
func (hrs *HotReloadServer) GetStats() *ReloadStats {
	hrs.stats.mu.RLock()
	defer hrs.stats.mu.RUnlock()

	stats := *hrs.stats
	return &stats
}

// printStats 打印统计信息
func (hrs *HotReloadServer) printStats() {
	stats := hrs.GetStats()
	uptime := time.Since(stats.StartTime)

	hrs.logger.Println("📊 Hot Reload Statistics:")
	hrs.logger.Printf("  Uptime: %v", uptime)
	hrs.logger.Printf("  Total Reloads: %d", stats.TotalReloads)
	hrs.logger.Printf("  Successful Reloads: %d", stats.SuccessReloads)
	hrs.logger.Printf("  Failed Reloads: %d", stats.FailedReloads)
	hrs.logger.Printf("  File Changes: %d", stats.FileChanges)
	hrs.logger.Printf("  Total Build Time: %v", stats.TotalBuildTime)
	if stats.TotalReloads > 0 {
		hrs.logger.Printf("  Average Build Time: %v", stats.AvgBuildTime)
	}
	if stats.LastError != "" {
		hrs.logger.Printf("  Last Error: %s", stats.LastError)
	}
}

// IsRunning 检查是否运行中
func (hrs *HotReloadServer) IsRunning() bool {
	hrs.mu.RLock()
	defer hrs.mu.RUnlock()
	return hrs.running
}