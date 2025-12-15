package services

import (
	"skyrix/internal/domain/subscriber/repository"
	"skyrix/internal/logger"
)

type SubscriberService struct {
	SubscriberRepository *repository.SubscriberRepository
	Logger               logger.Interface
}

func NewSubscriberService(
	subscriberRepository *repository.SubscriberRepository,
	logger logger.Interface,
) *SubscriberService {
	return &SubscriberService{
		SubscriberRepository: subscriberRepository,
		Logger:               logger,
	}
}

const (
	bucket     = "core"
	collection = "list"
)
