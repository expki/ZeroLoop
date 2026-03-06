package api

import (
	"encoding/json"
	"net/http"
	"os"

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
		Name string `json:"name"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	project := models.Project{
		ID:   uuid.New().String(),
		Name: req.Name,
	}

	// Create filesystem directory using project name (use existing folder if present)
	projectDir := fm.ProjectDir(req.Name)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		logger.Log.Errorw("failed to create project directory", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create project directory")
		return
	}

	if err := database.Get().Create(&project).Error; err != nil {
		logger.Log.Errorw("failed to create project", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var project models.Project
	if err := database.Get().Preload("Agents").First(&project, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func updateProject(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager) {
	id := r.PathValue("id")
	var req struct {
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	db := database.Get()

	if req.Name == nil || *req.Name == "" {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	// Look up current project to get old name for folder rename
	var project models.Project
	if err := db.First(&project, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	oldName := project.Name
	newName := *req.Name

	// Update DB
	result := db.Model(&project).Update("name", newName)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	// Rename folder on disk if name changed
	if oldName != newName {
		oldDir := fm.ProjectDir(oldName)
		newDir := fm.ProjectDir(newName)
		if _, err := os.Stat(oldDir); err == nil {
			if err := os.Rename(oldDir, newDir); err != nil {
				logger.Log.Warnw("failed to rename project directory", "error", err, "from", oldDir, "to", newDir)
			}
		}
	}

	db.First(&project, "id = ?", id)
	writeJSON(w, http.StatusOK, project)
}

func deleteProject(w http.ResponseWriter, r *http.Request, hub *Hub, fm *filemanager.FileManager) {
	id := r.PathValue("id")
	db := database.Get()

	// Look up project name before deleting (needed for folder name)
	var project models.Project
	if err := db.First(&project, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	projectName := project.Name

	// Clean up agents for all agents in this project
	var agts []models.Agent
	db.Where("project_id = ?", id).Select("id").Find(&agts)
	for _, agt := range agts {
		hub.cleanupAgent(agt.ID)
	}

	// Delete DB records (cascade deletes agents and messages)
	if err := db.Delete(&models.Project{}, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	// Delete project files from disk using project name
	projectDir := fm.ProjectDir(projectName)
	if _, err := os.Stat(projectDir); err == nil {
		if err := os.RemoveAll(projectDir); err != nil {
			logger.Log.Warnw("failed to delete project directory", "error", err, "project_id", id)
		}
	}

	// Clean up project file metadata
	db.Where("project_id = ?", id).Delete(&models.ProjectFile{})

	w.WriteHeader(http.StatusNoContent)
}
