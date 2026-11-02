package unitofwork

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"
	"gitlab.com/skyrix-lib/template/internal/domain/example/entity"
	"gitlab.com/skyrix-lib/template/internal/domain/example/repository"
)

type CreateTaskUOW struct {
	db   *gormdb.Database
	repo *repository.TaskRepository
}

func NewCreateTaskUnitOfWork(db *gormdb.Database, repo *repository.TaskRepository) *CreateTaskUOW {
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
