package tools

import (
	"context"
	"fmt"

	"github.com/expki/ZeroLoop.git/agent"
)

type NotifyUserTool struct{}

func (t *NotifyUserTool) Name() string { return "notify_user" }

func (t *NotifyUserTool) Description() string {
	return "Send a notification to the user's UI. Use for important status updates, alerts, or progress indicators. Types: info, success, warning, error, progress."
}

func (t *NotifyUserTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"info", "success", "warning", "error", "progress"},
				"description": "Notification type",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Notification title/heading",
			},
			"detail": map[string]any{
				"type":        "string",
				"description": "Detailed notification message",
			},
		},
		"required": []string{"type", "title"},
	}
}

func (t *NotifyUserTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	notifType, _ := args["type"].(string)
	title, _ := args["title"].(string)
	detail, _ := args["detail"].(string)

	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Map notification type to log entry type
	logType := "info"
	switch notifType {
	case "success":
		logType = "info"
	case "warning":
		logType = "warning"
	case "error":
		logType = "error"
	case "progress":
		logType = "progress"
	}

	content := title
	if detail != "" {
		content = fmt.Sprintf("%s\n\n%s", title, detail)
	}

	a.Log(agent.LogEntry{
		Type:    logType,
		Heading: title,
		Content: content,
		Kvps: map[string]string{
			"notification_type": notifType,
		},
		AgentNo: a.Number,
	})

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Notification sent: [%s] %s", notifType, title),
		BreakLoop: false,
	}, nil
}
