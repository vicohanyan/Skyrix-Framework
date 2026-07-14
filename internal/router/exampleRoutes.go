package router

import (
	"github.com/go-chi/chi/v5"

	"skyrix/internal/providers"
)

func mountExampleRoutes(r chi.Router, handlers *providers.Handlers) {
	r.Route("/examples/tasks", func(r chi.Router) {
		r.Post("/", handlers.ExampleTask.Create)
		r.Get("/", handlers.ExampleTask.List)
		r.Get("/{task_id}", handlers.ExampleTask.Get)
		r.Patch("/{task_id}/complete", handlers.ExampleTask.Complete)
	})
}
