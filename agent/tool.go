package agent

import (
	"context"

	"github.com/expki/ZeroLoop.git/llm"
)

// ToolResult is returned by tool execution
type ToolResult struct {
	Message   string // Output text from the tool
	BreakLoop bool   // If true, end the agent message loop
}

// Tool is the interface all agent tools implement
type Tool interface {
	Name() string
	Description() string
	Parameters() any // JSON Schema for parameters
	Execute(ctx context.Context, agent *Agent, args map[string]any) (*ToolResult, error)
}

// ToolRegistry holds all registered tools
type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// ToLLMTools converts registered tools to llm.Tool format for the API
func (r *ToolRegistry) ToLLMTools() []llm.Tool {
	tools := make([]llm.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return tools
}
