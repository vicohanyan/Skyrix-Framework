package entity

import "time"

const (
	TaskStatusPending   = "PENDING"
	TaskStatusCompleted = "COMPLETED"
)

type Task struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	PublicID    string `gorm:"type:varchar(40);not null;uniqueIndex"`
	Title       string `gorm:"type:varchar(160);not null"`
	Description string `gorm:"type:text;not null;default:''"`
	Status      string `gorm:"type:varchar(24);not null;index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Task) TableName() string { return "framework_example_tasks" }
