package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type checkFunc func(context.Context) error

func (f checkFunc) Check(ctx context.Context) error { return f(ctx) }

func serveHealth(t *testing.T, checks HealthChecks) (int, healthResponse) {
	t.Helper()
	r := chi.NewRouter()
	RegisterSystemRoutes(r, checks)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))
	var body healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return recorder.Code, body
}

func TestHealthOK(t *testing.T) {
	ok := checkFunc(func(context.Context) error { return nil })
	code, body := serveHealth(t, HealthChecks{{Name: "postgres", Checker: ok}, {Name: "redis", Checker: ok}})
	if code != http.StatusOK || body.Status != "ok" || body.Checks["postgres"] != "ok" || body.Checks["redis"] != "ok" {
		t.Fatalf("health = %d %+v", code, body)
	}
}

func TestHealthReportsFailedDependency(t *testing.T) {
	code, body := serveHealth(t, HealthChecks{
		{Name: "postgres", Checker: checkFunc(func(context.Context) error { return errors.New("connection refused to 10.0.0.1") })},
		{Name: "redis", Checker: checkFunc(func(context.Context) error { return nil })},
		{Name: "broken", Checker: nil},
	})
	if code != http.StatusServiceUnavailable || body.Status != "unavailable" {
		t.Fatalf("health = %d %+v, want 503", code, body)
	}
	if body.Checks["postgres"] != "unavailable" || body.Checks["redis"] != "ok" || body.Checks["broken"] != "unavailable" {
		t.Fatalf("checks = %+v", body.Checks)
	}
}

func TestHealthTimesOutHangingCheck(t *testing.T) {
	hanging := checkFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	recorder := httptest.NewRecorder()
	start := time.Now()
	HealthHandler(HealthChecks{{Name: "slow", Checker: hanging}}, 50*time.Millisecond).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("health took %v, want the timeout to apply", elapsed)
	}
}

func TestHealthWithoutChecks(t *testing.T) {
	code, body := serveHealth(t, nil)
	if code != http.StatusOK || body.Status != "ok" || len(body.Checks) != 0 {
		t.Fatalf("health = %d %+v", code, body)
	}
}
