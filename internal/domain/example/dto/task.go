package dto

import "time"

type CreateTaskInput struct {
	Title       string `json:"title" validate:"required,max=160"`
	Description string `json:"description" validate:"max=4000"`
}

type TaskResult struct {
	ID          uint64    `json:"id"`
	PublicID    string    `json:"public_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
