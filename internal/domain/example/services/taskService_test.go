package services

import (
	"context"
	"errors"
	"testing"

	"skyrix/internal/domain/example/dto"
	"skyrix/internal/domain/example/entity"
	"skyrix/internal/domain/example/interfaces"
)

func TestTaskServiceCreateAndComplete(t *testing.T) {
	repo := &taskMemoryRepository{}
	service := NewTaskService(repo, repo)

	created, err := service.Create(context.Background(), dto.CreateTaskInput{Title: " Example task "})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != entity.TaskStatusPending || created.Title != "Example task" {
		t.Fatalf("created task = %+v", created)
	}

	completed, err := service.Complete(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if completed.Status != entity.TaskStatusCompleted {
		t.Fatalf("status = %q", completed.Status)
	}
}

func TestTaskServiceRejectsBlankTitle(t *testing.T) {
	repo := &taskMemoryRepository{}
	service := NewTaskService(repo, repo)
	_, err := service.Create(context.Background(), dto.CreateTaskInput{Title: "  "})
	if !errors.Is(err, interfaces.ErrTaskTitleRequired) {
		t.Fatalf("Create() error = %v", err)
	}
}

type taskMemoryRepository struct {
	task *entity.Task
}

func (r *taskMemoryRepository) Create(_ context.Context, task *entity.Task) (*entity.Task, error) {
	task.ID = 1
	r.task = task
	return task, nil
}
func (r *taskMemoryRepository) Get(_ context.Context, id uint64) (*entity.Task, error) {
	if r.task == nil || r.task.ID != id {
		return nil, interfaces.ErrTaskNotFound
	}
	return r.task, nil
}
func (r *taskMemoryRepository) List(context.Context, int) ([]entity.Task, error) {
	if r.task == nil {
		return nil, nil
	}
	return []entity.Task{*r.task}, nil
}
func (r *taskMemoryRepository) UpdateStatus(_ context.Context, id uint64, from, to string) (bool, error) {
	if r.task == nil || r.task.ID != id || r.task.Status != from {
		return false, nil
	}
	r.task.Status = to
	return true, nil
}
