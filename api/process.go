package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// LogLine represents a single line of process output
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // "stdout" or "stderr"
	Text      string    `json:"text"`
}

// RingBuffer is a thread-safe, fixed-capacity circular buffer of log lines
type RingBuffer struct {
	lines []LogLine
	cap   int
	start int
	count int
	mu    sync.Mutex
}

// NewRingBuffer creates a ring buffer with the given capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		lines: make([]LogLine, capacity),
		cap:   capacity,
	}
}

// Append adds a log line to the ring buffer, evicting the oldest if full
func (rb *RingBuffer) Append(line LogLine) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	idx := (rb.start + rb.count) % rb.cap
	rb.lines[idx] = line
	if rb.count == rb.cap {
		rb.start = (rb.start + 1) % rb.cap
	} else {
		rb.count++
	}
}

// Lines returns all lines in order (oldest first)
func (rb *RingBuffer) Lines() []LogLine {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	result := make([]LogLine, rb.count)
	for i := 0; i < rb.count; i++ {
		result[i] = rb.lines[(rb.start+i)%rb.cap]
	}
	return result
}

// Tail returns the last n lines
func (rb *RingBuffer) Tail(n int) []LogLine {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if n > rb.count {
		n = rb.count
	}
	result := make([]LogLine, n)
	offset := rb.count - n
	for i := 0; i < n; i++ {
		result[i] = rb.lines[(rb.start+offset+i)%rb.cap]
	}
	return result
}

// ProcessInfo is the JSON-serializable summary of a process
type ProcessInfo struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Command   string     `json:"command"`
	Status    string     `json:"status"` // "running", "exited", "stopped"
	ExitCode  *int       `json:"exit_code,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	ExitedAt  *time.Time `json:"exited_at,omitempty"`
}

// processSession holds state for a running process
type processSession struct {
	id        string
	projectID string
	command   string
	cmd       *exec.Cmd
	buffer    *RingBuffer
	startedAt time.Time
	exitCode  *int
	exitedAt  *time.Time
	done      chan struct{}
	stopped   bool
	mu        sync.Mutex
}

func (s *processSession) info() ProcessInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := "running"
	if s.stopped {
		status = "stopped"
	} else if s.exitCode != nil {
		status = "exited"
	}
	return ProcessInfo{
		ID:        s.id,
		ProjectID: s.projectID,
		Command:   s.command,
		Status:    status,
		ExitCode:  s.exitCode,
		StartedAt: s.startedAt,
		ExitedAt:  s.exitedAt,
	}
}

const defaultRingBufferCap = 1000
const maxProcessesPerProject = 10
const maxProcessesGlobal = 50
const maxProcessLifetime = 24 * time.Hour

// ProcessManager manages background processes
type ProcessManager struct {
	sessions map[string]*processSession
	mu       sync.Mutex
}

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		sessions: make(map[string]*processSession),
	}
}

// Start launches a background process and returns its session
func (pm *ProcessManager) Start(processID, projectID, command, workDir string, onOutput func(processID string, line LogLine), onExit func(processID string, exitCode int)) (*processSession, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.sessions[processID]; ok {
		return nil, fmt.Errorf("process %s already exists", processID)
	}

	// Enforce process count limits
	runningCount := 0
	projectCount := 0
	for _, sess := range pm.sessions {
		sess.mu.Lock()
		alive := sess.exitCode == nil
		sess.mu.Unlock()
		if alive {
			runningCount++
			if sess.projectID == projectID {
				projectCount++
			}
		}
	}
	if runningCount >= maxProcessesGlobal {
		return nil, fmt.Errorf("global process limit reached (%d)", maxProcessesGlobal)
	}
	if projectCount >= maxProcessesPerProject {
		return nil, fmt.Errorf("project process limit reached (%d)", maxProcessesPerProject)
	}

	cmd := exec.Command("bash", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}
	// Disable stdout buffering for common runtimes (no PTY = block-buffered by default)
	cmd.Env = append(os.Environ(),
		"PYTHONUNBUFFERED=1",
		"NODE_NO_WARNINGS=1",
	)
	// Set process group so we can kill the entire tree
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	sess := &processSession{
		id:        processID,
		projectID: projectID,
		command:   command,
		cmd:       cmd,
		buffer:    NewRingBuffer(defaultRingBufferCap),
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}
	pm.sessions[processID] = sess

	// Read stdout in goroutine
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				line := LogLine{
					Timestamp: time.Now(),
					Stream:    "stdout",
					Text:      string(buf[:n]),
				}
				sess.buffer.Append(line)
				if onOutput != nil {
					onOutput(processID, line)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Read stderr in goroutine
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				line := LogLine{
					Timestamp: time.Now(),
					Stream:    "stderr",
					Text:      string(buf[:n]),
				}
				sess.buffer.Append(line)
				if onOutput != nil {
					onOutput(processID, line)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for process to exit in background
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		now := time.Now()
		sess.mu.Lock()
		sess.exitCode = &exitCode
		sess.exitedAt = &now
		sess.mu.Unlock()
		close(sess.done)
		if onExit != nil {
			onExit(processID, exitCode)
		}
	}()

	// Auto-kill after max lifetime
	go func() {
		select {
		case <-sess.done:
			return
		case <-time.After(maxProcessLifetime):
			pm.Stop(processID)
		}
	}()

	return sess, nil
}

// Stop terminates a process (SIGTERM, then SIGKILL after 5s)
func (pm *ProcessManager) Stop(processID string) {
	pm.mu.Lock()
	sess, ok := pm.sessions[processID]
	pm.mu.Unlock()
	if !ok {
		return
	}

	sess.mu.Lock()
	if sess.exitCode != nil {
		sess.mu.Unlock()
		return // already exited
	}
	sess.stopped = true
	sess.mu.Unlock()

	// Send SIGTERM to the process group
	if sess.cmd.Process != nil {
		_ = syscall.Kill(-sess.cmd.Process.Pid, syscall.SIGTERM)
	}

	// Wait up to 5s for graceful exit, then SIGKILL
	select {
	case <-sess.done:
		return
	case <-time.After(5 * time.Second):
		if sess.cmd.Process != nil {
			_ = syscall.Kill(-sess.cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}

// StopAll stops all processes for a project (empty projectID = stop all)
func (pm *ProcessManager) StopAll(projectID string) {
	pm.mu.Lock()
	var ids []string
	for id, sess := range pm.sessions {
		if projectID == "" || sess.projectID == projectID {
			ids = append(ids, id)
		}
	}
	pm.mu.Unlock()

	for _, id := range ids {
		pm.Stop(id)
	}
}

// Get returns a process session
func (pm *ProcessManager) Get(processID string) (*processSession, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	sess, ok := pm.sessions[processID]
	return sess, ok
}

// ListByProject returns all processes for a project
func (pm *ProcessManager) ListByProject(projectID string) []ProcessInfo {
	pm.mu.Lock()
	var sessions []*processSession
	for _, sess := range pm.sessions {
		if sess.projectID == projectID {
			sessions = append(sessions, sess)
		}
	}
	pm.mu.Unlock()
	var result []ProcessInfo
	for _, sess := range sessions {
		result = append(result, sess.info())
	}
	return result
}

// Cleanup removes stopped/exited processes older than the retention period
func (pm *ProcessManager) Cleanup(retention time.Duration) {
	pm.mu.Lock()
	type candidate struct {
		id   string
		sess *processSession
	}
	var candidates []candidate
	for id, sess := range pm.sessions {
		candidates = append(candidates, candidate{id, sess})
	}
	pm.mu.Unlock()

	var toDelete []string
	now := time.Now()
	for _, c := range candidates {
		c.sess.mu.Lock()
		if c.sess.exitCode != nil && c.sess.exitedAt != nil && now.Sub(*c.sess.exitedAt) > retention {
			toDelete = append(toDelete, c.id)
		}
		c.sess.mu.Unlock()
	}

	if len(toDelete) > 0 {
		pm.mu.Lock()
		for _, id := range toDelete {
			delete(pm.sessions, id)
		}
		pm.mu.Unlock()
	}
}

// GenerateID creates a new process ID
func GenerateProcessID() string {
	return uuid.New().String()
}

// --- REST API handlers ---

func listProcesses(w http.ResponseWriter, r *http.Request, pm *ProcessManager) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	processes := pm.ListByProject(projectID)
	if processes == nil {
		processes = []ProcessInfo{}
	}
	writeJSON(w, http.StatusOK, processes)
}

func getProcessLog(w http.ResponseWriter, r *http.Request, pm *ProcessManager) {
	id := r.PathValue("id")
	sess, ok := pm.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	// Parse optional tail parameter
	tailStr := r.URL.Query().Get("tail")
	if tailStr != "" {
		var tail int
		if _, err := fmt.Sscanf(tailStr, "%d", &tail); err == nil && tail > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"process": sess.info(),
				"lines":   sess.buffer.Tail(tail),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"process": sess.info(),
		"lines":   sess.buffer.Lines(),
	})
}

func stopProcess(w http.ResponseWriter, r *http.Request, pm *ProcessManager) {
	id := r.PathValue("id")
	sess, ok := pm.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}
	_ = sess // verify existence
	go pm.Stop(id) // non-blocking
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

// startCleanupLoop runs periodic cleanup of exited processes
func (pm *ProcessManager) startCleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			pm.Cleanup(5 * time.Minute)
		}
	}()
}

