package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbaitelman/leash/internal/policy"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
	return path
}

const validYAML = `
policies:
  - name: test-policy
    resource: datadog.monitor
    filters:
      - type: value
        key: name
        op: present
    actions:
      - type: report
`

func TestLoadPaths_SingleFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "policy.yaml", validYAML)

	policies, err := policy.LoadPaths([]string{path})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if policies[0].Name != "test-policy" {
		t.Errorf("name: got %q, want %q", policies[0].Name, "test-policy")
	}
	if policies[0].Resource != "datadog.monitor" {
		t.Errorf("resource: got %q, want %q", policies[0].Resource, "datadog.monitor")
	}
}

func TestLoadPaths_Directory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", `
policies:
  - name: policy-a
    resource: datadog.monitor
`)
	writeFile(t, dir, "b.yml", `
policies:
  - name: policy-b
    resource: datadog.slo
`)
	writeFile(t, dir, "skip.txt", "not yaml")

	policies, err := policy.LoadPaths([]string{dir})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(policies))
	}
}

func TestLoadPaths_SubdirectoryRecursion(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "root.yaml", `
policies:
  - name: root-policy
    resource: datadog.monitor
`)
	writeFile(t, sub, "nested.yaml", `
policies:
  - name: nested-policy
    resource: datadog.slo
`)

	policies, err := policy.LoadPaths([]string{dir})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(policies))
	}
}

func TestLoadPaths_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.yaml", `
policies:
  - resource: datadog.monitor
`)
	_, err := policy.LoadPaths([]string{path})
	if err == nil {
		t.Error("expected error for missing 'name' field")
	}
}

func TestLoadPaths_MissingResource(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.yaml", `
policies:
  - name: no-resource-policy
`)
	_, err := policy.LoadPaths([]string{path})
	if err == nil {
		t.Error("expected error for missing 'resource' field")
	}
}

func TestLoadPaths_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.yaml", `{not: valid: yaml: [`)
	_, err := policy.LoadPaths([]string{path})
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadPaths_NonexistentPath(t *testing.T) {
	_, err := policy.LoadPaths([]string{"/nonexistent/path/to/policies"})
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestLoadPaths_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "empty.yaml", "")
	policies, err := policy.LoadPaths([]string{path})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("expected 0 policies from empty file, got %d", len(policies))
	}
}

func TestLoadPaths_MultiplePoliciesInOneFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "multi.yaml", `
policies:
  - name: policy-one
    resource: datadog.monitor
  - name: policy-two
    resource: datadog.slo
`)
	policies, err := policy.LoadPaths([]string{path})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(policies))
	}
}

func TestLoadPaths_ParamsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "params.yaml", `
policies:
  - name: audit-policy
    resource: datadog.audit_event
    params:
      query: "@evt.name:Dashboard"
      lookback: 24h
      max_events: 500
`)
	policies, err := policy.LoadPaths([]string{path})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	params := policies[0].Params
	if params["query"] != "@evt.name:Dashboard" {
		t.Errorf("query: got %v", params["query"])
	}
	if params["lookback"] != "24h" {
		t.Errorf("lookback: got %v", params["lookback"])
	}
	if params["max_events"] != 500 {
		t.Errorf("max_events: got %v (%T)", params["max_events"], params["max_events"])
	}
}
