package database

import (
	"context"
	"fmt"

	"gitlab.com/skyrix-lib/framework/engine"
	tenantEntity "gitlab.com/skyrix-lib/framework/extensions/tenant/entity"
	"gitlab.com/skyrix-lib/framework/logging"
	exampleEntity "gitlab.com/skyrix-lib/template/internal/domain/example/entity"
)

type DBMigrator struct {
	DB     *engine.Database
	Logger logger.Interface
	Seeder *DBSeeder
}

func NewDBMigrator(db *engine.Database, log logger.Interface, seeder *DBSeeder) *DBMigrator {
	return &DBMigrator{
		DB:     db,
		Logger: log,
		Seeder: seeder,
	}
}

func (m *DBMigrator) Migrate(ctx context.Context) error {
	db := m.DB.WithContext(ctx)

	models := []any{
		&exampleEntity.Task{},
		&tenantEntity.Tenant{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate entities: %w", err)
	}

	m.Logger.Info("db automigrate completed", "models", len(models))

	if err := m.Seeder.Seed(ctx); err != nil {
		return fmt.Errorf("seed dictionaries: %w", err)
	}

	return nil
}
