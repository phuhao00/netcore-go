// Package main KCP客户端示例
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/netcore-go/pkg/core"
	"github.com/netcore-go/pkg/kcp"
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

// GameClient 游戏客户端
type GameClient struct {
	client     *kcp.KCPClient
	playerID   string
	playerName string
	position   PlayerPosition
	connected  bool
	lastPing   time.Time
	pingCount  int
}

// NewGameClient 创建游戏客户端
func NewGameClient(address string) *GameClient {
	client := &GameClient{
		position: PlayerPosition{X: 0, Y: 0, Z: 0},
	}
	
	// 创建KCP客户端
	kcpConfig := kcp.DefaultKCPConfig()
	kcpConfig.Nodelay = 1   // 启用nodelay模式
	kcpConfig.Interval = 10 // 10ms间隔
	kcpConfig.Resend = 2    // 快速重传
	kcpConfig.NC = 1        // 关闭流控
	
	client.client = kcp.NewKCPClient(address,
		kcp.WithKCPConfig(kcpConfig),
		kcp.WithClientTimeout(30*time.Second),
		kcp.WithClientRetryCount(3),
		kcp.WithOnMessage(client.onMessage),
		kcp.WithOnConnect(client.onConnect),
		kcp.WithOnDisconnect(client.onDisconnect),
	)
	
	return client
}

// onConnect 连接建立回调
func (c *GameClient) onConnect() {
	c.connected = true
	log.Println("Connected to KCP Game Server!")
	
	// 获取玩家ID
	if conn := c.client.GetConnection(); conn != nil {
		c.playerID = conn.ID()
		c.playerName = fmt.Sprintf("Player_%s", c.playerID[:8])
	}
	
	// 开始定期发送ping
	go c.pingLoop()
	
	// 开始随机移动（模拟游戏行为）
	go c.randomMovement()
}

// onDisconnect 连接断开回调
func (c *GameClient) onDisconnect(err error) {
	c.connected = false
	if err != nil {
		log.Printf("Disconnected from server with error: %v", err)
	} else {
		log.Println("Disconnected from server")
	}
}

// onMessage 消息处理回调
func (c *GameClient) onMessage(msg core.Message) {
	var gameMsg GameMessage
	if err := json.Unmarshal(msg.Data, &gameMsg); err != nil {
		log.Printf("Failed to parse game message: %v", err)
		return
	}
	
	// 处理不同类型的消息
	switch gameMsg.Type {
	case "welcome":
		c.handleWelcome(gameMsg)
	case "pong":
		c.handlePong(gameMsg)
	case "player_join":
		c.handlePlayerJoin(gameMsg)
	case "player_leave":
		c.handlePlayerLeave(gameMsg)
	case "position_update":
		c.handlePositionUpdate(gameMsg)
	case "player_action":
		c.handlePlayerAction(gameMsg)
	case "chat":
		c.handleChat(gameMsg)
	case "players_list":
		c.handlePlayersList(gameMsg)
	default:
		log.Printf("Unknown message type: %s", gameMsg.Type)
	}
}

// 消息处理方法
func (c *GameClient) handleWelcome(gameMsg GameMessage) {
	if data, ok := gameMsg.Data.(map[string]interface{}); ok {
		if message, ok := data["message"].(string); ok {
			log.Printf("Server: %s", message)
		}
	}
}

func (c *GameClient) handlePong(gameMsg GameMessage) {
	latency := time.Since(c.lastPing)
	log.Printf("Pong received - Latency: %v", latency)
}

func (c *GameClient) handlePlayerJoin(gameMsg GameMessage) {
	log.Printf("Player joined: %s", gameMsg.PlayerID)
}

func (c *GameClient) handlePlayerLeave(gameMsg GameMessage) {
	if data, ok := gameMsg.Data.(map[string]interface{}); ok {
		if playerName, ok := data["player_name"].(string); ok {
			log.Printf("Player left: %s (%s)", playerName, gameMsg.PlayerID)
		}
	}
}

func (c *GameClient) handlePositionUpdate(gameMsg GameMessage) {
	if data, ok := gameMsg.Data.(map[string]interface{}); ok {
		if posData, ok := data["position"].(map[string]interface{}); ok {
			x := getFloat64(posData, "x")
			y := getFloat64(posData, "y")
			z := getFloat64(posData, "z")
			log.Printf("Player %s moved to (%.2f, %.2f, %.2f)", gameMsg.PlayerID[:8], x, y, z)
		}
	}
}

func (c *GameClient) handlePlayerAction(gameMsg GameMessage) {
	if data, ok := gameMsg.Data.(map[string]interface{}); ok {
		if actionData, ok := data["action"].(map[string]interface{}); ok {
			action := getString(actionData, "action")
			log.Printf("Player %s performed action: %s", gameMsg.PlayerID[:8], action)
		}
	}
}

func (c *GameClient) handleChat(gameMsg GameMessage) {
	if data, ok := gameMsg.Data.(map[string]interface{}); ok {
		message := getString(data, "message")
		playerName := getString(data, "player_name")
		log.Printf("[Chat] %s: %s", playerName, message)
	}
}

func (c *GameClient) handlePlayersList(gameMsg GameMessage) {
	if data, ok := gameMsg.Data.(map[string]interface{}); ok {
		if playersData, ok := data["players"].([]interface{}); ok {
			log.Printf("\n=== Players Online (%d) ===", len(playersData))
			for _, playerData := range playersData {
				if player, ok := playerData.(map[string]interface{}); ok {
					id := getString(player, "id")
					name := getString(player, "name")
					if posData, ok := player["position"].(map[string]interface{}); ok {
						x := getFloat64(posData, "x")
						y := getFloat64(posData, "y")
						z := getFloat64(posData, "z")
						log.Printf("  - %s (%s): (%.2f, %.2f, %.2f)", name, id[:8], x, y, z)
					}
				}
			}
			log.Println("=========================")
		}
	}
}

// 游戏操作方法
func (c *GameClient) sendPing() {
	c.lastPing = time.Now()
	c.pingCount++
	
	pingMsg := GameMessage{
		Type:      "ping",
		PlayerID:  c.playerID,
		Data:      map[string]interface{}{"count": c.pingCount},
		Timestamp: time.Now().Unix(),
	}
	
	c.sendGameMessage(pingMsg)
}

func (c *GameClient) updatePosition(x, y, z float64) {
	c.position.X = x
	c.position.Y = y
	c.position.Z = z
	
	posMsg := GameMessage{
		Type:      "position_update",
		PlayerID:  c.playerID,
		Data:      map[string]interface{}{"x": x, "y": y, "z": z},
		Timestamp: time.Now().Unix(),
	}
	
	c.sendGameMessage(posMsg)
}

func (c *GameClient) performAction(action, target string, params interface{}) {
	actionMsg := GameMessage{
		Type:     "player_action",
		PlayerID: c.playerID,
		Data: map[string]interface{}{
			"action": action,
			"target": target,
			"params": params,
		},
		Timestamp: time.Now().Unix(),
	}
	
	c.sendGameMessage(actionMsg)
}

func (c *GameClient) sendChat(message string) {
	chatMsg := GameMessage{
		Type:      "chat",
		PlayerID:  c.playerID,
		Data:      map[string]interface{}{"message": message},
		Timestamp: time.Now().Unix(),
	}
	
	c.sendGameMessage(chatMsg)
}

func (c *GameClient) getPlayers() {
	playersMsg := GameMessage{
		Type:      "get_players",
		PlayerID:  c.playerID,
		Data:      map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}
	
	c.sendGameMessage(playersMsg)
}

func (c *GameClient) sendGameMessage(gameMsg GameMessage) {
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
	
	if err := c.client.SendMessage(msg); err != nil {
		log.Printf("Failed to send game message: %v", err)
	}
}

// 后台任务
func (c *GameClient) pingLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if c.connected {
				c.sendPing()
			}
		}
	}
}

func (c *GameClient) randomMovement() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if c.connected {
				// 随机移动
				x := c.position.X + (rand.Float64()-0.5)*10
				y := c.position.Y + (rand.Float64()-0.5)*10
				z := c.position.Z + (rand.Float64()-0.5)*2
				c.updatePosition(x, y, z)
				
				// 随机执行动作
				if rand.Float64() < 0.3 {
					actions := []string{"jump", "attack", "defend", "use_item", "cast_spell"}
					action := actions[rand.Intn(len(actions))]
					c.performAction(action, "", nil)
				}
			}
		}
	}
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
	log.Println("Starting KCP Game Client example...")
	
	// 创建游戏客户端
	client := NewGameClient("localhost:8085")
	
	defer func() {
		if err := client.client.Close(); err != nil {
			log.Printf("Error closing client: %v", err)
		}
	}()
	
	// 连接到服务器
	log.Println("Connecting to KCP Game Server...")
	if err := client.client.Connect(); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	
	// 等待连接建立
	time.Sleep(2 * time.Second)
	
	if !client.connected {
		log.Fatal("Failed to establish connection")
	}
	
	// 打印客户端信息
	log.Println("\n=== KCP Game Client Information ===")
	log.Printf("Player ID: %s", client.playerID)
	log.Printf("Player Name: %s", client.playerName)
	log.Println("Server Address: localhost:8085")
	log.Println("Protocol: KCP (UDP-based reliable transport)")
	log.Println("\nAvailable Commands:")
	log.Println("  ping          - Send ping to server")
	log.Println("  move <x> <y> <z> - Move to position")
	log.Println("  action <name> - Perform action")
	log.Println("  chat <message> - Send chat message")
	log.Println("  players       - Get players list")
	log.Println("  stats         - Show connection stats")
	log.Println("  auto          - Toggle auto movement")
	log.Println("  quit          - Exit client")
	log.Println("=====================================\n")
	
	// 获取玩家列表
	client.getPlayers()
	
	// 命令行交互
	scanner := bufio.NewScanner(os.Stdin)
	autoMovement := true
	
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		parts := strings.Fields(input)
		command := parts[0]
		
		switch command {
		case "ping":
			client.sendPing()
			
		case "move":
			if len(parts) >= 4 {
				x, _ := strconv.ParseFloat(parts[1], 64)
				y, _ := strconv.ParseFloat(parts[2], 64)
				z, _ := strconv.ParseFloat(parts[3], 64)
				client.updatePosition(x, y, z)
				log.Printf("Moved to (%.2f, %.2f, %.2f)", x, y, z)
			} else {
				log.Println("Usage: move <x> <y> <z>")
			}
			
		case "action":
			if len(parts) >= 2 {
				action := parts[1]
				client.performAction(action, "", nil)
				log.Printf("Performed action: %s", action)
			} else {
				log.Println("Usage: action <name>")
			}
			
		case "chat":
			if len(parts) >= 2 {
				message := strings.Join(parts[1:], " ")
				client.sendChat(message)
			} else {
				log.Println("Usage: chat <message>")
			}
			
		case "players":
			client.getPlayers()
			
		case "stats":
			if stats := client.client.GetStats(); stats != nil {
				log.Printf("\n=== Connection Stats ===")
				log.Printf("Connected At: %v", stats.ConnectedAt)
				log.Printf("Last Active: %v", stats.LastActive)
				log.Printf("Messages Sent: %d", stats.MessagesSent)
				log.Printf("Messages Received: %d", stats.MessagesReceived)
				log.Printf("Bytes Sent: %d", stats.BytesSent)
				log.Printf("Bytes Received: %d", stats.BytesReceived)
				log.Printf("Error Count: %d", stats.ErrorCount)
				if stats.LastError != "" {
					log.Printf("Last Error: %s", stats.LastError)
				}
				log.Println("========================")
			}
			
			if kcpStats := client.client.GetKCPStats(); kcpStats != nil {
				log.Printf("\n=== KCP Stats ===")
				for key, value := range kcpStats {
					log.Printf("%s: %v", key, value)
				}
				log.Println("=================")
			}
			
		case "auto":
			autoMovement = !autoMovement
			if autoMovement {
				log.Println("Auto movement enabled")
				go client.randomMovement()
			} else {
				log.Println("Auto movement disabled")
			}
			
		case "quit", "exit":
			log.Println("Goodbye!")
			return
			
		default:
			log.Printf("Unknown command: %s", command)
			log.Println("Type 'quit' to exit or use available commands")
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}

