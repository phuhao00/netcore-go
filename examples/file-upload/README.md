# File Upload Service Example

A comprehensive file upload and management service built with NetCore-Go, demonstrating secure file handling, metadata management, and RESTful API design.

## Features

### Core Functionality
- **Multi-format File Upload**: Support for images, documents, archives, and media files
- **File Validation**: MIME type checking, size limits, and security validation
- **Metadata Management**: Automatic extraction and storage of file metadata
- **Checksum Verification**: MD5 hash generation for file integrity
- **Download Tracking**: Monitor file access and usage statistics
- **Public/Private Access**: Flexible file visibility controls
- **Tagging System**: Organize files with custom tags
- **RESTful API**: Clean, intuitive API endpoints

### Security Features
- File type validation and whitelist
- Size limit enforcement
- Secure file storage with unique naming
- Input sanitization and validation

## Quick Start

### Prerequisites
- Go 1.19 or higher
- NetCore-Go framework

### Installation

1. Navigate to the example directory:
```bash
cd examples/file-upload
```

2. Install dependencies:
```bash
go mod tidy
```

3. Run the service:
```bash
go run main.go
```

4. The service will start on `http://localhost:8083`

## API Endpoints

### File Upload
```http
POST /api/upload
Content-Type: multipart/form-data

Parameters:
- file: The file to upload (required)
- uploaded_by: User identifier (optional, defaults to "anonymous")
- public: Set to "true" for public access (optional, defaults to false)
- tags: Comma-separated list of tags (optional)
```

**Example using curl:**
```bash
curl -X POST http://localhost:8083/api/upload \
  -F "file=@example.jpg" \
  -F "uploaded_by=user123" \
  -F "public=true" \
  -F "tags=image,profile,avatar"
```

### Get Files
```http
GET /api/files                    # Get all files
GET /api/files?public=true        # Get only public files
GET /api/files?user_id=user123    # Get files by specific user
```

### File Operations
```http
GET /api/files/{id}               # Get file metadata
GET /api/files/{id}/download      # Download file
DELETE /api/files/{id}            # Delete file
```

### Service Information
```http
GET /api/stats                    # Get service statistics
GET /health                       # Health check
GET /                            # API documentation
```

## Data Models

### FileInfo Structure
```go
type FileInfo struct {
    ID           string                 `json:"id"`
    OriginalName string                 `json:"original_name"`
    FileName     string                 `json:"file_name"`
    FilePath     string                 `json:"file_path"`
    FileSize     int64                  `json:"file_size"`
    MimeType     string                 `json:"mime_type"`
    Checksum     string                 `json:"checksum"`
    UploadedBy   string                 `json:"uploaded_by"`
    UploadedAt   time.Time              `json:"uploaded_at"`
    Downloads    int                    `json:"downloads"`
    Public       bool                   `json:"public"`
    ExpiresAt    *time.Time             `json:"expires_at,omitempty"`
    Tags         []string               `json:"tags"`
    Metadata     map[string]interface{} `json:"metadata"`
}
```

## Supported File Types

### Images
- JPEG (image/jpeg)
- PNG (image/png)
- GIF (image/gif)
- WebP (image/webp)

### Documents
- Plain Text (text/plain)
- CSV (text/csv)
- PDF (application/pdf)
- JSON (application/json)

### Archives
- ZIP (application/zip)

### Media
- MP4 Video (video/mp4)
- MP3 Audio (audio/mpeg)

## Configuration

### Default Settings
- **Upload Directory**: `./uploads`
- **Maximum File Size**: 100 MB
- **Server Port**: 8083

### Customization
Modify the constants in `main()` function:
```go
uploadDir := "./uploads"              // Change upload directory
maxSize := int64(100 * 1024 * 1024)  // Change max file size (bytes)
port := ":8083"                       // Change server port
```

## Architecture

### Components
1. **FileService**: Core business logic for file operations
2. **FileHandler**: HTTP request handlers and routing
3. **FileInfo**: Data model for file metadata
4. **Validation Layer**: File type and size validation
5. **Storage Layer**: Physical file storage management

### Design Patterns
- **Repository Pattern**: FileService acts as a repository for file operations
- **Handler Pattern**: Separate HTTP handlers for different endpoints
- **Middleware Pattern**: Validation and security checks
- **Factory Pattern**: Service initialization and configuration

## Usage Examples

### Upload a Profile Image
```bash
curl -X POST http://localhost:8083/api/upload \
  -F "file=@profile.jpg" \
  -F "uploaded_by=john_doe" \
  -F "public=true" \
  -F "tags=profile,avatar,user"
```

### Upload a Private Document
```bash
curl -X POST http://localhost:8083/api/upload \
  -F "file=@document.pdf" \
  -F "uploaded_by=jane_smith" \
  -F "public=false" \
  -F "tags=document,private,report"
```

### Get User's Files
```bash
curl "http://localhost:8083/api/files?user_id=john_doe"
```

### Download a File
```bash
curl -O "http://localhost:8083/api/files/123e4567-e89b-12d3-a456-426614174000/download"
```

### Get Service Statistics
```bash
curl "http://localhost:8083/api/stats"
```

## Production Considerations

### Security Enhancements
- Implement authentication and authorization
- Add rate limiting for uploads
- Scan files for malware
- Use HTTPS in production
- Implement CORS policies
- Add request logging and monitoring

### Performance Optimizations
- Implement file chunking for large uploads
- Add caching layer for metadata
- Use CDN for file delivery
- Implement background processing for file operations
- Add database persistence instead of in-memory storage

### Scalability Improvements
- Use cloud storage (AWS S3, Google Cloud Storage)
- Implement horizontal scaling
- Add load balancing
- Use message queues for async processing
- Implement file deduplication

### Monitoring and Logging
- Add structured logging
- Implement metrics collection
- Set up health checks
- Monitor disk usage
- Track upload/download patterns

## Testing

### Manual Testing
1. Start the service: `go run main.go`
2. Open `http://localhost:8083` for API documentation
3. Use curl commands or Postman to test endpoints
4. Check the `./uploads` directory for uploaded files

### Test Scenarios
- Upload various file types
- Test file size limits
- Verify public/private access
- Test file deletion
- Check download tracking
- Validate metadata extraction

## Troubleshooting

### Common Issues

**Upload fails with "file too large"**
- Check the `maxSize` configuration
- Verify client is sending correct Content-Length

**File type not allowed**
- Check the `allowedTypes` map in FileService
- Verify MIME type detection is working correctly

**Files not found after upload**
- Check upload directory permissions
- Verify disk space availability
- Check file path generation logic

**High memory usage**
- Consider implementing streaming uploads
- Add file cleanup for temporary files
- Monitor goroutine leaks

## Extensions

### Possible Enhancements
1. **Image Processing**: Thumbnail generation, resizing, format conversion
2. **File Versioning**: Track file versions and changes
3. **Batch Operations**: Upload/download multiple files
4. **File Sharing**: Generate shareable links with expiration
5. **Compression**: Automatic file compression for storage efficiency
6. **Backup Integration**: Automatic backup to cloud storage
7. **File Preview**: Generate previews for documents and images
8. **Search Functionality**: Full-text search in uploaded documents

## License

This example is part of the NetCore-Go project and follows the same license terms.