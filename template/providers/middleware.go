package providers

import (
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
