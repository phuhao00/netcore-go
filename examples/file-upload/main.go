// File Upload Service Example - NetCore-Go
// Comprehensive file upload and management service
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileInfo represents uploaded file information
type FileInfo struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	FileName     string    `json:"file_name"`
	FilePath     string    `json:"file_path"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	Checksum     string    `json:"checksum"`
	UploadedBy   string    `json:"uploaded_by"`
	UploadedAt   time.Time `json:"uploaded_at"`
	Downloads    int       `json:"downloads"`
	Public       bool      `json:"public"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Tags         []string  `json:"tags"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// UploadResponse represents the response after file upload
type UploadResponse struct {
	Success bool      `json:"success"`
	File    *FileInfo `json:"file,omitempty"`
	Error   string    `json:"error,omitempty"`
	URL     string    `json:"url,omitempty"`
}

// FileService handles file operations
type FileService struct {
	mu        sync.RWMutex
	files     map[string]*FileInfo
	uploadDir string
	maxSize   int64
	allowedTypes map[string]bool
}

// NewFileService creates a new file service
func NewFileService(uploadDir string, maxSize int64) *FileService {
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("Failed to create upload directory: %v", err)
	}
	
	// Define allowed file types
	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"text/plain":      true,
		"text/csv":        true,
		"application/pdf": true,
		"application/json": true,
		"application/zip": true,
		"video/mp4":       true,
		"audio/mpeg":      true,
	}
	
	return &FileService{
		files:        make(map[string]*FileInfo),
		uploadDir:    uploadDir,
		maxSize:      maxSize,
		allowedTypes: allowedTypes,
	}
}

// UploadFile handles file upload
func (fs *FileService) UploadFile(file multipart.File, header *multipart.FileHeader, uploadedBy string, public bool, tags []string) (*FileInfo, error) {
	// Validate file size
	if header.Size > fs.maxSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", fs.maxSize)
	}
	
	// Detect MIME type
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	
	mimeType := http.DetectContentType(buffer)
	
	// Validate file type
	if !fs.allowedTypes[mimeType] {
		return nil, fmt.Errorf("file type %s is not allowed", mimeType)
	}
	
	// Reset file pointer
	file.Seek(0, 0)
	
	// Generate unique filename
	fileID := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s%s", fileID, ext)
	filePath := filepath.Join(fs.uploadDir, fileName)
	
	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()
	
	// Copy file content and calculate checksum
	hash := md5.New()
	multiWriter := io.MultiWriter(dst, hash)
	
	size, err := io.Copy(multiWriter, file)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %v", err)
	}
	
	checksum := hex.EncodeToString(hash.Sum(nil))
	
	// Create file info
	fileInfo := &FileInfo{
		ID:           fileID,
		OriginalName: header.Filename,
		FileName:     fileName,
		FilePath:     filePath,
		FileSize:     size,
		MimeType:     mimeType,
		Checksum:     checksum,
		UploadedBy:   uploadedBy,
		UploadedAt:   time.Now(),
		Downloads:    0,
		Public:       public,
		Tags:         tags,
		Metadata:     make(map[string]interface{}),
	}
	
	// Add image metadata if it's an image
	if strings.HasPrefix(mimeType, "image/") {
		fileInfo.Metadata["type"] = "image"
		// In a real implementation, you could extract image dimensions here
	}
	
	// Store file info
	fs.mu.Lock()
	fs.files[fileID] = fileInfo
	fs.mu.Unlock()
	
	return fileInfo, nil
}

// GetFile returns file info by ID
func (fs *FileService) GetFile(fileID string) (*FileInfo, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	file, exists := fs.files[fileID]
	return file, exists
}

// GetAllFiles returns all files
func (fs *FileService) GetAllFiles() []*FileInfo {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	var files []*FileInfo
	for _, file := range fs.files {
		files = append(files, file)
	}
	return files
}

// GetPublicFiles returns all public files
func (fs *FileService) GetPublicFiles() []*FileInfo {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	var files []*FileInfo
	for _, file := range fs.files {
		if file.Public {
			files = append(files, file)
		}
	}
	return files
}

// GetFilesByUser returns files uploaded by a specific user
func (fs *FileService) GetFilesByUser(userID string) []*FileInfo {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	var files []*FileInfo
	for _, file := range fs.files {
		if file.UploadedBy == userID {
			files = append(files, file)
		}
	}
	return files
}

// IncrementDownloads increments the download count
func (fs *FileService) IncrementDownloads(fileID string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	if file, exists := fs.files[fileID]; exists {
		file.Downloads++
	}
}

// DeleteFile deletes a file
func (fs *FileService) DeleteFile(fileID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	file, exists := fs.files[fileID]
	if !exists {
		return fmt.Errorf("file not found")
	}
	
	// Delete physical file
	if err := os.Remove(file.FilePath); err != nil {
		log.Printf("Failed to delete physical file: %v", err)
	}
	
	// Remove from memory
	delete(fs.files, fileID)
	return nil
}

// GetStats returns service statistics
func (fs *FileService) GetStats() map[string]interface{} {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	totalFiles := len(fs.files)
	totalSize := int64(0)
	totalDownloads := 0
	publicFiles := 0
	typeStats := make(map[string]int)
	
	for _, file := range fs.files {
		totalSize += file.FileSize
		totalDownloads += file.Downloads
		if file.Public {
			publicFiles++
		}
		
		// Count by MIME type
		mainType := strings.Split(file.MimeType, "/")[0]
		typeStats[mainType]++
	}
	
	return map[string]interface{}{
		"total_files":     totalFiles,
		"total_size":      totalSize,
		"total_downloads": totalDownloads,
		"public_files":    publicFiles,
		"private_files":   totalFiles - publicFiles,
		"type_stats":      typeStats,
		"max_file_size":   fs.maxSize,
		"upload_dir":      fs.uploadDir,
	}
}

// HTTP Handlers
type FileHandler struct {
	service *FileService
}

func NewFileHandler(service *FileService) *FileHandler {
	return &FileHandler{service: service}
}

func (fh *FileHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Get form parameters
	uploadedBy := r.FormValue("uploaded_by")
	if uploadedBy == "" {
		uploadedBy = "anonymous"
	}
	
	public := r.FormValue("public") == "true"
	tagsStr := r.FormValue("tags")
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}
	
	// Upload file
	fileInfo, err := fh.service.UploadFile(file, header, uploadedBy, public, tags)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success: true,
		File:    fileInfo,
		URL:     fmt.Sprintf("/api/files/%s/download", fileInfo.ID),
	})
}

func (fh *FileHandler) handleFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Check query parameters
	userID := r.URL.Query().Get("user_id")
	publicOnly := r.URL.Query().Get("public") == "true"
	
	var files []*FileInfo
	
	if userID != "" {
		files = fh.service.GetFilesByUser(userID)
	} else if publicOnly {
		files = fh.service.GetPublicFiles()
	} else {
		files = fh.service.GetAllFiles()
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
		"total": len(files),
	})
}

func (fh *FileHandler) handleFileByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	
	fileID := parts[3]
	
	switch r.Method {
	case http.MethodGet:
		file, exists := fh.service.GetFile(fileID)
		if !exists {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(file)
		
	case http.MethodDelete:
		if err := fh.service.DeleteFile(fileID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		w.WriteHeader(http.StatusNoContent)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (fh *FileHandler) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Extract ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	
	fileID := parts[3]
	
	file, exists := fh.service.GetFile(fileID)
	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	
	// Check if file exists on disk
	if _, err := os.Stat(file.FilePath); os.IsNotExist(err) {
		http.Error(w, "File not found on disk", http.StatusNotFound)
		return
	}
	
	// Increment download count
	fh.service.IncrementDownloads(fileID)
	
	// Set headers
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.OriginalName))
	w.Header().Set("Content-Length", strconv.FormatInt(file.FileSize, 10))
	
	// Serve file
	http.ServeFile(w, r, file.FilePath)
}

func (fh *FileHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	stats := fh.service.GetStats()
	stats["generated_at"] = time.Now()
	
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Initialize file service
	uploadDir := "./uploads"
	maxSize := int64(100 * 1024 * 1024) // 100MB
	fileService := NewFileService(uploadDir, maxSize)
	handler := NewFileHandler(fileService)
	
	// Setup routes
	http.HandleFunc("/api/upload", handler.handleUpload)
	http.HandleFunc("/api/files", handler.handleFiles)
	http.HandleFunc("/api/files/", handler.handleFileByID)
	http.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/download") {
			handler.handleDownload(w, r)
		} else {
			handler.handleFileByID(w, r)
		}
	})
	http.HandleFunc("/api/stats", handler.handleStats)
	
	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "healthy",
			"timestamp":  time.Now(),
			"service":    "file-upload-service",
			"version":    "1.0.0",
			"upload_dir": uploadDir,
			"max_size":   maxSize,
		})
	})
	
	// Serve uploaded files statically
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))
	
	// Root endpoint with API documentation
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "NetCore-Go File Upload Service",
			"version": "1.0.0",
			"features": []string{
				"File upload with validation",
				"Multiple file type support",
				"File metadata management",
				"Download tracking",
				"Public/private file access",
				"File tagging system",
				"Checksum verification",
			},
			"endpoints": map[string]string{
				"POST /api/upload":              "Upload a file",
				"GET /api/files":                "Get all files",
				"GET /api/files?public=true":    "Get public files",
				"GET /api/files?user_id=X":      "Get files by user",
				"GET /api/files/{id}":           "Get file info",
				"DELETE /api/files/{id}":        "Delete file",
				"GET /api/files/{id}/download":  "Download file",
				"GET /api/stats":                "Get service statistics",
				"GET /health":                   "Health check",
			},
			"supported_types": []string{
				"Images: JPEG, PNG, GIF, WebP",
				"Documents: PDF, TXT, CSV, JSON",
				"Archives: ZIP",
				"Media: MP4, MP3",
			},
			"limits": map[string]interface{}{
				"max_file_size": fmt.Sprintf("%d MB", maxSize/(1024*1024)),
				"upload_dir":    uploadDir,
			},
		})
	})
	
	port := ":8083"
	fmt.Printf("📁 File Upload Service starting on port %s\n", port)
	fmt.Println("📖 API Documentation: http://localhost:8083")
	fmt.Println("🏥 Health Check: http://localhost:8083/health")
	fmt.Println("📊 Statistics: http://localhost:8083/api/stats")
	fmt.Printf("📂 Upload Directory: %s\n", uploadDir)
	fmt.Printf("📏 Max File Size: %d MB\n", maxSize/(1024*1024))
	
	log.Fatal(http.ListenAndServe(port, nil))
}