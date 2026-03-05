package tools

import (
	"context"
	"testing"
)

func TestResponseToolName(t *testing.T) {
	tool := &ResponseTool{}
	if tool.Name() != "response" {
		t.Errorf("expected name 'response', got '%s'", tool.Name())
	}
}

func TestResponseToolDescription(t *testing.T) {
	tool := &ResponseTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestResponseToolParameters(t *testing.T) {
	tool := &ResponseTool{}
	params := tool.Parameters()
	if params == nil {
		t.Error("expected non-nil parameters")
	}
	schema, ok := params.(map[string]any)
	if !ok {
		t.Fatal("expected map[string]any parameters")
	}
	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}
}

func TestResponseToolExecute(t *testing.T) {
	tool := &ResponseTool{}

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"message": "Hello world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.BreakLoop {
		t.Error("expected BreakLoop=true")
	}
	if result.Message != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", result.Message)
	}
}

func TestResponseToolEmptyMessage(t *testing.T) {
	tool := &ResponseTool{}

	_, err := tool.Execute(context.Background(), nil, map[string]any{})
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestResponseToolMissingMessage(t *testing.T) {
	tool := &ResponseTool{}

	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"other_key": "value",
	})
	if err == nil {
		t.Error("expected error for missing message key")
	}
}
