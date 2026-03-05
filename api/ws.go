package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/filemanager"
	"github.com/expki/ZeroLoop.git/llm"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/models"
	"github.com/expki/ZeroLoop.git/search"
	"github.com/expki/ZeroLoop.git/tools"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WSMessage is the envelope for all WebSocket communication
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client represents a single WebSocket connection
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	chatID    string
	projectID string
	mu        sync.Mutex
}

// Hub manages WebSocket connections and agent contexts
type Hub struct {
	clients       map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	mu            sync.RWMutex
	agents        map[string]*agent.Agent      // chatID -> agent (preserved across messages)
	agentMu       sync.Mutex
	cancels       map[string]context.CancelFunc // chatID -> cancel function for current run
	counters      map[string]*atomic.Int64      // chatID -> message number counter
	chatToProject map[string]string             // chatID -> projectID cache
	llmClient     *llm.Client
	fileManager   *filemanager.FileManager
	tools         *agent.ToolRegistry
}

// NewHub creates a new Hub
func NewHub(llmClient *llm.Client, fm *filemanager.FileManager) *Hub {
	registry := agent.NewToolRegistry()
	registry.Register(&tools.ResponseTool{})
	registry.Register(&tools.CodeExecutionTool{})
	registry.Register(&tools.CallSubordinateTool{})
	registry.Register(&tools.KnowledgeTool{})
	registry.Register(&tools.MemoryTool{})
	registry.Register(&tools.TextEditorTool{FM: fm})
	registry.Register(&tools.InputTool{})
	registry.Register(&tools.WebSearchTool{})
	registry.Register(&tools.BrowserTool{})
	registry.Register(&tools.DocumentQueryTool{})
	registry.Register(&tools.WaitTool{})
	registry.Register(&tools.NotifyUserTool{})
	registry.Register(&tools.SkillsTool{})
	registry.Register(&tools.SchedulerTool{})
	registry.Register(&tools.BehaviourAdjustmentTool{})
	registry.Register(&tools.VisionTool{})

	return &Hub{
		clients:       make(map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		agents:        make(map[string]*agent.Agent),
		cancels:       make(map[string]context.CancelFunc),
		counters:      make(map[string]*atomic.Int64),
		chatToProject: make(map[string]string),
		llmClient:     llmClient,
		fileManager:   fm,
		tools:         registry,
	}
}

// Run starts the hub event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.Log.Debugw("client connected", "clients", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			logger.Log.Debugw("client disconnected", "clients", len(h.clients))
		}
	}
}

// broadcast sends a message to all clients subscribed to a specific chat
func (h *Hub) broadcast(chatID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.chatID == chatID {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// broadcastToProject sends a message to all clients subscribed to any chat in the given project
func (h *Hub) broadcastToProject(projectID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.projectID == projectID {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// HandleWebSocket handles WebSocket upgrade and message loop
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Errorw("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1024 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Log.Warnw("websocket read error", "error", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Log.Warnw("invalid websocket message", "error", err)
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg WSMessage) {
	switch msg.Type {
	case "subscribe":
		var payload struct {
			ChatID string `json:"chat_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.mu.Lock()
		c.chatID = payload.ChatID
		c.mu.Unlock()
		// Look up project for this chat (cache first, then DB)
		c.hub.agentMu.Lock()
		if pid, ok := c.hub.chatToProject[payload.ChatID]; ok {
			c.mu.Lock()
			c.projectID = pid
			c.mu.Unlock()
		} else {
			var chat models.Chat
			if err := database.Get().Select("project_id").First(&chat, "id = ?", payload.ChatID).Error; err == nil && chat.ProjectID != "" {
				c.mu.Lock()
				c.projectID = chat.ProjectID
				c.mu.Unlock()
				c.hub.chatToProject[payload.ChatID] = chat.ProjectID
			}
		}
		c.hub.agentMu.Unlock()

	case "send_message":
		var payload struct {
			ChatID  string `json:"chat_id"`
			Content string `json:"content"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.ChatID == "" || payload.Content == "" {
			return
		}
		c.mu.Lock()
		c.chatID = payload.ChatID
		c.mu.Unlock()
		go c.hub.handleSendMessage(payload.ChatID, payload.Content)

	case "pause":
		var payload struct {
			ChatID string `json:"chat_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.hub.handlePause(payload.ChatID)

	case "clear":
		var payload struct {
			ChatID string `json:"chat_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.hub.handleClear(payload.ChatID)

	case "intervene":
		var payload struct {
			ChatID  string `json:"chat_id"`
			Content string `json:"content"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.ChatID != "" && payload.Content != "" {
			c.hub.handleIntervene(payload.ChatID, payload.Content)
		}
	}
}

// getCounter returns the atomic message counter for a chat, creating one if needed.
func (h *Hub) getCounter(chatID string) *atomic.Int64 {
	h.agentMu.Lock()
	defer h.agentMu.Unlock()
	counter, ok := h.counters[chatID]
	if !ok {
		counter = &atomic.Int64{}
		h.counters[chatID] = counter
	}
	return counter
}

// createLogCallback creates a log callback that uses the Hub's atomic counter
// for message numbering. This is safe to call multiple times for the same chat.
func (h *Hub) createLogCallback(chatID string, counter *atomic.Int64) agent.LogCallback {
	return func(entry agent.LogEntry) {
		var msgType models.MessageType
		switch entry.Type {
		case "agent":
			msgType = models.MessageTypeAgent
		case "response":
			msgType = models.MessageTypeResponse
		case "tool":
			msgType = models.MessageTypeTool
		case "code_exe":
			msgType = models.MessageTypeCodeExe
		case "error":
			msgType = models.MessageTypeError
		case "info":
			msgType = models.MessageTypeInfo
		default:
			msgType = models.MessageType(entry.Type)
		}

		if entry.Stream {
			streamData, _ := json.Marshal(map[string]any{
				"chat_id": chatID,
				"type":    entry.Type,
				"content": entry.Content,
				"agentno": entry.AgentNo,
				"stream":  true,
			})
			h.broadcast(chatID, WSMessage{Type: "stream", Payload: streamData})
			return
		}

		no := int(counter.Add(1))
		kvpsJSON := "{}"
		if entry.Kvps != nil {
			if b, err := json.Marshal(entry.Kvps); err == nil {
				kvpsJSON = string(b)
			}
		}

		msg := models.Message{
			ID:      uuid.New().String(),
			ChatID:  chatID,
			No:      no,
			Type:    msgType,
			Heading: entry.Heading,
			Content: entry.Content,
			Kvps:    kvpsJSON,
			AgentNo: entry.AgentNo,
		}
		database.Get().Create(&msg)

		// Index in search for knowledge tool
		if entry.Content != "" {
			_ = search.Index(search.Document{
				ID:      msg.ID,
				ChatID:  chatID,
				Content: entry.Content,
				Type:    string(msgType),
				Heading: entry.Heading,
			})
		}

		msgData, _ := json.Marshal(msg)
		h.broadcast(chatID, WSMessage{Type: "message", Payload: msgData})
	}
}

func (h *Hub) handleSendMessage(chatID, content string) {
	db := database.Get()

	// Verify chat exists
	var chat models.Chat
	if err := db.First(&chat, "id = ?", chatID).Error; err != nil {
		logger.Log.Errorw("chat not found", "chat_id", chatID)
		return
	}

	// Get current message count for numbering
	var msgCount int64
	db.Model(&models.Message{}).Where("chat_id = ?", chatID).Count(&msgCount)
	isFirstMessage := msgCount == 0

	// Initialize counter from DB
	counter := h.getCounter(chatID)
	counter.Store(msgCount)

	// Save user message
	userNo := int(counter.Add(1))
	userMsg := models.Message{
		ID:      uuid.New().String(),
		ChatID:  chatID,
		No:      userNo,
		Type:    models.MessageTypeUser,
		Content: content,
	}
	db.Create(&userMsg)

	// Broadcast user message to clients
	userMsgData, _ := json.Marshal(userMsg)
	h.broadcast(chatID, WSMessage{
		Type:    "message",
		Payload: userMsgData,
	})

	// Cancel any existing agent run for this chat
	h.agentMu.Lock()
	if cancel, ok := h.cancels[chatID]; ok {
		cancel()
	}

	// Mark chat as running
	db.Model(&chat).Update("running", true)
	chatUpdateData, _ := json.Marshal(map[string]any{"id": chatID, "running": true})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: chatUpdateData})

	// Create agent if needed, always refresh the log callback
	agentInstance, ok := h.agents[chatID]
	if !ok {
		agentInstance = agent.New(h.llmClient, h.tools, h.createLogCallback(chatID, counter))
		agentInstance.ChatID = chatID
		h.agents[chatID] = agentInstance
	} else {
		// Refresh callback so it uses current counter
		agentInstance.OnLog = h.createLogCallback(chatID, counter)
	}

	// Set project context on agent if chat belongs to a project
	if chat.ProjectID != "" {
		agentInstance.ProjectID = chat.ProjectID
		agentInstance.ProjectDir = h.fileManager.ProjectDir(chat.ProjectID)
		agentInstance.FileEventCallback = func(event agent.FileEvent) {
			eventData, _ := json.Marshal(event)
			h.broadcastToProject(event.ProjectID, WSMessage{
				Type:    "file_" + event.Action,
				Payload: eventData,
			})
		}
		// Cache the mapping
		h.chatToProject[chatID] = chat.ProjectID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	h.cancels[chatID] = cancel
	h.agentMu.Unlock()

	// Run agent
	_, err := agentInstance.Run(ctx, content)
	cancel()

	// Mark chat as not running
	db.Model(&chat).Update("running", false)
	chatDoneData, _ := json.Marshal(map[string]any{"id": chatID, "running": false})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: chatDoneData})

	if err != nil {
		logger.Log.Errorw("agent run failed", "chat_id", chatID, "error", err)
		errMsgData, _ := json.Marshal(map[string]any{
			"chat_id": chatID,
			"type":    "error",
			"heading": "Agent Error",
			"content": err.Error(),
		})
		h.broadcast(chatID, WSMessage{Type: "message", Payload: errMsgData})
	}

	// Auto-name chat after first successful response
	if isFirstMessage && err == nil {
		go h.autoNameChat(chatID, content)
	}
}

// autoNameChat uses the LLM to generate a short chat name from the first message
func (h *Hub) autoNameChat(chatID, userMessage string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &llm.ChatCompletionRequest{
		Messages: []llm.ChatMessage{
			{
				Role:    "system",
				Content: "You are a chat naming assistant. Respond with ONLY a short chat name (1-4 words) based on the user's message. No formatting, no quotes, no punctuation at the end. Use proper capitalization. Examples: Database Setup, Image Analysis, Code Review, Weather App",
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	resp, err := h.llmClient.ChatCompletion(ctx, req)
	if err != nil {
		logger.Log.Debugw("auto-name failed", "chat_id", chatID, "error", err)
		return
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == nil {
		return
	}

	name, ok := resp.Choices[0].Message.Content.(string)
	if !ok || name == "" {
		return
	}

	// Clean up the name
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "\"'`")
	if len(name) > 50 {
		name = name[:50]
	}
	if name == "" {
		return
	}

	// Update in database
	db := database.Get()
	db.Model(&models.Chat{}).Where("id = ?", chatID).Update("name", name)

	// Broadcast update to clients
	updateData, _ := json.Marshal(map[string]any{
		"id":   chatID,
		"name": name,
	})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: updateData})
	logger.Log.Debugw("auto-named chat", "chat_id", chatID, "name", name)
}

func (h *Hub) handlePause(chatID string) {
	h.agentMu.Lock()
	defer h.agentMu.Unlock()
	if cancel, ok := h.cancels[chatID]; ok {
		cancel()
		delete(h.cancels, chatID)
		// Agent is preserved in h.agents so history is kept

		db := database.Get()
		db.Model(&models.Chat{}).Where("id = ?", chatID).Update("running", false)

		chatUpdateData, _ := json.Marshal(map[string]any{"id": chatID, "running": false})
		h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: chatUpdateData})
	}
}

func (h *Hub) handleClear(chatID string) {
	// Cancel running agent and clean up state
	h.cleanupChat(chatID)

	// Delete all messages for this chat
	db := database.Get()
	var msgs []models.Message
	db.Where("chat_id = ?", chatID).Select("id").Find(&msgs)
	for _, m := range msgs {
		_ = search.Delete(m.ID)
	}
	db.Where("chat_id = ?", chatID).Delete(&models.Message{})

	// Notify clients
	clearData, _ := json.Marshal(map[string]any{"chat_id": chatID})
	h.broadcast(chatID, WSMessage{Type: "clear", Payload: clearData})
}

func (h *Hub) handleIntervene(chatID, content string) {
	h.agentMu.Lock()
	agentInstance, ok := h.agents[chatID]
	h.agentMu.Unlock()

	if !ok {
		return // No agent running for this chat
	}

	agentInstance.Intervene(content)

	// Also save the intervention as a message in the DB
	counter := h.getCounter(chatID)
	no := int(counter.Add(1))
	msg := models.Message{
		ID:      uuid.New().String(),
		ChatID:  chatID,
		No:      no,
		Type:    models.MessageTypeUser,
		Heading: "Intervention",
		Content: content,
	}
	database.Get().Create(&msg)

	msgData, _ := json.Marshal(msg)
	h.broadcast(chatID, WSMessage{Type: "message", Payload: msgData})
}

// cleanupChat cancels any running agent and removes in-memory state for a chat.
// Does NOT broadcast to WS clients or touch the database.
func (h *Hub) cleanupChat(chatID string) {
	h.agentMu.Lock()
	if cancel, ok := h.cancels[chatID]; ok {
		cancel()
		delete(h.cancels, chatID)
	}
	delete(h.agents, chatID)
	delete(h.counters, chatID)
	h.agentMu.Unlock()
}
