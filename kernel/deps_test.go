package kernel_test

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/internal/depcheck"
)

// TestKernelImportsOnlyStandardLibrary keeps kernel and kernel/... free of
// dependencies outside the Go standard library, so the contracts can be
// imported without pulling in framework infrastructure.
func TestKernelImportsOnlyStandardLibrary(t *testing.T) {
	depcheck.Check(t, "gitlab.com/skyrix-lib/framework/kernel", depcheck.StandardLibraryOnly)
}
