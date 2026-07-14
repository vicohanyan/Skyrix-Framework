package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"skyrix/internal/domain/example/dto"
	"skyrix/internal/domain/example/interfaces"
	"skyrix/internal/logger"
	"skyrix/internal/validation"
)

type ExampleTaskHandler struct {
	*BaseHandler
	service interfaces.TaskService
}

func NewExampleTaskHandler(log logger.Interface, validator *validation.Validator, service interfaces.TaskService) *ExampleTaskHandler {
	handler := &ExampleTaskHandler{
		BaseHandler: &BaseHandler{Logger: log, Validator: validator},
		service:     service,
	}
	handler.BaseHandler.WithAutoName(handler)
	return handler
}

// Create godoc
// @Summary Create an example task
// @Description Creates a task with PENDING status.
// @Tags example tasks
// @Accept json
// @Produce json
// @Param payload body dto.CreateTaskInput true "Task payload"
// @Success 201 {object} dto.TaskResult
// @Failure 400 {object} ErrorPayload
// @Failure 500 {object} ErrorPayload
// @Router /api/v1/examples/tasks/ [post]
func (h *ExampleTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.CreateTaskInput
	if !h.DecodeJSON(w, r, &input, 1<<20) {
		return
	}
	if details := h.MapValidationErrors(input); len(details) > 0 {
		h.WriteJSON(w, http.StatusBadRequest, ErrorPayload{Error: ErrorBody{
			Code: ErrCodeValidation, Message: "Validation failed", Details: details,
		}})
		return
	}
	result, err := h.service.Create(r.Context(), input)
	if err != nil {
		h.HandleError(w, r, err, "Failed to create example task", http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, http.StatusCreated, result)
}

// Get godoc
// @Summary Get an example task
// @Description Returns an example task by its numeric identifier.
// @Tags example tasks
// @Produce json
// @Param task_id path int true "Task ID" minimum(1)
// @Success 200 {object} dto.TaskResult
// @Failure 400 {object} ErrorPayload
// @Failure 404 {object} ErrorPayload
// @Failure 500 {object} ErrorPayload
// @Router /api/v1/examples/tasks/{task_id} [get]
func (h *ExampleTaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.taskID(w, r)
	if !ok {
		return
	}
	result, err := h.service.Get(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}
	h.WriteJSON(w, http.StatusOK, result)
}

// List godoc
// @Summary List example tasks
// @Description Returns example tasks in descending creation order.
// @Tags example tasks
// @Produce json
// @Param limit query int false "Maximum number of tasks" default(20) minimum(1) maximum(100)
// @Success 200 {array} dto.TaskResult
// @Failure 500 {object} ErrorPayload
// @Router /api/v1/examples/tasks/ [get]
func (h *ExampleTaskHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.service.List(r.Context(), limit)
	if err != nil {
		h.HandleError(w, r, err, "Failed to list example tasks", http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, http.StatusOK, result)
}

// Complete godoc
// @Summary Complete an example task
// @Description Changes an example task status from PENDING to COMPLETED.
// @Tags example tasks
// @Produce json
// @Param task_id path int true "Task ID" minimum(1)
// @Success 200 {object} dto.TaskResult
// @Failure 400 {object} ErrorPayload
// @Failure 404 {object} ErrorPayload
// @Failure 409 {object} ErrorPayload
// @Failure 500 {object} ErrorPayload
// @Router /api/v1/examples/tasks/{task_id}/complete [patch]
func (h *ExampleTaskHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.taskID(w, r)
	if !ok {
		return
	}
	result, err := h.service.Complete(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}
	h.WriteJSON(w, http.StatusOK, result)
}

func (h *ExampleTaskHandler) taskID(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	id, err := strconv.ParseUint(chi.URLParam(r, "task_id"), 10, 64)
	if err != nil || id == 0 {
		h.HandleError(w, r, err, "Invalid task ID", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func (h *ExampleTaskHandler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, interfaces.ErrTaskNotFound):
		h.HandleError(w, r, err, "Example task not found", http.StatusNotFound)
	case errors.Is(err, interfaces.ErrTaskAlreadyCompleted):
		h.HandleError(w, r, err, "Example task is already completed", http.StatusConflict)
	default:
		h.HandleError(w, r, err, "Example task operation failed", http.StatusInternalServerError)
	}
}
