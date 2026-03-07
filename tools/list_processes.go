package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/expki/ZeroLoop.git/agent"
)

// ProcessListItem is a summary of a process for the list tool
type ProcessListItem struct {
	ID       string
	Command  string
	Status   string
	ExitCode *int
}

// ProcessListerFunc returns all processes for a project.
// Injected from the Hub to avoid circular imports.
type ProcessListerFunc func(projectID string) []ProcessListItem

// ListProcessesTool lists all running and recently exited processes for the current project
type ListProcessesTool struct {
	Lister ProcessListerFunc
}

func (t *ListProcessesTool) Name() string { return "list_processes" }

func (t *ListProcessesTool) Description() string {
	return "List all running and recently exited background processes for the current project. Use this to discover processes started by previous agent sessions or to get an overview of active processes."
}

func (t *ListProcessesTool) Parameters() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *ListProcessesTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	projectID := ""
	if a != nil {
		projectID = a.ProjectID
	}
	if projectID == "" {
		return &agent.ToolResult{
			Message: "No project context available. This tool requires a project-scoped agent.",
		}, nil
	}

	if t.Lister == nil {
		return nil, fmt.Errorf("process lister not configured")
	}

	processes := t.Lister(projectID)
	if len(processes) == 0 {
		return &agent.ToolResult{
			Message:   "No processes found for this project.",
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d process(es):\n\n", len(processes)))
	for _, p := range processes {
		status := p.Status
		if p.ExitCode != nil {
			status = fmt.Sprintf("%s (exit code: %d)", status, *p.ExitCode)
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s — %s\n", p.ID[:8], p.Command, status))
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}
