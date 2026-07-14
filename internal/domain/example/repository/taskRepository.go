package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"skyrix/internal/domain/example/entity"
	"skyrix/internal/domain/example/interfaces"
	"skyrix/internal/engine"
)

type TaskRepository struct {
	db *engine.Database
}

func NewTaskRepository(db *engine.Database) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) WithTx(tx *gorm.DB) *TaskRepository {
	return &TaskRepository{db: &engine.Database{DB: tx, MainSchema: r.db.MainSchema}}
}

func (r *TaskRepository) Create(ctx context.Context, task *entity.Task) (*entity.Task, error) {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return nil, fmt.Errorf("create example task: %w", err)
	}
	return task, nil
}

func (r *TaskRepository) Get(ctx context.Context, id uint64) (*entity.Task, error) {
	var task entity.Task
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, interfaces.ErrTaskNotFound
		}
		return nil, fmt.Errorf("get example task: %w", err)
	}
	return &task, nil
}

func (r *TaskRepository) List(ctx context.Context, limit int) ([]entity.Task, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var tasks []entity.Task
	if err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list example tasks: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id uint64, from, to string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&entity.Task{}).
		Where("id = ? AND status = ?", id, from).
		Update("status", to)
	if result.Error != nil {
		return false, fmt.Errorf("update example task status: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
