package models

type Example struct {
	ID string `gorm:"primaryKey"`
}

func (Example) TableName() string {
	return "example"
}
