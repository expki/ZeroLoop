package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
)

type WaitTool struct{}

func (t *WaitTool) Name() string { return "wait" }

func (t *WaitTool) Description() string {
	return "Pause execution for a specified duration or until a specific time. Use 'seconds' for relative wait or 'until' for absolute timestamp (ISO 8601 format)."
}

func (t *WaitTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"seconds": map[string]any{
				"type":        "number",
				"description": "Number of seconds to wait",
			},
			"until": map[string]any{
				"type":        "string",
				"description": "ISO 8601 timestamp to wait until (e.g. '2024-01-15T14:30:00Z')",
			},
		},
	}
}

func (t *WaitTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	var waitDuration time.Duration

	if seconds, ok := args["seconds"].(float64); ok && seconds > 0 {
		waitDuration = time.Duration(seconds * float64(time.Second))
	} else if until, ok := args["until"].(string); ok && until != "" {
		target, err := time.Parse(time.RFC3339, until)
		if err != nil {
			// Try alternative formats
			target, err = time.Parse("2006-01-02T15:04:05", until)
			if err != nil {
				return nil, fmt.Errorf("invalid timestamp format: %s (use ISO 8601)", until)
			}
		}
		waitDuration = time.Until(target)
		if waitDuration <= 0 {
			return &agent.ToolResult{
				Message:   fmt.Sprintf("Target time %s has already passed", until),
				BreakLoop: false,
			}, nil
		}
	} else {
		return nil, fmt.Errorf("provide either 'seconds' or 'until' parameter")
	}

	// Cap wait at 10 minutes
	if waitDuration > 10*time.Minute {
		waitDuration = 10 * time.Minute
	}

	a.Log(agent.LogEntry{
		Type:    "info",
		Heading: "Waiting",
		Content: fmt.Sprintf("Pausing for %s", waitDuration.Round(time.Second)),
		AgentNo: a.Number,
	})

	select {
	case <-time.After(waitDuration):
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Waited %s", waitDuration.Round(time.Second)),
			BreakLoop: false,
		}, nil
	case <-ctx.Done():
		return &agent.ToolResult{
			Message:   "Wait cancelled",
			BreakLoop: false,
		}, nil
	}
}
