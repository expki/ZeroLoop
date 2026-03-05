package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/expki/ZeroLoop.git/llm"
)

type VisionTool struct{}

func (t *VisionTool) Name() string { return "vision_load" }

func (t *VisionTool) Description() string {
	return "Load an image from the filesystem and inject it into the conversation for visual analysis. Supports PNG, JPEG, and common image formats. The image will be compressed and sent to the LLM as multimodal content."
}

func (t *VisionTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the image file",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "Optional question about the image",
			},
		},
		"required": []string{"path"},
	}
}

func (t *VisionTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading image: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	// Determine mime type from extension
	ext := strings.ToLower(filepath.Ext(path))
	mimeType := "image/jpeg"
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".bmp":
		mimeType = "image/bmp"
	}

	// Compress to JPEG if not already, to reduce size
	if ext != ".jpg" && ext != ".jpeg" {
		reader := bytes.NewReader(data)
		img, _, err := image.Decode(reader)
		if err == nil {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err == nil {
				data = buf.Bytes()
				mimeType = "image/jpeg"
			}
		}
	}

	// Check file size (max 5MB after compression)
	if len(data) > 5*1024*1024 {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Image too large (%d bytes). Maximum 5MB after compression.", len(data)),
			BreakLoop: false,
		}, nil
	}

	// Encode to base64
	b64 := base64.StdEncoding.EncodeToString(data)

	// Build multimodal message for the LLM
	question, _ := args["question"].(string)
	if question == "" {
		question = "Describe this image in detail."
	}

	// Inject image into agent history as a user message with multimodal content
	imageContent := []map[string]any{
		{
			"type": "text",
			"text": fmt.Sprintf("[Image loaded from %s] %s", filepath.Base(path), question),
		},
		{
			"type": "image_url",
			"image_url": map[string]string{
				"url": fmt.Sprintf("data:%s;base64,%s", mimeType, b64),
			},
		},
	}

	a.History = append(a.History, llm.ChatMessage{
		Role:    "user",
		Content: imageContent,
	})

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Image loaded from %s (%d bytes). The image has been added to the conversation. Analyze it to answer: %s", filepath.Base(path), len(data), question),
		BreakLoop: false,
	}, nil
}
