package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/expki/ZeroLoop.git/agent"
)

// ProcessCheckResult holds status and output from a process check
type ProcessCheckResult struct {
	ID       string
	Command  string
	Status   string // "running", "exited", "stopped"
	ExitCode *int
	Lines    []ProcessOutputLine
}

// ProcessOutputLine is a single line of output with stream info
type ProcessOutputLine struct {
	Stream string // "stdout" or "stderr"
	Text   string
}

// ProcessCheckerFunc checks a process's status and returns tail output.
// Injected from the Hub to avoid circular imports.
type ProcessCheckerFunc func(processID string, tailLines int) (*ProcessCheckResult, error)

// CheckProcessTool checks the status of a background process
type CheckProcessTool struct {
	Checker ProcessCheckerFunc
}

func (t *CheckProcessTool) Name() string { return "check_process" }

func (t *CheckProcessTool) Description() string {
	return "Check the status of a background process started with run_process. Returns whether the process is still running, its exit code if exited, and the last N lines of output."
}

func (t *CheckProcessTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"process_id": map[string]any{
				"type":        "string",
				"description": "The process ID returned by run_process",
			},
			"tail_lines": map[string]any{
				"type":        "integer",
				"description": "Number of recent output lines to return (default: 50)",
				"default":     50,
			},
		},
		"required": []string{"process_id"},
	}
}

func (t *CheckProcessTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	processID, _ := args["process_id"].(string)
	if processID == "" {
		return nil, fmt.Errorf(`process_id is required. Example: {"process_id": "abc-123"}`)
	}

	tailLines := 50
	if tl, ok := args["tail_lines"].(float64); ok && tl > 0 {
		tailLines = int(tl)
	}

	if t.Checker == nil {
		return nil, fmt.Errorf("process checker not configured")
	}

	result, err := t.Checker(processID, tailLines)
	if err != nil {
		return &agent.ToolResult{
			Message: fmt.Sprintf("Error: %s", err.Error()),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Process: %s\nCommand: %s\nStatus: %s\n", result.ID, result.Command, result.Status))
	if result.ExitCode != nil {
		sb.WriteString(fmt.Sprintf("Exit Code: %d\n", *result.ExitCode))
	}

	if len(result.Lines) > 0 {
		sb.WriteString(fmt.Sprintf("\n--- Last %d lines of output ---\n", len(result.Lines)))
		for _, line := range result.Lines {
			prefix := "[stdout]"
			if line.Stream == "stderr" {
				prefix = "[stderr]"
			}
			sb.WriteString(fmt.Sprintf("%s %s", prefix, line.Text))
		}
	} else {
		sb.WriteString("\n(no output yet)")
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}
