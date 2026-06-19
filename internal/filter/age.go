package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("age", newAgeFilter)
}

type ageFilter struct {
	key      string
	op       string
	duration time.Duration
}

func newAgeFilter(spec map[string]any) (Filter, error) {
	key, _ := spec["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("age filter requires 'key'")
	}
	op, _ := spec["op"].(string)
	if op != "older-than" && op != "newer-than" {
		return nil, fmt.Errorf("age filter 'op' must be 'older-than' or 'newer-than', got %q", op)
	}
	valStr, _ := spec["value"].(string)
	if valStr == "" {
		return nil, fmt.Errorf("age filter requires 'value' as a duration string (e.g. '30d', '24h')")
	}

	dur, err := parseDuration(valStr)
	if err != nil {
		return nil, fmt.Errorf("age filter: %w", err)
	}
	return &ageFilter{key: key, op: op, duration: dur}, nil
}

func (f *ageFilter) Match(r resource.Resource) (bool, error) {
	props := r.Properties()
	raw, ok := props[f.key]
	if !ok {
		return false, nil
	}

	var t time.Time
	switch v := raw.(type) {
	case time.Time:
		t = v
	case *time.Time:
		if v == nil {
			return false, nil
		}
		t = *v
	default:
		return false, fmt.Errorf("age filter: field %q is not a time.Time (got %T)", f.key, raw)
	}

	age := time.Since(t)
	switch f.op {
	case "older-than":
		return age > f.duration, nil
	case "newer-than":
		return age < f.duration, nil
	}
	return false, nil
}

var dayPattern = regexp.MustCompile(`^(\d+)d$`)

// parseDuration supports Go durations plus a "d" (day) suffix.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if m := dayPattern.FindStringSubmatch(s); m != nil {
		days, _ := strconv.Atoi(m[1])
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
