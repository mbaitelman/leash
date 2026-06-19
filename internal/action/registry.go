package action

import "fmt"

var registry = map[string]Factory{}

// Register adds a Factory to the global registry.
// Called from init() in each action file.
func Register(name string, f Factory) {
	registry[name] = f
}

// Get returns the Factory for the given action type name.
func Get(actionType string) (Factory, error) {
	f, ok := registry[actionType]
	if !ok {
		return nil, fmt.Errorf("unknown action type %q", actionType)
	}
	return f, nil
}
