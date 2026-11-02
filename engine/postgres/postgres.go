// Package postgres connects an application to PostgreSQL through
// database/sql: configuration, opening the pgx connection pool, and the
// lifecycle component with the health check.
//
// The package does not use GORM and knows nothing about request schemas or
// domains: engine/postgres/gormdb opens a GORM connection from the same
// Config and adds schema switching.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
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

// OpenSQL opens a database/sql connection pool through the pgx driver and
// verifies the connection, for consumers such as database/migrate. The
// caller closes the pool.
func OpenSQL(ctx context.Context, cfg Config) (*sql.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, err
	}
	cfg.ConfigurePool(db)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ConfigurePool applies the pool settings of c to db; zero values keep the
// database/sql defaults. Packages that open the pool themselves (gormdb)
// use it to honour the same settings.
func (cfg Config) ConfigurePool(db *sql.DB) {
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
