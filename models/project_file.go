package models

import "time"

type ProjectFile struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"type:text;not null;index" json:"project_id"`
	Path      string    `gorm:"type:text;not null" json:"path"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	IsDir     bool      `gorm:"not null;default:false" json:"is_dir"`
	Size      int64     `gorm:"not null;default:0" json:"size"`
	MimeType  string    `gorm:"type:text;not null;default:''" json:"mime_type"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
