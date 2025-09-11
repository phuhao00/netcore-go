// Todo API Example - NetCore-Go
// A simple REST API demonstrating CRUD operations
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/http/server"
	"github.com/netcore-go/pkg/middleware"
	"github.com/netcore-go/pkg/database"
	"gorm.io/gorm"
)

// Todo represents a todo item
type Todo struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Title       string    `json:"title" gorm:"not null" validate:"required,min=1,max=200"`
	Description string    `json:"description" gorm:"type:text"`
	Completed   bool      `json:"completed" gorm:"default:false"`
	Priority    string    `json:"priority" gorm:"default:'medium'" validate:"oneof=low medium high"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TodoRequest represents the request payload for creating/updating todos
type TodoRequest struct {
	Title       string     `json:"title" validate:"required,min=1,max=200"`
	Description string     `json:"description" validate:"max=1000"`
	Completed   bool       `json:"completed"`
	Priority    string     `json:"priority" validate:"oneof=low medium high"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// TodoResponse represents the response format for todos
type TodoResponse struct {
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Completed   bool       `json:"completed"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TodoService handles business logic for todos
type TodoService struct {
	db *gorm.DB
}

// NewTodoService creates a new todo service
func NewTodoService(db *gorm.DB) *TodoService {
	return &TodoService{db: db}
}

// GetAll retrieves all todos with optional filtering
func (s *TodoService) GetAll(completed *bool, priority string, limit, offset int) ([]Todo, int64, error) {
	var todos []Todo
	var total int64

	query := s.db.Model(&Todo{})

	// Apply filters
	if completed != nil {
		query = query.Where("completed = ?", *completed)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&todos).Error; err != nil {
		return nil, 0, err
	}

	return todos, total, nil
}

// GetByID retrieves a todo by ID
func (s *TodoService) GetByID(id uint) (*Todo, error) {
	var todo Todo
	if err := s.db.First(&todo, id).Error; err != nil {
		return nil, err
	}
	return &todo, nil
}

// Create creates a new todo
func (s *TodoService) Create(req *TodoRequest) (*Todo, error) {
	todo := &Todo{
		Title:       req.Title,
		Description: req.Description,
		Completed:   req.Completed,
		Priority:    req.Priority,
		DueDate:     req.DueDate,
	}

	if todo.Priority == "" {
		todo.Priority = "medium"
	}

	if err := s.db.Create(todo).Error; err != nil {
		return nil, err
	}

	return todo, nil
}

// Update updates an existing todo
func (s *TodoService) Update(id uint, req *TodoRequest) (*Todo, error) {
	todo, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	todo.Title = req.Title
	todo.Description = req.Description
	todo.Completed = req.Completed
	todo.Priority = req.Priority
	todo.DueDate = req.DueDate

	if err := s.db.Save(todo).Error; err != nil {
		return nil, err
	}

	return todo, nil
}

// Delete deletes a todo by ID
func (s *TodoService) Delete(id uint) error {
	result := s.db.Delete(&Todo{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ToggleComplete toggles the completed status of a todo
func (s *TodoService) ToggleComplete(id uint) (*Todo, error) {
	todo, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	todo.Completed = !todo.Completed

	if err := s.db.Save(todo).Error; err != nil {
		return nil, err
	}

	return todo, nil
}

// GetStats returns statistics about todos
func (s *TodoService) GetStats() (map[string]interface{}, error) {
	var total, completed, pending int64
	var highPriority, mediumPriority, lowPriority int64

	// Total todos
	if err := s.db.Model(&Todo{}).Count(&total).Error; err != nil {
		return nil, err
	}

	// Completed todos
	if err := s.db.Model(&Todo{}).Where("completed = ?", true).Count(&completed).Error; err != nil {
		return nil, err
	}

	// Pending todos
	pending = total - completed

	// Priority breakdown
	if err := s.db.Model(&Todo{}).Where("priority = ?", "high").Count(&highPriority).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&Todo{}).Where("priority = ?", "medium").Count(&mediumPriority).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&Todo{}).Where("priority = ?", "low").Count(&lowPriority).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":     total,
		"completed": completed,
		"pending":   pending,
		"priority": map[string]int64{
			"high":   highPriority,
			"medium": mediumPriority,
			"low":    lowPriority,
		},
	}, nil
}

// TodoHandler handles HTTP requests for todos
type TodoHandler struct {
	service *TodoService
}

// NewTodoHandler creates a new todo handler
func NewTodoHandler(service *TodoService) *TodoHandler {
	return &TodoHandler{service: service}
}

// GetTodos handles GET /todos
// @Summary Get all todos
// @Description Get all todos with optional filtering and pagination
// @Tags todos
// @Accept json
// @Produce json
// @Param completed query boolean false "Filter by completion status"
// @Param priority query string false "Filter by priority (low, medium, high)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos [get]
func (h *TodoHandler) GetTodos(c *server.Context) error {
	// Parse query parameters
	var completed *bool
	if completedStr := c.QueryParam("completed"); completedStr != "" {
		if completedVal, err := strconv.ParseBool(completedStr); err == nil {
			completed = &completedVal
		}
	}

	priority := c.QueryParam("priority")

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Get todos
	todos, total, err := h.service.GetAll(completed, priority, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve todos",
		})
	}

	// Convert to response format
	response := make([]TodoResponse, len(todos))
	for i, todo := range todos {
		response[i] = TodoResponse{
			ID:          todo.ID,
			Title:       todo.Title,
			Description: todo.Description,
			Completed:   todo.Completed,
			Priority:    todo.Priority,
			DueDate:     todo.DueDate,
			CreatedAt:   todo.CreatedAt,
			UpdatedAt:   todo.UpdatedAt,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"todos": response,
		"pagination": map[string]interface{}{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetTodo handles GET /todos/:id
// @Summary Get a todo by ID
// @Description Get a specific todo by its ID
// @Tags todos
// @Accept json
// @Produce json
// @Param id path int true "Todo ID"
// @Success 200 {object} TodoResponse
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos/{id} [get]
func (h *TodoHandler) GetTodo(c *server.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid todo ID",
		})
	}

	todo, err := h.service.GetByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Todo not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve todo",
		})
	}

	response := TodoResponse{
		ID:          todo.ID,
		Title:       todo.Title,
		Description: todo.Description,
		Completed:   todo.Completed,
		Priority:    todo.Priority,
		DueDate:     todo.DueDate,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}

	return c.JSON(http.StatusOK, response)
}

// CreateTodo handles POST /todos
// @Summary Create a new todo
// @Description Create a new todo item
// @Tags todos
// @Accept json
// @Produce json
// @Param todo body TodoRequest true "Todo data"
// @Success 201 {object} TodoResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos [post]
func (h *TodoHandler) CreateTodo(c *server.Context) error {
	var req TodoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	todo, err := h.service.Create(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create todo",
		})
	}

	response := TodoResponse{
		ID:          todo.ID,
		Title:       todo.Title,
		Description: todo.Description,
		Completed:   todo.Completed,
		Priority:    todo.Priority,
		DueDate:     todo.DueDate,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}

	return c.JSON(http.StatusCreated, response)
}

// UpdateTodo handles PUT /todos/:id
// @Summary Update a todo
// @Description Update an existing todo item
// @Tags todos
// @Accept json
// @Produce json
// @Param id path int true "Todo ID"
// @Param todo body TodoRequest true "Todo data"
// @Success 200 {object} TodoResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos/{id} [put]
func (h *TodoHandler) UpdateTodo(c *server.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid todo ID",
		})
	}

	var req TodoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	todo, err := h.service.Update(uint(id), &req)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Todo not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to update todo",
		})
	}

	response := TodoResponse{
		ID:          todo.ID,
		Title:       todo.Title,
		Description: todo.Description,
		Completed:   todo.Completed,
		Priority:    todo.Priority,
		DueDate:     todo.DueDate,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}

	return c.JSON(http.StatusOK, response)
}

// DeleteTodo handles DELETE /todos/:id
// @Summary Delete a todo
// @Description Delete a todo item by ID
// @Tags todos
// @Accept json
// @Produce json
// @Param id path int true "Todo ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos/{id} [delete]
func (h *TodoHandler) DeleteTodo(c *server.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid todo ID",
		})
	}

	if err := h.service.Delete(uint(id)); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Todo not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to delete todo",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// ToggleTodo handles PATCH /todos/:id/toggle
// @Summary Toggle todo completion
// @Description Toggle the completed status of a todo
// @Tags todos
// @Accept json
// @Produce json
// @Param id path int true "Todo ID"
// @Success 200 {object} TodoResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos/{id}/toggle [patch]
func (h *TodoHandler) ToggleTodo(c *server.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid todo ID",
		})
	}

	todo, err := h.service.ToggleComplete(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Todo not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to toggle todo",
		})
	}

	response := TodoResponse{
		ID:          todo.ID,
		Title:       todo.Title,
		Description: todo.Description,
		Completed:   todo.Completed,
		Priority:    todo.Priority,
		DueDate:     todo.DueDate,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}

	return c.JSON(http.StatusOK, response)
}

// GetStats handles GET /todos/stats
// @Summary Get todo statistics
// @Description Get statistics about todos
// @Tags todos
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /todos/stats [get]
func (h *TodoHandler) GetStats(c *server.Context) error {
	stats, err := h.service.GetStats()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve statistics",
		})
	}

	return c.JSON(http.StatusOK, stats)
}

// @title Todo API
// @version 1.0
// @description A simple Todo API built with NetCore-Go
// @contact.name NetCore-Go Team
// @contact.email support@netcore-go.dev
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Database configuration
	dbConfig := &database.Config{
		Driver:   "sqlite",
		Path:     "todos.db",
		LogLevel: "info",
	}

	// Connect to database
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&Todo{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Create services and handlers
	todoService := NewTodoService(db)
	todoHandler := NewTodoHandler(todoService)

	// Create NetCore-Go application
	app := core.New(&core.Config{
		Name:    "Todo API",
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
	httpServer.Use(middleware.RequestID())

	// Health check endpoint
	httpServer.GET("/health", func(c *server.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"service":   "Todo API",
			"version":   "1.0.0",
		})
	})

	// API routes
	api := httpServer.Group("/api/v1")
	{
		// Todo routes
		api.GET("/todos", todoHandler.GetTodos)
		api.POST("/todos", todoHandler.CreateTodo)
		api.GET("/todos/stats", todoHandler.GetStats)
		api.GET("/todos/:id", todoHandler.GetTodo)
		api.PUT("/todos/:id", todoHandler.UpdateTodo)
		api.DELETE("/todos/:id", todoHandler.DeleteTodo)
		api.PATCH("/todos/:id/toggle", todoHandler.ToggleTodo)
	}

	// Add server to application
	app.AddServer(httpServer)

	// Graceful shutdown
	app.OnShutdown(func(ctx context.Context) error {
		log.Println("Closing database connection...")
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	})

	// Start the application
	log.Println("🚀 Starting Todo API server on http://localhost:8080")
	log.Println("📖 API Documentation: http://localhost:8080/swagger/index.html")
	log.Println("❤️  Health Check: http://localhost:8080/health")

	if err := app.Run(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}