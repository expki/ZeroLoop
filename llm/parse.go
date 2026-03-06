package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// toolCallTagRegex matches <tool_call>...</tool_call> blocks
var toolCallTagRegex = regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)

// ParseToolCalls extracts tool calls from raw model output text.
// Returns the cleaned content (with tool call markup removed) and any parsed tool calls.
// Supports multiple formats: <tool_call> XML tags, bare JSON objects, and JSON arrays.
func ParseToolCalls(text string) (string, []ToolCall) {
	// Try XML tag format first: <tool_call>...</tool_call>
	if content, calls := parseToolCallTags(text); len(calls) > 0 {
		return content, calls
	}

	// Try bare JSON at end of text (common with Llama 3.1+)
	if content, calls := parseBareToolCallJSON(text); len(calls) > 0 {
		return content, calls
	}

	// No tool calls found
	return text, nil
}

// parseToolCallTags extracts tool calls from <tool_call>...</tool_call> tags
func parseToolCallTags(text string) (string, []ToolCall) {
	matches := toolCallTagRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var toolCalls []ToolCall
	content := text
	for _, match := range matches {
		tc, ok := parseToolCallJSON(match[1], len(toolCalls))
		if ok {
			toolCalls = append(toolCalls, tc)
		}
		content = strings.Replace(content, match[0], "", 1)
	}

	if len(toolCalls) == 0 {
		return text, nil
	}
	return strings.TrimSpace(content), toolCalls
}

// parseBareToolCallJSON tries to find a JSON tool call object or array at the end of text
func parseBareToolCallJSON(text string) (string, []ToolCall) {
	trimmed := strings.TrimSpace(text)

	// Try JSON array: [{"name": "...", ...}, ...]
	if idx := findLastJSONArray(trimmed); idx >= 0 {
		jsonStr := trimmed[idx:]
		if calls := parseToolCallArray(jsonStr, 0); len(calls) > 0 {
			content := strings.TrimSpace(trimmed[:idx])
			return content, calls
		}
	}

	// Try single JSON object: {"name": "...", ...}
	if idx := findLastToolCallObject(trimmed); idx >= 0 {
		jsonStr := trimmed[idx:]
		tc, ok := parseToolCallJSON(jsonStr, 0)
		if ok {
			content := strings.TrimSpace(trimmed[:idx])
			return content, []ToolCall{tc}
		}
	}

	return text, nil
}

// parseToolCallJSON parses a single JSON tool call object
func parseToolCallJSON(jsonStr string, index int) (ToolCall, bool) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return ToolCall{}, false
	}

	name, _ := raw["name"].(string)
	if name == "" {
		return ToolCall{}, false
	}

	// Support both "arguments" and "parameters" keys
	args := raw["arguments"]
	if args == nil {
		args = raw["parameters"]
	}

	argsJSON := "{}"
	if args != nil {
		if b, err := json.Marshal(args); err == nil {
			argsJSON = string(b)
		}
	}

	return ToolCall{
		Index: index,
		ID:    fmt.Sprintf("call_%d", index),
		Type:  "function",
		Function: ToolCallFunction{
			Name:      name,
			Arguments: argsJSON,
		},
	}, true
}

// parseToolCallArray parses a JSON array of tool call objects
func parseToolCallArray(jsonStr string, startIndex int) []ToolCall {
	var raw []map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}

	var calls []ToolCall
	for i, item := range raw {
		name, _ := item["name"].(string)
		if name == "" {
			continue
		}
		args := item["arguments"]
		if args == nil {
			args = item["parameters"]
		}
		argsJSON := "{}"
		if args != nil {
			if b, err := json.Marshal(args); err == nil {
				argsJSON = string(b)
			}
		}
		calls = append(calls, ToolCall{
			Index: startIndex + i,
			ID:    fmt.Sprintf("call_%d", startIndex+i),
			Type:  "function",
			Function: ToolCallFunction{
				Name:      name,
				Arguments: argsJSON,
			},
		})
	}
	return calls
}

// findLastJSONArray finds the start index of the last JSON array in text
// that could be a tool call array
func findLastJSONArray(text string) int {
	depth := 0
	end := -1
	for i := len(text) - 1; i >= 0; i-- {
		ch := text[i]
		if ch == ']' {
			if depth == 0 {
				end = i
			}
			depth++
		} else if ch == '[' {
			depth--
			if depth == 0 && end >= 0 {
				return i
			}
		}
	}
	return -1
}

// findLastToolCallObject finds the start index of the last JSON object in text
// that contains a "name" field (indicating it's a tool call)
func findLastToolCallObject(text string) int {
	depth := 0
	end := -1
	for i := len(text) - 1; i >= 0; i-- {
		ch := text[i]
		if ch == '}' {
			if depth == 0 {
				end = i
			}
			depth++
		} else if ch == '{' {
			depth--
			if depth == 0 && end >= 0 {
				candidate := text[i : end+1]
				var raw map[string]any
				if err := json.Unmarshal([]byte(candidate), &raw); err == nil {
					if _, hasName := raw["name"]; hasName {
						return i
					}
				}
				end = -1
			}
		}
	}
	return -1
}
