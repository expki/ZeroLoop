package api

import (
	"encoding/json"
	"net/http"

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
	mux.HandleFunc("PATCH /api/projects/{id}", updateProject)
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

	// Chat routes
	mux.HandleFunc("GET /api/chats", listChats)
	mux.HandleFunc("POST /api/chats", createChat)
	mux.HandleFunc("GET /api/chats/{id}", getChat)
	mux.HandleFunc("DELETE /api/chats/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleteChat(w, r, hub)
	})
	mux.HandleFunc("PATCH /api/chats/{id}", renameChat)
	mux.HandleFunc("GET /api/chats/{id}/messages", getChatMessages)
	mux.HandleFunc("POST /api/chats/{id}/export", exportChat)
	mux.HandleFunc("POST /api/chats/{id}/branch", func(w http.ResponseWriter, r *http.Request) {
		branchChat(w, r)
	})
	mux.HandleFunc("GET /api/health/llm", func(w http.ResponseWriter, r *http.Request) {
		llmHealth(w, r, hub)
	})
	mux.HandleFunc("POST /api/completions", func(w http.ResponseWriter, r *http.Request) {
		codeCompletion(w, r, hub)
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

func listChats(w http.ResponseWriter, r *http.Request) {
	var chats []models.Chat
	db := database.Get()
	query := db.Order("updated_at DESC")
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	result := query.Find(&chats)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func createChat(w http.ResponseWriter, r *http.Request) {
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
		req.Name = "New Chat"
	}

	chat := models.Chat{
		ID:        uuid.New().String(),
		ProjectID: req.ProjectID,
		Name:      req.Name,
	}
	if err := database.Get().Create(&chat).Error; err != nil {
		logger.Log.Errorw("failed to create chat", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}
	writeJSON(w, http.StatusCreated, chat)
}

func getChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var chat models.Chat
	if err := database.Get().First(&chat, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

func deleteChat(w http.ResponseWriter, r *http.Request, hub *Hub) {
	id := r.PathValue("id")

	// Clean up in-memory agent/counter state without broadcasting
	hub.cleanupChat(id)

	// Clean up search index
	db := database.Get()
	var msgs []models.Message
	db.Where("chat_id = ?", id).Select("id").Find(&msgs)
	for _, m := range msgs {
		_ = search.Delete(m.ID)
	}

	if err := db.Delete(&models.Chat{}, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete chat")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renameChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	db := database.Get()
	result := db.Model(&models.Chat{}).Where("id = ?", id).Update("name", req.Name)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename chat")
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}

	var chat models.Chat
	db.First(&chat, "id = ?", id)
	writeJSON(w, http.StatusOK, chat)
}

func getChatMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var messages []models.Message
	result := database.Get().Where("chat_id = ?", id).Order("no ASC").Find(&messages)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch messages")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func exportChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	db := database.Get()

	var chat models.Chat
	if err := db.First(&chat, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}

	var messages []models.Message
	db.Where("chat_id = ?", id).Order("no ASC").Find(&messages)

	export := map[string]any{
		"chat":     chat,
		"messages": messages,
	}
	writeJSON(w, http.StatusOK, export)
}

func branchChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		MessageNo int `json:"message_no"` // Branch from this message number (inclusive)
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	db := database.Get()

	// Get original chat
	var originalChat models.Chat
	if err := db.First(&originalChat, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}

	// Create new branched chat
	newChat := models.Chat{
		ID:        uuid.New().String(),
		ProjectID: originalChat.ProjectID,
		Name:      originalChat.Name + " (branch)",
	}
	if err := db.Create(&newChat).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create branched chat")
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
			ChatID:  newChat.ID,
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
				ChatID:  newChat.ID,
				Content: msg.Content,
				Type:    string(msg.Type),
				Heading: msg.Heading,
			})
		}
	}

	writeJSON(w, http.StatusCreated, newChat)
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
