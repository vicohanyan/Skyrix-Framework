package providers

import (
	"net/http"

	authmiddleware "gitlab.com/skyrix-lib/auth/middleware"
	frameworkmiddleware "gitlab.com/skyrix-lib/framework/httpserver/middleware"
	appmiddleware "gitlab.com/skyrix-lib/template/internal/middleware"

	"github.com/goforj/wire"
)

type GlobalMiddleware struct {
	ManyRequests   *appmiddleware.ManyRequestsMiddleware
	Recover        *frameworkmiddleware.RecoverMiddleware
	GzipDecompress *frameworkmiddleware.GzipDecompressMiddleware
}

var GlobalMiddlewareProviderSet = wire.NewSet(
	appmiddleware.NewManyRequestsMiddleware,
	frameworkmiddleware.NewRecoverMiddleware,
	frameworkmiddleware.NewGzipDecompressMiddleware,

	wire.Struct(new(GlobalMiddleware), "*"),
)

// APIMiddleware is the ordered middleware chain of the /api/v1 route group.
// The first element is the outermost: it runs first. The router applies the
// chain as given and knows nothing about its elements; /health and /metrics
// are outside the group.
type APIMiddleware []func(http.Handler) http.Handler

// ProvideAPIMiddleware is the default API chain: bearer-token authentication.
// To add middleware, replace this provider in the binary's Wire graph with
// one that returns the full chain in the wanted order, for example
// APIMiddleware{auth.Handle, tenant.Handle}.
func ProvideAPIMiddleware(auth *authmiddleware.AuthMiddleware) APIMiddleware {
	return APIMiddleware{auth.Handle}
}
