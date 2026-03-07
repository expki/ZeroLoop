package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/expki/ZeroLoop.git/agent"
)

// sessionKey uniquely identifies a session (scoped per agent + session number)
type sessionKey struct {
	AgentID    string
	SessionNum int
}

// sessionState tracks working directory and environment across calls
type sessionState struct {
	Cwd string
	Env []string
}

// sessionStore manages session state across tool calls
type sessionStore struct {
	sessions map[sessionKey]*sessionState
	mu       sync.Mutex
}

var globalSessions = &sessionStore{
	sessions: make(map[sessionKey]*sessionState),
}

func (s *sessionStore) get(agentID string, num int, projectDir string) *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionKey{AgentID: agentID, SessionNum: num}
	state, ok := s.sessions[key]
	if !ok {
		cwd := projectDir
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		state = &sessionState{Cwd: cwd, Env: os.Environ()}
		s.sessions[key] = state
	}
	return state
}

func (s *sessionStore) reset(agentID string, num int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionKey{AgentID: agentID, SessionNum: num})
}

type CodeExecutionTool struct{}

func (t *CodeExecutionTool) Name() string { return "code_execution" }

func (t *CodeExecutionTool) Description() string {
	return "Execute short-lived terminal commands, Python, or Node.js code. Has a 2-minute timeout. Sessions preserve working directory and environment across calls. Use 'session' to select session number (0 default), 'reset' to clear session state. IMPORTANT: Do NOT use this for long-running processes like servers, watchers, or build tasks — use the run_process tool instead, which runs processes in the background with live output streaming."
}

func (t *CodeExecutionTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"runtime": map[string]any{
				"type":        "string",
				"enum":        []string{"shell", "python", "node"},
				"description": "The runtime: 'shell' for bash commands, 'python' for Python code, 'node' for Node.js code",
			},
			"code": map[string]any{
				"type":        "string",
				"description": "The code or command to execute",
			},
			"session": map[string]any{
				"type":        "integer",
				"description": "Session number (0 default). Different sessions maintain separate working directories.",
				"default":     0,
			},
			"reset": map[string]any{
				"type":        "boolean",
				"description": "Set true to reset session state (cwd, env). Use when you want a fresh start.",
				"default":     false,
			},
		},
		"required": []string{"runtime", "code"},
	}
}

func (t *CodeExecutionTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	runtime, _ := args["runtime"].(string)
	code, _ := args["code"].(string)
	if code == "" {
		return nil, fmt.Errorf(`code is required. Example: {"runtime": "shell", "code": "echo hello"}`)
	}

	sessionNum := 0
	if s, ok := args["session"].(float64); ok {
		sessionNum = int(s)
	}

	shouldReset := false
	if r, ok := args["reset"].(bool); ok {
		shouldReset = r
	}

	// Scope sessions per agent to prevent cross-agent state leakage
	agentID := ""
	if a != nil {
		agentID = a.AgentID
	}

	if shouldReset {
		globalSessions.reset(agentID, sessionNum)
	}

	projectDir := ""
	if a != nil {
		projectDir = a.ProjectDir
	}
	state := globalSessions.get(agentID, sessionNum, projectDir)

	execCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime {
	case "python":
		cmd = exec.CommandContext(execCtx, "python3", "-c", code)
	case "node":
		cmd = exec.CommandContext(execCtx, "node", "-e", code)
	default: // shell
		// Wrap in a subshell that captures the final cwd
		// so we can persist it for the next call
		wrappedCode := code + "\necho __ZL_CWD__$(pwd)"
		cmd = exec.CommandContext(execCtx, "bash", "-c", wrappedCode)
	}

	cmd.Dir = state.Cwd
	cmd.Env = state.Env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutStr := stdout.String()

	// Extract and persist working directory for shell commands
	if runtime == "" || runtime == "shell" {
		if idx := strings.LastIndex(stdoutStr, "__ZL_CWD__"); idx >= 0 {
			newCwd := strings.TrimSpace(stdoutStr[idx+len("__ZL_CWD__"):])
			if newCwd != "" {
				state.Cwd = newCwd
			}
			stdoutStr = stdoutStr[:idx]
		}
	}

	// Also capture environment changes from export commands
	// (limited - full env persistence would need a more complex approach)

	var result strings.Builder
	stdoutStr = strings.TrimRight(stdoutStr, "\n")
	if stdoutStr != "" {
		result.WriteString(stdoutStr)
	}
	stderrStr := strings.TrimRight(stderr.String(), "\n")
	if stderrStr != "" {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderrStr)
	}
	if err != nil {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("Exit: %s", err.Error()))
	}

	output := result.String()
	if output == "" {
		output = "(no output)"
	}

	// Truncate very long output
	if len(output) > 15000 {
		output = output[:15000] + "\n... (output truncated)"
	}

	return &agent.ToolResult{
		Message:   output,
		BreakLoop: false,
	}, nil
}
