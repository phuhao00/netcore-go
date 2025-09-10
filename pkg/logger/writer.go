package logger

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ConsoleWriter 控制台写入器
type ConsoleWriter struct {
	Output io.Writer
	mu     sync.Mutex
}

// Write 写入数据
func (w *ConsoleWriter) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.Output == nil {
		w.Output = os.Stdout
	}
	
	_, err := w.Output.Write(data)
	return err
}

// Close 关闭写入器
func (w *ConsoleWriter) Close() error {
	return nil
}

// FileWriter 文件写入器
type FileWriter struct {
	mu           sync.Mutex
	filename     string
	file         *os.File
	maxSize      int64  // 最大文件大小（字节）
	maxAge       int    // 最大保存天数
	maxBackups   int    // 最大备份文件数
	compress     bool   // 是否压缩备份文件
	currentSize  int64  // 当前文件大小
	bufferSize   int    // 缓冲区大小
	buffer       *bufio.Writer
	flushInterval time.Duration // 刷新间隔
	lastFlush    time.Time     // 上次刷新时间
}

// FileWriterConfig 文件写入器配置
type FileWriterConfig struct {
	Filename      string        // 文件名
	MaxSize       int64         // 最大文件大小（字节）
	MaxAge        int           // 最大保存天数
	MaxBackups    int           // 最大备份文件数
	Compress      bool          // 是否压缩备份文件
	BufferSize    int           // 缓冲区大小
	FlushInterval time.Duration // 刷新间隔
}

// NewFileWriter 创建文件写入器
func NewFileWriter(config *FileWriterConfig) (*FileWriter, error) {
	if config == nil {
		config = &FileWriterConfig{}
	}
	
	if config.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	
	// 设置默认值
	if config.MaxSize <= 0 {
		config.MaxSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxAge <= 0 {
		config.MaxAge = 30 // 30天
	}
	if config.MaxBackups <= 0 {
		config.MaxBackups = 10
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 4096 // 4KB
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}
	
	w := &FileWriter{
		filename:      config.Filename,
		maxSize:       config.MaxSize,
		maxAge:        config.MaxAge,
		maxBackups:    config.MaxBackups,
		compress:      config.Compress,
		bufferSize:    config.BufferSize,
		flushInterval: config.FlushInterval,
		lastFlush:     time.Now(),
	}
	
	if err := w.openFile(); err != nil {
		return nil, err
	}
	
	// 启动定时刷新
	go w.flushLoop()
	
	return w, nil
}

// Write 写入数据
func (w *FileWriter) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// 检查是否需要轮转
	if w.currentSize+int64(len(data)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}
	
	// 写入数据
	n, err := w.buffer.Write(data)
	if err != nil {
		return err
	}
	
	w.currentSize += int64(n)
	
	// 检查是否需要立即刷新
	if time.Since(w.lastFlush) > w.flushInterval {
		if err := w.buffer.Flush(); err != nil {
			return err
		}
		w.lastFlush = time.Now()
	}
	
	return nil
}

// Close 关闭写入器
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.buffer != nil {
		w.buffer.Flush()
	}
	
	if w.file != nil {
		return w.file.Close()
	}
	
	return nil
}

// openFile 打开文件
func (w *FileWriter) openFile() error {
	// 确保目录存在
	dir := filepath.Dir(w.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// 打开文件
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	// 获取文件大小
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	w.file = file
	w.currentSize = info.Size()
	w.buffer = bufio.NewWriterSize(file, w.bufferSize)
	
	return nil
}

// rotate 轮转日志文件
func (w *FileWriter) rotate() error {
	// 刷新缓冲区
	if w.buffer != nil {
		w.buffer.Flush()
	}
	
	// 关闭当前文件
	if w.file != nil {
		w.file.Close()
	}
	
	// 重命名当前文件
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.%s", w.filename, timestamp)
	
	if err := os.Rename(w.filename, backupName); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}
	
	// 压缩备份文件
	if w.compress {
		go w.compressFile(backupName)
	}
	
	// 清理旧文件
	go w.cleanupOldFiles()
	
	// 打开新文件
	return w.openFile()
}

// compressFile 压缩文件
func (w *FileWriter) compressFile(filename string) {
	compressedName := filename + ".gz"
	
	input, err := os.Open(filename)
	if err != nil {
		return
	}
	defer input.Close()
	
	output, err := os.Create(compressedName)
	if err != nil {
		return
	}
	defer output.Close()
	
	gzWriter := gzip.NewWriter(output)
	defer gzWriter.Close()
	
	if _, err := io.Copy(gzWriter, input); err != nil {
		return
	}
	
	// 删除原文件
	os.Remove(filename)
}

// cleanupOldFiles 清理旧文件
func (w *FileWriter) cleanupOldFiles() {
	dir := filepath.Dir(w.filename)
	base := filepath.Base(w.filename)
	
	files, err := filepath.Glob(filepath.Join(dir, base+".*"))
	if err != nil {
		return
	}
	
	// 按修改时间排序并删除超出限制的文件
	type fileInfo struct {
		name    string
		modTime time.Time
	}
	
	var fileInfos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		// 检查文件年龄
		if w.maxAge > 0 && time.Since(info.ModTime()) > time.Duration(w.maxAge)*24*time.Hour {
			os.Remove(file)
			continue
		}
		
		fileInfos = append(fileInfos, fileInfo{
			name:    file,
			modTime: info.ModTime(),
		})
	}
	
	// 删除超出数量限制的文件
	if len(fileInfos) > w.maxBackups {
		// 按时间排序（旧的在前）
		for i := 0; i < len(fileInfos)-1; i++ {
			for j := i + 1; j < len(fileInfos); j++ {
				if fileInfos[i].modTime.After(fileInfos[j].modTime) {
					fileInfos[i], fileInfos[j] = fileInfos[j], fileInfos[i]
				}
			}
		}
		
		// 删除最旧的文件
		for i := 0; i < len(fileInfos)-w.maxBackups; i++ {
			os.Remove(fileInfos[i].name)
		}
	}
}

// flushLoop 定时刷新循环
func (w *FileWriter) flushLoop() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		w.mu.Lock()
		if w.buffer != nil {
			w.buffer.Flush()
			w.lastFlush = time.Now()
		}
		w.mu.Unlock()
	}
}

// NetworkWriter 网络写入器
type NetworkWriter struct {
	mu       sync.Mutex
	network  string
	address  string
	conn     net.Conn
	timeout  time.Duration
	retryMax int
	buffer   [][]byte
	bufferSize int
}

// NetworkWriterConfig 网络写入器配置
type NetworkWriterConfig struct {
	Network    string        // 网络类型（tcp, udp等）
	Address    string        // 地址
	Timeout    time.Duration // 超时时间
	RetryMax   int           // 最大重试次数
	BufferSize int           // 缓冲区大小
}

// NewNetworkWriter 创建网络写入器
func NewNetworkWriter(config *NetworkWriterConfig) (*NetworkWriter, error) {
	if config == nil {
		config = &NetworkWriterConfig{}
	}
	
	if config.Network == "" {
		config.Network = "tcp"
	}
	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.RetryMax <= 0 {
		config.RetryMax = 3
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	
	w := &NetworkWriter{
		network:    config.Network,
		address:    config.Address,
		timeout:    config.Timeout,
		retryMax:   config.RetryMax,
		bufferSize: config.BufferSize,
		buffer:     make([][]byte, 0, config.BufferSize),
	}
	
	if err := w.connect(); err != nil {
		return nil, err
	}
	
	return w, nil
}

// Write 写入数据
func (w *NetworkWriter) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// 如果连接断开，尝试重连
	if w.conn == nil {
		if err := w.connect(); err != nil {
			// 连接失败，缓存数据
			w.bufferData(data)
			return err
		}
	}
	
	// 设置写入超时
	if w.timeout > 0 {
		w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	}
	
	// 写入数据
	_, err := w.conn.Write(data)
	if err != nil {
		// 写入失败，关闭连接并缓存数据
		w.conn.Close()
		w.conn = nil
		w.bufferData(data)
		return err
	}
	
	// 发送缓存的数据
	w.flushBuffer()
	
	return nil
}

// Close 关闭写入器
func (w *NetworkWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.conn != nil {
		return w.conn.Close()
	}
	
	return nil
}

// connect 连接到远程服务器
func (w *NetworkWriter) connect() error {
	conn, err := net.DialTimeout(w.network, w.address, w.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s://%s: %w", w.network, w.address, err)
	}
	
	w.conn = conn
	return nil
}

// bufferData 缓存数据
func (w *NetworkWriter) bufferData(data []byte) {
	if len(w.buffer) >= w.bufferSize {
		// 缓冲区满，删除最旧的数据
		w.buffer = w.buffer[1:]
	}
	
	// 复制数据
	buf := make([]byte, len(data))
	copy(buf, data)
	w.buffer = append(w.buffer, buf)
}

// flushBuffer 刷新缓冲区
func (w *NetworkWriter) flushBuffer() {
	for len(w.buffer) > 0 {
		data := w.buffer[0]
		w.buffer = w.buffer[1:]
		
		if w.timeout > 0 {
			w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
		}
		
		if _, err := w.conn.Write(data); err != nil {
			// 写入失败，重新缓存数据
			w.buffer = append([][]byte{data}, w.buffer...)
			w.conn.Close()
			w.conn = nil
			break
		}
	}
}

// HTTPWriter HTTP写入器
type HTTPWriter struct {
	mu     sync.Mutex
	url    string
	client *http.Client
	headers map[string]string
	method string
}

// HTTPWriterConfig HTTP写入器配置
type HTTPWriterConfig struct {
	URL     string            // URL地址
	Method  string            // HTTP方法
	Headers map[string]string // HTTP头
	Timeout time.Duration     // 超时时间
}

// NewHTTPWriter 创建HTTP写入器
func NewHTTPWriter(config *HTTPWriterConfig) *HTTPWriter {
	if config == nil {
		config = &HTTPWriterConfig{}
	}
	
	if config.Method == "" {
		config.Method = "POST"
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	
	client := &http.Client{
		Timeout: config.Timeout,
	}
	
	return &HTTPWriter{
		url:     config.URL,
		client:  client,
		headers: config.Headers,
		method:  config.Method,
	}
}

// Write 写入数据
func (w *HTTPWriter) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	req, err := http.NewRequest(w.method, w.url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// 设置头部
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}
	
	// 设置默认Content-Type
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// Close 关闭写入器
func (w *HTTPWriter) Close() error {
	return nil
}

// MultiWriter 多重写入器
type MultiWriter struct {
	writers []Writer
}

// NewMultiWriter 创建多重写入器
func NewMultiWriter(writers ...Writer) *MultiWriter {
	return &MultiWriter{
		writers: writers,
	}
}

// Write 写入数据
func (w *MultiWriter) Write(data []byte) error {
	var errs []error
	
	for _, writer := range w.writers {
		if err := writer.Write(data); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("multi writer errors: %v", errs)
	}
	
	return nil
}

// Close 关闭写入器
func (w *MultiWriter) Close() error {
	var errs []error
	
	for _, writer := range w.writers {
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("multi writer close errors: %v", errs)
	}
	
	return nil
}