package config

import (
	"fmt"
	sections "gitlab.com/skyrix-lib/framework/config"
	"strings"
	"testing"
)

type driverOptions struct {
	Enabled   bool   `yaml:"enabled"`
	Count     int    `yaml:"count"`
	Password  string `yaml:"password" secret:"true"`
	validated int
}

func (o *driverOptions) Validate() error {
	o.validated++
	if o.Count < 0 {
		return fmt.Errorf("sensitive-password")
	}
	return nil
}

func TestDriverSectionsUseSharedLoader(t *testing.T) {
	cfg := &Config{source: []byte("platform:\n  selected:\n    enabled: false\n    count: 2\n    password: yaml-secret\n  excluded:\n    count: invalid\n")}
	t.Setenv("PLATFORM_SELECTED_COUNT", "3")
	t.Setenv("PLATFORM_EXCLUDED_COUNT", "invalid")
	opts := &driverOptions{Enabled: true, Count: 1, Password: "default-secret"}
	section := sections.Section{Key: "platform.selected", Value: opts}
	if err := cfg.LoadSections(section); err != nil {
		t.Fatal(err)
	}
	if opts.Enabled || opts.Count != 3 || opts.Password != "yaml-secret" || opts.validated != 1 {
		t.Fatalf("selected section not loaded: count=%d enabled=%v validate=%d", opts.Count, opts.Enabled, opts.validated)
	}
	loader, err := sections.New(section)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(loader.Safe(), loader.Fields()), "yaml-secret") {
		t.Fatal("secret exposed")
	}
	t.Setenv("PLATFORM_SELECTED_COUNT", "-1")
	if err := cfg.LoadSections(section); err == nil || strings.Contains(err.Error(), "sensitive-password") {
		t.Fatalf("Validate was not safely propagated: %v", err)
	}
}
