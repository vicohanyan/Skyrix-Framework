package service

import (
	"slices"
	"testing"

	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
)

func TestConfigRoles(t *testing.T) {
	cfg := Config{AllowedRoles: []string{" customer ", "", "staff"}}
	want := []security.Role{"customer", "staff"}
	if got := cfg.Roles(); !slices.Equal(got, want) {
		t.Fatalf("Roles() = %v, want %v", got, want)
	}
	if got := (Config{}).Roles(); len(got) != 0 {
		t.Fatalf("Roles() of empty config = %v", got)
	}
}
