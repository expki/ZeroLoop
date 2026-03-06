package llm

import (
	"testing"
)

func TestParseToolCallsXMLTags(t *testing.T) {
	text := `I'll search for that information.
<tool_call>
{"name": "web_search", "arguments": {"query": "golang best practices"}}
</tool_call>`

	content, calls := ParseToolCalls(text)
	if content != "I'll search for that information." {
		t.Errorf("unexpected content: %q", content)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "web_search" {
		t.Errorf("expected name 'web_search', got %q", calls[0].Function.Name)
	}
	if calls[0].ID != "call_0" {
		t.Errorf("expected id 'call_0', got %q", calls[0].ID)
	}
	if calls[0].Type != "function" {
		t.Errorf("expected type 'function', got %q", calls[0].Type)
	}
}

func TestParseToolCallsMultipleXMLTags(t *testing.T) {
	text := `Let me do two things.
<tool_call>
{"name": "web_search", "arguments": {"query": "test"}}
</tool_call>
<tool_call>
{"name": "response", "arguments": {"message": "Done"}}
</tool_call>`

	content, calls := ParseToolCalls(text)
	if content != "Let me do two things." {
		t.Errorf("unexpected content: %q", content)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "web_search" {
		t.Errorf("first call: expected 'web_search', got %q", calls[0].Function.Name)
	}
	if calls[1].Function.Name != "response" {
		t.Errorf("second call: expected 'response', got %q", calls[1].Function.Name)
	}
	if calls[0].Index != 0 || calls[1].Index != 1 {
		t.Errorf("expected indices 0 and 1, got %d and %d", calls[0].Index, calls[1].Index)
	}
}

func TestParseToolCallsBareJSON(t *testing.T) {
	text := `I'll search for that.
{"name": "web_search", "arguments": {"query": "test"}}`

	content, calls := ParseToolCalls(text)
	if content != "I'll search for that." {
		t.Errorf("unexpected content: %q", content)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "web_search" {
		t.Errorf("expected 'web_search', got %q", calls[0].Function.Name)
	}
}

func TestParseToolCallsParametersKey(t *testing.T) {
	// Llama 3.1+ uses "parameters" instead of "arguments"
	text := `{"name": "web_search", "parameters": {"query": "test"}}`

	content, calls := ParseToolCalls(text)
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "web_search" {
		t.Errorf("expected 'web_search', got %q", calls[0].Function.Name)
	}
}

func TestParseToolCallsNoToolCalls(t *testing.T) {
	text := "Just a regular response with no tool calls."

	content, calls := ParseToolCalls(text)
	if content != text {
		t.Errorf("expected original text, got %q", content)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestParseToolCallsJSONArray(t *testing.T) {
	text := `Here are the results:
[{"name": "web_search", "arguments": {"query": "test"}}, {"name": "response", "arguments": {"message": "done"}}]`

	content, calls := ParseToolCalls(text)
	if content != "Here are the results:" {
		t.Errorf("unexpected content: %q", content)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "web_search" {
		t.Errorf("first call: expected 'web_search', got %q", calls[0].Function.Name)
	}
	if calls[1].Function.Name != "response" {
		t.Errorf("second call: expected 'response', got %q", calls[1].Function.Name)
	}
}

func TestParseToolCallsEmptyInput(t *testing.T) {
	content, calls := ParseToolCalls("")
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestParseToolCallsOnlyToolCall(t *testing.T) {
	text := `<tool_call>
{"name": "response", "arguments": {"message": "Hello world"}}
</tool_call>`

	content, calls := ParseToolCalls(text)
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "response" {
		t.Errorf("expected 'response', got %q", calls[0].Function.Name)
	}
}

func TestParseToolCallsIgnoresNonToolJSON(t *testing.T) {
	// JSON objects without "name" field should not be parsed as tool calls
	text := `Here is some data: {"key": "value", "count": 42}`

	content, calls := ParseToolCalls(text)
	if content != text {
		t.Errorf("expected original text, got %q", content)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}
