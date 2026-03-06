package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/config"
)

type WebSearchTool struct{}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web using SearXNG. Returns titles, URLs, and descriptions of matching results."
}

func (t *WebSearchTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default 10)",
				"default":     10,
			},
		},
		"required": []string{"query"},
	}
}

type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

func (t *WebSearchTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf(`query is required. Example: {"query": "search terms"}`)
	}

	maxResults := 10
	if m, ok := args["max_results"].(float64); ok && m > 0 {
		maxResults = int(m)
	}

	cfg := config.Get()
	searxngURL := cfg.SearXNGURL
	if searxngURL == "" {
		return &agent.ToolResult{
			Message:   "Web search is not configured. Set SEARXNG_URL environment variable.",
			BreakLoop: false,
		}, nil
	}

	// Build SearXNG request
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("engines", "google,duckduckgo,bing")

	reqURL := fmt.Sprintf("%s/search?%s", strings.TrimRight(searxngURL, "/"), params.Encode())

	httpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "GET", reqURL, nil)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Search request error: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Search failed: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading search response: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	var searchResp searxngResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error parsing search response: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	results := searchResp.Results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	if len(results) == 0 {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("No results found for: %q", query),
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %q (%d results)\n\n", query, len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Result %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Title: %s\n", r.Title))
		sb.WriteString(fmt.Sprintf("URL: %s\n", r.URL))
		if r.Content != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", r.Content))
		}
		sb.WriteString("\n")
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}
