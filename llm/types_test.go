package llm

import (
	"encoding/json"
	"testing"
)

func TestToolCallIndexDeserialization(t *testing.T) {
	// This tests the fix for the streaming tool call accumulation bug.
	// Tool call deltas from llama.cpp include an "index" field that must
	// be properly deserialized for correct multi-tool-call argument routing.

	// First chunk: tool call with index, id, and name
	chunk1JSON := `{
		"choices": [{
			"index": 0,
			"delta": {
				"tool_calls": [{
					"index": 0,
					"id": "call_0",
					"type": "function",
					"function": {"name": "text_editor", "arguments": ""}
				}]
			}
		}]
	}`

	var chunk1 ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunk1JSON), &chunk1); err != nil {
		t.Fatalf("unmarshal chunk1: %v", err)
	}
	if len(chunk1.Choices) == 0 || len(chunk1.Choices[0].Delta.ToolCalls) == 0 {
		t.Fatal("expected tool call in chunk1")
	}
	tc1 := chunk1.Choices[0].Delta.ToolCalls[0]
	if tc1.Index != 0 {
		t.Errorf("expected index 0, got %d", tc1.Index)
	}
	if tc1.ID != "call_0" {
		t.Errorf("expected id 'call_0', got %q", tc1.ID)
	}
	if tc1.Function.Name != "text_editor" {
		t.Errorf("expected name 'text_editor', got %q", tc1.Function.Name)
	}

	// Second chunk: second tool call with different index
	chunk2JSON := `{
		"choices": [{
			"index": 0,
			"delta": {
				"tool_calls": [{
					"index": 1,
					"id": "call_1",
					"type": "function",
					"function": {"name": "response", "arguments": ""}
				}]
			}
		}]
	}`

	var chunk2 ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunk2JSON), &chunk2); err != nil {
		t.Fatalf("unmarshal chunk2: %v", err)
	}
	tc2 := chunk2.Choices[0].Delta.ToolCalls[0]
	if tc2.Index != 1 {
		t.Errorf("expected index 1, got %d", tc2.Index)
	}
	if tc2.ID != "call_1" {
		t.Errorf("expected id 'call_1', got %q", tc2.ID)
	}

	// Third chunk: argument fragment for first tool (index 0, no id)
	chunk3JSON := `{
		"choices": [{
			"index": 0,
			"delta": {
				"tool_calls": [{
					"index": 0,
					"function": {"arguments": "{\"method\": \"write\"}"}
				}]
			}
		}]
	}`

	var chunk3 ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunk3JSON), &chunk3); err != nil {
		t.Fatalf("unmarshal chunk3: %v", err)
	}
	tc3 := chunk3.Choices[0].Delta.ToolCalls[0]
	if tc3.Index != 0 {
		t.Errorf("expected index 0 for argument fragment, got %d", tc3.Index)
	}
	if tc3.ID != "" {
		t.Errorf("expected empty id for subsequent chunk, got %q", tc3.ID)
	}
	if tc3.Function.Arguments != `{"method": "write"}` {
		t.Errorf("expected arguments, got %q", tc3.Function.Arguments)
	}

	// Simulate the accumulation logic from agent.go
	toolCalls := []ToolCall{}
	allDeltas := []ToolCall{tc1, tc2, tc3}

	for _, tc := range allDeltas {
		found := false
		for j := range toolCalls {
			if toolCalls[j].Index == tc.Index {
				toolCalls[j].Function.Arguments += tc.Function.Arguments
				if tc.Function.Name != "" {
					toolCalls[j].Function.Name = tc.Function.Name
				}
				if tc.ID != "" {
					toolCalls[j].ID = tc.ID
				}
				found = true
				break
			}
		}
		if !found {
			toolCalls = append(toolCalls, tc)
		}
	}

	// Verify: should have 2 tool calls
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}

	// First tool call should have the arguments (not the second one!)
	if toolCalls[0].Function.Name != "text_editor" {
		t.Errorf("expected first tool 'text_editor', got %q", toolCalls[0].Function.Name)
	}
	if toolCalls[0].Function.Arguments != `{"method": "write"}` {
		t.Errorf("expected arguments on text_editor, got %q", toolCalls[0].Function.Arguments)
	}

	// Second tool call should have empty arguments
	if toolCalls[1].Function.Name != "response" {
		t.Errorf("expected second tool 'response', got %q", toolCalls[1].Function.Name)
	}
	if toolCalls[1].Function.Arguments != "" {
		t.Errorf("expected empty arguments on response, got %q", toolCalls[1].Function.Arguments)
	}
}

func TestToolCallIndexAccumulationOldBugReproduction(t *testing.T) {
	// This reproduces the exact bug that was fixed.
	// With ID-based matching (the old code), argument fragment for index 0
	// would incorrectly be appended to the LAST tool call (index 1) because
	// tc.ID="" matches "tc.ID == '' && j == len(toolCalls)-1".

	toolCalls := []ToolCall{
		{Index: 0, ID: "call_0", Function: ToolCallFunction{Name: "text_editor", Arguments: ""}},
		{Index: 1, ID: "call_1", Function: ToolCallFunction{Name: "response", Arguments: ""}},
	}

	// Argument fragment for index 0 (no ID in subsequent chunks)
	fragment := ToolCall{
		Index:    0,
		ID:       "", // No ID in subsequent chunks
		Function: ToolCallFunction{Arguments: `{"content": "hello world"}`},
	}

	// OLD (buggy) matching: tc.ID=="" matches last entry via "j == len-1"
	// Would put arguments on response (index 1) instead of text_editor (index 0)
	oldMatch := -1
	for j := range toolCalls {
		if toolCalls[j].ID == fragment.ID || (fragment.ID == "" && j == len(toolCalls)-1) {
			oldMatch = j
			break
		}
	}
	if oldMatch != 1 {
		t.Fatalf("old bug reproduction failed: expected match on index 1 (the bug), got %d", oldMatch)
	}

	// NEW (fixed) matching: uses Index field
	newMatch := -1
	for j := range toolCalls {
		if toolCalls[j].Index == fragment.Index {
			newMatch = j
			break
		}
	}
	if newMatch != 0 {
		t.Fatalf("new matching failed: expected match on index 0, got %d", newMatch)
	}

	// Apply the correct match
	toolCalls[newMatch].Function.Arguments += fragment.Function.Arguments

	// Verify text_editor got the content
	if toolCalls[0].Function.Arguments != `{"content": "hello world"}` {
		t.Errorf("text_editor should have content, got %q", toolCalls[0].Function.Arguments)
	}
	// Verify response is still empty
	if toolCalls[1].Function.Arguments != "" {
		t.Errorf("response should be empty, got %q", toolCalls[1].Function.Arguments)
	}
}
