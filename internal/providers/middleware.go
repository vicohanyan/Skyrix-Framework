package providers

import (
	"skyrix/internal/middleware"

	"github.com/google/wire"
)

type GlobalMiddleware struct {
	ManyRequests   *middleware.ManyRequestsMiddleware
	Recover        *middleware.RecoverMiddleware
	GzipDecompress *middleware.GzipDecompressMiddleware
}

var GlobalMiddlewareProviderSet = wire.NewSet(
	middleware.NewManyRequestsMiddleware,
	middleware.NewRecoverMiddleware,
	middleware.NewGzipDecompressMiddleware,

	wire.Struct(new(GlobalMiddleware), "*"),
)
