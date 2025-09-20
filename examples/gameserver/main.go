package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/phuhao00/netcore-go"
	"github.com/phuhao00/netcore-go/pkg/core"
	"github.com/phuhao00/netcore-go/pkg/heartbeat"
	"github.com/phuhao00/netcore-go/pkg/pool"
)

// GameMessage 游戏消息
type GameMessage struct {
	Type      string      `json:"type"`
	PlayerID  string      `json:"player_id"`
	RoomID    string      `json:"room_id"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// Player 玩家信息
type Player struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	HP       int       `json:"hp"`
	Score    int       `json:"score"`
	LastSeen time.Time `json:"last_seen"`
	Conn     core.Connection
}

// GameRoom 游戏房间
type GameRoom struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Players map[string]*Player `json:"players"`
	MaxSize int                `json:"max_size"`
	Status  string             `json:"status"` // waiting, playing, finished
	mu      sync.RWMutex
}

// GameServer 游戏服务器
type GameServer struct {
	server    core.Server
	rooms     map[string]*GameRoom
	players   map[string]*Player
	mu        sync.RWMutex
	heartbeat *heartbeat.HeartbeatDetector
	pool      pool.ConnectionPool
	stats     *GameStats
}

// GameStats 游戏统计
type GameStats struct {
	TotalPlayers   int64 `json:"total_players"`
	ActiveRooms    int64 `json:"active_rooms"`
	MessagesPerSec int64 `json:"messages_per_sec"`
	AverageLatency int64 `json:"average_latency"`
	LastUpdateTime int64 `json:"last_update_time"`
	mu             sync.RWMutex
}

// NewGameServer 创建游戏服务器
func NewGameServer() *GameServer {
	gs := &GameServer{
		rooms:   make(map[string]*GameRoom),
		players: make(map[string]*Player),
		stats:   &GameStats{},
	}

	// 创建连接池
	gs.pool = pool.NewConnectionPool(&pool.Config{
		Address:     "localhost:0",
		MinSize:     10,
		MaxSize:     1000,
		IdleTimeout: time.Minute * 5,
		ConnTimeout: time.Second * 30,
	})

	// 创建心跳检测器
	// heartbeatConfig := &heartbeat.HeartbeatConfig{
	// 	Interval:          time.Second * 30,
	// 	Timeout:           time.Second * 10,
	// 	MaxMissed:         3,
	// 	Enabled:           true,
	// 	AutoReconnect:     true,
	// 	ReconnectInterval: time.Second * 5,
	// 	MaxReconnectTries: 5,
	// }
	// 注意：heartbeat.NewDetector需要connection参数，这里先设为nil
	// gs.heartbeat = heartbeat.NewHeartbeatDetector(nil, heartbeatConfig)

	return gs
}

// Start 启动游戏服务器
func (gs *GameServer) Start(addr string) error {
	// 暂时简化服务器创建
	gs.server = netcore.NewServer(nil)

	// 添加中间件
	// middlewareManager := middleware.NewManager() // 暂时注释掉
	// middlewareManager.RegisterWebAPIPreset() // 暂时注释掉

	// 设置消息处理器
	// gs.server.SetMessageHandler(gs.handleMessage) // 暂时注释掉
	// gs.server.SetConnectHandler(gs.handleConnect) // 暂时注释掉
	// gs.server.SetDisconnectHandler(gs.handleDisconnect) // 暂时注释掉

	// 启动心跳检测
	// gs.heartbeat.Start() // 暂时注释掉，因为heartbeat需要具体的连接

	// 启动统计更新
	go gs.updateStats()

	// 启动游戏循环
	go gs.gameLoop()

	log.Printf("游戏服务器启动在 %s", addr)
	return gs.server.Start(addr)
}

// handleConnect 处理连接
func (gs *GameServer) handleConnect(conn core.Connection) {
	log.Printf("新玩家连接: %s", conn.RemoteAddr())

	// 添加到连接池
	// gs.pool.AddConnection(conn) // 暂时注释掉，因为接口不匹配

	// 发送欢迎消息
	welcomeMsg := GameMessage{
		Type:      "welcome",
		Data:      "欢迎来到游戏服务器！",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(welcomeMsg)
	coreMsg := core.NewMessage(core.MessageTypeJSON, data)
	conn.SendMessage(*coreMsg)
}

// handleDisconnect 处理断开连接
func (gs *GameServer) handleDisconnect(conn core.Connection) {
	log.Printf("玩家断开连接: %s", conn.RemoteAddr())

	// 从连接池移除
	// gs.pool.RemoveConnection(conn) // 暂时注释掉，因为接口不匹配

	// 查找并移除玩家
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for playerID, player := range gs.players {
		if player.Conn.ID() == conn.ID() {
			// 从房间移除玩家
			gs.removePlayerFromRoom(playerID)
			// 删除玩家
			delete(gs.players, playerID)
			break
		}
	}
}

// handleMessage 处理消息
func (gs *GameServer) handleMessage(conn core.Connection, data []byte) {
	var msg GameMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("解析消息失败: %v", err)
		return
	}

	msg.Timestamp = time.Now().Unix()

	switch msg.Type {
	case "join":
		gs.handleJoin(conn, &msg)
	case "leave":
		// gs.handleLeave(conn, &msg) // 方法未实现，暂时注释掉
	case "move":
		gs.handleMove(conn, &msg)
	case "attack":
		gs.handleAttack(conn, &msg)
	case "chat":
		gs.handleChat(conn, &msg)
	case "heartbeat":
		gs.handleHeartbeat(conn, &msg)
	default:
		log.Printf("未知消息类型: %s", msg.Type)
	}
}

// handleJoin 处理加入游戏
func (gs *GameServer) handleJoin(conn core.Connection, msg *GameMessage) {
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	playerName, _ := data["name"].(string)
	roomID, _ := data["room_id"].(string)

	if playerName == "" {
		playerName = fmt.Sprintf("Player_%d", rand.Intn(10000))
	}

	if roomID == "" {
		roomID = "default"
	}

	// 创建玩家
	player := &Player{
		ID:       fmt.Sprintf("%s_%d", conn.RemoteAddr(), time.Now().UnixNano()),
		Name:     playerName,
		X:        rand.Float64() * 100,
		Y:        rand.Float64() * 100,
		HP:       100,
		Score:    0,
		LastSeen: time.Now(),
		Conn:     conn,
	}

	gs.mu.Lock()
	gs.players[player.ID] = player
	gs.mu.Unlock()

	// 加入房间
	room := gs.getOrCreateRoom(roomID)
	gs.addPlayerToRoom(player, room)

	// 发送加入成功消息
	response := GameMessage{
		Type:     "joined",
		PlayerID: player.ID,
		RoomID:   roomID,
		Data: map[string]interface{}{
			"player": player,
			"room":   room,
		},
		Timestamp: time.Now().Unix(),
	}

	gs.sendToPlayer(player.ID, &response)

	// 通知房间其他玩家
	gs.broadcastToRoom(roomID, &GameMessage{
		Type:      "player_joined",
		PlayerID:  player.ID,
		RoomID:    roomID,
		Data:      player,
		Timestamp: time.Now().Unix(),
	}, player.ID)
}

// handleMove 处理移动
func (gs *GameServer) handleMove(conn core.Connection, msg *GameMessage) {
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	x, _ := data["x"].(float64)
	y, _ := data["y"].(float64)

	gs.mu.Lock()
	player, exists := gs.players[msg.PlayerID]
	if exists {
		player.X = x
		player.Y = y
		player.LastSeen = time.Now()
	}
	gs.mu.Unlock()

	if !exists {
		return
	}

	// 广播移动消息
	gs.broadcastToRoom(msg.RoomID, &GameMessage{
		Type:     "player_moved",
		PlayerID: msg.PlayerID,
		RoomID:   msg.RoomID,
		Data: map[string]interface{}{
			"x": x,
			"y": y,
		},
		Timestamp: time.Now().Unix(),
	}, msg.PlayerID)
}

// handleAttack 处理攻击
func (gs *GameServer) handleAttack(conn core.Connection, msg *GameMessage) {
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	targetID, _ := data["target"].(string)
	damage, _ := data["damage"].(float64)

	gs.mu.Lock()
	attacker, attackerExists := gs.players[msg.PlayerID]
	target, targetExists := gs.players[targetID]
	gs.mu.Unlock()

	if !attackerExists || !targetExists {
		return
	}

	// 计算伤害
	damageValue := int(damage)
	if damageValue <= 0 {
		damageValue = 10
	}

	gs.mu.Lock()
	target.HP -= damageValue
	if target.HP < 0 {
		target.HP = 0
	}
	attacker.Score += damageValue
	gs.mu.Unlock()

	// 广播攻击消息
	gs.broadcastToRoom(msg.RoomID, &GameMessage{
		Type:     "player_attacked",
		PlayerID: msg.PlayerID,
		RoomID:   msg.RoomID,
		Data: map[string]interface{}{
			"attacker":  attacker.ID,
			"target":    target.ID,
			"damage":    damageValue,
			"target_hp": target.HP,
		},
		Timestamp: time.Now().Unix(),
	})

	// 检查是否死亡
	if target.HP <= 0 {
		gs.broadcastToRoom(msg.RoomID, &GameMessage{
			Type:     "player_died",
			PlayerID: target.ID,
			RoomID:   msg.RoomID,
			Data: map[string]interface{}{
				"killer": attacker.ID,
			},
			Timestamp: time.Now().Unix(),
		})

		// 重生玩家
		go func() {
			time.Sleep(time.Second * 5)
			gs.mu.Lock()
			target.HP = 100
			target.X = rand.Float64() * 100
			target.Y = rand.Float64() * 100
			gs.mu.Unlock()

			gs.broadcastToRoom(msg.RoomID, &GameMessage{
				Type:     "player_respawned",
				PlayerID: target.ID,
				RoomID:   msg.RoomID,
				Data: map[string]interface{}{
					"x":  target.X,
					"y":  target.Y,
					"hp": target.HP,
				},
				Timestamp: time.Now().Unix(),
			})
		}()
	}
}

// handleChat 处理聊天
func (gs *GameServer) handleChat(conn core.Connection, msg *GameMessage) {
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	message, _ := data["message"].(string)
	if message == "" {
		return
	}

	gs.mu.RLock()
	player, exists := gs.players[msg.PlayerID]
	gs.mu.RUnlock()

	if !exists {
		return
	}

	// 广播聊天消息
	gs.broadcastToRoom(msg.RoomID, &GameMessage{
		Type:     "chat",
		PlayerID: msg.PlayerID,
		RoomID:   msg.RoomID,
		Data: map[string]interface{}{
			"player_name": player.Name,
			"message":     message,
		},
		Timestamp: time.Now().Unix(),
	})
}

// handleHeartbeat 处理心跳
func (gs *GameServer) handleHeartbeat(conn core.Connection, msg *GameMessage) {
	gs.mu.Lock()
	if player, exists := gs.players[msg.PlayerID]; exists {
		player.LastSeen = time.Now()
	}
	gs.mu.Unlock()

	// 发送心跳响应
	response := GameMessage{
		Type:      "heartbeat_ack",
		PlayerID:  msg.PlayerID,
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(response)
	coreMsg := core.NewMessage(core.MessageTypeJSON, data)
	conn.SendMessage(*coreMsg)
}

// getOrCreateRoom 获取或创建房间
func (gs *GameServer) getOrCreateRoom(roomID string) *GameRoom {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	room, exists := gs.rooms[roomID]
	if !exists {
		room = &GameRoom{
			ID:      roomID,
			Name:    fmt.Sprintf("Room %s", roomID),
			Players: make(map[string]*Player),
			MaxSize: 10,
			Status:  "waiting",
		}
		gs.rooms[roomID] = room
	}

	return room
}

// addPlayerToRoom 添加玩家到房间
func (gs *GameServer) addPlayerToRoom(player *Player, room *GameRoom) {
	room.mu.Lock()
	defer room.mu.Unlock()

	if len(room.Players) < room.MaxSize {
		room.Players[player.ID] = player
		if len(room.Players) >= 2 && room.Status == "waiting" {
			room.Status = "playing"
		}
	}
}

// removePlayerFromRoom 从房间移除玩家
func (gs *GameServer) removePlayerFromRoom(playerID string) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, room := range gs.rooms {
		room.mu.Lock()
		if _, exists := room.Players[playerID]; exists {
			delete(room.Players, playerID)
			if len(room.Players) == 0 {
				room.Status = "waiting"
			}
		}
		room.mu.Unlock()
	}
}

// sendToPlayer 发送消息给指定玩家
func (gs *GameServer) sendToPlayer(playerID string, msg *GameMessage) {
	gs.mu.RLock()
	player, exists := gs.players[playerID]
	gs.mu.RUnlock()

	if !exists {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	coreMsg := core.NewMessage(core.MessageTypeJSON, data)
	player.Conn.SendMessage(*coreMsg)
}

// broadcastToRoom 广播消息到房间
func (gs *GameServer) broadcastToRoom(roomID string, msg *GameMessage, excludePlayerID ...string) {
	gs.mu.RLock()
	room, exists := gs.rooms[roomID]
	gs.mu.RUnlock()

	if !exists {
		return
	}

	excludeMap := make(map[string]bool)
	for _, id := range excludePlayerID {
		excludeMap[id] = true
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	room.mu.RLock()
	for playerID, player := range room.Players {
		if !excludeMap[playerID] {
			coreMsg := core.NewMessage(core.MessageTypeJSON, data)
			player.Conn.SendMessage(*coreMsg)
		}
	}
	room.mu.RUnlock()
}

// gameLoop 游戏循环
func (gs *GameServer) gameLoop() {
	ticker := time.NewTicker(time.Second / 60) // 60 FPS
	defer ticker.Stop()

	for range ticker.C {
		gs.updateGame()
	}
}

// updateGame 更新游戏状态
func (gs *GameServer) updateGame() {
	now := time.Now()

	// 检查离线玩家
	gs.mu.Lock()
	for playerID, player := range gs.players {
		if now.Sub(player.LastSeen) > time.Minute*2 {
			// 移除离线玩家
			gs.removePlayerFromRoom(playerID)
			delete(gs.players, playerID)
			log.Printf("移除离线玩家: %s", playerID)
		}
	}
	gs.mu.Unlock()

	// 清理空房间
	gs.mu.Lock()
	for roomID, room := range gs.rooms {
		room.mu.RLock()
		playerCount := len(room.Players)
		room.mu.RUnlock()

		if playerCount == 0 {
			delete(gs.rooms, roomID)
		}
	}
	gs.mu.Unlock()
}

// updateStats 更新统计信息
func (gs *GameServer) updateStats() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for range ticker.C {
		gs.mu.RLock()
		totalPlayers := int64(len(gs.players))
		activeRooms := int64(len(gs.rooms))
		gs.mu.RUnlock()

		gs.stats.mu.Lock()
		gs.stats.TotalPlayers = totalPlayers
		gs.stats.ActiveRooms = activeRooms
		gs.stats.LastUpdateTime = time.Now().Unix()
		gs.stats.mu.Unlock()

		log.Printf("游戏统计 - 玩家: %d, 房间: %d", totalPlayers, activeRooms)
	}
}

// GetStats 获取统计信息
func (gs *GameServer) GetStats() *GameStats {
	gs.stats.mu.RLock()
	defer gs.stats.mu.RUnlock()

	return &GameStats{
		TotalPlayers:   gs.stats.TotalPlayers,
		ActiveRooms:    gs.stats.ActiveRooms,
		MessagesPerSec: gs.stats.MessagesPerSec,
		AverageLatency: gs.stats.AverageLatency,
		LastUpdateTime: gs.stats.LastUpdateTime,
	}
}

// Stop 停止游戏服务器
func (gs *GameServer) Stop() error {
	if gs.heartbeat != nil {
		gs.heartbeat.Stop()
	}

	if gs.pool != nil {
		gs.pool.Close()
	}

	if gs.server != nil {
		return gs.server.Stop()
	}

	return nil
}

func main() {
	gameServer := NewGameServer()

	// 启动HTTP API服务器（用于管理和监控）
	go func() {
		http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			stats := gameServer.GetStats()
			json.NewEncoder(w).Encode(stats)
		})

		http.HandleFunc("/rooms", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			gameServer.mu.RLock()
			rooms := make(map[string]*GameRoom)
			for id, room := range gameServer.rooms {
				rooms[id] = room
			}
			gameServer.mu.RUnlock()
			json.NewEncoder(w).Encode(rooms)
		})

		log.Println("HTTP API服务器启动在 :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 启动游戏服务器
	if err := gameServer.Start(":9999"); err != nil {
		log.Fatalf("启动游戏服务器失败: %v", err)
	}
}
