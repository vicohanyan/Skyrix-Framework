package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSystemMetricsRoute(t *testing.T) {
	r := chi.NewRouter()
	mountSystemRoutes(r)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)

	r.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", contentType)
	}
	if !strings.Contains(recorder.Body.String(), "go_goroutines") {
		t.Fatalf("response does not contain runtime metrics: %s", recorder.Body.String())
	}
}
