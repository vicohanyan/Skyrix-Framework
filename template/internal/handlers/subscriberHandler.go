package handlers

import (
	httphandler "gitlab.com/skyrix-lib/framework/httpserver/handler"
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/framework/validation" // Added import for validation
	"gitlab.com/skyrix-lib/template/internal/domain/subscriber/services"
)

type SubscriberHandler struct {
	*httphandler.BaseHandler
	SubscriberService *services.SubscriberService
}

func NewSubscriberHandler(logger logger.Interface, subscriberService *services.SubscriberService, validator *validation.Validator) *SubscriberHandler {
	return &SubscriberHandler{
		BaseHandler:       &httphandler.BaseHandler{HandlerName: "SubscriberHandler", Logger: logger, Validator: validator}, // Pass validator here
		SubscriberService: subscriberService,
	}
}
