// NetCore-Go CLI Development Commands
// Enhanced development workflow commands
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

// DevConfig 开发配置
type DevConfig struct {
	HotReload    bool              `json:"hot_reload"`
	AutoRestart  bool              `json:"auto_restart"`
	WatchPaths   []string          `json:"watch_paths"`
	IgnorePaths  []string          `json:"ignore_paths"`
	BuildCommand string            `json:"build_command"`
	RunCommand   string            `json:"run_command"`
	TestCommand  string            `json:"test_command"`
	LintCommand  string            `json:"lint_command"`
	EnvVars      map[string]string `json:"env_vars"`
	Port         int               `json:"port"`
	ProxyPort    int               `json:"proxy_port"`
}

// dev 命令 - 开发模式
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start development server with hot reload",
	Long: `Start the development server with hot reload, auto-restart, and other development features.

Features:
  - Hot reload on file changes
  - Automatic restart on crash
  - Live reload for web assets
  - Integrated testing
  - Code linting
  - Performance monitoring

Examples:
  netcore-cli dev
  netcore-cli dev --port=3000
  netcore-cli dev --hot-reload=false
  netcore-cli dev --watch="pkg,cmd"`,
	RunE: runDev,
}

func init() {
	devCmd.Flags().Int("port", 8080, "development server port")
	devCmd.Flags().Int("proxy-port", 8081, "proxy server port")
	devCmd.Flags().Bool("hot-reload", true, "enable hot reload")
	devCmd.Flags().Bool("auto-restart", true, "enable auto restart")
	devCmd.Flags().StringSlice("watch", []string{"pkg", "cmd", "internal"}, "directories to watch")
	devCmd.Flags().StringSlice("ignore", []string{"vendor", "node_modules", ".git"}, "directories to ignore")
	devCmd.Flags().String("build-cmd", "go build -o ./bin/app ./cmd", "build command")
	devCmd.Flags().String("run-cmd", "./bin/app", "run command")
	devCmd.Flags().Bool("test", false, "run tests on file changes")
	devCmd.Flags().Bool("lint", false, "run linter on file changes")
	devCmd.Flags().Bool("race", false, "enable race detection")
	devCmd.Flags().StringToString("env", map[string]string{}, "environment variables")
}

func runDev(cmd *cobra.Command, args []string) error {
	// 获取配置
	port, _ := cmd.Flags().GetInt("port")
	proxyPort, _ := cmd.Flags().GetInt("proxy-port")
	hotReload, _ := cmd.Flags().GetBool("hot-reload")
	autoRestart, _ := cmd.Flags().GetBool("auto-restart")
	watchPaths, _ := cmd.Flags().GetStringSlice("watch")
	ignorePaths, _ := cmd.Flags().GetStringSlice("ignore")
	buildCmd, _ := cmd.Flags().GetString("build-cmd")
	runCmd, _ := cmd.Flags().GetString("run-cmd")
	runTests, _ := cmd.Flags().GetBool("test")
	runLint, _ := cmd.Flags().GetBool("lint")
	raceDetection, _ := cmd.Flags().GetBool("race")
	envVars, _ := cmd.Flags().GetStringToString("env")
	
	// 创建开发配置
	devConfig := &DevConfig{
		HotReload:    hotReload,
		AutoRestart:  autoRestart,
		WatchPaths:   watchPaths,
		IgnorePaths:  ignorePaths,
		BuildCommand: buildCmd,
		RunCommand:   runCmd,
		EnvVars:      envVars,
		Port:         port,
		ProxyPort:    proxyPort,
	}
	
	// 添加race检测
	if raceDetection {
		devConfig.BuildCommand = strings.Replace(devConfig.BuildCommand, "go build", "go build -race", 1)
	}
	
	fmt.Println("🚀 Starting NetCore-Go development server...")
	fmt.Printf("📁 Watching directories: %s\n", strings.Join(watchPaths, ", "))
	fmt.Printf("🌐 Server will be available at: http://localhost:%d\n", port)
	
	if hotReload {
		fmt.Printf("🔥 Hot reload enabled on port: %d\n", proxyPort)
	}
	
	// 启动开发服务器
	devServer := NewDevServer(devConfig)
	
	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// 启动服务器
	if err := devServer.Start(); err != nil {
		return fmt.Errorf("failed to start development server: %w", err)
	}
	
	// 运行测试和代码检查
	if runTests {
		go devServer.RunPeriodicTests()
	}
	
	if runLint {
		go devServer.RunPeriodicLint()
	}
	
	// 等待中断信号
	<-sigChan
	fmt.Println("\n🛑 Shutting down development server...")
	
	if err := devServer.Stop(); err != nil {
		fmt.Printf("Error stopping development server: %v\n", err)
	}
	
	fmt.Println("✅ Development server stopped")
	return nil
}

// DevServer 开发服务器
type DevServer struct {
	config    *DevConfig
	process   *os.Process
	watcher   *FileWatcher
	proxyServer *ProxyServer
	running   bool
}

// NewDevServer 创建开发服务器
func NewDevServer(config *DevConfig) *DevServer {
	return &DevServer{
		config: config,
	}
}

// Start 启动开发服务器
func (ds *DevServer) Start() error {
	// 初始构建
	if err := ds.build(); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}
	
	// 启动应用程序
	if err := ds.startApp(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}
	
	// 启动文件监视器
	if ds.config.HotReload {
		ds.watcher = NewFileWatcher(ds.config.WatchPaths, ds.config.IgnorePaths)
		ds.watcher.OnChange = ds.handleFileChange
		if err := ds.watcher.Start(); err != nil {
			return fmt.Errorf("failed to start file watcher: %w", err)
		}
	}
	
	// 启动代理服务器（用于热重载）
	if ds.config.HotReload {
		ds.proxyServer = NewProxyServer(ds.config.ProxyPort, ds.config.Port)
		go ds.proxyServer.Start()
	}
	
	ds.running = true
	return nil
}

// Stop 停止开发服务器
func (ds *DevServer) Stop() error {
	ds.running = false
	
	// 停止应用程序
	if ds.process != nil {
		ds.process.Kill()
		ds.process.Wait()
	}
	
	// 停止文件监视器
	if ds.watcher != nil {
		ds.watcher.Stop()
	}
	
	// 停止代理服务器
	if ds.proxyServer != nil {
		ds.proxyServer.Stop()
	}
	
	return nil
}

// build 构建应用程序
func (ds *DevServer) build() error {
	fmt.Println("🔨 Building application...")
	
	cmdParts := strings.Fields(ds.config.BuildCommand)
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// 设置环境变量
	cmd.Env = os.Environ()
	for key, value := range ds.config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build command failed: %w", err)
	}
	
	fmt.Println("✅ Build completed")
	return nil
}

// startApp 启动应用程序
func (ds *DevServer) startApp() error {
	fmt.Println("🚀 Starting application...")
	
	cmdParts := strings.Fields(ds.config.RunCommand)
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// 设置环境变量
	cmd.Env = os.Environ()
	for key, value := range ds.config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}
	
	ds.process = cmd.Process
	fmt.Printf("✅ Application started (PID: %d)\n", ds.process.Pid)
	
	// 监控进程状态
	if ds.config.AutoRestart {
		go ds.monitorProcess(cmd)
	}
	
	return nil
}

// monitorProcess 监控进程状态
func (ds *DevServer) monitorProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	if ds.running && err != nil {
		fmt.Printf("⚠️  Application crashed: %v\n", err)
		fmt.Println("🔄 Restarting application...")
		
		// 重新构建和启动
		if buildErr := ds.build(); buildErr != nil {
			fmt.Printf("❌ Rebuild failed: %v\n", buildErr)
			return
		}
		
		if startErr := ds.startApp(); startErr != nil {
			fmt.Printf("❌ Restart failed: %v\n", startErr)
		}
	}
}

// handleFileChange 处理文件变更
func (ds *DevServer) handleFileChange(path string) {
	fmt.Printf("📝 File changed: %s\n", path)
	
	// 停止当前进程
	if ds.process != nil {
		ds.process.Kill()
		ds.process.Wait()
	}
	
	// 重新构建
	if err := ds.build(); err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
		return
	}
	
	// 重新启动
	if err := ds.startApp(); err != nil {
		fmt.Printf("❌ Restart failed: %v\n", err)
	}
	
	// 通知代理服务器重新加载
	if ds.proxyServer != nil {
		ds.proxyServer.NotifyReload()
	}
}

// RunPeriodicTests 定期运行测试
func (ds *DevServer) RunPeriodicTests() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if ds.running {
				ds.runTests()
			}
		}
	}
}

// RunPeriodicLint 定期运行代码检查
func (ds *DevServer) RunPeriodicLint() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if ds.running {
				ds.runLint()
			}
		}
	}
}

// runTests 运行测试
func (ds *DevServer) runTests() {
	fmt.Println("🧪 Running tests...")
	cmd := exec.Command("go", "test", "./...")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		fmt.Printf("❌ Tests failed:\n%s\n", output)
	} else {
		fmt.Println("✅ All tests passed")
	}
}

// runLint 运行代码检查
func (ds *DevServer) runLint() {
	fmt.Println("🔍 Running linter...")
	
	// 尝试使用golangci-lint
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		cmd := exec.Command("golangci-lint", "run")
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("⚠️  Linter issues found:\n%s\n", output)
		} else {
			fmt.Println("✅ No linter issues")
		}
	} else {
		// 使用go vet作为后备
		cmd := exec.Command("go", "vet", "./...")
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("⚠️  Vet issues found:\n%s\n", output)
		} else {
			fmt.Println("✅ No vet issues")
		}
	}
}

// FileWatcher 文件监视器（简化实现）
type FileWatcher struct {
	watchPaths  []string
	ignorePaths []string
	OnChange    func(string)
	running     bool
	stopChan    chan struct{}
}

// NewFileWatcher 创建文件监视器
func NewFileWatcher(watchPaths, ignorePaths []string) *FileWatcher {
	return &FileWatcher{
		watchPaths:  watchPaths,
		ignorePaths: ignorePaths,
		stopChan:    make(chan struct{}),
	}
}

// Start 启动文件监视器
func (fw *FileWatcher) Start() error {
	fw.running = true
	
	// 简化的文件监视实现
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		lastModTimes := make(map[string]time.Time)
		
		for {
			select {
			case <-ticker.C:
				for _, watchPath := range fw.watchPaths {
					filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return nil
						}
						
						// 跳过目录和忽略的路径
						if info.IsDir() || fw.shouldIgnore(path) {
							return nil
						}
						
						// 只监视Go文件
						if !strings.HasSuffix(path, ".go") {
							return nil
						}
						
						lastMod, exists := lastModTimes[path]
						if !exists || info.ModTime().After(lastMod) {
							lastModTimes[path] = info.ModTime()
							if exists && fw.OnChange != nil {
								fw.OnChange(path)
							}
						}
						
						return nil
					})
				}
			case <-fw.stopChan:
				return
			}
		}
	}()
	
	return nil
}

// Stop 停止文件监视器
func (fw *FileWatcher) Stop() {
	fw.running = false
	close(fw.stopChan)
}

// shouldIgnore 检查是否应该忽略路径
func (fw *FileWatcher) shouldIgnore(path string) bool {
	for _, ignorePath := range fw.ignorePaths {
		if strings.Contains(path, ignorePath) {
			return true
		}
	}
	return false
}

// ProxyServer 代理服务器（用于热重载）
type ProxyServer struct {
	proxyPort     int
	targetPort    int
	running       bool
	server        *http.Server
	wsConnections []*websocket.Conn
	mu            sync.Mutex
}

// NewProxyServer 创建代理服务器
func NewProxyServer(proxyPort, targetPort int) *ProxyServer {
	return &ProxyServer{
		proxyPort:  proxyPort,
		targetPort: targetPort,
	}
}

// Start 启动代理服务器
func (ps *ProxyServer) Start() error {
	ps.running = true
	
	// 创建HTTP服务器
	mux := http.NewServeMux()
	
	// WebSocket升级器
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有来源
		},
	}
	
	// WebSocket处理器
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		
		ps.mu.Lock()
		ps.wsConnections = append(ps.wsConnections, conn)
		ps.mu.Unlock()
		
		// 保持连接活跃
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	})
	
	// 代理处理器
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 创建到目标服务器的请求
		targetURL := fmt.Sprintf("http://localhost:%d%s", ps.targetPort, r.URL.Path)
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
		
		req, err := http.NewRequest(r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		// 复制请求头
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
		
		// 发送请求
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		
		// 复制响应头
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		
		// 注入热重载脚本到HTML响应
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/html") {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				htmlContent := string(body)
				reloadScript := fmt.Sprintf(`
<script>
(function() {
	var ws = new WebSocket('ws://localhost:%d/ws');
	ws.onmessage = function(event) {
		if (event.data === 'reload') {
			location.reload();
		}
	};
	ws.onclose = function() {
		setTimeout(function() {
			location.reload();
		}, 1000);
	};
})();
</script>
</body>`, ps.proxyPort)
				htmlContent = strings.Replace(htmlContent, "</body>", reloadScript, 1)
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(htmlContent)))
				w.WriteHeader(resp.StatusCode)
				w.Write([]byte(htmlContent))
				return
			}
		}
		
		// 复制响应体
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})
	
	// 启动HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", ps.proxyPort),
		Handler: mux,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Proxy server error: %v", err)
		}
	}()
	
	ps.server = server
	fmt.Printf("🔥 Hot reload proxy started on port %d\n", ps.proxyPort)
	return nil
}

// Stop 停止代理服务器
func (ps *ProxyServer) Stop() {
	ps.running = false
	
	// 关闭所有WebSocket连接
	ps.mu.Lock()
	for _, conn := range ps.wsConnections {
		conn.Close()
	}
	ps.wsConnections = nil
	ps.mu.Unlock()
	
	// 关闭HTTP服务器
	if ps.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ps.server.Shutdown(ctx)
	}
}

// NotifyReload 通知重新加载
func (ps *ProxyServer) NotifyReload() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	// 向所有WebSocket连接发送重载消息
	var activeConnections []*websocket.Conn
	for _, conn := range ps.wsConnections {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("reload")); err != nil {
			conn.Close()
		} else {
			activeConnections = append(activeConnections, conn)
		}
	}
	
	// 更新活跃连接列表
	ps.wsConnections = activeConnections
	
	fmt.Printf("🔄 Notified %d clients to reload\n", len(activeConnections))
}

// test 命令 - 运行测试
var testCmd = &cobra.Command{
	Use:   "test [packages...]",
	Short: "Run tests with enhanced features",
	Long: `Run tests with coverage, benchmarks, and other enhanced features.

Examples:
  netcore-cli test
  netcore-cli test --coverage
  netcore-cli test --benchmark
  netcore-cli test --race
  netcore-cli test ./pkg/...`,
	RunE: runTest,
}

func init() {
	testCmd.Flags().Bool("coverage", false, "generate coverage report")
	testCmd.Flags().Bool("benchmark", false, "run benchmarks")
	testCmd.Flags().Bool("race", false, "enable race detection")
	testCmd.Flags().Bool("verbose", false, "verbose output")
	testCmd.Flags().String("output", "", "output format (json, xml)")
	testCmd.Flags().Duration("timeout", 10*time.Minute, "test timeout")
	testCmd.Flags().Int("parallel", 0, "number of parallel tests")
}

func runTest(cmd *cobra.Command, args []string) error {
	coverage, _ := cmd.Flags().GetBool("coverage")
	benchmark, _ := cmd.Flags().GetBool("benchmark")
	race, _ := cmd.Flags().GetBool("race")
	verbose, _ := cmd.Flags().GetBool("verbose")
	_, _ = cmd.Flags().GetString("output") // 暂未使用
	timeout, _ := cmd.Flags().GetDuration("timeout")
	parallel, _ := cmd.Flags().GetInt("parallel")
	
	// 构建测试命令
	testArgs := []string{"test"}
	
	if coverage {
		testArgs = append(testArgs, "-cover", "-coverprofile=coverage.out")
	}
	
	if benchmark {
		testArgs = append(testArgs, "-bench=.")
	}
	
	if race {
		testArgs = append(testArgs, "-race")
	}
	
	if verbose {
		testArgs = append(testArgs, "-v")
	}
	
	if timeout > 0 {
		testArgs = append(testArgs, fmt.Sprintf("-timeout=%s", timeout))
	}
	
	if parallel > 0 {
		testArgs = append(testArgs, fmt.Sprintf("-parallel=%d", parallel))
	}
	
	// 添加包路径
	if len(args) > 0 {
		testArgs = append(testArgs, args...)
	} else {
		testArgs = append(testArgs, "./...")
	}
	
	fmt.Println("🧪 Running tests...")
	
	// 执行测试
	testCmd := exec.Command("go", testArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}
	
	// 生成覆盖率报告
	if coverage {
		fmt.Println("📊 Generating coverage report...")
		
		// HTML报告
	htmlCmd := exec.Command("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html")
		if err := htmlCmd.Run(); err != nil {
			fmt.Printf("⚠️  Failed to generate HTML coverage report: %v\n", err)
		} else {
			fmt.Println("✅ Coverage report generated: coverage.html")
		}
		
		// 显示覆盖率统计
		funcCmd := exec.Command("go", "tool", "cover", "-func=coverage.out")
		funcCmd.Stdout = os.Stdout
		funcCmd.Run()
	}
	
	fmt.Println("✅ All tests completed")
	return nil
}

// lint 命令 - 代码检查
var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run code linting and static analysis",
	Long: `Run comprehensive code linting and static analysis.

Tools used:
  - go vet
  - golangci-lint (if available)
  - go fmt
  - goimports (if available)

Examples:
  netcore-cli lint
  netcore-cli lint --fix
  netcore-cli lint --config=.golangci.yml`,
	RunE: runLint,
}

func init() {
	lintCmd.Flags().Bool("fix", false, "automatically fix issues")
	lintCmd.Flags().String("config", "", "linter config file")
	lintCmd.Flags().Bool("verbose", false, "verbose output")
}

func runLint(cmd *cobra.Command, args []string) error {
	fix, _ := cmd.Flags().GetBool("fix")
	config, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	
	fmt.Println("🔍 Running code linting...")
	
	// 运行go fmt
	fmt.Println("📝 Checking code formatting...")
	fmtArgs := []string{"fmt"}
	if !fix {
		fmtArgs = append(fmtArgs, "-n")
	}
	fmtArgs = append(fmtArgs, "./...")
	
	fmtCmd := exec.Command("go", fmtArgs...)
	if verbose {
		fmtCmd.Stdout = os.Stdout
	}
	fmtCmd.Stderr = os.Stderr
	fmtCmd.Run()
	
	// 运行go vet
	fmt.Println("🔍 Running go vet...")
	vetCmd := exec.Command("go", "vet", "./...")
	if verbose {
		vetCmd.Stdout = os.Stdout
	}
	vetCmd.Stderr = os.Stderr
	vetCmd.Run()
	
	// 运行golangci-lint（如果可用）
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		fmt.Println("🚀 Running golangci-lint...")
		
		lintArgs := []string{"run"}
		if config != "" {
			lintArgs = append(lintArgs, "-c", config)
		}
		if fix {
			lintArgs = append(lintArgs, "--fix")
		}
		if verbose {
			lintArgs = append(lintArgs, "-v")
		}
		
		golangciCmd := exec.Command("golangci-lint", lintArgs...)
		golangciCmd.Stdout = os.Stdout
		golangciCmd.Stderr = os.Stderr
		golangciCmd.Run()
	} else {
		fmt.Println("⚠️  golangci-lint not found, install it for better linting")
	}
	
	// 运行goimports（如果可用）
	if _, err := exec.LookPath("goimports"); err == nil && fix {
		fmt.Println("📦 Running goimports...")
		importsCmd := exec.Command("goimports", "-w", ".")
		if verbose {
			importsCmd.Stdout = os.Stdout
		}
		importsCmd.Stderr = os.Stderr
		importsCmd.Run()
	}
	
	fmt.Println("✅ Linting completed")
	return nil
}