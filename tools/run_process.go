package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
)

// ProcessStarterFunc is a function that starts a background process and returns its ID
// and a done channel that closes when the process exits.
// Injected from the Hub to avoid circular imports between tools/ and api/.
type ProcessStarterFunc func(projectID, command, workDir string) (processID string, done <-chan struct{}, err error)

// RunProcessTool starts a long-running background process
type RunProcessTool struct {
	Starter ProcessStarterFunc
	Checker ProcessCheckerFunc
}

func (t *RunProcessTool) Name() string { return "run_process" }

func (t *RunProcessTool) Description() string {
	return "Start a long-running background process (e.g. dev servers, watchers, build tasks). The process runs asynchronously in the background — do NOT append & to the command. Output is streamed to the user in real-time via the Processes panel. Returns a process ID that can be used with check_process and list_processes. Use code_execution for short-lived commands instead."
}

func (t *RunProcessTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute (runs via bash -c)",
			},
			"working_directory": map[string]any{
				"type":        "string",
				"description": "Working directory for the process. Defaults to the project directory.",
			},
		},
		"required": []string{"command"},
	}
}

func (t *RunProcessTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return nil, fmt.Errorf(`command is required. Example: {"command": "npm run dev"}`)
	}
	// Strip trailing & — the process already runs in the background
	command = strings.TrimSpace(command)
	command = strings.TrimRight(command, "&")
	command = strings.TrimSpace(command)

	workDir, _ := args["working_directory"].(string)
	if a != nil {
		if workDir == "" {
			workDir = a.ProjectDir
		} else {
			absWorkDir, err := filepath.Abs(workDir)
			if err != nil || !strings.HasPrefix(absWorkDir+"/", a.ProjectDir+"/") {
				return &agent.ToolResult{
					Message: "working_directory must be within the project directory",
				}, nil
			}
			workDir = absWorkDir
		}
	}

	projectID := ""
	if a != nil {
		projectID = a.ProjectID
	}

	if t.Starter == nil {
		return nil, fmt.Errorf("process starter not configured")
	}

	processID, done, err := t.Starter(projectID, command, workDir)
	if err != nil {
		return &agent.ToolResult{
			Message: fmt.Sprintf("Failed to start process: %s", err.Error()),
		}, nil
	}

	// Wait up to 3 seconds to catch immediate failures (bad command, port in use, etc.)
	select {
	case <-done:
		// Process exited within 3 seconds — likely an error
		lines := ""
		if t.Checker != nil {
			if result, err := t.Checker(processID, 20); err == nil {
				if result.ExitCode != nil {
					lines = fmt.Sprintf("\nExit code: %d", *result.ExitCode)
				}
				if len(result.Lines) > 0 {
					lines += "\nOutput:\n"
					for _, l := range result.Lines {
						lines += l.Text
					}
				}
			}
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Process exited immediately after starting.%s\nCommand: %s\nProcess ID: %s", lines, command, processID),
			BreakLoop: false,
		}, nil
	case <-time.After(3 * time.Second):
		// Still running — success
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Process started successfully and is running.\nCommand: %s\nProcess ID: %s\nUse check_process to monitor its status and output.", command, processID),
			BreakLoop: false,
		}, nil
	}
}
