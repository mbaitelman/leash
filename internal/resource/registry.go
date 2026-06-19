package resource

import (
	"fmt"
	"sort"
)

var registry = map[string]Provider{}

// Register adds a Provider to the global registry.
// Called from init() in each resource file.
func Register(p Provider) {
	registry[p.ResourceType()] = p
}

// Get returns the Provider for the given resource type name.
func Get(resourceType string) (Provider, error) {
	p, ok := registry[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type %q — run 'leash list-resources' for available types", resourceType)
	}
	return p, nil
}

// ListTypes returns all registered resource type names, sorted.
func ListTypes() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
