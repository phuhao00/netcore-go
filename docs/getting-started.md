# Getting Started with NetCore-Go

Welcome to NetCore-Go! This guide will help you build your first application using our high-performance Go framework.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Your First Application](#your-first-application)
4. [Project Structure](#project-structure)
5. [Configuration](#configuration)
6. [Routing and Handlers](#routing-and-handlers)
7. [Middleware](#middleware)
8. [Database Integration](#database-integration)
9. [Testing](#testing)
10. [Deployment](#deployment)
11. [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.21 or later** - [Download Go](https://golang.org/dl/)
- **Git** - [Download Git](https://git-scm.com/downloads)
- **Docker** (optional) - [Download Docker](https://www.docker.com/get-started)
- **Kubernetes** (optional) - [Install kubectl](https://kubernetes.io/docs/tasks/tools/)

### Verify Installation

```bash
# Check Go version
go version
# Should output: go version go1.21.x

# Check Git
git --version

# Check Docker (optional)
docker --version
```

## Installation

### Install NetCore-Go CLI

The easiest way to get started is with our CLI tool:

```bash
# Install the CLI tool
go install github.com/netcore-go/netcore-go/cmd/netcore-cli@latest

# Verify installation
netcore-cli --version
```

### Alternative: Manual Installation

If you prefer to set up manually:

```bash
# Create a new directory
mkdir my-netcore-app
cd my-netcore-app

# Initialize Go module
go mod init my-netcore-app

# Add NetCore-Go dependency
go get github.com/netcore-go/netcore-go@latest
```

## Your First Application

### Using the CLI (Recommended)

```bash
# Create a new project with interactive wizard
netcore-cli new my-first-app --interactive

# Or create with default settings
netcore-cli new my-first-app

# Navigate to the project
cd my-first-app

# Install dependencies
go mod tidy

# Start development server
netcore-cli dev
```

The interactive wizard will guide you through:
- Project configuration
- Feature selection
- Database setup
- Authentication options
- Deployment preferences

### Manual Setup

Create a simple `main.go` file:

```go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/netcore-go/netcore-go/pkg/core"
	"github.com/netcore-go/netcore-go/pkg/http/server"
	"github.com/netcore-go/netcore-go/pkg/middleware"
)

func main() {
	// Create application instance
	app := core.New(&core.Config{
		Name:    "my-first-app",
		Version: "1.0.0",
		Debug:   true,
	})

	// Create HTTP server
	httpServer := server.New(&server.Config{
		Port: 8080,
		Host: "localhost",
	})

	// Add middleware
	httpServer.Use(middleware.Logger())
	httpServer.Use(middleware.Recovery())
	httpServer.Use(middleware.CORS())

	// Define routes
	httpServer.GET("/", homeHandler)
	httpServer.GET("/api/health", healthHandler)
	httpServer.GET("/api/users/:id", getUserHandler)
	httpServer.POST("/api/users", createUserHandler)

	// Add server to application
	app.AddServer(httpServer)

	// Start the application
	log.Println("Starting server on http://localhost:8080")
	if err := app.Run(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// Handler functions
func homeHandler(c *server.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Welcome to NetCore-Go!",
		"version": "1.0.0",
		"time":    time.Now(),
	})
}

func healthHandler(c *server.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"uptime": time.Since(startTime).String(),
	})
}

func getUserHandler(c *server.Context) error {
	userID := c.Param("id")
	
	// Simulate user data
	user := map[string]interface{}{
		"id":    userID,
		"name":  "John Doe",
		"email": "john@example.com",
	}
	
	return c.JSON(http.StatusOK, user)
}

func createUserHandler(c *server.Context) error {
	var user map[string]interface{}
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}
	
	// Simulate user creation
	user["id"] = "123"
	user["created_at"] = time.Now()
	
	return c.JSON(http.StatusCreated, user)
}

var startTime = time.Now()
```

### Run Your Application

```bash
# Run the application
go run main.go

# Test the endpoints
curl http://localhost:8080/
curl http://localhost:8080/api/health
curl http://localhost:8080/api/users/123
```

## Project Structure

NetCore-Go follows a standard project structure:

```
my-first-app/
├── cmd/                    # Application entrypoints
│   └── main.go            # Main application file
├── internal/              # Private application code
│   ├── handlers/          # HTTP request handlers
│   │   ├── user.go
│   │   └── auth.go
│   ├── services/          # Business logic
│   │   ├── user_service.go
│   │   └── auth_service.go
│   ├── models/            # Data models
│   │   ├── user.go
│   │   └── auth.go
│   ├── repositories/      # Data access layer
│   │   └── user_repository.go
│   └── middleware/        # Custom middleware
│       └── auth.go
├── pkg/                   # Public library code
├── api/                   # API definitions
│   ├── openapi.yaml
│   └── proto/
├── web/                   # Static assets
│   ├── static/
│   └── templates/
├── configs/               # Configuration files
│   ├── config.yaml
│   ├── config.dev.yaml
│   └── config.prod.yaml
├── scripts/               # Build and deployment scripts
│   ├── build.sh
│   └── deploy.sh
├── docs/                  # Documentation
├── tests/                 # Test files
│   ├── integration/
│   └── e2e/
├── deployments/           # Deployment configurations
│   ├── docker/
│   ├── kubernetes/
│   └── helm/
├── examples/              # Example code
├── go.mod                 # Go module file
├── go.sum                 # Go dependencies
├── Dockerfile             # Docker configuration
├── Makefile              # Build automation
└── README.md             # Project documentation
```

## Configuration

NetCore-Go supports multiple configuration sources:

### YAML Configuration

Create `configs/config.yaml`:

```yaml
server:
  port: 8080
  host: localhost
  read_timeout: 30s
  write_timeout: 30s

database:
  type: postgres
  host: localhost
  port: 5432
  user: myuser
  password: mypassword
  dbname: myapp
  sslmode: disable

cache:
  type: redis
  addr: localhost:6379
  password: ""
  db: 0

auth:
  jwt_secret: your-secret-key
  token_expiry: 24h

logging:
  level: info
  format: json
```

### Environment Variables

```bash
# Server configuration
export SERVER_PORT=8080
export SERVER_HOST=localhost

# Database configuration
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=myuser
export DB_PASSWORD=mypassword
export DB_NAME=myapp

# Authentication
export JWT_SECRET=your-secret-key
```

### Loading Configuration

```go
package main

import (
	"github.com/netcore-go/netcore-go/pkg/config"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	Port int    `yaml:"port" env:"SERVER_PORT"`
	Host string `yaml:"host" env:"SERVER_HOST"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host" env:"DB_HOST"`
	Port     int    `yaml:"port" env:"DB_PORT"`
	User     string `yaml:"user" env:"DB_USER"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	DBName   string `yaml:"dbname" env:"DB_NAME"`
}

type AuthConfig struct {
	JWTSecret   string `yaml:"jwt_secret" env:"JWT_SECRET"`
	TokenExpiry string `yaml:"token_expiry" env:"TOKEN_EXPIRY"`
}

func main() {
	var cfg Config
	
	// Load configuration
	if err := config.Load(&cfg, "configs/config.yaml"); err != nil {
		log.Fatal("Failed to load config:", err)
	}
	
	// Use configuration
	log.Printf("Starting server on %s:%d", cfg.Server.Host, cfg.Server.Port)
}
```

## Routing and Handlers

### Basic Routing

```go
// GET request
httpServer.GET("/users", listUsersHandler)

// POST request
httpServer.POST("/users", createUserHandler)

// PUT request
httpServer.PUT("/users/:id", updateUserHandler)

// DELETE request
httpServer.DELETE("/users/:id", deleteUserHandler)

// Route parameters
httpServer.GET("/users/:id/posts/:postId", getUserPostHandler)

// Query parameters
httpServer.GET("/search", searchHandler)
```

### Route Groups

```go
// API v1 group
v1 := httpServer.Group("/api/v1")
{
	v1.GET("/users", listUsersHandler)
	v1.POST("/users", createUserHandler)
	v1.GET("/users/:id", getUserHandler)
	v1.PUT("/users/:id", updateUserHandler)
	v1.DELETE("/users/:id", deleteUserHandler)
}

// Admin group with middleware
admin := httpServer.Group("/admin")
admin.Use(middleware.Auth())
admin.Use(middleware.RequireRole("admin"))
{
	admin.GET("/stats", adminStatsHandler)
	admin.POST("/users/:id/ban", banUserHandler)
}
```

### Handler Examples

```go
func listUsersHandler(c *server.Context) error {
	// Query parameters
	page := c.QueryParam("page")
	limit := c.QueryParam("limit")
	
	// Default values
	if page == "" {
		page = "1"
	}
	if limit == "" {
		limit = "10"
	}
	
	// Simulate user list
	users := []map[string]interface{}{
		{"id": 1, "name": "John Doe", "email": "john@example.com"},
		{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
	}
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"users": users,
		"page":  page,
		"limit": limit,
	})
}

func createUserHandler(c *server.Context) error {
	var user struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}
	
	// Bind and validate request body
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}
	
	if err := c.Validate(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	
	// Simulate user creation
	createdUser := map[string]interface{}{
		"id":         123,
		"name":       user.Name,
		"email":      user.Email,
		"created_at": time.Now(),
	}
	
	return c.JSON(http.StatusCreated, createdUser)
}

func getUserPostHandler(c *server.Context) error {
	userID := c.Param("id")
	postID := c.Param("postId")
	
	// Simulate post data
	post := map[string]interface{}{
		"id":      postID,
		"user_id": userID,
		"title":   "Sample Post",
		"content": "This is a sample post content.",
	}
	
	return c.JSON(http.StatusOK, post)
}
```

## Middleware

NetCore-Go provides built-in middleware and supports custom middleware:

### Built-in Middleware

```go
import "github.com/netcore-go/netcore-go/pkg/middleware"

// Logger middleware
httpServer.Use(middleware.Logger())

// Recovery middleware
httpServer.Use(middleware.Recovery())

// CORS middleware
httpServer.Use(middleware.CORS())

// Rate limiting
httpServer.Use(middleware.RateLimit(100, time.Minute))

// Authentication
httpServer.Use(middleware.JWT("your-secret-key"))

// Request ID
httpServer.Use(middleware.RequestID())

// Compression
httpServer.Use(middleware.Gzip())
```

### Custom Middleware

```go
// Custom logging middleware
func customLogger() server.MiddlewareFunc {
	return func(next server.HandlerFunc) server.HandlerFunc {
		return func(c *server.Context) error {
			start := time.Now()
			
			// Process request
			err := next(c)
			
			// Log request
			log.Printf("%s %s - %v - %v",
				c.Request().Method,
				c.Request().URL.Path,
				c.Response().Status,
				time.Since(start),
			)
			
			return err
		}
	}
}

// Authentication middleware
func authMiddleware() server.MiddlewareFunc {
	return func(next server.HandlerFunc) server.HandlerFunc {
		return func(c *server.Context) error {
			token := c.Request().Header.Get("Authorization")
			
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Missing authorization token",
				})
			}
			
			// Validate token (simplified)
			if !isValidToken(token) {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Invalid token",
				})
			}
			
			// Set user context
			c.Set("user_id", getUserIDFromToken(token))
			
			return next(c)
		}
	}
}

// Use custom middleware
httpServer.Use(customLogger())
httpServer.Use(authMiddleware())
```

## Database Integration

NetCore-Go supports multiple databases:

### PostgreSQL Example

```go
package main

import (
	"github.com/netcore-go/netcore-go/pkg/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Name      string    `json:"name" gorm:"not null"`
	Email     string    `json:"email" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func main() {
	// Database configuration
	dbConfig := &database.Config{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "myuser",
		Password: "mypassword",
		DBName:   "myapp",
		SSLMode:  "disable",
	}
	
	// Connect to database
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	
	// Auto-migrate models
	db.AutoMigrate(&User{})
	
	// Create HTTP server with database
	httpServer := server.New(&server.Config{Port: 8080})
	
	// Inject database into context
	httpServer.Use(func(next server.HandlerFunc) server.HandlerFunc {
		return func(c *server.Context) error {
			c.Set("db", db)
			return next(c)
		}
	})
	
	// Routes
	httpServer.GET("/users", listUsersHandler)
	httpServer.POST("/users", createUserHandler)
	httpServer.GET("/users/:id", getUserHandler)
	
	// Start server
	app := core.New()
	app.AddServer(httpServer)
	app.Run()
}

func listUsersHandler(c *server.Context) error {
	db := c.Get("db").(*gorm.DB)
	
	var users []User
	if err := db.Find(&users).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch users",
		})
	}
	
	return c.JSON(http.StatusOK, users)
}

func createUserHandler(c *server.Context) error {
	db := c.Get("db").(*gorm.DB)
	
	var user User
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}
	
	if err := db.Create(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create user",
		})
	}
	
	return c.JSON(http.StatusCreated, user)
}

func getUserHandler(c *server.Context) error {
	db := c.Get("db").(*gorm.DB)
	userID := c.Param("id")
	
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch user",
		})
	}
	
	return c.JSON(http.StatusOK, user)
}
```

## Testing

NetCore-Go provides comprehensive testing utilities:

```go
package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/netcore-go/netcore-go/pkg/testing"
	"github.com/stretchr/testify/assert"
)

func TestUserHandlers(t *testing.T) {
	// Create test suite
	suite := testing.NewUnitTestSuite("UserHandlers", "User handler tests")
	
	// Setup test server
	server := setupTestServer()
	
	suite.AddTest(testing.NewUnitTest(
		"ListUsers",
		"Should return list of users",
		func(ctx *testing.TestContext) error {
			// Create request
			req := httptest.NewRequest("GET", "/users", nil)
			rec := httptest.NewRecorder()
			
			// Execute request
			server.ServeHTTP(rec, req)
			
			// Assertions
			ctx.Assertions.Equal(http.StatusOK, rec.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			ctx.Assertions.NoError(err)
			
			ctx.Assertions.Contains(response, "users")
			return nil
		},
	))
	
	suite.AddTest(testing.NewUnitTest(
		"CreateUser",
		"Should create a new user",
		func(ctx *testing.TestContext) error {
			// Prepare request body
			user := map[string]string{
				"name":  "John Doe",
				"email": "john@example.com",
			}
			body, _ := json.Marshal(user)
			
			// Create request
			req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			
			// Execute request
			server.ServeHTTP(rec, req)
			
			// Assertions
			ctx.Assertions.Equal(http.StatusCreated, rec.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			ctx.Assertions.NoError(err)
			
			ctx.Assertions.Equal("John Doe", response["name"])
			ctx.Assertions.Equal("john@example.com", response["email"])
			return nil
		},
	))
	
	// Run tests
	suite.Run(t)
}

func setupTestServer() http.Handler {
	// Create test server with same configuration as main server
	server := server.New(&server.Config{Port: 8080})
	server.GET("/users", listUsersHandler)
	server.POST("/users", createUserHandler)
	return server
}
```

### Integration Tests

```go
func TestUserIntegration(t *testing.T) {
	// Setup test database
	db := setupTestDB()
	defer cleanupTestDB(db)
	
	// Create integration test suite
	suite := testing.NewIntegrationTestSuite("UserIntegration", "User integration tests")
	
	// Add HTTP request step
	test := &testing.IntegrationTest{
		Name: "CreateAndRetrieveUser",
		Steps: []*testing.TestStep{
			testing.NewTestStep("create_user", testing.ActionHTTPRequest).
				WithRequest(testing.NewHTTPRequest("POST", "/users").
					WithBody(map[string]string{
						"name":  "Integration Test User",
						"email": "test@example.com",
					})).
				WithExpectedResponse(&testing.HTTPResponse{
					StatusCode: 201,
				}).
				WithValidation(func(result *testing.StepResult) error {
					response := result.Response.Body.(map[string]interface{})
					if response["id"] == nil {
						return fmt.Errorf("expected user ID in response")
					}
					return nil
				}),
			
			testing.NewTestStep("get_user", testing.ActionHTTPRequest).
				WithRequest(testing.NewHTTPRequest("GET", "/users/1")).
				WithExpectedResponse(&testing.HTTPResponse{
					StatusCode: 200,
				}),
		},
	}
	
	suite.AddTest(test)
	
	// Run integration tests
	result, err := suite.Run()
	assert.NoError(t, err)
	assert.True(t, result.Success)
}
```

## Deployment

### Docker Deployment

```dockerfile
# Multi-stage Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/configs ./configs

EXPOSE 8080
CMD ["./main"]
```

```bash
# Build and run with Docker
docker build -t my-netcore-app .
docker run -p 8080:8080 my-netcore-app
```

### Kubernetes Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-netcore-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-netcore-app
  template:
    metadata:
      labels:
        app: my-netcore-app
    spec:
      containers:
      - name: app
        image: my-netcore-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: SERVER_PORT
          value: "8080"
        - name: DB_HOST
          value: "postgres-service"
        livenessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: my-netcore-app-service
spec:
  selector:
    app: my-netcore-app
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

```bash
# Deploy to Kubernetes
kubectl apply -f deployment.yaml

# Check deployment status
kubectl get pods
kubectl get services
```

## Next Steps

Congratulations! You've built your first NetCore-Go application. Here's what you can explore next:

### Advanced Topics

1. **[Microservices Architecture](microservices.md)** - Build distributed systems
2. **[Authentication & Authorization](authentication.md)** - Secure your APIs
3. **[Caching Strategies](caching.md)** - Improve performance
4. **[Message Queues](messaging.md)** - Async communication
5. **[Monitoring & Observability](monitoring.md)** - Production monitoring

### Best Practices

1. **[Error Handling](best-practices/error-handling.md)** - Robust error management
2. **[API Design](best-practices/api-design.md)** - RESTful API guidelines
3. **[Security](best-practices/security.md)** - Security best practices
4. **[Performance](best-practices/performance.md)** - Optimization techniques
5. **[Testing](best-practices/testing.md)** - Comprehensive testing strategies

### Community Resources

- 📖 [Full Documentation](https://docs.netcore-go.dev)
- 💬 [Discord Community](https://discord.gg/netcore-go)
- 🐛 [GitHub Issues](https://github.com/netcore-go/netcore-go/issues)
- 📧 [Mailing List](https://groups.google.com/g/netcore-go)
- 🎥 [Video Tutorials](https://youtube.com/netcore-go)

### Example Projects

- [**Todo API**](../examples/todo-api/) - Simple REST API
- [**Blog Platform**](../examples/blog-platform/) - Full-stack application
- [**E-commerce Backend**](../examples/ecommerce/) - Microservices architecture
- [**Chat Application**](../examples/chat-app/) - WebSocket real-time app
- [**File Upload Service**](../examples/file-upload/) - File handling service

Happy coding with NetCore-Go! 🚀