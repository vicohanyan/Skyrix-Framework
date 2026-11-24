package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise real platformgen + Wire with two declarations in the same providers
// package. Imports are shared intentionally; construction and lifecycle are not.
// The fixture reuses the Template's go.mod, internal/config and internal/providers/auth.go
// (with the external Auth library) under another module path.
func TestPlatformDriverSelection(t *testing.T) {
	l := projectLayout(t)
	requireLibrary(t, l.authLib, "gitlab.com/skyrix-lib/auth", "SKYRIX_AUTH_LIB")
	template := l.template
	dir := t.TempDir()
	env := append(workspaceEnv(t, l, dir, false), "PLATFORM_FIRST_ENABLED=false", "PLATFORM_SECOND_ENABLED=false")
	run := func(workdir string, extra []string, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = workdir
		cmd.Env = append(append([]string{}, env...), extra...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, output)
		}
		return string(output)
	}
	write := func(name, source string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Keep the fixture on the repository's existing versions, without new deps.
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(template, name))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		if name == "go.mod" {
			source = strings.Replace(source, "module gitlab.com/skyrix-lib/template", "module example.com/selection", 1)
		}
		write(name, source)
	}
	write("internal/probe/probe.go", `package probe
var AppConfig, Constructed, Components, Started int
var Selected string
`)
	configSource, err := os.ReadFile(filepath.Join(template, "internal/config/config.go"))
	if err != nil {
		t.Fatal(err)
	}
	write("internal/config/config.go", string(configSource))
	write("internal/config/probe.go", `package config
import "example.com/selection/internal/probe"
func New() (*Config,error) {probe.AppConfig++;return LoadConfig()}
`)
	driver := `package platform
import (
 "context"
 "github.com/goforj/wire"
 "example.com/selection/internal/probe"
 frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
 "github.com/spf13/cobra"
 "fmt"
 "gitlab.com/skyrix-lib/framework/kernel"
)
type Config struct { Enabled bool ` + "`yaml:\"enabled\" env:\"PLATFORM_{{NAME}}_ENABLED\"`" + ` }
func NewConfig() *Config { return &Config{Enabled:true} }
func (c *Config) Validate() error {if c.Enabled{return fmt.Errorf("fixture expects YAML/env override")};return nil}
type Module struct{}
var ProviderSet=wire.NewSet(New)
func New(cfg *Config)(*Module,error) {
 probe.Constructed++;probe.Selected="{{name}}"
 return &Module{},nil
}
func (*Module) Name() string {return "{{name}}"}
func (*Module) Components() []kernel.Component {probe.Components++;return []kernel.Component{component{}}}
func (*Module) Commands() []frameworkcobra.CommandFactory {
 return []frameworkcobra.CommandFactory{func()*cobra.Command{return &cobra.Command{Use:"{{name}}",Short:"Fixture command"}}}
}
type component struct{}
func (component) Start(context.Context) error {probe.Started++;return nil}
func (component) Stop(context.Context) error {return nil}
`
	for _, name := range []string{"first", "second"} {
		write(name+"/platform/module.go", strings.ReplaceAll(strings.ReplaceAll(driver, "{{name}}", name), "{{NAME}}", strings.ToUpper(name)))
		write(name+"/platform/manifest.yaml", "name: "+name+"\nconfig_key: platform."+name+"\nconfig_example: |\n  enabled: false\n")
	}
	write("internal/providers/platform.go", `package providers
import (
 first "example.com/selection/first/platform"
 second "example.com/selection/second/platform"
 frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
 "gitlab.com/skyrix-lib/framework/kernel"
 "github.com/spf13/cobra"
)
//skyrix:platform example.com/selection/first/platform
//skyrix:platform example.com/selection/second/platform
func ProvideFirstPlatform(module *first.Module)(*kernel.Platform,error){return kernel.NewPlatform(module)}
func ProvideSecondPlatform(module *second.Module)(*kernel.Platform,error){return kernel.NewPlatform(module)}
func ConsoleCommands() []*cobra.Command{return frameworkcobra.ModuleCommands(new(first.Module))}
`)
	for _, name := range []string{"first", "second"} {
		title := strings.ToUpper(name[:1]) + name[1:]
		write("cmd/"+name+"/wire.go", `//go:build wireinject

package main
import (
 "github.com/goforj/wire"
 "gitlab.com/skyrix-lib/framework/kernel"
 "example.com/selection/internal/config"
 "example.com/selection/internal/providers"
)
func build()(*kernel.Platform,error){
 wire.Build(config.New,providers.`+title+`platformSet,providers.Provide`+title+`Platform)
 return nil,nil
}
`)
		write("cmd/"+name+"/main.go", `package main
import (
 "context"
 "os"
 "example.com/selection/internal/probe"
 "example.com/selection/internal/providers"
 "gitlab.com/skyrix-lib/framework/cobra/console"
 "gitlab.com/skyrix-lib/framework/cobra/platformcmd"
 "gitlab.com/skyrix-lib/framework/cobra/configcmd"
 "gitlab.com/skyrix-lib/framework/kernel"
)
func main(){
 if len(os.Args)>1 {
  app:=console.New(platformcmd.Command(),configcmd.Env(providers.PlatformSections()...));app.Add(providers.ConsoleCommands()...)
  if err:=app.Execute(context.Background(),os.Args[1:],os.Stdout,os.Stderr);err!=nil{panic(err)}
  if probe.AppConfig+probe.Constructed+probe.Components+probe.Started!=0 {panic("help/completion built the graph")}
  return
 }
 platform,err:=build();if err!=nil{panic(err)}
 if probe.AppConfig!=1 || probe.Constructed!=1 || probe.Selected!="`+name+`" || len(platform.Modules())!=1 || platform.Modules()[0].Name()!="`+name+`" {panic("wrong driver composition")}
 runtime:=kernel.NewRuntime(platform.Components()...)
 if err:=runtime.Start(context.Background());err!=nil{panic(err)}
 if err:=runtime.Stop(context.Background());err!=nil{panic(err)}
 if probe.Components!=1 || probe.Started!=1 {panic("wrong lifecycle")}
}
`)
	}

	authProviders, err := os.ReadFile(filepath.Join(template, "internal/providers/auth.go"))
	if err != nil {
		t.Fatal(err)
	}
	write("internal/providers/auth.go", strings.ReplaceAll(string(authProviders), "gitlab.com/skyrix-lib/template", "example.com/selection"))
	write("cmd/jwt/wire.go", `//go:build wireinject
package main
import (
 "github.com/goforj/wire"
 "gitlab.com/skyrix-lib/auth/service"
 "gitlab.com/skyrix-lib/auth/contracts"
 "gitlab.com/skyrix-lib/auth/middleware"
 "github.com/redis/go-redis/v9"
 engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
 "example.com/selection/internal/providers"
)
func legacy(cfg *service.Config, client *redis.Client, root engineredis.Keyspace)(*middleware.AuthMiddleware,func(),error){wire.Build(newResource,newLogger,providers.AuthProviderSet);return nil,nil,nil}
func revoking(cfg *service.Config, client *redis.Client, root engineredis.Keyspace)(*middleware.AuthMiddleware,func(),error){wire.Build(newResource,newLogger,providers.AuthRevocationSet);return nil,nil,nil}
func revoker(cfg *service.Config,checker contracts.RevocationChecker)(*service.Validator,func(),error){wire.Build(newResource,newLogger,service.NewRevokingValidator);return nil,nil,nil}
func verifier(cfg *service.Config)(*service.Validator,func(),error){wire.Build(newResource,newLogger,service.NewValidator);return nil,nil,nil}
func signer(cfg *service.Config)(*service.Signer,func(),error){wire.Build(newResource,newLogger,providers.AuthSignerSet);return nil,nil,nil}
`)
	write("cmd/jwt/main.go", `package main
import (
 "errors"
 "context"
 "os"
 "strings"
 "gitlab.com/skyrix-lib/auth/service"
 "gitlab.com/skyrix-lib/framework/logger"
)
type checker struct{}
func(checker) IsRevoked(context.Context,string,string)(bool,error){return false,nil}
var events []string
type resource struct{}
func newResource()(*resource,func()){events=append(events,"resource");return &resource{},func(){events=append(events,"close-resource")}}
func newLogger(*resource)(logger.Interface,func()){events=append(events,"logger");return noop{},func(){events=append(events,"close-logger")}}
type noop struct{}
func(noop) Error(string,...interface{}){}
func(noop) Warn(string,...interface{}){}
func(noop) Info(string,...interface{}){}
func(noop) Debug(string,...interface{}){}
func(noop) With(...interface{})logger.Interface{return noop{}}
func main(){
 check:=func(isNil bool,cleanup func(),err error){
  if !isNil||cleanup!=nil||!errors.Is(err,os.ErrNotExist){panic("constructor did not propagate error")}
  if strings.Join(events,",")!="resource,logger,close-logger,close-resource"{panic(strings.Join(events,","))}
 }
 cfg:=&service.Config{Algorithm:"RS256",PublicKeyPath:"absent-public.pem",PrivateKeyPath:"absent-private.pem"}
 value,cleanup,err:=verifier(cfg);check(value==nil,cleanup,err)
 events=nil
 signed,cleanup,err:=signer(cfg);check(signed==nil,cleanup,err)
 events=nil
 revoked,cleanup,err:=revoker(cfg,checker{});check(revoked==nil,cleanup,err)
}
`)
	generator := filepath.Join(dir, "platformgen")
	run(template, nil, "go", "build", "-o", generator, "gitlab.com/skyrix-lib/framework/cmd/platformgen")
	wire := strings.TrimSpace(run(dir, nil, "go", "tool", "-n", "wire"))
	generate := func() {
		run(filepath.Join(dir, "internal", "providers"), nil, generator)
		run(dir, nil, wire, "gen", "./cmd/first", "./cmd/second", "./cmd/jwt")
	}
	generate()
	run(dir, nil, "go", "run", "./cmd/jwt")
	paths := []string{"internal/providers/platform_gen.go", "cmd/first/wire_gen.go", "cmd/second/wire_gen.go", "cmd/jwt/wire_gen.go"}
	before := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(data)
	}
	generate()
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if before[path] != string(data) {
			t.Fatalf("generation changed %s", path)
		}
	}
	for _, name := range []string{"first", "second"} {
		other := "first"
		if name == "first" {
			other = "second"
		}
		code := before["cmd/"+name+"/wire_gen.go"]
		if strings.Contains(code, `"example.com/selection/`+other+`/platform"`) {
			t.Fatalf("%s graph imports the unselected constructor directly", name)
		}
		// Bad config of the excluded driver must neither be decoded nor validated.
		run(dir, []string{"PLATFORM_" + strings.ToUpper(other) + "_ENABLED=invalid"}, "go", "run", "./cmd/"+name)
	}
	invalid := []string{"PLATFORM_FIRST_ENABLED=invalid", "PLATFORM_SECOND_ENABLED=invalid"}
	output := run(dir, invalid, "go", "run", "./cmd/first", "--help")
	if !strings.Contains(output, "Fixture command") {
		t.Fatal("selected platform command missing from help")
	}
	run(dir, invalid, "go", "run", "./cmd/first", "first", "--help")
	run(dir, invalid, "go", "run", "./cmd/first", "completion", "bash")
	run(dir, invalid, "go", "run", "./cmd/first", "__complete", "first", "")
	output = run(dir, invalid, "go", "run", "./cmd/first", "config:env")
	if !strings.Contains(output, "PLATFORM_FIRST_ENABLED") || !strings.Contains(output, "PLATFORM_SECOND_ENABLED") {
		t.Fatal(output)
	}
	cmd := exec.Command("go", "run", "./cmd/first")
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, env...), invalid...)
	outputBytes, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(outputBytes), "config platform.first:") {
		t.Fatalf("selected config was not loaded: %v\n%s", err, outputBytes)
	}
	// Exercise add/remove/list through cobra.Command with real manifests. Keep
	// user-owned config, domains, graph sources and generated Wire files byte-exact.
	write("config/user.yaml", "platform:\n  second:\n    password: keep-me\n")
	write("internal/domain/user.go", "package domain\n")
	preserved := map[string]string{}
	for _, path := range []string{"go.mod", "go.sum", "config/user.yaml", "internal/domain/user.go", "internal/providers/platform.go", "cmd/first/wire.go", "cmd/second/wire.go", "cmd/first/wire_gen.go", "cmd/second/wire_gen.go"} {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		preserved[path] = string(data)
	}
	executable := filepath.Join(dir, "console")
	run(dir, nil, "go", "build", "-o", executable, "./cmd/first")
	output = run(dir, invalid, executable, "platform", "list")
	if !strings.Contains(output, "first\tplatform.first") || !strings.Contains(output, "second\tplatform.second") {
		t.Fatal(output)
	}
	// An invalid package/API/manifest must leave declarations and generation intact.
	for _, bad := range []string{"example.com/selection/missing/platform", "../evil", "example.com/selection/second/platform@v1"} {
		cmd := exec.Command(executable, "platform", "add", bad)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("accepted %s: %s", bad, out)
		}
		for _, path := range []string{"internal/providers/platform.go", "internal/providers/platform_gen.go"} {
			data, _ := os.ReadFile(filepath.Join(dir, path))
			want := preserved[path]
			if path == "internal/providers/platform_gen.go" {
				want = before[path]
			}
			if string(data) != want {
				t.Fatalf("invalid add modified %s", path)
			}
		}
	}

	write("third/platform/module.go", strings.ReplaceAll(strings.ReplaceAll(driver, "{{name}}", "third"), "{{NAME}}", "THIRD"))
	for _, manifest := range []string{
		"name: third\nconfig_key: platform.third\nconfig_example: '{}'\nruntimes: [http]\n",
		"name: first\nconfig_key: platform.third\nconfig_example: '{}'\n",
		"name: third\nconfig_key: platform.first\nconfig_example: '{}'\n",
		"name: third\nconfig_key: platform.first.nested\nconfig_example: '{}'\n",
		"name: third\nconfig_key: platform_first\nconfig_example: '{}'\n",
	} {
		write("third/platform/manifest.yaml", manifest)
		cmd := exec.Command(executable, "platform", "add", "example.com/selection/third/platform")
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("accepted conflicting manifest %q: %s", manifest, out)
		}
		source, _ := os.ReadFile(filepath.Join(dir, "internal/providers/platform.go"))
		generated, _ := os.ReadFile(filepath.Join(dir, "internal/providers/platform_gen.go"))
		if string(source) != preserved["internal/providers/platform.go"] || string(generated) != before["internal/providers/platform_gen.go"] {
			t.Fatal("invalid manifest changed files")
		}
	}
	output = run(dir, nil, executable, "platform", "add", "example.com/selection/second/platform")
	if !strings.Contains(output, "No changes") {
		t.Fatal(output)
	}
	genPath := filepath.Join(dir, "internal/providers/platform_gen.go")
	savedPath := filepath.Join(dir, "internal/providers/saved.generated")
	if err := os.Rename(genPath, savedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(genPath, 0755); err != nil {
		t.Fatal(err)
	}
	failedRemove := exec.Command(executable, "platform", "remove", "example.com/selection/second/platform")
	failedRemove.Dir = dir
	failedRemove.Env = env
	if out, err := failedRemove.CombinedOutput(); err == nil {
		t.Fatalf("accepted directory as generated file: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "internal/providers/platform.go"))
	if string(data) != preserved["internal/providers/platform.go"] {
		t.Fatal("failed publication changed declarations")
	}
	if err := os.Remove(genPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(savedPath, genPath); err != nil {
		t.Fatal(err)
	}
	run(dir, nil, executable, "platform", "remove", "example.com/selection/second/platform")
	output = run(dir, nil, executable, "platform", "list")
	if strings.Contains(output, "second") {
		t.Fatal(output)
	}
	generated, _ := os.ReadFile(filepath.Join(dir, "internal/providers/platform_gen.go"))
	if strings.Contains(string(generated), "SecondplatformSet") {
		t.Fatal("removed set retained")
	}
	output = run(dir, nil, executable, "platform", "remove", "example.com/selection/second/platform")
	if !strings.Contains(output, "No changes") {
		t.Fatal(output)
	}
	run(dir, nil, executable, "platform", "add", "example.com/selection/second/platform")
	for path, want := range preserved {
		data, err := os.ReadFile(filepath.Join(dir, path))
		got := string(data)
		if path == "internal/providers/platform.go" {
			declaration := "//skyrix:platform example.com/selection/second/platform\n"
			got = strings.ReplaceAll(got, declaration, "")
			want = strings.ReplaceAll(want, declaration, "")
		}
		if err != nil || got != want {
			t.Fatalf("changed user file %s", path)
		}
	}
	generated, _ = os.ReadFile(filepath.Join(dir, "internal/providers/platform_gen.go"))
	if string(generated) != before["internal/providers/platform_gen.go"] {
		t.Fatal("remove/add generation differs")
	}

	// The already-built Console must also operate on projects with the legacy layout.
	legacyDir := filepath.Join(dir, "providers")
	if err := os.Rename(filepath.Join(dir, "internal", "providers"), legacyDir); err != nil {
		t.Fatal(err)
	}
	driverPath := "example.com/selection/second/platform"
	run(dir, nil, executable, "platform", "remove", driverPath)
	if output := run(dir, nil, executable, "platform", "list"); strings.Contains(output, driverPath) {
		t.Fatal(output)
	}
	run(dir, nil, executable, "platform", "add", driverPath)
	if output := run(dir, nil, executable, "platform", "list"); !strings.Contains(output, driverPath) {
		t.Fatal(output)
	}
	legacyGenerated, err := os.ReadFile(filepath.Join(legacyDir, "platform_gen.go"))
	if err != nil || !bytes.Equal(legacyGenerated, generated) {
		t.Fatalf("legacy layout produced different platform adapters: %v", err)
	}
}
