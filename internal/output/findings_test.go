package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mbaitelman/leash/internal/output"
)

func sampleReport() *output.FindingsReport {
	return &output.FindingsReport{
		RunID:       "run-abc-123",
		GeneratedAt: time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
		DryRun:      true,
		Policies: []output.PolicyResult{
			{
				PolicyName: "prod-monitors-must-have-team-tag",
				Resource:   "datadog.monitor",
				MatchCount: 2,
				Matches: []output.ResourceMatch{
					{ID: "111", Properties: map[string]any{"name": "alert A"}},
					{ID: "222", Properties: map[string]any{"name": "alert B"}},
				},
				PassCount: 1,
				Passing: []output.ResourceMatch{
					{ID: "333", Properties: map[string]any{"name": "ok monitor"}},
				},
				ActionsTaken: []output.ActionRecord{
					{ResourceID: "111", ActionType: "report", DryRun: true, Success: true},
				},
			},
		},
	}
}

func TestWriteJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, sampleReport()); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
}

func TestWriteJSON_RoundTrip(t *testing.T) {
	report := sampleReport()
	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got output.FindingsReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.RunID != report.RunID {
		t.Errorf("run_id: got %q, want %q", got.RunID, report.RunID)
	}
	if got.DryRun != report.DryRun {
		t.Errorf("dry_run: got %v, want %v", got.DryRun, report.DryRun)
	}
	if len(got.Policies) != 1 {
		t.Fatalf("policies: got %d, want 1", len(got.Policies))
	}
	if got.Policies[0].MatchCount != 2 {
		t.Errorf("match_count: got %d, want 2", got.Policies[0].MatchCount)
	}
	if got.Policies[0].PassCount != 1 {
		t.Errorf("pass_count: got %d, want 1", got.Policies[0].PassCount)
	}
}

func TestWriteJSON_ContainsExpectedFields(t *testing.T) {
	var buf bytes.Buffer
	output.WriteJSON(&buf, sampleReport())
	body := buf.String()

	for _, want := range []string{
		`"run_id"`, `"run-abc-123"`,
		`"dry_run"`, `"match_count"`, `"pass_count"`,
		`"policy_name"`, `"prod-monitors-must-have-team-tag"`,
		`"actions_taken"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestWriteText_ContainsRunID(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteText(&buf, sampleReport()); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(buf.String(), "run-abc-123") {
		t.Errorf("text output missing run ID\n%s", buf.String())
	}
}

func TestWriteText_DryRunMode(t *testing.T) {
	var buf bytes.Buffer
	output.WriteText(&buf, sampleReport())
	if !strings.Contains(buf.String(), "DRY RUN") {
		t.Error("expected 'DRY RUN' in text output")
	}
}

func TestWriteText_LiveMode(t *testing.T) {
	r := sampleReport()
	r.DryRun = false
	var buf bytes.Buffer
	output.WriteText(&buf, r)
	if !strings.Contains(buf.String(), "LIVE") {
		t.Error("expected 'LIVE' in text output")
	}
}

func TestWriteText_ListsMatchedIDs(t *testing.T) {
	var buf bytes.Buffer
	output.WriteText(&buf, sampleReport())
	body := buf.String()
	if !strings.Contains(body, "111") {
		t.Error("expected matched resource ID '111' in text output")
	}
	if !strings.Contains(body, "222") {
		t.Error("expected matched resource ID '222' in text output")
	}
}

func TestWriteText_SkipsZeroMatchPolicies(t *testing.T) {
	r := &output.FindingsReport{
		RunID:       "x",
		GeneratedAt: time.Now(),
		Policies: []output.PolicyResult{
			{PolicyName: "no-matches", Resource: "datadog.monitor", MatchCount: 0},
		},
	}
	var buf bytes.Buffer
	output.WriteText(&buf, r)
	if strings.Contains(buf.String(), "--- no-matches ---") {
		t.Error("detail block should not appear for zero-match policy")
	}
}

func TestWriteText_PolicyTable(t *testing.T) {
	var buf bytes.Buffer
	output.WriteText(&buf, sampleReport())
	body := buf.String()
	if !strings.Contains(body, "POLICY") || !strings.Contains(body, "RESOURCE") || !strings.Contains(body, "MATCHES") {
		t.Error("expected table headers in text output")
	}
	if !strings.Contains(body, "prod-monitors-must-have-team-tag") {
		t.Error("expected policy name in table")
	}
}
