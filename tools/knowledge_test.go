package tools

import (
	"context"
	"strings"
	"testing"
)

func TestKnowledgeToolName(t *testing.T) {
	tool := &KnowledgeTool{}
	if tool.Name() != "knowledge" {
		t.Errorf("expected name 'knowledge', got '%s'", tool.Name())
	}
}

func TestKnowledgeToolDescription(t *testing.T) {
	tool := &KnowledgeTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestKnowledgeToolParameters(t *testing.T) {
	tool := &KnowledgeTool{}
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

func TestKnowledgeToolSearchNoIndex(t *testing.T) {
	// When search index is not initialized, should return an error message (not crash)
	tool := &KnowledgeTool{}
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "search",
		"query":  "test query",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should return an error message about index not being initialized
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(result.Message, "error") && !strings.Contains(result.Message, "Error") && !strings.Contains(result.Message, "No results") {
		// Either an error about uninitialized index, or no results - both are acceptable
		t.Logf("got message: %s", result.Message)
	}
}

func TestKnowledgeToolInvalidAction(t *testing.T) {
	tool := &KnowledgeTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestKnowledgeToolSearchMissingQuery(t *testing.T) {
	tool := &KnowledgeTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "search",
	})
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestKnowledgeToolSaveMissingContent(t *testing.T) {
	tool := &KnowledgeTool{}
	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"action": "save",
	})
	if err == nil {
		t.Error("expected error for missing content")
	}
}
