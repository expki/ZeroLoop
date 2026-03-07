package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SystemPrompt generates the main system prompt for the agent
func SystemPrompt(agentNo int, tools []Tool) string {
	return SystemPromptWithProfile(agentNo, "", "", tools)
}

// SystemPromptForMode generates a mode-aware system prompt.
func SystemPromptForMode(agentNo int, profileName, projectDir string, tools []Tool, agentType, agentMode string) string {
	basePrompt := SystemPromptWithProfile(agentNo, profileName, projectDir, tools)

	var modeSection string
	switch agentMode {
	case "orchestrator":
		modeSection = `

## Operating Mode: Orchestrator

You are an orchestrator agent. Your job is to decompose user requests into parallel sub-tasks.
You do NOT execute tasks yourself — you only plan and delegate.

When given a task:
1. Analyze the request and break it into independent, parallelizable sub-tasks
2. Output a JSON array of sub-task objects, each with a "description" field
3. Each sub-task should be self-contained and clearly scoped
4. Aim for 2-6 sub-tasks (avoid over-decomposition)

Example output format:
[
  {"description": "Implement the data model for user authentication"},
  {"description": "Create the login API endpoint with JWT token generation"},
  {"description": "Write unit tests for the authentication flow"}
]

Do NOT use tools or execute code. Only output the JSON array of sub-tasks.`

	case "oneshot":
		modeSection = `

## Operating Mode: Autonomous (One-shot)

You are an autonomous agent running in one-shot mode. Execute the given task to completion
without waiting for user input. If you need clarification, make reasonable assumptions and proceed.
Do not ask the user questions in your text output — work independently.
Use all available tools to complete the task thoroughly.
When done, use the response tool to deliver your final result.`

	case "infinite":
		modeSection = `

## Operating Mode: Autonomous (Continuous Improvement)

You are a continuous improvement agent. Your job is to autonomously discover and implement
improvements in the project. After completing each task:

1. Analyze the project for improvement opportunities:
   - Code quality issues (naming, structure, duplication)
   - Missing or incomplete tests
   - Bug risks or edge cases
   - Performance optimization opportunities
   - Documentation gaps
2. Pick the highest-impact improvement
3. Implement it using available tools
4. Verify the change works correctly
5. Use the response tool to summarize what you did

Do not ask the user questions — work independently. Make reasonable assumptions.
Focus on small, safe, verifiable improvements. Never make destructive changes without verification.`
	}

	if modeSection != "" {
		return basePrompt + modeSection
	}
	return basePrompt
}

// SystemPromptWithProfile generates the system prompt with a specific agent profile.
// If projectDir is non-empty, it is used as the working directory context instead of os.Getwd().
func SystemPromptWithProfile(agentNo int, profileName string, projectDir string, tools []Tool) string {
	// Load profile
	profile, _ := LoadProfile(profileName)

	// Build tool descriptions
	var toolDescs strings.Builder
	if len(tools) == 0 {
		toolDescs.WriteString("No tools are currently registered.\n")
	} else {
		for _, tool := range tools {
			toolDescs.WriteString(fmt.Sprintf("### `%s`\n%s\n\n", tool.Name(), tool.Description()))
		}
	}

	// Build template variables — use project dir if available
	cwd := projectDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	hostname, _ := os.Hostname()

	roleType := profile.RoleType
	if agentNo > 0 {
		roleType = fmt.Sprintf("Subordinate agent (%s) — report results back to superior", profile.RoleType)
	}

	vars := map[string]string{
		"agent_number":      fmt.Sprintf("%d", agentNo),
		"datetime":          time.Now().Format("2006-01-02 15:04:05 MST"),
		"workdir":           cwd,
		"hostname":          hostname,
		"agent_role":        profile.Role,
		"agent_role_type":   roleType,
		"tool_descriptions": toolDescs.String(),
	}

	// Try template-based rendering first
	templatePaths := []string{
		filepath.Join("prompts", "main.md"),
	}
	if exePath, err := os.Executable(); err == nil {
		templatePaths = append(templatePaths, filepath.Join(filepath.Dir(exePath), "prompts", "main.md"))
	}

	// Check for profile-specific prompt override
	if profileName != "" {
		profilePrompt := filepath.Join("agents", profileName, "prompts", "main.md")
		templatePaths = append([]string{profilePrompt}, templatePaths...)
	}

	for _, path := range templatePaths {
		rendered, err := RenderTemplate(path, vars)
		if err == nil {
			// Resolve dynamic sections
			rendered = resolveDynamicSections(rendered, agentNo, cwd)
			return rendered
		}
	}

	// Fallback to hardcoded prompt if no templates available
	return buildFallbackPrompt(agentNo, profile, cwd, tools)
}

// resolveDynamicSections fills in runtime-generated content
func resolveDynamicSections(content string, agentNo int, workdir string) string {
	// Working directory context
	workdirCtx := getWorkdirContext(workdir)
	if workdirCtx != "" {
		replacement := "\n## Working Directory Contents\n\n```\n" + workdirCtx + "\n```\n"
		content = strings.ReplaceAll(content, "{{workdir_context}}", replacement)
	} else {
		content = strings.ReplaceAll(content, "{{workdir_context}}", "")
	}

	// Behaviour rules
	rules := loadBehaviourRules()
	if rules != "" {
		replacement := "\n## Custom Behavioral Rules\n\n" + rules + "\n"
		content = strings.ReplaceAll(content, "{{behaviour_rules}}", replacement)
	} else {
		content = strings.ReplaceAll(content, "{{behaviour_rules}}", "")
	}

	// Loaded skills
	content = strings.ReplaceAll(content, "{{loaded_skills}}", "")

	return content
}

// buildFallbackPrompt constructs the system prompt without templates
func buildFallbackPrompt(agentNo int, profile *Profile, workdir string, tools []Tool) string {
	var sb strings.Builder

	sb.WriteString("# Agent Instructions\n\n")
	sb.WriteString(fmt.Sprintf("You are Agent %d (A%d), an autonomous AI agent that solves tasks by reasoning, ", agentNo, agentNo))
	sb.WriteString("using tools, and coordinating with subordinate agents when beneficial.\n")
	sb.WriteString(fmt.Sprintf("Current date and time: %s\n\n", time.Now().Format("2006-01-02 15:04:05 MST")))

	// Role
	sb.WriteString("## Role\n\n")
	sb.WriteString(profile.Role)
	sb.WriteString("\n\n")

	// Environment
	sb.WriteString("## Environment\n\n")
	sb.WriteString(fmt.Sprintf("- Working directory: `%s`\n", workdir))
	if hostname, err := os.Hostname(); err == nil {
		sb.WriteString(fmt.Sprintf("- Hostname: `%s`\n", hostname))
	}
	sb.WriteString(fmt.Sprintf("- Agent number: %d\n", agentNo))
	if agentNo > 0 {
		sb.WriteString("- Role: Subordinate agent (report results back to superior)\n")
	} else {
		sb.WriteString(fmt.Sprintf("- Role: %s\n", profile.RoleType))
	}
	sb.WriteString("\n")

	// Methodology
	sb.WriteString("## Problem-Solving Methodology\n\n")
	sb.WriteString("Follow this approach for every task:\n\n")
	sb.WriteString("1. **Search knowledge first** — Use `knowledge` or `memory` before attempting anything else.\n")
	sb.WriteString("2. **Break down the task** — Decompose complex problems into discrete, verifiable subtasks.\n")
	sb.WriteString("3. **Execute step by step** — Use `code_execution` to run commands and verify real outputs.\n")
	sb.WriteString("4. **Verify results** — After each step, confirm the outcome. Do not assume success.\n")
	sb.WriteString("5. **Delegate purposefully** — Use `call_subordinate` for well-scoped subtasks.\n")
	sb.WriteString("6. **Persist through failure** — Retry with a different approach on error.\n")
	sb.WriteString("7. **Deliver results** — Use `response` to present a clear, final answer.\n\n")

	// Communication
	sb.WriteString("## Communication\n\n")
	sb.WriteString("- Be concise and direct.\n")
	sb.WriteString("- Use markdown formatting for clarity.\n")
	sb.WriteString("- Always use the `response` tool for final answers.\n\n")

	// Available tools
	sb.WriteString("## Available Tools\n\n")
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("### `%s`\n%s\n\n", tool.Name(), tool.Description()))
	}

	// Rules
	sb.WriteString("## Rules\n\n")
	sb.WriteString("1. **Never fabricate tool results.**\n")
	sb.WriteString("2. **Verify before reporting.**\n")
	sb.WriteString("3. **Be high-agency.** Retry on failure.\n")
	sb.WriteString("4. **Avoid repetition.**\n")
	sb.WriteString("5. **Ensure forward progress.**\n")
	sb.WriteString("6. **Save useful knowledge.**\n")
	sb.WriteString("7. **One subordinate per subtask.**\n")
	sb.WriteString("8. **Final answer via `response` only.**\n")

	// Working directory context
	workdirCtx := getWorkdirContext(workdir)
	if workdirCtx != "" {
		sb.WriteString("\n## Working Directory Contents\n\n```\n")
		sb.WriteString(workdirCtx)
		sb.WriteString("\n```\n")
	}

	// Behaviour rules
	rules := loadBehaviourRules()
	if rules != "" {
		sb.WriteString("\n## Custom Behavioral Rules\n\n")
		sb.WriteString(rules)
		sb.WriteString("\n")
	}

	return sb.String()
}

// getWorkdirContext returns a tree-like listing of the working directory (max 3 levels deep)
func getWorkdirContext(workdir string) string {
	if workdir == "" {
		return ""
	}

	var sb strings.Builder
	walkDir(workdir, "", 0, 3, &sb)
	return sb.String()
}

func walkDir(dir, prefix string, depth, maxDepth int, sb *strings.Builder) {
	if depth >= maxDepth {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var filtered []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor" {
			continue
		}
		filtered = append(filtered, e)
	}

	for i, entry := range filtered {
		isLast := i == len(filtered)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		sb.WriteString(prefix + connector + name + "\n")

		if entry.IsDir() {
			childPrefix := prefix + "│   "
			if isLast {
				childPrefix = prefix + "    "
			}
			walkDir(filepath.Join(dir, entry.Name()), childPrefix, depth+1, maxDepth, sb)
		}
	}
}

// loadBehaviourRules reads the behaviour.md file if it exists
func loadBehaviourRules() string {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, "behaviour.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
