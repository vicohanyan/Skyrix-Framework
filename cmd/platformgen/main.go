package main

// Command platformgen generates independent Wire adapters for platform drivers.
//
// Run it from the application's providers directory (via go generate). It
// reads the //skyrix:platform directives in platform.go and writes
// platform_gen.go with one independent provider set per driver. The application
// owns platform composition and console command selection.
// Modules with a config=<Field> option bind <Field> of the application's
// config.Config; the config package defaults to <module>/internal/config and
// can be set with -config.
import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"

	"gitlab.com/skyrix-lib/framework/scaffold"
)

const directivePrefix = "//skyrix:platform "

type directive struct {
	ImportPath string
	Config     string
	Alias      string
}

func main() {
	configImport := flag.String("config", "", "import path of the application config package (default <module>/internal/config)")
	flag.Parse()
	if err := run(*configImport); err != nil {
		fmt.Fprintln(os.Stderr, "platformgen:", err)
		os.Exit(1)
	}
}

func run(configImport string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("working directory: %w", err)
	}
	if configImport == "" {
		if configImport, err = defaultConfigImport(cwd); err != nil {
			return err
		}
	}
	source := filepath.Join(cwd, "platform.go")
	directives, err := parseDirectives(source)
	if err != nil {
		return err
	}
	// Without directives there are no generated sets; application-owned platform
	// constructors still define each binary's composition.
	if len(directives) > 0 {
		if err := resolvePackages(directives); err != nil {
			return err
		}
	}
	if err := validateConfigFields(directives, configImport); err != nil {
		return err
	}
	generated, err := render(directives, configImport)
	if err != nil {
		return err
	}
	output := filepath.Join(cwd, "platform_gen.go")
	if err := os.WriteFile(output, generated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	return nil
}

func parseDirectives(path string) ([]*directive, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	seen := map[string]struct{}{}
	var result []*directive
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(text, directivePrefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(text, directivePrefix))
		if len(fields) == 0 || len(fields) > 2 {
			return nil, fmt.Errorf("%s:%d: expected %s<import-path> [config=<field>]", path, line, directivePrefix)
		}
		item := &directive{ImportPath: fields[0]}
		if _, duplicate := seen[item.ImportPath]; duplicate {
			return nil, fmt.Errorf("%s:%d: duplicate platform %q", path, line, item.ImportPath)
		}
		seen[item.ImportPath] = struct{}{}
		if len(fields) == 2 {
			config, ok := strings.CutPrefix(fields[1], "config=")
			if !ok || !validIdentifier(config) {
				return nil, fmt.Errorf("%s:%d: invalid config option %q", path, line, fields[1])
			}
			item.Config = config
		}
		result = append(result, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return result, nil
}

func resolvePackages(directives []*directive) error {
	patterns := make([]string, len(directives))
	for index, item := range directives {
		patterns[index] = item.ImportPath
	}
	loaded, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedTypes}, patterns...)
	if err != nil {
		return fmt.Errorf("load platform packages: %w", err)
	}
	if packages.PrintErrors(loaded) > 0 {
		return errors.New("load platform packages: package errors reported")
	}
	byPath := make(map[string]*packages.Package, len(loaded))
	for _, pkg := range loaded {
		byPath[pkg.PkgPath] = pkg
	}
	aliases := map[string]struct{}{"wire": {}, "config": {}}
	for _, item := range directives {
		pkg := byPath[item.ImportPath]
		if pkg == nil || pkg.Types == nil {
			return fmt.Errorf("load platform package %q: package not found", item.ImportPath)
		}
		for _, symbol := range []string{"ProviderSet", "Module"} {
			if pkg.Types.Scope().Lookup(symbol) == nil {
				return fmt.Errorf("platform package %q does not export %s", item.ImportPath, symbol)
			}
		}
		if item.Config != "" && pkg.Types.Scope().Lookup("Config") == nil {
			return fmt.Errorf("platform package %q uses config but does not export Config", item.ImportPath)
		}
		item.Alias = uniqueAlias(item.ImportPath, pkg.Name, aliases)
		aliases[strings.ToLower(item.Alias)] = struct{}{}
	}
	return nil
}

func validateConfigFields(directives []*directive, configImport string) error {
	if !hasConfig(directives) {
		return nil
	}
	loaded, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedTypes}, configImport)
	if err != nil || len(loaded) != 1 || loaded[0].Types == nil {
		return fmt.Errorf("load application config package: %w", err)
	}
	object := loaded[0].Types.Scope().Lookup("Config")
	if object == nil {
		return errors.New("application config package does not export Config")
	}
	for _, item := range directives {
		if item.Config == "" {
			continue
		}
		field, _, _ := types.LookupFieldOrMethod(types.NewPointer(object.Type()), true, loaded[0].Types, item.Config)
		if field == nil {
			return fmt.Errorf("application config.Config does not contain field %q", item.Config)
		}
	}
	return nil
}

func render(directives []*directive, configImport string) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("// Code generated by platformgen. DO NOT EDIT.\n\npackage providers\n")
	if len(directives) > 0 {
		output.WriteString("\nimport (\n\t\"github.com/goforj/wire\"\n")
		if hasConfig(directives) {
			fmt.Fprintf(&output, "\t\"%s\"\n", configImport)
		}
		for _, item := range directives {
			fmt.Fprintf(&output, "\t%s %q\n", item.Alias, item.ImportPath)
		}
		output.WriteString(")\n")
	}
	for _, item := range directives {
		name := strings.ToUpper(item.Alias[:1]) + item.Alias[1:] + "Set"
		if strings.HasPrefix(name, "_") {
			name = "Driver" + name
		}
		fmt.Fprintf(&output, "\n// %s provides only %s; composition is application-owned.\n", name, item.ImportPath)
		fmt.Fprintf(&output, "var %s = wire.NewSet(\n\t%s.ProviderSet,\n", name, item.Alias)
		if item.Config != "" {
			fmt.Fprintf(&output, "\twire.FieldsOf(new(*config.Config), %q),\n", item.Config)
			fmt.Fprintf(&output, "\twire.Bind(new(%s.Config), new(*config.%s)),\n", item.Alias, item.Config)
		}
		output.WriteString(")\n")
	}
	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w\n%s", err, output.String())
	}
	return formatted, nil
}

func hasConfig(directives []*directive) bool {
	for _, item := range directives {
		if item.Config != "" {
			return true
		}
	}
	return false
}

func uniqueAlias(importPath, packageName string, used map[string]struct{}) string {
	parts := strings.Split(strings.Trim(importPath, "/"), "/")
	base := packageName
	if len(parts) > 1 {
		base = parts[len(parts)-2] + packageName
	}
	base = sanitizeIdentifier(base)
	alias := base
	for suffix := 2; ; suffix++ {
		if _, exists := used[strings.ToLower(alias)]; !exists {
			return alias
		}
		alias = fmt.Sprintf("%s%d", base, suffix)
	}
}

var invalidIdentifier = regexp.MustCompile(`[^[:alnum:]_]`)

func sanitizeIdentifier(value string) string {
	value = invalidIdentifier.ReplaceAllString(value, "")
	if value == "" || value[0] >= '0' && value[0] <= '9' {
		return "platformmodule"
	}
	return value
}

func validIdentifier(value string) bool {
	if value == "" || sanitizeIdentifier(value) != value {
		return false
	}
	return value[0] < '0' || value[0] > '9'
}

// defaultConfigImport returns <module>/internal/config for the module that
// contains dir.
func defaultConfigImport(dir string) (string, error) {
	for current := dir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			modulePath, err := scaffold.ModulePath(current)
			if err != nil {
				return "", err
			}
			return modulePath + "/internal/config", nil
		}
		if parent := filepath.Dir(current); parent == current {
			return "", fmt.Errorf("no go.mod found above %s; pass -config", dir)
		}
	}
}
