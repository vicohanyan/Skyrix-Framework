package kernel

import (
	"context"
	"testing"
)

type testModule struct {
	name       string
	components []Component
}

func (m testModule) Name() string            { return m.name }
func (m testModule) Components() []Component { return m.components }

type testComponent struct{ name string }

func (*testComponent) Start(context.Context) error { return nil }
func (*testComponent) Stop(context.Context) error  { return nil }

func TestPlatformPreservesModuleOrder(t *testing.T) {
	first := &testComponent{name: "first"}
	second := &testComponent{name: "second"}
	p, err := NewPlatform(
		testModule{name: "one", components: []Component{first}},
		testModule{name: "two", components: []Component{second}},
	)
	if err != nil {
		t.Fatal(err)
	}
	components := p.Components()
	if len(components) != 2 || components[0] != first || components[1] != second {
		t.Fatalf("components = %#v", components)
	}
	modules := p.Modules()
	if len(modules) != 2 || modules[0].Name() != "one" || modules[1].Name() != "two" {
		t.Fatalf("modules = %#v", modules)
	}
}

func TestPlatformRejectsInvalidModules(t *testing.T) {
	if _, err := NewPlatform(testModule{name: "eventbus"}, testModule{name: "eventbus"}); err == nil {
		t.Fatal("duplicate module error = nil")
	}
	if _, err := NewPlatform(testModule{}); err == nil {
		t.Fatal("empty module name error = nil")
	}
	if _, err := NewPlatform(nil); err == nil {
		t.Fatal("nil module error = nil")
	}
}
