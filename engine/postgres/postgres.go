// Package postgres connects an application to PostgreSQL: configuration,
// opening a GORM or database/sql connection pool, and the lifecycle
// component with the health check.
//
// The package exposes *gorm.DB and *sql.DB directly. It knows nothing about
// tenants, schemas or domains: engine.Database adds schema switching on top
// of the *gorm.DB it provides.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.com/skyrix-lib/framework/logger"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config describes a PostgreSQL connection. Zero pool values keep the
// database/sql defaults.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	// MainSchema is the search_path of new connections.
	MainSchema string
	// SSLMode is the libpq sslmode; "disable" when empty.
	SSLMode string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Validate reports configuration errors.
func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Host) == "" {
		errs = append(errs, errors.New("postgres: host is required"))
	}
	if c.Port < 0 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("postgres: invalid port %d", c.Port))
	}
	if strings.TrimSpace(c.Database) == "" {
		errs = append(errs, errors.New("postgres: database is required"))
	}
	return errors.Join(errs...)
}

// DSN returns the libpq keyword/value connection string. Values are quoted,
// so passwords may contain spaces and quotes.
func (c Config) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	parts := []string{
		"host=" + quote(c.Host),
		fmt.Sprintf("port=%d", c.Port),
		"user=" + quote(c.User),
		"password=" + quote(c.Password),
		"dbname=" + quote(c.Database),
		"sslmode=" + quote(sslMode),
	}
	if c.MainSchema != "" {
		parts = append(parts, "search_path="+quote(c.MainSchema))
	}
	return strings.Join(parts, " ")
}

// quote quotes a libpq connection string value.
func quote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	return "'" + value + "'"
}

// Open validates the configuration and opens a GORM connection pool. It does
// not contact the server: the Component pings it on Start. The caller closes
// the pool.
func Open(cfg Config) (*gorm.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	db, err := gorm.Open(gormpostgres.Open(cfg.DSN()), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	configurePool(sqlDB, cfg)
	return db, nil
}

// OpenSQL opens a database/sql connection pool through the pgx driver without
// GORM and verifies the connection, for consumers such as database/migrate.
// The caller closes the pool.
func OpenSQL(ctx context.Context, cfg Config) (*sql.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, err
	}
	configurePool(db, cfg)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Provide opens the GORM pool for Wire and returns a cleanup that closes it.
// Add NewComponent to the binary's runtime so the connection is verified on
// start.
func Provide(cfg Config, log logger.Interface) (*gorm.DB, func(), error) {
	db, err := Open(cfg)
	if err != nil {
		log.Error("Unable to initialize postgres database", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			log.Info("Closing postgres database connection")
			_ = sqlDB.Close()
		}
	}
	return db, cleanup, nil
}

func configurePool(db *sql.DB, cfg Config) {
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}
