package handlers

import (
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/framework/validation" // Added import for validation
	"gitlab.com/skyrix-lib/template/internal/domain/subscriber/services"
)

type SubscriberHandler struct {
	*BaseHandler
	SubscriberService *services.SubscriberService
}

func NewSubscriberHandler(logger logger.Interface, subscriberService *services.SubscriberService, validator *validation.Validator) *SubscriberHandler {
	return &SubscriberHandler{
		BaseHandler:       &BaseHandler{HandlerName: "SubscriberHandler", Logger: logger, Validator: validator}, // Pass validator here
		SubscriberService: subscriberService,
	}
}
