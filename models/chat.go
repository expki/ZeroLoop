package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AgentType represents the kind of agent
type AgentType string

const (
	AgentTypeStandard  AgentType = "standard"
	AgentTypeAutomated AgentType = "automated"
)

// AgentMode represents the operating mode within a type
type AgentMode string

const (
	AgentModeDirect       AgentMode = "direct"
	AgentModeOrchestrator AgentMode = "orchestrator"
	AgentModeOneshot      AgentMode = "oneshot"
	AgentModeInfinite     AgentMode = "infinite"
)

// AgentStatus represents the lifecycle state
type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusFailed    AgentStatus = "failed"
	AgentStatusPaused    AgentStatus = "paused"
)

type Agent struct {
	ID        string      `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string      `gorm:"type:text;not null;index" json:"project_id"`
	Name      string      `gorm:"type:text;not null;default:'New Agent'" json:"name"`
	Type      AgentType   `gorm:"type:text;not null;default:'standard'" json:"type"`
	Mode      AgentMode   `gorm:"type:text;not null;default:'direct'" json:"mode"`
	Status    AgentStatus `gorm:"type:text;not null;default:'idle'" json:"status"`
	ParentID  *string     `gorm:"type:text;index" json:"parent_id,omitempty"`
	Running   bool        `gorm:"not null;default:false" json:"running"`
	CreatedAt time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
	Messages  []Message   `gorm:"foreignKey:AgentID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
	Children  []Agent     `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE" json:"children,omitempty"`
}

// TableName preserves backward-compatible DB table name
func (Agent) TableName() string { return "chats" }

// BeforeSave keeps Running in sync with Status for backward compatibility
func (a *Agent) BeforeSave(tx *gorm.DB) error {
	a.Running = a.Status == AgentStatusRunning
	return nil
}

// ValidateTypeMode checks that the type/mode combination is valid
func (a *Agent) ValidateTypeMode() error {
	switch a.Type {
	case AgentTypeStandard, "":
		if a.Mode != AgentModeDirect && a.Mode != AgentModeOrchestrator && a.Mode != "" {
			return fmt.Errorf("standard agent does not support mode %q (use direct or orchestrator)", a.Mode)
		}
	case AgentTypeAutomated:
		if a.Mode != AgentModeOneshot && a.Mode != AgentModeInfinite {
			return fmt.Errorf("automated agent does not support mode %q (use oneshot or infinite)", a.Mode)
		}
	default:
		return fmt.Errorf("unknown agent type %q (use standard or automated)", a.Type)
	}
	return nil
}
