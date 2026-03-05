package tools

import (
	"context"
	"strings"
	"testing"
)

func TestCodeExecutionToolName(t *testing.T) {
	tool := &CodeExecutionTool{}
	if tool.Name() != "code_execution" {
		t.Errorf("expected name 'code_execution', got '%s'", tool.Name())
	}
}

func TestCodeExecutionToolDescription(t *testing.T) {
	tool := &CodeExecutionTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestCodeExecutionToolParameters(t *testing.T) {
	tool := &CodeExecutionTool{}
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

func TestCodeExecutionShell(t *testing.T) {
	tool := &CodeExecutionTool{}

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "echo hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "hello") {
		t.Errorf("expected 'hello' in output, got '%s'", result.Message)
	}
	if result.BreakLoop {
		t.Error("code execution should not break loop")
	}
}

func TestCodeExecutionEmptyCode(t *testing.T) {
	tool := &CodeExecutionTool{}

	_, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
	})
	if err == nil {
		t.Error("expected error for empty code")
	}
}

func TestCodeExecutionPython(t *testing.T) {
	tool := &CodeExecutionTool{}

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "python",
		"code":    "print('python works')",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "python works") {
		t.Errorf("expected 'python works' in output, got '%s'", result.Message)
	}
}

func TestCodeExecutionFailingCommand(t *testing.T) {
	tool := &CodeExecutionTool{}

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "exit 1",
	})
	if err != nil {
		t.Fatal(err) // tool should not return error, just capture exit code
	}
	if !strings.Contains(result.Message, "Exit") {
		t.Errorf("expected exit info in output, got '%s'", result.Message)
	}
}

func TestCodeExecutionDefaultRuntime(t *testing.T) {
	tool := &CodeExecutionTool{}

	// Empty runtime should default to shell (bash)
	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "",
		"code":    "echo default_shell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "default_shell") {
		t.Errorf("expected 'default_shell' in output, got '%s'", result.Message)
	}
}

func TestCodeExecutionNoOutput(t *testing.T) {
	tool := &CodeExecutionTool{}

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "(no output)" {
		t.Errorf("expected '(no output)', got '%s'", result.Message)
	}
}

func TestCodeExecutionStderr(t *testing.T) {
	tool := &CodeExecutionTool{}

	result, err := tool.Execute(context.Background(), nil, map[string]any{
		"runtime": "shell",
		"code":    "echo error_msg >&2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "STDERR") {
		t.Errorf("expected 'STDERR' label in output, got '%s'", result.Message)
	}
	if !strings.Contains(result.Message, "error_msg") {
		t.Errorf("expected 'error_msg' in output, got '%s'", result.Message)
	}
}
