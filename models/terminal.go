package models

import "time"

type Terminal struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"type:text;not null;index" json:"project_id"`
	Name      string    `gorm:"type:text;not null;default:'Terminal'" json:"name"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
