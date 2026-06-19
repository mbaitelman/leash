package filter

import (
	"fmt"

	"github.com/mbaitelman/leash/internal/policy"
)

// Build constructs a Filter from a single FilterSpec.
// It detects boolean meta-filters (and/or/not) before delegating to the type registry.
func Build(spec policy.FilterSpec) (Filter, error) {
	m := map[string]any(spec)

	if raw, ok := m["and"]; ok {
		return buildGroup("and", raw)
	}
	if raw, ok := m["or"]; ok {
		return buildGroup("or", raw)
	}
	if raw, ok := m["not"]; ok {
		children, err := toFilterList(raw)
		if err != nil {
			return nil, fmt.Errorf("not filter: %w", err)
		}
		if len(children) != 1 {
			return nil, fmt.Errorf("not filter requires exactly one child, got %d", len(children))
		}
		return &notFilter{child: children[0]}, nil
	}

	filterType, ok := m["type"].(string)
	if !ok || filterType == "" {
		return nil, fmt.Errorf("filter spec missing 'type' field (or boolean key 'and'/'or'/'not')")
	}

	factory, err := Get(filterType)
	if err != nil {
		return nil, err
	}
	return factory(m)
}

// BuildChain constructs a slice of Filters from a slice of FilterSpecs.
// All filters in a chain are AND-ed together by the engine.
func BuildChain(specs []policy.FilterSpec) ([]Filter, error) {
	filters := make([]Filter, 0, len(specs))
	for _, spec := range specs {
		f, err := Build(spec)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

func buildGroup(op string, raw any) (Filter, error) {
	children, err := toFilterList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s filter: %w", op, err)
	}
	if op == "and" {
		return &andFilter{children: children}, nil
	}
	return &orFilter{children: children}, nil
}

func toFilterList(raw any) ([]Filter, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("expected a list of filter specs, got %T", raw)
	}

	filters := make([]Filter, 0, len(items))
	for _, item := range items {
		// yaml.v3 preserves the named FilterSpec type when decoding nested sequences,
		// so we must handle both FilterSpec and plain map[string]any.
		var specMap policy.FilterSpec
		switch v := item.(type) {
		case policy.FilterSpec:
			specMap = v
		case map[string]any:
			specMap = policy.FilterSpec(v)
		default:
			return nil, fmt.Errorf("each filter spec must be a mapping, got %T", item)
		}
		f, err := Build(specMap)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}
