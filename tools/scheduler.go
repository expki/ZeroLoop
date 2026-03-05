package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
	"github.com/google/uuid"
)

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Schedule    string    `json:"schedule"` // cron expression, or empty for adhoc/planned
	NextRun     time.Time `json:"next_run"`
	LastRun     time.Time `json:"last_run,omitempty"`
	Status      string    `json:"status"` // pending, running, completed, failed
	Result      string    `json:"result,omitempty"`
	TaskType    string    `json:"task_type"` // scheduled, adhoc, planned
	PlannedRuns []string  `json:"planned_runs,omitempty"` // ISO timestamps for planned tasks
	CreatedAt   time.Time `json:"created_at"`
}

// taskStore is the in-memory task store
var taskStore = struct {
	sync.Mutex
	tasks map[string]*ScheduledTask
}{tasks: make(map[string]*ScheduledTask)}

type SchedulerTool struct{}

func (t *SchedulerTool) Name() string { return "scheduler" }

func (t *SchedulerTool) Description() string {
	return "Manage scheduled tasks. Methods: 'list_tasks', 'find_task_by_name', 'show_task', 'run_task', 'delete_task', 'create_scheduled_task' (cron), 'create_adhoc_task', 'create_planned_task' (specific times)."
}

func (t *SchedulerTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"enum":        []string{"list_tasks", "find_task_by_name", "show_task", "run_task", "delete_task", "create_scheduled_task", "create_adhoc_task", "create_planned_task"},
				"description": "Operation to perform",
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "Task ID (for show_task, run_task, delete_task)",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Task name (for find_task_by_name or create operations)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Task description/instructions for the agent when running",
			},
			"schedule": map[string]any{
				"type":        "string",
				"description": "Cron expression for scheduled tasks (e.g., '*/5 * * * *' for every 5 minutes)",
			},
			"planned_times": map[string]any{
				"type":        "string",
				"description": "Comma-separated ISO timestamps for planned tasks",
			},
		},
		"required": []string{"method"},
	}
}

func (t *SchedulerTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	method, _ := args["method"].(string)

	switch method {
	case "list_tasks":
		return t.listTasks()
	case "find_task_by_name":
		return t.findTaskByName(args)
	case "show_task":
		return t.showTask(args)
	case "run_task":
		return t.runTask(ctx, a, args)
	case "delete_task":
		return t.deleteTask(args)
	case "create_scheduled_task":
		return t.createScheduledTask(args)
	case "create_adhoc_task":
		return t.createAdhocTask(args)
	case "create_planned_task":
		return t.createPlannedTask(args)
	default:
		return nil, fmt.Errorf("invalid method: %s", method)
	}
}

func (t *SchedulerTool) listTasks() (*agent.ToolResult, error) {
	taskStore.Lock()
	defer taskStore.Unlock()

	if len(taskStore.tasks) == 0 {
		return &agent.ToolResult{
			Message:   "No scheduled tasks.",
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Scheduled tasks (%d):\n\n", len(taskStore.tasks)))
	for _, task := range taskStore.tasks {
		sb.WriteString(fmt.Sprintf("- **%s** (id: %s)\n  Type: %s | Status: %s | Schedule: %s\n",
			task.Name, task.ID, task.TaskType, task.Status, task.Schedule))
		if !task.NextRun.IsZero() {
			sb.WriteString(fmt.Sprintf("  Next run: %s\n", task.NextRun.Format(time.RFC3339)))
		}
		sb.WriteString("\n")
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) findTaskByName(args map[string]any) (*agent.ToolResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	taskStore.Lock()
	defer taskStore.Unlock()

	name = strings.ToLower(name)
	for _, task := range taskStore.tasks {
		if strings.Contains(strings.ToLower(task.Name), name) {
			data, _ := json.MarshalIndent(task, "", "  ")
			return &agent.ToolResult{
				Message:   fmt.Sprintf("Found task:\n%s", string(data)),
				BreakLoop: false,
			}, nil
		}
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("No task found matching: %q", name),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) showTask(args map[string]any) (*agent.ToolResult, error) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	taskStore.Lock()
	task, ok := taskStore.tasks[taskID]
	taskStore.Unlock()

	if !ok {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Task not found: %s", taskID),
			BreakLoop: false,
		}, nil
	}

	data, _ := json.MarshalIndent(task, "", "  ")
	return &agent.ToolResult{
		Message:   string(data),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) runTask(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	taskStore.Lock()
	task, ok := taskStore.tasks[taskID]
	if !ok {
		taskStore.Unlock()
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Task not found: %s", taskID),
			BreakLoop: false,
		}, nil
	}
	task.Status = "running"
	task.LastRun = time.Now()
	taskStore.Unlock()

	// Run the task as a subordinate agent
	sub := a.NewSubordinate()
	result, err := sub.Run(ctx, task.Description)

	taskStore.Lock()
	if err != nil {
		task.Status = "failed"
		task.Result = err.Error()
	} else {
		task.Status = "completed"
		task.Result = result
	}
	taskStore.Unlock()

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Task %q %s.\nResult: %s", task.Name, task.Status, task.Result),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) deleteTask(args map[string]any) (*agent.ToolResult, error) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	taskStore.Lock()
	task, ok := taskStore.tasks[taskID]
	if ok {
		delete(taskStore.tasks, taskID)
	}
	taskStore.Unlock()

	if !ok {
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Task not found: %s", taskID),
			BreakLoop: false,
		}, nil
	}

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Deleted task: %q", task.Name),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) createScheduledTask(args map[string]any) (*agent.ToolResult, error) {
	name, _ := args["name"].(string)
	desc, _ := args["description"].(string)
	schedule, _ := args["schedule"].(string)

	if name == "" || desc == "" || schedule == "" {
		return nil, fmt.Errorf("name, description, and schedule are required")
	}

	task := &ScheduledTask{
		ID:          uuid.New().String(),
		Name:        name,
		Description: desc,
		Schedule:    schedule,
		Status:      "pending",
		TaskType:    "scheduled",
		CreatedAt:   time.Now(),
	}

	taskStore.Lock()
	taskStore.tasks[task.ID] = task
	taskStore.Unlock()

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Created scheduled task %q (id: %s, schedule: %s)", name, task.ID, schedule),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) createAdhocTask(args map[string]any) (*agent.ToolResult, error) {
	name, _ := args["name"].(string)
	desc, _ := args["description"].(string)

	if name == "" || desc == "" {
		return nil, fmt.Errorf("name and description are required")
	}

	task := &ScheduledTask{
		ID:          uuid.New().String(),
		Name:        name,
		Description: desc,
		Status:      "pending",
		TaskType:    "adhoc",
		CreatedAt:   time.Now(),
	}

	taskStore.Lock()
	taskStore.tasks[task.ID] = task
	taskStore.Unlock()

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Created adhoc task %q (id: %s)", name, task.ID),
		BreakLoop: false,
	}, nil
}

func (t *SchedulerTool) createPlannedTask(args map[string]any) (*agent.ToolResult, error) {
	name, _ := args["name"].(string)
	desc, _ := args["description"].(string)
	plannedTimes, _ := args["planned_times"].(string)

	if name == "" || desc == "" || plannedTimes == "" {
		return nil, fmt.Errorf("name, description, and planned_times are required")
	}

	times := strings.Split(plannedTimes, ",")
	var parsedTimes []string
	var firstRun time.Time
	for _, t := range times {
		t = strings.TrimSpace(t)
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %s", t)
		}
		parsedTimes = append(parsedTimes, t)
		if firstRun.IsZero() || parsed.Before(firstRun) {
			firstRun = parsed
		}
	}

	task := &ScheduledTask{
		ID:          uuid.New().String(),
		Name:        name,
		Description: desc,
		Status:      "pending",
		TaskType:    "planned",
		PlannedRuns: parsedTimes,
		NextRun:     firstRun,
		CreatedAt:   time.Now(),
	}

	taskStore.Lock()
	taskStore.tasks[task.ID] = task
	taskStore.Unlock()

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Created planned task %q (id: %s) with %d scheduled runs", name, task.ID, len(parsedTimes)),
		BreakLoop: false,
	}, nil
}
