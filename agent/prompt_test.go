package agent

import (
	"strings"
	"testing"
)

func TestSystemPrompt(t *testing.T) {
	tools := []Tool{
		&mockTool{name: "response"},
		&mockTool{name: "code_execution"},
	}

	prompt := SystemPrompt(0, tools)

	// Should contain agent identity
	if !strings.Contains(prompt, "Agent 0") {
		t.Error("expected 'Agent 0' in prompt")
	}

	// Should contain tool names
	if !strings.Contains(prompt, "response") {
		t.Error("expected 'response' tool in prompt")
	}
	if !strings.Contains(prompt, "code_execution") {
		t.Error("expected 'code_execution' tool in prompt")
	}

	// Should contain rules section
	if !strings.Contains(prompt, "Rules") {
		t.Error("expected 'Rules' section in prompt")
	}

	// Should contain communication section
	if !strings.Contains(prompt, "Communication") {
		t.Error("expected 'Communication' section in prompt")
	}
}

func TestSystemPromptSubordinate(t *testing.T) {
	prompt := SystemPrompt(1, nil)
	if !strings.Contains(prompt, "Agent 1") {
		t.Error("expected 'Agent 1' for subordinate")
	}
	// Subordinate should not contain Agent 0
	if strings.Contains(prompt, "Agent 0") {
		t.Error("subordinate prompt should not contain Agent 0")
	}
}

func TestSystemPromptNoTools(t *testing.T) {
	prompt := SystemPrompt(0, nil)
	// Should still have the available tools header
	if !strings.Contains(prompt, "Available Tools") {
		t.Error("expected 'Available Tools' section even with no tools")
	}
}
