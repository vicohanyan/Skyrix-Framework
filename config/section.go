// Package config loads explicitly supplied, consumer-owned configuration sections.
package config

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Section describes one consumer-owned pointer to a struct. Existing values are
// defaults. Do not log Value; use Loader.Safe instead.
type Section struct {
	Key   string
	Value any
	// YAMLPath overrides the dotted key's YAML path. An empty slice selects
	// the document root. Nil uses Key. Intended for legacy application formats.
	YAMLPath []string
	// LegacyEnv uses explicit env tags (without a section prefix), and
	// env-default tags. Normal sections derive every env name from their key.
	LegacyEnv bool
}

type Validator interface{ Validate() error }

// Field is value-free metadata shared by loading and diagnostic tools.
type Field struct {
	Path   string
	Env    string
	Type   string
	Secret bool
}

// Metadata describes a selected section without exposing its working target.
type Metadata struct {
	Key       string
	EnvPrefix string
	YAMLPath  []string
	Fields    []Field
}

// Sections returns detached section metadata, including the effective YAML path.
// Legacy fields can override the derived EnvPrefix; Field.Env is authoritative.
func (l *Loader) Sections() []Metadata {
	out := make([]Metadata, 0, len(l.entries))
	for _, e := range l.entries {
		path := e.section.YAMLPath
		if path == nil {
			path = strings.Split(e.section.Key, ".")
		}
		m := Metadata{Key: e.section.Key, EnvPrefix: envPrefix(e.section.Key), YAMLPath: append([]string{}, path...)}
		for _, f := range e.fields {
			m.Fields = append(m.Fields, f.Field)
		}
		out = append(out, m)
	}
	return out
}
func envPrefix(key string) string { return strings.ToUpper(strings.ReplaceAll(key, ".", "_")) + "_" }

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
		if err := walk(v.Elem().Type(), nil, "", false, s, &e.fields); err != nil {
			return nil, err
		}
		for _, f := range e.fields {
			if envs[f.Env] || paths[f.Path] {
				return nil, fmt.Errorf("config %s: conflicting field or env name", f.Path)
			}
			envs[f.Env], paths[f.Path] = true, true
		}
		l.entries = append(l.entries, e)
	}
	return l, nil
}

func walk(t reflect.Type, index []int, prefix string, secret bool, s Section, fields *[]binding) error {
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
		if f.Type.Kind() == reflect.Struct {
			if err := walk(f.Type, idx, path, hidden, s, fields); err != nil {
				return err
			}
			continue
		}
		if !supported(f.Type) {
			return fmt.Errorf("config %s.%s: unsupported type", s.Key, path)
		}
		env := envPrefix(s.Key) + strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
		if s.LegacyEnv && f.Tag.Get("env") != "" {
			env = f.Tag.Get("env")
		}
		*fields = append(*fields, binding{Field: Field{Path: s.Key + "." + path, Env: env, Type: f.Type.String(), Secret: hidden}, index: idx})
	}
	return nil
}
func supported(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	case reflect.Slice:
		return supportedList(t, map[reflect.Type]bool{})
	}
	return false
}

func supportedList(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t.Kind() != reflect.Slice {
		return supported(t)
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	return supportedList(t.Elem(), seen)
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
// targets; discard them on error. lookup nil uses os.LookupEnv (including empty).
func (l *Loader) Load(r io.Reader, lookup func(string) (string, bool)) error {
	if lookup == nil {
		lookup = os.LookupEnv
	}
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
		v := reflect.ValueOf(s.Value).Elem()
		if s.LegacyEnv {
			for _, f := range e.fields {
				sf := v.Type().FieldByIndex(f.index)
				if def, ok := sf.Tag.Lookup("env-default"); ok {
					if err := decodeText(def, v.FieldByIndex(f.index)); err != nil {
						return fmt.Errorf("config %s: invalid default", f.Path)
					}
				}
			}
		}
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
			if err := overlay(n, v, s.Key); err != nil {
				return err
			}
		}
		for _, f := range e.fields {
			if raw, ok := lookup(f.Env); ok {
				target := v.FieldByIndex(f.index)
				if s.LegacyEnv && target.Kind() == reflect.Slice && target.Type().Elem().Kind() == reflect.String {
					var parts []string
					if raw != "" {
						parts = strings.Split(raw, ",")
					}
					list := reflect.MakeSlice(target.Type(), len(parts), len(parts))
					for i, p := range parts {
						list.Index(i).SetString(strings.TrimSpace(p))
					}
					target.Set(list)
				} else if err := decodeText(raw, target); err != nil {
					return fmt.Errorf("config %s: invalid environment value", f.Path)
				}
			}
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
func overlay(n *yaml.Node, v reflect.Value, path string) error {
	if v.Kind() == reflect.Struct {
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
			item, err := child(n, name)
			if err != nil {
				return fmt.Errorf("config %s: invalid YAML mapping", path)
			}
			if item != nil {
				if err := overlay(item, v.Field(i), path+"."+name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if n.Tag == "!!null" || n.Kind == yaml.AliasNode {
		return fmt.Errorf("config %s: null and aliases are unsupported", path)
	}
	if v.Kind() == reflect.Slice {
		if n.Kind != yaml.SequenceNode {
			return fmt.Errorf("config %s: expected list", path)
		}
		result := reflect.MakeSlice(v.Type(), len(n.Content), len(n.Content))
		for i, item := range n.Content {
			if err := overlay(item, result.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		v.Set(result)
		return nil
	}
	if err := n.Decode(v.Addr().Interface()); err != nil {
		return fmt.Errorf("config %s: invalid value", path)
	}
	return nil
}
func decodeText(raw string, v reflect.Value) error {
	if v.Kind() == reflect.String {
		v.SetString(raw)
		return nil
	}
	if v.Type() == durationType {
		d, err := time.ParseDuration(raw)
		if err == nil {
			v.SetInt(int64(d))
		}
		return err
	}
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &n); err != nil {
		return err
	}
	if len(n.Content) == 0 {
		return fmt.Errorf("empty value")
	}
	return overlay(n.Content[0], v, "env")
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
	switch v.Kind() {
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
	case reflect.Slice:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = safe(v.Index(i), false)
		}
		return out
	default:
		return v.Interface()
	}
}
