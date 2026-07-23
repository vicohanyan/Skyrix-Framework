package example

import (
	"github.com/goforj/wire"

	"gitlab.com/skyrix-lib/template/internal/domain/example/interfaces"
	"gitlab.com/skyrix-lib/template/internal/domain/example/repository"
	"gitlab.com/skyrix-lib/template/internal/domain/example/services"
	unitofwork "gitlab.com/skyrix-lib/template/internal/domain/example/unitOfWork"
)

var ProviderSet = wire.NewSet(
	repository.NewTaskRepository,
	unitofwork.NewCreateTaskUnitOfWork,
	services.NewTaskService,
	wire.Bind(new(interfaces.TaskRepository), new(*repository.TaskRepository)),
	wire.Bind(new(interfaces.CreateTaskUnitOfWork), new(*unitofwork.CreateTaskUOW)),
	wire.Bind(new(interfaces.TaskService), new(*services.TaskService)),
)
