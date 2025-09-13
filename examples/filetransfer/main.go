package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netcore-go"
	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/pool"
)

// FileTransferMessage 文件传输消息
type FileTransferMessage struct {
	Type      string      `json:"type"`
	FileID    string      `json:"file_id"`
	Filename  string      `json:"filename"`
	FileSize  int64       `json:"file_size"`
	ChunkSize int         `json:"chunk_size"`
	ChunkID   int         `json:"chunk_id"`
	TotalChunks int       `json:"total_chunks"`
	Data      []byte      `json:"data,omitempty"`
	Checksum  string      `json:"checksum"`
	Metadata  interface{} `json:"metadata,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// FileInfo 文件信息
type FileInfo struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	ChunkSize   int       `json:"chunk_size"`
	TotalChunks int       `json:"total_chunks"`
	UploadedChunks []bool `json:"uploaded_chunks"`
	Progress    float64   `json:"progress"`
	Status      string    `json:"status"` // uploading, completed, failed, paused
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	FilePath    string    `json:"file_path"`
}

// Client 客户端信息
type Client struct {
	ID       string          `json:"id"`
	Conn     core.Connection `json:"-"`
	Files    map[string]*FileInfo `json:"files"`
	LastSeen time.Time       `json:"last_seen"`
	mu       sync.RWMutex    `json:"-"`
}

// FileTransferServer 文件传输服务器
type FileTransferServer struct {
	server      core.Server
	uploadDir   string
	downloadDir string
	files       map[string]*FileInfo
	clients     map[string]*Client
	pool        pool.ConnectionPool
	stats       *TransferStats
	mu          sync.RWMutex
}

// TransferStats 传输统计
type TransferStats struct {
	TotalFiles      int64 `json:"total_files"`
	CompletedFiles  int64 `json:"completed_files"`
	TotalBytes      int64 `json:"total_bytes"`
	TransferredBytes int64 `json:"transferred_bytes"`
	ActiveTransfers int64 `json:"active_transfers"`
	TransferRate    int64 `json:"transfer_rate"` // bytes per second
	LastUpdateTime  int64 `json:"last_update_time"`
	mu              sync.RWMutex
}

const (
	DefaultChunkSize = 64 * 1024 // 64KB
	MaxFileSize      = 1024 * 1024 * 1024 // 1GB
)

// NewFileTransferServer 创建文件传输服务器
func NewFileTransferServer(uploadDir, downloadDir string) *FileTransferServer {
	fs := &FileTransferServer{
		clients:     make(map[string]*Client),
		files:       make(map[string]*FileInfo),
		uploadDir:   uploadDir,
		downloadDir: downloadDir,
		stats:       &TransferStats{},
	}

	// 创建目录
	os.MkdirAll(uploadDir, 0755)
	os.MkdirAll(downloadDir, 0755)

	// 创建连接池
	fs.pool = pool.NewConnectionPool(&pool.Config{
		MinSize:     5,
		MaxSize:     100,
		IdleTimeout: time.Minute * 10,
		ConnTimeout: time.Second * 30,
	})

	return fs
}

// Start 启动文件传输服务器
func (fs *FileTransferServer) Start(addr string) error {
	config := &netcore.Config{
		Host: "localhost",
		Port: 8080,
	}

	fs.server = netcore.NewServer(config)

	// 设置处理器
	// fs.server.SetMessageHandler(fs.handleMessage)
	// fs.server.SetConnectHandler(fs.handleConnect)
	// fs.server.SetDisconnectHandler(fs.handleDisconnect)

	// 启动统计更新
	go fs.updateStats()

	// 启动清理任务
	go fs.cleanupTask()

	log.Printf("文件传输服务器启动在 %s", addr)
	log.Printf("上传目录: %s", fs.uploadDir)
	log.Printf("下载目录: %s", fs.downloadDir)

	return fs.server.Start(addr)
}

// handleConnect 处理连接
func (fs *FileTransferServer) handleConnect(conn core.Connection) {
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	log.Printf("新客户端连接: %s (%s)", clientID, conn.RemoteAddr())

	// 添加到连接池
	// fs.pool.AddConnection(conn) // 暂时注释掉，因为类型不匹配

	// 创建客户端
	client := &Client{
		ID:       clientID,
		Conn:     conn,
		Files:    make(map[string]*FileInfo),
		LastSeen: time.Now(),
	}

	fs.mu.Lock()
	fs.clients[clientID] = client
	fs.mu.Unlock()

	// 发送欢迎消息
	welcomeMsg := &FileTransferMessage{
		Type:      "welcome",
		Metadata: map[string]interface{}{
			"client_id":        clientID,
			"max_file_size":    MaxFileSize,
			"default_chunk_size": DefaultChunkSize,
			"supported_commands": []string{"upload", "download", "list", "delete", "pause", "resume"},
		},
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, welcomeMsg)
}

// handleDisconnect 处理断开连接
func (fs *FileTransferServer) handleDisconnect(conn core.Connection) {
	log.Printf("客户端断开连接: %s", conn.RemoteAddr())

	// 从连接池移除
	// fs.pool.RemoveConnection(conn) // 暂时注释掉，因为类型不匹配

	// 查找并移除客户端
	fs.mu.Lock()
	defer fs.mu.Unlock()

	for clientID, client := range fs.clients {
		if client.Conn.ID() == conn.ID() {
			// 暂停所有正在进行的传输
			client.mu.Lock()
			for _, fileInfo := range client.Files {
				if fileInfo.Status == "uploading" {
					fileInfo.Status = "paused"
					fileInfo.UpdatedAt = time.Now()
				}
			}
			client.mu.Unlock()

			delete(fs.clients, clientID)
			break
		}
	}
}

// handleMessage 处理消息
func (fs *FileTransferServer) handleMessage(conn core.Connection, data []byte) {
	var msg FileTransferMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("解析消息失败: %v", err)
		return
	}

	msg.Timestamp = time.Now().Unix()

	switch msg.Type {
	case "upload_start":
		fs.handleUploadStart(conn, &msg)
	case "upload_chunk":
		fs.handleUploadChunk(conn, &msg)
	case "upload_complete":
		fs.handleUploadComplete(conn, &msg)
	case "download_request":
		fs.handleDownloadRequest(conn, &msg)
	case "list_files":
		fs.handleListFiles(conn, &msg)
	case "delete_file":
		fs.handleDeleteFile(conn, &msg)
	case "pause_transfer":
		fs.handlePauseTransfer(conn, &msg)
	case "resume_transfer":
		fs.handleResumeTransfer(conn, &msg)
	case "get_progress":
		fs.handleGetProgress(conn, &msg)
	default:
		log.Printf("未知消息类型: %s", msg.Type)
	}
}

// handleUploadStart 处理上传开始
func (fs *FileTransferServer) handleUploadStart(conn core.Connection, msg *FileTransferMessage) {
	client := fs.findClientByConnection(conn)
	if client == nil {
		fs.sendError(conn, "客户端未找到")
		return
	}

	// 验证文件大小
	if msg.FileSize > MaxFileSize {
		fs.sendError(conn, fmt.Sprintf("文件大小超过限制 (%d bytes)", MaxFileSize))
		return
	}

	// 设置默认分块大小
	if msg.ChunkSize <= 0 {
		msg.ChunkSize = DefaultChunkSize
	}

	// 计算总分块数
	totalChunks := int((msg.FileSize + int64(msg.ChunkSize) - 1) / int64(msg.ChunkSize))

	// 创建文件信息
	fileInfo := &FileInfo{
		ID:             msg.FileID,
		Filename:       msg.Filename,
		Size:           msg.FileSize,
		ChunkSize:      msg.ChunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: make([]bool, totalChunks),
		Progress:       0.0,
		Status:         "uploading",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		FilePath:       filepath.Join(fs.uploadDir, msg.FileID+"_"+msg.Filename),
	}

	// 检查是否为断点续传
	fs.mu.RLock()
	existingFile, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if exists {
		// 断点续传
		fileInfo = existingFile
		fileInfo.Status = "uploading"
		fileInfo.UpdatedAt = time.Now()
		log.Printf("断点续传文件: %s (进度: %.2f%%)", msg.Filename, fileInfo.Progress)
	} else {
		// 新文件上传
		fs.mu.Lock()
		fs.files[msg.FileID] = fileInfo
		fs.mu.Unlock()

		// 创建文件
		file, err := os.Create(fileInfo.FilePath)
		if err != nil {
			fs.sendError(conn, fmt.Sprintf("创建文件失败: %v", err))
			return
		}
		file.Close()

		log.Printf("开始上传文件: %s (大小: %d bytes)", msg.Filename, msg.FileSize)
	}

	// 添加到客户端文件列表
	client.mu.Lock()
	client.Files[msg.FileID] = fileInfo
	client.LastSeen = time.Now()
	client.mu.Unlock()

	// 发送响应
	response := &FileTransferMessage{
		Type:        "upload_ready",
		FileID:      msg.FileID,
		TotalChunks: fileInfo.TotalChunks,
		Metadata: map[string]interface{}{
			"uploaded_chunks": fileInfo.UploadedChunks,
			"progress":        fileInfo.Progress,
		},
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)
}

// handleUploadChunk 处理上传分块
func (fs *FileTransferServer) handleUploadChunk(conn core.Connection, msg *FileTransferMessage) {
	client := fs.findClientByConnection(conn)
	if client == nil {
		fs.sendError(conn, "客户端未找到")
		return
	}

	fs.mu.RLock()
	fileInfo, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	if fileInfo.Status != "uploading" {
		fs.sendError(conn, "文件未处于上传状态")
		return
	}

	// 验证分块ID
	if msg.ChunkID < 0 || msg.ChunkID >= fileInfo.TotalChunks {
		fs.sendError(conn, "无效的分块ID")
		return
	}

	// 检查分块是否已上传
	if fileInfo.UploadedChunks[msg.ChunkID] {
		// 分块已存在，发送确认
		fs.sendChunkAck(conn, msg.FileID, msg.ChunkID, true)
		return
	}

	// 写入分块数据
	file, err := os.OpenFile(fileInfo.FilePath, os.O_WRONLY, 0644)
	if err != nil {
		fs.sendError(conn, fmt.Sprintf("打开文件失败: %v", err))
		return
	}
	defer file.Close()

	// 定位到分块位置
	offset := int64(msg.ChunkID) * int64(fileInfo.ChunkSize)
	_, err = file.Seek(offset, 0)
	if err != nil {
		fs.sendError(conn, fmt.Sprintf("定位文件失败: %v", err))
		return
	}

	// 写入数据
	_, err = file.Write(msg.Data)
	if err != nil {
		fs.sendError(conn, fmt.Sprintf("写入文件失败: %v", err))
		return
	}

	// 更新上传状态
	fileInfo.UploadedChunks[msg.ChunkID] = true
	fileInfo.UpdatedAt = time.Now()

	// 计算进度
	uploadedCount := 0
	for _, uploaded := range fileInfo.UploadedChunks {
		if uploaded {
			uploadedCount++
		}
	}
	fileInfo.Progress = float64(uploadedCount) / float64(fileInfo.TotalChunks) * 100

	// 更新统计
	fs.stats.mu.Lock()
	fs.stats.TransferredBytes += int64(len(msg.Data))
	fs.stats.mu.Unlock()

	// 更新客户端最后活跃时间
	client.mu.Lock()
	client.LastSeen = time.Now()
	client.mu.Unlock()

	// 发送确认
	fs.sendChunkAck(conn, msg.FileID, msg.ChunkID, false)

	log.Printf("接收分块 %d/%d (文件: %s, 进度: %.2f%%)", 
		msg.ChunkID+1, fileInfo.TotalChunks, fileInfo.Filename, fileInfo.Progress)
}

// handleUploadComplete 处理上传完成
func (fs *FileTransferServer) handleUploadComplete(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.RLock()
	fileInfo, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	// 验证所有分块都已上传
	for i, uploaded := range fileInfo.UploadedChunks {
		if !uploaded {
			fs.sendError(conn, fmt.Sprintf("分块 %d 未上传", i))
			return
		}
	}

	// 验证文件完整性
	if msg.Checksum != "" {
		calculatedChecksum, err := fs.calculateFileChecksum(fileInfo.FilePath)
		if err != nil {
			fs.sendError(conn, fmt.Sprintf("计算校验和失败: %v", err))
			return
		}

		if calculatedChecksum != msg.Checksum {
			fs.sendError(conn, "文件校验和不匹配")
			return
		}

		fileInfo.Checksum = calculatedChecksum
	}

	// 更新文件状态
	fileInfo.Status = "completed"
	fileInfo.Progress = 100.0
	fileInfo.UpdatedAt = time.Now()

	// 更新统计
	fs.stats.mu.Lock()
	fs.stats.CompletedFiles++
	fs.stats.mu.Unlock()

	// 发送完成确认
	response := &FileTransferMessage{
		Type:      "upload_completed",
		FileID:    msg.FileID,
		Checksum:  fileInfo.Checksum,
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)

	log.Printf("文件上传完成: %s (大小: %d bytes)", fileInfo.Filename, fileInfo.Size)
}

// handleDownloadRequest 处理下载请求
func (fs *FileTransferServer) handleDownloadRequest(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.RLock()
	fileInfo, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	if fileInfo.Status != "completed" {
		fs.sendError(conn, "文件未完成上传")
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(fileInfo.FilePath); os.IsNotExist(err) {
		fs.sendError(conn, "文件不存在")
		return
	}

	// 发送文件信息
	response := &FileTransferMessage{
		Type:        "download_ready",
		FileID:      fileInfo.ID,
		Filename:    fileInfo.Filename,
		FileSize:    fileInfo.Size,
		ChunkSize:   fileInfo.ChunkSize,
		TotalChunks: fileInfo.TotalChunks,
		Checksum:    fileInfo.Checksum,
		Timestamp:   time.Now().Unix(),
	}

	fs.sendMessage(conn, response)

	// 开始发送文件分块
	go fs.sendFileChunks(conn, fileInfo)
}

// sendFileChunks 发送文件分块
func (fs *FileTransferServer) sendFileChunks(conn core.Connection, fileInfo *FileInfo) {
	file, err := os.Open(fileInfo.FilePath)
	if err != nil {
		fs.sendError(conn, fmt.Sprintf("打开文件失败: %v", err))
		return
	}
	defer file.Close()

	buffer := make([]byte, fileInfo.ChunkSize)

	for chunkID := 0; chunkID < fileInfo.TotalChunks; chunkID++ {
		// 读取分块数据
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			fs.sendError(conn, fmt.Sprintf("读取文件失败: %v", err))
			return
		}

		// 发送分块
		chunkMsg := &FileTransferMessage{
			Type:      "download_chunk",
			FileID:    fileInfo.ID,
			ChunkID:   chunkID,
			Data:      buffer[:n],
			Timestamp: time.Now().Unix(),
		}

		fs.sendMessage(conn, chunkMsg)

		// 更新统计
		fs.stats.mu.Lock()
		fs.stats.TransferredBytes += int64(n)
		fs.stats.mu.Unlock()

		// 控制发送速度，避免网络拥塞
		time.Sleep(time.Millisecond * 10)
	}

	// 发送下载完成消息
	completeMsg := &FileTransferMessage{
		Type:      "download_completed",
		FileID:    fileInfo.ID,
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, completeMsg)

	log.Printf("文件下载完成: %s", fileInfo.Filename)
}

// handleListFiles 处理文件列表请求
func (fs *FileTransferServer) handleListFiles(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.RLock()
	fileList := make([]*FileInfo, 0, len(fs.files))
	for _, fileInfo := range fs.files {
		fileList = append(fileList, fileInfo)
	}
	fs.mu.RUnlock()

	response := &FileTransferMessage{
		Type:      "file_list",
		Metadata:  fileList,
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)
}

// handleDeleteFile 处理删除文件
func (fs *FileTransferServer) handleDeleteFile(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.Lock()
	fileInfo, exists := fs.files[msg.FileID]
	if exists {
		delete(fs.files, msg.FileID)
	}
	fs.mu.Unlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	// 删除物理文件
	if err := os.Remove(fileInfo.FilePath); err != nil {
		log.Printf("删除文件失败: %v", err)
	}

	// 从客户端文件列表移除
	client := fs.findClientByConnection(conn)
	if client != nil {
		client.mu.Lock()
		delete(client.Files, msg.FileID)
		client.mu.Unlock()
	}

	response := &FileTransferMessage{
		Type:      "file_deleted",
		FileID:    msg.FileID,
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)

	log.Printf("文件已删除: %s", fileInfo.Filename)
}

// handlePauseTransfer 处理暂停传输
func (fs *FileTransferServer) handlePauseTransfer(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.RLock()
	fileInfo, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	fileInfo.Status = "paused"
	fileInfo.UpdatedAt = time.Now()

	response := &FileTransferMessage{
		Type:      "transfer_paused",
		FileID:    msg.FileID,
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)

	log.Printf("传输已暂停: %s", fileInfo.Filename)
}

// handleResumeTransfer 处理恢复传输
func (fs *FileTransferServer) handleResumeTransfer(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.RLock()
	fileInfo, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	fileInfo.Status = "uploading"
	fileInfo.UpdatedAt = time.Now()

	response := &FileTransferMessage{
		Type:      "transfer_resumed",
		FileID:    msg.FileID,
		Metadata: map[string]interface{}{
			"uploaded_chunks": fileInfo.UploadedChunks,
			"progress":        fileInfo.Progress,
		},
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)

	log.Printf("传输已恢复: %s", fileInfo.Filename)
}

// handleGetProgress 处理获取进度
func (fs *FileTransferServer) handleGetProgress(conn core.Connection, msg *FileTransferMessage) {
	fs.mu.RLock()
	fileInfo, exists := fs.files[msg.FileID]
	fs.mu.RUnlock()

	if !exists {
		fs.sendError(conn, "文件未找到")
		return
	}

	response := &FileTransferMessage{
		Type:   "progress_info",
		FileID: msg.FileID,
		Metadata: map[string]interface{}{
			"filename":        fileInfo.Filename,
			"progress":        fileInfo.Progress,
			"status":          fileInfo.Status,
			"uploaded_chunks": fileInfo.UploadedChunks,
			"total_chunks":    fileInfo.TotalChunks,
			"file_size":       fileInfo.Size,
		},
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, response)
}

// sendMessage 发送消息
func (fs *FileTransferServer) sendMessage(conn core.Connection, msg *FileTransferMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("序列化消息失败: %v", err)
		return
	}

	coreMsg := core.NewMessage(core.MessageTypeJSON, data)
	conn.SendMessage(*coreMsg)
}

// sendError 发送错误消息
func (fs *FileTransferServer) sendError(conn core.Connection, errorMsg string) {
	errorResponse := &FileTransferMessage{
		Type:      "error",
		Metadata:  errorMsg,
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, errorResponse)
}

// sendChunkAck 发送分块确认
func (fs *FileTransferServer) sendChunkAck(conn core.Connection, fileID string, chunkID int, duplicate bool) {
	ackMsg := &FileTransferMessage{
		Type:    "chunk_ack",
		FileID:  fileID,
		ChunkID: chunkID,
		Metadata: map[string]interface{}{
			"duplicate": duplicate,
		},
		Timestamp: time.Now().Unix(),
	}

	fs.sendMessage(conn, ackMsg)
}

// findClientByConnection 根据连接查找客户端
func (fs *FileTransferServer) findClientByConnection(conn core.Connection) *Client {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	for _, client := range fs.clients {
		if client.Conn.ID() == conn.ID() {
			return client
		}
	}

	return nil
}

// calculateFileChecksum 计算文件校验和
func (fs *FileTransferServer) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// updateStats 更新统计信息
func (fs *FileTransferServer) updateStats() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	lastTransferredBytes := int64(0)

	for range ticker.C {
		fs.mu.RLock()
		totalFiles := int64(len(fs.files))
		completedFiles := int64(0)
		totalBytes := int64(0)
		activeTransfers := int64(0)

		for _, fileInfo := range fs.files {
			totalBytes += fileInfo.Size
			if fileInfo.Status == "completed" {
				completedFiles++
			} else if fileInfo.Status == "uploading" {
				activeTransfers++
			}
		}
		fs.mu.RUnlock()

		fs.stats.mu.Lock()
		currentTransferredBytes := fs.stats.TransferredBytes
		transferRate := (currentTransferredBytes - lastTransferredBytes) / 5 // bytes per second
		lastTransferredBytes = currentTransferredBytes

		fs.stats.TotalFiles = totalFiles
		fs.stats.CompletedFiles = completedFiles
		fs.stats.TotalBytes = totalBytes
		fs.stats.ActiveTransfers = activeTransfers
		fs.stats.TransferRate = transferRate
		fs.stats.LastUpdateTime = time.Now().Unix()
		fs.stats.mu.Unlock()

		log.Printf("传输统计 - 文件: %d/%d, 活跃传输: %d, 传输速率: %d B/s", 
			completedFiles, totalFiles, activeTransfers, transferRate)
	}
}

// cleanupTask 清理任务
func (fs *FileTransferServer) cleanupTask() {
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// 清理超时的未完成文件
		fs.mu.Lock()
		for fileID, fileInfo := range fs.files {
			if fileInfo.Status == "uploading" && now.Sub(fileInfo.UpdatedAt) > time.Hour {
				// 删除超时文件
				os.Remove(fileInfo.FilePath)
				delete(fs.files, fileID)
				log.Printf("清理超时文件: %s", fileInfo.Filename)
			}
		}
		fs.mu.Unlock()

		// 清理离线客户端
		fs.mu.Lock()
		for clientID, client := range fs.clients {
			if now.Sub(client.LastSeen) > time.Minute*30 {
				delete(fs.clients, clientID)
				log.Printf("清理离线客户端: %s", clientID)
			}
		}
		fs.mu.Unlock()
	}
}

// GetStats 获取统计信息
func (fs *FileTransferServer) GetStats() *TransferStats {
	fs.stats.mu.RLock()
	defer fs.stats.mu.RUnlock()

	return &TransferStats{
		TotalFiles:       fs.stats.TotalFiles,
		CompletedFiles:   fs.stats.CompletedFiles,
		TotalBytes:       fs.stats.TotalBytes,
		TransferredBytes: fs.stats.TransferredBytes,
		ActiveTransfers:  fs.stats.ActiveTransfers,
		TransferRate:     fs.stats.TransferRate,
		LastUpdateTime:   fs.stats.LastUpdateTime,
	}
}

// Stop 停止文件传输服务器
func (fs *FileTransferServer) Stop() error {
	// if fs.pool != nil {
	// 	fs.pool.Close()
	// }

	if fs.server != nil {
		return fs.server.Stop()
	}

	return nil
}

func main() {
	uploadDir := "./uploads"
	downloadDir := "./downloads"

	fileServer := NewFileTransferServer(uploadDir, downloadDir)

	// 启动HTTP API服务器
	go func() {
		http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			stats := fileServer.GetStats()
			json.NewEncoder(w).Encode(stats)
		})

		http.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fileServer.mu.RLock()
			files := make(map[string]*FileInfo)
			for id, file := range fileServer.files {
				files[id] = file
			}
			fileServer.mu.RUnlock()
			json.NewEncoder(w).Encode(files)
		})

		http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
			fileID := strings.TrimPrefix(r.URL.Path, "/download/")
			if fileID == "" {
				http.Error(w, "文件ID不能为空", http.StatusBadRequest)
				return
			}

			fileServer.mu.RLock()
			fileInfo, exists := fileServer.files[fileID]
			fileServer.mu.RUnlock()

			if !exists {
				http.Error(w, "文件未找到", http.StatusNotFound)
				return
			}

			if fileInfo.Status != "completed" {
				http.Error(w, "文件未完成上传", http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileInfo.Filename))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))

			http.ServeFile(w, r, fileInfo.FilePath)
		})

		log.Println("HTTP API服务器启动在 :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 启动文件传输服务器
	if err := fileServer.Start(":9997"); err != nil {
		log.Fatalf("启动文件传输服务器失败: %v", err)
	}
}


