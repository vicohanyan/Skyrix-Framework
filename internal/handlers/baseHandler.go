package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	"skyrix/internal/logger"
	"skyrix/internal/validation"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ---- Stable codes
const (
	ErrCodeValidation = "VALIDATION_FAILED"
	ErrCodeAuth       = "UNAUTHORIZED"
	ErrCodeForbidden  = "FORBIDDEN"
	ErrCodeNotFound   = "NOT_FOUND"
	ErrCodeConflict   = "CONFLICT"
	ErrCodeInternal   = "INTERNAL_ERROR"
)

type ErrorPayload struct {
	Error ErrorBody `json:"error"`
}
type ErrorBody struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Details   []FieldError `json:"details,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type BaseHandler struct {
	HandlerName string // set via constructor or WithAutoName
	Logger      logger.Interface
	Validator   *validation.Validator
}

// ToDo make interface for handlers
//type CRUDHandler interface {
//	List(w http.ResponseWriter, r *http.Request)   // GET /items
//	Get(w http.ResponseWriter, r *http.Request)    // GET /items/{id}
//	Create(w http.ResponseWriter, r *http.Request) // POST /items
//	Update(w http.ResponseWriter, r *http.Request) // PUT/PATCH /items/{id}
//	Delete(w http.ResponseWriter, r *http.Request) // DELETE /items/{id}
//}

// WithAutoName sets HandlerName from the concrete handler type.
func (b *BaseHandler) WithAutoName(h any) *BaseHandler {
	b.HandlerName = typeName(h)
	return b
}

func typeName(h any) string {
	if h == nil {
		return "unknown_handler"
	}
	t := reflect.Indirect(reflect.ValueOf(h)).Type()
	if t == nil {
		return "unknown_handler"
	}
	return t.Name()
}

func (b *BaseHandler) WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// HandleError writes a structured JSON error and logs it.
// It auto-infers action (caller func), method (r.Method), handlerName (b.HandlerName).
// Pass opt to override any auto-detected fields; opt can be nil.
func (b *BaseHandler) HandleError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
	userMessage string,
	status int,
) {
	// Infer action (function name that called HandleError)
	action := callerFunc() // default

	// Infer method
	method := ""
	if r != nil && r.Method != "" {
		method = r.Method
	}

	// Request meta (best-effort)
	var reqID, url, remote any
	if r != nil {
		reqID = chimw.GetReqID(r.Context())
		url = r.URL.String()
		remote = r.RemoteAddr
	}

	// Log (structured), keep it resilient to nil logger
	if b.Logger != nil && err != nil {
		b.Logger.Error("handler error",
			"handler", b.HandlerName,
			"action", action,
			"status", status,
			"code", statusToCode(status),
			"message", userMessage,
			"error", err,
			"http_method", method,
			"url", url,
			"remote", remote,
			"request_id", reqID,
		)
	}

	// Client-facing JSON payload
	payload := ErrorPayload{
		Error: ErrorBody{
			Code:      statusToCode(status),
			Message:   userMessage,
			RequestID: toString(reqID),
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// callerFunc returns the short name of the function that called HandleError.
func callerFunc() string {
	// 0 Callers, 1 callerFunc, 2 HandleError, 3 <YourHandlerMethod>
	const skip = 3
	var pcs [1]uintptr
	if runtime.Callers(skip, pcs[:]) < 1 {
		return "unknown_action"
	}
	fn := runtime.FuncForPC(pcs[0])
	if fn == nil {
		return "unknown_action"
	}
	full := fn.Name()
	// Trim package path and receiver, keep only method
	if i := strings.LastIndex(full, "."); i >= 0 && i+1 < len(full) {
		base := full[i+1:]
		if j := strings.LastIndex(base, ")"); j >= 0 && j+1 < len(base) && base[j+1] == '.' {
			return base[j+2:]
		}
		return base
	}
	return full
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func WriteGzipJSON(w http.ResponseWriter, gz []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gz)
}
