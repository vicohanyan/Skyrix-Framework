package handlers

import (
	"encoding/json"
	"net/http"
	"skyrix/internal/kernel/db/scope"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func statusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return ErrCodeValidation
	case http.StatusUnauthorized:
		return ErrCodeAuth
	case http.StatusForbidden:
		return ErrCodeForbidden
	case http.StatusNotFound:
		return ErrCodeNotFound
	case http.StatusConflict:
		return ErrCodeConflict
	default:
		return ErrCodeInternal
	}
}

// writeStructuredError centralizes logging + JSON response.
// NOTE: kept as a free function to keep BaseHandler small.
func writeStructuredError(
	log any, // logger.Interface (kept as any to decouple file from logger synchronization cycles if needed)
	w http.ResponseWriter,
	err error,
	userMessage string,
	status int,
	handlerName string,
	action string,
	httpMethod string,
	rOpt ...*http.Request,
) {
	var r *http.Request
	if len(rOpt) > 0 {
		r = rOpt[0]
	}

	// Log (best-effort, only if logger present)
	if lg, ok := log.(interface {
		Error(msg string, kv ...any)
	}); ok {
		var reqID, url, remote, tenant any
		if r != nil {
			reqID = chimw.GetReqID(r.Context())
			url = r.URL.String()
			remote = r.RemoteAddr
			tenant = scope.TenantFrom(r.Context())
		}
		lg.Error("handler error",
			"handler", handlerName,
			"action", action,
			"status", status,
			"code", statusToCode(status),
			"message", userMessage,
			"error", err,
			"http_method", httpMethod,
			"url", url,
			"remote", remote,
			"request_id", reqID,
			"tenant", tenant,
		)
	}

	// JSON response
	var reqID string
	if r != nil {
		reqID = chimw.GetReqID(r.Context())
	}
	payload := ErrorPayload{
		Error: ErrorBody{
			Code:      statusToCode(status),
			Message:   userMessage,
			RequestID: reqID,
		},
	}
	// Use a tiny local writer to avoid importing BaseHandler here.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
