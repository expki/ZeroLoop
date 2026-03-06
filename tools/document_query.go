package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/llm"
)

type DocumentQueryTool struct{}

func (t *DocumentQueryTool) Name() string { return "document_query" }

func (t *DocumentQueryTool) Description() string {
	return "Read and analyze documents from local files or URLs. Supports text, HTML, and common document formats. Use 'content' mode to get the full text, or 'query' mode to ask questions about the document."
}

func (t *DocumentQueryTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"uri": map[string]any{
				"type":        "string",
				"description": "File path or URL of the document to read",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"content", "query"},
				"description": "Mode: 'content' returns full text, 'query' answers questions about the document",
				"default":     "content",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "For query mode: the question to answer about the document",
			},
		},
		"required": []string{"uri"},
	}
}

func (t *DocumentQueryTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	uri, _ := args["uri"].(string)
	if uri == "" {
		return nil, fmt.Errorf(`uri is required. Example: {"uri": "path/to/file.txt"}`)
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "content"
	}

	// Fetch document content
	content, err := t.fetchDocument(ctx, uri)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading document: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	if mode == "content" {
		// Return raw content (truncated)
		if len(content) > 15000 {
			content = content[:15000] + "\n... (content truncated)"
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Document content from %s:\n\n%s", uri, content),
			BreakLoop: false,
		}, nil
	}

	// Query mode: use LLM to answer questions about the document
	question, _ := args["question"].(string)
	if question == "" {
		return nil, fmt.Errorf(`question is required for query mode. Example: {"uri": "file.txt", "mode": "query", "question": "your question"}`)
	}

	// Truncate content for LLM context
	if len(content) > 10000 {
		content = content[:10000] + "\n... (truncated)"
	}

	result, err := a.LLM.ChatCompletion(ctx, []llm.ChatMessage{
		{
			Role:    "system",
			Content: "You are a document analysis assistant. Answer questions based solely on the provided document content. Be precise and cite relevant sections.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Document from %s:\n\n%s\n\nQuestion: %s", uri, content, question),
		},
	}, nil, nil)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error querying document: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	answer := result.Content
	return &agent.ToolResult{
		Message:   fmt.Sprintf("Document analysis (%s):\n\n%s", uri, answer),
		BreakLoop: false,
	}, nil
}

func (t *DocumentQueryTool) fetchDocument(ctx context.Context, uri string) (string, error) {
	// Check if it's a URL
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return t.fetchURL(ctx, uri)
	}

	// Local file
	if !filepath.IsAbs(uri) {
		cwd, _ := os.Getwd()
		uri = filepath.Join(cwd, uri)
	}

	data, err := os.ReadFile(uri)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", uri, err)
	}

	return string(data), nil
}

func (t *DocumentQueryTool) fetchURL(ctx context.Context, url string) (string, error) {
	httpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ZeroLoop/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit read to 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}

	content := string(body)

	// Basic HTML stripping if content type suggests HTML
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "html") {
		content = stripHTML(content)
	}

	return content, nil
}

// stripHTML does a basic removal of HTML tags
func stripHTML(html string) string {
	var sb strings.Builder
	inTag := false
	inScript := false
	inStyle := false

	lower := strings.ToLower(html)
	for i := 0; i < len(html); i++ {
		if !inTag && html[i] == '<' {
			// Check for script/style tags
			remaining := lower[i:]
			if strings.HasPrefix(remaining, "<script") {
				inScript = true
			} else if strings.HasPrefix(remaining, "</script") {
				inScript = false
			} else if strings.HasPrefix(remaining, "<style") {
				inStyle = true
			} else if strings.HasPrefix(remaining, "</style") {
				inStyle = false
			}
			inTag = true
			continue
		}
		if inTag && html[i] == '>' {
			inTag = false
			continue
		}
		if !inTag && !inScript && !inStyle {
			sb.WriteByte(html[i])
		}
	}

	// Collapse whitespace
	result := sb.String()
	lines := strings.Split(result, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}
