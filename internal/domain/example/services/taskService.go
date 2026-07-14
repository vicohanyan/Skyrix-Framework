package services

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"

	"skyrix/internal/domain/example/dto"
	"skyrix/internal/domain/example/entity"
	"skyrix/internal/domain/example/interfaces"
)

type TaskService struct {
	repository interfaces.TaskRepository
	createUOW  interfaces.CreateTaskUnitOfWork
}

func NewTaskService(repository interfaces.TaskRepository, createUOW interfaces.CreateTaskUnitOfWork) *TaskService {
	return &TaskService{repository: repository, createUOW: createUOW}
}

func (s *TaskService) Create(ctx context.Context, input dto.CreateTaskInput) (dto.TaskResult, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return dto.TaskResult{}, interfaces.ErrTaskTitleRequired
	}
	task, err := s.createUOW.Create(ctx, &entity.Task{
		PublicID:    ulid.Make().String(),
		Title:       title,
		Description: strings.TrimSpace(input.Description),
		Status:      entity.TaskStatusPending,
	})
	if err != nil {
		return dto.TaskResult{}, err
	}
	return taskResult(task), nil
}

func (s *TaskService) Get(ctx context.Context, id uint64) (dto.TaskResult, error) {
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return dto.TaskResult{}, err
	}
	return taskResult(task), nil
}

func (s *TaskService) List(ctx context.Context, limit int) ([]dto.TaskResult, error) {
	tasks, err := s.repository.List(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]dto.TaskResult, 0, len(tasks))
	for index := range tasks {
		result = append(result, taskResult(&tasks[index]))
	}
	return result, nil
}

func (s *TaskService) Complete(ctx context.Context, id uint64) (dto.TaskResult, error) {
	changed, err := s.repository.UpdateStatus(ctx, id, entity.TaskStatusPending, entity.TaskStatusCompleted)
	if err != nil {
		return dto.TaskResult{}, err
	}
	if !changed {
		task, getErr := s.repository.Get(ctx, id)
		if getErr != nil {
			return dto.TaskResult{}, getErr
		}
		if task.Status == entity.TaskStatusCompleted {
			return dto.TaskResult{}, interfaces.ErrTaskAlreadyCompleted
		}
	}
	return s.Get(ctx, id)
}

func taskResult(task *entity.Task) dto.TaskResult {
	return dto.TaskResult{
		ID: task.ID, PublicID: task.PublicID, Title: task.Title,
		Description: task.Description, Status: task.Status,
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
	}
}

var _ interfaces.TaskService = (*TaskService)(nil)
