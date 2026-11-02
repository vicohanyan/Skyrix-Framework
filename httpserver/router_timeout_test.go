package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A zero timeout disables the timeout middleware: the request context stays
// usable and the handler responds normally.
func TestNewRouterZeroTimeoutServesRequest(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		r := NewRouter(RouterOptions{Timeout: timeout})
		var ctxErr error
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			ctxErr = req.Context().Err()
			if _, ok := req.Context().Deadline(); ok {
				t.Errorf("Timeout %v: request context has a deadline", timeout)
			}
			w.WriteHeader(http.StatusNoContent)
		})

		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

		if ctxErr != nil {
			t.Fatalf("Timeout %v: request context error = %v, want nil", timeout, ctxErr)
		}
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("Timeout %v: status = %d, want %d", timeout, recorder.Code, http.StatusNoContent)
		}
	}
}

// A positive timeout cancels the context of a slow handler and answers 504.
func TestNewRouterTimeoutCancelsSlowHandler(t *testing.T) {
	r := NewRouter(RouterOptions{Timeout: 20 * time.Millisecond})
	var ctxErr error
	r.Get("/slow", func(_ http.ResponseWriter, req *http.Request) {
		select {
		case <-req.Context().Done():
			ctxErr = req.Context().Err()
		case <-time.After(2 * time.Second):
		}
	})

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/slow", http.NoBody))

	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("request context error = %v, want %v", ctxErr, context.DeadlineExceeded)
	}
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusGatewayTimeout)
	}
}
