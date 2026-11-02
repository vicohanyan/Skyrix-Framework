package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"gitlab.com/skyrix-lib/framework/kernel"
)

// DefaultHealthTimeout bounds how long /health waits for all checks.
const DefaultHealthTimeout = 2 * time.Second

// HealthCheck is a named dependency checked by /health.
type HealthCheck struct {
	Name    string
	Checker kernel.HealthChecker
}

// HealthChecks are the dependencies of a binary checked by /health, such as
// the PostgreSQL and Redis components.
type HealthChecks []HealthCheck

const (
	healthOK          = "ok"
	healthUnavailable = "unavailable"
)

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// HealthHandler runs every check in parallel within timeout
// (DefaultHealthTimeout when not positive). It responds 200 when all checks
// pass and 503 otherwise, with the status of each check as JSON:
//
//	{"status":"unavailable","checks":{"postgres":"unavailable","redis":"ok"}}
//
// Check errors are not exposed in the response.
func HealthHandler(checks HealthChecks, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		timeout = DefaultHealthTimeout
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		response := healthResponse{Status: healthOK, Checks: make(map[string]string, len(checks))}
		var (
			mu sync.Mutex
			wg sync.WaitGroup
		)
		for _, check := range checks {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status := healthOK
				if check.Checker == nil || check.Checker.Check(ctx) != nil {
					status = healthUnavailable
				}
				mu.Lock()
				defer mu.Unlock()
				response.Checks[check.Name] = status
				if status != healthOK {
					response.Status = healthUnavailable
				}
			}()
		}
		wg.Wait()

		code := http.StatusOK
		if response.Status != healthOK {
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(response)
	})
}
