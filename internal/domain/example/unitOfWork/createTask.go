package unitofwork

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"skyrix/internal/domain/example/entity"
	"skyrix/internal/domain/example/repository"
	"skyrix/internal/engine"
)

type CreateTaskUOW struct {
	db   *engine.Database
	repo *repository.TaskRepository
}

func NewCreateTaskUnitOfWork(db *engine.Database, repo *repository.TaskRepository) *CreateTaskUOW {
	return &CreateTaskUOW{db: db, repo: repo}
}

func (u *CreateTaskUOW) Create(ctx context.Context, task *entity.Task) (*entity.Task, error) {
	var created *entity.Task
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		created, err = u.repo.WithTx(tx).Create(ctx, task)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create example task transaction: %w", err)
	}
	return created, nil
}
