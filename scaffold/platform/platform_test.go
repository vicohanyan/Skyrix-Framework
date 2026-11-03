package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = "name: eventbus\nconfig_key: platform.eventbus\nconfig_example: |\n  enabled: false\n"

func TestManifest(t *testing.T) {
	m, err := ReadManifest(strings.NewReader(validManifest))
	if err != nil || m.Name != "eventbus" || m.ConfigKey != "platform.eventbus" {
		t.Fatalf("%+v %v", m, err)
	}
	for _, s := range []string{
		"", strings.ReplaceAll(validManifest, "eventbus", "../bus"), strings.Replace(validManifest, "platform.eventbus", "bad-key", 1),
		validManifest + "runtimes: [http]\n", validManifest + "version: v1\n", validManifest + "---\nname: other\n", validManifest + "name: duplicate\n",
		"name: bus\nconfig_key: platform.bus\nconfig_example: ''\n",
		"name: bus\nconfig_key: platform.bus\nconfig_example: '[invalid'\n",
	} {
		if _, err := ReadManifest(strings.NewReader(s)); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestParse(t *testing.T) {
	drivers, err := parse([]byte("package providers\n// //skyrix:platform example.com/commented/platform\n" + prefix + "example.com/first/platform\n" + prefix + "example.com/second/platform\n"))
	if err != nil || len(drivers) != 2 || drivers[1].ImportPath != "example.com/second/platform" {
		t.Fatalf("%v %v", drivers, err)
	}
	for _, line := range []string{prefix, prefix + "example.com/bus config=Field", prefix + "example.com/bus binaries=http", prefix + "../evil", prefix + "example.com/bus@v1", prefix + "example.com/bus\n" + prefix + "example.com/bus"} {
		if _, err := parse([]byte(line)); err == nil {
			t.Fatalf("accepted %q", line)
		}
	}
}
func TestRenderIndependentAndRepeatable(t *testing.T) {
	ds := []Driver{{ImportPath: "example.com/first/platform", alias: "firstplatform", Manifest: Manifest{ConfigKey: "platform.first"}}, {ImportPath: "example.com/second/platform", alias: "secondplatform", Manifest: Manifest{ConfigKey: "platform.second"}}}
	a, err := render(ds, "example.com/app/internal/config")
	if err != nil {
		t.Fatal(err)
	}
	b, err := render(ds, "example.com/app/internal/config")
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("generation differs")
	}
	for _, want := range []string{"var FirstplatformSet = wire.NewSet(firstplatform.ProviderSet, ProvideFirstplatformConfig)", "var SecondplatformSet = wire.NewSet(secondplatform.ProviderSet, ProvideSecondplatformConfig)", `Key: "platform.first"`, "value := firstplatform.NewConfig()", "cfg.LoadSections(sections.Section"} {
		if !strings.Contains(string(a), want) {
			t.Fatalf("missing %s:\n%s", want, a)
		}
	}
	for _, bad := range []string{"var PlatformSet", "wire.FieldsOf", "wire.Bind", "kernel.NewPlatform", "ModuleCommands"} {
		if strings.Contains(string(a), bad) {
			t.Fatalf("unexpected %s", bad)
		}
	}
}
func TestInvalidChangePreservesFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"platform.go", "platform_gen.go", "config.yaml", "domain.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("user file"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"../evil", "example.com/bus config=Field", "example.com/bus@v1"} {
		if _, err := Change(dir, path, true); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	for _, name := range []string{"platform.go", "platform_gen.go", "config.yaml", "domain.go"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != "user file" {
			t.Fatalf("modified %s", name)
		}
	}
}

func TestGeneratedFileOwnership(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "platform_gen.go")
	if err := os.WriteFile(path, []byte("package providers // user file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := replace(path, []byte(header)); err == nil {
		t.Fatal("overwrote user file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "package providers // user file" {
		t.Fatal("changed user file")
	}
}
func TestNearestConfigImport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.27\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "providers")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := configImport(dir)
	if err != nil || got != "example.com/app/internal/config" {
		t.Fatalf("%q %v", got, err)
	}
}
