package agent

import (
	"context"
	"testing"
)

type mockTool struct {
	name string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool" }
func (t *mockTool) Parameters() any     { return map[string]any{"type": "object"} }
func (t *mockTool) Execute(ctx context.Context, a *Agent, args map[string]any) (*ToolResult, error) {
	return &ToolResult{Message: "mock result"}, nil
}

func TestToolRegistry(t *testing.T) {
	r := NewToolRegistry()

	// Empty registry
	if _, ok := r.Get("foo"); ok {
		t.Error("expected tool not found in empty registry")
	}
	if len(r.All()) != 0 {
		t.Error("expected empty All()")
	}

	// Register and retrieve
	tool := &mockTool{name: "test"}
	r.Register(tool)

	got, ok := r.Get("test")
	if !ok {
		t.Fatal("expected tool to be found")
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", got.Name())
	}

	// All returns registered tools
	all := r.All()
	if len(all) != 1 {
		t.Errorf("expected 1 tool, got %d", len(all))
	}
}

func TestToolRegistryToLLMTools(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&mockTool{name: "foo"})
	r.Register(&mockTool{name: "bar"})

	llmTools := r.ToLLMTools()
	if len(llmTools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(llmTools))
	}

	names := map[string]bool{}
	for _, lt := range llmTools {
		if lt.Type != "function" {
			t.Errorf("expected type 'function', got '%s'", lt.Type)
		}
		names[lt.Function.Name] = true
	}
	if !names["foo"] || !names["bar"] {
		t.Error("expected both foo and bar in LLM tools")
	}
}

func TestToolRegistryOverwrite(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&mockTool{name: "test"})
	r.Register(&mockTool{name: "test"}) // same name overwrites

	all := r.All()
	if len(all) != 1 {
		t.Errorf("expected 1 tool after overwrite, got %d", len(all))
	}
}

func TestToolExecute(t *testing.T) {
	tool := &mockTool{name: "test"}
	result, err := tool.Execute(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "mock result" {
		t.Errorf("expected 'mock result', got '%s'", result.Message)
	}
}
