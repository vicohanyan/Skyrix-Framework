package middleware

import "net/http"

// ManyRequestsMiddleware TODO: Add limits, bans, count store (Redis), keys (IP/schemaResolver/ua/uuid).
type ManyRequestsMiddleware struct {
}

func NewManyRequestsMiddleware() *ManyRequestsMiddleware {
	return &ManyRequestsMiddleware{}
}

func (m *ManyRequestsMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
