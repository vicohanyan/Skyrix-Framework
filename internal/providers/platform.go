package providers

import (
	"skyrix/internal/engine/tenantPackage"

	"github.com/google/wire"
)

var PlatformProviderSet = wire.NewSet(
	tenantPackage.ProviderSet,
	// auth.ProviderSet, // later
)
