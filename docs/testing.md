# Testing

Leash has three testing surfaces, each requiring different levels of setup:

| Level | Requires DD credentials? | Speed | What it covers |
|---|---|---|---|
| Unit tests | No | Fast | Filter logic, policy parsing, output formatting |
| Dry-run validation | No | Fast | Policy YAML correctness against live code |
| Integration tests | Yes | Slow | Full end-to-end against a real Datadog org |

---

## Unit testing the filter engine

The filter engine is the core of Leash and can be tested entirely without Datadog credentials. Filters operate on `resource.Resource` values, which expose a `Properties() map[string]any` map. You can construct synthetic resources in tests without making any API calls.

### Writing a unit test

Create a test file in `internal/filter/`:

```go
// internal/filter/value_test.go
package filter_test

import (
    "testing"

    "github.com/mbaitelman/leash/internal/filter"
    "github.com/mbaitelman/leash/internal/policy"
)

// fakeResource is a test double that returns arbitrary Properties.
type fakeResource struct {
    props map[string]any
}

func (r *fakeResource) Type() string              { return "test.resource" }
func (r *fakeResource) ID() string                { return "test-id" }
func (r *fakeResource) Properties() map[string]any { return r.props }
func (r *fakeResource) Raw() any                  { return nil }

func TestValueFilter_Contains(t *testing.T) {
    r := &fakeResource{props: map[string]any{
        "tags": []string{"env:prod", "team:platform"},
        "name": "High error rate - prod",
    }}

    tests := []struct {
        spec policy.FilterSpec
        want bool
    }{
        {
            policy.FilterSpec{"type": "value", "key": "tags", "op": "contains", "value": "env:prod"},
            true,
        },
        {
            policy.FilterSpec{"type": "value", "key": "tags", "op": "contains", "value": "env:staging"},
            false,
        },
        {
            policy.FilterSpec{"type": "value", "key": "name", "op": "regex", "value": "prod$"},
            true,
        },
        {
            policy.FilterSpec{"type": "value", "key": "name", "op": "not-regex", "value": "^\\[PROD\\]"},
            true,
        },
    }

    for _, tt := range tests {
        f, err := filter.Build(tt.spec)
        if err != nil {
            t.Fatalf("Build(%v): %v", tt.spec, err)
        }
        got, err := f.Match(r)
        if err != nil {
            t.Fatalf("Match: %v", err)
        }
        if got != tt.want {
            t.Errorf("spec=%v: got %v, want %v", tt.spec, got, tt.want)
        }
    }
}

func TestBooleanFilter_Not(t *testing.T) {
    r := &fakeResource{props: map[string]any{
        "name": "plain monitor name",
    }}

    spec := policy.FilterSpec{
        "not": []any{
            policy.FilterSpec{"type": "value", "key": "name", "op": "regex", "value": "^\\[PROD\\]"},
        },
    }

    f, err := filter.Build(spec)
    if err != nil {
        t.Fatalf("Build: %v", err)
    }
    got, err := f.Match(r)
    if err != nil {
        t.Fatalf("Match: %v", err)
    }
    if !got {
        t.Error("expected name without [PROD] prefix to match the not filter")
    }
}
```

Run unit tests in Docker (no credentials needed):

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  golang:1.26.4-alpine \
  go test ./internal/filter/... -v
```

Or locally if Go is installed:

```bash
go test ./internal/filter/... -v
go test ./internal/policy/... -v
go test ./internal/output/... -v
```

---

## Policy validation (no credentials)

`leash validate` parses and validates YAML policy files against all registered resource types, filter types, and action types — without making any Datadog API calls. Use this as the fast feedback loop during policy development.

```bash
# Validate all policies in a directory
leash validate --policy ./policies/

# Validate a single file
leash validate --policy ./policies/monitor-naming.yaml
```

What validation checks:
- Required fields (`name`, `resource`) are present
- `resource` type is registered (e.g. `datadog.monitor`)
- Every filter spec has a valid `type` (or `and`/`or`/`not` key)
- Every filter's `op` is recognized
- Every action spec has a valid `type`
- Action-specific required fields are present (e.g. `tag` requires `tags`)

What validation does **not** check:
- Whether the filter `key` actually exists on the resource (checked at runtime)
- Whether the Datadog API is reachable
- Whether the credentials have sufficient permissions

### Use in CI

Add a validation step to your PR pipeline — it requires no secrets and runs in seconds:

```yaml
# GitHub Actions example
- name: Validate Leash policies
  run: docker run --rm -v ${{ github.workspace }}/policies:/policies:ro ghcr.io/mbaitelman/leash:latest validate --policy /policies/
```

---

## Dry-run mode

`--dry-run=true` (the default) is Leash's primary safety mechanism. In dry-run mode:

- All **read** operations run normally (resources are fetched, filters are evaluated).
- All **mutating** actions (`tag`, `delete`) log their intended operation but do not call any write API.
- The `notify` action logs the message to stderr instead of sending it.
- The `report` action always runs regardless of dry-run state.
- The findings JSON still includes `actions_taken` entries, all with `"dry_run": true`.

This means you can point Leash at a production Datadog org, see exactly which resources would be affected, and review the findings before enabling live execution.

```bash
# Safe — reads only, no mutations
leash run --policy ./policies/ --dry-run=true

# Live — will mutate resources
leash run --policy ./policies/ --dry-run=false
```

**Tip:** Always run in dry-run first and review the findings JSON before enabling live mode. Pay particular attention to match counts — a policy matching thousands of resources is a signal to add more specific filters.

### Dry-run with text output

Use `--output-format text` to get a human-readable summary during development:

```bash
leash run --policy ./policies/ --output-format text
```

```
Leash Run  3f2a1b4c-...
Generated: 2026-06-18T12:00:00Z
Mode:      DRY RUN (no mutations)

POLICY                              RESOURCE          MATCHES
prod-monitors-must-have-team-tag    datadog.monitor   12
slo-missing-team-tag                datadog.slo       3

--- prod-monitors-must-have-team-tag ---
  12345678
  23456789
  ...
```

---

## Integration testing against Datadog

Integration tests exercise the full stack: policy loading, Datadog API calls, filter evaluation, and action execution.

### Prerequisites

- A Datadog organization (a sandbox/dev org is strongly recommended)
- An API key and Application key with read permissions
- At least one resource of each type you want to test against

### Running an integration test

```bash
export DD_API_KEY=your-api-key
export DD_APP_KEY=your-app-key
export DD_SITE=datadoghq.com

# 1. Validate first (fast, no API calls)
leash validate --policy ./policies/examples/

# 2. Dry run — see what would match
leash run --policy ./policies/examples/ --output-format text

# 3. Review the findings JSON
leash run --policy ./policies/examples/ --output-file /tmp/findings.json
cat /tmp/findings.json | python3 -m json.tool

# 4. Live run (only after reviewing dry-run output)
leash run --policy ./policies/examples/ --dry-run=false
```

### Testing a specific resource type

Narrow the test to a single policy file during development:

```bash
leash run --policy ./policies/examples/monitor-naming.yaml --output-format text
```

### Verifying filter correctness

To confirm a filter is selecting the right resources:

1. Run in dry-run mode and capture the JSON findings.
2. Check the `matches[].properties` of the matched resources — these are the field values Leash used to evaluate filters.
3. Cross-check a few resource IDs directly in the Datadog UI.

```bash
leash run --policy ./policies/examples/monitor-naming.yaml \
  | python3 -c "
import json, sys
r = json.load(sys.stdin)
for p in r['policies']:
    print(f\"{p['policy_name']}: {p['match_count']} matches\")
    for m in p['matches'][:3]:
        print(f\"  id={m['id']}  name={m['properties'].get('name','?')}\")
"
```

### Testing with Docker

```bash
docker run --rm \
  -e DD_API_KEY=$DD_API_KEY \
  -e DD_APP_KEY=$DD_APP_KEY \
  -e DD_SITE=$DD_SITE \
  -v $(pwd)/policies:/policies:ro \
  ghcr.io/mbaitelman/leash:latest run --policy /policies/examples/ --output-format text
```

---

## Testing new resource types

When adding a new resource type to `internal/resource/`, the recommended development workflow is:

1. **Implement the provider** — write `YourTypeProvider.List()` returning a slice of `Resource`.
2. **Unit test `Properties()`** — construct a synthetic SDK struct, call `Properties()`, and assert the expected keys and values are present.
3. **Dry-run smoke test** — run `leash run` with a simple `report`-only policy targeting the new type. Review the `properties` in the findings JSON to verify all expected keys are populated.
4. **Integration test filters** — write a policy with each filter type you intend to support (`value`, `tag`, `age`) and verify the match counts are correct.

Example unit test for a new resource type:

```go
// internal/resource/yourtype_test.go
package resource_test

import (
    "testing"
    "time"
    // import the SDK struct for your type
)

func TestYourTypeResource_Properties(t *testing.T) {
    now := time.Now()
    inner := sdkpkg.YourType{
        Id:       sdkpkg.PtrString("abc-123"),
        Name:     sdkpkg.PtrString("My Resource"),
        Modified: &now,
    }
    r := &yourTypeResource{inner: inner}

    props := r.Properties()

    if props["id"] != "abc-123" {
        t.Errorf("id: got %v", props["id"])
    }
    if props["name"] != "My Resource" {
        t.Errorf("name: got %v", props["name"])
    }
    if _, ok := props["modified"]; !ok {
        t.Error("expected 'modified' key in properties")
    }
}
```

---

## Common validation errors and fixes

| Error | Cause | Fix |
|---|---|---|
| `unknown resource type "datadog.foo"` | Typo in `resource:` field | Run `leash list-resources` for valid names |
| `filter spec missing 'type' field` | Filter is missing `type:` key | Add `type: value` (or `and`/`or`/`not`) |
| `unknown filter type "fuzzy"` | Unrecognized filter type | Valid types: `value`, `tag`, `age` |
| `not filter requires exactly one child` | `not:` has 0 or 2+ children | Use `and:` for multiple conditions |
| `age filter 'op' must be 'older-than' or 'newer-than'` | Invalid op | Check spelling |
| `tag action requires 'tags'` | Missing `tags:` list on tag action | Add a `tags:` list with at least one entry |
| `delete action: 'confirm: true' must be set` | Forgot to add `confirm: true` | Add `confirm: true` under the delete action |
