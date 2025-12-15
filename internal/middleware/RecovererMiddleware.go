package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"

	"skyrix/internal/logger"
)

type RecoverMiddleware struct {
	log logger.Interface
}

func NewRecoverMiddleware(l logger.Interface) *RecoverMiddleware {
	return &RecoverMiddleware{log: l}
}

func (m *RecoverMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				if m.log != nil {
					m.log.Error("panic recovered",
						"method", r.Method,
						"url", r.URL.String(),
						"remote", r.RemoteAddr,
						"ua", r.UserAgent(),
						"panic", rec,
						"stack", string(debug.Stack()),
					)
				}

				if !strings.EqualFold(r.Header.Get("Connection"), "Upgrade") {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}
