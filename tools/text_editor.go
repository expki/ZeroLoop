package tools

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/filemanager"
	"github.com/expki/ZeroLoop.git/models"
	"github.com/google/uuid"
)

// fileMtimes tracks last known modification times for stale detection on patches.
// Evicts oldest entries when exceeding maxFileMtimes to prevent unbounded growth.
var fileMtimes = struct {
	sync.Mutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

const maxFileMtimes = 500

func trackFileMtime(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	fileMtimes.Lock()
	defer fileMtimes.Unlock()
	fileMtimes.m[path] = info.ModTime()
	// Evict random entries if map is too large (simple bounded cache)
	if len(fileMtimes.m) > maxFileMtimes {
		for k := range fileMtimes.m {
			delete(fileMtimes.m, k)
			if len(fileMtimes.m) <= maxFileMtimes {
				break
			}
		}
	}
}

type TextEditorTool struct {
	FM *filemanager.FileManager
}

func (t *TextEditorTool) Name() string { return "text_editor" }

func (t *TextEditorTool) Description() string {
	return "Read, write, or patch text files. Methods: 'read' (path required), 'write' (path AND text required — text is the full file content to write), 'patch' (path, text, start_line, end_line required)."
}

func (t *TextEditorTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"enum":        []string{"read", "write", "patch"},
				"description": "Operation to perform",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File path (absolute or relative to working directory)",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "For write: full file content to write. For patch: replacement text for the specified line range.",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "For read/patch: starting line number (1-based, default 1)",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "For read/patch: ending line number (inclusive). For read: default is end of file.",
			},
		},
		"required": []string{"method", "path"},
	}
}

func (t *TextEditorTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	method, _ := args["method"].(string)
	path, _ := args["path"].(string)
	if path == "" {
		return nil, fmt.Errorf(`path is required. Example: {"method": "read", "path": "file.txt"}`)
	}

	// Resolve path based on project context
	if a != nil && a.ProjectID != "" && t.FM != nil {
		// If the model provides an absolute path that starts with the project dir,
		// strip the prefix to make it relative (ValidatePath only accepts relative paths)
		projectDir := t.FM.ProjectDir(a.ProjectID)
		if filepath.IsAbs(path) && strings.HasPrefix(path, projectDir+string(filepath.Separator)) {
			path = strings.TrimPrefix(path, projectDir+string(filepath.Separator))
		}
		// Project-scoped: validate and resolve within project directory
		absPath, err := t.FM.ValidatePath(a.ProjectID, path)
		if err != nil {
			return &agent.ToolResult{
				Message:   fmt.Sprintf("Invalid path: %s", err.Error()),
				BreakLoop: false,
			}, nil
		}
		path = absPath
	} else if !filepath.IsAbs(path) {
		// Non-project: resolve relative to cwd with basic traversal prevention
		cleaned := filepath.Clean(path)
		if strings.HasPrefix(cleaned, "..") {
			return &agent.ToolResult{
				Message:   "Path traversal not allowed",
				BreakLoop: false,
			}, nil
		}
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, cleaned)
	}

	var result *agent.ToolResult
	var err error

	switch method {
	case "read":
		if t.FM != nil {
			t.FM.Locks.RLock(path)
			defer t.FM.Locks.RUnlock(path)
		}
		result, err = t.executeRead(path, args)
	case "write":
		if t.FM != nil {
			t.FM.Locks.Lock(path)
			defer t.FM.Locks.Unlock(path)
		}
		result, err = t.executeWrite(path, args)
		if err == nil && result != nil && a != nil && a.ProjectID != "" {
			t.notifyFileEvent(a, path, "changed")
		}
	case "patch":
		if t.FM != nil {
			t.FM.Locks.Lock(path)
			defer t.FM.Locks.Unlock(path)
		}
		result, err = t.executePatch(path, args)
		if err == nil && result != nil && a != nil && a.ProjectID != "" && !strings.HasPrefix(result.Message, "STALE:") && !strings.HasPrefix(result.Message, "Error") && !strings.HasPrefix(result.Message, "Line range") {
			t.notifyFileEvent(a, path, "changed")
		}
	default:
		return nil, fmt.Errorf("invalid method: %s (use 'read', 'write', or 'patch')", method)
	}

	return result, err
}

// notifyFileEvent updates DB metadata and broadcasts a file event via WebSocket.
func (t *TextEditorTool) notifyFileEvent(a *agent.Agent, absPath, action string) {
	if a.ProjectDir == "" {
		return
	}

	relPath, err := filepath.Rel(a.ProjectDir, absPath)
	if err != nil {
		return
	}

	// Get file info
	var size int64
	if info, err := os.Stat(absPath); err == nil {
		size = info.Size()
	}

	name := filepath.Base(relPath)
	mimeType := mime.TypeByExtension(filepath.Ext(name))

	// Determine if this is a new file or update
	db := database.Get()
	var existing models.ProjectFile
	if db.Where("project_id = ? AND path = ?", a.ProjectID, relPath).First(&existing).Error != nil {
		// New file — create metadata
		action = "created"
		pf := models.ProjectFile{
			ID:        uuid.New().String(),
			ProjectID: a.ProjectID,
			Path:      relPath,
			Name:      name,
			Size:      size,
			MimeType:  mimeType,
		}
		db.Create(&pf)
	} else {
		// Existing file — update metadata
		db.Model(&existing).Updates(map[string]any{"size": size})
	}

	// Broadcast via WebSocket
	if a.FileEventCallback != nil {
		a.FileEventCallback(agent.FileEvent{
			ProjectID: a.ProjectID,
			Path:      relPath,
			Name:      name,
			Size:      size,
			Action:    action,
		})
	}
}

func (t *TextEditorTool) executeRead(path string, args map[string]any) (*agent.ToolResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading file: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	// Track mtime for stale detection
	trackFileMtime(path)

	lines := strings.Split(string(data), "\n")

	startLine := 1
	if s, ok := args["start_line"].(float64); ok && s > 0 {
		startLine = int(s)
	}
	endLine := len(lines)
	if e, ok := args["end_line"].(float64); ok && e > 0 {
		endLine = int(e)
	}

	// Clamp ranges
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Invalid range: start_line %d > end_line %d", startLine, endLine),
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s (%d lines total, showing %d-%d)\n\n", path, len(lines), startLine, endLine))

	lineNumWidth := len(strconv.Itoa(endLine))
	for i := startLine - 1; i < endLine; i++ {
		sb.WriteString(fmt.Sprintf("%*d | %s\n", lineNumWidth, i+1, lines[i]))
	}

	output := sb.String()
	if len(output) > 15000 {
		output = output[:15000] + "\n... (output truncated)"
	}

	return &agent.ToolResult{
		Message:   output,
		BreakLoop: false,
	}, nil
}

func (t *TextEditorTool) executeWrite(path string, args map[string]any) (*agent.ToolResult, error) {
	content, ok := args["text"].(string)
	if !ok || content == "" {
		return &agent.ToolResult{
			Message:   `Error: 'text' parameter is required for write method. You must include the file content to write in the 'text' field. Example: {"method": "write", "path": "file.txt", "text": "your text here"}`,
			BreakLoop: false,
		}, nil
	}

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error creating directories: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error writing file: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	// Update mtime tracking
	trackFileMtime(path)

	lines := strings.Count(content, "\n") + 1
	return &agent.ToolResult{
		Message:   fmt.Sprintf("File written: %s (%d lines)", path, lines),
		BreakLoop: false,
	}, nil
}

func (t *TextEditorTool) executePatch(path string, args map[string]any) (*agent.ToolResult, error) {
	content, _ := args["text"].(string)

	startLine := 0
	if s, ok := args["start_line"].(float64); ok {
		startLine = int(s)
	}
	endLine := 0
	if e, ok := args["end_line"].(float64); ok {
		endLine = int(e)
	}
	if startLine <= 0 || endLine <= 0 {
		return nil, fmt.Errorf(`start_line and end_line are required for patch (1-based). Example: {"method": "patch", "path": "file.txt", "text": "new content", "start_line": 5, "end_line": 10}`)
	}

	// Stale detection: check if file was modified since last read
	fileMtimes.Lock()
	lastMtime, tracked := fileMtimes.m[path]
	fileMtimes.Unlock()

	if tracked {
		if info, err := os.Stat(path); err == nil {
			if info.ModTime().After(lastMtime) {
				return &agent.ToolResult{
					Message:   fmt.Sprintf("STALE: File %s was modified since last read. Please read the file again before patching.", path),
					BreakLoop: false,
				}, nil
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading file: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	lines := strings.Split(string(data), "\n")
	if startLine > len(lines) || endLine > len(lines) {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Line range %d-%d out of bounds (file has %d lines)", startLine, endLine, len(lines)),
			BreakLoop: false,
		}, nil
	}

	// Replace lines [startLine, endLine] (1-based inclusive) with content
	newLines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines)-endLine+startLine-1+len(newLines))
	result = append(result, lines[:startLine-1]...)
	result = append(result, newLines...)
	result = append(result, lines[endLine:]...)

	output := strings.Join(result, "\n")
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error writing file: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	// Update mtime
	trackFileMtime(path)

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Patched %s: replaced lines %d-%d with %d new lines", path, startLine, endLine, len(newLines)),
		BreakLoop: false,
	}, nil
}
