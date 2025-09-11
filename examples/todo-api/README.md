# Todo API Example

A comprehensive REST API example built with NetCore-Go demonstrating CRUD operations, database integration, validation, and API documentation.

## Features

- ✅ **CRUD Operations** - Create, Read, Update, Delete todos
- 🗄️ **Database Integration** - SQLite with GORM ORM
- ✔️ **Data Validation** - Request validation with custom rules
- 📊 **Statistics** - Todo statistics and analytics
- 🔄 **Toggle Completion** - Quick toggle for todo completion status
- 📖 **API Documentation** - Auto-generated Swagger/OpenAPI docs
- 🏥 **Health Checks** - Health monitoring endpoints
- 🔍 **Filtering & Pagination** - Advanced query capabilities
- 🏷️ **Priority Levels** - Low, Medium, High priority support
- 📅 **Due Dates** - Optional due date tracking
- 🚀 **Production Ready** - Proper error handling and logging

## Quick Start

### Prerequisites

- Go 1.21 or later
- NetCore-Go CLI (optional)

### Installation

```bash
# Clone the repository
git clone https://github.com/netcore-go/netcore-go.git
cd netcore-go/examples/todo-api

# Install dependencies
go mod tidy

# Run the application
go run main.go
```

### Using NetCore-Go CLI

```bash
# Create a new project based on this example
netcore-cli new my-todo-api --template=todo-api

# Start development server
netcore-cli dev
```

## API Endpoints

### Base URL
```
http://localhost:8080/api/v1
```

### Health Check
```http
GET /health
```

### Todo Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/todos` | Get all todos with filtering and pagination |
| POST | `/todos` | Create a new todo |
| GET | `/todos/{id}` | Get a specific todo by ID |
| PUT | `/todos/{id}` | Update a todo |
| DELETE | `/todos/{id}` | Delete a todo |
| PATCH | `/todos/{id}/toggle` | Toggle todo completion status |
| GET | `/todos/stats` | Get todo statistics |

## API Usage Examples

### 1. Create a Todo

```bash
curl -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Learn NetCore-Go",
    "description": "Complete the NetCore-Go tutorial and build a sample application",
    "priority": "high",
    "due_date": "2024-12-31T23:59:59Z"
  }'
```

**Response:**
```json
{
  "id": 1,
  "title": "Learn NetCore-Go",
  "description": "Complete the NetCore-Go tutorial and build a sample application",
  "completed": false,
  "priority": "high",
  "due_date": "2024-12-31T23:59:59Z",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### 2. Get All Todos

```bash
# Get all todos
curl http://localhost:8080/api/v1/todos

# Get completed todos only
curl "http://localhost:8080/api/v1/todos?completed=true"

# Get high priority todos
curl "http://localhost:8080/api/v1/todos?priority=high"

# Get todos with pagination
curl "http://localhost:8080/api/v1/todos?page=1&limit=5"

# Combine filters
curl "http://localhost:8080/api/v1/todos?completed=false&priority=high&page=1&limit=10"
```

**Response:**
```json
{
  "todos": [
    {
      "id": 1,
      "title": "Learn NetCore-Go",
      "description": "Complete the NetCore-Go tutorial and build a sample application",
      "completed": false,
      "priority": "high",
      "due_date": "2024-12-31T23:59:59Z",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 1,
    "total_pages": 1
  }
}
```

### 3. Get a Specific Todo

```bash
curl http://localhost:8080/api/v1/todos/1
```

### 4. Update a Todo

```bash
curl -X PUT http://localhost:8080/api/v1/todos/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Master NetCore-Go",
    "description": "Complete the NetCore-Go tutorial, build a sample application, and contribute to the project",
    "completed": true,
    "priority": "high",
    "due_date": "2024-12-31T23:59:59Z"
  }'
```

### 5. Toggle Todo Completion

```bash
curl -X PATCH http://localhost:8080/api/v1/todos/1/toggle
```

### 6. Delete a Todo

```bash
curl -X DELETE http://localhost:8080/api/v1/todos/1
```

### 7. Get Statistics

```bash
curl http://localhost:8080/api/v1/todos/stats
```

**Response:**
```json
{
  "total": 10,
  "completed": 6,
  "pending": 4,
  "priority": {
    "high": 3,
    "medium": 5,
    "low": 2
  }
}
```

## Data Models

### Todo Model

```go
type Todo struct {
    ID          uint       `json:"id"`
    Title       string     `json:"title"`       // Required, 1-200 characters
    Description string     `json:"description"` // Optional, max 1000 characters
    Completed   bool       `json:"completed"`   // Default: false
    Priority    string     `json:"priority"`    // low, medium, high (default: medium)
    DueDate     *time.Time `json:"due_date"`    // Optional
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

### Validation Rules

- **Title**: Required, minimum 1 character, maximum 200 characters
- **Description**: Optional, maximum 1000 characters
- **Priority**: Must be one of: `low`, `medium`, `high`
- **Due Date**: Optional, must be a valid ISO 8601 datetime

## Query Parameters

### GET /todos

| Parameter | Type | Description | Example |
|-----------|------|-------------|----------|
| `completed` | boolean | Filter by completion status | `?completed=true` |
| `priority` | string | Filter by priority (low, medium, high) | `?priority=high` |
| `page` | integer | Page number (default: 1) | `?page=2` |
| `limit` | integer | Items per page (default: 10, max: 100) | `?limit=20` |

## Error Handling

The API returns consistent error responses:

```json
{
  "error": "Error message describing what went wrong"
}
```

### HTTP Status Codes

- `200 OK` - Successful GET, PUT, PATCH requests
- `201 Created` - Successful POST requests
- `204 No Content` - Successful DELETE requests
- `400 Bad Request` - Invalid request data or parameters
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server-side errors

## Database Schema

The application uses SQLite with the following schema:

```sql
CREATE TABLE todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT,
    completed BOOLEAN DEFAULT FALSE,
    priority TEXT DEFAULT 'medium',
    due_date DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Configuration

The application can be configured through environment variables:

```bash
# Server configuration
export SERVER_PORT=8080
export SERVER_HOST=localhost

# Database configuration
export DB_PATH=todos.db
export DB_LOG_LEVEL=info

# Application configuration
export APP_DEBUG=true
export APP_NAME="Todo API"
export APP_VERSION="1.0.0"
```

## Development

### Project Structure

```
todo-api/
├── main.go              # Main application file
├── go.mod              # Go module dependencies
├── go.sum              # Go module checksums
├── README.md           # This file
├── todos.db            # SQLite database (created automatically)
└── docs/               # API documentation (auto-generated)
```

### Running Tests

```bash
# Run unit tests
go test -v

# Run tests with coverage
go test -v -cover

# Run integration tests
go test -v -tags=integration
```

### Building

```bash
# Build for current platform
go build -o todo-api main.go

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o todo-api-linux main.go

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o todo-api.exe main.go
```

### Docker

```bash
# Build Docker image
docker build -t todo-api .

# Run with Docker
docker run -p 8080:8080 todo-api

# Run with volume for persistent data
docker run -p 8080:8080 -v $(pwd)/data:/app/data todo-api
```

## API Documentation

The API includes auto-generated Swagger/OpenAPI documentation:

- **Swagger UI**: http://localhost:8080/swagger/index.html
- **OpenAPI JSON**: http://localhost:8080/swagger/doc.json
- **OpenAPI YAML**: http://localhost:8080/swagger/doc.yaml

## Performance

This example demonstrates:

- **Database Connection Pooling** - Efficient database connections
- **Request Validation** - Early validation to prevent invalid data
- **Pagination** - Efficient handling of large datasets
- **Proper Indexing** - Database indexes for optimal query performance
- **Graceful Shutdown** - Clean application shutdown

## Security Features

- **Input Validation** - All inputs are validated
- **SQL Injection Prevention** - Using GORM ORM with parameterized queries
- **CORS Support** - Cross-origin resource sharing configuration
- **Request ID Tracking** - Unique request IDs for debugging
- **Error Handling** - Secure error messages without sensitive information

## Extending the Example

This example can be extended with:

1. **Authentication** - Add JWT or OAuth2 authentication
2. **User Management** - Multi-user support with user-specific todos
3. **Categories/Tags** - Organize todos with categories or tags
4. **File Attachments** - Add file upload capabilities
5. **Real-time Updates** - WebSocket support for live updates
6. **Email Notifications** - Due date reminders
7. **Search** - Full-text search capabilities
8. **Audit Logging** - Track changes to todos
9. **Rate Limiting** - API rate limiting
10. **Caching** - Redis caching for improved performance

## Related Examples

- [**Blog Platform**](../blog-platform/) - Full-stack application with authentication
- [**E-commerce Backend**](../ecommerce/) - Microservices architecture
- [**Chat Application**](../chat-app/) - WebSocket real-time communication
- [**File Upload Service**](../file-upload/) - File handling and storage

## Support

For questions and support:

- 📖 [NetCore-Go Documentation](https://docs.netcore-go.dev)
- 💬 [Discord Community](https://discord.gg/netcore-go)
- 🐛 [GitHub Issues](https://github.com/netcore-go/netcore-go/issues)
- 📧 [Email Support](mailto:support@netcore-go.dev)

## License

This example is part of the NetCore-Go project and is released under the [MIT License](../../LICENSE).