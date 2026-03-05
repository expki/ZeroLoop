package tools

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/llm"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/mcp"
	"github.com/expki/ZeroLoop.git/search"
)

// --- Test helpers ---

// testAgent creates a minimal agent for tool testing with a log collector.
func testAgent(t *testing.T) (*agent.Agent, *logCollector) {
	t.Helper()
	lc := &logCollector{}
	a := agent.New(&llm.Client{}, agent.NewToolRegistry(), lc.callback)
	a.ChatID = "test-chat"
	return a, lc
}

type logCollector struct {
	mu      sync.Mutex
	entries []agent.LogEntry
}

func (lc *logCollector) callback(entry agent.LogEntry) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.entries = append(lc.entries, entry)
}

func (lc *logCollector) last() agent.LogEntry {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if len(lc.entries) == 0 {
		return agent.LogEntry{}
	}
	return lc.entries[len(lc.entries)-1]
}

func (lc *logCollector) count() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return len(lc.entries)
}

// --- TextEditorTool Integration Tests ---

func TestTextEditorWriteAndRead(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	content := "line one\nline two\nline three\n"

	// Write file
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":  "write",
		"path":    tmpFile,
		"content": content,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "File written") {
		t.Errorf("expected 'File written', got %q", result.Message)
	}

	// Read file back
	result, err = tool.Execute(context.Background(), nil, map[string]any{
		"method": "read",
		"path":   tmpFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "line one") {
		t.Errorf("expected 'line one' in read output, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "line two") {
		t.Errorf("expected 'line two' in read output, got %q", result.Message)
	}
}

func TestTextEditorWriteEmptyContent(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "empty.txt")

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":  "write",
		"path":    tmpFile,
		"content": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should return error about empty content
	if !strings.Contains(result.Message, "required") && !strings.Contains(result.Message, "non-empty") {
		t.Errorf("expected error about empty content, got %q", result.Message)
	}
	// File should NOT exist or be empty
	data, readErr := os.ReadFile(tmpFile)
	if readErr == nil && len(data) > 0 {
		t.Error("file should not have content when content param is empty")
	}
}

func TestTextEditorWriteMissingContent(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "nocontent.txt")

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "write",
		"path":   tmpFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "required") {
		t.Errorf("expected error about missing content, got %q", result.Message)
	}
}

func TestTextEditorWriteCreatesParentDirs(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "deep", "nested", "dir", "file.txt")

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":  "write",
		"path":    tmpFile,
		"content": "nested content",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "File written") {
		t.Errorf("expected success, got %q", result.Message)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested content" {
		t.Errorf("expected 'nested content', got %q", string(data))
	}
}

func TestTextEditorReadLineRange(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "lines.txt")
	os.WriteFile(tmpFile, []byte("a\nb\nc\nd\ne\n"), 0644)

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":     "read",
		"path":       tmpFile,
		"start_line": float64(2),
		"end_line":   float64(4),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "showing 2-4") {
		t.Errorf("expected range 2-4 in output, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "b") || !strings.Contains(result.Message, "d") {
		t.Errorf("expected lines b-d, got %q", result.Message)
	}
}

func TestTextEditorReadNonexistent(t *testing.T) {
	tool := &TextEditorTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "read",
		"path":   "/tmp/nonexistent_file_12345.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Error") {
		t.Errorf("expected error message, got %q", result.Message)
	}
}

func TestTextEditorPatch(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "patch.txt")
	os.WriteFile(tmpFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	// Read first (required for mtime tracking)
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "read",
		"path":   tmpFile,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Patch lines 2-3
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":     "patch",
		"path":       tmpFile,
		"content":    "replaced_a\nreplaced_b",
		"start_line": float64(2),
		"end_line":   float64(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Patched") {
		t.Errorf("expected 'Patched', got %q", result.Message)
	}

	data, _ := os.ReadFile(tmpFile)
	if !strings.Contains(string(data), "replaced_a") {
		t.Errorf("expected patched content, got %q", string(data))
	}
	if strings.Contains(string(data), "line2") {
		t.Errorf("old content should be replaced, got %q", string(data))
	}
}

func TestTextEditorPatchStaleDetection(t *testing.T) {
	tool := &TextEditorTool{}
	tmpFile := filepath.Join(t.TempDir(), "stale.txt")
	os.WriteFile(tmpFile, []byte("original\n"), 0644)

	// Read to track mtime
	_, _ = tool.Execute(context.Background(), nil, map[string]any{
		"method": "read",
		"path":   tmpFile,
	})

	// Modify file externally
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(tmpFile, []byte("modified externally\n"), 0644)

	// Patch should detect staleness
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":     "patch",
		"path":       tmpFile,
		"content":    "new content",
		"start_line": float64(1),
		"end_line":   float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "STALE") {
		t.Errorf("expected STALE detection, got %q", result.Message)
	}
}

func TestTextEditorPatchMissingLines(t *testing.T) {
	tool := &TextEditorTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":  "patch",
		"path":    "/tmp/anything.txt",
		"content": "x",
	})
	if err == nil {
		t.Error("expected error for missing start_line/end_line")
	}
}

func TestTextEditorInvalidMethod(t *testing.T) {
	tool := &TextEditorTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "delete",
		"path":   "/tmp/anything.txt",
	})
	if err == nil {
		t.Error("expected error for invalid method")
	}
}

// --- WaitTool Integration Tests ---

func TestWaitToolSeconds(t *testing.T) {
	tool := &WaitTool{}
	a, _ := testAgent(t)

	start := time.Now()
	result, err := tool.Execute(context.Background(), a, map[string]any{
		"seconds": float64(0.1),
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Waited") {
		t.Errorf("expected 'Waited', got %q", result.Message)
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("expected at least 80ms wait, got %v", elapsed)
	}
}

func TestWaitToolContextCancel(t *testing.T) {
	tool := &WaitTool{}
	a, _ := testAgent(t)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := tool.Execute(ctx, a, map[string]any{
		"seconds": float64(10),
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "cancelled") {
		t.Errorf("expected 'cancelled', got %q", result.Message)
	}
	if elapsed > 2*time.Second {
		t.Errorf("should have cancelled quickly, took %v", elapsed)
	}
}

func TestWaitToolPastTimestamp(t *testing.T) {
	tool := &WaitTool{}
	a, _ := testAgent(t)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	result, err := tool.Execute(context.Background(), a, map[string]any{
		"until": past,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "already passed") {
		t.Errorf("expected 'already passed', got %q", result.Message)
	}
}

func TestWaitToolMissingParams(t *testing.T) {
	tool := &WaitTool{}
	a, _ := testAgent(t)

	_, err := tool.Execute(context.Background(), a, map[string]any{})
	if err == nil {
		t.Error("expected error for missing seconds/until")
	}
}

func TestWaitToolInvalidTimestamp(t *testing.T) {
	tool := &WaitTool{}
	a, _ := testAgent(t)

	_, err := tool.Execute(context.Background(), a, map[string]any{
		"until": "not-a-timestamp",
	})
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestWaitToolMaxCap(t *testing.T) {
	tool := &WaitTool{}
	a, _ := testAgent(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Request 99999 seconds - should cap at 10 minutes, then cancel via context
	result, err := tool.Execute(ctx, a, map[string]any{
		"seconds": float64(99999),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should be cancelled by context, not wait the full duration
	if !strings.Contains(result.Message, "cancelled") {
		t.Errorf("expected cancellation, got %q", result.Message)
	}
}

// --- NotifyUserTool Integration Tests ---

func TestNotifyUserTool(t *testing.T) {
	tool := &NotifyUserTool{}
	a, lc := testAgent(t)

	result, err := tool.Execute(context.Background(), a, map[string]any{
		"type":   "info",
		"title":  "Test Notification",
		"detail": "Some detail text",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Notification sent") {
		t.Errorf("expected success, got %q", result.Message)
	}
	if lc.count() == 0 {
		t.Error("expected log entry from notification")
	}
	entry := lc.last()
	if entry.Type != "info" {
		t.Errorf("expected type 'info', got %q", entry.Type)
	}
	if entry.Heading != "Test Notification" {
		t.Errorf("expected heading 'Test Notification', got %q", entry.Heading)
	}
}

func TestNotifyUserToolWarning(t *testing.T) {
	tool := &NotifyUserTool{}
	a, lc := testAgent(t)

	_, err := tool.Execute(context.Background(), a, map[string]any{
		"type":  "warning",
		"title": "Warning Test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if lc.last().Type != "warning" {
		t.Errorf("expected 'warning' log type, got %q", lc.last().Type)
	}
}

func TestNotifyUserToolError(t *testing.T) {
	tool := &NotifyUserTool{}
	a, lc := testAgent(t)

	_, err := tool.Execute(context.Background(), a, map[string]any{
		"type":  "error",
		"title": "Error Test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if lc.last().Type != "error" {
		t.Errorf("expected 'error' log type, got %q", lc.last().Type)
	}
}

func TestNotifyUserToolMissingTitle(t *testing.T) {
	tool := &NotifyUserTool{}
	a, _ := testAgent(t)

	_, err := tool.Execute(context.Background(), a, map[string]any{
		"type": "info",
	})
	if err == nil {
		t.Error("expected error for missing title")
	}
}

// --- SchedulerTool Integration Tests ---

func TestSchedulerCreateAndListTasks(t *testing.T) {
	tool := &SchedulerTool{}

	// Clear task store
	taskStore.Lock()
	taskStore.tasks = make(map[string]*ScheduledTask)
	taskStore.Unlock()

	// Create adhoc task
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":      "create_adhoc_task",
		"name":        "Test Task",
		"description": "Do something",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Created adhoc task") {
		t.Errorf("expected creation message, got %q", result.Message)
	}

	// List tasks
	result, err = tool.Execute(context.Background(), nil, map[string]any{
		"method": "list_tasks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Test Task") {
		t.Errorf("expected task in list, got %q", result.Message)
	}
}

func TestSchedulerCreateScheduledTask(t *testing.T) {
	tool := &SchedulerTool{}
	taskStore.Lock()
	taskStore.tasks = make(map[string]*ScheduledTask)
	taskStore.Unlock()

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":      "create_scheduled_task",
		"name":        "Cron Task",
		"description": "Run every 5 minutes",
		"schedule":    "*/5 * * * *",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Created scheduled task") {
		t.Errorf("expected creation, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "*/5 * * * *") {
		t.Errorf("expected schedule in message, got %q", result.Message)
	}
}

func TestSchedulerCreatePlannedTask(t *testing.T) {
	tool := &SchedulerTool{}
	taskStore.Lock()
	taskStore.tasks = make(map[string]*ScheduledTask)
	taskStore.Unlock()

	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":        "create_planned_task",
		"name":          "Planned Task",
		"description":   "Run at specific time",
		"planned_times": future,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Created planned task") {
		t.Errorf("expected creation, got %q", result.Message)
	}
}

func TestSchedulerPlannedTaskInvalidTime(t *testing.T) {
	tool := &SchedulerTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":        "create_planned_task",
		"name":          "Bad Task",
		"description":   "desc",
		"planned_times": "not-a-time",
	})
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestSchedulerFindTaskByName(t *testing.T) {
	tool := &SchedulerTool{}
	taskStore.Lock()
	taskStore.tasks = make(map[string]*ScheduledTask)
	taskStore.Unlock()

	// Create task
	tool.Execute(context.Background(), nil, map[string]any{
		"method":      "create_adhoc_task",
		"name":        "Unique Find Me",
		"description": "desc",
	})

	// Find it
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "find_task_by_name",
		"name":   "find me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Unique Find Me") {
		t.Errorf("expected to find task, got %q", result.Message)
	}
}

func TestSchedulerShowAndDeleteTask(t *testing.T) {
	tool := &SchedulerTool{}
	taskStore.Lock()
	taskStore.tasks = make(map[string]*ScheduledTask)
	taskStore.Unlock()

	// Create task
	result, _ := tool.Execute(context.Background(), nil, map[string]any{
		"method":      "create_adhoc_task",
		"name":        "To Delete",
		"description": "will be deleted",
	})
	// Extract task ID from result
	var taskID string
	taskStore.Lock()
	for id := range taskStore.tasks {
		taskID = id
	}
	taskStore.Unlock()

	// Show task
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":  "show_task",
		"task_id": taskID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "To Delete") {
		t.Errorf("expected task details, got %q", result.Message)
	}

	// Delete task
	result, err = tool.Execute(context.Background(), nil, map[string]any{
		"method":  "delete_task",
		"task_id": taskID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Deleted") {
		t.Errorf("expected deletion, got %q", result.Message)
	}

	// Verify deleted
	result, _ = tool.Execute(context.Background(), nil, map[string]any{
		"method":  "show_task",
		"task_id": taskID,
	})
	if !strings.Contains(result.Message, "not found") {
		t.Errorf("expected 'not found', got %q", result.Message)
	}
}

func TestSchedulerListEmpty(t *testing.T) {
	tool := &SchedulerTool{}
	taskStore.Lock()
	taskStore.tasks = make(map[string]*ScheduledTask)
	taskStore.Unlock()

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "list_tasks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "No scheduled tasks") {
		t.Errorf("expected empty list, got %q", result.Message)
	}
}

func TestSchedulerInvalidMethod(t *testing.T) {
	tool := &SchedulerTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "invalid_method",
	})
	if err == nil {
		t.Error("expected error for invalid method")
	}
}

func TestSchedulerMissingRequired(t *testing.T) {
	tool := &SchedulerTool{}

	// Missing name for create
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method":      "create_adhoc_task",
		"description": "desc only",
	})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Missing task_id for show
	_, err = tool.Execute(context.Background(), nil, map[string]any{
		"method": "show_task",
	})
	if err == nil {
		t.Error("expected error for missing task_id")
	}

	// Missing task_id for delete
	_, err = tool.Execute(context.Background(), nil, map[string]any{
		"method": "delete_task",
	})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

// --- SkillsTool Integration Tests ---

func TestSkillsListNoDir(t *testing.T) {
	tool := &SkillsTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "list",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should handle missing dir gracefully
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSkillsListWithSkills(t *testing.T) {
	tool := &SkillsTool{}
	tmpDir := t.TempDir()

	// Create skill files
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "coding.md"), []byte("# Coding Skill\nUse best practices."), 0644)

	dirSkill := filepath.Join(skillsDir, "research")
	os.MkdirAll(dirSkill, 0755)
	os.WriteFile(filepath.Join(dirSkill, "SKILL.md"), []byte("# Research Skill\nDo thorough research."), 0644)

	// Override skills dir for test
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "list",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "coding") {
		t.Errorf("expected 'coding' skill, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "research") {
		t.Errorf("expected 'research' skill, got %q", result.Message)
	}
}

func TestSkillsLoadAndLimit(t *testing.T) {
	tool := &SkillsTool{}
	a, _ := testAgent(t)
	a.Number = 999 // unique agent number for test
	tmpDir := t.TempDir()

	// Clear loaded skills for this agent
	loadedSkills.Lock()
	delete(loadedSkills.m, 999)
	loadedSkills.Unlock()

	// Create skills
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	for i := 0; i < 6; i++ {
		os.WriteFile(filepath.Join(skillsDir, fmt.Sprintf("skill%d.md", i)), []byte(fmt.Sprintf("# Skill %d content", i)), 0644)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Load 5 skills (max)
	for i := 0; i < 5; i++ {
		result, err := tool.Execute(context.Background(), a, map[string]any{
			"method": "load",
			"name":   fmt.Sprintf("skill%d", i),
		})
		if err != nil {
			t.Fatalf("loading skill%d: %v", i, err)
		}
		if !strings.Contains(result.Message, "loaded") {
			t.Errorf("expected loaded message for skill%d, got %q", i, result.Message)
		}
	}

	// 6th should fail (limit)
	result, err := tool.Execute(context.Background(), a, map[string]any{
		"method": "load",
		"name":   "skill5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Maximum") {
		t.Errorf("expected max limit message, got %q", result.Message)
	}

	// Loading same skill again should say already loaded
	result, err = tool.Execute(context.Background(), a, map[string]any{
		"method": "load",
		"name":   "skill0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "already loaded") {
		t.Errorf("expected 'already loaded', got %q", result.Message)
	}

	// Cleanup
	loadedSkills.Lock()
	delete(loadedSkills.m, 999)
	loadedSkills.Unlock()
}

func TestSkillsLoadNotFound(t *testing.T) {
	tool := &SkillsTool{}
	a, _ := testAgent(t)
	a.Number = 998

	loadedSkills.Lock()
	delete(loadedSkills.m, 998)
	loadedSkills.Unlock()

	result, err := tool.Execute(context.Background(), a, map[string]any{
		"method": "load",
		"name":   "nonexistent_skill",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "not found") {
		t.Errorf("expected 'not found', got %q", result.Message)
	}

	loadedSkills.Lock()
	delete(loadedSkills.m, 998)
	loadedSkills.Unlock()
}

func TestSkillsInvalidMethod(t *testing.T) {
	tool := &SkillsTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"method": "delete",
	})
	if err == nil {
		t.Error("expected error for invalid method")
	}
}

func TestSkillsLoadMissingName(t *testing.T) {
	tool := &SkillsTool{}
	a, _ := testAgent(t)

	_, err := tool.Execute(context.Background(), a, map[string]any{
		"method": "load",
	})
	if err == nil {
		t.Error("expected error for missing name")
	}
}

// --- DocumentQueryTool Integration Tests ---

func TestDocumentQueryContentMode(t *testing.T) {
	tool := &DocumentQueryTool{}
	tmpFile := filepath.Join(t.TempDir(), "doc.txt")
	os.WriteFile(tmpFile, []byte("This is a test document with important information."), 0644)

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"uri":  tmpFile,
		"mode": "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "important information") {
		t.Errorf("expected content in result, got %q", result.Message)
	}
}

func TestDocumentQueryDefaultMode(t *testing.T) {
	tool := &DocumentQueryTool{}
	tmpFile := filepath.Join(t.TempDir(), "doc2.txt")
	os.WriteFile(tmpFile, []byte("Default mode content."), 0644)

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"uri": tmpFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Default mode content") {
		t.Errorf("expected content (default mode=content), got %q", result.Message)
	}
}

func TestDocumentQueryMissingURI(t *testing.T) {
	tool := &DocumentQueryTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{})
	if err == nil {
		t.Error("expected error for missing uri")
	}
}

func TestDocumentQueryFileNotFound(t *testing.T) {
	tool := &DocumentQueryTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"uri": "/tmp/nonexistent_doc_99999.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Error") {
		t.Errorf("expected error message, got %q", result.Message)
	}
}

func TestDocumentQueryMissingQuestionForQueryMode(t *testing.T) {
	tool := &DocumentQueryTool{}
	tmpFile := filepath.Join(t.TempDir(), "doc3.txt")
	os.WriteFile(tmpFile, []byte("content"), 0644)

	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"uri":  tmpFile,
		"mode": "query",
	})
	if err == nil {
		t.Error("expected error for query mode without question")
	}
}

func TestStripHTML(t *testing.T) {
	input := `<html><head><title>Test</title><script>alert('x')</script><style>.x{}</style></head><body><p>Hello <b>World</b></p></body></html>`
	result := stripHTML(input)
	if strings.Contains(result, "<") {
		t.Errorf("expected no HTML tags, got %q", result)
	}
	if strings.Contains(result, "alert") {
		t.Errorf("expected script content removed, got %q", result)
	}
	if !strings.Contains(result, "Hello") || !strings.Contains(result, "World") {
		t.Errorf("expected text preserved, got %q", result)
	}
}

// --- BehaviourAdjustmentTool Integration Tests ---

func TestBehaviourViewNoRules(t *testing.T) {
	tool := &BehaviourAdjustmentTool{}
	// Ensure no behaviour file in cwd
	path := getBehaviourFilePath()
	os.Remove(path)

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "view",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "No behavioral rules") {
		t.Errorf("expected no rules message, got %q", result.Message)
	}
}

func TestBehaviourResetNonexistent(t *testing.T) {
	tool := &BehaviourAdjustmentTool{}
	path := getBehaviourFilePath()
	os.Remove(path)

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "reset",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "reset") {
		t.Errorf("expected reset message, got %q", result.Message)
	}
}

func TestBehaviourInvalidAction(t *testing.T) {
	tool := &BehaviourAdjustmentTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestBehaviourUpdateMissingRules(t *testing.T) {
	tool := &BehaviourAdjustmentTool{}
	a, _ := testAgent(t)
	_, err := tool.Execute(context.Background(), a, map[string]any{
		"action": "update",
	})
	if err == nil {
		t.Error("expected error for missing rules")
	}
}

// --- InputTool Integration Tests ---

func TestInputToolDelegatesToCodeExecution(t *testing.T) {
	tool := &InputTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"text": "echo input_test_42",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "input_test_42") {
		t.Errorf("expected output from delegated execution, got %q", result.Message)
	}
}

func TestInputToolWithSession(t *testing.T) {
	tool := &InputTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"text":    "echo session_input",
		"session": float64(50),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "session_input") {
		t.Errorf("expected output, got %q", result.Message)
	}
}

// --- VisionTool Integration Tests ---

func TestVisionToolLoadImage(t *testing.T) {
	tool := &VisionTool{}
	a, _ := testAgent(t)

	// Create a small test PNG image
	tmpFile := filepath.Join(t.TempDir(), "test.png")
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	f, _ := os.Create(tmpFile)
	png.Encode(f, img)
	f.Close()

	histLenBefore := len(a.History)
	result, err := tool.Execute(context.Background(), a, map[string]any{
		"path":     tmpFile,
		"question": "What color is this?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Image loaded") {
		t.Errorf("expected image loaded message, got %q", result.Message)
	}
	// Should have injected a message into history
	if len(a.History) != histLenBefore+1 {
		t.Errorf("expected history to grow by 1, grew by %d", len(a.History)-histLenBefore)
	}
}

func TestVisionToolMissingPath(t *testing.T) {
	tool := &VisionTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestVisionToolNonexistentFile(t *testing.T) {
	tool := &VisionTool{}
	a, _ := testAgent(t)
	result, err := tool.Execute(context.Background(), a, map[string]any{
		"path": "/tmp/nonexistent_image_999.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Error") {
		t.Errorf("expected error message, got %q", result.Message)
	}
}

// --- WebSearchTool Integration Tests ---

func TestWebSearchNoConfig(t *testing.T) {
	tool := &WebSearchTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"query": "test search",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "not configured") {
		t.Errorf("expected 'not configured' message, got %q", result.Message)
	}
}

func TestWebSearchMissingQuery(t *testing.T) {
	tool := &WebSearchTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{})
	if err == nil {
		t.Error("expected error for missing query")
	}
}

// --- BrowserTool Integration Tests ---

func TestBrowserToolScriptGeneration(t *testing.T) {
	tool := &BrowserTool{}

	tests := []struct {
		action string
		args   map[string]any
		expect string
	}{
		{"navigate", map[string]any{"url": "https://example.com"}, "page.goto"},
		{"click", map[string]any{"selector": "#btn", "url": "https://example.com"}, "page.click"},
		{"type", map[string]any{"selector": "#input", "text": "hello", "url": "https://example.com"}, "page.fill"},
		{"screenshot", map[string]any{"url": "https://example.com"}, "page.screenshot"},
		{"extract", map[string]any{"url": "https://example.com"}, "inner_text"},
		{"evaluate", map[string]any{"text": "document.title", "url": "https://example.com"}, "page.evaluate"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			script := tool.buildPlaywrightScript(tt.action, tt.args)
			if !strings.Contains(script, tt.expect) {
				t.Errorf("action %q: expected %q in script, got %q", tt.action, tt.expect, script)
			}
			if !strings.Contains(script, "sync_playwright") {
				t.Errorf("expected playwright import in script")
			}
			if !strings.Contains(script, "browser.close()") {
				t.Errorf("expected browser.close() in script")
			}
		})
	}
}

func TestBrowserToolExtractWithSelector(t *testing.T) {
	tool := &BrowserTool{}
	script := tool.buildPlaywrightScript("extract", map[string]any{
		"selector": "div.content",
		"url":      "https://example.com",
	})
	if !strings.Contains(script, "query_selector") {
		t.Errorf("expected query_selector for extract with selector, got %q", script)
	}
}

// --- MemoryTool Integration Tests ---

func TestMemorySaveLoadDelete(t *testing.T) {
	// Initialize logger (required by search)
	logger.Init(true, "")
	defer logger.Sync()

	// Initialize search index in temp dir
	tmpDir := t.TempDir()
	os.Setenv("SEARCH_DIR", filepath.Join(tmpDir, "search"))
	defer os.Unsetenv("SEARCH_DIR")

	if err := search.Init(); err != nil {
		t.Skipf("search init failed: %v", err)
	}
	defer search.Close()

	tool := &MemoryTool{}

	// Save a memory
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"action":  "save",
		"content": "The capital of France is Paris",
		"heading": "Geography Fact",
		"area":    "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "Memory saved") {
		t.Errorf("expected save success, got %q", result.Message)
	}

	// Extract ID from save result
	if !strings.Contains(result.Message, "id:") {
		t.Errorf("expected ID in save result, got %q", result.Message)
	}

	// Load memories
	result, err = tool.Execute(context.Background(), nil, map[string]any{
		"action": "load",
		"query":  "capital France Paris",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should find the memory
	if !strings.Contains(result.Message, "Paris") && !strings.Contains(result.Message, "No memories") {
		t.Errorf("expected Paris or no results, got %q", result.Message)
	}
}

func TestMemorySaveMissingContent(t *testing.T) {
	tool := &MemoryTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "save",
	})
	if err == nil {
		t.Error("expected error for missing content")
	}
}

func TestMemoryLoadMissingQuery(t *testing.T) {
	tool := &MemoryTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "load",
	})
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestMemoryDeleteMissingIDs(t *testing.T) {
	tool := &MemoryTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "delete",
	})
	if err == nil {
		t.Error("expected error for missing ids")
	}
}

func TestMemoryForgetMissingQuery(t *testing.T) {
	tool := &MemoryTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "forget",
	})
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestMemoryInvalidAction(t *testing.T) {
	tool := &MemoryTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

// --- MCP Tool Adapter Tests ---

func TestMCPToolAdapterName(t *testing.T) {
	tool := NewMCPToolAdapter(mcp.MCPTool{
		Name:        "test_tool",
		Description: "A test tool",
		ServerName:  "test_server",
	}, nil)
	if tool.Name() != "mcp_test_tool" {
		t.Errorf("expected 'mcp_test_tool', got %q", tool.Name())
	}
}

func TestMCPToolAdapterDescription(t *testing.T) {
	tool := NewMCPToolAdapter(mcp.MCPTool{
		Name:        "test_tool",
		Description: "A test tool",
		ServerName:  "test_server",
	}, nil)
	desc := tool.Description()
	if !strings.Contains(desc, "test_server") {
		t.Errorf("expected server name in description, got %q", desc)
	}
	if !strings.Contains(desc, "A test tool") {
		t.Errorf("expected tool description, got %q", desc)
	}
}

func TestMCPToolAdapterParametersNilSchema(t *testing.T) {
	tool := NewMCPToolAdapter(mcp.MCPTool{
		Name: "no_schema",
	}, nil)
	params := tool.Parameters()
	if params == nil {
		t.Fatal("expected non-nil parameters")
	}
	schema, ok := params.(map[string]any)
	if !ok {
		t.Fatal("expected map schema")
	}
	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}
}

func TestMCPToolAdapterParametersCustomSchema(t *testing.T) {
	tool := NewMCPToolAdapter(mcp.MCPTool{
		Name:        "with_schema",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}},
	}, nil)
	params := tool.Parameters()
	schema, ok := params.(map[string]any)
	if !ok {
		t.Fatal("expected map schema")
	}
	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties")
	}
	if _, exists := props["x"]; !exists {
		t.Error("expected property 'x'")
	}
}

// --- All Tools: Interface Compliance Tests ---

func TestAllToolsImplementInterface(t *testing.T) {
	// Verify every tool satisfies the Tool interface
	tools := []agent.Tool{
		&ResponseTool{},
		&CodeExecutionTool{},
		&CallSubordinateTool{},
		&KnowledgeTool{},
		&MemoryTool{},
		&TextEditorTool{},
		&InputTool{},
		&WebSearchTool{},
		&BrowserTool{},
		&DocumentQueryTool{},
		&WaitTool{},
		&NotifyUserTool{},
		&SkillsTool{},
		&SchedulerTool{},
		&BehaviourAdjustmentTool{},
		&VisionTool{},
	}

	for _, tool := range tools {
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.Name() == "" {
				t.Error("Name() should not be empty")
			}
			if tool.Description() == "" {
				t.Error("Description() should not be empty")
			}
			params := tool.Parameters()
			if params == nil {
				t.Error("Parameters() should not be nil")
			}
			schema, ok := params.(map[string]any)
			if !ok {
				t.Fatal("Parameters() should return map[string]any")
			}
			if schema["type"] != "object" {
				t.Errorf("Parameters type should be 'object', got %v", schema["type"])
			}
			props, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatal("Parameters should have 'properties' map")
			}
			if len(props) == 0 {
				t.Error("Parameters should have at least one property")
			}
		})
	}
}

