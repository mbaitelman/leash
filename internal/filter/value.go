package filter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("value", newValueFilter)
}

type valueFilter struct {
	key    string
	op     string
	value  any
	regex  *regexp.Regexp
}

func newValueFilter(spec map[string]any) (Filter, error) {
	key, _ := spec["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("value filter requires 'key'")
	}
	op, _ := spec["op"].(string)
	if op == "" {
		op = "eq"
	}
	val := spec["value"]

	f := &valueFilter{key: key, op: op, value: val}

	if op == "regex" || op == "not-regex" {
		pattern, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("value filter op %q requires a string 'value'", op)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("value filter: invalid regex %q: %w", pattern, err)
		}
		f.regex = re
	}

	return f, nil
}

func (f *valueFilter) Match(r resource.Resource) (bool, error) {
	props := r.Properties()
	target, exists := props[f.key]

	switch f.op {
	case "present":
		return exists && target != nil, nil
	case "absent":
		return !exists || target == nil, nil
	}

	if !exists {
		return false, nil
	}

	switch f.op {
	case "eq":
		return fmt.Sprintf("%v", target) == fmt.Sprintf("%v", f.value), nil
	case "ne":
		return fmt.Sprintf("%v", target) != fmt.Sprintf("%v", f.value), nil
	case "contains":
		return matchContains(target, f.value), nil
	case "not-contains":
		return !matchContains(target, f.value), nil
	case "regex":
		return f.regex.MatchString(fmt.Sprintf("%v", target)), nil
	case "not-regex":
		return !f.regex.MatchString(fmt.Sprintf("%v", target)), nil
	case "in":
		return matchIn(target, f.value), nil
	case "not-in":
		return !matchIn(target, f.value), nil
	case "gt", "lt", "gte", "lte":
		return matchNumeric(f.op, target, f.value)
	default:
		return false, fmt.Errorf("unknown value filter op %q", f.op)
	}
}

func matchContains(target, needle any) bool {
	needleStr := fmt.Sprintf("%v", needle)
	switch tv := target.(type) {
	case string:
		return strings.Contains(tv, needleStr)
	case []string:
		for _, s := range tv {
			if s == needleStr {
				return true
			}
		}
	case []any:
		for _, item := range tv {
			if fmt.Sprintf("%v", item) == needleStr {
				return true
			}
		}
	}
	return false
}

func matchIn(target, list any) bool {
	targetStr := fmt.Sprintf("%v", target)
	switch l := list.(type) {
	case []any:
		for _, item := range l {
			if fmt.Sprintf("%v", item) == targetStr {
				return true
			}
		}
	case []string:
		for _, s := range l {
			if s == targetStr {
				return true
			}
		}
	}
	return false
}

func matchNumeric(op string, target, value any) (bool, error) {
	toFloat := func(v any) (float64, error) {
		switch n := v.(type) {
		case int:
			return float64(n), nil
		case int64:
			return float64(n), nil
		case float64:
			return n, nil
		default:
			return 0, fmt.Errorf("cannot compare non-numeric value %v (%T)", v, v)
		}
	}

	t, err := toFloat(target)
	if err != nil {
		return false, fmt.Errorf("value filter: target %w", err)
	}
	v, err := toFloat(value)
	if err != nil {
		return false, fmt.Errorf("value filter: comparison %w", err)
	}

	switch op {
	case "gt":
		return t > v, nil
	case "lt":
		return t < v, nil
	case "gte":
		return t >= v, nil
	case "lte":
		return t <= v, nil
	}
	return false, nil
}
