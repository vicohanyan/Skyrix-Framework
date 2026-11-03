package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/internal/depcheck"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestValidate(t *testing.T) {
	err := postgres.Config{Port: 70000}.Validate()
	for _, want := range []string{"host", "port", "database"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() = %v, want %s error", err, want)
		}
	}
	if err := (postgres.Config{Host: "db", Port: 5432, Database: "app"}).Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestValidatePort(t *testing.T) {
	for port, valid := range map[int]bool{-1: false, 0: false, 1: true, 5432: true, 65535: true, 65536: false} {
		err := postgres.Config{Host: "db", Port: port, Database: "app"}.Validate()
		if valid != (err == nil) {
			t.Errorf("Validate(port %d) = %v, want valid %v", port, err, valid)
		}
		if err != nil && !strings.Contains(err.Error(), "1-65535") {
			t.Errorf("Validate(port %d) = %v, want range in message", port, err)
		}
	}
}

func TestValidateSSLMode(t *testing.T) {
	for _, mode := range []string{"", "disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		cfg := postgres.Config{Host: "db", Port: 5432, Database: "app", SSLMode: mode}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate(sslmode %q) = %v", mode, err)
		}
		// pgx accepts every mode without contacting the server.
		if _, err := cfg.ConnConfig(); err != nil {
			t.Errorf("ConnConfig(sslmode %q) = %v", mode, err)
		}
	}
	for _, mode := range []string{"bogus-secret", "REQUIRE", " disable", "true"} {
		err := postgres.Config{Host: "db", Port: 5432, Database: "app", SSLMode: mode, Password: "pw-secret"}.Validate()
		if err == nil || !strings.Contains(err.Error(), "sslmode") {
			t.Fatalf("Validate(sslmode %q) = %v, want sslmode error", mode, err)
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "bogus") {
			t.Fatalf("Validate(sslmode %q) leaked a value: %v", mode, err)
		}
	}
}

// TestConnConfigTLS checks the TLS settings pgx derives from SSLMode by
// parsing only; no connection, and no real TLS handshake, is made.
func TestConnConfigTLS(t *testing.T) {
	cfg := postgres.Config{Host: "db", Port: 5432, Database: "app"}
	for mode, wantTLS := range map[string]bool{"": false, "disable": false, "require": true, "verify-ca": true, "verify-full": true} {
		cfg.SSLMode = mode
		parsed, err := cfg.ConnConfig()
		if err != nil {
			t.Fatalf("ConnConfig(sslmode %q) = %v", mode, err)
		}
		if gotTLS := parsed.TLSConfig != nil; gotTLS != wantTLS {
			t.Errorf("sslmode %q: TLS = %v, want %v", mode, gotTLS, wantTLS)
		}
		if mode == "" || mode == "disable" {
			if len(parsed.Fallbacks) != 0 {
				t.Errorf("sslmode %q: fallbacks = %d, want none", mode, len(parsed.Fallbacks))
			}
		}
	}
	cfg.SSLMode = "verify-full"
	parsed, err := cfg.ConnConfig()
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TLSConfig.InsecureSkipVerify || parsed.TLSConfig.ServerName != "db" {
		t.Fatalf("verify-full TLS config = %+v", parsed.TLSConfig)
	}
}

const leakyPassword = `Secr' et\Tail "quoted"`

// leakFragments are parts of leakyPassword that must not appear in any
// error: pgx's own masking used to leave "et\Tail" visible.
var leakFragments = []string{"Secr", "Tail", "quoted", `et\`}

func assertNoLeak(t *testing.T, text string) {
	t.Helper()
	for _, fragment := range leakFragments {
		if strings.Contains(text, fragment) {
			t.Fatalf("secret fragment %q leaked: %s", fragment, text)
		}
	}
	if strings.Contains(text, "host=") || strings.Contains(text, "password") {
		t.Fatalf("connection string leaked: %s", text)
	}
}

// TestParseErrorsAreSafe triggers pgx parse errors after Validate passes,
// through libpq environment variables, and checks both open paths.
func TestParseErrorsAreSafe(t *testing.T) {
	cfg := postgres.Config{Host: "db", Port: 5432, User: "app", Password: leakyPassword, Database: "app", SSLMode: "verify-full"}
	cases := map[string]struct{ env, value, reason string }{
		"tls":     {"PGSSLROOTCERT", filepath.Join(t.TempDir(), "missing.pem"), "failed to configure TLS: unable to read CA file"},
		"timeout": {"PGCONNECT_TIMEOUT", "not-a-number", "invalid connect_timeout"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(tc.env, tc.value)
			_, connErr := cfg.ConnConfig()
			_, openErr := postgres.OpenSQL(context.Background(), cfg)
			for _, err := range []error{connErr, openErr} {
				var parseErr *postgres.ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("error = %v, want *postgres.ParseError", err)
				}
				if !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("error = %v, want reason %q", err, tc.reason)
				}
				var pgxErr *pgconn.ParseConfigError
				if errors.As(err, &pgxErr) || errors.Unwrap(err) != nil {
					t.Fatalf("error exposes the pgx error: %#v", err)
				}
				for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
					assertNoLeak(t, fmt.Sprintf(format, err))
				}
			}
		})
	}
}

func TestDSNRoundTripsSpecialCharacters(t *testing.T) {
	for _, password := range []string{`p a'ss\`, `'`, `\`, `\'`, `a b  c`, `"x" 'y'`, `p@ss:w/rd?x=1&y=%20`, leakyPassword, ``} {
		cfg := postgres.Config{Host: "db", Port: 5432, User: `us'er \`, Password: password, Database: `d b`, MainSchema: `s'c`}
		parsed, err := cfg.ConnConfig()
		if err != nil {
			t.Fatalf("ConnConfig(%q) = %v", password, err)
		}
		if parsed.Password != password || parsed.User != `us'er \` || parsed.Database != `d b` || parsed.RuntimeParams["search_path"] != `s'c` {
			t.Fatalf("password %q parsed as %+v", password, parsed.Config)
		}
	}
}

func TestDSN(t *testing.T) {
	cfg := postgres.Config{
		Host: "db", Port: 5432, User: "app", Password: `p a'ss\`, Database: "orders", MainSchema: "core",
	}
	want := `host='db' port=5432 user='app' password='p a\'ss\\' dbname='orders' sslmode='disable' search_path='core'`
	if got := cfg.DSN(); got != want {
		t.Fatalf("DSN() = %s\nwant    %s", got, want)
	}
	parsed, err := pgconn.ParseConfig(cfg.DSN())
	if err != nil {
		t.Fatalf("pgconn.ParseConfig(DSN()) error = %v", err)
	}
	if parsed.Password != `p a'ss\` || parsed.Database != "orders" || parsed.RuntimeParams["search_path"] != "core" {
		t.Fatalf("parsed DSN = %+v", parsed)
	}

	cfg.SSLMode, cfg.MainSchema = "require", ""
	if got := cfg.DSN(); !strings.Contains(got, "sslmode='require'") || strings.Contains(got, "search_path") {
		t.Fatalf("DSN() = %s", got)
	}
}

func TestConfigurePool(t *testing.T) {
	db, err := sql.Open("pgx", postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none"}.DSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	postgres.Config{MaxOpenConns: 3}.ConfigurePool(db)
	if got := db.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", got)
	}
}

func TestComponentStartPingsAndStopCloses(t *testing.T) {
	db, err := sql.Open("pgx", postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none"}.DSN())
	if err != nil {
		t.Fatal(err)
	}
	component := postgres.NewComponent(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := component.Start(ctx); err == nil {
		t.Fatal("Start() = nil, want ping error")
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	if err := postgres.NewComponent(nil).Check(ctx); err == nil {
		t.Fatal("Check() with nil db = nil, want error")
	}
}

func TestOpenSQLRejectsInvalidConfig(t *testing.T) {
	if _, err := postgres.OpenSQL(context.Background(), postgres.Config{}); err == nil {
		t.Fatal("OpenSQL(empty config) error = nil")
	}
	// Port 0 used to reach pgx and return its partially masked DSN.
	_, err := postgres.OpenSQL(context.Background(), postgres.Config{Host: "db", Database: "app", Password: leakyPassword})
	if err == nil || !strings.Contains(err.Error(), "invalid port 0") {
		t.Fatalf("OpenSQL(port 0) error = %v", err)
	}
	assertNoLeak(t, fmt.Sprintf("%+v", err))
}

func TestOpenPoolDoesNotConnect(t *testing.T) {
	db, err := postgres.OpenPool(postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none", MaxOpenConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := db.Stats().MaxOpenConnections; got != 2 {
		t.Fatalf("MaxOpenConnections = %d, want 2", got)
	}
}

// TestAgainstRealPostgres runs the lifecycle against SKYRIX_TEST_POSTGRES_DSN.
func TestAgainstRealPostgres(t *testing.T) {
	dsn := os.Getenv("SKYRIX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SKYRIX_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := pgconn.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg := postgres.Config{
		Host: parsed.Host, Port: int(parsed.Port), User: parsed.User, Password: parsed.Password,
		Database: parsed.Database, SSLMode: "disable",
	}
	ctx := context.Background()

	sqlDB, err := postgres.OpenSQL(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenSQL() error = %v", err)
	}
	checkLifecycle(t, postgres.NewComponent(sqlDB))
}

func checkLifecycle(t *testing.T, component *postgres.Component) {
	t.Helper()
	ctx := context.Background()
	if err := component.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := component.Check(ctx); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := component.Check(ctx); err == nil {
		t.Fatal("Check() after Stop = nil, want error")
	}
}

// TestPostgresImportsOnlyInfrastructure keeps the connection package free of
// GORM and of its consumers (gormdb, tenant, domains): it may import only the
// standard library, pgx (connection string parsing and the database/sql
// driver) and the kernel contracts.
func TestPostgresImportsOnlyInfrastructure(t *testing.T) {
	depcheck.CheckPackage(t, "gitlab.com/skyrix-lib/framework/engine/postgres", func(pkg depcheck.Package) bool {
		switch pkg.ImportPath {
		case "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v5/stdlib",
			"gitlab.com/skyrix-lib/framework/kernel":
			return true
		}
		return pkg.Standard
	})
}
