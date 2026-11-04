package config_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/config"
)

type options struct {
	Enabled  bool          `yaml:"enabled" env:"APP_ENABLED"`
	Count    int           `yaml:"count" env:"APP_COUNT"`
	Label    string        `yaml:"label" env:"APP_LABEL"`
	Timeout  time.Duration `yaml:"timeout" env:"APP_TIMEOUT"`
	Hosts    []string      `yaml:"hosts" env:"APP_HOSTS"`
	Database struct {
		Host     string `yaml:"host" env:"HOST"`
		Password string `yaml:"password" env:"PASSWORD" secret:"true"`
	} `yaml:"database" env-prefix:"DB_"`
}

func TestYAMLWithEnvironmentSecretsAndOverrides(t *testing.T) {
	o := options{Count: 1, Label: "default"}
	loader, err := config.New(config.Section{Key: "app", Value: &o})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"APP_ENABLED": "false", "APP_COUNT": "0", "APP_LABEL": "", "APP_TIMEOUT": "3s", "APP_HOSTS": "a,b", "DB_PASSWORD": "gitlab-secret"} {
		t.Setenv(k, v)
	}
	err = loader.Load(strings.NewReader("app:\n  enabled: true\n  count: 8\n  label: production\n  timeout: 2s\n  hosts: [yaml]\n  database:\n    host: postgres\n"))
	if err != nil {
		t.Fatal(err)
	}
	if o.Enabled || o.Count != 0 || o.Label != "" || o.Timeout != 3*time.Second || len(o.Hosts) != 2 || o.Hosts[1] != "b" || o.Database.Host != "postgres" || o.Database.Password != "gitlab-secret" {
		t.Fatal("YAML/env precedence failed")
	}
	if strings.Contains(fmt.Sprint(loader.Safe()), "gitlab-secret") {
		t.Fatal("secret leaked")
	}
	fields := loader.Fields()
	if fields[6].Env != "DB_PASSWORD" {
		t.Fatalf("env-prefix metadata: %v", fields)
	}
	fields[0].Env = "changed"
	if loader.Fields()[0].Env == "changed" {
		t.Fatal("metadata aliases working data")
	}
}

func TestYAMLZeroValuesAndIndependentTargets(t *testing.T) {
	a := options{Enabled: true, Count: 4, Label: "old", Timeout: time.Second}
	b := a
	x, _ := config.New(config.Section{Key: "app", Value: &a})
	y, _ := config.New(config.Section{Key: "app", Value: &b})
	if err := x.Load(strings.NewReader("app: {enabled: false, count: 0, label: '', hosts: [], timeout: 0s}")); err != nil {
		t.Fatal(err)
	}
	if a.Enabled || a.Count != 0 || a.Label != "" || a.Timeout != 0 || len(a.Hosts) != 0 || !b.Enabled || b.Count != 4 {
		t.Fatal("zero values or isolation lost")
	}
	if err := y.Load(nil); err != nil {
		t.Fatal(err)
	}
	if b.Timeout != time.Second {
		t.Fatal("struct defaults lost")
	}
}

type invalid struct {
	Secret string `secret:"true"`
}

func (i *invalid) Validate() error { return fmt.Errorf("secret: %s", i.Secret) }

func TestErrorsDoNotExposeValuesAndUnselectedSectionsAreIgnored(t *testing.T) {
	for _, source := range []string{"app: {count: leaked-secret}", "app: {count: 1, count: 2}", "app: [leaked-secret"} {
		var o options
		l, _ := config.New(config.Section{Key: "app", Value: &o})
		err := l.Load(strings.NewReader(source))
		if err == nil || strings.Contains(err.Error(), "leaked-secret") {
			t.Fatalf("unsafe/missing error: %v", err)
		}
	}
	t.Run("environment", func(t *testing.T) {
		t.Setenv("APP_COUNT", "leaked-secret")
		var o options
		l, _ := config.New(config.Section{Key: "app", Value: &o})
		err := l.Load(nil)
		if err == nil || strings.Contains(err.Error(), "leaked-secret") {
			t.Fatal(err)
		}
	})
	bad := invalid{Secret: "leaked-secret"}
	l, _ := config.New(config.Section{Key: "app", Value: &bad})
	if err := l.Load(nil); err == nil || strings.Contains(err.Error(), bad.Secret) {
		t.Fatal(err)
	}
	var o options
	l, _ = config.New(config.Section{Key: "app", Value: &o})
	if err := l.Load(strings.NewReader("redis: {port: invalid}\napp: {count: 3}")); err != nil || o.Count != 3 {
		t.Fatalf("selected section: %v", err)
	}
}

func TestCleanenvDefaultsRequiredAndCustomSeparators(t *testing.T) {
	var o struct {
		Port     int      `yaml:"port" env:"SERVICE_PORT" env-default:"8080"`
		Hosts    []string `yaml:"hosts" env:"SERVICE_HOSTS" env-separator:";"`
		Password string   `yaml:"password" env:"SERVICE_PASSWORD" env-required:"true" secret:"true"`
	}
	l, _ := config.New(config.Section{Key: "service", Value: &o})
	if err := l.Load(nil); err == nil {
		t.Fatal("missing required value accepted")
	}
	t.Setenv("SERVICE_PASSWORD", "secret")
	t.Setenv("SERVICE_HOSTS", "one;two")
	if err := l.Load(nil); err != nil {
		t.Fatal(err)
	}
	if o.Port != 8080 || len(o.Hosts) != 2 || o.Hosts[1] != "two" {
		t.Fatal("cleanenv tags not applied")
	}
	if l.Fields()[0].Default != "8080" {
		t.Fatal("default metadata")
	}
}

func TestMetadataSnapshotAndRedaction(t *testing.T) {
	type settings struct {
		Count    int    `env:"META_COUNT" env-default:"12"`
		Password string `env:"META_PASSWORD" env-default:"secret-tag" secret:"true"`
		Public   []string
		Tokens   []string               `secret:"true"`
		Hidden   struct{ Token string } `secret:"true"`
	}
	s := settings{Public: []string{"first"}, Tokens: []string{"sensitive-token"}}
	s.Hidden.Token = "sensitive-nested"
	path := []string{"CUSTOM"}
	l, err := config.New(config.Section{Key: "app", Value: &s, YAMLPath: path})
	if err != nil {
		t.Fatal(err)
	}
	path[0] = "changed"
	before := fmt.Sprint(l.Sections())
	if strings.Contains(before, "secret-tag") {
		t.Fatal("default secret leaked")
	}
	t.Setenv("META_COUNT", "21")
	if err := l.Load(strings.NewReader("CUSTOM: {count: 99}")); err != nil {
		t.Fatal(err)
	}
	if s.Count != 21 || fmt.Sprint(l.Sections()) != before {
		t.Fatal("metadata changed after load")
	}
	m := l.Sections()
	m[0].YAMLPath[0] = "changed"
	m[0].Fields[0].Env = "changed"
	if l.Sections()[0].YAMLPath[0] != "CUSTOM" || l.Fields()[0].Env != "META_COUNT" {
		t.Fatal("metadata aliases")
	}
	snapshot := l.Safe()
	if text := fmt.Sprint(snapshot); strings.Contains(text, "sensitive-") || strings.Contains(text, "secret-tag") {
		t.Fatal("snapshot leaked secrets")
	}
	snapshot["app"].(map[string]any)["public"].([]any)[0] = "changed"
	if s.Public[0] != "first" {
		t.Fatal("snapshot mutated target")
	}
}

func TestInvalidSections(t *testing.T) {
	var a, b struct{ Name string }
	for _, key := range []string{"app", "app.child"} {
		if _, err := config.New(config.Section{Key: "app", Value: &a}, config.Section{Key: key, Value: &b}); err == nil {
			t.Fatal("overlap accepted")
		}
	}
	var nilTarget *options
	if _, err := config.New(config.Section{Key: "app", Value: nilTarget}); err == nil {
		t.Fatal("nil target accepted")
	}
}
