package api

import (
	"encoding/json"
	"net/http"

	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/filemanager"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/models"
	"github.com/google/uuid"
)

func listProjects(w http.ResponseWriter, r *http.Request) {
	var projects []models.Project
	result := database.Get().Order("updated_at DESC").Find(&projects)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch projects")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func createProject(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	project := models.Project{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
	}

	// Create filesystem directory
	if err := fm.CreateProject(project.ID); err != nil {
		logger.Log.Errorw("failed to create project directory", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create project directory")
		return
	}

	if err := database.Get().Create(&project).Error; err != nil {
		logger.Log.Errorw("failed to create project", "error", err)
		// Clean up directory on DB failure
		fm.DeleteProject(project.ID)
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var project models.Project
	if err := database.Get().Preload("Chats").First(&project, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	db := database.Get()
	updates := map[string]any{}
	if req.Name != nil && *req.Name != "" {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	result := db.Model(&models.Project{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var project models.Project
	db.First(&project, "id = ?", id)
	writeJSON(w, http.StatusOK, project)
}

func deleteProject(w http.ResponseWriter, r *http.Request, hub *Hub, fm *filemanager.FileManager) {
	id := r.PathValue("id")
	db := database.Get()

	// Clean up agents for all chats in this project
	var chats []models.Chat
	db.Where("project_id = ?", id).Select("id").Find(&chats)
	for _, chat := range chats {
		hub.cleanupChat(chat.ID)
	}

	// Delete DB records (cascade deletes chats and messages)
	if err := db.Delete(&models.Project{}, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	// Delete project files from disk
	if err := fm.DeleteProject(id); err != nil {
		logger.Log.Warnw("failed to delete project directory", "error", err, "project_id", id)
	}

	// Clean up project file metadata
	db.Where("project_id = ?", id).Delete(&models.ProjectFile{})

	w.WriteHeader(http.StatusNoContent)
}
