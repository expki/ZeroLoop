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
	"github.com/expki/ZeroLoop.git/config"
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

// mustMarshal marshals v to JSON, panicking on error (for internal messages that must not fail).
func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic("json.Marshal failed: " + err.Error())
	}
	return data
}

// Client represents a single WebSocket connection
type Client struct {
	hub               *Hub
	conn              *websocket.Conn
	send              chan []byte
	agentID           string
	projectID         string
	subscribedProcess string // tracks which process this client is viewing
	mu                sync.Mutex
	closed            bool
}

// safeSend sends data to the client's send channel without panicking if it's closed.
func (c *Client) safeSend(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

// Hub manages WebSocket connections and agent contexts
type Hub struct {
	clients        map[*Client]bool
	register       chan *Client
	unregister     chan *Client
	mu             sync.RWMutex
	agents         map[string]*agent.Agent      // agentID -> agent (preserved across messages)
	agentMu        sync.Mutex
	cancels        map[string]context.CancelFunc // agentID -> cancel function for current run
	counters       map[string]*atomic.Int64      // agentID -> message number counter
	agentToProject map[string]string             // agentID -> projectID cache
	queues         map[string][]string           // agentID -> queued user messages
	queueMu        sync.Mutex
	childAgents    map[string][]string           // parentID -> []childAgentID
	childCancels   map[string]context.CancelFunc // childAgentID -> cancel
	llmSemaphore   chan struct{}                  // limits concurrent LLM calls
	llmClient       *llm.Client
	fileManager     *filemanager.FileManager
	tools           *agent.ToolRegistry
	terminalManager *TerminalManager
	processManager  *ProcessManager
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

	// Initialize LLM semaphore from config
	cfg := config.Get()
	var llmSem chan struct{}
	if cfg.LLMSlots > 0 {
		llmSem = make(chan struct{}, cfg.LLMSlots)
	}

	h := &Hub{
		clients:        make(map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		agents:         make(map[string]*agent.Agent),
		cancels:        make(map[string]context.CancelFunc),
		counters:       make(map[string]*atomic.Int64),
		agentToProject: make(map[string]string),
		queues:         make(map[string][]string),
		childAgents:    make(map[string][]string),
		childCancels:   make(map[string]context.CancelFunc),
		llmSemaphore:   llmSem,
		llmClient:       llmClient,
		fileManager:     fm,
		tools:           registry,
		terminalManager: NewTerminalManager(),
		processManager:  NewProcessManager(),
	}

	// Start process cleanup loop
	h.processManager.startCleanupLoop()

	// Register process tools with closures that capture the hub
	registry.Register(&tools.RunProcessTool{
		Starter: func(projectID, command, workDir string) (string, <-chan struct{}, error) {
			processID := GenerateProcessID()
			sess, err := h.processManager.Start(processID, projectID, command, workDir,
				// onOutput: send to subscribed clients only
				func(pid string, line LogLine) {
					outData, _ := json.Marshal(map[string]any{
						"process_id": pid,
						"stream":     line.Stream,
						"text":       line.Text,
						"timestamp":  line.Timestamp,
					})
					h.mu.RLock()
					for client := range h.clients {
						if client.subscribedProcess == pid {
							client.safeSend(mustMarshal(WSMessage{Type: "process_output", Payload: outData}))
						}
					}
					h.mu.RUnlock()
				},
				// onExit: broadcast to project
				func(pid string, exitCode int) {
					exitData, _ := json.Marshal(map[string]any{
						"process_id": pid,
						"exit_code":  exitCode,
					})
					h.broadcastToProject(projectID, WSMessage{Type: "process_exit", Payload: exitData})
				},
			)
			if err != nil {
				return "", nil, err
			}
			// Broadcast process_started to project
			startData, _ := json.Marshal(map[string]any{
				"process_id": processID,
				"project_id": projectID,
				"command":    command,
			})
			h.broadcastToProject(projectID, WSMessage{Type: "process_started", Payload: startData})
			return processID, sess.done, nil
		},
		Checker: func(processID string, tailLines int) (*tools.ProcessCheckResult, error) {
			sess, ok := h.processManager.Get(processID)
			if !ok {
				return nil, fmt.Errorf("process %s not found", processID)
			}
			info := sess.info()
			tailOutput := sess.buffer.Tail(tailLines)
			outputLines := make([]tools.ProcessOutputLine, len(tailOutput))
			for i, l := range tailOutput {
				outputLines[i] = tools.ProcessOutputLine{
					Stream: l.Stream,
					Text:   l.Text,
				}
			}
			return &tools.ProcessCheckResult{
				ID:       info.ID,
				Command:  info.Command,
				Status:   info.Status,
				ExitCode: info.ExitCode,
				Lines:    outputLines,
			}, nil
		},
	})

	registry.Register(&tools.CheckProcessTool{
		Checker: func(processID string, tailLines int) (*tools.ProcessCheckResult, error) {
			sess, ok := h.processManager.Get(processID)
			if !ok {
				return nil, fmt.Errorf("process %s not found", processID)
			}
			info := sess.info()
			tailOutput := sess.buffer.Tail(tailLines)
			lines := make([]tools.ProcessOutputLine, len(tailOutput))
			for i, l := range tailOutput {
				lines[i] = tools.ProcessOutputLine{Stream: l.Stream, Text: l.Text}
			}
			return &tools.ProcessCheckResult{
				ID:       info.ID,
				Command:  info.Command,
				Status:   info.Status,
				ExitCode: info.ExitCode,
				Lines:    lines,
			}, nil
		},
	})

	registry.Register(&tools.ListProcessesTool{
		Lister: func(projectID string) []tools.ProcessListItem {
			infos := h.processManager.ListByProject(projectID)
			items := make([]tools.ProcessListItem, len(infos))
			for i, info := range infos {
				items[i] = tools.ProcessListItem{
					ID:       info.ID,
					Command:  info.Command,
					Status:   info.Status,
					ExitCode: info.ExitCode,
				}
			}
			return items
		},
	})

	return h
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
				client.mu.Lock()
				client.closed = true
				close(client.send)
				client.mu.Unlock()
			}
			h.mu.Unlock()
			logger.Log.Debugw("client disconnected", "clients", len(h.clients))
		}
	}
}

// broadcast sends a message to all clients subscribed to a specific agent
func (h *Hub) broadcast(agentID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.agentID == agentID {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// broadcastToProject sends a message to all clients subscribed to any agent in the given project
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
			AgentID string `json:"agent_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.mu.Lock()
		c.agentID = payload.AgentID
		c.mu.Unlock()
		// Look up project for this agent (cache first, then DB)
		c.hub.agentMu.Lock()
		if pid, ok := c.hub.agentToProject[payload.AgentID]; ok {
			c.mu.Lock()
			c.projectID = pid
			c.mu.Unlock()
		} else {
			var agt models.Agent
			if err := database.Get().Select("project_id").First(&agt, "id = ?", payload.AgentID).Error; err == nil && agt.ProjectID != "" {
				c.mu.Lock()
				c.projectID = agt.ProjectID
				c.mu.Unlock()
				c.hub.agentToProject[payload.AgentID] = agt.ProjectID
			}
		}
		c.hub.agentMu.Unlock()

	case "send_message":
		var payload struct {
			AgentID string `json:"agent_id"`
			Content string `json:"content"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.AgentID == "" || payload.Content == "" {
			return
		}

		// Reject messages to child agents (they are managed by the orchestrator)
		var parentCheck models.Agent
		if err := database.Get().Select("parent_id").First(&parentCheck, "id = ?", payload.AgentID).Error; err == nil && parentCheck.ParentID != nil {
			logger.Log.Debugw("rejecting message to child agent", "agent_id", payload.AgentID)
			return
		}

		c.mu.Lock()
		c.agentID = payload.AgentID
		c.mu.Unlock()

		// If agent is already running, check if it's automated (inject via intervene) or queue
		c.hub.agentMu.Lock()
		_, isRunning := c.hub.cancels[payload.AgentID]
		agentInstance, hasAgent := c.hub.agents[payload.AgentID]
		c.hub.agentMu.Unlock()
		if isRunning {
			// For automated agents, inject as intervention instead of queueing
			if hasAgent && agentInstance.Type == "automated" {
				c.hub.handleIntervene(payload.AgentID, payload.Content)
			} else {
				c.hub.queueMessage(payload.AgentID, payload.Content)
			}
		} else {
			go c.hub.handleSendMessage(payload.AgentID, payload.Content)
		}

	case "cancel":
		var payload struct {
			AgentID string `json:"agent_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.AgentID != "" {
			c.hub.handleCancel(payload.AgentID)
		}

	case "pause":
		var payload struct {
			AgentID string `json:"agent_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.hub.handlePause(payload.AgentID)

	case "clear":
		var payload struct {
			AgentID string `json:"agent_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.hub.handleClear(payload.AgentID)

	case "resume":
		var payload struct {
			AgentID string `json:"agent_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.AgentID != "" {
			go c.hub.handleResume(payload.AgentID)
		}

	case "intervene":
		var payload struct {
			AgentID string `json:"agent_id"`
			Content string `json:"content"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.AgentID != "" && payload.Content != "" {
			c.hub.handleIntervene(payload.AgentID, payload.Content)
		}

	case "terminal_subscribe":
		var payload struct {
			TerminalID string `json:"terminal_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.TerminalID == "" {
			return
		}
		// Look up project directory for this terminal
		var workDir string
		var term models.Terminal
		if err := database.Get().Select("project_id").First(&term, "id = ?", payload.TerminalID).Error; err == nil && term.ProjectID != "" {
			workDir = c.hub.fileManager.ProjectDir(term.ProjectID)
		}
		// Start PTY if not already running, then stream output to this client
		sess, err := c.hub.terminalManager.Start(payload.TerminalID, 80, 24, workDir)
		if err != nil {
			logger.Log.Errorw("failed to start terminal", "terminal_id", payload.TerminalID, "error", err)
			return
		}
		// Start reading PTY output in background (only if session is new)
		go func() {
			c.hub.terminalManager.ReadLoop(payload.TerminalID, func(data []byte) {
				outData, _ := json.Marshal(map[string]string{
					"terminal_id": payload.TerminalID,
					"data":        string(data),
				})
				c.safeSend(mustMarshal(WSMessage{Type: "terminal_output", Payload: outData}))
			})
			// PTY exited
			exitData, _ := json.Marshal(map[string]string{"terminal_id": payload.TerminalID})
			c.safeSend(mustMarshal(WSMessage{Type: "terminal_exit", Payload: exitData}))
			_ = sess // keep reference
		}()

	case "terminal_input":
		var payload struct {
			TerminalID string `json:"terminal_id"`
			Data       string `json:"data"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.TerminalID != "" && payload.Data != "" {
			_ = c.hub.terminalManager.Write(payload.TerminalID, []byte(payload.Data))
		}

	case "terminal_resize":
		var payload struct {
			TerminalID string `json:"terminal_id"`
			Cols       uint16 `json:"cols"`
			Rows       uint16 `json:"rows"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.TerminalID != "" {
			_ = c.hub.terminalManager.Resize(payload.TerminalID, payload.Cols, payload.Rows)
		}

	case "process_subscribe":
		var payload struct {
			ProcessID string `json:"process_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.ProcessID == "" {
			return
		}
		// Guard against duplicate subscribe
		c.mu.Lock()
		if c.subscribedProcess == payload.ProcessID {
			c.mu.Unlock()
			return
		}
		c.subscribedProcess = payload.ProcessID
		c.mu.Unlock()

		sess, ok := c.hub.processManager.Get(payload.ProcessID)
		if !ok {
			return
		}
		// Send buffered log history
		lines := sess.buffer.Lines()
		for _, line := range lines {
			outData, _ := json.Marshal(map[string]any{
				"process_id": payload.ProcessID,
				"stream":     line.Stream,
				"text":       line.Text,
				"timestamp":  line.Timestamp,
			})
			c.safeSend(mustMarshal(WSMessage{Type: "process_output", Payload: outData}))
		}
		// Send current process info
		infoData, _ := json.Marshal(sess.info())
		c.safeSend(mustMarshal(WSMessage{Type: "process_info", Payload: infoData}))

	case "process_stop":
		var payload struct {
			ProcessID string `json:"process_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.ProcessID != "" {
			go c.hub.processManager.Stop(payload.ProcessID)
		}
	}
}

// getCounter returns the atomic message counter for an agent, creating one if needed.
func (h *Hub) getCounter(agentID string) *atomic.Int64 {
	h.agentMu.Lock()
	defer h.agentMu.Unlock()
	counter, ok := h.counters[agentID]
	if !ok {
		counter = &atomic.Int64{}
		h.counters[agentID] = counter
	}
	return counter
}

// createLogCallback creates a log callback that uses the Hub's atomic counter
// for message numbering. This is safe to call multiple times for the same agent.
func (h *Hub) createLogCallback(agentID string, counter *atomic.Int64) agent.LogCallback {
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
				"agent_id": agentID,
				"type":     entry.Type,
				"content":  entry.Content,
				"agentno":  entry.AgentNo,
				"stream":   true,
			})
			h.broadcast(agentID, WSMessage{Type: "stream", Payload: streamData})
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
			AgentID: agentID,
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
				AgentID: agentID,
				Content: entry.Content,
				Type:    string(msgType),
				Heading: entry.Heading,
			})
		}

		msgData, _ := json.Marshal(msg)
		h.broadcast(agentID, WSMessage{Type: "message", Payload: msgData})
	}
}

// createChildLogCallback creates a log callback for orchestrator child agents.
// Messages are saved under the child's AgentID but broadcast to the parent's WS subscribers.
func (h *Hub) createChildLogCallback(childAgentID, parentAgentID string, counter *atomic.Int64) agent.LogCallback {
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
				"agent_id":       parentAgentID,
				"child_agent_id": childAgentID,
				"type":           entry.Type,
				"content":        entry.Content,
				"agentno":        entry.AgentNo,
				"stream":         true,
			})
			h.broadcast(parentAgentID, WSMessage{Type: "stream", Payload: streamData})
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
			AgentID: childAgentID,
			No:      no,
			Type:    msgType,
			Heading: entry.Heading,
			Content: entry.Content,
			Kvps:    kvpsJSON,
			AgentNo: entry.AgentNo,
		}
		database.Get().Create(&msg)

		if entry.Content != "" {
			_ = search.Index(search.Document{
				ID:      msg.ID,
				AgentID: childAgentID,
				Content: entry.Content,
				Type:    string(msgType),
				Heading: entry.Heading,
			})
		}

		// Broadcast to parent's subscribers with child_agent_id field
		childMsgData, _ := json.Marshal(map[string]any{
			"id":             msg.ID,
			"agent_id":       parentAgentID,
			"child_agent_id": childAgentID,
			"no":             msg.No,
			"type":           msg.Type,
			"heading":        msg.Heading,
			"content":        msg.Content,
			"kvps":           msg.Kvps,
			"agentno":        msg.AgentNo,
			"timestamp":      msg.CreatedAt,
		})
		h.broadcast(parentAgentID, WSMessage{Type: "child_message", Payload: childMsgData})
	}
}

// queueMessage saves a user message to the DB, broadcasts it, and adds it to the in-memory queue.
func (h *Hub) queueMessage(agentID, content string) {
	db := database.Get()

	// Initialize counter from DB
	counter := h.getCounter(agentID)

	// Save user message to DB immediately so it appears in agent
	userNo := int(counter.Add(1))
	userMsg := models.Message{
		ID:      uuid.New().String(),
		AgentID: agentID,
		No:      userNo,
		Type:    models.MessageTypeUser,
		Content: content,
	}
	db.Create(&userMsg)

	// Broadcast user message to clients
	userMsgData, _ := json.Marshal(userMsg)
	h.broadcast(agentID, WSMessage{Type: "message", Payload: userMsgData})

	// Add to queue
	h.queueMu.Lock()
	h.queues[agentID] = append(h.queues[agentID], content)
	h.queueMu.Unlock()

	// Broadcast queue size update
	h.queueMu.Lock()
	queueLen := len(h.queues[agentID])
	h.queueMu.Unlock()
	queueData, _ := json.Marshal(map[string]any{"id": agentID, "queue_size": queueLen})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: queueData})
}

// popQueue removes and returns the next queued message, or "" if empty.
func (h *Hub) popQueue(agentID string) string {
	h.queueMu.Lock()
	defer h.queueMu.Unlock()
	q := h.queues[agentID]
	if len(q) == 0 {
		return ""
	}
	msg := q[0]
	h.queues[agentID] = q[1:]
	return msg
}

// clearQueue removes all queued messages for an agent.
func (h *Hub) clearQueue(agentID string) {
	h.queueMu.Lock()
	delete(h.queues, agentID)
	h.queueMu.Unlock()
}

// handleCancel stops the current agent run and clears the message queue.
func (h *Hub) handleCancel(agentID string) {
	// Clear message queue first
	h.clearQueue(agentID)

	h.agentMu.Lock()
	agentInstance, hasAgent := h.agents[agentID]
	cancel, hasCancel := h.cancels[agentID]
	if !hasCancel {
		h.agentMu.Unlock()
		// Broadcast to clear any stale queue UI state
		updateData, _ := json.Marshal(map[string]any{"id": agentID, "running": false, "paused": false, "queue_size": 0})
		h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: updateData})
		return
	}

	// Collect child IDs under lock, then cancel them after releasing
	childIDs := make([]string, len(h.childAgents[agentID]))
	copy(childIDs, h.childAgents[agentID])

	// Set cancelled flag BEFORE canceling so the agent returns ErrCancelled
	if hasAgent {
		agentInstance.SetCancelled(true)
		agentInstance.CancelTools() // Stop any in-progress tool execution
	}
	cancel()
	delete(h.cancels, agentID)
	h.agentMu.Unlock()

	// Cancel all child agents outside the lock
	for _, childID := range childIDs {
		h.cancelChildAgent(childID)
	}

	// Clean up child tracking
	h.agentMu.Lock()
	delete(h.childAgents, agentID)
	h.agentMu.Unlock()

	// Update DB and broadcast cancelled state
	db := database.Get()
	db.Model(&models.Agent{}).Where("id = ?", agentID).Updates(map[string]any{"running": false, "status": models.AgentStatusFailed})
	// Also mark child agents in DB
	if len(childIDs) > 0 {
		db.Model(&models.Agent{}).Where("id IN ?", childIDs).Updates(map[string]any{"running": false, "status": models.AgentStatusFailed})
	}

	updateData, _ := json.Marshal(map[string]any{"id": agentID, "running": false, "paused": false, "queue_size": 0})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: updateData})
}

// cancelChildAgent cancels a single child agent's in-memory state without touching the parent's childAgents map.
func (h *Hub) cancelChildAgent(childID string) {
	h.agentMu.Lock()
	if childInstance, ok := h.agents[childID]; ok {
		childInstance.SetCancelled(true)
		childInstance.CancelTools()
	}
	if childCancel, ok := h.cancels[childID]; ok {
		childCancel()
		delete(h.cancels, childID)
	}
	delete(h.agents, childID)
	delete(h.counters, childID)
	h.agentMu.Unlock()
}

func (h *Hub) handleSendMessage(agentID, content string) {
	db := database.Get()

	// Verify agent exists
	var agt models.Agent
	if err := db.First(&agt, "id = ?", agentID).Error; err != nil {
		logger.Log.Errorw("agent not found", "agent_id", agentID)
		return
	}

	// Get current message count for numbering
	var msgCount int64
	db.Model(&models.Message{}).Where("chat_id = ?", agentID).Count(&msgCount)
	isFirstMessage := msgCount == 0

	// Initialize counter from DB
	counter := h.getCounter(agentID)
	counter.Store(msgCount)

	// Save user message
	userNo := int(counter.Add(1))
	userMsg := models.Message{
		ID:      uuid.New().String(),
		AgentID: agentID,
		No:      userNo,
		Type:    models.MessageTypeUser,
		Content: content,
	}
	db.Create(&userMsg)

	// Broadcast user message to clients
	userMsgData, _ := json.Marshal(userMsg)
	h.broadcast(agentID, WSMessage{
		Type:    "message",
		Payload: userMsgData,
	})

	h.agentMu.Lock()

	// Mark agent as running
	db.Model(&agt).Update("running", true)
	chatUpdateData, _ := json.Marshal(map[string]any{"id": agentID, "running": true})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: chatUpdateData})

	// Create agent if needed, always refresh the log callback
	agentInstance, ok := h.agents[agentID]
	if !ok {
		agentInstance = agent.New(h.llmClient, h.tools, h.createLogCallback(agentID, counter))
		agentInstance.AgentID = agentID
		h.agents[agentID] = agentInstance
	} else {
		// Refresh callback so it uses current counter
		agentInstance.OnLog = h.createLogCallback(agentID, counter)
	}

	// Set type/mode from DB record
	agentInstance.Type = string(agt.Type)
	agentInstance.Mode = string(agt.Mode)
	agentInstance.LLMSemaphore = h.llmSemaphore

	// Set up orchestrator child spawner
	if agt.Mode == models.AgentModeOrchestrator {
		parentID := agentID
		agentInstance.ChildSpawner = func(task string) (*agent.Agent, string, error) {
			childID := uuid.New().String()
			childAgt := models.Agent{
				ID:        childID,
				ProjectID: agt.ProjectID,
				Name:      task,
				Type:      models.AgentTypeStandard,
				Mode:      models.AgentModeDirect,
				Status:    models.AgentStatusRunning,
				ParentID:  &parentID,
			}
			if err := database.Get().Create(&childAgt).Error; err != nil {
				return nil, "", fmt.Errorf("failed to create child agent: %w", err)
			}

			// Track child
			h.agentMu.Lock()
			h.childAgents[parentID] = append(h.childAgents[parentID], childID)
			h.agentMu.Unlock()

			childCounter := &atomic.Int64{}
			h.agentMu.Lock()
			h.counters[childID] = childCounter
			h.agentMu.Unlock()

			childLogCb := h.createChildLogCallback(childID, parentID, childCounter)
			child := agentInstance.NewChild(childID, childLogCb)

			if agt.ProjectID != "" {
				child.ProjectID = agt.ProjectID
				child.ProjectDir = h.fileManager.ProjectDir(agt.ProjectID)
				child.FileEventCallback = agentInstance.FileEventCallback
			}

			h.agentMu.Lock()
			h.agents[childID] = child
			h.agentMu.Unlock()

			return child, childID, nil
		}
	}

	// Set project context on agent if agent belongs to a project
	if agt.ProjectID != "" {
		agentInstance.ProjectID = agt.ProjectID
		agentInstance.ProjectDir = h.fileManager.ProjectDir(agt.ProjectID)
		agentInstance.FileEventCallback = func(event agent.FileEvent) {
			eventData, _ := json.Marshal(event)
			h.broadcastToProject(event.ProjectID, WSMessage{
				Type:    "file_event",
				Payload: eventData,
			})
		}
		// Cache the mapping
		h.agentToProject[agentID] = agt.ProjectID
	}

	// Clear any previous pause/cancelled state when sending a new message
	agentInstance.SetPaused(false)
	agentInstance.SetCancelled(false)

	// Per-mode context strategy
	var ctx context.Context
	var cancel context.CancelFunc
	switch agt.Mode {
	case models.AgentModeInfinite:
		ctx, cancel = context.WithCancel(context.Background())
	case models.AgentModeOneshot:
		timeout := time.Duration(config.Get().OneshotTimeoutMin) * time.Minute
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	default:
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	}
	h.cancels[agentID] = cancel
	h.agentMu.Unlock()

	// Run agent
	_, err := agentInstance.Run(ctx, content)
	cancel()

	// Clean up cancel from map
	h.agentMu.Lock()
	delete(h.cancels, agentID)
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
	if nextMsg := h.popQueue(agentID); nextMsg != "" {
		// Broadcast queue size update
		h.queueMu.Lock()
		queueLen := len(h.queues[agentID])
		h.queueMu.Unlock()
		queueData, _ := json.Marshal(map[string]any{"id": agentID, "queue_size": queueLen})
		h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: queueData})

		// Process next queued message (reuse goroutine)
		h.handleSendMessage(agentID, nextMsg)
		return
	}

	// Mark agent as not running
	db.Model(&agt).Update("running", false)
	chatDoneData, _ := json.Marshal(map[string]any{"id": agentID, "running": false, "queue_size": 0})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: chatDoneData})

	if err != nil {
		logger.Log.Errorw("agent run failed", "agent_id", agentID, "error", err)
		errMsgData, _ := json.Marshal(map[string]any{
			"agent_id": agentID,
			"type":     "error",
			"heading":  "Agent Error",
			"content":  err.Error(),
		})
		h.broadcast(agentID, WSMessage{Type: "message", Payload: errMsgData})
	}

	// Auto-name agent after first successful response
	if isFirstMessage && err == nil {
		go h.autoNameAgent(agentID, content)
	}
}

// autoNameAgent uses the LLM to generate a short agent name from the first message
func (h *Hub) autoNameAgent(agentID, userMessage string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.llmClient.ChatCompletion(ctx, []llm.ChatMessage{
		{
			Role:    "system",
			Content: "You are an agent naming assistant. Respond with ONLY a short agent name (1-4 words) based on the user's message. No formatting, no quotes, no punctuation at the end. Use proper capitalization. Examples: Database Setup, Image Analysis, Code Review, Weather App",
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}, nil, nil)
	if err != nil {
		logger.Log.Debugw("auto-name failed", "agent_id", agentID, "error", err)
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
	db.Model(&models.Agent{}).Where("id = ?", agentID).Update("name", name)

	// Broadcast update to clients
	updateData, _ := json.Marshal(map[string]any{
		"id":   agentID,
		"name": name,
	})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: updateData})
	logger.Log.Debugw("auto-named agent", "agent_id", agentID, "name", name)
}

func (h *Hub) handlePause(agentID string) {
	h.agentMu.Lock()
	agentInstance, hasAgent := h.agents[agentID]
	cancel, hasCancel := h.cancels[agentID]
	if !hasCancel {
		h.agentMu.Unlock()
		return // Nothing running to pause
	}

	// Set paused flag BEFORE canceling so the agent returns ErrPaused (not a raw error)
	if hasAgent {
		agentInstance.SetPaused(true)
	}
	cancel()
	delete(h.cancels, agentID)
	h.agentMu.Unlock()

	// Best-effort slot save to preserve KV cache for faster resume
	slotFilename := fmt.Sprintf("zeroloop_pause_%s", agentID)
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotSave(saveCtx, 0, slotFilename); err != nil {
		logger.Log.Debugw("slot save skipped (best-effort)", "agent_id", agentID, "error", err)
	}
	saveCancel()

	// Update DB and broadcast paused state
	db := database.Get()
	db.Model(&models.Agent{}).Where("id = ?", agentID).Update("running", false)

	chatUpdateData, _ := json.Marshal(map[string]any{"id": agentID, "running": false, "paused": true})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: chatUpdateData})
}

func (h *Hub) handleResume(agentID string) {
	h.agentMu.Lock()
	agentInstance, hasAgent := h.agents[agentID]
	if !hasAgent || !agentInstance.IsPaused() {
		h.agentMu.Unlock()
		return // Nothing to resume
	}

	// Cancel any stale context (shouldn't exist, but be safe)
	if cancel, ok := h.cancels[agentID]; ok {
		cancel()
	}

	// Ensure counter is initialized
	counter := h.counters[agentID]
	if counter == nil {
		counter = &atomic.Int64{}
		h.counters[agentID] = counter
		var msgCount int64
		database.Get().Model(&models.Message{}).Where("chat_id = ?", agentID).Count(&msgCount)
		counter.Store(msgCount)
	}

	// Refresh log callback with current counter
	agentInstance.OnLog = h.createLogCallback(agentID, counter)

	// Per-mode context strategy (matches handleSendMessage)
	var ctx context.Context
	var cancel context.CancelFunc
	switch models.AgentMode(agentInstance.Mode) {
	case models.AgentModeInfinite:
		ctx, cancel = context.WithCancel(context.Background())
	case models.AgentModeOneshot:
		timeout := time.Duration(config.Get().OneshotTimeoutMin) * time.Minute
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	default:
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	}
	h.cancels[agentID] = cancel
	h.agentMu.Unlock()

	// Best-effort slot restore for faster prompt processing
	slotFilename := fmt.Sprintf("zeroloop_pause_%s", agentID)
	restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotRestore(restoreCtx, 0, slotFilename); err != nil {
		logger.Log.Debugw("slot restore skipped (best-effort)", "agent_id", agentID, "error", err)
	}
	restoreCancel()

	// Mark agent as running
	db := database.Get()
	db.Model(&models.Agent{}).Where("id = ?", agentID).Updates(map[string]any{"running": true, "status": models.AgentStatusRunning})
	chatUpdateData, _ := json.Marshal(map[string]any{"id": agentID, "running": true, "paused": false})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: chatUpdateData})

	// Continue agent loop from where it was paused
	_, err := agentInstance.Continue(ctx)
	cancel()

	// Clean up cancel from map
	h.agentMu.Lock()
	delete(h.cancels, agentID)
	h.agentMu.Unlock()

	// If paused again, exit quietly
	if errors.Is(err, agent.ErrPaused) {
		return
	}

	// Agent completed — clear pause state and erase saved slot
	agentInstance.SetPaused(false)
	eraseCtx, eraseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotErase(eraseCtx, 0); err != nil {
		logger.Log.Debugw("slot erase skipped (best-effort)", "agent_id", agentID, "error", err)
	}
	eraseCancel()

	db.Model(&models.Agent{}).Where("id = ?", agentID).Update("running", false)
	chatDoneData, _ := json.Marshal(map[string]any{"id": agentID, "running": false})
	h.broadcast(agentID, WSMessage{Type: "chat_update", Payload: chatDoneData})

	if err != nil {
		logger.Log.Errorw("agent resume failed", "agent_id", agentID, "error", err)
		errMsgData, _ := json.Marshal(map[string]any{
			"agent_id": agentID,
			"type":     "error",
			"heading":  "Agent Error",
			"content":  err.Error(),
		})
		h.broadcast(agentID, WSMessage{Type: "message", Payload: errMsgData})
	}
}

func (h *Hub) handleClear(agentID string) {
	// Cancel running agent and clean up state (including children)
	h.cleanupAgent(agentID)

	db := database.Get()

	// Find child agents and delete their messages + search entries
	var childIDs []string
	db.Model(&models.Agent{}).Where("parent_id = ?", agentID).Pluck("id", &childIDs)
	for _, childID := range childIDs {
		var childMsgs []models.Message
		db.Where("chat_id = ?", childID).Select("id").Find(&childMsgs)
		for _, m := range childMsgs {
			_ = search.Delete(m.ID)
		}
		db.Where("chat_id = ?", childID).Delete(&models.Message{})
	}
	// Delete child agent DB records
	if len(childIDs) > 0 {
		db.Where("parent_id = ?", agentID).Delete(&models.Agent{})
	}

	// Delete all messages for this agent
	var msgs []models.Message
	db.Where("chat_id = ?", agentID).Select("id").Find(&msgs)
	for _, m := range msgs {
		_ = search.Delete(m.ID)
	}
	db.Where("chat_id = ?", agentID).Delete(&models.Message{})

	// Notify clients
	clearData, _ := json.Marshal(map[string]any{"agent_id": agentID})
	h.broadcast(agentID, WSMessage{Type: "clear", Payload: clearData})
}

func (h *Hub) handleIntervene(agentID, content string) {
	h.agentMu.Lock()
	agentInstance, ok := h.agents[agentID]
	h.agentMu.Unlock()

	if !ok {
		return // No agent running for this agent session
	}

	agentInstance.Intervene(content)

	// Also save the intervention as a message in the DB
	counter := h.getCounter(agentID)
	no := int(counter.Add(1))
	msg := models.Message{
		ID:      uuid.New().String(),
		AgentID: agentID,
		No:      no,
		Type:    models.MessageTypeUser,
		Heading: "Intervention",
		Content: content,
	}
	database.Get().Create(&msg)

	msgData, _ := json.Marshal(msg)
	h.broadcast(agentID, WSMessage{Type: "message", Payload: msgData})
}

// cleanupAgent cancels any running agent and removes in-memory state for an agent session.
// Also cleans up any child agents. Does NOT broadcast to WS clients or touch the database.
func (h *Hub) cleanupAgent(agentID string) {
	h.clearQueue(agentID)

	// Collect child IDs under lock, then clean them up after releasing
	h.agentMu.Lock()
	childIDs := make([]string, len(h.childAgents[agentID]))
	copy(childIDs, h.childAgents[agentID])
	delete(h.childAgents, agentID)

	if agentInstance, ok := h.agents[agentID]; ok {
		agentInstance.SetCancelled(true)
		agentInstance.CancelTools()
	}
	if cancel, ok := h.cancels[agentID]; ok {
		cancel()
		delete(h.cancels, agentID)
	}
	delete(h.agents, agentID)
	delete(h.counters, agentID)
	h.agentMu.Unlock()

	// Cancel child agents outside the parent lock
	for _, childID := range childIDs {
		h.cancelChildAgent(childID)
	}

	// Best-effort slot erase to free server memory
	eraseCtx, eraseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := h.llmClient.SlotErase(eraseCtx, 0); err != nil {
		logger.Log.Debugw("slot erase skipped (best-effort)", "agent_id", agentID, "error", err)
	}
	eraseCancel()
}
