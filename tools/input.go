package tools

import (
	"context"

	"github.com/expki/ZeroLoop.git/agent"
)

type InputTool struct{}

func (t *InputTool) Name() string { return "input" }

func (t *InputTool) Description() string {
	return "Send keyboard input to an active terminal session. Use when a running process needs input (e.g., prompts, confirmations, interactive commands). The input is sent to the specified code_execution session."
}

func (t *InputTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "The text input to send (newline is appended automatically)",
			},
			"session": map[string]any{
				"type":        "integer",
				"description": "Session number to send input to (default 0)",
				"default":     0,
			},
		},
		"required": []string{"text"},
	}
}

func (t *InputTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	text, _ := args["text"].(string)

	sessionNum := 0
	if s, ok := args["session"].(float64); ok {
		sessionNum = int(s)
	}

	// Delegate to code_execution with the input as a shell command
	codeExec := &CodeExecutionTool{}
	return codeExec.Execute(ctx, a, map[string]any{
		"runtime": "shell",
		"code":    text,
		"session": float64(sessionNum),
	})
}
