package interfaces

import (
	"context"
	"errors"

	"skyrix/internal/domain/example/dto"
	"skyrix/internal/domain/example/entity"
)

var (
	ErrTaskNotFound         = errors.New("example task not found")
	ErrTaskTitleRequired    = errors.New("example task title is required")
	ErrTaskAlreadyCompleted = errors.New("example task is already completed")
)

type TaskService interface {
	Create(ctx context.Context, input dto.CreateTaskInput) (dto.TaskResult, error)
	Get(ctx context.Context, id uint64) (dto.TaskResult, error)
	List(ctx context.Context, limit int) ([]dto.TaskResult, error)
	Complete(ctx context.Context, id uint64) (dto.TaskResult, error)
}

type TaskRepository interface {
	Create(ctx context.Context, task *entity.Task) (*entity.Task, error)
	Get(ctx context.Context, id uint64) (*entity.Task, error)
	List(ctx context.Context, limit int) ([]entity.Task, error)
	UpdateStatus(ctx context.Context, id uint64, from, to string) (bool, error)
}

type CreateTaskUnitOfWork interface {
	Create(ctx context.Context, task *entity.Task) (*entity.Task, error)
}
