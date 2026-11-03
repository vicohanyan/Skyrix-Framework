package makecmd

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	support "gitlab.com/skyrix-lib/framework/scaffold"
)

var segmentPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*([_-][A-Za-z][A-Za-z0-9]*)*$`)

type selection struct {
	domain, domainType string
	name               support.Name
	use                string
}

func parseSelection(kind, raw string) (selection, error) {
	var out selection
	parts := strings.Split(raw, "/")
	if kind != "domain" && kind != "entity" && kind != "repository" && kind != "service" && kind != "handler" && len(parts) != 1 {
		return out, fmt.Errorf("%s accepts one name (no slashes)", kind)
	}
	var names []support.Name
	for _, part := range parts {
		if !segmentPattern.MatchString(part) {
			return out, fmt.Errorf("invalid name %q: use ASCII letter-led segments, '/' for domain paths, '-' or '_' for compound names", raw)
		}
		name, err := support.NormalizeName(part)
		if err != nil {
			return out, err
		}
		if (kind == "domain" || len(names) < len(parts)-1) && (name.Package == "main" || name.Package == "init") {
			return out, fmt.Errorf("domain package %q is reserved", part)
		}
		names = append(names, name)
	}
	out.name = names[len(names)-1]
	out.use = strings.ToLower(strings.NewReplacer("_", "-", "/", "-").Replace(parts[len(parts)-1]))
	if kind == "domain" {
		for _, name := range names {
			out.domainType += name.Type
		}
		for i, name := range names {
			parts[i] = name.Package
		}
		out.domain = strings.Join(parts, "/")
	} else if kind == "entity" || kind == "repository" || kind == "service" || kind == "handler" {
		if len(parts) < 2 {
			return out, fmt.Errorf("%s needs <domain-path>/<name>, for example example/invoice", kind)
		}
		var path []string
		for _, name := range names[:len(names)-1] {
			path = append(path, name.Package)
			out.domainType += name.Type
		}
		out.domain = strings.Join(path, "/")
	}
	return out, nil
}
func lowerFirst(s string) string { r := []rune(s); r[0] = unicode.ToLower(r[0]); return string(r) }
func (c *MakeCommand) generate(kind, raw string, db bool) (string, error) {
	root, err := support.ProjectRoot(c.ProjectRoot)
	if err != nil {
		return "", err
	}
	op, err := newOperation(root)
	if err != nil {
		return "", err
	}
	if kind == "stubs" {
		if err := exportStubs(op); err != nil {
			return "", err
		}
		if err := op.apply(); err != nil {
			return "", err
		}
		return "stubs exported: stubs/skyrix (existing files are never overwritten)", nil
	}
	selected, err := parseSelection(kind, raw)
	if err != nil {
		return "", err
	}
	module, err := support.ModulePath(root)
	if err != nil {
		return "", err
	}
	domainDir := c.DomainDir
	if domainDir == "" {
		domainDir = DefaultDomainDir
	}
	if _, err := op.path(domainDir); err != nil {
		return "", err
	}
	domainPath := filepath.Join(domainDir, filepath.FromSlash(selected.domain))
	domainImport := module + "/" + filepath.ToSlash(domainPath)
	data := stubData{Module: module, DomainImport: domainImport, Package: selected.name.Package, Type: selected.name.Type, Var: lowerFirst(selected.name.Type), Use: selected.use, JobName: strings.ReplaceAll(selected.use, "-", ".")}
	if selected.domain != "" && kind != "domain" {
		if _, err := op.load(filepath.Join(domainPath, "provider.go")); err != nil {
			return "", fmt.Errorf("domain %s is unavailable; run make domain %s first: %w", selected.domain, selected.domain, err)
		}
	}
	switch kind {
	case "entity":
		data.Package = "entity"
	case "repository":
		data.Package = "repository"
	case "service":
		data.Package = "services"
	case "handler":
		data.Package = "handlers"
	case "command":
		data.Package = "commands"
	case "job":
		data.Package = "jobs"
	case "seeder":
		data.Package = "seeders"
	}
	if selected.domain == "" {
		data.DomainImport = ""
	}
	create := func(stub, path string, required ...string) error {
		content, err := renderStub(op, stub, data, true)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, content, parser.ParseComments)
		if err != nil {
			return err
		}
		names := topNames(file)
		for _, symbol := range required {
			if !names[symbol] {
				return fmt.Errorf("stub %s must declare %s", stub, symbol)
			}
		}
		return op.create(path, content)
	}
	switch kind {
	case "domain":
		path, err := op.path(domainPath)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("domain already exists: %s", selected.domain)
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := create("domain", filepath.Join(domainPath, "provider.go"), "ProviderSet"); err != nil {
			return "", err
		}
		for _, sub := range []string{"dto", "entity", "interfaces", "repository", "services", "unitOfWork"} {
			data.Package = strings.ToLower(sub)
			if err := create("doc", filepath.Join(domainPath, sub, "doc.go")); err != nil {
				return "", err
			}
		}
		if err := op.edit("providers/domains.go", func(e *editor) error {
			alias, err := e.importName(domainImport, strings.ToLower(selected.domainType))
			if err != nil {
				return err
			}
			for _, set := range []string{"DomainProviderSet", "ConsoleDomainProviderSet"} {
				if err := e.addSet(set, alias+".ProviderSet"); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return "", err
		}
	case "repository", "service":
		sub, suffix := "repository", "Repository"
		if kind == "service" {
			sub, suffix = "services", "Service"
		}
		if err := create(kind, filepath.Join(domainPath, sub, data.Var+suffix+".go"), data.Type+suffix, "New"+data.Type+suffix); err != nil {
			return "", err
		}
		if err := op.edit(filepath.Join(domainPath, "provider.go"), func(e *editor) error {
			alias, err := e.importName(domainImport+"/"+sub, sub)
			if err != nil {
				return err
			}
			return e.addSet("ProviderSet", alias+".New"+data.Type+suffix)
		}); err != nil {
			return "", err
		}
	case "entity":
		if err := create(kind, filepath.Join(domainPath, "entity", data.Var+".go"), data.Type, "New"+data.Type); err != nil {
			return "", err
		}
		if err := op.edit("internal/database/database.go", func(e *editor) error {
			alias, err := e.importName(domainImport+"/entity", strings.ToLower(selected.domainType)+"Entity")
			if err != nil {
				return err
			}
			return e.addLiteral("Models", "&"+alias+"."+data.Type+"{}", "models")
		}); err != nil {
			return "", err
		}
	case "handler":
		data.Type = selected.domainType + data.Type
		data.Var = lowerFirst(data.Type)
		if err := create(kind, filepath.Join("internal/handlers", data.Var+"Handler.go"), data.Type+"Handler", "New"+data.Type+"Handler"); err != nil {
			return "", err
		}
		if err := op.edit("providers/handlers.go", func(e *editor) error {
			if err := e.requireBundle("HandlerProviderSet", "Handlers"); err != nil {
				return err
			}
			alias, err := e.importName(module+"/internal/handlers", "handlers")
			if err != nil {
				return err
			}
			if err := e.addField("Handlers", data.Type, "*"+alias+"."+data.Type+"Handler"); err != nil {
				return err
			}
			return e.addSet("HandlerProviderSet", alias+".New"+data.Type+"Handler")
		}); err != nil {
			return "", err
		}
	case "command":
		stub, list := "command", "All"
		if db {
			stub, list = "command_db", "DatabaseAll"
		}
		if err := create(stub, filepath.Join("internal/commands", data.Var+"Command.go"), data.Type); err != nil {
			return "", err
		}
		if err := op.edit("internal/commands/commands.go", func(e *editor) error {
			param, err := e.commandParam(list, db)
			if err != nil {
				return err
			}
			return e.addLiteral(list, data.Type+"("+param+")", "commands")
		}); err != nil {
			return "", err
		}
	case "job":
		if err := create(kind, filepath.Join("internal/jobs", data.Var+"Job.go"), data.Type+"Job", "New"+data.Type+"Job"); err != nil {
			return "", err
		}
		if err := op.edit("providers/jobs.go", func(e *editor) error {
			for _, set := range []string{"JobProviderSet", "ConsoleJobProviderSet"} {
				if err := e.requireBundle(set, "Jobs"); err != nil {
					return err
				}
			}
			alias, err := e.importName(module+"/internal/jobs", "jobs")
			if err != nil {
				return err
			}
			if err := e.addField("Jobs", data.Type+"Job", "*"+alias+"."+data.Type+"Job"); err != nil {
				return err
			}
			for _, set := range []string{"JobProviderSet", "ConsoleJobProviderSet"} {
				if err := e.addSet(set, alias+".New"+data.Type+"Job"); err != nil {
					return err
				}
			}
			return e.addJobRegistration(data.Type + "Job")
		}); err != nil {
			return "", err
		}
	case "seeder":
		if err := create(kind, filepath.Join("internal/database/seeders", data.Var+"Seeder.go"), data.Type+"Seeder", "New"+data.Type+"Seeder"); err != nil {
			return "", err
		}
		if err := op.edit("internal/database/database.go", func(e *editor) error {
			alias, err := e.importName(module+"/internal/database/seeders", "seeders")
			if err != nil {
				return err
			}
			if err := e.addSet("ProviderSet", alias+".New"+data.Type+"Seeder"); err != nil {
				return err
			}
			return e.addSeeder(data.Var, "*"+alias+"."+data.Type+"Seeder")
		}); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown generator %q", kind)
	}
	if err := op.apply(); err != nil {
		return "", err
	}
	message := fmt.Sprintf("%s created: %s", kind, raw)
	if kind != "command" {
		message += "\nRun Wire explicitly when these sets are selected: go tool wire gen ./cmd/http ./cmd/cobra ./cmd/worker"
	}
	if kind == "handler" {
		message += "\nImplement the handler and add its route explicitly."
	}
	if kind == "entity" {
		message += "\nModel registered; no migration or schema change was applied."
	}
	return message, nil
}
