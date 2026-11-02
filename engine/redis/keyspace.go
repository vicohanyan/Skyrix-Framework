package redis

import "strings"

// separator joins key segments.
const separator = ":"

// Keyspace builds keys under a prefix. The application root keyspace comes
// from Config.KeyPrefix; each consumer takes its own namespace with Sub, so
// services and consumers sharing a Redis database do not collide:
//
//	root, _ := redis.NewKeyspace("orders")  // orders
//	tenants := root.Sub("tenant")          // orders:tenant
//	tenants.Key("domain", "example.com")   // orders:tenant:domain:example.com
//
// The zero Keyspace has no prefix; consumers reject it with IsZero.
type Keyspace struct {
	prefix string
}

// NewKeyspace returns a keyspace rooted at prefix. Surrounding whitespace and
// one trailing separator are removed; an empty prefix is ErrEmptyKeyPrefix.
func NewKeyspace(prefix string) (Keyspace, error) {
	prefix = normalizePrefix(prefix)
	if prefix == "" {
		return Keyspace{}, ErrEmptyKeyPrefix
	}
	return Keyspace{prefix: prefix}, nil
}

// Sub returns the namespace name inside k. It panics when name is empty
// (after trimming whitespace and separators): namespaces are constants of the
// consumer, so an empty one is a programming error.
func (k Keyspace) Sub(name string) Keyspace {
	name = strings.Trim(strings.TrimSpace(name), separator)
	if name == "" {
		panic("redis: empty keyspace namespace")
	}
	return Keyspace{prefix: k.join(name)}
}

// Key joins parts with the separator under the prefix.
func (k Keyspace) Key(parts ...string) string {
	return k.join(strings.Join(parts, separator))
}

// Prefix returns the prefix without a trailing separator.
func (k Keyspace) Prefix() string { return k.prefix }

// IsZero reports whether k has no prefix.
func (k Keyspace) IsZero() bool { return k.prefix == "" }

// String returns the prefix.
func (k Keyspace) String() string { return k.prefix }

func (k Keyspace) join(rest string) string {
	if k.prefix == "" {
		return rest
	}
	return k.prefix + separator + rest
}

func normalizePrefix(prefix string) string {
	return strings.TrimSuffix(strings.TrimSpace(prefix), separator)
}
