package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// helpAndStandaloneArgs never need the application container.
var helpAndStandaloneArgs = [][]string{
	nil,
	{"--help"},
	{"help", "db"},
	{"db", "--help"},
	{"db", "automigrate", "--help"},
	{"db", "seed", "--help"},
	{"db", "migrate", "--help"},
	{"db", "migrate:status", "--help"},
	{"db", "migrate:diff", "--help"},
	{"jobs:run", "--help"},
	{"hello", "--help"},
	{"make", "--help"},
	{"make", "domain", "--help"},
	{"completion", "bash"},
	{"make", "domain", "Payment"},
}

// TestHelpAndStandaloneDoNotConnect runs help and standalone commands with
// PostgreSQL and Redis pointing at listeners that count connections.
func TestHelpAndStandaloneDoNotConnect(t *testing.T) {
	database, redis := countingListener(t), countingListener(t)
	setConsoleEnv(t, database.addr, redis.addr)
	chdirToProject(t)

	for _, args := range helpAndStandaloneArgs {
		if err := run(context.Background(), args, &strings.Builder{}, &strings.Builder{}); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
	}
	if _, err := os.Stat(filepath.Join("internal", "domain", "payment", "provider.go")); err != nil {
		t.Fatalf("make domain did not run: %v", err)
	}
	if n := database.count.Load(); n != 0 {
		t.Fatalf("database connections = %d, want 0", n)
	}
	if n := redis.count.Load(); n != 0 {
		t.Fatalf("redis connections = %d, want 0", n)
	}
}

// TestHelpAndStandaloneDoNotBuildContainer uses a configuration with which
// the application container cannot be built: the commands still succeed.
func TestHelpAndStandaloneDoNotBuildContainer(t *testing.T) {
	setConsoleEnv(t, "127.0.0.1:1", "127.0.0.1:1")
	t.Setenv("DB_PORT", "not-a-port")
	chdirToProject(t)

	if err := run(context.Background(), []string{"hello"}, &strings.Builder{}, &strings.Builder{}); err == nil {
		t.Fatal("hello with an invalid configuration error = nil; the test needs a container that cannot be built")
	}
	for _, args := range helpAndStandaloneArgs[:len(helpAndStandaloneArgs)-1] {
		var stdout strings.Builder
		if err := run(context.Background(), args, &stdout, &strings.Builder{}); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		if stdout.Len() == 0 {
			t.Fatalf("%q printed nothing", args)
		}
	}
}

// TestDBMigrateConnectsOnlyToDatabase checks that db migrate opens only the
// PostgreSQL connection.
func TestDBMigrateConnectsOnlyToDatabase(t *testing.T) {
	database, redis := countingListener(t), countingListener(t)
	setConsoleEnv(t, database.addr, redis.addr)

	if err := run(context.Background(), []string{"db", "migrate"}, &strings.Builder{}, &strings.Builder{}); err == nil {
		t.Fatal("db migrate against a fake server error = nil")
	}
	if database.count.Load() == 0 {
		t.Fatal("db migrate did not connect to the database")
	}
	if n := redis.count.Load(); n != 0 {
		t.Fatalf("redis connections = %d, want 0", n)
	}
}

type listener struct {
	addr  string
	count atomic.Int64
}

// countingListener accepts and immediately closes connections, counting them.
func countingListener(t *testing.T) *listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	l := &listener{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			l.count.Add(1)
			_ = conn.Close()
		}
	}()
	return l
}

func setConsoleEnv(t *testing.T, databaseAddr, redisAddr string) {
	t.Helper()
	dbHost, dbPort := splitAddr(t, databaseAddr)
	redisHost, redisPort := splitAddr(t, redisAddr)
	for key, value := range map[string]string{
		"APP_ENV": "missing-environment", "LOG_OUTPUT": "stdout", "LOG_LEVEL": "error",
		"DB_HOST": dbHost, "DB_PORT": dbPort, "DB_USER": "app", "DB_PASS": "app", "DB_NAME": "app",
		"REDIS_HOST": redisHost, "REDIS_PORT": redisPort, "REDIS_KEY_PREFIX": "app",
	} {
		t.Setenv(key, value)
	}
}

func splitAddr(t *testing.T, addr string) (string, string) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatal(err)
	}
	return host, port
}

// chdirToProject moves into an empty Go project, where make writes its files.
func chdirToProject(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}
