package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/llm"
)

type BehaviourAdjustmentTool struct{}

func (t *BehaviourAdjustmentTool) Name() string { return "behaviour_adjustment" }

func (t *BehaviourAdjustmentTool) Description() string {
	return "Update persistent behavioral rules for the agent. New rules are merged with existing ones using the LLM. Rules persist across conversations and guide agent behavior."
}

func (t *BehaviourAdjustmentTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"update", "view", "reset"},
				"description": "Action: 'update' to add/modify rules, 'view' to see current rules, 'reset' to clear all rules",
			},
			"rules": map[string]any{
				"type":        "string",
				"description": "For update: new behavioral rules to merge with existing ones",
			},
		},
		"required": []string{"action"},
	}
}

func getBehaviourFilePath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "behaviour.md")
}

func (t *BehaviourAdjustmentTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	action, _ := args["action"].(string)

	switch action {
	case "view":
		return t.executeView()
	case "reset":
		return t.executeReset()
	case "update":
		return t.executeUpdate(ctx, a, args)
	default:
		return nil, fmt.Errorf("invalid action: %s (use 'update', 'view', or 'reset')", action)
	}
}

func (t *BehaviourAdjustmentTool) executeView() (*agent.ToolResult, error) {
	path := getBehaviourFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &agent.ToolResult{
				Message:   "No behavioral rules are currently set.",
				BreakLoop: false,
			}, nil
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading behaviour file: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Current behavioral rules:\n\n%s", string(data)),
		BreakLoop: false,
	}, nil
}

func (t *BehaviourAdjustmentTool) executeReset() (*agent.ToolResult, error) {
	path := getBehaviourFilePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error resetting behaviour: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   "Behavioral rules have been reset.",
		BreakLoop: false,
	}, nil
}

func (t *BehaviourAdjustmentTool) executeUpdate(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	newRules, _ := args["rules"].(string)
	if newRules == "" {
		return nil, fmt.Errorf(`rules is required for update action. Example: {"action": "update", "rules": "new behavioral rules"}`)
	}

	// Read existing rules
	path := getBehaviourFilePath()
	existingRules := ""
	if data, err := os.ReadFile(path); err == nil {
		existingRules = string(data)
	}

	// Use LLM to merge rules
	var mergedRules string
	if existingRules == "" {
		mergedRules = newRules
	} else {
		mergePrompt := fmt.Sprintf(
			"Merge the following behavioral rules into a single coherent set. Remove duplicates, resolve conflicts (newer rules take precedence), and organize logically.\n\nExisting rules:\n%s\n\nNew rules:\n%s\n\nOutput ONLY the merged rules as a markdown list, nothing else.",
			existingRules, newRules,
		)

		result, err := a.LLM.ChatCompletion(ctx, []llm.ChatMessage{
			{Role: "system", Content: "You merge behavioral rules. Output ONLY the merged rules."},
			{Role: "user", Content: mergePrompt},
		}, nil, nil)
		if err != nil {
			// Fallback: just append
			mergedRules = existingRules + "\n\n" + newRules
		} else if result.Content != "" {
			mergedRules = strings.TrimSpace(result.Content)
		} else {
			mergedRules = existingRules + "\n\n" + newRules
		}
	}

	if err := os.WriteFile(path, []byte(mergedRules), 0644); err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error saving behaviour: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Behavioral rules updated:\n\n%s", mergedRules),
		BreakLoop: false,
	}, nil
}
