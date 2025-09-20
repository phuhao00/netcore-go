// Package main KCP服务器示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/phuhao00/netcore-go"
	"github.com/phuhao00/netcore-go/pkg/core"
	"github.com/phuhao00/netcore-go/pkg/kcp"
)

// GameMessage 游戏消息
type GameMessage struct {
	Type      string      `json:"type"`
	PlayerID  string      `json:"player_id"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// PlayerPosition 玩家位置
type PlayerPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// PlayerAction 玩家动作
type PlayerAction struct {
	Action string      `json:"action"`
	Target string      `json:"target,omitempty"`
	Params interface{} `json:"params,omitempty"`
}

// GameServer 游戏服务器
type GameServer struct {
	server  *kcp.KCPServer
	players map[string]*Player
	mu      sync.RWMutex
}

// Player 玩家信息
type Player struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Position   PlayerPosition         `json:"position"`
	Connection *kcp.KCPConnection     `json:"-"`
	LastSeen   time.Time              `json:"last_seen"`
	Stats      map[string]interface{} `json:"stats"`
}

// NewGameServer 创建游戏服务器
func NewGameServer() *GameServer {
	return &GameServer{
		players: make(map[string]*Player),
	}
}

// KCPHandler KCP消息处理器
type KCPHandler struct {
	gameServer *GameServer
}

// OnConnect 连接建立
func (h *KCPHandler) OnConnect(conn core.Connection) {
	kcpConn := conn.(*kcp.KCPConnection)
	log.Printf("KCP client connected: %s (ID: %s)", conn.RemoteAddr(), kcpConn.ID())

	// 发送欢迎消息
	welcomeMsg := GameMessage{
		Type:      "welcome",
		PlayerID:  kcpConn.ID(),
		Data:      map[string]interface{}{"message": "Welcome to NetCore-Go KCP Game Server!"},
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(welcomeMsg)
	msg := core.Message{
		Type:      core.MessageTypeJSON,
		Data:      data,
		Timestamp: time.Now(),
	}

	if err := conn.SendMessage(msg); err != nil {
		log.Printf("Failed to send welcome message: %v", err)
	}

	// 创建玩家
	player := &Player{
		ID:         kcpConn.ID(),
		Name:       fmt.Sprintf("Player_%s", kcpConn.ID()[:8]),
		Position:   PlayerPosition{X: 0, Y: 0, Z: 0},
		Connection: kcpConn,
		LastSeen:   time.Now(),
		Stats:      make(map[string]interface{}),
	}

	h.gameServer.mu.Lock()
	h.gameServer.players[kcpConn.ID()] = player
	h.gameServer.mu.Unlock()

	// 广播玩家加入消息
	h.broadcastPlayerJoin(player)
}

// OnMessage 消息处理
func (h *KCPHandler) OnMessage(conn core.Connection, msg core.Message) {
	kcpConn := conn.(*kcp.KCPConnection)
	log.Printf("KCP message received from %s: type=%v, size=%d",
		conn.RemoteAddr(), msg.Type, len(msg.Data))

	// 更新玩家最后活跃时间
	h.gameServer.mu.Lock()
	if player, exists := h.gameServer.players[kcpConn.ID()]; exists {
		player.LastSeen = time.Now()
	}
	h.gameServer.mu.Unlock()

	// 解析游戏消息
	var gameMsg GameMessage
	if err := json.Unmarshal(msg.Data, &gameMsg); err != nil {
		log.Printf("Failed to parse game message: %v", err)
		return
	}

	// 处理不同类型的消息
	switch gameMsg.Type {
	case "ping":
		h.handlePing(kcpConn, gameMsg)
	case "position_update":
		h.handlePositionUpdate(kcpConn, gameMsg)
	case "player_action":
		h.handlePlayerAction(kcpConn, gameMsg)
	case "chat":
		h.handleChat(kcpConn, gameMsg)
	case "get_players":
		h.handleGetPlayers(kcpConn)
	default:
		log.Printf("Unknown message type: %s", gameMsg.Type)
	}
}

// OnDisconnect 连接断开
func (h *KCPHandler) OnDisconnect(conn core.Connection, err error) {
	kcpConn := conn.(*kcp.KCPConnection)
	if err != nil {
		log.Printf("KCP client disconnected with error: %v", err)
	} else {
		log.Printf("KCP client disconnected: %s", conn.RemoteAddr())
	}

	// 移除玩家
	h.gameServer.mu.Lock()
	if player, exists := h.gameServer.players[kcpConn.ID()]; exists {
		delete(h.gameServer.players, kcpConn.ID())
		h.gameServer.mu.Unlock()

		// 广播玩家离开消息
		h.broadcastPlayerLeave(player)
	} else {
		h.gameServer.mu.Unlock()
	}
}

// handlePing 处理ping消息
func (h *KCPHandler) handlePing(conn *kcp.KCPConnection, gameMsg GameMessage) {
	pongMsg := GameMessage{
		Type:      "pong",
		PlayerID:  conn.ID(),
		Data:      gameMsg.Data,
		Timestamp: time.Now().Unix(),
	}

	h.sendGameMessage(conn, pongMsg)
}

// handlePositionUpdate 处理位置更新
func (h *KCPHandler) handlePositionUpdate(conn *kcp.KCPConnection, gameMsg GameMessage) {
	positionData, ok := gameMsg.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid position data")
		return
	}

	position := PlayerPosition{
		X: getFloat64(positionData, "x"),
		Y: getFloat64(positionData, "y"),
		Z: getFloat64(positionData, "z"),
	}

	// 更新玩家位置
	h.gameServer.mu.Lock()
	if player, exists := h.gameServer.players[conn.ID()]; exists {
		player.Position = position
		player.LastSeen = time.Now()
	}
	h.gameServer.mu.Unlock()

	// 广播位置更新给其他玩家
	h.broadcastPositionUpdate(conn.ID(), position)
}

// handlePlayerAction 处理玩家动作
func (h *KCPHandler) handlePlayerAction(conn *kcp.KCPConnection, gameMsg GameMessage) {
	actionData, ok := gameMsg.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid action data")
		return
	}

	action := PlayerAction{
		Action: getString(actionData, "action"),
		Target: getString(actionData, "target"),
		Params: actionData["params"],
	}

	log.Printf("Player %s performed action: %s", conn.ID(), action.Action)

	// 广播动作给其他玩家
	h.broadcastPlayerAction(conn.ID(), action)
}

// handleChat 处理聊天消息
func (h *KCPHandler) handleChat(conn *kcp.KCPConnection, gameMsg GameMessage) {
	chatData, ok := gameMsg.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid chat data")
		return
	}

	message := getString(chatData, "message")
	log.Printf("Chat from %s: %s", conn.ID(), message)

	// 广播聊天消息
	chatMsg := GameMessage{
		Type:      "chat",
		PlayerID:  conn.ID(),
		Data:      map[string]interface{}{"message": message, "player_name": h.getPlayerName(conn.ID())},
		Timestamp: time.Now().Unix(),
	}

	h.broadcastToAll(chatMsg)
}

// handleGetPlayers 处理获取玩家列表请求
func (h *KCPHandler) handleGetPlayers(conn *kcp.KCPConnection) {
	h.gameServer.mu.RLock()
	players := make([]*Player, 0, len(h.gameServer.players))
	for _, player := range h.gameServer.players {
		// 创建副本，不包含连接信息
		playerCopy := &Player{
			ID:       player.ID,
			Name:     player.Name,
			Position: player.Position,
			LastSeen: player.LastSeen,
			Stats:    player.Stats,
		}
		players = append(players, playerCopy)
	}
	h.gameServer.mu.RUnlock()

	playersMsg := GameMessage{
		Type:      "players_list",
		PlayerID:  conn.ID(),
		Data:      map[string]interface{}{"players": players},
		Timestamp: time.Now().Unix(),
	}

	h.sendGameMessage(conn, playersMsg)
}

// 广播相关方法
func (h *KCPHandler) broadcastPlayerJoin(player *Player) {
	joinMsg := GameMessage{
		Type:      "player_join",
		PlayerID:  player.ID,
		Data:      map[string]interface{}{"player": player},
		Timestamp: time.Now().Unix(),
	}
	h.broadcastToOthers(player.ID, joinMsg)
}

func (h *KCPHandler) broadcastPlayerLeave(player *Player) {
	leaveMsg := GameMessage{
		Type:      "player_leave",
		PlayerID:  player.ID,
		Data:      map[string]interface{}{"player_id": player.ID, "player_name": player.Name},
		Timestamp: time.Now().Unix(),
	}
	h.broadcastToAll(leaveMsg)
}

func (h *KCPHandler) broadcastPositionUpdate(playerID string, position PlayerPosition) {
	posMsg := GameMessage{
		Type:      "position_update",
		PlayerID:  playerID,
		Data:      map[string]interface{}{"position": position},
		Timestamp: time.Now().Unix(),
	}
	h.broadcastToOthers(playerID, posMsg)
}

func (h *KCPHandler) broadcastPlayerAction(playerID string, action PlayerAction) {
	actionMsg := GameMessage{
		Type:      "player_action",
		PlayerID:  playerID,
		Data:      map[string]interface{}{"action": action},
		Timestamp: time.Now().Unix(),
	}
	h.broadcastToOthers(playerID, actionMsg)
}

func (h *KCPHandler) broadcastToAll(gameMsg GameMessage) {
	h.gameServer.mu.RLock()
	defer h.gameServer.mu.RUnlock()

	for _, player := range h.gameServer.players {
		if player.Connection != nil && player.Connection.IsActive() {
			h.sendGameMessage(player.Connection, gameMsg)
		}
	}
}

func (h *KCPHandler) broadcastToOthers(excludePlayerID string, gameMsg GameMessage) {
	h.gameServer.mu.RLock()
	defer h.gameServer.mu.RUnlock()

	for playerID, player := range h.gameServer.players {
		if playerID != excludePlayerID && player.Connection != nil && player.Connection.IsActive() {
			h.sendGameMessage(player.Connection, gameMsg)
		}
	}
}

func (h *KCPHandler) sendGameMessage(conn *kcp.KCPConnection, gameMsg GameMessage) {
	data, err := json.Marshal(gameMsg)
	if err != nil {
		log.Printf("Failed to marshal game message: %v", err)
		return
	}

	msg := core.Message{
		Type:      core.MessageTypeJSON,
		Data:      data,
		Timestamp: time.Now(),
	}

	if err := conn.SendMessage(msg); err != nil {
		log.Printf("Failed to send game message: %v", err)
	}
}

func (h *KCPHandler) getPlayerName(playerID string) string {
	h.gameServer.mu.RLock()
	defer h.gameServer.mu.RUnlock()

	if player, exists := h.gameServer.players[playerID]; exists {
		return player.Name
	}
	return "Unknown"
}

// 辅助函数
func getFloat64(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}

func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func main() {
	log.Println("Starting KCP Game Server example...")

	// 创建游戏服务器
	gameServer := NewGameServer()

	// 创建KCP配置
	kcpConfig := kcp.DefaultKCPConfig()
	kcpConfig.Nodelay = 1   // 启用nodelay模式，适合游戏
	kcpConfig.Interval = 10 // 10ms间隔，低延迟
	kcpConfig.Resend = 2    // 快速重传
	kcpConfig.NC = 1        // 关闭流控
	kcpConfig.SndWnd = 256  // 增大发送窗口
	kcpConfig.RcvWnd = 256  // 增大接收窗口

	// 创建KCP服务器
	server := netcore.NewKCPServer(
		netcore.WithMaxConnections(1000),
		netcore.WithReadBufferSize(8192),
		netcore.WithWriteBufferSize(8192),
		netcore.WithConnectionPool(true),
		netcore.WithMemoryPool(true),
		netcore.WithGoroutinePool(true),
	)

	// 设置KCP配置
	server.SetKCPConfig(kcpConfig)

	// 设置消息处理器
	handler := &KCPHandler{gameServer: gameServer}
	server.SetHandler(handler)
	gameServer.server = server

	// 启动服务器
	go func() {
		log.Println("KCP Game Server listening on :8085")
		if err := server.Start(":8085"); err != nil {
			log.Fatalf("Failed to start KCP server: %v", err)
		}
	}()

	// 等待一段时间让服务器启动
	time.Sleep(1 * time.Second)

	// 打印服务器信息
	log.Println("\n=== KCP Game Server Information ===")
	log.Println("Server Address: localhost:8085")
	log.Println("Protocol: KCP (UDP-based reliable transport)")
	log.Println("Game Features:")
	log.Println("  - Real-time player position updates")
	log.Println("  - Player actions and interactions")
	log.Println("  - Chat system")
	log.Println("  - Player list management")
	log.Println("  - Ping/Pong heartbeat")
	log.Println("KCP Configuration:")
	log.Printf("  - Nodelay: %d (Low latency mode)\n", kcpConfig.Nodelay)
	log.Printf("  - Interval: %dms\n", kcpConfig.Interval)
	log.Printf("  - Window Size: %d/%d (Send/Recv)\n", kcpConfig.SndWnd, kcpConfig.RcvWnd)
	log.Printf("  - MTU: %d bytes\n", kcpConfig.MTU)
	log.Println("Optimizations: Connection Pool, Memory Pool, Goroutine Pool")
	log.Println("=======================================\n")

	// 定期打印服务器统计信息和游戏状态
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := server.GetStats()
				kcpStats := server.GetKCPStats()

				log.Printf("\n=== Server Stats ===")
				log.Printf("Active Connections: %d", stats.ActiveConnections)
				log.Printf("Total Connections: %d", stats.TotalConnections)
				log.Printf("Messages Received: %d", stats.MessagesReceived)
				log.Printf("Messages Sent: %d", stats.MessagesSent)
				log.Printf("Bytes Received: %d", stats.BytesReceived)
				log.Printf("Bytes Sent: %d", stats.BytesSent)
				log.Printf("Uptime: %v", time.Since(stats.StartTime))

				if activeConns, ok := kcpStats["active_connections"].(int); ok {
					log.Printf("KCP Active Connections: %d", activeConns)
				}

				// 游戏统计
				gameServer.mu.RLock()
				playerCount := len(gameServer.players)
				gameServer.mu.RUnlock()

				log.Printf("Active Players: %d", playerCount)

				if stats.ErrorCount > 0 {
					log.Printf("Error Count: %d", stats.ErrorCount)
					log.Printf("Last Error: %s", stats.LastError)
				}
				log.Printf("===================\n")
			}
		}
	}()

	// 定期清理不活跃的玩家
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				gameServer.mu.Lock()
				for playerID, player := range gameServer.players {
					if time.Since(player.LastSeen) > 5*time.Minute {
						log.Printf("Removing inactive player: %s", playerID)
						delete(gameServer.players, playerID)
						if player.Connection != nil {
							player.Connection.Close()
						}
					}
				}
				gameServer.mu.Unlock()
			}
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("KCP Game Server is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("\nShutting down KCP Game Server...")

	// 关闭服务器
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	} else {
		log.Println("KCP Game Server stopped successfully")
	}

	log.Println("Goodbye!")
}
