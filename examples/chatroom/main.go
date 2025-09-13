package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/netcore-go"
	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/longpoll"
	"github.com/netcore-go/pkg/pool"
)

// ChatMessage 聊天消息
type ChatMessage struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // message, join, leave, system
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// User 用户信息
type User struct {
	ID       string          `json:"id"`
	Username string          `json:"username"`
	RoomID   string          `json:"room_id"`
	Conn     core.Connection `json:"-"`
	JoinTime time.Time       `json:"join_time"`
	LastSeen time.Time       `json:"last_seen"`
	Status   string          `json:"status"` // online, away, offline
}

// ChatRoom 聊天室
type ChatRoom struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Users       map[string]*User   `json:"users"`
	Messages    []*ChatMessage     `json:"messages"`
	MaxUsers    int                `json:"max_users"`
	MaxMessages int                `json:"max_messages"`
	CreatedAt   time.Time          `json:"created_at"`
	mu          sync.RWMutex       `json:"-"`
}

// ChatServer 聊天服务器
type ChatServer struct {
	server    core.Server
	rooms     map[string]*ChatRoom
	users     map[string]*User
	longPoll  *longpoll.LongPollManager
	pool      pool.ConnectionPool
	stats     *ChatStats
	mu        sync.RWMutex
}

// ChatStats 聊天统计
type ChatStats struct {
	TotalUsers      int64 `json:"total_users"`
	ActiveRooms     int64 `json:"active_rooms"`
	TotalMessages   int64 `json:"total_messages"`
	MessagesPerMin  int64 `json:"messages_per_min"`
	LastUpdateTime  int64 `json:"last_update_time"`
	mu              sync.RWMutex
}

// NewChatServer 创建聊天服务器
func NewChatServer() *ChatServer {
	cs := &ChatServer{
		rooms: make(map[string]*ChatRoom),
		users: make(map[string]*User),
		stats: &ChatStats{},
	}

	// 创建连接池
	cs.pool = pool.NewConnectionPool(&pool.Config{
		MinSize:     5,
		MaxSize:     500,
		IdleTimeout: time.Minute * 10,
		ConnTimeout: time.Second * 30,
	})

	// 创建长轮询管理器
	cs.longPoll = longpoll.NewManager(&longpoll.Config{
		MaxHistory:      1000,
		CleanupInterval: time.Minute,
		DefaultTimeout:  time.Second * 30,
	})

	// 创建默认房间
	cs.createDefaultRooms()

	return cs
}

// createDefaultRooms 创建默认房间
func (cs *ChatServer) createDefaultRooms() {
	defaultRooms := []struct {
		id, name, description string
	}{
		{"general", "General", "General discussion room"},
		{"tech", "Technology", "Technology discussion room"},
		{"random", "Random", "Random chat room"},
	}

	for _, room := range defaultRooms {
		cs.rooms[room.id] = &ChatRoom{
			ID:          room.id,
			Name:        room.name,
			Description: room.description,
			Users:       make(map[string]*User),
			Messages:    make([]*ChatMessage, 0),
			MaxUsers:    100,
			MaxMessages: 1000,
			CreatedAt:   time.Now(),
		}
	}
}

// Start 启动聊天服务器
func (cs *ChatServer) Start(addr string) error {
	config := &netcore.Config{
		Host: "localhost",
		Port: 8080,
	}

	cs.server = netcore.NewServer(config)

	// 设置处理器
	// cs.server.SetMessageHandler(cs.handleMessage)
	// cs.server.SetConnectHandler(cs.handleConnect)
	// cs.server.SetDisconnectHandler(cs.handleDisconnect)

	// 启动统计更新
	go cs.updateStats()

	// 启动清理任务
	go cs.cleanupTask()

	log.Printf("聊天服务器启动在 %s", addr)
	return cs.server.Start(addr)
}

// handleConnect 处理连接
func (cs *ChatServer) handleConnect(conn core.Connection) {
	log.Printf("新用户连接: %s", conn.RemoteAddr())

	// 添加到连接池
	// cs.pool.AddConnection(conn) // 暂时注释掉，因为类型不匹配

	// 发送欢迎消息
	welcomeMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "system",
		Content:   "欢迎来到聊天室！请发送 join 命令加入房间。",
		Timestamp: time.Now(),
	}

	cs.sendMessageToConnection(conn, welcomeMsg)

	// 发送可用房间列表
	cs.sendRoomList(conn)
}

// handleDisconnect 处理断开连接
func (cs *ChatServer) handleDisconnect(conn core.Connection) {
	log.Printf("用户断开连接: %s", conn.RemoteAddr())

	// 从连接池移除
	// cs.pool.RemoveConnection(conn) // 暂时注释掉，因为类型不匹配

	// 查找并移除用户
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for userID, user := range cs.users {
		if user.Conn.ID() == conn.ID() {
			// 从房间移除用户
			cs.removeUserFromRoom(userID)
			// 删除用户
			delete(cs.users, userID)
			break
		}
	}
}

// handleMessage 处理消息
func (cs *ChatServer) handleMessage(conn core.Connection, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("解析消息失败: %v", err)
		return
	}

	msgType, _ := msg["type"].(string)
	content, _ := msg["content"].(string)
	roomID, _ := msg["room_id"].(string)
	userID, _ := msg["user_id"].(string)
	username, _ := msg["username"].(string)

	switch msgType {
	case "join":
		cs.handleJoin(conn, userID, username, roomID)
	case "leave":
		cs.handleLeave(conn, userID)
	case "message":
		cs.handleChatMessage(conn, userID, roomID, content)
	case "private":
		targetUserID, _ := msg["target_user_id"].(string)
		cs.handlePrivateMessage(conn, userID, targetUserID, content)
	case "list_rooms":
		cs.sendRoomList(conn)
	case "list_users":
		cs.sendUserList(conn, roomID)
	case "ping":
		cs.handlePing(conn, userID)
	default:
		log.Printf("未知消息类型: %s", msgType)
	}
}

// handleJoin 处理加入房间
func (cs *ChatServer) handleJoin(conn core.Connection, userID, username, roomID string) {
	if userID == "" {
		userID = fmt.Sprintf("user_%d", time.Now().UnixNano())
	}

	if username == "" {
		username = fmt.Sprintf("Guest_%d", time.Now().UnixNano()%10000)
	}

	if roomID == "" {
		roomID = "general"
	}

	// 检查房间是否存在
	cs.mu.RLock()
	room, exists := cs.rooms[roomID]
	cs.mu.RUnlock()

	if !exists {
		errorMsg := &ChatMessage{
			ID:        cs.generateMessageID(),
			Type:      "error",
			Content:   fmt.Sprintf("房间 %s 不存在", roomID),
			Timestamp: time.Now(),
		}
		cs.sendMessageToConnection(conn, errorMsg)
		return
	}

	// 检查房间是否已满
	room.mu.RLock()
	roomFull := len(room.Users) >= room.MaxUsers
	room.mu.RUnlock()

	if roomFull {
		errorMsg := &ChatMessage{
			ID:        cs.generateMessageID(),
			Type:      "error",
			Content:   fmt.Sprintf("房间 %s 已满", roomID),
			Timestamp: time.Now(),
		}
		cs.sendMessageToConnection(conn, errorMsg)
		return
	}

	// 创建用户
	user := &User{
		ID:       userID,
		Username: username,
		RoomID:   roomID,
		Conn:     conn,
		JoinTime: time.Now(),
		LastSeen: time.Now(),
		Status:   "online",
	}

	// 如果用户已存在，先移除
	cs.mu.Lock()
	if existingUser, exists := cs.users[userID]; exists {
		cs.removeUserFromRoomUnsafe(existingUser.ID)
	}
	cs.users[userID] = user
	cs.mu.Unlock()

	// 添加到房间
	room.mu.Lock()
	room.Users[userID] = user
	room.mu.Unlock()

	// 发送加入成功消息
	joinMsg := &ChatMessage{
		ID:       cs.generateMessageID(),
		Type:     "joined",
		RoomID:   roomID,
		UserID:   userID,
		Username: username,
		Content:  fmt.Sprintf("成功加入房间 %s", room.Name),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"room_info": map[string]interface{}{
				"id":          room.ID,
				"name":        room.Name,
				"description": room.Description,
				"user_count":  len(room.Users),
			},
		},
	}

	cs.sendMessageToConnection(conn, joinMsg)

	// 发送最近的消息历史
	cs.sendRecentMessages(conn, roomID, 10)

	// 通知房间其他用户
	systemMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "join",
		RoomID:    roomID,
		UserID:    userID,
		Username:  username,
		Content:   fmt.Sprintf("%s 加入了房间", username),
		Timestamp: time.Now(),
	}

	cs.broadcastToRoom(roomID, systemMsg, userID)
	cs.addMessageToRoom(roomID, systemMsg)

	// 发布长轮询事件
	cs.longPoll.Publish(&longpoll.Event{
		ID:   systemMsg.ID,
		Type: "user_joined",
		Data: systemMsg,
	})
}

// handleLeave 处理离开房间
func (cs *ChatServer) handleLeave(conn core.Connection, userID string) {
	cs.mu.Lock()
	user, exists := cs.users[userID]
	if exists {
		cs.removeUserFromRoomUnsafe(userID)
		delete(cs.users, userID)
	}
	cs.mu.Unlock()

	if !exists {
		return
	}

	// 发送离开确认
	leaveMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "left",
		Content:   "已离开房间",
		Timestamp: time.Now(),
	}

	cs.sendMessageToConnection(conn, leaveMsg)

	// 通知房间其他用户
	systemMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "leave",
		RoomID:    user.RoomID,
		UserID:    userID,
		Username:  user.Username,
		Content:   fmt.Sprintf("%s 离开了房间", user.Username),
		Timestamp: time.Now(),
	}

	cs.broadcastToRoom(user.RoomID, systemMsg)
	cs.addMessageToRoom(user.RoomID, systemMsg)

	// 发布长轮询事件
	cs.longPoll.Publish(&longpoll.Event{
		ID:   systemMsg.ID,
		Type: "user_left",
		Data: systemMsg,
	})
}

// handleChatMessage 处理聊天消息
func (cs *ChatServer) handleChatMessage(conn core.Connection, userID, roomID, content string) {
	if content == "" {
		return
	}

	cs.mu.RLock()
	user, userExists := cs.users[userID]
	_, roomExists := cs.rooms[roomID]
	cs.mu.RUnlock()

	if !userExists || !roomExists {
		errorMsg := &ChatMessage{
			ID:        cs.generateMessageID(),
			Type:      "error",
			Content:   "用户或房间不存在",
			Timestamp: time.Now(),
		}
		cs.sendMessageToConnection(conn, errorMsg)
		return
	}

	// 更新用户最后活跃时间
	cs.mu.Lock()
	user.LastSeen = time.Now()
	cs.mu.Unlock()

	// 创建聊天消息
	chatMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "message",
		RoomID:    roomID,
		UserID:    userID,
		Username:  user.Username,
		Content:   content,
		Timestamp: time.Now(),
	}

	// 广播消息
	cs.broadcastToRoom(roomID, chatMsg)
	cs.addMessageToRoom(roomID, chatMsg)

	// 更新统计
	cs.stats.mu.Lock()
	cs.stats.TotalMessages++
	cs.stats.mu.Unlock()

	// 发布长轮询事件
	cs.longPoll.Publish(&longpoll.Event{
		ID:   chatMsg.ID,
		Type: "message",
		Data: chatMsg,
	})
}

// handlePrivateMessage 处理私聊消息
func (cs *ChatServer) handlePrivateMessage(conn core.Connection, senderID, targetUserID, content string) {
	if content == "" {
		return
	}

	cs.mu.RLock()
	sender, senderExists := cs.users[senderID]
	target, targetExists := cs.users[targetUserID]
	cs.mu.RUnlock()

	if !senderExists || !targetExists {
		errorMsg := &ChatMessage{
			ID:        cs.generateMessageID(),
			Type:      "error",
			Content:   "发送者或接收者不存在",
			Timestamp: time.Now(),
		}
		cs.sendMessageToConnection(conn, errorMsg)
		return
	}

	// 创建私聊消息
	privateMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "private",
		UserID:    senderID,
		Username:  sender.Username,
		Content:   content,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"target_user_id": targetUserID,
			"target_username": target.Username,
		},
	}

	// 发送给目标用户
	cs.sendMessageToConnection(target.Conn, privateMsg)

	// 发送确认给发送者
	confirmMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "private_sent",
		Content:   fmt.Sprintf("私聊消息已发送给 %s", target.Username),
		Timestamp: time.Now(),
	}

	cs.sendMessageToConnection(conn, confirmMsg)
}

// handlePing 处理心跳
func (cs *ChatServer) handlePing(conn core.Connection, userID string) {
	cs.mu.Lock()
	if user, exists := cs.users[userID]; exists {
		user.LastSeen = time.Now()
	}
	cs.mu.Unlock()

	// 发送pong响应
	pongMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "pong",
		Timestamp: time.Now(),
	}

	cs.sendMessageToConnection(conn, pongMsg)
}

// sendRoomList 发送房间列表
func (cs *ChatServer) sendRoomList(conn core.Connection) {
	cs.mu.RLock()
	roomList := make([]map[string]interface{}, 0, len(cs.rooms))
	for _, room := range cs.rooms {
		room.mu.RLock()
		roomInfo := map[string]interface{}{
			"id":          room.ID,
			"name":        room.Name,
			"description": room.Description,
			"user_count":  len(room.Users),
			"max_users":   room.MaxUsers,
		}
		room.mu.RUnlock()
		roomList = append(roomList, roomInfo)
	}
	cs.mu.RUnlock()

	listMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "room_list",
		Content:   "可用房间列表",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"rooms": roomList,
		},
	}

	cs.sendMessageToConnection(conn, listMsg)
}

// sendUserList 发送用户列表
func (cs *ChatServer) sendUserList(conn core.Connection, roomID string) {
	cs.mu.RLock()
	room, exists := cs.rooms[roomID]
	cs.mu.RUnlock()

	if !exists {
		errorMsg := &ChatMessage{
			ID:        cs.generateMessageID(),
			Type:      "error",
			Content:   "房间不存在",
			Timestamp: time.Now(),
		}
		cs.sendMessageToConnection(conn, errorMsg)
		return
	}

	room.mu.RLock()
	userList := make([]map[string]interface{}, 0, len(room.Users))
	for _, user := range room.Users {
		userInfo := map[string]interface{}{
			"id":        user.ID,
			"username":  user.Username,
			"status":    user.Status,
			"join_time": user.JoinTime,
		}
		userList = append(userList, userInfo)
	}
	room.mu.RUnlock()

	listMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "user_list",
		RoomID:    roomID,
		Content:   fmt.Sprintf("房间 %s 的用户列表", roomID),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"users": userList,
		},
	}

	cs.sendMessageToConnection(conn, listMsg)
}

// sendRecentMessages 发送最近的消息
func (cs *ChatServer) sendRecentMessages(conn core.Connection, roomID string, count int) {
	cs.mu.RLock()
	room, exists := cs.rooms[roomID]
	cs.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	messages := room.Messages
	start := len(messages) - count
	if start < 0 {
		start = 0
	}
	recentMessages := messages[start:]
	room.mu.RUnlock()

	historyMsg := &ChatMessage{
		ID:        cs.generateMessageID(),
		Type:      "history",
		RoomID:    roomID,
		Content:   "消息历史",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"messages": recentMessages,
		},
	}

	cs.sendMessageToConnection(conn, historyMsg)
}

// sendMessageToConnection 发送消息到连接
func (cs *ChatServer) sendMessageToConnection(conn core.Connection, msg *ChatMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("序列化消息失败: %v", err)
		return
	}

	coreMsg := core.NewMessage(core.MessageTypeJSON, data)
	conn.SendMessage(*coreMsg)
}

// broadcastToRoom 广播消息到房间
func (cs *ChatServer) broadcastToRoom(roomID string, msg *ChatMessage, excludeUserID ...string) {
	cs.mu.RLock()
	room, exists := cs.rooms[roomID]
	cs.mu.RUnlock()

	if !exists {
		return
	}

	excludeMap := make(map[string]bool)
	for _, id := range excludeUserID {
		excludeMap[id] = true
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("序列化消息失败: %v", err)
		return
	}

	room.mu.RLock()
	for userID, user := range room.Users {
		if !excludeMap[userID] {
				coreMsg := core.NewMessage(core.MessageTypeJSON, data)
			user.Conn.SendMessage(*coreMsg)
		}
	}
	room.mu.RUnlock()
}

// addMessageToRoom 添加消息到房间
func (cs *ChatServer) addMessageToRoom(roomID string, msg *ChatMessage) {
	cs.mu.RLock()
	room, exists := cs.rooms[roomID]
	cs.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.Lock()
	room.Messages = append(room.Messages, msg)

	// 限制消息数量
	if len(room.Messages) > room.MaxMessages {
		room.Messages = room.Messages[len(room.Messages)-room.MaxMessages:]
	}
	room.mu.Unlock()
}

// removeUserFromRoom 从房间移除用户
func (cs *ChatServer) removeUserFromRoom(userID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.removeUserFromRoomUnsafe(userID)
}

// removeUserFromRoomUnsafe 从房间移除用户（不加锁）
func (cs *ChatServer) removeUserFromRoomUnsafe(userID string) {
	user, exists := cs.users[userID]
	if !exists {
		return
	}

	room, exists := cs.rooms[user.RoomID]
	if !exists {
		return
	}

	room.mu.Lock()
	delete(room.Users, userID)
	room.mu.Unlock()
}

// generateMessageID 生成消息ID
func (cs *ChatServer) generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// updateStats 更新统计信息
func (cs *ChatServer) updateStats() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	lastMessageCount := int64(0)

	for range ticker.C {
		cs.mu.RLock()
		totalUsers := int64(len(cs.users))
		activeRooms := int64(len(cs.rooms))
		cs.mu.RUnlock()

		cs.stats.mu.Lock()
		currentMessages := cs.stats.TotalMessages
		cs.stats.MessagesPerMin = currentMessages - lastMessageCount
		lastMessageCount = currentMessages

		cs.stats.TotalUsers = totalUsers
		cs.stats.ActiveRooms = activeRooms
		cs.stats.LastUpdateTime = time.Now().Unix()
		cs.stats.mu.Unlock()

		log.Printf("聊天统计 - 用户: %d, 房间: %d, 消息/分钟: %d", 
			totalUsers, activeRooms, cs.stats.MessagesPerMin)
	}
}

// cleanupTask 清理任务
func (cs *ChatServer) cleanupTask() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// 清理离线用户
		cs.mu.Lock()
		for userID, user := range cs.users {
			if now.Sub(user.LastSeen) > time.Minute*10 {
				cs.removeUserFromRoomUnsafe(userID)
				delete(cs.users, userID)
				log.Printf("清理离线用户: %s", userID)
			}
		}
		cs.mu.Unlock()
	}
}

// GetStats 获取统计信息
func (cs *ChatServer) GetStats() *ChatStats {
	cs.stats.mu.RLock()
	defer cs.stats.mu.RUnlock()

	return &ChatStats{
		TotalUsers:     cs.stats.TotalUsers,
		ActiveRooms:    cs.stats.ActiveRooms,
		TotalMessages:  cs.stats.TotalMessages,
		MessagesPerMin: cs.stats.MessagesPerMin,
		LastUpdateTime: cs.stats.LastUpdateTime,
	}
}

// Stop 停止聊天服务器
func (cs *ChatServer) Stop() error {
	// if cs.longPoll != nil {
		// 	cs.longPoll.Stop()
		// }

	if cs.pool != nil {
		cs.pool.Close()
	}

	if cs.server != nil {
		return cs.server.Stop()
	}

	return nil
}

func main() {
	chatServer := NewChatServer()

	// 启动HTTP API服务器
	go func() {
		http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			stats := chatServer.GetStats()
			json.NewEncoder(w).Encode(stats)
		})

		http.HandleFunc("/rooms", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			chatServer.mu.RLock()
			rooms := make(map[string]*ChatRoom)
			for id, room := range chatServer.rooms {
				rooms[id] = room
			}
			chatServer.mu.RUnlock()
			json.NewEncoder(w).Encode(rooms)
		})

		// 长轮询HTTP接口
		// longPollMiddleware := longpoll.NewHTTPMiddleware(chatServer.longPoll, &longpoll.HTTPLongPollConfig{
		// 	BasePath:    "/longpoll",
		// 	Timeout:     time.Second * 30,
		// 	MaxEvents:   10,
		// 	CORSEnabled: true,
		// })

		// http.Handle("/longpoll/", longPollMiddleware)

		log.Println("HTTP API服务器启动在 :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 启动聊天服务器
	if err := chatServer.Start(":9998"); err != nil {
		log.Fatalf("启动聊天服务器失败: %v", err)
	}
}


