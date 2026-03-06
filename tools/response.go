package tools

import (
	"context"
	"fmt"

	"github.com/expki/ZeroLoop.git/agent"
)

type ResponseTool struct{}

func (t *ResponseTool) Name() string { return "response" }

func (t *ResponseTool) Description() string {
	return "Deliver your final response to the user. Use this tool when you have the answer ready. The message parameter contains your response in markdown format."
}

func (t *ResponseTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Your final response to the user in markdown format",
			},
		},
		"required": []string{"message"},
	}
}

func (t *ResponseTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return nil, fmt.Errorf(`response message is required. Example: {"message": "your response text here"}`)
	}
	return &agent.ToolResult{
		Message:   message,
		BreakLoop: true,
	}, nil
}
