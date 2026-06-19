package filter

import "github.com/mbaitelman/leash/internal/resource"

// Filter evaluates whether a resource matches a condition.
type Filter interface {
	Match(r resource.Resource) (bool, error)
}

// Factory constructs a Filter from the raw YAML spec map.
type Factory func(spec map[string]any) (Filter, error)
