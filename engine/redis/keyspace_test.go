package redis_test

import (
	"errors"
	"testing"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
)

func TestKeyspace(t *testing.T) {
	root, err := engineredis.NewKeyspace("  orders: ")
	if err != nil {
		t.Fatalf("NewKeyspace() error = %v", err)
	}
	if root.Prefix() != "orders" || root.IsZero() {
		t.Fatalf("root = %q, want orders", root.Prefix())
	}

	tests := []struct {
		got, want string
	}{
		{root.Key("a"), "orders:a"},
		{root.Key("a", "b", "c"), "orders:a:b:c"},
		{root.Sub("tenant").Key("domain", "example.com"), "orders:tenant:domain:example.com"},
		{root.Sub(" auth: ").Sub("sess").Key("jti"), "orders:auth:sess:jti"},
		{root.Sub("auth").Prefix(), "orders:auth"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("key = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestKeyspaceNamespacesDoNotCollide(t *testing.T) {
	a, _ := engineredis.NewKeyspace("service-a")
	b, _ := engineredis.NewKeyspace("service-b")
	if a.Sub("auth").Key("x") == b.Sub("auth").Key("x") {
		t.Fatal("keys of different prefixes collide")
	}
	if a.Sub("auth").Key("x") == a.Sub("tenant").Key("x") {
		t.Fatal("keys of different namespaces collide")
	}
}

func TestNewKeyspaceRejectsEmptyPrefix(t *testing.T) {
	for _, prefix := range []string{"", " ", ":"} {
		if _, err := engineredis.NewKeyspace(prefix); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
			t.Errorf("NewKeyspace(%q) error = %v, want ErrEmptyKeyPrefix", prefix, err)
		}
	}
	if !(engineredis.Keyspace{}).IsZero() {
		t.Fatal("zero Keyspace IsZero() = false")
	}
}

func TestSubPanicsOnEmptyNamespace(t *testing.T) {
	root, _ := engineredis.NewKeyspace("app")
	defer func() {
		if recover() == nil {
			t.Fatal("Sub(\"\") did not panic")
		}
	}()
	root.Sub(" : ")
}
