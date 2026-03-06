package tools

import (
	"context"
	"fmt"

	"github.com/expki/ZeroLoop.git/agent"
)

type CallSubordinateTool struct{}

func (t *CallSubordinateTool) Name() string { return "call_subordinate" }

func (t *CallSubordinateTool) Description() string {
	return "Delegate a task to a subordinate agent. Use when a sub-task needs focused attention. The subordinate gets its own context and tools. Optionally specify a profile for specialization (default, developer, researcher, hacker)."
}

func (t *CallSubordinateTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The task description for the subordinate agent",
			},
			"profile": map[string]any{
				"type":        "string",
				"description": "Agent profile to use (default, developer, researcher, hacker). Determines the subordinate's specialization.",
				"default":     "default",
			},
		},
		"required": []string{"message"},
	}
}

func (t *CallSubordinateTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return nil, fmt.Errorf(`message is required. Example: {"message": "task description for the subordinate"}`)
	}

	profile, _ := args["profile"].(string)

	// Create subordinate agent
	sub := a.NewSubordinate()

	// Set profile if specified
	if profile != "" {
		sub.Profile = profile
	}

	// Run the subordinate agent
	result, err := sub.Run(ctx, message)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Subordinate agent error: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   result,
		BreakLoop: false,
	}, nil
}
