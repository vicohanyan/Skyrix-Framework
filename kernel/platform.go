package kernel

import "fmt"

// Module is a compile-time platform module: an engine core or an optional
// package (eventbus, tenant, ...) that contributes lifecycle components.
// Further capabilities, such as console commands, are optional interfaces
// declared by the hosts that consume them.
type Module interface {
	Name() string
	Components() []Component
}

// Platform is the ordered set of modules compiled into a binary.
type Platform struct {
	modules []Module
}

// NewPlatform validates module names and preserves registration order.
func NewPlatform(modules ...Module) (*Platform, error) {
	seen := make(map[string]struct{}, len(modules))
	registered := make([]Module, 0, len(modules))
	for index, module := range modules {
		if module == nil {
			return nil, fmt.Errorf("platform module %d is nil", index)
		}
		name := module.Name()
		if name == "" {
			return nil, fmt.Errorf("platform module %d has an empty name", index)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("platform module %q is registered more than once", name)
		}
		seen[name] = struct{}{}
		registered = append(registered, module)
	}
	return &Platform{modules: registered}, nil
}

// Modules returns the registered modules in registration order.
func (p *Platform) Modules() []Module {
	if p == nil {
		return nil
	}
	return append([]Module(nil), p.modules...)
}

// Components returns the components of all modules in registration order.
func (p *Platform) Components() []Component {
	if p == nil {
		return nil
	}
	var components []Component
	for _, module := range p.modules {
		components = append(components, module.Components()...)
	}
	return components
}
