package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	queues        map[string][]string           // chatID -> queued user messages
	queueMu       sync.Mutex
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
		queues:        make(map[string][]string),
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

		// If agent is already running, queue the message instead of canceling
		c.hub.agentMu.Lock()
		_, isRunning := c.hub.cancels[payload.ChatID]
		c.hub.agentMu.Unlock()
		if isRunning {
			c.hub.queueMessage(payload.ChatID, payload.Content)
		} else {
			go c.hub.handleSendMessage(payload.ChatID, payload.Content)
		}

	case "cancel":
		var payload struct {
			ChatID string `json:"chat_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.ChatID != "" {
			c.hub.handleCancel(payload.ChatID)
		}

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

	case "resume":
		var payload struct {
			ChatID string `json:"chat_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.ChatID != "" {
			go c.hub.handleResume(payload.ChatID)
		}

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

// queueMessage saves a user message to the DB, broadcasts it, and adds it to the in-memory queue.
func (h *Hub) queueMessage(chatID, content string) {
	db := database.Get()

	// Initialize counter from DB
	counter := h.getCounter(chatID)

	// Save user message to DB immediately so it appears in chat
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
	h.broadcast(chatID, WSMessage{Type: "message", Payload: userMsgData})

	// Add to queue
	h.queueMu.Lock()
	h.queues[chatID] = append(h.queues[chatID], content)
	h.queueMu.Unlock()

	// Broadcast queue size update
	h.queueMu.Lock()
	queueLen := len(h.queues[chatID])
	h.queueMu.Unlock()
	queueData, _ := json.Marshal(map[string]any{"id": chatID, "queue_size": queueLen})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: queueData})
}

// popQueue removes and returns the next queued message, or "" if empty.
func (h *Hub) popQueue(chatID string) string {
	h.queueMu.Lock()
	defer h.queueMu.Unlock()
	q := h.queues[chatID]
	if len(q) == 0 {
		return ""
	}
	msg := q[0]
	h.queues[chatID] = q[1:]
	return msg
}

// clearQueue removes all queued messages for a chat.
func (h *Hub) clearQueue(chatID string) {
	h.queueMu.Lock()
	delete(h.queues, chatID)
	h.queueMu.Unlock()
}

// handleCancel stops the current agent run and clears the message queue.
func (h *Hub) handleCancel(chatID string) {
	// Clear message queue first
	h.clearQueue(chatID)

	h.agentMu.Lock()
	agentInstance, hasAgent := h.agents[chatID]
	cancel, hasCancel := h.cancels[chatID]
	if !hasCancel {
		h.agentMu.Unlock()
		// Broadcast to clear any stale queue UI state
		updateData, _ := json.Marshal(map[string]any{"id": chatID, "running": false, "paused": false, "queue_size": 0})
		h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: updateData})
		return
	}

	// Set cancelled flag BEFORE canceling so the agent returns ErrCancelled
	if hasAgent {
		agentInstance.SetCancelled(true)
		agentInstance.CancelTools() // Stop any in-progress tool execution
	}
	cancel()
	delete(h.cancels, chatID)
	h.agentMu.Unlock()

	// Update DB and broadcast cancelled state
	db := database.Get()
	db.Model(&models.Chat{}).Where("id = ?", chatID).Update("running", false)

	updateData, _ := json.Marshal(map[string]any{"id": chatID, "running": false, "paused": false, "queue_size": 0})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: updateData})
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

	h.agentMu.Lock()

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
				Type:    "file_event",
				Payload: eventData,
			})
		}
		// Cache the mapping
		h.chatToProject[chatID] = chat.ProjectID
	}

	// Clear any previous pause/cancelled state when sending a new message
	agentInstance.SetPaused(false)
	agentInstance.SetCancelled(false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	h.cancels[chatID] = cancel
	h.agentMu.Unlock()

	// Run agent
	_, err := agentInstance.Run(ctx, content)
	cancel()

	// Clean up cancel from map
	h.agentMu.Lock()
	delete(h.cancels, chatID)
	h.agentMu.Unlock()

	// If paused, handlePause already updated the state — exit quietly
	if errors.Is(err, agent.ErrPaused) {
		return
	}

	// If cancelled, handleCancel already updated the state — exit quietly
	if errors.Is(err, agent.ErrCancelled) {
		agentInstance.SetCancelled(false)
		return
	}

	// Agent completed normally — clear any pause state
	agentInstance.SetPaused(false)

	// Check for queued messages before marking as not running
	if nextMsg := h.popQueue(chatID); nextMsg != "" {
		// Broadcast queue size update
		h.queueMu.Lock()
		queueLen := len(h.queues[chatID])
		h.queueMu.Unlock()
		queueData, _ := json.Marshal(map[string]any{"id": chatID, "queue_size": queueLen})
		h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: queueData})

		// Process next queued message (reuse goroutine)
		h.handleSendMessage(chatID, nextMsg)
		return
	}

	// Mark chat as not running
	db.Model(&chat).Update("running", false)
	chatDoneData, _ := json.Marshal(map[string]any{"id": chatID, "running": false, "queue_size": 0})
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

	result, err := h.llmClient.ChatCompletion(ctx, []llm.ChatMessage{
		{
			Role:    "system",
			Content: "You are a chat naming assistant. Respond with ONLY a short chat name (1-4 words) based on the user's message. No formatting, no quotes, no punctuation at the end. Use proper capitalization. Examples: Database Setup, Image Analysis, Code Review, Weather App",
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}, nil, nil)
	if err != nil {
		logger.Log.Debugw("auto-name failed", "chat_id", chatID, "error", err)
		return
	}

	name := result.Content
	if name == "" {
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
	agentInstance, hasAgent := h.agents[chatID]
	cancel, hasCancel := h.cancels[chatID]
	if !hasCancel {
		h.agentMu.Unlock()
		return // Nothing running to pause
	}

	// Set paused flag BEFORE canceling so the agent returns ErrPaused (not a raw error)
	if hasAgent {
		agentInstance.SetPaused(true)
	}
	cancel()
	delete(h.cancels, chatID)
	h.agentMu.Unlock()

	// Best-effort slot save to preserve KV cache for faster resume
	slotFilename := fmt.Sprintf("zeroloop_pause_%s", chatID)
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotSave(saveCtx, 0, slotFilename); err != nil {
		logger.Log.Debugw("slot save skipped (best-effort)", "chat_id", chatID, "error", err)
	}
	saveCancel()

	// Update DB and broadcast paused state
	db := database.Get()
	db.Model(&models.Chat{}).Where("id = ?", chatID).Update("running", false)

	chatUpdateData, _ := json.Marshal(map[string]any{"id": chatID, "running": false, "paused": true})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: chatUpdateData})
}

func (h *Hub) handleResume(chatID string) {
	h.agentMu.Lock()
	agentInstance, hasAgent := h.agents[chatID]
	if !hasAgent || !agentInstance.IsPaused() {
		h.agentMu.Unlock()
		return // Nothing to resume
	}

	// Cancel any stale context (shouldn't exist, but be safe)
	if cancel, ok := h.cancels[chatID]; ok {
		cancel()
	}

	// Ensure counter is initialized
	counter := h.counters[chatID]
	if counter == nil {
		counter = &atomic.Int64{}
		h.counters[chatID] = counter
		var msgCount int64
		database.Get().Model(&models.Message{}).Where("chat_id = ?", chatID).Count(&msgCount)
		counter.Store(msgCount)
	}

	// Refresh log callback with current counter
	agentInstance.OnLog = h.createLogCallback(chatID, counter)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	h.cancels[chatID] = cancel
	h.agentMu.Unlock()

	// Best-effort slot restore for faster prompt processing
	slotFilename := fmt.Sprintf("zeroloop_pause_%s", chatID)
	restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotRestore(restoreCtx, 0, slotFilename); err != nil {
		logger.Log.Debugw("slot restore skipped (best-effort)", "chat_id", chatID, "error", err)
	}
	restoreCancel()

	// Mark chat as running
	db := database.Get()
	db.Model(&models.Chat{}).Where("id = ?", chatID).Update("running", true)
	chatUpdateData, _ := json.Marshal(map[string]any{"id": chatID, "running": true, "paused": false})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: chatUpdateData})

	// Continue agent loop from where it was paused
	_, err := agentInstance.Continue(ctx)
	cancel()

	// Clean up cancel from map
	h.agentMu.Lock()
	delete(h.cancels, chatID)
	h.agentMu.Unlock()

	// If paused again, exit quietly
	if errors.Is(err, agent.ErrPaused) {
		return
	}

	// Agent completed — clear pause state and erase saved slot
	agentInstance.SetPaused(false)
	eraseCtx, eraseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotErase(eraseCtx, 0); err != nil {
		logger.Log.Debugw("slot erase skipped (best-effort)", "chat_id", chatID, "error", err)
	}
	eraseCancel()

	db.Model(&models.Chat{}).Where("id = ?", chatID).Update("running", false)
	chatDoneData, _ := json.Marshal(map[string]any{"id": chatID, "running": false})
	h.broadcast(chatID, WSMessage{Type: "chat_update", Payload: chatDoneData})

	if err != nil {
		logger.Log.Errorw("agent resume failed", "chat_id", chatID, "error", err)
		errMsgData, _ := json.Marshal(map[string]any{
			"chat_id": chatID,
			"type":    "error",
			"heading": "Agent Error",
			"content": err.Error(),
		})
		h.broadcast(chatID, WSMessage{Type: "message", Payload: errMsgData})
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
	h.clearQueue(chatID)

	h.agentMu.Lock()
	if agentInstance, ok := h.agents[chatID]; ok {
		agentInstance.SetCancelled(true)
		agentInstance.CancelTools()
	}
	if cancel, ok := h.cancels[chatID]; ok {
		cancel()
		delete(h.cancels, chatID)
	}
	delete(h.agents, chatID)
	delete(h.counters, chatID)
	h.agentMu.Unlock()

	// Best-effort slot erase to free server memory
	eraseCtx, eraseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotErase(eraseCtx, 0); err != nil {
		logger.Log.Debugw("slot erase skipped (best-effort)", "chat_id", chatID, "error", err)
	}
	eraseCancel()
}
