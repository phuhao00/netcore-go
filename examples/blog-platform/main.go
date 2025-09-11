// Blog Platform Example - NetCore-Go
// A comprehensive blog platform demonstrating advanced features
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Blog represents a blog post
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

// Comment represents a blog comment
type Comment struct {
	ID        string    `json:"id"`
	BlogID    string    `json:"blog_id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// BlogService handles blog operations
type BlogService struct {
	blogs    map[string]*Blog
	comments map[string][]*Comment
}

// NewBlogService creates a new blog service
func NewBlogService() *BlogService {
	service := &BlogService{
		blogs:    make(map[string]*Blog),
		comments: make(map[string][]*Comment),
	}
	
	// Add sample data
	service.addSampleData()
	return service
}

func (bs *BlogService) addSampleData() {
	// Sample blog posts
	blogs := []*Blog{
		{
			ID:        uuid.New().String(),
			Title:     "Getting Started with NetCore-Go",
			Content:   "NetCore-Go is a high-performance Go framework for building modern web applications...",
			Author:    "NetCore Team",
			Tags:      []string{"go", "framework", "tutorial"},
			Published: true,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
			ViewCount: 150,
			LikeCount: 25,
		},
		{
			ID:        uuid.New().String(),
			Title:     "Building Microservices with NetCore-Go",
			Content:   "Learn how to build scalable microservices using NetCore-Go's powerful features...",
			Author:    "Jane Developer",
			Tags:      []string{"microservices", "architecture", "scalability"},
			Published: true,
			CreatedAt: time.Now().Add(-48 * time.Hour),
			UpdatedAt: time.Now().Add(-48 * time.Hour),
			ViewCount: 89,
			LikeCount: 12,
		},
		{
			ID:        uuid.New().String(),
			Title:     "Performance Optimization Tips",
			Content:   "Discover advanced techniques to optimize your NetCore-Go applications...",
			Author:    "Performance Expert",
			Tags:      []string{"performance", "optimization", "tips"},
			Published: false,
			CreatedAt: time.Now().Add(-12 * time.Hour),
			UpdatedAt: time.Now().Add(-12 * time.Hour),
			ViewCount: 0,
			LikeCount: 0,
		},
	}
	
	for _, blog := range blogs {
		bs.blogs[blog.ID] = blog
	}
}

// GetAllBlogs returns all published blogs
func (bs *BlogService) GetAllBlogs() []*Blog {
	var blogs []*Blog
	for _, blog := range bs.blogs {
		if blog.Published {
			blogs = append(blogs, blog)
		}
	}
	return blogs
}

// GetBlogByID returns a blog by ID
func (bs *BlogService) GetBlogByID(id string) (*Blog, bool) {
	blog, exists := bs.blogs[id]
	if exists && blog.Published {
		// Increment view count
		blog.ViewCount++
		return blog, true
	}
	return nil, false
}

// CreateBlog creates a new blog post
func (bs *BlogService) CreateBlog(blog *Blog) *Blog {
	blog.ID = uuid.New().String()
	blog.CreatedAt = time.Now()
	blog.UpdatedAt = time.Now()
	bs.blogs[blog.ID] = blog
	return blog
}

// UpdateBlog updates an existing blog post
func (bs *BlogService) UpdateBlog(id string, updates *Blog) (*Blog, bool) {
	blog, exists := bs.blogs[id]
	if !exists {
		return nil, false
	}
	
	if updates.Title != "" {
		blog.Title = updates.Title
	}
	if updates.Content != "" {
		blog.Content = updates.Content
	}
	if updates.Tags != nil {
		blog.Tags = updates.Tags
	}
	blog.Published = updates.Published
	blog.UpdatedAt = time.Now()
	
	return blog, true
}

// DeleteBlog deletes a blog post
func (bs *BlogService) DeleteBlog(id string) bool {
	_, exists := bs.blogs[id]
	if exists {
		delete(bs.blogs, id)
		delete(bs.comments, id)
		return true
	}
	return false
}

// AddComment adds a comment to a blog post
func (bs *BlogService) AddComment(blogID string, comment *Comment) *Comment {
	comment.ID = uuid.New().String()
	comment.BlogID = blogID
	comment.CreatedAt = time.Now()
	
	bs.comments[blogID] = append(bs.comments[blogID], comment)
	return comment
}

// GetComments returns comments for a blog post
func (bs *BlogService) GetComments(blogID string) []*Comment {
	return bs.comments[blogID]
}

// HTTP Handlers
type BlogHandler struct {
	service *BlogService
}

func NewBlogHandler(service *BlogService) *BlogHandler {
	return &BlogHandler{service: service}
}

func (bh *BlogHandler) handleBlogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case http.MethodGet:
		blogs := bh.service.GetAllBlogs()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"blogs": blogs,
			"total": len(blogs),
		})
		
	case http.MethodPost:
		var blog Blog
		if err := json.NewDecoder(r.Body).Decode(&blog); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		createdBlog := bh.service.CreateBlog(&blog)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createdBlog)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (bh *BlogHandler) handleBlogByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Extract ID from URL path
	path := r.URL.Path
	id := path[len("/api/blogs/"):]
	
	switch r.Method {
	case http.MethodGet:
		blog, exists := bh.service.GetBlogByID(id)
		if !exists {
			http.Error(w, "Blog not found", http.StatusNotFound)
			return
		}
		
		comments := bh.service.GetComments(id)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"blog":     blog,
			"comments": comments,
		})
		
	case http.MethodPut:
		var updates Blog
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		blog, exists := bh.service.UpdateBlog(id, &updates)
		if !exists {
			http.Error(w, "Blog not found", http.StatusNotFound)
			return
		}
		
		json.NewEncoder(w).Encode(blog)
		
	case http.MethodDelete:
		if !bh.service.DeleteBlog(id) {
			http.Error(w, "Blog not found", http.StatusNotFound)
			return
		}
		
		w.WriteHeader(http.StatusNoContent)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (bh *BlogHandler) handleComments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var comment Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	createdComment := bh.service.AddComment(comment.BlogID, &comment)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdComment)
}

func (bh *BlogHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	blogs := bh.service.GetAllBlogs()
	totalViews := 0
	totalLikes := 0
	totalComments := 0
	
	for _, blog := range blogs {
		totalViews += blog.ViewCount
		totalLikes += blog.LikeCount
		totalComments += len(bh.service.GetComments(blog.ID))
	}
	
	stats := map[string]interface{}{
		"total_blogs":    len(blogs),
		"total_views":    totalViews,
		"total_likes":    totalLikes,
		"total_comments": totalComments,
		"generated_at":   time.Now(),
	}
	
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Initialize services
	blogService := NewBlogService()
	blogHandler := NewBlogHandler(blogService)
	
	// Setup routes
	http.HandleFunc("/api/blogs", blogHandler.handleBlogs)
	http.HandleFunc("/api/blogs/", blogHandler.handleBlogByID)
	http.HandleFunc("/api/comments", blogHandler.handleComments)
	http.HandleFunc("/api/stats", blogHandler.handleStats)
	
	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"service":   "blog-platform",
		})
	})
	
	// Serve static files (if any)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	
	// Root endpoint with API documentation
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "NetCore-Go Blog Platform",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"GET /api/blogs":        "Get all blogs",
				"POST /api/blogs":       "Create a new blog",
				"GET /api/blogs/{id}":   "Get blog by ID",
				"PUT /api/blogs/{id}":   "Update blog by ID",
				"DELETE /api/blogs/{id}": "Delete blog by ID",
				"POST /api/comments":    "Add a comment",
				"GET /api/stats":        "Get platform statistics",
				"GET /health":           "Health check",
			},
		})
	})
	
	port := ":8080"
	fmt.Printf("🚀 Blog Platform starting on port %s\n", port)
	fmt.Println("📖 API Documentation: http://localhost:8080")
	fmt.Println("🏥 Health Check: http://localhost:8080/health")
	fmt.Println("📊 Statistics: http://localhost:8080/api/stats")
	
	log.Fatal(http.ListenAndServe(port, nil))
}