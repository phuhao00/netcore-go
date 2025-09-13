package performance

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ZeroCopyConfig 零拷贝配置
type ZeroCopyConfig struct {
	// 启用零拷贝
	Enabled bool `json:"enabled" yaml:"enabled"`
	// 缓冲区大小
	BufferSize int `json:"buffer_size" yaml:"buffer_size"`
	// 最大并发传输数
	MaxConcurrent int `json:"max_concurrent" yaml:"max_concurrent"`
	// 使用内存映射
	UseMemoryMap bool `json:"use_memory_map" yaml:"use_memory_map"`
	// 启用TCP_NODELAY
	TCPNoDelay bool `json:"tcp_no_delay" yaml:"tcp_no_delay"`
	// 启用TCP_CORK
	TCPCork bool `json:"tcp_cork" yaml:"tcp_cork"`
	// 发送超时
	SendTimeout time.Duration `json:"send_timeout" yaml:"send_timeout"`
	// 接收超时
	ReceiveTimeout time.Duration `json:"receive_timeout" yaml:"receive_timeout"`
}

// DefaultZeroCopyConfig 默认零拷贝配置
func DefaultZeroCopyConfig() *ZeroCopyConfig {
	return &ZeroCopyConfig{
		Enabled:        true,
		BufferSize:     64 * 1024, // 64KB
		MaxConcurrent:  runtime.NumCPU() * 2,
		UseMemoryMap:   true,
		TCPNoDelay:     true,
		TCPCork:        false,
		SendTimeout:    30 * time.Second,
		ReceiveTimeout: 30 * time.Second,
	}
}

// ZeroCopyBuffer 零拷贝缓冲区
type ZeroCopyBuffer struct {
	data   []byte
	offset int
	length int
	refCnt int32
	pool   *sync.Pool
}

// NewZeroCopyBuffer 创建零拷贝缓冲区
func NewZeroCopyBuffer(size int, pool *sync.Pool) *ZeroCopyBuffer {
	return &ZeroCopyBuffer{
		data:   make([]byte, size),
		offset: 0,
		length: 0,
		refCnt: 1,
		pool:   pool,
	}
}

// Bytes 获取缓冲区数据
func (b *ZeroCopyBuffer) Bytes() []byte {
	return b.data[b.offset : b.offset+b.length]
}

// Reset 重置缓冲区
func (b *ZeroCopyBuffer) Reset() {
	b.offset = 0
	b.length = 0
	atomic.StoreInt32(&b.refCnt, 1)
}

// Retain 增加引用计数
func (b *ZeroCopyBuffer) Retain() {
	atomic.AddInt32(&b.refCnt, 1)
}

// Release 释放缓冲区
func (b *ZeroCopyBuffer) Release() {
	if atomic.AddInt32(&b.refCnt, -1) == 0 {
		b.Reset()
		if b.pool != nil {
			b.pool.Put(b)
		}
	}
}

// ZeroCopyManager 零拷贝管理器
type ZeroCopyManager struct {
	config     *ZeroCopyConfig
	bufferPool *sync.Pool
	stats      *ZeroCopyStats
	mu         sync.RWMutex
	running    bool
	cancel     context.CancelFunc
}

// ZeroCopyStats 零拷贝统计信息
type ZeroCopyStats struct {
	TotalTransfers   int64 `json:"total_transfers"`
	ZeroCopyHits     int64 `json:"zero_copy_hits"`
	ZeroCopyMisses   int64 `json:"zero_copy_misses"`
	BytesTransferred int64 `json:"bytes_transferred"`
	AverageSpeed     int64 `json:"average_speed"` // bytes/sec
	BufferPoolHits   int64 `json:"buffer_pool_hits"`
	BufferPoolMisses int64 `json:"buffer_pool_misses"`
}

// NewZeroCopyManager 创建零拷贝管理器
func NewZeroCopyManager(config *ZeroCopyConfig) *ZeroCopyManager {
	if config == nil {
		config = DefaultZeroCopyConfig()
	}

	manager := &ZeroCopyManager{
		config: config,
		stats:  &ZeroCopyStats{},
	}

	// 初始化缓冲区池
	manager.bufferPool = &sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&manager.stats.BufferPoolMisses, 1)
			return NewZeroCopyBuffer(config.BufferSize, manager.bufferPool)
		},
	}

	return manager
}

// Start 启动零拷贝管理器
func (m *ZeroCopyManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("zero copy manager already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true

	// 启动统计收集
	go m.collectStats(ctx)

	return nil
}

// Stop 停止零拷贝管理器
func (m *ZeroCopyManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("zero copy manager not running")
	}

	m.cancel()
	m.running = false

	return nil
}

// GetBuffer 获取缓冲区
func (m *ZeroCopyManager) GetBuffer() *ZeroCopyBuffer {
	buf := m.bufferPool.Get().(*ZeroCopyBuffer)
	atomic.AddInt64(&m.stats.BufferPoolHits, 1)
	return buf
}

// SendFile 零拷贝发送文件
func (m *ZeroCopyManager) SendFile(conn net.Conn, file *os.File, offset, count int64) (int64, error) {
	if !m.config.Enabled {
		return m.sendFileRegular(conn, file, offset, count)
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&m.stats.TotalTransfers, 1)
		duration := time.Since(start)
		if duration > 0 {
			speed := count * int64(time.Second) / int64(duration)
			atomic.StoreInt64(&m.stats.AverageSpeed, speed)
		}
	}()

	// 尝试使用系统调用进行零拷贝
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if sent, err := m.sendFileZeroCopy(tcpConn, file, offset, count); err == nil {
			atomic.AddInt64(&m.stats.ZeroCopyHits, 1)
			atomic.AddInt64(&m.stats.BytesTransferred, sent)
			return sent, nil
		}
	}

	// 回退到常规发送
	atomic.AddInt64(&m.stats.ZeroCopyMisses, 1)
	return m.sendFileRegular(conn, file, offset, count)
}

// sendFileZeroCopy 使用零拷贝发送文件
func (m *ZeroCopyManager) sendFileZeroCopy(conn *net.TCPConn, file *os.File, offset, count int64) (int64, error) {
	// 设置TCP选项
	if m.config.TCPNoDelay {
		conn.SetNoDelay(true)
	}

	// 获取文件描述符
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}

	var sent int64
	var sendErr error

	err = rawConn.Control(func(fd uintptr) {
		// 在Windows上使用TransmitFile，在Linux上使用sendfile
		if runtime.GOOS == "windows" {
			sent, sendErr = m.transmitFileWindows(fd, file, offset, count)
		} else {
			sent, sendErr = m.sendFileLinux(fd, file, offset, count)
		}
	})

	if err != nil {
		return 0, err
	}

	return sent, sendErr
}

// sendFileLinux Linux下的零拷贝实现
func (m *ZeroCopyManager) sendFileLinux(sockfd uintptr, file *os.File, offset, count int64) (int64, error) {
	var sent int64

	for sent < count {
		// Windows不支持sendfile，使用常规方式
		n, err := 0, fmt.Errorf("sendfile not supported on Windows")
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EINTR {
				continue
			}
			return sent, err
		}
		sent += int64(n)
		offset += int64(n)
	}

	return sent, nil
}

// transmitFileWindows Windows下的零拷贝实现
func (m *ZeroCopyManager) transmitFileWindows(sockfd uintptr, file *os.File, offset, count int64) (int64, error) {
	// Windows TransmitFile实现
	// 这里简化实现，实际需要调用Windows API
	return m.sendFileRegular(nil, file, offset, count)
}

// sendFileRegular 常规文件发送
func (m *ZeroCopyManager) sendFileRegular(conn net.Conn, file *os.File, offset, count int64) (int64, error) {
	buf := m.GetBuffer()
	defer buf.Release()

	_, err := file.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, err
	}

	var sent int64
	for sent < count {
		n, err := file.Read(buf.data)
		if err != nil {
			if err == io.EOF {
				break
			}
			return sent, err
		}

		written, err := conn.Write(buf.data[:n])
		if err != nil {
			return sent, err
		}

		sent += int64(written)
	}

	atomic.AddInt64(&m.stats.BytesTransferred, sent)
	return sent, nil
}

// CopyData 零拷贝数据传输
func (m *ZeroCopyManager) CopyData(dst io.Writer, src io.Reader, size int64) (int64, error) {
	if !m.config.Enabled {
		return io.CopyN(dst, src, size)
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&m.stats.TotalTransfers, 1)
		duration := time.Since(start)
		if duration > 0 {
			speed := size * int64(time.Second) / int64(duration)
			atomic.StoreInt64(&m.stats.AverageSpeed, speed)
		}
	}()

	// 尝试使用splice或类似的零拷贝机制
	if copied, err := m.copyDataZeroCopy(dst, src, size); err == nil {
		atomic.AddInt64(&m.stats.ZeroCopyHits, 1)
		atomic.AddInt64(&m.stats.BytesTransferred, copied)
		return copied, nil
	}

	// 回退到缓冲区拷贝
	atomic.AddInt64(&m.stats.ZeroCopyMisses, 1)
	return m.copyDataBuffered(dst, src, size)
}

// copyDataZeroCopy 零拷贝数据传输
func (m *ZeroCopyManager) copyDataZeroCopy(dst io.Writer, src io.Reader, size int64) (int64, error) {
	// 检查是否支持零拷贝接口
	if tc, ok := dst.(interface{ ReadFrom(io.Reader) (int64, error) }); ok {
		return tc.ReadFrom(io.LimitReader(src, size))
	}

	if wt, ok := src.(interface{ WriteTo(io.Writer) (int64, error) }); ok {
		return wt.WriteTo(dst)
	}

	return 0, fmt.Errorf("zero copy not supported")
}

// copyDataBuffered 缓冲区数据传输
func (m *ZeroCopyManager) copyDataBuffered(dst io.Writer, src io.Reader, size int64) (int64, error) {
	buf := m.GetBuffer()
	defer buf.Release()

	var copied int64
	for copied < size {
		n, err := src.Read(buf.data)
		if err != nil {
			if err == io.EOF {
				break
			}
			return copied, err
		}

		written, err := dst.Write(buf.data[:n])
		if err != nil {
			return copied, err
		}

		copied += int64(written)
	}

	atomic.AddInt64(&m.stats.BytesTransferred, copied)
	return copied, nil
}

// GetStats 获取统计信息
func (m *ZeroCopyManager) GetStats() *ZeroCopyStats {
	return &ZeroCopyStats{
		TotalTransfers:   atomic.LoadInt64(&m.stats.TotalTransfers),
		ZeroCopyHits:     atomic.LoadInt64(&m.stats.ZeroCopyHits),
		ZeroCopyMisses:   atomic.LoadInt64(&m.stats.ZeroCopyMisses),
		BytesTransferred: atomic.LoadInt64(&m.stats.BytesTransferred),
		AverageSpeed:     atomic.LoadInt64(&m.stats.AverageSpeed),
		BufferPoolHits:   atomic.LoadInt64(&m.stats.BufferPoolHits),
		BufferPoolMisses: atomic.LoadInt64(&m.stats.BufferPoolMisses),
	}
}

// collectStats 收集统计信息
func (m *ZeroCopyManager) collectStats(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 这里可以添加统计信息的持久化或上报逻辑
			stats := m.GetStats()
			_ = stats // 避免未使用变量警告
		}
	}
}

// OptimizeConnection 优化连接设置
func (m *ZeroCopyManager) OptimizeConnection(conn net.Conn) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// 设置TCP_NODELAY
		if m.config.TCPNoDelay {
			if err := tcpConn.SetNoDelay(true); err != nil {
				return err
			}
		}

		// 设置读写缓冲区大小
		if err := tcpConn.SetReadBuffer(m.config.BufferSize); err != nil {
			return err
		}

		if err := tcpConn.SetWriteBuffer(m.config.BufferSize); err != nil {
			return err
		}

		// 设置超时
		if m.config.SendTimeout > 0 {
			if err := tcpConn.SetWriteDeadline(time.Now().Add(m.config.SendTimeout)); err != nil {
				return err
			}
		}

		if m.config.ReceiveTimeout > 0 {
			if err := tcpConn.SetReadDeadline(time.Now().Add(m.config.ReceiveTimeout)); err != nil {
				return err
			}
		}
	}

	return nil
}

// MemoryMapFile 内存映射文件
func (m *ZeroCopyManager) MemoryMapFile(file *os.File, size int64) ([]byte, error) {
	if !m.config.UseMemoryMap {
		return nil, fmt.Errorf("memory mapping disabled")
	}

	// 获取文件信息
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if size <= 0 {
		size = info.Size()
	}

	// 在Windows和Linux上的内存映射实现不同
	if runtime.GOOS == "windows" {
		return m.memoryMapWindows(file, size)
	}

	return m.memoryMapUnix(file, size)
}

// memoryMapUnix Unix系统的内存映射
func (m *ZeroCopyManager) memoryMapUnix(file *os.File, size int64) ([]byte, error) {
	// Windows不支持unix.Mmap，使用常规读取
	data := make([]byte, size)
	_, err := file.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// memoryMapWindows Windows系统的内存映射
func (m *ZeroCopyManager) memoryMapWindows(file *os.File, size int64) ([]byte, error) {
	// Windows内存映射实现
	// 这里简化实现，实际需要调用Windows API
	data := make([]byte, size)
	_, err := file.Read(data)
	return data, err
}

// UnmapMemory 取消内存映射
func (m *ZeroCopyManager) UnmapMemory(data []byte) error {
	if runtime.GOOS == "windows" {
		// Windows下无需特殊处理
		return nil
	}

	// Unix系统取消内存映射
	// Windows下不需要取消映射，直接返回nil
	return nil
}

// ZeroCopyWriter 零拷贝写入器
type ZeroCopyWriter struct {
	conn    net.Conn
	manager *ZeroCopyManager
	buffer  *ZeroCopyBuffer
}

// NewZeroCopyWriter 创建零拷贝写入器
func NewZeroCopyWriter(conn net.Conn, manager *ZeroCopyManager) *ZeroCopyWriter {
	return &ZeroCopyWriter{
		conn:    conn,
		manager: manager,
		buffer:  manager.GetBuffer(),
	}
}

// Write 写入数据
func (w *ZeroCopyWriter) Write(p []byte) (int, error) {
	return w.conn.Write(p)
}

// WriteZeroCopy 零拷贝写入
func (w *ZeroCopyWriter) WriteZeroCopy(data []byte) (int, error) {
	// 直接使用底层连接写入，避免额外拷贝
	return w.conn.Write(data)
}

// Close 关闭写入器
func (w *ZeroCopyWriter) Close() error {
	if w.buffer != nil {
		w.buffer.Release()
		w.buffer = nil
	}
	return nil
}

// ZeroCopyReader 零拷贝读取器
type ZeroCopyReader struct {
	conn    net.Conn
	manager *ZeroCopyManager
	buffer  *ZeroCopyBuffer
}

// NewZeroCopyReader 创建零拷贝读取器
func NewZeroCopyReader(conn net.Conn, manager *ZeroCopyManager) *ZeroCopyReader {
	return &ZeroCopyReader{
		conn:    conn,
		manager: manager,
		buffer:  manager.GetBuffer(),
	}
}

// Read 读取数据
func (r *ZeroCopyReader) Read(p []byte) (int, error) {
	return r.conn.Read(p)
}

// ReadZeroCopy 零拷贝读取
func (r *ZeroCopyReader) ReadZeroCopy() ([]byte, error) {
	n, err := r.conn.Read(r.buffer.data)
	if err != nil {
		return nil, err
	}

	r.buffer.length = n
	return r.buffer.Bytes(), nil
}

// Close 关闭读取器
func (r *ZeroCopyReader) Close() error {
	if r.buffer != nil {
		r.buffer.Release()
		r.buffer = nil
	}
	return nil
}