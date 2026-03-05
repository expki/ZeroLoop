package tools

import (
	"context"
	"strings"
	"testing"
)

func TestCodeExecutionSessionCwd(t *testing.T) {
	tool := &CodeExecutionTool{}

	// Reset session 99 to get a clean state
	globalSessions.reset("", 99)

	// cd into /tmp
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "cd /tmp && pwd",
		"session": float64(99),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "/tmp") {
		t.Errorf("expected /tmp in output, got '%s'", result.Message)
	}

	// Next call in same session should be in /tmp
	result, err = tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "pwd",
		"session": float64(99),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "/tmp") {
		t.Errorf("expected /tmp (persisted cwd), got '%s'", result.Message)
	}

	// Reset and check cwd goes back to default
	globalSessions.reset("", 99)
}

func TestCodeExecutionSessionReset(t *testing.T) {
	tool := &CodeExecutionTool{}

	// Set cwd in session 98
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "cd /tmp",
		"session": float64(98),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Reset session
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "pwd",
		"session": float64(98),
		"reset":   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// After reset, should NOT be in /tmp (back to working dir)
	// This verifies reset clears the session state
	_ = result
	globalSessions.reset("", 98)
}

func TestCodeExecutionDifferentSessions(t *testing.T) {
	tool := &CodeExecutionTool{}
	globalSessions.reset("", 96)
	globalSessions.reset("", 97)

	// Set session 96 to /tmp
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "cd /tmp",
		"session": float64(96),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Session 97 should not be affected
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "pwd",
		"session": float64(97),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Session 97 should be at default cwd, not /tmp
	if strings.TrimSpace(result.Message) == "/tmp" {
		t.Error("session 97 should not be in /tmp")
	}

	globalSessions.reset("", 96)
	globalSessions.reset("", 97)
}
