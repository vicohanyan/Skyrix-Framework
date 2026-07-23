package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type MainSchema string

type Options struct {
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	MainSchema string
}

func InitPostgres(cfg Options) (DB *gorm.DB, err error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable search_path=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.MainSchema)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return DB, nil
}
