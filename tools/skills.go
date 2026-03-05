package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/expki/ZeroLoop.git/agent"
)

// loadedSkills tracks currently loaded skills per agent
var loadedSkills = struct {
	sync.Mutex
	m map[int][]string // agentNo -> loaded skill names
}{m: make(map[int][]string)}

const maxLoadedSkills = 5

type SkillsTool struct{}

func (t *SkillsTool) Name() string { return "skills" }

func (t *SkillsTool) Description() string {
	return "Manage SKILL.md-based skills. Methods: 'list' shows all available skills, 'load' injects a skill's content into your context (max 5 loaded). Skills are markdown files in the skills/ directory."
}

func (t *SkillsTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"enum":        []string{"list", "load"},
				"description": "Action to perform",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "For load: name of the skill to load",
			},
		},
		"required": []string{"method"},
	}
}

func getSkillsDir() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "skills")
}

func (t *SkillsTool) Execute(ctx context.Context, a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	method, _ := args["method"].(string)

	switch method {
	case "list":
		return t.executeList()
	case "load":
		return t.executeLoad(a, args)
	default:
		return nil, fmt.Errorf("invalid method: %s (use 'list' or 'load')", method)
	}
}

func (t *SkillsTool) executeList() (*agent.ToolResult, error) {
	dir := getSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &agent.ToolResult{
				Message:   "No skills directory found. Create a skills/ directory with SKILL.md files.",
				BreakLoop: false,
			}, nil
		}
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Error reading skills directory: %s", err.Error()),
			BreakLoop: false,
		}, nil
	}

	var skills []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check for SKILL.md inside the directory
			skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err == nil {
				skills = append(skills, entry.Name())
			}
		} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			skills = append(skills, name)
		}
	}

	if len(skills) == 0 {
		return &agent.ToolResult{
			Message:   "No skills found in skills/ directory.",
			BreakLoop: false,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available skills (%d):\n\n", len(skills)))
	for i, s := range skills {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}

	return &agent.ToolResult{
		Message:   sb.String(),
		BreakLoop: false,
	}, nil
}

func (t *SkillsTool) executeLoad(a *agent.Agent, args map[string]any) (*agent.ToolResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required for load method")
	}

	// Check loaded skills limit
	loadedSkills.Lock()
	loaded := loadedSkills.m[a.Number]
	for _, s := range loaded {
		if s == name {
			loadedSkills.Unlock()
			return &agent.ToolResult{
				Message:   fmt.Sprintf("Skill %q is already loaded.", name),
				BreakLoop: false,
			}, nil
		}
	}
	if len(loaded) >= maxLoadedSkills {
		loadedSkills.Unlock()
		return &agent.ToolResult{
			Message:   fmt.Sprintf("Maximum %d skills can be loaded. Currently loaded: %s", maxLoadedSkills, strings.Join(loaded, ", ")),
			BreakLoop: false,
		}, nil
	}
	loadedSkills.Unlock()

	// Find and read the skill file
	dir := getSkillsDir()
	var content []byte
	var err error

	// Try directory format first
	skillFile := filepath.Join(dir, name, "SKILL.md")
	content, err = os.ReadFile(skillFile)
	if err != nil {
		// Try flat file format
		skillFile = filepath.Join(dir, name+".md")
		content, err = os.ReadFile(skillFile)
		if err != nil {
			return &agent.ToolResult{
				Message:   fmt.Sprintf("Skill %q not found.", name),
				BreakLoop: false,
			}, nil
		}
	}

	// Register as loaded
	loadedSkills.Lock()
	loadedSkills.m[a.Number] = append(loadedSkills.m[a.Number], name)
	loadedSkills.Unlock()

	return &agent.ToolResult{
		Message:   fmt.Sprintf("Skill %q loaded:\n\n%s", name, string(content)),
		BreakLoop: false,
	}, nil
}

// GetLoadedSkills returns the content of all loaded skills for an agent
func GetLoadedSkills(agentNo int) string {
	loadedSkills.Lock()
	loaded := loadedSkills.m[agentNo]
	loadedSkills.Unlock()

	if len(loaded) == 0 {
		return ""
	}

	var sb strings.Builder
	dir := getSkillsDir()
	for _, name := range loaded {
		var content []byte
		skillFile := filepath.Join(dir, name, "SKILL.md")
		content, err := os.ReadFile(skillFile)
		if err != nil {
			skillFile = filepath.Join(dir, name+".md")
			content, _ = os.ReadFile(skillFile)
		}
		if len(content) > 0 {
			sb.WriteString(fmt.Sprintf("\n### Skill: %s\n%s\n", name, string(content)))
		}
	}
	return sb.String()
}
