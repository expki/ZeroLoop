package models

import "time"

type MessageType string

const (
	MessageTypeUser     MessageType = "user"
	MessageTypeAgent    MessageType = "agent"
	MessageTypeResponse MessageType = "response"
	MessageTypeTool     MessageType = "tool"
	MessageTypeCodeExe  MessageType = "code_exe"
	MessageTypeWarning  MessageType = "warning"
	MessageTypeError    MessageType = "error"
	MessageTypeInfo     MessageType = "info"
	MessageTypeUtil     MessageType = "util"
	MessageTypeHint     MessageType = "hint"
	MessageTypeProgress MessageType = "progress"
)

type Message struct {
	ID        string      `gorm:"primaryKey;type:text" json:"id"`
	ChatID    string      `gorm:"type:text;not null;index" json:"chat_id"`
	No        int         `gorm:"not null" json:"no"`
	Type      MessageType `gorm:"type:text;not null" json:"type"`
	Heading   string      `gorm:"type:text;not null;default:''" json:"heading"`
	Content   string      `gorm:"type:text;not null;default:''" json:"content"`
	Kvps      string      `gorm:"type:text;not null;default:'{}'" json:"kvps"`
	AgentNo   int         `gorm:"not null;default:0" json:"agentno"`
	CreatedAt time.Time   `gorm:"autoCreateTime" json:"timestamp"`
}
