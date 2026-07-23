package services

import (
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/domain/subscriber/repository"
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
