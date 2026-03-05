package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profile represents an agent profile configuration
type Profile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Role        string `json:"role"`
	RoleType    string `json:"role_type"`
}

// LoadProfile loads a profile from the agents/ directory
func LoadProfile(name string) (*Profile, error) {
	if name == "" {
		name = "default"
	}

	// Try multiple locations for the profile
	paths := []string{
		filepath.Join("agents", name, "profile.json"),
	}

	// Also check relative to executable
	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), "agents", name, "profile.json"))
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var profile Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			continue
		}
		return &profile, nil
	}

	// Return default profile if none found
	return &Profile{
		Name:        "default",
		Description: "Default agent profile",
		Role:        "You are a helpful, capable AI assistant that solves problems autonomously.",
		RoleType:    "Primary agent",
	}, nil
}

// RenderTemplate processes a prompt template by resolving includes and variables
func RenderTemplate(templatePath string, vars map[string]string) (string, error) {
	content, err := loadTemplate(templatePath)
	if err != nil {
		return "", err
	}

	return resolveTemplate(content, filepath.Dir(templatePath), vars, 0)
}

func loadTemplate(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("loading template %s: %w", path, err)
	}
	return string(data), nil
}

func resolveTemplate(content, baseDir string, vars map[string]string, depth int) (string, error) {
	if depth > 10 {
		return content, fmt.Errorf("max include depth exceeded")
	}

	// Process {{include filename}} directives
	result := content
	for {
		idx := strings.Index(result, "{{include ")
		if idx < 0 {
			break
		}
		end := strings.Index(result[idx:], "}}")
		if end < 0 {
			break
		}
		end += idx

		directive := result[idx+len("{{include ") : end]
		filename := strings.TrimSpace(directive)

		// Load the included file
		includePath := filepath.Join(baseDir, filename)
		included, err := loadTemplate(includePath)
		if err != nil {
			// If include not found, leave a placeholder
			included = fmt.Sprintf("<!-- include %s not found -->", filename)
		} else {
			// Recursively resolve includes
			included, _ = resolveTemplate(included, filepath.Dir(includePath), vars, depth+1)
		}

		result = result[:idx] + included + result[end+2:]
	}

	// Process {{variable}} placeholders
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// ListProfiles returns all available profile names
func ListProfiles() []string {
	profiles := []string{"default"}

	dirs := []string{"agents"}
	if exePath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exePath), "agents"))
	}

	seen := map[string]bool{"default": true}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), "_") && !seen[entry.Name()] {
				profilePath := filepath.Join(dir, entry.Name(), "profile.json")
				if _, err := os.Stat(profilePath); err == nil {
					profiles = append(profiles, entry.Name())
					seen[entry.Name()] = true
				}
			}
		}
	}

	return profiles
}
