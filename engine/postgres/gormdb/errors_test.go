package gormdb_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"
	"gitlab.com/skyrix-lib/framework/logger"

	"github.com/jackc/pgx/v5/pgconn"
	gormlogger "gorm.io/gorm/logger"
)

const leakyPassword = `Secr' et\\Tail "quoted"`

func assertNoLeak(t *testing.T, text string) {
	t.Helper()
	for _, fragment := range []string{"Secr", "Tail", "quoted", "host=", "password"} {
		if strings.Contains(text, fragment) {
			t.Fatalf("secret fragment %q leaked: %s", fragment, text)
		}
	}
}

// recordingLogger records everything logged through logger.Interface.
type recordingLogger struct {
	mu    *sync.Mutex
	lines *[]string
}

func newRecordingLogger() recordingLogger {
	return recordingLogger{mu: &sync.Mutex{}, lines: &[]string{}}
}

func (l recordingLogger) record(level, msg string, kv ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.lines = append(*l.lines, fmt.Sprintf("%s %s %+v %#v", level, msg, kv, kv))
}

func (l recordingLogger) Error(msg string, kv ...interface{})  { l.record("error", msg, kv...) }
func (l recordingLogger) Info(msg string, kv ...interface{})   { l.record("info", msg, kv...) }
func (l recordingLogger) Debug(msg string, kv ...interface{})  { l.record("debug", msg, kv...) }
func (l recordingLogger) Warn(msg string, kv ...interface{})   { l.record("warn", msg, kv...) }
func (l recordingLogger) With(...interface{}) logger.Interface { return l }

func (l recordingLogger) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(*l.lines, "\n")
}

// recordingGORMLogger records GORM's internal logging.
type recordingGORMLogger struct{ recordingLogger }

func (l recordingGORMLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return l }
func (l recordingGORMLogger) Info(_ context.Context, msg string, data ...interface{}) {
	l.record("info", msg, data...)
}
func (l recordingGORMLogger) Warn(_ context.Context, msg string, data ...interface{}) {
	l.record("warn", msg, data...)
}
func (l recordingGORMLogger) Error(_ context.Context, msg string, data ...interface{}) {
	l.record("error", msg, data...)
}
func (l recordingGORMLogger) Trace(context.Context, time.Time, func() (string, int64), error) {}

// TestOpenErrorsAndLogsAreSafe covers the GORM path: errors of Open, the log
// line of Provide and GORM's own default logger, which gorm.Open uses when
// initialisation fails. Not parallel: it replaces gormlogger.Default.
func TestOpenErrorsAndLogsAreSafe(t *testing.T) {
	gormLog := recordingGORMLogger{newRecordingLogger()}
	previous := gormlogger.Default
	gormlogger.Default = gormLog
	t.Cleanup(func() { gormlogger.Default = previous })

	base := postgres.Config{Host: "db", Port: 5432, User: "app", Password: leakyPassword, Database: "app"}
	cases := map[string]struct {
		cfg   postgres.Config
		env   string
		value string
		parse bool
	}{
		"port 0":       {cfg: withPort(base, 0)},
		"bad sslmode":  {cfg: withSSLMode(base, "Secret-mode")},
		"tls rootcert": {cfg: withSSLMode(base, "verify-full"), env: "PGSSLROOTCERT", value: filepath.Join(t.TempDir(), "missing.pem"), parse: true},
		"timeout env":  {cfg: base, env: "PGCONNECT_TIMEOUT", value: "not-a-number", parse: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(tc.env, tc.value)
			}
			_, openErr := gormdb.Open(tc.cfg)
			log := newRecordingLogger()
			_, _, provideErr := gormdb.Provide(tc.cfg, log)
			for _, err := range []error{openErr, provideErr} {
				if err == nil {
					t.Fatal("error = nil")
				}
				var parseErr *postgres.ParseError
				if tc.parse != errors.As(err, &parseErr) {
					t.Fatalf("error = %v, want *postgres.ParseError %v", err, tc.parse)
				}
				var pgxErr *pgconn.ParseConfigError
				if errors.As(err, &pgxErr) {
					t.Fatalf("error exposes the pgx error: %#v", err)
				}
				for _, format := range []string{"%v", "%+v", "%#v"} {
					assertNoLeak(t, fmt.Sprintf(format, err))
				}
			}
			if !strings.Contains(log.text(), "Unable to initialize postgres database") {
				t.Fatalf("Provide did not log the failure: %q", log.text())
			}
			assertNoLeak(t, log.text())
		})
	}
	assertNoLeak(t, gormLog.text())

	// A valid configuration opens lazily; GORM logs nothing about it.
	db, err := gormdb.Open(base)
	if err != nil {
		t.Fatalf("Open(valid) error = %v", err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	assertNoLeak(t, gormLog.text())
}

func withPort(cfg postgres.Config, port int) postgres.Config {
	cfg.Port = port
	return cfg
}

func withSSLMode(cfg postgres.Config, mode string) postgres.Config {
	cfg.SSLMode = mode
	return cfg
}
