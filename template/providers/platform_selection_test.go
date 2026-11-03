package providers_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise real platformgen + Wire with two declarations in the same providers
// package. Imports are shared intentionally; construction and lifecycle are not.
func TestPlatformDriverSelection(t *testing.T) {
	template, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly -buildvcs=false", "PLATFORM_FIRST_ENABLED=false", "PLATFORM_SECOND_ENABLED=false")
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
			source = strings.Replace(source, "replace gitlab.com/skyrix-lib/framework => ..", "replace gitlab.com/skyrix-lib/framework => "+filepath.Dir(template), 1)
		}
		write(name, source)
	}
	write("internal/probe/probe.go", `package probe
var AppConfig, Constructed, Components, Started int
var Selected string
`)
	write("internal/config/config.go", `package config
import "example.com/selection/internal/probe"
type Config struct { First *First; Second *Second }
type First struct{}
type Second struct{}
func (*First) DefaultEnabled() bool { return false }
func (*Second) DefaultEnabled() bool { return false }
func New() *Config {probe.AppConfig++;return &Config{First:&First{},Second:&Second{}}}
`)
	driver := `package platform
import (
 "context"
 "github.com/goforj/wire"
 "github.com/spf13/cobra"
 "example.com/selection/internal/probe"
 frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
 "gitlab.com/skyrix-lib/framework/config"
 "gitlab.com/skyrix-lib/framework/kernel"
)
type Config interface { DefaultEnabled() bool }
type options struct { Enabled bool ` + "`yaml:\"enabled\"`" + ` }
type Module struct{}
var ProviderSet=wire.NewSet(New)
func New(cfg Config)(*Module,error) {
 opts:=&options{Enabled:cfg.DefaultEnabled()}
 loader,err:=config.New(config.Section{Key:"platform.{{name}}",Value:opts})
 if err!=nil{return nil,err};if err:=loader.Load(nil,nil);err!=nil{return nil,err}
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
		write(name+"/platform/module.go", strings.ReplaceAll(driver, "{{name}}", name))
	}
	write("providers/platform.go", `package providers
import (
 first "example.com/selection/first/platform"
 second "example.com/selection/second/platform"
 frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
 "gitlab.com/skyrix-lib/framework/kernel"
 "github.com/spf13/cobra"
)
//skyrix:platform example.com/selection/first/platform config=First
//skyrix:platform example.com/selection/second/platform config=Second
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
 "example.com/selection/providers"
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
 "example.com/selection/providers"
 "gitlab.com/skyrix-lib/framework/cobra/console"
 "gitlab.com/skyrix-lib/framework/kernel"
)
func main(){
 if len(os.Args)>1 {
  app:=console.New();app.AddCobra(providers.ConsoleCommands()...)
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
	generator := filepath.Join(dir, "platformgen")
	run(template, nil, "go", "build", "-o", generator, "gitlab.com/skyrix-lib/framework/cmd/platformgen")
	wire := strings.TrimSpace(run(template, nil, "go", "tool", "-n", "wire"))
	generate := func() {
		run(filepath.Join(dir, "providers"), nil, generator)
		run(dir, nil, wire, "gen", "./cmd/first", "./cmd/second")
	}
	generate()
	paths := []string{"providers/platform_gen.go", "cmd/first/wire_gen.go", "cmd/second/wire_gen.go"}
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
	cmd := exec.Command("go", "run", "./cmd/first")
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, env...), invalid...)
	outputBytes, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(outputBytes), "config platform.first.enabled") {
		t.Fatalf("selected config was not loaded: %v\n%s", err, outputBytes)
	}
}
