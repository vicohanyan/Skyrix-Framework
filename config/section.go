// Package config loads explicitly supplied, consumer-owned configuration sections.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"go.yaml.in/yaml/v3"
)

// Section describes one consumer-owned pointer to a struct. Existing values are
// defaults. Do not log Value; use Loader.Safe instead.
type Section struct {
	Key   string
	Value any
	// YAMLPath overrides the dotted key's YAML path. An empty slice selects
	// the document root. Nil uses Key.
	YAMLPath []string
}

type Validator interface{ Validate() error }

// Field describes a field without exposing its current value.
// Default is a formatted snapshot taken by New; secret defaults are redacted.
type Field struct {
	Path    string
	Env     string
	Type    string
	Secret  bool
	Default string
}

// Metadata describes a selected section without exposing its working target.
type Metadata struct {
	Key      string
	YAMLPath []string
	Fields   []Field
}

// Sections returns detached section metadata, including the effective YAML path.
// Environment names come from cleanenv tags, including nested env-prefix tags.
func (l *Loader) Sections() []Metadata {
	out := make([]Metadata, 0, len(l.entries))
	for _, e := range l.entries {
		path := e.section.YAMLPath
		if path == nil {
			path = strings.Split(e.section.Key, ".")
		}
		m := Metadata{Key: e.section.Key, YAMLPath: append([]string{}, path...)}
		for _, f := range e.fields {
			m.Fields = append(m.Fields, f.Field)
		}
		out = append(out, m)
	}
	return out
}

type binding struct {
	Field
	index []int
}
type entry struct {
	section Section
	fields  []binding
}

// Loader is an instance-local composition; it has no registry or connections.
// A Loader and its targets must not be mutated concurrently.
type Loader struct{ entries []entry }

var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
var durationType = reflect.TypeOf(time.Duration(0))

func New(sections ...Section) (*Loader, error) {
	l := &Loader{}
	envs, paths := map[string]bool{}, map[string]bool{}
	for _, s := range sections {
		if !keyPattern.MatchString(s.Key) {
			return nil, fmt.Errorf("config: invalid section key")
		}
		v := reflect.ValueOf(s.Value)
		if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
			return nil, fmt.Errorf("config %s: expected struct pointer", s.Key)
		}
		for _, previous := range l.entries {
			if s.Key == previous.section.Key || strings.HasPrefix(s.Key, previous.section.Key+".") || strings.HasPrefix(previous.section.Key, s.Key+".") {
				return nil, fmt.Errorf("config %s: overlapping section keys", s.Key)
			}
		}
		if s.YAMLPath != nil {
			s.YAMLPath = append([]string{}, s.YAMLPath...)
		}
		e := entry{section: s}
		if err := walk(v.Elem().Type(), nil, "", "", false, s, &e.fields); err != nil {
			return nil, err
		}
		for i := range e.fields {
			f := &e.fields[i]
			if f.Secret {
				f.Default = "[REDACTED]"
			} else {
				value := v.Elem().FieldByIndex(f.index)
				f.Default = formatDefault(value)
				if raw, ok := v.Elem().Type().FieldByIndex(f.index).Tag.Lookup("env-default"); ok && value.IsZero() {
					if value.Kind() == reflect.String {
						f.Default = strconv.Quote(raw)
					} else {
						f.Default = raw
					}
				}
			}
			if paths[f.Path] {
				return nil, fmt.Errorf("config %s: conflicting field or env name", f.Path)
			}
			for _, env := range strings.Split(f.Env, ",") {
				if env == "" {
					continue
				}
				if envs[env] {
					return nil, fmt.Errorf("config %s: conflicting env name", f.Path)
				}
				envs[env] = true
			}
			paths[f.Path] = true
		}
		l.entries = append(l.entries, e)
	}
	return l, nil
}

// formatDefault makes strings and lists unambiguous and keeps table rows on one line.
func formatDefault(v reflect.Value) string {
	if v.Type() == durationType {
		return time.Duration(v.Int()).String()
	}
	if v.Kind() == reflect.String {
		return strconv.Quote(v.String())
	}
	if v.Kind() == reflect.Slice {
		if data, err := json.Marshal(v.Interface()); err == nil {
			return string(data)
		}
	}
	return fmt.Sprint(v.Interface())
}

func walk(t reflect.Type, index []int, prefix, envPrefix string, secret bool, s Section, fields *[]binding) error {
	seen := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if name == "-" {
			continue
		}
		if strings.Contains(f.Tag.Get("yaml"), ",") {
			return fmt.Errorf("config %s: YAML options are unsupported", s.Key)
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		if !namePattern.MatchString(name) || seen[name] {
			return fmt.Errorf("config %s: invalid or duplicate field name", s.Key)
		}
		seen[name] = true
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		idx := append(append([]int{}, index...), i)
		hidden := secret || f.Tag.Get("secret") == "true"
		if f.Type.Kind() == reflect.Struct && f.Type != reflect.TypeOf(time.Time{}) {
			if err := walk(f.Type, idx, path, envPrefix+f.Tag.Get("env-prefix"), hidden, s, fields); err != nil {
				return err
			}
			continue
		}
		env := f.Tag.Get("env")
		if env != "" {
			names := strings.Split(env, ",")
			for i := range names {
				names[i] = envPrefix + names[i]
			}
			env = strings.Join(names, ",")
		}
		*fields = append(*fields, binding{Field: Field{Path: s.Key + "." + path, Env: env, Type: f.Type.String(), Secret: hidden}, index: idx})
	}
	return nil
}

// Fields returns an independent metadata slice in declaration order.
func (l *Loader) Fields() []Field {
	var out []Field
	for _, e := range l.entries {
		for _, f := range e.fields {
			out = append(out, f.Field)
		}
	}
	return out
}

// Load overlays YAML and then environment on the targets' defaults. Only the
// supplied sections are decoded and validated. Failed loads may partially update
// targets; discard them on error. Environment names and parsing use cleanenv tags.
func (l *Loader) Load(r io.Reader) error {
	var root yaml.Node
	if r != nil {
		d := yaml.NewDecoder(r)
		if err := d.Decode(&root); err != nil && err != io.EOF {
			return fmt.Errorf("config: invalid YAML document")
		}
		var extra yaml.Node
		if err := d.Decode(&extra); err != io.EOF {
			return fmt.Errorf("config: expected one YAML document")
		}
	}
	var doc *yaml.Node
	if len(root.Content) > 0 {
		doc = root.Content[0]
	}
	for _, e := range l.entries {
		s := e.section
		path := s.YAMLPath
		if path == nil {
			path = strings.Split(s.Key, ".")
		}
		n := doc
		for _, part := range path {
			var err error
			n, err = child(n, part)
			if err != nil {
				return fmt.Errorf("config %s: invalid YAML mapping", s.Key)
			}
		}
		if n != nil {
			if err := n.Decode(s.Value); err != nil {
				return fmt.Errorf("config %s: invalid YAML value", s.Key)
			}
		}
		if err := cleanenv.ReadEnv(s.Value); err != nil {
			return fmt.Errorf("config %s: invalid environment or default value", s.Key)
		}
	}
	for _, e := range l.entries {
		if validator, ok := e.section.Value.(Validator); ok {
			if validator.Validate() != nil {
				return fmt.Errorf("config %s: validation failed", e.section.Key)
			}
		}
	}
	return nil
}
func child(n *yaml.Node, key string) (*yaml.Node, error) {
	if n == nil {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("mapping required")
	}
	seen := map[string]bool{}
	var found *yaml.Node
	for i := 0; i < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || seen[k.Value] {
			return nil, fmt.Errorf("duplicate or invalid key")
		}
		seen[k.Value] = true
		if k.Value == key {
			found = n.Content[i+1]
		}
	}
	return found, nil
}

// Safe returns a detached diagnostic tree. Secret fields (including whole
// structs or lists) are replaced without changing the working configuration.
func (l *Loader) Safe() map[string]any {
	out := map[string]any{}
	for _, e := range l.entries {
		out[e.section.Key] = safe(reflect.ValueOf(e.section.Value).Elem(), false)
	}
	return out
}
func safe(v reflect.Value, hidden bool) any {
	if hidden {
		return "[REDACTED]"
	}
	if v.Type() == reflect.TypeOf(time.Time{}) {
		return v.Interface()
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return safe(v.Elem(), false)
	case reflect.Map:
		out := map[string]any{}
		iter := v.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = safe(iter.Value(), false)
		}
		return out
	case reflect.Struct:
		out := map[string]any{}
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.PkgPath != "" {
				continue
			}
			name := strings.Split(f.Tag.Get("yaml"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = strings.ToLower(f.Name)
			}
			out[name] = safe(v.Field(i), f.Tag.Get("secret") == "true")
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = safe(v.Index(i), false)
		}
		return out
	default:
		return v.Interface()
	}
}
