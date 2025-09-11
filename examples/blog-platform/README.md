# Blog Platform Example

A comprehensive blog platform built with NetCore-Go demonstrating advanced web application features including CRUD operations, content management, and RESTful API design.

## Features

- ✅ **Blog Management** - Create, read, update, delete blog posts
- 📝 **Content Editor** - Rich content management with tags and categories
- 💬 **Comment System** - User comments and engagement features
- 📊 **Analytics** - View counts, likes, and engagement statistics
- 🔍 **Search & Filter** - Find blogs by tags, author, or content
- 👤 **Author Management** - Multi-author support
- 📱 **RESTful API** - Clean API design for frontend integration
- 🏥 **Health Monitoring** - Built-in health checks and status endpoints

## API Endpoints

### Blog Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/blogs` | Get all published blogs |
| POST | `/api/blogs` | Create a new blog post |
| GET | `/api/blogs/{id}` | Get specific blog with comments |
| PUT | `/api/blogs/{id}` | Update existing blog post |
| DELETE | `/api/blogs/{id}` | Delete blog post |

### Comment Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/comments` | Add comment to blog post |

### Analytics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/stats` | Get platform statistics |
| GET | `/health` | Health check endpoint |

## Quick Start

### 1. Run the Server

```bash
cd examples/blog-platform
go run main.go
```

The server will start on `http://localhost:8080`

### 2. Explore the API

Visit `http://localhost:8080` to see the API documentation.

### 3. Create a Blog Post

```bash
curl -X POST http://localhost:8080/api/blogs \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My First Blog Post",
    "content": "This is the content of my first blog post using NetCore-Go!",
    "author": "John Doe",
    "tags": ["tutorial", "getting-started"],
    "published": true
  }'
```

### 4. Get All Blogs

```bash
curl http://localhost:8080/api/blogs
```

**Response:**
```json
{
  "blogs": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "title": "Getting Started with NetCore-Go",
      "content": "NetCore-Go is a high-performance Go framework...",
      "author": "NetCore Team",
      "tags": ["go", "framework", "tutorial"],
      "published": true,
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z",
      "view_count": 150,
      "like_count": 25
    }
  ],
  "total": 1
}
```

### 5. Get Specific Blog with Comments

```bash
curl http://localhost:8080/api/blogs/{blog-id}
```

### 6. Add a Comment

```bash
curl -X POST http://localhost:8080/api/comments \
  -H "Content-Type: application/json" \
  -d '{
    "blog_id": "123e4567-e89b-12d3-a456-426614174000",
    "author": "Jane Reader",
    "content": "Great article! Very helpful for beginners."
  }'
```

### 7. Get Platform Statistics

```bash
curl http://localhost:8080/api/stats
```

**Response:**
```json
{
  "total_blogs": 3,
  "total_views": 239,
  "total_likes": 37,
  "total_comments": 8,
  "generated_at": "2024-01-15T15:30:00Z"
}
```

## Data Models

### Blog Post

```go
type Blog struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Content     string    `json:"content"`
    Author      string    `json:"author"`
    Tags        []string  `json:"tags"`
    Published   bool      `json:"published"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    ViewCount   int       `json:"view_count"`
    LikeCount   int       `json:"like_count"`
}
```

### Comment

```go
type Comment struct {
    ID        string    `json:"id"`
    BlogID    string    `json:"blog_id"`
    Author    string    `json:"author"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}
```

## Architecture

### Service Layer

- **BlogService**: Handles all blog-related business logic
- **In-Memory Storage**: Simple map-based storage for demonstration
- **UUID Generation**: Unique identifiers for all entities

### HTTP Layer

- **BlogHandler**: HTTP request handlers for blog operations
- **JSON API**: RESTful JSON API with proper HTTP status codes
- **Error Handling**: Comprehensive error responses

### Features Demonstrated

1. **RESTful API Design**: Proper HTTP methods and status codes
2. **JSON Serialization**: Automatic JSON encoding/decoding
3. **URL Routing**: Path-based routing for different endpoints
4. **Data Validation**: Input validation and error handling
5. **Business Logic**: Separation of concerns with service layer
6. **Statistics**: Real-time analytics and metrics
7. **Health Checks**: Monitoring and status endpoints

## Extending the Example

### Add Database Integration

```go
// Replace in-memory storage with database
type BlogService struct {
    db *sql.DB
}
```

### Add Authentication

```go
// Add JWT middleware
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Validate JWT token
        next(w, r)
    }
}
```

### Add Pagination

```go
// Add pagination parameters
func (bs *BlogService) GetBlogs(page, limit int) ([]*Blog, int) {
    // Implement pagination logic
}
```

### Add Search

```go
// Add search functionality
func (bs *BlogService) SearchBlogs(query string) []*Blog {
    // Implement search logic
}
```

## Production Considerations

1. **Database**: Replace in-memory storage with PostgreSQL/MySQL
2. **Authentication**: Add JWT-based authentication
3. **Validation**: Add comprehensive input validation
4. **Caching**: Implement Redis caching for popular posts
5. **Rate Limiting**: Add rate limiting for API endpoints
6. **Logging**: Add structured logging with levels
7. **Monitoring**: Add metrics and observability
8. **Security**: Add CORS, CSRF protection, and input sanitization

## Testing

```bash
# Run the server
go run main.go

# Test health endpoint
curl http://localhost:8080/health

# Test blog creation and retrieval
curl -X POST http://localhost:8080/api/blogs -H "Content-Type: application/json" -d '{"title":"Test","content":"Test content","author":"Tester","published":true}'

# Get statistics
curl http://localhost:8080/api/stats
```

## Next Steps

- Explore the [E-commerce Backend](../ecommerce/) example for microservices architecture
- Check out the [Chat Application](../chat-app/) for WebSocket real-time features
- Try the [File Upload Service](../file-upload/) for file handling capabilities

Happy blogging with NetCore-Go! 📝✨