package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/search"
	"github.com/google/uuid"
)

type MemoryTool struct{}

func (t *MemoryTool) Name() string { return "memory" }

func (t *MemoryTool) Description() string {
	return "Manage persistent memory. Actions: 'save' stores information, 'load' searches memory by query, 'delete' removes entries by ID, 'forget' removes entries matching a query. Memory persists across conversations."
}

func (t *MemoryTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"save", "load", "delete", "forget"},
				"description": "Action to perform",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "For load/forget: the search query",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "For save: the content to memorize",
			},
			"heading": map[string]any{
				"type":        "string",
				"description": "For save: a short title for the memory entry",
			},
			"area": map[string]any{
				"type":        "string",
				"enum":        []string{"main", "fragments", "solutions"},
				"description": "Memory area (default: main)",
				"default":     "main",
			},
			"ids": map[string]any{
				"type":        "string",
				"description": "For delete: comma-separated document IDs to remove",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "For load: maximum results (default 10)",
				"default":     10,
			},
		},
		"required": []string{"action"},
	}
}

func (t *MemoryTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	action, _ := args["action"].(string)

	switch action {
	case "save":
		return t.executeSave(args)
	case "load":
		return t.executeLoad(args)
	case "delete":
		return t.executeDelete(args)
	case "forget":
		return t.executeForget(args)
	default:
		return nil, fmt.Errorf("invalid action: %s (use 'save', 'load', 'delete', or 'forget')", action)
	}
}

func (t *MemoryTool) executeSave(args map[string]any) (*agent.ToolResult, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return nil, fmt.Errorf(`content is required for save action. Example: {"action": "save", "content": "information to remember", "heading": "title"}`)
	}

	heading, _ := args["heading"].(string)
	if heading == "" {
		heading = "Memory entry"
	}

	area, _ := args["area"].(string)
	if area == "" {
		area = "main"
	}

	doc := search.Document{
		ID:        uuid.New().String(),
		Content:   content,
		Type:      "memory:" + area,
		Heading:   heading,
		CreatedAt: time.Now(),
	}

	if err := search.Index(doc); err != nil {
		if logger.Log != nil {
			logger.Log.Errorw("failed to save memory", "error", err)
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Failed to save memory: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Memory saved (id: %s): %q in area %q", doc.ID, heading, area),
		BreakLoop: false,
	}, nil
}

func (t *MemoryTool) executeLoad(args map[string]any) (*agent.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf(`query is required for load action. Example: {"action": "load", "query": "search terms"}`)
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	results, err := search.Search(query, limit)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Memory search error: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	// Filter to memory entries only
	var memResults []search.SearchResult
	for _, r := range results {
		if strings.HasPrefix(r.Type, "memory:") {
			memResults = append(memResults, r)
		}
	}

	if len(memResults) == 0 {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("No memories found for: %q", query),
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d memories for: %q\n\n", len(memResults), query))
	for i, r := range memResults {
		sb.WriteString(fmt.Sprintf("--- Memory %d (id: %s, score: %.2f) ---\n", i+1, r.ID, r.Score))
		if r.Heading != "" {
			sb.WriteString(fmt.Sprintf("Heading: %s\n", r.Heading))
		}
		sb.WriteString(fmt.Sprintf("Area: %s\n", strings.TrimPrefix(r.Type, "memory:")))
		content := r.Content
		if len(content) > 1000 {
			content = content[:1000] + "..."
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}

func (t *MemoryTool) executeDelete(args map[string]any) (*agent.ToolResult, error) {
	idsStr, _ := args["ids"].(string)
	if idsStr == "" {
		return nil, fmt.Errorf(`ids is required for delete action. Example: {"action": "delete", "ids": "doc-id-1,doc-id-2"}`)
	}

	ids := strings.Split(idsStr, ",")
	var deleted, failed int
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := search.Delete(id); err != nil {
			failed++
		} else {
			deleted++
		}
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Deleted %d memories (%d failed)", deleted, failed),
		BreakLoop: false,
	}, nil
}

func (t *MemoryTool) executeForget(args map[string]any) (*agent.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf(`query is required for forget action. Example: {"action": "forget", "query": "what to forget"}`)
	}

	results, err := search.Search(query, 50)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Memory search error: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	var deleted int
	for _, r := range results {
		if strings.HasPrefix(r.Type, "memory:") && r.Score > 0.5 {
			if err := search.Delete(r.ID); err == nil {
				deleted++
			}
		}
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Forgot %d memories matching: %q", deleted, query),
		BreakLoop: false,
	}, nil
}
