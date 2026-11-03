package makecmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
)

const fixtureModule = "code.example.dev/finance/ledger/v2"

var allActions = [][]string{
	{"domain", "sales/order_flow"},
	{"entity", "sales/order_flow/line_item"},
	{"repository", "sales/order_flow/line_item"},
	{"service", "sales/order_flow/reconcile_batch"},
	{"handler", "sales/order_flow/invoice_view"},
	{"command", "reconcile_accounts"},
	{"command", "scan_invoices", "--db"},
	{"job", "refresh_balances"},
	{"seeder", "currency_codes"},
	{"seeder", "tax_codes"},
	{"stubs"},
	{"migration", "custom_trigger"},
}

func writeProject(t *testing.T, root, name, source string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
}
func testMaker(root string) *MakeCommand {
	return &MakeCommand{ProjectRoot: root, Now: func() time.Time { return time.Date(2026, 11, 3, 10, 20, 30, 0, time.UTC) }, LookupAtlas: func() (string, error) { return "", errors.New("atlas not installed") }}
}
func registrationProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module " + fixtureModule + "\n\ngo 1.27\n",
		"providers/domains.go": `// User domain composition.
package providers // package comment
import di "github.com/goforj/wire"
// UserSet stays untouched.
var UserSet = di.NewSet()
var DomainProviderSet = di.NewSet(UserSet /* keep domain */)
var ConsoleDomainProviderSet = di.NewSet(
 UserSet, // keep console domain
)
`,
		"providers/handlers.go": `package providers
import "github.com/goforj/wire"
// Handlers retains user fields.
type Handlers struct { User *UserHandler /* user field */ }
type UserHandler struct{}
func NewUserHandler()*UserHandler{return &UserHandler{}}
var HandlerProviderSet=wire.NewSet(NewUserHandler,wire.Struct(new(Handlers),"*"))
`,
		"providers/jobs.go": `package providers
import (
 "github.com/goforj/wire"
 engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
 customJobs "` + fixtureModule + `/internal/jobs"
)
type Jobs struct { UserJob *customJobs.UserJob /* keep job */ }
func ProvideRegisteredJobsRegistry(reg *engineJobs.RuntimeRegistry, all *Jobs) engineJobs.Registry {
 // keep registry comment
 reg.Register(all.UserJob) // user registration
 return reg
}
var JobProviderSet=wire.NewSet(engineJobs.NewRegistry,customJobs.NewUserJob,wire.Struct(new(Jobs),"*"),ProvideRegisteredJobsRegistry)
var ConsoleJobProviderSet=wire.NewSet(engineJobs.NewRegistry,customJobs.NewUserJob,wire.Struct(new(Jobs),"*"),ProvideRegisteredJobsRegistry)
`,
		"internal/commands/commands.go": `// User console descriptions.
package commands
import (
 "gitlab.com/skyrix-lib/framework/cobra/console"
 "gitlab.com/skyrix-lib/framework/database/dbcmd"
)
type Deps struct{}
func Hello(deps *console.Graph[*Deps])*console.Command{return &console.Command{Use:"hello"}}
func All(app *console.Graph[*Deps]) []*console.Command {
 return []*console.Command{Hello(app) /* keep command */}
}
func DatabaseAll(database *console.Graph[*dbcmd.Deps]) []*console.Command {
 return []*console.Command{}
}
`,
		"internal/database/database.go": `package database
import (
 "github.com/goforj/wire"
 "gitlab.com/skyrix-lib/framework/database/dbcmd"
 customSeeds "` + fixtureModule + `/internal/database/seeders"
)
type UserModel struct{}
// Models keeps explicit user registrations.
func Models() dbcmd.Models {return dbcmd.Models{&UserModel{} /* user model */}}
func Seeders(user *customSeeds.UserSeeder /* comma, in a comment */) dbcmd.Seeders {
 return dbcmd.Seeders{user /* user seeder */}
}
var ProviderSet=wire.NewSet(Models,Seeders,customSeeds.NewUserSeeder)
`,
		"internal/database/seeders/user.go": "package seeders\ntype UserSeeder struct{}\nfunc NewUserSeeder()*UserSeeder{return &UserSeeder{}}\n",
	}
	for name, source := range files {
		writeProject(t, root, name, source)
	}
	return root
}
func projectSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func assertUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := projectSnapshot(t, root)
	if len(after) != len(before) {
		t.Fatalf("partial files remain: before=%d after=%d", len(before), len(after))
	}
	for path, want := range before {
		if after[path] != want {
			t.Fatalf("changed %s on error", path)
		}
	}
}
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch %s; got:\n%s", path, got)
	}
}
func TestAllGeneratorsGolden(t *testing.T) {
	root := registrationProject(t)
	maker := testMaker(root)
	for _, args := range allActions {
		if _, err := executeScaffoldCommand(maker.Command().Cobra(), args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	snapshot := projectSnapshot(t, root)
	paths := make([]string, 0, len(snapshot))
	for path := range snapshot {
		if !strings.HasPrefix(path, "stubs/") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	var result strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&result, "-- %s --\n%s\n", path, snapshot[path])
	}
	golden(t, "all", result.String())
	// A second operation conflicts without touching any existing or generated file.
	for _, args := range allActions {
		if _, err := executeScaffoldCommand(maker.Command().Cobra(), args...); err == nil {
			t.Fatalf("repeat accepted %v", args)
		}
		assertUnchanged(t, root, snapshot)
	}
}
func TestUnsupportedRegistrationIsAtomic(t *testing.T) {
	for _, test := range []struct{ kind, raw, file, bad string }{
		{"domain", "billing", "providers/domains.go", "package providers\nvar DomainProviderSet = custom()\n"},
		{"service", "sales/reconcile", "internal/domain/sales/provider.go", "package sales\nvar ProviderSet = custom()\n"},
		{"entity", "sales/invoice", "internal/database/database.go", "package database\nfunc Models() any {if true{return nil};return nil}\n"},
		{"handler", "sales/view", "providers/handlers.go", "package providers\ntype Handlers = map[string]any\n"},
		{"command", "sync", "internal/commands/commands.go", "package commands\nfunc All(){ println(1) }\n"},
		{"job", "refresh", "providers/jobs.go", "package providers\nfunc ProvideRegisteredJobsRegistry(){if true{return}}\n"},
		{"seeder", "currencies", "internal/database/database.go", "package database\nfunc Seeders() any {return custom()}\n"},
	} {
		t.Run(test.kind, func(t *testing.T) {
			root := registrationProject(t)
			maker := testMaker(root)
			if _, err := maker.generate("domain", "sales", false); err != nil {
				t.Fatal(err)
			}
			writeProject(t, root, test.file, test.bad)
			before := projectSnapshot(t, root)
			if _, err := maker.generate(test.kind, test.raw, false); err == nil {
				t.Fatal("unsupported registration accepted")
			}
			assertUnchanged(t, root, before)
		})
	}
}
func TestProjectStubs(t *testing.T) {
	root := registrationProject(t)
	maker := testMaker(root)
	if _, err := maker.generate("domain", "sales", false); err != nil {
		t.Fatal(err)
	}
	if _, err := maker.generate("stubs", "", false); err != nil {
		t.Fatal(err)
	}
	entries, _ := builtins.ReadDir("stubs")
	for _, entry := range entries {
		want, _ := builtins.ReadFile("stubs/" + entry.Name())
		got, err := os.ReadFile(filepath.Join(root, "stubs/skyrix", entry.Name()))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("stub export differs")
		}
	}
	writeProject(t, root, "stubs/skyrix/service.stub", `package {{.Package}}
// Project override for {{.Module}}.
type {{.Type}}Service struct{ Project bool }
func New{{.Type}}Service()*{{.Type}}Service{return &{{.Type}}Service{Project:true}}
`)
	if _, err := maker.generate("service", "sales/custom_batch", false); err != nil {
		t.Fatal(err)
	}
	source := readTestFile(t, filepath.Join(root, "internal/domain/sales/services/customBatchService.go"))
	if !strings.Contains(source, "Project: true") || !strings.Contains(source, fixtureModule) {
		t.Fatal(source)
	}
	before := projectSnapshot(t, root)
	if _, err := maker.generate("stubs", "", false); err == nil {
		t.Fatal("overwrote project stubs")
	}
	assertUnchanged(t, root, before)
	for _, source := range []string{"{{.Unknown}}", "package wrong {", "package services\ntype Unrelated struct{}"} {
		writeProject(t, root, "stubs/skyrix/service.stub", source)
		before = projectSnapshot(t, root)
		if _, err := maker.generate("service", "sales/fail", false); err == nil {
			t.Fatal("bad stub accepted")
		}
		assertUnchanged(t, root, before)
	}
}
func TestNamesAndPathSafety(t *testing.T) {
	root := registrationProject(t)
	maker := testMaker(root)
	before := projectSnapshot(t, root)
	for _, raw := range []string{"../escape", "/escape", "sales//order", "sales/../order", "sales/.hidden", "sales/1order", "sales/order_", "sales/order--flow", "sales/type", "sales/main", "sales/init", "sales/Ünicode", "sales/has space", " sales"} {
		if _, err := maker.generate("domain", raw, false); err == nil {
			t.Fatalf("accepted %q", raw)
		}
		assertUnchanged(t, root, before)
	}
	maker.DomainDir = "../outside"
	if _, err := maker.generate("domain", "escape", false); err == nil {
		t.Fatal("DomainDir escaped root")
	}
	assertUnchanged(t, root, before)
	maker.DomainDir = ""
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "internal/domain")); err != nil {
		t.Fatal(err)
	}
	if _, err := maker.generate("domain", "escape", false); err == nil {
		t.Fatal("followed symlink")
	}
	entries, _ := os.ReadDir(external)
	if len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}
func TestOperationRollsBackWriteFailure(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, "a.go", "original user contents")
	if err := os.Chmod(filepath.Join(root, "a.go"), 0600); err != nil {
		t.Fatal(err)
	}
	op, err := newOperation(root)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := op.load("a.go")
	if err != nil {
		t.Fatal(err)
	}
	ch.after = []byte("modified")
	if err := op.create("new/nested/b.txt", []byte("created")); err != nil {
		t.Fatal(err)
	}
	if err := op.create("new/nested/c.txt", []byte("fail")); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fail := errors.New("injected write failure")
	err = op.applyWith(func(temp, path string, exists bool) error {
		calls++
		if calls == 3 {
			return fail
		}
		return publishFile(temp, path, exists)
	})
	if !errors.Is(err, fail) {
		t.Fatalf("%v", err)
	}
	files := projectSnapshot(t, root)
	if len(files) != 1 || files["a.go"] != "original user contents" {
		t.Fatal(files)
	}
	info, _ := os.Stat(filepath.Join(root, "a.go"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("permissions lost")
	}
	if _, err := os.Stat(filepath.Join(root, "new")); !os.IsNotExist(err) {
		t.Fatal("own directories remained")
	}
}
func TestMakeHelpNeedsNoProject(t *testing.T) {
	t.Setenv("JWT_EXPIRE", "invalid")
	t.Setenv("DB_PORT", "invalid")
	maker := testMaker(filepath.Join(t.TempDir(), "missing"))
	for _, args := range [][]string{{"--help"}, {"domain", "--help"}, {"entity", "--help"}, {"repository", "--help"}, {"service", "--help"}, {"handler", "--help"}, {"command", "--db", "--help"}, {"job", "--help"}, {"seeder", "--help"}, {"stubs", "--help"}, {"migration", "--help"}, {"__complete", "make", ""}, {"completion", "bash"}} {
		if args[0] != "--help" && args[0] != "completion" && args[0] != "__complete" {
			args = append([]string{"make"}, args...)
		}
		app := console.New(maker.Command())
		var out strings.Builder
		if err := app.Execute(context.Background(), args, &out, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

// Build actual Template sources after generation under another module path.
// Wire is the tool already declared by Template; this test installs nothing.
func TestGeneratedTemplateBuildsAndWire(t *testing.T) {
	root, framework := copyTemplate(t)
	maker := testMaker(root)
	for _, args := range allActions {
		if _, err := executeScaffoldCommand(maker.Command().Cobra(), args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	run := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly -buildvcs=false")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	// Wire graphs and generated injectors must stay unchanged until explicit Wire.
	original := readTestFile(t, filepath.Join(framework, "template/cmd/cobra/wire.go"))
	got := readTestFile(t, filepath.Join(root, "cmd/cobra/wire.go"))
	if got != strings.ReplaceAll(original, "gitlab.com/skyrix-lib/template", fixtureModule) {
		t.Fatal("generator edited Wire source")
	}
	tool := strings.TrimSpace(run("go", "tool", "-n", "wire"))
	run(tool, "gen", "./cmd/http", "./cmd/cobra", "./cmd/worker")
	before := map[string]string{}
	for _, name := range []string{"http", "cobra", "worker"} {
		path := "cmd/" + name + "/wire_gen.go"
		before[path] = readTestFile(t, filepath.Join(root, path))
	}
	run(tool, "gen", "./cmd/http", "./cmd/cobra", "./cmd/worker")
	for path, want := range before {
		if readTestFile(t, filepath.Join(root, path)) != want {
			t.Fatalf("Wire not repeatable: %s", path)
		}
	}
	run("go", "build", "./...")
	writeProject(t, root, "internal/commands/generated_test.go", `package commands
import (
 "context"
 "io"
 "testing"
 "gitlab.com/skyrix-lib/framework/cobra/console"
 "gitlab.com/skyrix-lib/framework/database/dbcmd"
)
func TestGeneratedCommandsAreLazy(t *testing.T){
 builds,closes:=0,0
 appGraph:=console.NewGraph(console.NeedsApp,func()(*Deps,func(),error){builds++;return &Deps{},func(){closes++},nil})
 dbGraph:=console.NewGraph(console.NeedsDatabase,func()(*dbcmd.Deps,func(),error){builds++;return &dbcmd.Deps{},func(){closes++},nil})
 for _,args:=range [][]string{{"--help"},{"scan-invoices","--help"},{"completion","bash"},{"__complete","scan-invoices",""}}{
  app:=console.New(ReconcileAccounts(appGraph),ScanInvoices(dbGraph))
  if err:=app.Execute(context.Background(),args,io.Discard,io.Discard);err!=nil{t.Fatal(err)}
  if builds!=0{t.Fatal("help/completion built graph")}
 }
 app:=console.New(ReconcileAccounts(appGraph),ScanInvoices(dbGraph))
 if err:=app.Execute(context.Background(),[]string{"scan-invoices"},io.Discard,io.Discard);err!=nil{t.Fatal(err)}
 if builds!=1||appGraph.Built()||!dbGraph.Built(){t.Fatal("wrong graph selected")}
 dbGraph.Close();appGraph.Close();if closes!=1{t.Fatal("cleanup not called")}
}
`)
	run("go", "test", "-count=1", "./internal/commands", "./internal/database/...", "./internal/jobs", "./internal/handlers", "./internal/domain/sales/...")
	binary := filepath.Join(root, "console")
	run("go", "build", "-o", binary, "./cmd/cobra")
	jobCmd := exec.Command(binary, "jobs:run", "--name", "refresh.balances")
	jobCmd.Dir = root
	jobCmd.Env = append(os.Environ(), "DB_PORT=invalid", "REDIS_PORT=invalid", "JWT_EXPIRE=invalid", "LOG_OUTPUT=stdout")
	if out, err := jobCmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "job succeeded") {
		t.Fatalf("generated job not wired: %v\n%s", err, out)
	}

	for _, args := range [][]string{{"--help"}, {"scan-invoices", "--help"}, {"reconcile-accounts", "--help"}, {"completion", "bash"}, {"__complete", "scan-invoices", ""}, {"make", "job", "--help"}} {
		cmd := exec.Command(binary, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "JWT_EXPIRE=invalid", "DB_PORT=invalid", "REDIS_PORT=invalid", "LOG_OUTPUT=invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generated help %v: %v\n%s", args, err, out)
		}
	}
}
func copyTemplate(t *testing.T) (string, string) {
	t.Helper()
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(framework, "template")
	root := t.TempDir()
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(source, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".go") || relative == "go.mod" {
			data = []byte(strings.ReplaceAll(string(data), "gitlab.com/skyrix-lib/template", fixtureModule))
		}
		if relative == "go.mod" {
			data = []byte(strings.ReplaceAll(string(data), "replace gitlab.com/skyrix-lib/framework => ..", "replace gitlab.com/skyrix-lib/framework => "+framework))
		}
		writeProject(t, root, relative, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, framework
}

func TestASTRegistrationIsIdempotent(t *testing.T) {
	root := registrationProject(t)
	op, err := newOperation(root)
	if err != nil {
		t.Fatal(err)
	}
	edit := func(e *editor) error {
		alias, err := e.importName(fixtureModule+"/internal/domain/sales", "sales")
		if err != nil {
			return err
		}
		return e.addSet("DomainProviderSet", alias+".ProviderSet")
	}
	if err := op.edit("providers/domains.go", edit); err != nil {
		t.Fatal(err)
	}
	first := string(op.files["providers/domains.go"].after)
	if err := op.edit("providers/domains.go", edit); err != nil {
		t.Fatal(err)
	}
	if string(op.files["providers/domains.go"].after) != first {
		t.Fatal("duplicate imports or providers")
	}
	golden(t, "ast-domains", first)
	// A generated type name in a differently named user file is still a conflict.
	writeProject(t, root, "internal/commands/user.go", "package commands\nfunc Reserved(){}\n")
	before := projectSnapshot(t, root)
	if _, err := testMaker(root).generate("command", "reserved", false); err == nil {
		t.Fatal("accepted symbol conflict")
	}
	assertUnchanged(t, root, before)
}
func TestMigrationHashFailureRollsBack(t *testing.T) {
	for _, existing := range []bool{false, true} {
		root := registrationProject(t)
		maker := testMaker(root)
		if existing {
			writeProject(t, root, "migrations/atlas.sum", "user checksum\n")
		}
		binary := filepath.Join(t.TempDir(), "atlas")
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'changed checksum' > migrations/atlas.sum\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
		maker.LookupAtlas = func() (string, error) { return binary, nil }
		before := projectSnapshot(t, root)
		if _, err := executeScaffoldCommand(maker.Command().Cobra(), "migration", "test_hash"); err == nil {
			t.Fatal("hash failure ignored")
		}
		assertUnchanged(t, root, before)
	}
}
func TestQuotedModulePath(t *testing.T) {
	root := registrationProject(t)
	writeProject(t, root, "go.mod", "module \""+fixtureModule+"\" // project module\n\ngo 1.27\n")
	if _, err := testMaker(root).generate("domain", "billing", false); err != nil {
		t.Fatal(err)
	}
	if source := readTestFile(t, filepath.Join(root, "providers/domains.go")); !strings.Contains(source, fixtureModule+"/internal/domain/billing") {
		t.Fatal(source)
	}
}
