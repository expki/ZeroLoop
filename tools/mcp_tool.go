package tools

import (
	"context"
	"fmt"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/mcp"
)

// MCPToolAdapter wraps an MCP tool as an agent Tool
type MCPToolAdapter struct {
	mcpTool    mcp.MCPTool
	mcpManager *mcp.MCPManager
}

// NewMCPToolAdapter creates a new adapter for an MCP tool
func NewMCPToolAdapter(tool mcp.MCPTool, manager *mcp.MCPManager) *MCPToolAdapter {
	return &MCPToolAdapter{
		mcpTool:    tool,
		mcpManager: manager,
	}
}

func (t *MCPToolAdapter) Name() string { return "mcp_" + t.mcpTool.Name }

func (t *MCPToolAdapter) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.mcpTool.ServerName, t.mcpTool.Description)
}

func (t *MCPToolAdapter) Parameters() any {
	if t.mcpTool.InputSchema != nil {
		return t.mcpTool.InputSchema
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *MCPToolAdapter) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	result, err := t.mcpManager.CallTool(t.mcpTool.Name, args)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("MCP tool error: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	if len(result) > 15000 {
		result = result[:15000] + "\n... (output truncated)"
	}

	return &agent.ToolResult{
		Message:   result,
		BreakLoop: false,
	}, nil
}
