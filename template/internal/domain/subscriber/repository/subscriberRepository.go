package repository

import (
	"gitlab.com/skyrix-lib/framework/engine"
	"gitlab.com/skyrix-lib/framework/logging"
	"strings"
)

type SubscriberRepository struct {
	DB           *engine.Database
	DbMainSchema string
	logger       logger.Interface
}

func NewSubscriberRepository(db *engine.Database, dbMainSchema string, logger logger.Interface) *SubscriberRepository {
	return &SubscriberRepository{
		DB:           db,
		DbMainSchema: dbMainSchema,
		logger:       logger,
	}
}

func normLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
