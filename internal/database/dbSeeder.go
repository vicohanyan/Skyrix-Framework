package database

import (
	"context"
	"skyrix/internal/engine"
	"skyrix/internal/logger"
)

type DBSeeder struct {
	DB     *engine.Database
	Logger logger.Interface
}

func NewDBSeeder(db *engine.Database, log logger.Interface) *DBSeeder {
	return &DBSeeder{
		DB:     db,
		Logger: log,
	}
}

func (s *DBSeeder) Seed(ctx context.Context) error {

	s.Logger.Info("db dictionary seed completed")
	return nil
}
