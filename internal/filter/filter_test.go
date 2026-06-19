package filter_test

import (
	"testing"
	"time"

	"github.com/mbaitelman/leash/internal/filter"
	"github.com/mbaitelman/leash/internal/policy"
)

// fakeResource is a test double that returns arbitrary Properties.
type fakeResource struct {
	props map[string]any
}

func (r *fakeResource) Type() string               { return "fake.resource" }
func (r *fakeResource) ID() string                 { return "fake-id" }
func (r *fakeResource) Properties() map[string]any { return r.props }
func (r *fakeResource) Raw() any                   { return nil }

func fake(kv ...any) *fakeResource {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return &fakeResource{props: m}
}

func buildFilter(t *testing.T, kv ...any) filter.Filter {
	t.Helper()
	spec := policy.FilterSpec{}
	for i := 0; i+1 < len(kv); i += 2 {
		spec[kv[i].(string)] = kv[i+1]
	}
	f, err := filter.Build(spec)
	if err != nil {
		t.Fatalf("Build(%v): %v", spec, err)
	}
	return f
}

func mustMatch(t *testing.T, f filter.Filter, r *fakeResource) {
	t.Helper()
	got, err := f.Match(r)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if !got {
		t.Error("expected match=true, got false")
	}
}

func mustNotMatch(t *testing.T, f filter.Filter, r *fakeResource) {
	t.Helper()
	got, err := f.Match(r)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if got {
		t.Error("expected match=false, got true")
	}
}

// ── Value filter ──────────────────────────────────────────────────────────────

func TestValueFilter_Eq(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "eq", "value", "prod")
	mustMatch(t, f, fake("name", "prod"))
	mustNotMatch(t, f, fake("name", "staging"))
}

func TestValueFilter_Ne(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "ne", "value", "prod")
	mustMatch(t, f, fake("name", "staging"))
	mustNotMatch(t, f, fake("name", "prod"))
}

func TestValueFilter_Present(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "present")
	mustMatch(t, f, fake("name", "anything"))
	mustNotMatch(t, f, fake())
}

func TestValueFilter_Absent(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "absent")
	mustMatch(t, f, fake())
	mustNotMatch(t, f, fake("name", "something"))
}

func TestValueFilter_Contains_String(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "contains", "value", "prod")
	mustMatch(t, f, fake("name", "prod-monitor"))
	mustNotMatch(t, f, fake("name", "staging-monitor"))
}

func TestValueFilter_Contains_Slice(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "tags", "op", "contains", "value", "env:prod")
	mustMatch(t, f, fake("tags", []string{"env:prod", "team:platform"}))
	mustNotMatch(t, f, fake("tags", []string{"env:staging"}))
}

func TestValueFilter_NotContains(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "tags", "op", "not-contains", "value", "env:prod")
	mustMatch(t, f, fake("tags", []string{"env:staging"}))
	mustNotMatch(t, f, fake("tags", []string{"env:prod", "team:x"}))
}

func TestValueFilter_Regex(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "regex", "value", `^\[PROD\]`)
	mustMatch(t, f, fake("name", "[PROD] high error rate"))
	mustNotMatch(t, f, fake("name", "high error rate"))
}

func TestValueFilter_NotRegex(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "name", "op", "not-regex", "value", `^\[PROD\]`)
	mustMatch(t, f, fake("name", "some monitor"))
	mustNotMatch(t, f, fake("name", "[PROD] alert"))
}

func TestValueFilter_In(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "status", "op", "in", "value", []any{"live", "paused"})
	mustMatch(t, f, fake("status", "live"))
	mustMatch(t, f, fake("status", "paused"))
	mustNotMatch(t, f, fake("status", "deleted"))
}

func TestValueFilter_NotIn(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "status", "op", "not-in", "value", []any{"live", "paused"})
	mustMatch(t, f, fake("status", "deleted"))
	mustNotMatch(t, f, fake("status", "live"))
}

func TestValueFilter_Gt(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "count", "op", "gt", "value", float64(10))
	mustMatch(t, f, fake("count", float64(11)))
	mustNotMatch(t, f, fake("count", float64(10)))
	mustNotMatch(t, f, fake("count", float64(9)))
}

func TestValueFilter_Lt(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "count", "op", "lt", "value", float64(10))
	mustMatch(t, f, fake("count", float64(9)))
	mustNotMatch(t, f, fake("count", float64(10)))
}

func TestValueFilter_Gte(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "count", "op", "gte", "value", float64(10))
	mustMatch(t, f, fake("count", float64(10)))
	mustMatch(t, f, fake("count", float64(11)))
	mustNotMatch(t, f, fake("count", float64(9)))
}

func TestValueFilter_Lte(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "count", "op", "lte", "value", float64(10))
	mustMatch(t, f, fake("count", float64(10)))
	mustNotMatch(t, f, fake("count", float64(11)))
}

func TestValueFilter_MissingKey_ReturnsFalse(t *testing.T) {
	f := buildFilter(t, "type", "value", "key", "missing", "op", "eq", "value", "x")
	mustNotMatch(t, f, fake())
}

func TestValueFilter_MissingKeyRequired(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{"type": "value"})
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestValueFilter_InvalidRegex(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{
		"type": "value", "key": "name", "op": "regex", "value": `[invalid`,
	})
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

// ── Age filter ────────────────────────────────────────────────────────────────

func TestAgeFilter_OlderThan(t *testing.T) {
	f := buildFilter(t, "type", "age", "key", "created", "op", "older-than", "value", "1h")
	old := fake("created", time.Now().Add(-2*time.Hour))
	recent := fake("created", time.Now().Add(-30*time.Minute))
	mustMatch(t, f, old)
	mustNotMatch(t, f, recent)
}

func TestAgeFilter_NewerThan(t *testing.T) {
	f := buildFilter(t, "type", "age", "key", "created", "op", "newer-than", "value", "1h")
	recent := fake("created", time.Now().Add(-30*time.Minute))
	old := fake("created", time.Now().Add(-2*time.Hour))
	mustMatch(t, f, recent)
	mustNotMatch(t, f, old)
}

func TestAgeFilter_DaySuffix(t *testing.T) {
	f := buildFilter(t, "type", "age", "key", "created", "op", "older-than", "value", "7d")
	old := fake("created", time.Now().Add(-8*24*time.Hour))
	recent := fake("created", time.Now().Add(-6*24*time.Hour))
	mustMatch(t, f, old)
	mustNotMatch(t, f, recent)
}

func TestAgeFilter_MissingKey_ReturnsFalse(t *testing.T) {
	f := buildFilter(t, "type", "age", "key", "created", "op", "older-than", "value", "1h")
	mustNotMatch(t, f, fake())
}

func TestAgeFilter_InvalidOp(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{
		"type": "age", "key": "created", "op": "before", "value": "1h",
	})
	if err == nil {
		t.Error("expected error for invalid op")
	}
}

func TestAgeFilter_InvalidDuration(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{
		"type": "age", "key": "created", "op": "older-than", "value": "notaduration",
	})
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

// ── Tag filter ────────────────────────────────────────────────────────────────

func TestTagFilter_Present(t *testing.T) {
	f := buildFilter(t, "type", "tag", "key", "env", "op", "present")
	mustMatch(t, f, fake("tags", []string{"env:prod", "team:platform"}))
	mustMatch(t, f, fake("tags", []string{"env:staging"}))
	mustNotMatch(t, f, fake("tags", []string{"team:platform"}))
}

func TestTagFilter_Present_KeyOnly(t *testing.T) {
	// key without value: checks bare "key" or "key:anything"
	f := buildFilter(t, "type", "tag", "key", "env")
	mustMatch(t, f, fake("tags", []string{"env:prod"}))
	mustMatch(t, f, fake("tags", []string{"env"}))
	mustNotMatch(t, f, fake("tags", []string{"team:x"}))
}

func TestTagFilter_Absent(t *testing.T) {
	f := buildFilter(t, "type", "tag", "key", "env", "value", "prod", "op", "absent")
	mustMatch(t, f, fake("tags", []string{"env:staging"}))
	mustNotMatch(t, f, fake("tags", []string{"env:prod"}))
}

func TestTagFilter_Absent_NoTagsField(t *testing.T) {
	f := buildFilter(t, "type", "tag", "key", "env", "op", "absent")
	mustMatch(t, f, fake()) // no "tags" key → absent is true
}

func TestTagFilter_Present_NoTagsField(t *testing.T) {
	f := buildFilter(t, "type", "tag", "key", "env", "op", "present")
	mustNotMatch(t, f, fake())
}

func TestTagFilter_Eq(t *testing.T) {
	f := buildFilter(t, "type", "tag", "key", "env", "value", "prod", "op", "eq")
	mustMatch(t, f, fake("tags", []string{"env:prod"}))
	mustNotMatch(t, f, fake("tags", []string{"env:staging"}))
}

func TestTagFilter_SliceAny(t *testing.T) {
	f := buildFilter(t, "type", "tag", "key", "env", "value", "prod", "op", "present")
	mustMatch(t, f, fake("tags", []any{"env:prod", "team:x"}))
}

func TestTagFilter_MissingKey(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{"type": "tag"})
	if err == nil {
		t.Error("expected error for missing key")
	}
}

// ── Boolean filters ───────────────────────────────────────────────────────────

func TestAndFilter_AllMatch(t *testing.T) {
	spec := policy.FilterSpec{
		"and": []any{
			policy.FilterSpec{"type": "value", "key": "env", "op": "eq", "value": "prod"},
			policy.FilterSpec{"type": "value", "key": "team", "op": "present"},
		},
	}
	f, err := filter.Build(spec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	mustMatch(t, f, fake("env", "prod", "team", "platform"))
	mustNotMatch(t, f, fake("env", "prod")) // team absent
	mustNotMatch(t, f, fake("team", "x"))   // env wrong
}

func TestOrFilter_AnyMatch(t *testing.T) {
	spec := policy.FilterSpec{
		"or": []any{
			policy.FilterSpec{"type": "value", "key": "env", "op": "eq", "value": "prod"},
			policy.FilterSpec{"type": "value", "key": "env", "op": "eq", "value": "staging"},
		},
	}
	f, err := filter.Build(spec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	mustMatch(t, f, fake("env", "prod"))
	mustMatch(t, f, fake("env", "staging"))
	mustNotMatch(t, f, fake("env", "dev"))
}

func TestNotFilter_Negates(t *testing.T) {
	spec := policy.FilterSpec{
		"not": []any{
			policy.FilterSpec{"type": "value", "key": "name", "op": "regex", "value": `^\[PROD\]`},
		},
	}
	f, err := filter.Build(spec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	mustMatch(t, f, fake("name", "regular monitor"))
	mustNotMatch(t, f, fake("name", "[PROD] monitor"))
}

func TestNotFilter_RequiresExactlyOneChild(t *testing.T) {
	spec := policy.FilterSpec{
		"not": []any{
			policy.FilterSpec{"type": "value", "key": "a", "op": "present"},
			policy.FilterSpec{"type": "value", "key": "b", "op": "present"},
		},
	}
	_, err := filter.Build(spec)
	if err == nil {
		t.Error("expected error for not with 2 children")
	}
}

// ── Builder ───────────────────────────────────────────────────────────────────

func TestBuild_MissingType(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{"key": "name"})
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestBuild_UnknownType(t *testing.T) {
	_, err := filter.Build(policy.FilterSpec{"type": "fuzzy", "key": "name"})
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestBuildChain_ANDsFilters(t *testing.T) {
	specs := []policy.FilterSpec{
		{"type": "value", "key": "env", "op": "eq", "value": "prod"},
		{"type": "value", "key": "team", "op": "present"},
	}
	filters, err := filter.BuildChain(specs)
	if err != nil {
		t.Fatalf("BuildChain: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}

	r := fake("env", "prod", "team", "sre")
	for _, f := range filters {
		ok, err := f.Match(r)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("expected all filters to match")
		}
	}
}

func TestBuildChain_Empty(t *testing.T) {
	filters, err := filter.BuildChain(nil)
	if err != nil {
		t.Fatalf("BuildChain(nil): %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(filters))
	}
}
