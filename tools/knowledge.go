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

type KnowledgeTool struct{}

func (t *KnowledgeTool) Name() string { return "knowledge" }

func (t *KnowledgeTool) Description() string {
	return "Search or save information in the knowledge base. Use action 'search' to find stored knowledge, or 'save' to memorize important information for future use."
}

func (t *KnowledgeTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"search", "save"},
				"description": "Action to perform: 'search' to query knowledge base, 'save' to store new knowledge",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "For search: the search query. For save: not used.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "For save: the knowledge content to store. For search: not used.",
			},
			"heading": map[string]any{
				"type":        "string",
				"description": "For save: a short heading/title for the knowledge entry.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *KnowledgeTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	action, _ := args["action"].(string)

	switch action {
	case "search":
		return t.executeSearch(args)
	case "save":
		return t.executeSave(args)
	default:
		return nil, fmt.Errorf("invalid action: %s (use 'search' or 'save')", action)
	}
}

func (t *KnowledgeTool) executeSearch(args map[string]any) (*agent.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf(`query is required for search action. Example: {"action": "search", "query": "search terms"}`)
	}

	results, err := search.Search(query, 10)
	if err != nil {
		if logger.Log != nil {
			logger.Log.Warnw("knowledge search failed", "error", err, "query", query)
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Search error: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	if len(results) == 0 {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("No results found for: %q", query),
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results for: %q\n\n", len(results), query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Result %d (score: %.2f) ---\n", i+1, r.Score))
		if r.Heading != "" {
			sb.WriteString(fmt.Sprintf("Heading: %s\n", r.Heading))
		}
		if r.Type != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", r.Type))
		}
		content := r.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}

func (t *KnowledgeTool) executeSave(args map[string]any) (*agent.ToolResult, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return nil, fmt.Errorf(`content is required for save action. Example: {"action": "save", "content": "knowledge to store", "heading": "title"}`)
	}

	heading, _ := args["heading"].(string)
	if heading == "" {
		heading = "Knowledge entry"
	}

	doc := search.Document{
		ID:        uuid.New().String(),
		Content:   content,
		Type:      "knowledge",
		Heading:   heading,
		CreatedAt: time.Now(),
	}

	if err := search.Index(doc); err != nil {
		if logger.Log != nil {
			logger.Log.Errorw("failed to save knowledge", "error", err)
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Failed to save knowledge: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Knowledge saved: %q", heading),
		BreakLoop: false,
	}, nil
}
