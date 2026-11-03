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
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib" // also registers the "pgx" database/sql driver
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
	// SSLMode is the libpq sslmode: disable, allow, prefer, require,
	// verify-ca or verify-full. Empty means "disable",
	// which turns TLS off; the other modes are chosen explicitly.
	SSLMode string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// sslModes are the accepted libpq sslmode values.
var sslModes = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// Validate reports configuration errors. The messages never contain the
// password or other connection string values.
func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Host) == "" {
		errs = append(errs, errors.New("postgres: host is required"))
	}
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("postgres: invalid port %d, want 1-65535", c.Port))
	}
	if strings.TrimSpace(c.Database) == "" {
		errs = append(errs, errors.New("postgres: database is required"))
	}
	if c.SSLMode != "" && !slices.Contains(sslModes, c.SSLMode) {
		// The value itself is not echoed: a misplaced secret must not leak.
		errs = append(errs, fmt.Errorf("postgres: invalid sslmode, want one of %s", strings.Join(sslModes, ", ")))
	}
	return errors.Join(errs...)
}

// ParseError reports connection settings that pgx cannot parse, for example
// an unreadable PGSSLROOTCERT. It carries only pgx's reason, never the
// connection string, and wraps no error, so neither formatting nor
// errors.As can reach the password. Match it with errors.As.
type ParseError struct {
	// Reason is pgx's description of the problem; empty if unknown.
	Reason string
}

func (e *ParseError) Error() string {
	if e.Reason == "" {
		return "postgres: invalid connection settings"
	}
	return "postgres: invalid connection settings: " + e.Reason
}

// ConnConfig validates c and parses its connection string. Parse errors are
// returned as *ParseError without the connection string. pgx also applies
// the libpq PG* environment variables that the connection string leaves
// unset (for example PGSSLROOTCERT).
func (c Config) ConnConfig() (*pgx.ConnConfig, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	parsed, err := pgx.ParseConfig(c.DSN())
	if err != nil {
		return nil, safeParseError(err)
	}
	return parsed, nil
}

// safeParseError rebuilds a pgx parse error without the connection string.
// pgx masks the password only on a best-effort basis, so its message is
// never returned as is: the reason is read from a copy whose ConnString is
// cleared. The wrapped cause is dropped except for TLS setup, whose causes
// are fixed texts and file errors.
func safeParseError(err error) error {
	var parseErr *pgconn.ParseConfigError
	if !errors.As(err, &parseErr) {
		return &ParseError{}
	}
	detached := *parseErr
	detached.ConnString = ""
	reason, ok := strings.CutPrefix(detached.Error(), "cannot parse ``: ")
	if !ok {
		return &ParseError{}
	}
	if cause := errors.Unwrap(parseErr); cause != nil {
		trimmed, ok := strings.CutSuffix(reason, " ("+cause.Error()+")")
		if !ok {
			return &ParseError{}
		}
		reason = trimmed
		if reason == "failed to configure TLS" {
			reason += ": " + cause.Error()
		}
	}
	return &ParseError{Reason: reason}
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

// OpenPool opens a database/sql connection pool through pgx without
// contacting the server. The connection string is parsed once, by
// ConnConfig, so its errors are safe. The caller closes the pool.
func OpenPool(cfg Config) (*sql.DB, error) {
	connConfig, err := cfg.ConnConfig()
	if err != nil {
		return nil, err
	}
	db := stdlib.OpenDB(*connConfig)
	cfg.ConfigurePool(db)
	return db, nil
}

// OpenSQL opens a database/sql connection pool through the pgx driver and
// verifies the connection, for consumers such as database/migrate. The
// caller closes the pool.
func OpenSQL(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := OpenPool(cfg)
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
