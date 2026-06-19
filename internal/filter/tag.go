package filter

import (
	"fmt"
	"strings"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("tag", newTagFilter)
}

type tagFilter struct {
	key   string
	value string
	op    string
}

// newTagFilter is a convenience filter for Datadog tag arrays.
// Tags are stored as "key:value" strings in the "tags" property.
func newTagFilter(spec map[string]any) (Filter, error) {
	key, _ := spec["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("tag filter requires 'key'")
	}
	op, _ := spec["op"].(string)
	if op == "" {
		op = "present"
	}
	val, _ := spec["value"].(string)

	return &tagFilter{key: key, value: val, op: op}, nil
}

func (f *tagFilter) Match(r resource.Resource) (bool, error) {
	props := r.Properties()
	raw, ok := props["tags"]
	if !ok {
		if f.op == "absent" {
			return true, nil
		}
		return false, nil
	}

	var tags []string
	switch v := raw.(type) {
	case []string:
		tags = v
	case []any:
		for _, item := range v {
			tags = append(tags, fmt.Sprintf("%v", item))
		}
	default:
		return false, fmt.Errorf("tag filter: 'tags' field is not a string slice (got %T)", raw)
	}

	switch f.op {
	case "present":
		return hasTag(tags, f.key, f.value), nil
	case "absent":
		return !hasTag(tags, f.key, f.value), nil
	case "eq":
		return hasTagExact(tags, f.key, f.value), nil
	default:
		return false, fmt.Errorf("unknown tag filter op %q", f.op)
	}
}

// hasTag checks whether any tag matches the key (and optionally value).
func hasTag(tags []string, key, value string) bool {
	for _, t := range tags {
		if value == "" {
			// presence check: tag starts with "key:" or equals "key"
			if t == key || strings.HasPrefix(t, key+":") {
				return true
			}
		} else {
			if t == key+":"+value {
				return true
			}
		}
	}
	return false
}

func hasTagExact(tags []string, key, value string) bool {
	return hasTag(tags, key, value)
}
