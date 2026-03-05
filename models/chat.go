package models

import "time"

type Chat struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"type:text;not null;index" json:"project_id"`
	Name      string    `gorm:"type:text;not null;default:'New Chat'" json:"name"`
	Running   bool      `gorm:"not null;default:false" json:"running"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Messages  []Message `gorm:"foreignKey:ChatID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
}
