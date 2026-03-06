package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/filemanager"
	"github.com/expki/ZeroLoop.git/llm"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/models"
	"github.com/expki/ZeroLoop.git/search"
	"github.com/google/uuid"
)

// RegisterRoutes registers all API routes on the given mux
func RegisterRoutes(mux *http.ServeMux, hub *Hub, fm *filemanager.FileManager) {
	// Project routes
	mux.HandleFunc("GET /api/projects", listProjects)
	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		createProject(w, r, fm)
	})
	mux.HandleFunc("GET /api/projects/{id}", getProject)
	mux.HandleFunc("PATCH /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		updateProject(w, r, fm)
	})
	mux.HandleFunc("DELETE /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleteProject(w, r, hub, fm)
	})

	// File routes (Go 1.22+ {path...} wildcard syntax)
	mux.HandleFunc("GET /api/projects/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		listProjectFiles(w, r, fm)
	})
	mux.HandleFunc("GET /api/projects/{id}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		readProjectFile(w, r, fm)
	})
	mux.HandleFunc("POST /api/projects/{id}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		createProjectFile(w, r, fm, hub)
	})
	mux.HandleFunc("PUT /api/projects/{id}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		updateProjectFile(w, r, fm, hub)
	})
	mux.HandleFunc("DELETE /api/projects/{id}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		deleteProjectFile(w, r, fm, hub)
	})
	mux.HandleFunc("POST /api/projects/{id}/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadProjectFiles(w, r, fm, hub)
	})
	mux.HandleFunc("POST /api/projects/{id}/files/_move", func(w http.ResponseWriter, r *http.Request) {
		moveProjectFile(w, r, fm, hub)
	})
	mux.HandleFunc("GET /api/projects/{id}/search", func(w http.ResponseWriter, r *http.Request) {
		searchProjectFiles(w, r, fm)
	})

	// Agent routes
	mux.HandleFunc("GET /api/agents", listAgents)
	mux.HandleFunc("POST /api/agents", createAgent)
	mux.HandleFunc("GET /api/agents/{id}", getAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleteAgent(w, r, hub)
	})
	mux.HandleFunc("PATCH /api/agents/{id}", renameAgent)
	mux.HandleFunc("GET /api/agents/{id}/messages", getAgentMessages)
	mux.HandleFunc("POST /api/agents/{id}/export", exportAgent)
	mux.HandleFunc("POST /api/agents/{id}/branch", func(w http.ResponseWriter, r *http.Request) {
		branchAgent(w, r)
	})
	// Terminal routes
	mux.HandleFunc("GET /api/terminals", listTerminals)
	mux.HandleFunc("POST /api/terminals", createTerminal)
	mux.HandleFunc("DELETE /api/terminals/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleteTerminal(w, r, hub)
	})
	mux.HandleFunc("PATCH /api/terminals/{id}", renameTerminal)

	mux.HandleFunc("GET /api/health/llm", func(w http.ResponseWriter, r *http.Request) {
		llmHealth(w, r, hub)
	})
	mux.HandleFunc("POST /api/completions", func(w http.ResponseWriter, r *http.Request) {
		codeCompletion(w, r, hub)
	})
	mux.HandleFunc("POST /api/completions/smart", func(w http.ResponseWriter, r *http.Request) {
		smartCodeCompletion(w, r, hub)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func listAgents(w http.ResponseWriter, r *http.Request) {
	var agents []models.Agent
	db := database.Get()
	query := db.Order("updated_at DESC")
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	result := query.Find(&agents)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch agents")
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func createAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		ProjectID string `json:"project_id"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	// Verify project exists
	var project models.Project
	if err := database.Get().First(&project, "id = ?", req.ProjectID).Error; err != nil {
		writeError(w, http.StatusBadRequest, "project not found")
		return
	}
	if req.Name == "" {
		req.Name = "New Agent"
	}

	agt := models.Agent{
		ID:        uuid.New().String(),
		ProjectID: req.ProjectID,
		Name:      req.Name,
	}
	if err := database.Get().Create(&agt).Error; err != nil {
		logger.Log.Errorw("failed to create agent", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}
	writeJSON(w, http.StatusCreated, agt)
}

func getAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var agt models.Agent
	if err := database.Get().First(&agt, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, agt)
}

func deleteAgent(w http.ResponseWriter, r *http.Request, hub *Hub) {
	id := r.PathValue("id")

	// Clean up in-memory agent/counter state without broadcasting
	hub.cleanupAgent(id)

	// Clean up search index
	db := database.Get()
	var msgs []models.Message
	db.Where("chat_id = ?", id).Select("id").Find(&msgs)
	for _, m := range msgs {
		_ = search.Delete(m.ID)
	}

	if err := db.Delete(&models.Agent{}, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renameAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	db := database.Get()
	result := db.Model(&models.Agent{}).Where("id = ?", id).Update("name", req.Name)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename agent")
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var agt models.Agent
	db.First(&agt, "id = ?", id)
	writeJSON(w, http.StatusOK, agt)
}

func getAgentMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var messages []models.Message
	result := database.Get().Where("chat_id = ?", id).Order("no ASC").Find(&messages)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch messages")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func exportAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	db := database.Get()

	var agt models.Agent
	if err := db.First(&agt, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var messages []models.Message
	db.Where("chat_id = ?", id).Order("no ASC").Find(&messages)

	export := map[string]any{
		"agent":    agt,
		"messages": messages,
	}
	writeJSON(w, http.StatusOK, export)
}

func branchAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		MessageNo int `json:"message_no"` // Branch from this message number (inclusive)
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	db := database.Get()

	// Get original agent
	var originalAgt models.Agent
	if err := db.First(&originalAgt, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Create new branched agent
	newAgt := models.Agent{
		ID:        uuid.New().String(),
		ProjectID: originalAgt.ProjectID,
		Name:      originalAgt.Name + " (branch)",
	}
	if err := db.Create(&newAgt).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create branched agent")
		return
	}

	// Copy messages up to the branch point
	var messages []models.Message
	query := db.Where("chat_id = ?", id).Order("no ASC")
	if req.MessageNo > 0 {
		query = query.Where("no <= ?", req.MessageNo)
	}
	query.Find(&messages)

	for i, msg := range messages {
		newMsg := models.Message{
			ID:      uuid.New().String(),
			AgentID: newAgt.ID,
			No:      i + 1,
			Type:    msg.Type,
			Heading: msg.Heading,
			Content: msg.Content,
			Kvps:    msg.Kvps,
			AgentNo: msg.AgentNo,
		}
		db.Create(&newMsg)

		// Index in search
		if msg.Content != "" {
			_ = search.Index(search.Document{
				ID:      newMsg.ID,
				AgentID: newAgt.ID,
				Content: msg.Content,
				Type:    string(msg.Type),
				Heading: msg.Heading,
			})
		}
	}

	writeJSON(w, http.StatusCreated, newAgt)
}

func llmHealth(w http.ResponseWriter, r *http.Request, hub *Hub) {
	ctx := r.Context()
	health, err := hub.llmClient.Health(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func codeCompletion(w http.ResponseWriter, r *http.Request, hub *Hub) {
	var req struct {
		Prefix    string   `json:"prefix"`
		Suffix    string   `json:"suffix"`
		MaxTokens int      `json:"max_tokens"`
		Stop      []string `json:"stop"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10) // 256KB limit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prefix == "" && req.Suffix == "" {
		writeError(w, http.StatusBadRequest, "prefix or suffix is required")
		return
	}

	if req.MaxTokens <= 0 {
		req.MaxTokens = 128
	}
	if req.MaxTokens > 512 {
		req.MaxTokens = 512
	}
	if req.Stop == nil {
		req.Stop = []string{"\n\n"}
	}

	infillReq := &llm.InfillRequest{
		InputPrefix: req.Prefix,
		InputSuffix: req.Suffix,
		NPredict:    req.MaxTokens,
		Temperature: 0.2,
		Stop:        req.Stop,
	}

	resp, err := hub.llmClient.Infill(r.Context(), infillReq)
	if err != nil {
		logger.Log.Debugw("infill request failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]string{"text": ""})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"text": resp.Content})
}

func smartCodeCompletion(w http.ResponseWriter, r *http.Request, hub *Hub) {
	var req struct {
		Prefix   string `json:"prefix"`
		Suffix   string `json:"suffix"`
		Filename string `json:"filename"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prefix == "" && req.Suffix == "" {
		writeError(w, http.StatusBadRequest, "prefix or suffix is required")
		return
	}

	ctx := r.Context()

	// Build evaluation context: last 20 lines of prefix, first 10 of suffix
	prefixLines := strings.Split(req.Prefix, "\n")
	suffixLines := strings.Split(req.Suffix, "\n")
	evalPrefix := req.Prefix
	if len(prefixLines) > 20 {
		evalPrefix = strings.Join(prefixLines[len(prefixLines)-20:], "\n")
	}
	evalSuffix := req.Suffix
	if len(suffixLines) > 10 {
		evalSuffix = strings.Join(suffixLines[:10], "\n")
	}

	// Step 1: Evaluate if completion is appropriate using LLM
	evalPrompt := fmt.Sprintf(`Analyze the code at the cursor position (marked by <CURSOR>) and decide if autocomplete should trigger.

Return ONLY valid JSON: {"complete":true,"max_tokens":N} or {"complete":false}

When to complete (complete=true):
- Cursor after partial identifier, keyword, operator, dot, arrow, colon, equals, comma
- Cursor mid-line where more code naturally follows
- Cursor at end of line where a continuation is expected (opening brace, arrow function, etc.)
- Cursor after indentation on a new line inside a code block

When NOT to complete (complete=false):
- Cursor on an empty/blank line between complete, separate code blocks
- Cursor right after a complete closing brace/block with no continuation expected
- Cursor inside a finished standalone comment
- File is empty or has no meaningful code context

max_tokens guide: 16 (finish current word/expression), 32 (complete current line/short statement), 64 (multi-line block, 2-4 lines), 128 (function body or larger block)

File: %s

%s<CURSOR>%s`, req.Filename, evalPrefix, evalSuffix)

	messages := []llm.ChatMessage{
		{Role: "user", Content: evalPrompt},
	}

	evalResult, err := hub.llmClient.ChatCompletion(ctx, messages, nil, &llm.CompletionRequest{
		NPredict:    50,
		Temperature: 0.1,
	})
	if err != nil {
		logger.Log.Debugw("smart completion eval failed, falling back to infill", "error", err)
		// Fall back to standard infill on evaluation failure
		fallbackInfill(w, r, hub, req.Prefix, req.Suffix)
		return
	}

	// Parse evaluation result
	type evalResponse struct {
		Complete  bool `json:"complete"`
		MaxTokens int  `json:"max_tokens"`
	}
	var eval evalResponse
	content := strings.TrimSpace(evalResult.Content)
	// Extract JSON object from response (model might include extra text)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}
	if err := json.Unmarshal([]byte(content), &eval); err != nil {
		logger.Log.Debugw("smart completion eval parse failed, defaulting to complete", "content", evalResult.Content, "error", err)
		eval.Complete = true
		eval.MaxTokens = 64
	}

	if !eval.Complete {
		writeJSON(w, http.StatusOK, map[string]any{"text": "", "skipped": true})
		return
	}

	// Clamp max_tokens to reasonable bounds
	if eval.MaxTokens <= 0 {
		eval.MaxTokens = 64
	}
	if eval.MaxTokens > 256 {
		eval.MaxTokens = 256
	}

	// Step 2: Perform the actual infill completion
	infillReq := &llm.InfillRequest{
		InputPrefix: req.Prefix,
		InputSuffix: req.Suffix,
		NPredict:    eval.MaxTokens,
		Temperature: 0.2,
		Stop:        []string{"\n\n"},
	}

	resp, err := hub.llmClient.Infill(ctx, infillReq)
	if err != nil {
		logger.Log.Debugw("smart completion infill failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"text": "", "skipped": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"text": resp.Content, "skipped": false})
}

// fallbackInfill performs a standard infill when smart evaluation fails
func fallbackInfill(w http.ResponseWriter, r *http.Request, hub *Hub, prefix, suffix string) {
	infillReq := &llm.InfillRequest{
		InputPrefix: prefix,
		InputSuffix: suffix,
		NPredict:    64,
		Temperature: 0.2,
		Stop:        []string{"\n\n"},
	}

	resp, err := hub.llmClient.Infill(r.Context(), infillReq)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"text": "", "skipped": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"text": resp.Content, "skipped": false})
}
