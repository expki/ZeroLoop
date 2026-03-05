package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
)

type BrowserTool struct{}

func (t *BrowserTool) Name() string { return "browser" }

func (t *BrowserTool) Description() string {
	return "Automate a headless web browser. Actions: 'navigate' (go to URL), 'click' (click element by selector), 'type' (type text into element), 'screenshot' (capture page), 'extract' (get page text content), 'evaluate' (run JavaScript)."
}

func (t *BrowserTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"navigate", "click", "type", "screenshot", "extract", "evaluate"},
				"description": "Browser action to perform",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "For navigate: the URL to visit",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector for click/type actions",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "For type: the text to enter. For evaluate: JavaScript code.",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "For screenshot: file path to save the screenshot",
			},
		},
		"required": []string{"action"},
	}
}

func (t *BrowserTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	action, _ := args["action"].(string)

	// Build a Python script using playwright to execute the browser action
	// This approach works without requiring Go browser dependencies
	script := t.buildPlaywrightScript(action, args)

	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "python3", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR: " + stderr.String()
	}
	if err != nil {
		if output != "" {
			output += "\n"
		}
		output += fmt.Sprintf("Error: %s", err.Error())
		// Check if playwright is not installed
		if strings.Contains(stderr.String(), "playwright") || strings.Contains(stderr.String(), "No module") {
			output += "\n\nPlaywright may not be installed. Install with: pip install playwright && playwright install chromium"
		}
	}

	if output == "" {
		output = "(no output)"
	}

	if len(output) > 15000 {
		output = output[:15000] + "\n... (output truncated)"
	}

	return &agent.ToolResult{
		Message:   output,
		BreakLoop: false,
	}, nil
}

func (t *BrowserTool) buildPlaywrightScript(action string, args map[string]any) string {
	url, _ := args["url"].(string)
	selector, _ := args["selector"].(string)
	text, _ := args["text"].(string)
	outputPath, _ := args["output_path"].(string)

	if outputPath == "" {
		outputPath = "/tmp/screenshot.png"
	}

	// Escape strings for Python
	escURL := strings.ReplaceAll(url, "'", "\\'")
	escSelector := strings.ReplaceAll(selector, "'", "\\'")
	escText := strings.ReplaceAll(text, "'", "\\'")
	escOutput := strings.ReplaceAll(outputPath, "'", "\\'")

	var script strings.Builder
	script.WriteString("from playwright.sync_api import sync_playwright\n")
	script.WriteString("import json\n\n")
	script.WriteString("with sync_playwright() as p:\n")
	script.WriteString("    browser = p.chromium.launch(headless=True)\n")
	script.WriteString("    page = browser.new_page()\n")
	script.WriteString("    try:\n")

	switch action {
	case "navigate":
		script.WriteString(fmt.Sprintf("        page.goto('%s', timeout=30000)\n", escURL))
		script.WriteString("        print(f'Navigated to: {page.url}')\n")
		script.WriteString("        print(f'Title: {page.title()}')\n")
	case "click":
		if url != "" {
			script.WriteString(fmt.Sprintf("        page.goto('%s', timeout=30000)\n", escURL))
		}
		script.WriteString(fmt.Sprintf("        page.click('%s', timeout=10000)\n", escSelector))
		script.WriteString(fmt.Sprintf("        print('Clicked: %s')\n", escSelector))
		script.WriteString("        page.wait_for_load_state('networkidle', timeout=10000)\n")
		script.WriteString("        print(f'Current URL: {page.url}')\n")
	case "type":
		if url != "" {
			script.WriteString(fmt.Sprintf("        page.goto('%s', timeout=30000)\n", escURL))
		}
		script.WriteString(fmt.Sprintf("        page.fill('%s', '%s', timeout=10000)\n", escSelector, escText))
		script.WriteString(fmt.Sprintf("        print('Typed into: %s')\n", escSelector))
	case "screenshot":
		if url != "" {
			script.WriteString(fmt.Sprintf("        page.goto('%s', timeout=30000)\n", escURL))
		}
		script.WriteString(fmt.Sprintf("        page.screenshot(path='%s', full_page=True)\n", escOutput))
		script.WriteString(fmt.Sprintf("        print('Screenshot saved to: %s')\n", escOutput))
	case "extract":
		if url != "" {
			script.WriteString(fmt.Sprintf("        page.goto('%s', timeout=30000)\n", escURL))
		}
		if selector != "" {
			script.WriteString(fmt.Sprintf("        el = page.query_selector('%s')\n", escSelector))
			script.WriteString("        if el:\n")
			script.WriteString("            print(el.inner_text())\n")
			script.WriteString("        else:\n")
			script.WriteString(fmt.Sprintf("            print('Element not found: %s')\n", escSelector))
		} else {
			script.WriteString("        print(page.inner_text('body'))\n")
		}
	case "evaluate":
		if url != "" {
			script.WriteString(fmt.Sprintf("        page.goto('%s', timeout=30000)\n", escURL))
		}
		script.WriteString(fmt.Sprintf("        result = page.evaluate('%s')\n", escText))
		script.WriteString("        print(json.dumps(result, indent=2, default=str))\n")
	}

	script.WriteString("    finally:\n")
	script.WriteString("        browser.close()\n")

	return script.String()
}
