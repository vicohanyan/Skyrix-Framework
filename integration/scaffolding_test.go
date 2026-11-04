package integration

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"
)

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
func writeFixture(t *testing.T, root, name, source string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestSeparateTemplateScaffolding(t *testing.T) {
	l := projectLayout(t)
	dir := t.TempDir()
	copyTree(t, l.template, dir, ".git", "logs", "secret", ".idea")
	// A different module path catches hardcoded Template imports in stubs/registrations.
	const module = "example.com/generated/service"
	files := snapshot(t, dir)
	for name, source := range files {
		if strings.HasSuffix(name, ".go") || name == "go.mod" {
			writeFixture(t, dir, name, strings.ReplaceAll(source, "gitlab.com/skyrix-lib/template", module))
		}
	}
	env := append(workspaceEnv(t, l, dir, false), "APP_ENV=fixture", "SERVICE_NAME=fixture", "LOG_OUTPUT=stdout", "LOG_LEVEL=error", "DB_HOST=127.0.0.1", "DB_PORT=1", "DB_USER=fixture", "DB_NAME=fixture", "REDIS_PORT=invalid", "JWT_EXPIRE=invalid")
	run := func(args ...string) string { t.Helper(); return goCommand(t, dir, env, args[0], args[1:]...) }
	binary := filepath.Join(t.TempDir(), "console")
	run("go", "build", "-o", binary, "./cmd/cobra")
	// Project overrides use the current exported contract, never legacy Deps imports.
	run(binary, "make", "stubs")
	for _, name := range []string{"command", "command_db", "seeder"} {
		path := filepath.Join(dir, "stubs/skyrix", name+".stub")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := strings.Replace(string(data), "import (", "import (\n \"fmt\"", 1)
		if name == "command" {
			source = strings.Replace(source, "return nil", `_, err := fmt.Fprintln(cmd.OutOrStdout(), cfg.ServiceName); return err`, 1)
		} else if name == "command_db" {
			source = strings.Replace(source, "return nil", `_, err := fmt.Fprintln(cmd.OutOrStdout(), deps.DB != nil); return err`, 1)
		} else {
			source = strings.Replace(source, "import (", "import (\n \"os\"", 1)
			source = strings.Replace(source, "return nil", `_, err := fmt.Fprintln(os.Stdout, "seed fixture executed"); return err`, 1)
		}
		writeFixture(t, dir, "stubs/skyrix/"+name+".stub", "// Project override: preserve me.\n"+source)
	}
	// Existing comments and independent files must survive AST registration.
	for _, name := range []string{"providers/commands.go", "providers/database.go"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, dir, name, "// User registration comment.\n"+string(data))
	}
	writeFixture(t, dir, "providers/user.go", "package providers\n// User code must stay byte-exact.\nconst userValue = 42\n")
	before := snapshot(t, dir)
	actions := [][]string{
		{"domain", "sales/order_flow"}, {"entity", "sales/order_flow/line_item"},
		{"repository", "sales/order_flow/line_item"}, {"service", "sales/order_flow/reconcile_batch"},
		{"uow", "sales/order_flow/CreateOrder"}, {"handler", "sales/order_flow/invoice_view"},
		{"command", "reconcile_accounts"}, {"command", "scan_invoices", "--db"},
		{"job", "refresh_balances"}, {"seeder", "currency_codes"},
	}
	for _, args := range actions {
		run(append([]string{binary, "make"}, args...)...)
	}
	after := snapshot(t, dir)
	for name, source := range before {
		if name == "providers/commands.go" || name == "providers/database.go" || name == "providers/domains.go" || name == "providers/handlers.go" || name == "providers/jobs.go" {
			continue
		}
		if after[name] != source {
			t.Fatalf("generator changed user file %s", name)
		}
	}
	for _, name := range []string{"providers/commands.go", "providers/database.go"} {
		if !strings.Contains(after[name], "// User registration comment.") {
			t.Fatalf("lost comment in %s", name)
		}
	}
	for _, name := range []string{"internal/commands/reconcileAccountsCommand.go", "internal/commands/scanInvoicesCommand.go", "database/seeders/currencyCodesSeeder.go"} {
		if !strings.Contains(after[name], "Project override: preserve me.") {
			t.Fatalf("project stub not used for %s", name)
		}
	}
	if after["tools/schemaloader/main.go"] != before["tools/schemaloader/main.go"] {
		t.Fatal("entity generation implicitly selected migration models")
	}
	if _, err := os.Stat(filepath.Join(dir, "internal/database")); err == nil {
		t.Fatal("legacy database directory created")
	}
	// Failure after preparing a generated command must publish neither file nor import.
	commands := after["providers/commands.go"]
	writeFixture(t, dir, "providers/commands.go", strings.Replace(commands, "func ProvideConsoleCommands", "func UserCommands", 1))
	failedBefore := snapshot(t, dir)
	cmd := exec.Command(binary, "make", "command", "must_rollback")
	cmd.Dir = dir
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("accepted missing registration: %s", output)
	}
	if !reflect.DeepEqual(failedBefore, snapshot(t, dir)) {
		t.Fatal("failed generation changed user files")
	}
	writeFixture(t, dir, "providers/commands.go", commands)
	// All selected graphs must still compile and Wire must be byte-repeatable.
	wire := strings.TrimSpace(run("go", "tool", "-n", "wire"))
	run(wire, "gen", "./cmd/http", "./cmd/cobra", "./cmd/worker")
	generated := snapshot(t, dir)
	run(wire, "gen", "./cmd/http", "./cmd/cobra", "./cmd/worker")
	repeated := snapshot(t, dir)
	for _, name := range []string{"http", "cobra", "worker"} {
		path := "cmd/" + name + "/wire_gen.go"
		if repeated[path] != generated[path] {
			t.Fatalf("Wire generation is not repeatable: %s", path)
		}
	}
	if !strings.Contains(generated["cmd/cobra/wire_gen.go"], "NewCurrencyCodesSeeder()") {
		t.Fatal("db seed graph lacks generated seeder")
	}
	run("go", "build", "./...")
	run("go", "build", "-o", binary, "./cmd/cobra")
	// No real DB is contacted: generated commands/seeders perform no queries and
	// the database pool is opened lazily without pinging the closed fixture port.
	if output := run(binary, "reconcile-accounts"); !strings.Contains(output, "fixture") {
		t.Fatal(output)
	}
	if output := run(binary, "scan-invoices"); !strings.Contains(output, "true") {
		t.Fatal(output)
	}
	if output := run(binary, "db", "seed"); strings.Count(output, "seed fixture executed") != 1 {
		t.Fatal(output)
	}
	writeFixture(t, dir, "cmd/cobra/registration_fixture_test.go", strings.ReplaceAll(registrationFixture, "MODULE", module))
	run("go", "test", "-count=1", "-run", "TestGeneratedGraphSelection", "./cmd/cobra")
	// Help/completion also work with invalid config, independent of both graphs.
	invalidEnv := append(append([]string{}, env...), "DB_PORT=invalid", "LOG_OUTPUT=invalid")
	for _, args := range [][]string{{"--help"}, {"scan-invoices", "--help"}, {"reconcile-accounts", "--help"}, {"db", "seed", "--help"}, {"completion", "bash"}, {"__complete", "scan-invoices", ""}} {
		goCommand(t, dir, invalidEnv, binary, args...)
	}
	// Exercise manual SQL lifecycle with an empty PATH: no Atlas CLI or server.
	manualEnv := append(append([]string{}, invalidEnv...), "PATH="+t.TempDir())
	goCommand(t, dir, manualEnv, binary, "make", "migration", "manual_one")
	sql, err := filepath.Glob(filepath.Join(dir, "database/migrations/*_manual_one.sql"))
	if err != nil || len(sql) != 1 {
		t.Fatal(sql, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "database/migrations/atlas.sum")); err != nil {
		t.Fatal(err)
	}
	firstSQL := filepath.Join(dir, "database/migrations/20000101000000_manual_one.sql")
	if err := os.Rename(sql[0], firstSQL); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "database/migrations/"+filepath.Base(firstSQL), "CREATE TABLE fixture (id bigint PRIMARY KEY);\n")
	goCommand(t, dir, manualEnv, binary, "db", "migrate:hash")
	goCommand(t, dir, manualEnv, binary, "make", "migration", "manual_two")
	secondSQL, err := filepath.Glob(filepath.Join(dir, "database/migrations/*_manual_two.sql"))
	if err != nil || len(secondSQL) != 1 {
		t.Fatal(secondSQL, err)
	}
	writeFixture(t, dir, "database/migrations/"+filepath.Base(secondSQL[0]), "SELECT 1;\n")
	goCommand(t, dir, manualEnv, binary, "db", "migrate:hash")
	// Rebuild after writing SQL: embedding must read the same directory as generators.
	run("go", "build", "-o", binary, "./cmd/cobra")
	writeFixture(t, dir, "cmd/cobra/migration_source_fixture_test.go", strings.ReplaceAll(migrationSourceFixture, "MODULE", module))
	run("go", "test", "-count=1", "-run", "TestGeneratedMigrationSource", "./cmd/cobra")
	cmd = exec.Command(binary, "db", "migrate:diff", "no_cli")
	cmd.Dir = dir
	cmd.Env = manualEnv
	beforeDiff := snapshot(t, dir)
	if output, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("atlas CLI is not installed")) {
		t.Fatalf("missing Atlas: %v\n%s", err, output)
	}
	if !reflect.DeepEqual(beforeDiff, snapshot(t, dir)) {
		t.Fatal("missing Atlas modified project")
	}
	if _, err := os.Stat(filepath.Join(dir, "migrations")); !os.IsNotExist(err) {
		t.Fatal("legacy migrations directory used")
	}
}

const registrationFixture = `package main
import (
 "context"
 "errors"
 "io"
 "strings"
 "testing"
 "gitlab.com/skyrix-lib/framework/cobra/console"
 "gitlab.com/skyrix-lib/framework/database/dbcmd"
)
func TestGeneratedGraphSelection(t *testing.T) {
 t.Setenv("SERVICE_NAME", "fixture")
 makeGraphs := func() (graphs, *int, *int, *int) {
  app, database, closes := new(int), new(int), new(int)
  g:=graphs{
   App:console.NewGraph(func()(*appGraph,func(),error){*app++; a,close,err:=buildApp(); return a,func(){*closes++;if close!=nil{close()}},err}),
   Database:console.NewGraph(func()(*dbcmd.Deps,func(),error){*database++; d,close,err:=buildDatabase();return d,func(){*closes++;if close!=nil{close()}},err}),
  }
  return g,app,database,closes
 }
 for _, args:=range [][]string{{"--help"},{"scan-invoices","--help"},{"completion","bash"},{"__complete","reconcile-accounts",""}} {
  g,a,d,c:=makeGraphs(); defer g.Close()
  if err:=newConsole(g).Execute(context.Background(),args,io.Discard,io.Discard);err!=nil{t.Fatal(err)}
  g.Close();if *a!=0||*d!=0||*c!=0{t.Fatal("help/completion built graph")}
 }
 for _, name:=range []string{"reconcile-accounts","scan-invoices"} {
  g,a,d,c:=makeGraphs()
  err:=newConsole(g).Execute(context.Background(),[]string{name},io.Discard,io.Discard)
  g.Close();g.Close();if err!=nil{t.Fatal(err)}
  if (name=="reconcile-accounts"&&(*a!=1||*d!=0))||(name=="scan-invoices"&&(*a!=0||*d!=1))||*c!=1{t.Fatalf("wrong graph for %s",name)}
 }
 g,a,d,_:=makeGraphs(); failure:=errors.New("fixture build failure"); closes:=0
 g.App=console.NewGraph(func()(*appGraph,func(),error){return nil,func(){closes++},failure})
 for i:=0;i<2;i++ {
  err:=newConsole(g).Execute(context.Background(),[]string{"reconcile-accounts"},io.Discard,io.Discard)
  if !errors.Is(err,failure)||!strings.Contains(err.Error(),"build command dependencies"){t.Fatal(err)}
 }
 g.Close();if closes!=1||*a!=0||*d!=0{t.Fatal("failed graph cleanup was not exactly once")}
}
`
const migrationSourceFixture = `package main
import (
 "testing"
 "strings"
 "gitlab.com/skyrix-lib/framework/database/migrate"
 "MODULE/providers"
)
func TestGeneratedMigrationSource(t *testing.T){
 files,err:=migrate.Load(providers.ProvideMigrationSource());if err!=nil{t.Fatal(err)}
 if len(files)!=2||!strings.Contains(string(files[0].SQL),"CREATE TABLE fixture"){t.Fatalf("wrong embedded SQL: %+v",files)}
}
`

// Ambiguous provider shapes must fail before any generated output is published.
func TestProviderRegistrationFailuresPreserveFiles(t *testing.T) {
	l := projectLayout(t)
	for _, kind := range []string{"command", "command-db", "seeder"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			copyTree(t, l.template, root, ".git", "logs", "secret", ".idea")
			path := "providers/commands.go"
			source := "package providers\nfunc ProvideConsoleCommands() {}\n"
			args := []string{"command", "ambiguous"}
			if kind == "command-db" {
				args = append(args, "--db")
			}
			if kind == "seeder" {
				path = "providers/database.go"
				source = "package providers\nfunc Seeders() any { return userSeeders() }\n"
				args = []string{"seeder", "ambiguous"}
			}
			writeFixture(t, root, path, source)
			before := snapshot(t, root)
			maker := makecmd.NewMakeCommand()
			maker.ProjectRoot = root
			command := maker.Command()
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatal("accepted ambiguous registry")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("partial generation changed user files")
			}
		})
	}
}
