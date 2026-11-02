package config_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/config"
)

type options struct {
	Enabled bool
	Count   int
	Label   string
	Timeout time.Duration
	Hosts   []string
	Nested  struct {
		Password string `secret:"true"`
		Ports    []int
	}
	Hidden struct{ Token string } `secret:"true"`
}

func TestPriorityAndSafe(t *testing.T) {
	o := options{Enabled: true, Count: 7, Label: "default", Timeout: time.Second}
	o.Nested.Password = "secret"
	o.Hidden.Token = "token"
	l, err := config.New(config.Section{Key: "platform.eventbus", Value: &o})
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"PLATFORM_EVENTBUS_ENABLED": "false", "PLATFORM_EVENTBUS_COUNT": "0", "PLATFORM_EVENTBUS_LABEL": "", "PLATFORM_EVENTBUS_TIMEOUT": "3s", "PLATFORM_EVENTBUS_HOSTS": "[a, b]"}
	err = l.Load(strings.NewReader("platform:\n  eventbus:\n    enabled: true\n    count: 8\n    timeout: 2s\n    nested:\n      ports: [1, 2]\n"), func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	if o.Enabled || o.Count != 0 || o.Label != "" || o.Timeout != 3*time.Second || len(o.Hosts) != 2 || len(o.Nested.Ports) != 2 {
		t.Fatalf("unexpected non-secret values")
	}
	output := fmt.Sprint(l.Safe())
	if strings.Contains(output, "secret") || strings.Contains(output, "token") {
		t.Fatal("secret leaked")
	}
	if o.Nested.Password != "secret" || o.Hidden.Token != "token" {
		t.Fatal("working secrets mutated")
	}
	fields := l.Fields()
	if fields[6].Env != "PLATFORM_EVENTBUS_NESTED_PORTS" {
		t.Fatalf("metadata: %v", fields)
	}
	fields[0].Env = "changed"
	if l.Fields()[0].Env == "changed" {
		t.Fatal("metadata aliased")
	}
}
func TestYAMLZeroAndIndependent(t *testing.T) {
	a := options{Enabled: true, Count: 4, Label: "old", Timeout: time.Second}
	b := a
	x, _ := config.New(config.Section{Key: "app", Value: &a})
	y, _ := config.New(config.Section{Key: "app", Value: &b})
	none := func(string) (string, bool) { return "", false }
	if err := x.Load(strings.NewReader("app: {enabled: false, count: 0, label: '', hosts: [], timeout: 0s}"), none); err != nil {
		t.Fatal(err)
	}
	if a.Enabled || a.Count != 0 || a.Label != "" || a.Timeout != 0 || !b.Enabled || b.Count != 4 {
		t.Fatal("precedence/independence")
	}
	if err := y.Load(nil, none); err != nil {
		t.Fatal(err)
	}
	if b.Timeout != time.Second {
		t.Fatal("default lost")
	}
}

type invalid struct {
	Secret string `secret:"true"`
}

func (i *invalid) Validate() error { return fmt.Errorf("secret: %s", i.Secret) }
func TestErrorsAndDisabled(t *testing.T) {
	for _, source := range []string{"app: {count: leaked-secret}", "app: {nested: {ports: [leaked-secret]}}", "app: {count: 1, count: 2}"} {
		var o options
		l, _ := config.New(config.Section{Key: "app", Value: &o})
		err := l.Load(strings.NewReader(source), func(string) (string, bool) { return "", false })
		if err == nil || strings.Contains(err.Error(), "leaked-secret") || !strings.Contains(err.Error(), "app") {
			t.Fatalf("unsafe/missing error: %v", err)
		}
	}
	var o options
	l, _ := config.New(config.Section{Key: "app", Value: &o})
	err := l.Load(nil, func(k string) (string, bool) { return "leaked-secret", k == "APP_COUNT" })
	if err == nil || !strings.Contains(err.Error(), "app.count") || strings.Contains(err.Error(), "leaked-secret") {
		t.Fatal(err)
	}
	bad := invalid{Secret: "leaked-secret"}
	l, _ = config.New(config.Section{Key: "app", Value: &bad})
	err = l.Load(nil, nil)
	if err == nil || strings.Contains(err.Error(), "leaked-secret") {
		t.Fatal(err)
	}
	l, _ = config.New(config.Section{Key: "app", Value: &o})
	if err := l.Load(strings.NewReader("redis: {port: invalid}\nauth: {secret: null}\napp: {count: 3}"), func(string) (string, bool) { return "", false }); err != nil {
		t.Fatal(err)
	}
}
func TestConflicts(t *testing.T) {
	type collision struct {
		A_B string
		A   struct{ B string }
	}
	var c collision
	if _, err := config.New(config.Section{Key: "app", Value: &c}); err == nil {
		t.Fatal("env collision accepted")
	}
	var a, b options
	for _, key := range []string{"app", "app.child"} {
		if _, err := config.New(config.Section{Key: "app", Value: &a}, config.Section{Key: key, Value: &b}); err == nil {
			t.Fatal("overlap accepted")
		}
	}
}
func TestSafeListsAndSnapshot(t *testing.T) {
	type settings struct {
		Tokens []string `secret:"true"`
		Nested struct {
			Password string `secret:"true"`
			Public   []string
		}
	}
	o := settings{Tokens: []string{"sensitive-token"}}
	o.Nested.Password = "sensitive-password"
	o.Nested.Public = []string{"first"}
	l, err := config.New(config.Section{Key: "app", Value: &o})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := l.Safe()
	text := fmt.Sprint(snapshot)
	if strings.Contains(text, "sensitive-") {
		t.Fatal("secret list or nested secret leaked")
	}
	nested := snapshot["app"].(map[string]any)["nested"].(map[string]any)
	nested["public"].([]any)[0] = "changed"
	if o.Nested.Public[0] != "first" || len(o.Tokens) != 1 {
		t.Fatal("snapshot changed live data")
	}
	if !l.Fields()[0].Secret || !l.Fields()[1].Secret {
		t.Fatal("missing secret metadata")
	}
}

func TestMalformedYAMLAndSchemaErrorsAreSafe(t *testing.T) {
	var o options
	l, _ := config.New(config.Section{Key: "app", Value: &o})
	err := l.Load(strings.NewReader("app: [sensitive-value"), nil)
	if err == nil || strings.Contains(err.Error(), "sensitive-value") {
		t.Fatal(err)
	}
	type names struct{ Name string }
	var a, b names
	if _, err := config.New(config.Section{Key: "platform.eventbus", Value: &a}, config.Section{Key: "platform_eventbus", Value: &b}); err == nil {
		t.Fatal("section env collision accepted")
	}
	var nilTarget *names
	if _, err := config.New(config.Section{Key: "app", Value: nilTarget}); err == nil {
		t.Fatal("nil target accepted")
	}
}

func TestSectionMetadataIsDetached(t *testing.T) {
	var o options
	path := []string{"legacy"}
	l, err := config.New(config.Section{Key: "platform.eventbus", Value: &o, YAMLPath: path})
	if err != nil {
		t.Fatal(err)
	}
	path[0] = "changed"
	metadata := l.Sections()
	if metadata[0].Key != "platform.eventbus" || metadata[0].EnvPrefix != "PLATFORM_EVENTBUS_" || metadata[0].YAMLPath[0] != "legacy" {
		t.Fatal("invalid metadata")
	}
	metadata[0].YAMLPath[0] = "changed"
	metadata[0].Fields[0].Env = "changed"
	if l.Sections()[0].YAMLPath[0] != "legacy" || l.Fields()[0].Env == "changed" {
		t.Fatal("metadata aliased")
	}
	type recursive []recursive
	var r struct{ Values recursive }
	if _, err := config.New(config.Section{Key: "app", Value: &r}); err == nil {
		t.Fatal("recursive slice accepted")
	}
}

func TestMetadataDefaultsSurviveLoad(t *testing.T) {
	type settings struct {
		Count    int           `env:"OLD_COUNT" env-default:"12"`
		Password string        `env:"OLD_PASSWORD" env-default:"secret-tag" secret:"true"`
		Timeout  time.Duration `env:"OLD_TIMEOUT" env-default:"3s"`
	}
	s := settings{Count: 4, Password: "secret-initial"}
	l, err := config.New(config.Section{Key: "app", Value: &s, LegacyEnv: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Count != 4 || s.Timeout != 0 {
		t.Fatal("metadata mutated target")
	}
	before := fmt.Sprint(l.Sections())
	if strings.Contains(before, "secret-tag") || strings.Contains(before, "secret-initial") {
		t.Fatal("default leaked")
	}
	if err := l.Load(strings.NewReader("app: {Count: 99, count: 99}"), func(k string) (string, bool) { return "21", k == "OLD_COUNT" }); err != nil {
		t.Fatal(err)
	}
	if s.Count != 21 || fmt.Sprint(l.Sections()) != before {
		t.Fatal("metadata contains loaded values")
	}
	fields := l.Fields()
	if fields[0].Default != "12" || fields[1].Default != "[REDACTED]" || fields[2].Default != "3s" {
		t.Fatalf("defaults: %v", fields)
	}
	fields[0].Default = "changed"
	if l.Fields()[0].Default != "12" {
		t.Fatal("default snapshot aliased")
	}
}
