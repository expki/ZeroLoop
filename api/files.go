package api

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/filemanager"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func listProjectFiles(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager) {
	projectID := r.PathValue("id")

	// Verify project exists
	var project models.Project
	if err := database.Get().First(&project, "id = ?", projectID).Error; err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	files, err := fm.ListFiles(projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func readProjectFile(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager) {
	projectID := r.PathValue("id")
	filePath := r.PathValue("path")

	content, err := fm.ReadFile(projectID, filePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"content": string(content),
		"path":    filePath,
	})
}

func createProjectFile(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager, hub *Hub) {
	projectID := r.PathValue("id")
	filePath := r.PathValue("path")

	var req struct {
		Content string `json:"content"`
		IsDir   bool   `json:"is_dir"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	if req.IsDir {
		if err := fm.CreateDir(projectID, filePath); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := fm.WriteFile(projectID, filePath, []byte(req.Content)); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Create DB metadata
	name := filepath.Base(filePath)
	mimeType := ""
	if !req.IsDir {
		mimeType = mime.TypeByExtension(filepath.Ext(name))
	}

	pf := models.ProjectFile{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Path:      filePath,
		Name:      name,
		IsDir:     req.IsDir,
		Size:      int64(len(req.Content)),
		MimeType:  mimeType,
	}
	database.Get().Create(&pf)

	// Broadcast file_created event
	broadcastFileEvent(hub, projectID, filePath, name, int64(len(req.Content)), "created", "")

	writeJSON(w, http.StatusCreated, pf)
}

func updateProjectFile(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager, hub *Hub) {
	projectID := r.PathValue("id")
	filePath := r.PathValue("path")

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := fm.WriteFile(projectID, filePath, []byte(req.Content)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Update DB metadata
	db := database.Get()
	size := int64(len(req.Content))
	db.Model(&models.ProjectFile{}).
		Where("project_id = ? AND path = ?", projectID, filePath).
		Updates(map[string]any{"size": size})

	// Broadcast file_changed event
	broadcastFileEvent(hub, projectID, filePath, filepath.Base(filePath), size, "changed", "")

	writeJSON(w, http.StatusOK, map[string]any{
		"path": filePath,
		"size": size,
	})
}

func deleteProjectFile(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager, hub *Hub) {
	projectID := r.PathValue("id")
	filePath := r.PathValue("path")

	// Use recursive delete to handle non-empty directories
	if err := fm.DeleteFileRecursive(projectID, filePath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Delete DB metadata (the file itself + any nested children)
	db := database.Get()
	db.Where("project_id = ? AND path = ?", projectID, filePath).Delete(&models.ProjectFile{})
	db.Where("project_id = ? AND path LIKE ?", projectID, filePath+"/%").Delete(&models.ProjectFile{})

	// Broadcast file_deleted event
	broadcastFileEvent(hub, projectID, filePath, filepath.Base(filePath), 0, "deleted", "")

	w.WriteHeader(http.StatusNoContent)
}

func uploadProjectFiles(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager, hub *Hub) {
	projectID := r.PathValue("id")

	// Verify project exists
	var project models.Project
	if err := database.Get().First(&project, "id = ?", projectID).Error; err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Limit upload size to 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	var uploaded []models.ProjectFile
	for _, fileHeaders := range r.MultipartForm.File {
		for _, fh := range fileHeaders {
			file, err := fh.Open()
			if err != nil {
				continue
			}

			size, err := fm.WriteFileFromReader(projectID, fh.Filename, file)
			file.Close()
			if err != nil {
				logger.Log.Warnw("failed to write uploaded file", "error", err, "filename", fh.Filename)
				continue
			}

			mimeType := mime.TypeByExtension(filepath.Ext(fh.Filename))
			pf := models.ProjectFile{
				ID:        uuid.New().String(),
				ProjectID: projectID,
				Path:      fh.Filename,
				Name:      filepath.Base(fh.Filename),
				IsDir:     false,
				Size:      size,
				MimeType:  mimeType,
			}

			// Upsert: delete existing then create
			db := database.Get()
			db.Where("project_id = ? AND path = ?", projectID, fh.Filename).Delete(&models.ProjectFile{})
			db.Create(&pf)
			uploaded = append(uploaded, pf)

			broadcastFileEvent(hub, projectID, fh.Filename, pf.Name, size, "created", "")
		}
	}

	writeJSON(w, http.StatusCreated, uploaded)
}

func moveProjectFile(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager, hub *Hub) {
	projectID := r.PathValue("id")

	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" {
		writeError(w, http.StatusBadRequest, "from and to are required")
		return
	}

	// Perform the filesystem rename
	err := fm.RenameFile(projectID, req.From, req.To)
	if err != nil {
		if strings.Contains(err.Error(), "destination already exists") {
			writeError(w, http.StatusConflict, "destination already exists")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Update DB metadata in a transaction
	db := database.Get()
	txErr := db.Transaction(func(tx *gorm.DB) error {
		// Update the moved/renamed entry itself
		tx.Model(&models.ProjectFile{}).
			Where("project_id = ? AND path = ?", projectID, req.From).
			Updates(map[string]any{
				"path": req.To,
				"name": filepath.Base(req.To),
			})

		// For directory rename: update all nested paths
		oldPrefix := req.From + "/"
		newPrefix := req.To + "/"
		var nested []models.ProjectFile
		tx.Where("project_id = ? AND path LIKE ?", projectID, oldPrefix+"%").Find(&nested)
		for _, f := range nested {
			newNestedPath := newPrefix + strings.TrimPrefix(f.Path, oldPrefix)
			tx.Model(&f).Updates(map[string]any{
				"path": newNestedPath,
				"name": filepath.Base(newNestedPath),
			})
		}
		return nil
	})
	if txErr != nil {
		logger.Log.Errorw("rename DB transaction failed", "error", txErr,
			"project_id", projectID, "from", req.From, "to", req.To)
	}

	// Broadcast a single atomic rename event
	broadcastFileEvent(hub, projectID, req.To, filepath.Base(req.To), 0, "renamed", req.From)

	writeJSON(w, http.StatusOK, map[string]string{
		"from": req.From,
		"to":   req.To,
	})
}

// broadcastFileEvent sends a file event to all clients subscribed to the project.
// For rename events, pass the old path as oldPath; for other events pass "".
func broadcastFileEvent(hub *Hub, projectID, path, name string, size int64, action string, oldPath string) {
	payload := map[string]any{
		"project_id": projectID,
		"path":       path,
		"name":       name,
		"size":       size,
		"action":     action,
	}
	if oldPath != "" {
		payload["old_path"] = oldPath
	}
	eventData, _ := json.Marshal(payload)
	hub.broadcastToProject(projectID, WSMessage{
		Type:    "file_event",
		Payload: eventData,
	})
}

// readProjectFileRaw serves the raw file content (for binary files / downloads).
func readProjectFileRaw(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager) {
	projectID := r.PathValue("id")
	filePath := r.PathValue("path")

	content, err := fm.ReadFile(projectID, filePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = http.DetectContentType(content)
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+filepath.Base(filePath)+"\"")
	_, _ = io.Copy(w, io.NopCloser(io.NewSectionReader(
		readerAtFromBytes(content), 0, int64(len(content)),
	)))
}

type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n = copy(p, b[off:])
	return n, nil
}

func readerAtFromBytes(b []byte) io.ReaderAt {
	return bytesReaderAt(b)
}
