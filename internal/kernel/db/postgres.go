package db

import (
	"fmt"
	"skyrix/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type MainSchema string

func InitPostgres(cfg *config.Database) (DB *gorm.DB, err error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable search_path=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Pass, cfg.Name, cfg.MainSchema)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return DB, nil
}
