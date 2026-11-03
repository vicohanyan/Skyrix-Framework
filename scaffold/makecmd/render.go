package makecmd

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed stubs/*.stub
var builtins embed.FS

// stubData is the shared contract available to built-in and project templates.
type stubData struct {
	Module, DomainImport, Package, Type, Var, Use, JobName string
}

func renderStub(op *operation, name string, data stubData, goSource bool) ([]byte, error) {
	source, err := op.optional(filepath.Join("stubs", "skyrix", name+".stub"))
	if err != nil {
		return nil, err
	}
	if source == nil {
		source, err = builtins.ReadFile("stubs/" + name + ".stub")
		if err != nil {
			return nil, err
		}
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(source))
	if err != nil {
		return nil, fmt.Errorf("stub %s: %w", name, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("stub %s: %w", name, err)
	}
	if !goSource {
		return out.Bytes(), nil
	}
	result, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("stub %s generated invalid Go: %w", name, err)
	}
	return result, nil
}

func exportStubs(op *operation) error {
	entries, err := fs.ReadDir(builtins, "stubs")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".stub") {
			continue
		}
		source, err := builtins.ReadFile("stubs/" + e.Name())
		if err != nil {
			return err
		}
		if err := op.create(filepath.Join("stubs", "skyrix", e.Name()), source); err != nil {
			return err
		}
	}
	return nil
}
