package filter

import "fmt"

var registry = map[string]Factory{}

// Register adds a Factory to the global registry.
// Called from init() in each filter file.
func Register(name string, f Factory) {
	registry[name] = f
}

// Get returns the Factory for the given filter type name.
func Get(filterType string) (Factory, error) {
	f, ok := registry[filterType]
	if !ok {
		return nil, fmt.Errorf("unknown filter type %q", filterType)
	}
	return f, nil
}
