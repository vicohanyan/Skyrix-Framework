package handlers

import (
	"skyrix/internal/domain/subscriber/services"
	"skyrix/internal/logger"
	"skyrix/internal/validation" // Added import for validation
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
