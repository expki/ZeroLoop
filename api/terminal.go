package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/google/uuid"

	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/models"
)

// ptySession holds the state for a running terminal PTY
type ptySession struct {
	ptmx    *os.File
	cmd     *exec.Cmd
	mu      sync.Mutex
	done    chan struct{}
	stopped bool
}

// TerminalManager manages active PTY sessions
type TerminalManager struct {
	sessions map[string]*ptySession // terminalID -> session
	mu       sync.Mutex
}

// NewTerminalManager creates a new terminal manager
func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*ptySession),
	}
}

// Start creates and starts a PTY session for a terminal
func (tm *TerminalManager) Start(terminalID string, cols, rows uint16, workDir string) (*ptySession, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if sess, ok := tm.sessions[terminalID]; ok {
		return sess, nil // already running
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if workDir != "" {
		cmd.Dir = workDir
	}

	winSize := &pty.Winsize{Rows: rows, Cols: cols}
	if cols == 0 {
		winSize.Cols = 80
	}
	if rows == 0 {
		winSize.Rows = 24
	}

	ptmx, err := pty.StartWithSize(cmd, winSize)
	if err != nil {
		return nil, err
	}

	sess := &ptySession{
		ptmx: ptmx,
		cmd:  cmd,
		done: make(chan struct{}),
	}
	tm.sessions[terminalID] = sess

	// Wait for process to exit in background
	go func() {
		_ = cmd.Wait()
		sess.mu.Lock()
		sess.stopped = true
		sess.mu.Unlock()
		close(sess.done)
	}()

	return sess, nil
}

// Write sends input to a terminal PTY
func (tm *TerminalManager) Write(terminalID string, data []byte) error {
	tm.mu.Lock()
	sess, ok := tm.sessions[terminalID]
	tm.mu.Unlock()
	if !ok {
		return io.ErrClosedPipe
	}
	_, err := sess.ptmx.Write(data)
	return err
}

// Resize changes the PTY window size
func (tm *TerminalManager) Resize(terminalID string, cols, rows uint16) error {
	tm.mu.Lock()
	sess, ok := tm.sessions[terminalID]
	tm.mu.Unlock()
	if !ok {
		return io.ErrClosedPipe
	}
	return pty.Setsize(sess.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// Stop terminates a terminal PTY session
func (tm *TerminalManager) Stop(terminalID string) {
	tm.mu.Lock()
	sess, ok := tm.sessions[terminalID]
	if !ok {
		tm.mu.Unlock()
		return
	}
	delete(tm.sessions, terminalID)
	tm.mu.Unlock()

	sess.ptmx.Close()
	if sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}
}

// Get returns the PTY session if it exists
func (tm *TerminalManager) Get(terminalID string) (*ptySession, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	sess, ok := tm.sessions[terminalID]
	return sess, ok
}

// ReadLoop reads from the PTY and calls the callback with output data.
// It blocks until the PTY closes or an error occurs.
func (tm *TerminalManager) ReadLoop(terminalID string, onData func(data []byte)) {
	tm.mu.Lock()
	sess, ok := tm.sessions[terminalID]
	tm.mu.Unlock()
	if !ok {
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := sess.ptmx.Read(buf)
		if n > 0 {
			// Ensure valid UTF-8 output for the browser
			data := buf[:n]
			if utf8.Valid(data) {
				onData(data)
			} else {
				// Send as-is; xterm.js handles binary
				onData(data)
			}
		}
		if err != nil {
			return
		}
	}
}

// --- REST API handlers ---

func listTerminals(w http.ResponseWriter, r *http.Request) {
	var terminals []models.Terminal
	db := database.Get()
	query := db.Order("created_at DESC")
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	result := query.Find(&terminals)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch terminals")
		return
	}
	writeJSON(w, http.StatusOK, terminals)
}

func createTerminal(w http.ResponseWriter, r *http.Request) {
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
		req.Name = "Terminal"
	}

	terminal := models.Terminal{
		ID:        uuid.New().String(),
		ProjectID: req.ProjectID,
		Name:      req.Name,
	}
	if err := database.Get().Create(&terminal).Error; err != nil {
		logger.Log.Errorw("failed to create terminal", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create terminal")
		return
	}
	writeJSON(w, http.StatusCreated, terminal)
}

func deleteTerminal(w http.ResponseWriter, r *http.Request, hub *Hub) {
	id := r.PathValue("id")

	// Stop any running PTY session
	hub.terminalManager.Stop(id)

	if err := database.Get().Delete(&models.Terminal{}, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete terminal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renameTerminal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	db := database.Get()
	result := db.Model(&models.Terminal{}).Where("id = ?", id).Update("name", req.Name)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename terminal")
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "terminal not found")
		return
	}

	var terminal models.Terminal
	db.First(&terminal, "id = ?", id)
	writeJSON(w, http.StatusOK, terminal)
}
