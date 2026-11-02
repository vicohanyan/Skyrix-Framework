package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
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

// DSN returns the libpq connection string for the options.
func DSN(cfg Options) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable search_path=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.MainSchema)
}

func InitPostgres(cfg Options) (DB *gorm.DB, err error) {
	DB, err = gorm.Open(postgres.Open(DSN(cfg)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return DB, nil
}

// OpenSQL opens a database/sql connection pool through the pgx driver and
// verifies the connection.
func OpenSQL(ctx context.Context, cfg Options) (*sql.DB, error) {
	db, err := sql.Open("pgx", DSN(cfg))
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
