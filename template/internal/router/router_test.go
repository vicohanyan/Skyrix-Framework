package router

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	authmiddleware "gitlab.com/skyrix-lib/auth/middleware"
	"gitlab.com/skyrix-lib/auth/security"
	authservice "gitlab.com/skyrix-lib/auth/service"
	frameworkmiddleware "gitlab.com/skyrix-lib/framework/httpserver/middleware"
	"gitlab.com/skyrix-lib/framework/validation"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/internal/domain/example/dto"
	"gitlab.com/skyrix-lib/template/internal/handlers"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/golang-jwt/jwt/v5"
)

// TestRoutesRequireToken checks that the example routes require a valid
// bearer token, the roles of JWT_ALLOWED_ROLES apply, and the system routes
// stay open.
func TestRoutesRequireToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	token := func(role string) string {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, security.CustomClaims{
			UserID:           42,
			Role:             security.Role(role),
			RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		}).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + signed
	}

	tests := []struct {
		name          string
		allowedRoles  []string
		path          string
		authorization string
		want          int
	}{
		{name: "health is open", path: "/health", want: http.StatusOK},
		{name: "metrics is open", path: "/metrics", want: http.StatusOK},
		{name: "example without token", path: "/api/v1/examples/tasks/", want: http.StatusUnauthorized},
		{name: "example with invalid token", path: "/api/v1/examples/tasks/", authorization: "Bearer invalid", want: http.StatusUnauthorized},
		{name: "example with token, empty role list", path: "/api/v1/examples/tasks/", authorization: token("customer"), want: http.StatusOK},
		{name: "example with allowed role", allowedRoles: []string{"admin"}, path: "/api/v1/examples/tasks/", authorization: token("admin"), want: http.StatusOK},
		{name: "example with other role", allowedRoles: []string{"admin"}, path: "/api/v1/examples/tasks/", authorization: token("customer"), want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jwtConfig := &config.JWT{Algorithm: "RS256", AllowedRoles: test.allowedRoles}
			path := filepath.Join(t.TempDir(), "public.pem")
			if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey)}), 0600); err != nil {
				t.Fatal(err)
			}
			jwtConfig.PublicKeyPath = path
			jwtService, err := authservice.NewValidator(noopLogger{}, jwtConfig)
			if err != nil {
				t.Fatal(err)
			}
			auth := authmiddleware.NewAuthMiddleware(jwtService, noopLogger{}, authmiddleware.OptionsFromConfig(jwtConfig))
			handler := InitRouter(
				&config.HttpServer{Timeout: 5 * time.Second},
				&providers.GlobalMiddleware{
					Recover:        frameworkmiddleware.NewRecoverMiddleware(noopLogger{}),
					GzipDecompress: frameworkmiddleware.NewGzipDecompressMiddleware(noopLogger{}),
				},
				auth,
				NewNoopTenantMiddleware(),
				&providers.Handlers{ExampleTask: handlers.NewExampleTaskHandler(noopLogger{}, validation.NewValidator(), stubTaskService{})},
				nil,
			)

			request := httptest.NewRequest(http.MethodGet, test.path, http.NoBody)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("GET %s status = %d, want %d; body: %s", test.path, recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

type stubTaskService struct{}

func (stubTaskService) Create(context.Context, dto.CreateTaskInput) (dto.TaskResult, error) {
	return dto.TaskResult{}, nil
}
func (stubTaskService) Get(context.Context, uint64) (dto.TaskResult, error) {
	return dto.TaskResult{}, nil
}
func (stubTaskService) List(context.Context, int) ([]dto.TaskResult, error) {
	return []dto.TaskResult{}, nil
}
func (stubTaskService) Complete(context.Context, uint64) (dto.TaskResult, error) {
	return dto.TaskResult{}, nil
}

type noopLogger struct{}

func (noopLogger) Error(string, ...interface{}) {}
func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Debug(string, ...interface{}) {}
func (noopLogger) Warn(string, ...interface{})  {}

func (noopLogger) With(...interface{}) logger.Interface { return noopLogger{} }
