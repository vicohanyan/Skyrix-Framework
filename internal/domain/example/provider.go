package example

import (
	"github.com/google/wire"

	"skyrix/internal/domain/example/interfaces"
	"skyrix/internal/domain/example/repository"
	"skyrix/internal/domain/example/services"
	unitofwork "skyrix/internal/domain/example/unitOfWork"
)

var ProviderSet = wire.NewSet(
	repository.NewTaskRepository,
	unitofwork.NewCreateTaskUnitOfWork,
	services.NewTaskService,
	wire.Bind(new(interfaces.TaskRepository), new(*repository.TaskRepository)),
	wire.Bind(new(interfaces.CreateTaskUnitOfWork), new(*unitofwork.CreateTaskUOW)),
	wire.Bind(new(interfaces.TaskService), new(*services.TaskService)),
)
