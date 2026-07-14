package providers

import (
	"skyrix/internal/middleware"

	"github.com/google/wire"
)

type GlobalMiddleware struct {
	ManyRequests   *middleware.ManyRequestsMiddleware
	Recover        *middleware.RecoverMiddleware
	GzipDecompress *middleware.GzipDecompressMiddleware
	Auth           *middleware.AuthMiddleware
}

var GlobalMiddlewareProviderSet = wire.NewSet(
	middleware.NewManyRequestsMiddleware,
	middleware.NewRecoverMiddleware,
	middleware.NewGzipDecompressMiddleware,
	middleware.NewAuthMiddleware,

	wire.Struct(new(GlobalMiddleware), "*"),
)
