// Chat Application Example - NetCore-Go
// Real-time chat application with WebSocket support
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // text, image, file, system
	Timestamp time.Time `json:"timestamp"`
}

// Room represents a chat room
type Room struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	CreatedBy   string             `json:"created_by"`
	CreatedAt   time.Time          `json:"created_at"`
	Clients     map[string]*Client `json:"-"`
	Messages    []*Message         `json:"messages"`
	mu          sync.RWMutex       `json:"-"`
}

// User represents a chat user
type User struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Avatar   string    `json:"avatar"`
	Status   string    `json:"status"` // online, offline, away
	JoinedAt time.Time `json:"joined_at"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID       string
	User     *User
	Room     *Room
	Conn     WebSocketConnection
	Send     chan *Message
	mu       sync.Mutex
}

// WebSocketConnection interface for mock WebSocket implementation
type WebSocketConnection interface {
	WriteJSON(v interface{}) error
	ReadJSON(v interface{}) error
	Close() error
}

// MockWebSocketConnection for demonstration purposes
type MockWebSocketConnection struct {
	closed bool
	mu     sync.Mutex
}

func (m *MockWebSocketConnection) WriteJSON(v interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("connection closed")
	}
	// In a real implementation, this would write to WebSocket
	log.Printf("WebSocket Write: %+v", v)
	return nil
}

func (m *MockWebSocketConnection) ReadJSON(v interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("connection closed")
	}
	// In a real implementation, this would read from WebSocket
	return nil
}

func (m *MockWebSocketConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// ChatHub manages all chat rooms and clients
type ChatHub struct {
	rooms   map[string]*Room
	users   map[string]*User
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewChatHub creates a new chat hub
func NewChatHub() *ChatHub {
	ctx, cancel := context.WithCancel(context.Background())
	hub := &ChatHub{
		rooms:  make(map[string]*Room),
		users:  make(map[string]*User),
		ctx:    ctx,
		cancel: cancel,
	}
	
	// Create default rooms
	hub.createDefaultRooms()
	return hub
}

func (h *ChatHub) createDefaultRooms() {
	defaultRooms := []*Room{
		{
			ID:          uuid.New().String(),
			Name:        "General",
			Description: "General discussion room",
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			Clients:     make(map[string]*Client),
			Messages:    []*Message{},
		},
		{
			ID:          uuid.New().String(),
			Name:        "Tech Talk",
			Description: "Technology discussions and programming",
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			Clients:     make(map[string]*Client),
			Messages:    []*Message{},
		},
		{
			ID:          uuid.New().String(),
			Name:        "Random",
			Description: "Random conversations and fun",
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			Clients:     make(map[string]*Client),
			Messages:    []*Message{},
		},
	}
	
	for _, room := range defaultRooms {
		h.rooms[room.ID] = room
	}
}

// CreateRoom creates a new chat room
func (h *ChatHub) CreateRoom(name, description, createdBy string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	room := &Room{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		Clients:     make(map[string]*Client),
		Messages:    []*Message{},
	}
	
	h.rooms[room.ID] = room
	return room
}

// GetRoom returns a room by ID
func (h *ChatHub) GetRoom(roomID string) (*Room, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	room, exists := h.rooms[roomID]
	return room, exists
}

// GetAllRooms returns all available rooms
func (h *ChatHub) GetAllRooms() []*Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	var rooms []*Room
	for _, room := range h.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// CreateUser creates a new user
func (h *ChatHub) CreateUser(username, email string) *User {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	user := &User{
		ID:       uuid.New().String(),
		Username: username,
		Email:    email,
		Avatar:   fmt.Sprintf("/avatars/%s.png", username),
		Status:   "online",
		JoinedAt: time.Now(),
	}
	
	h.users[user.ID] = user
	return user
}

// GetUser returns a user by ID
func (h *ChatHub) GetUser(userID string) (*User, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	user, exists := h.users[userID]
	return user, exists
}

// JoinRoom adds a client to a room
func (h *ChatHub) JoinRoom(client *Client, roomID string) error {
	room, exists := h.GetRoom(roomID)
	if !exists {
		return fmt.Errorf("room not found")
	}
	
	room.mu.Lock()
	defer room.mu.Unlock()
	
	client.Room = room
	room.Clients[client.ID] = client
	
	// Send join message
	joinMessage := &Message{
		ID:        uuid.New().String(),
		RoomID:    roomID,
		UserID:    "system",
		Username:  "System",
		Content:   fmt.Sprintf("%s joined the room", client.User.Username),
		Type:      "system",
		Timestamp: time.Now(),
	}
	
	room.Messages = append(room.Messages, joinMessage)
	h.broadcastToRoom(room, joinMessage)
	
	return nil
}

// LeaveRoom removes a client from a room
func (h *ChatHub) LeaveRoom(client *Client) {
	if client.Room == nil {
		return
	}
	
	room := client.Room
	room.mu.Lock()
	defer room.mu.Unlock()
	
	delete(room.Clients, client.ID)
	
	// Send leave message
	leaveMessage := &Message{
		ID:        uuid.New().String(),
		RoomID:    room.ID,
		UserID:    "system",
		Username:  "System",
		Content:   fmt.Sprintf("%s left the room", client.User.Username),
		Type:      "system",
		Timestamp: time.Now(),
	}
	
	room.Messages = append(room.Messages, leaveMessage)
	h.broadcastToRoom(room, leaveMessage)
	
	client.Room = nil
}

// SendMessage sends a message to a room
func (h *ChatHub) SendMessage(client *Client, content, messageType string) {
	if client.Room == nil {
		return
	}
	
	room := client.Room
	room.mu.Lock()
	defer room.mu.Unlock()
	
	message := &Message{
		ID:        uuid.New().String(),
		RoomID:    room.ID,
		UserID:    client.User.ID,
		Username:  client.User.Username,
		Content:   content,
		Type:      messageType,
		Timestamp: time.Now(),
	}
	
	room.Messages = append(room.Messages, message)
	h.broadcastToRoom(room, message)
}

// broadcastToRoom sends a message to all clients in a room
func (h *ChatHub) broadcastToRoom(room *Room, message *Message) {
	for _, client := range room.Clients {
		select {
		case client.Send <- message:
		default:
			// Client's send channel is full, close it
			close(client.Send)
			delete(room.Clients, client.ID)
		}
	}
}

// GetRoomMessages returns recent messages from a room
func (h *ChatHub) GetRoomMessages(roomID string, limit int) ([]*Message, error) {
	room, exists := h.GetRoom(roomID)
	if !exists {
		return nil, fmt.Errorf("room not found")
	}
	
	room.mu.RLock()
	defer room.mu.RUnlock()
	
	messages := room.Messages
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	
	return messages, nil
}

// GetOnlineUsers returns all online users
func (h *ChatHub) GetOnlineUsers() []*User {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	var users []*User
	for _, user := range h.users {
		if user.Status == "online" {
			users = append(users, user)
		}
	}
	return users
}

// HTTP Handlers
type ChatHandler struct {
	hub *ChatHub
}

func NewChatHandler(hub *ChatHub) *ChatHandler {
	return &ChatHandler{hub: hub}
}

func (ch *ChatHandler) handleRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case http.MethodGet:
		rooms := ch.hub.GetAllRooms()
		
		// Create response with room info and client counts
		type RoomInfo struct {
			*Room
			ClientCount int `json:"client_count"`
		}
		
		var roomInfos []RoomInfo
		for _, room := range rooms {
			roomInfos = append(roomInfos, RoomInfo{
				Room:        room,
				ClientCount: len(room.Clients),
			})
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rooms": roomInfos,
			"total": len(roomInfos),
		})
		
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			CreatedBy   string `json:"created_by"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		room := ch.hub.CreateRoom(req.Name, req.Description, req.CreatedBy)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(room)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ch *ChatHandler) handleRoomMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "Room ID required", http.StatusBadRequest)
		return
	}
	
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || l != 1 {
			limit = 50
		}
	}
	
	messages, err := ch.hub.GetRoomMessages(roomID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
		"room_id":  roomID,
	})
}

func (ch *ChatHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		user := ch.hub.CreateUser(req.Username, req.Email)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ch *ChatHandler) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	users := ch.hub.GetOnlineUsers()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

func (ch *ChatHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would upgrade HTTP to WebSocket
	// For now, we'll return a mock response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "WebSocket endpoint - upgrade to WebSocket protocol",
		"note":    "This is a mock implementation. In production, use gorilla/websocket or similar.",
	})
}

func (ch *ChatHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	rooms := ch.hub.GetAllRooms()
	onlineUsers := ch.hub.GetOnlineUsers()
	
	totalMessages := 0
	totalClients := 0
	
	for _, room := range rooms {
		totalMessages += len(room.Messages)
		totalClients += len(room.Clients)
	}
	
	stats := map[string]interface{}{
		"total_rooms":    len(rooms),
		"total_users":    len(ch.hub.users),
		"online_users":   len(onlineUsers),
		"total_messages": totalMessages,
		"active_clients": totalClients,
		"generated_at":   time.Now(),
	}
	
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Initialize chat hub
	hub := NewChatHub()
	handler := NewChatHandler(hub)
	
	// Setup routes
	http.HandleFunc("/api/rooms", handler.handleRooms)
	http.HandleFunc("/api/rooms/messages", handler.handleRoomMessages)
	http.HandleFunc("/api/users", handler.handleUsers)
	http.HandleFunc("/api/users/online", handler.handleOnlineUsers)
	http.HandleFunc("/api/stats", handler.handleStats)
	http.HandleFunc("/ws", handler.handleWebSocket)
	
	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"service":   "chat-application",
			"version":   "1.0.0",
		})
	})
	
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	
	// Root endpoint with API documentation
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "NetCore-Go Chat Application",
			"version": "1.0.0",
			"features": []string{
				"Real-time messaging",
				"Multiple chat rooms",
				"User management",
				"WebSocket support",
				"Message history",
				"Online user tracking",
			},
			"endpoints": map[string]string{
				"GET /api/rooms":              "Get all chat rooms",
				"POST /api/rooms":             "Create a new room",
				"GET /api/rooms/messages":     "Get room messages",
				"POST /api/users":             "Create a new user",
				"GET /api/users/online":       "Get online users",
				"GET /api/stats":              "Get chat statistics",
				"GET /ws":                     "WebSocket endpoint",
				"GET /health":                 "Health check",
			},
		})
	})
	
	port := ":8082"
	fmt.Printf("💬 Chat Application starting on port %s\n", port)
	fmt.Println("📖 API Documentation: http://localhost:8082")
	fmt.Println("🏥 Health Check: http://localhost:8082/health")
	fmt.Println("🔌 WebSocket: ws://localhost:8082/ws")
	fmt.Println("📊 Statistics: http://localhost:8082/api/stats")
	
	log.Fatal(http.ListenAndServe(port, nil))
}