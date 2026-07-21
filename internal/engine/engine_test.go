package engine_test

import (
	"context"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/mbaitelman/leash/internal/engine"
	"github.com/mbaitelman/leash/internal/policy"
	"github.com/mbaitelman/leash/internal/resource"
)

// fakeRes is a minimal resource.Resource for engine tests.
type fakeRes struct {
	id    string
	props map[string]any
}

func (r *fakeRes) Type() string               { return "fake.engine" }
func (r *fakeRes) ID() string                 { return r.id }
func (r *fakeRes) Properties() map[string]any { return r.props }
func (r *fakeRes) Raw() any                   { return nil }

// fakeProvider is a resource.Provider that returns a fixed slice.
type fakeProvider struct {
	resources []resource.Resource
}

func (p *fakeProvider) ResourceType() string { return "fake.engine" }
func (p *fakeProvider) List(_ context.Context, _ *datadog.APIClient) ([]resource.Resource, error) {
	return p.resources, nil
}

func registerFakeProvider(resources []resource.Resource) {
	resource.Register(&fakeProvider{resources: resources})
}

// fakeParamProvider is a resource.ParameterizedProvider that records the
// params it receives.
type fakeParamProvider struct {
	resources    []resource.Resource
	gotParams    map[string]any
	calledParams bool
}

func (p *fakeParamProvider) ResourceType() string { return "fake.params" }
func (p *fakeParamProvider) List(_ context.Context, _ *datadog.APIClient) ([]resource.Resource, error) {
	return p.resources, nil
}
func (p *fakeParamProvider) ListWithParams(_ context.Context, _ *datadog.APIClient, params map[string]any) ([]resource.Resource, error) {
	p.calledParams = true
	p.gotParams = params
	return p.resources, nil
}
func (p *fakeParamProvider) ValidateParams(map[string]any) error { return nil }

func TestEngine_Run_AllMatch(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "a", props: map[string]any{"env": "prod"}},
		&fakeRes{id: "b", props: map[string]any{"env": "prod"}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{
			Name:     "all-match",
			Resource: "fake.engine",
			Filters:  []policy.FilterSpec{{"type": "value", "key": "env", "op": "eq", "value": "prod"}},
			Actions:  []policy.ActionSpec{{"type": "report"}},
		},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Policies) != 1 {
		t.Fatalf("expected 1 policy result, got %d", len(report.Policies))
	}
	result := report.Policies[0]
	if result.MatchCount != 2 {
		t.Errorf("match_count: got %d, want 2", result.MatchCount)
	}
	if result.PassCount != 0 {
		t.Errorf("pass_count: got %d, want 0", result.PassCount)
	}
}

func TestEngine_Run_NoneMatch(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "a", props: map[string]any{"env": "staging"}},
		&fakeRes{id: "b", props: map[string]any{"env": "dev"}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{
			Name:     "none-match",
			Resource: "fake.engine",
			Filters:  []policy.FilterSpec{{"type": "value", "key": "env", "op": "eq", "value": "prod"}},
			Actions:  []policy.ActionSpec{{"type": "report"}},
		},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := report.Policies[0]
	if result.MatchCount != 0 {
		t.Errorf("match_count: got %d, want 0", result.MatchCount)
	}
	if result.PassCount != 2 {
		t.Errorf("pass_count: got %d, want 2", result.PassCount)
	}
}

func TestEngine_Run_PartialMatch(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "match-1", props: map[string]any{"env": "prod"}},
		&fakeRes{id: "pass-1", props: map[string]any{"env": "staging"}},
		&fakeRes{id: "match-2", props: map[string]any{"env": "prod"}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{
			Name:     "partial",
			Resource: "fake.engine",
			Filters:  []policy.FilterSpec{{"type": "value", "key": "env", "op": "eq", "value": "prod"}},
			Actions:  []policy.ActionSpec{{"type": "report"}},
		},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := report.Policies[0]
	if result.MatchCount != 2 {
		t.Errorf("match_count: got %d, want 2", result.MatchCount)
	}
	if result.PassCount != 1 {
		t.Errorf("pass_count: got %d, want 1", result.PassCount)
	}
}

func TestEngine_Run_NoFilters_AllMatch(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "x", props: map[string]any{}},
		&fakeRes{id: "y", props: map[string]any{}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{Name: "no-filters", Resource: "fake.engine"},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Policies[0].MatchCount != 2 {
		t.Errorf("expected 2 matches with no filters, got %d", report.Policies[0].MatchCount)
	}
}

func TestEngine_Run_DryRunFlag(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "z", props: map[string]any{}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{Name: "dry-check", Resource: "fake.engine", Actions: []policy.ActionSpec{{"type": "report"}}},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !report.DryRun {
		t.Error("expected DryRun=true in report")
	}
	if report.RunID == "" {
		t.Error("expected non-empty RunID")
	}
}

func TestEngine_Run_UnknownResourceType(t *testing.T) {
	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{Name: "bad", Resource: "no.such.type"},
	}
	_, err := e.Run(policies, true)
	if err == nil {
		t.Error("expected error for unknown resource type")
	}
}

func TestEngine_Run_ParamsPassedToProvider(t *testing.T) {
	fp := &fakeParamProvider{resources: []resource.Resource{
		&fakeRes{id: "e1", props: map[string]any{}},
	}}
	resource.Register(fp)

	e := engine.New(context.Background(), nil)
	params := map[string]any{"query": "@evt.name:Dashboard", "lookback": "24h"}
	policies := []policy.Policy{
		{Name: "with-params", Resource: "fake.params", Params: params},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !fp.calledParams {
		t.Fatal("expected ListWithParams to be called")
	}
	if fp.gotParams["query"] != "@evt.name:Dashboard" {
		t.Errorf("params not passed through: got %v", fp.gotParams)
	}
	if report.Policies[0].MatchCount != 1 {
		t.Errorf("match_count: got %d, want 1", report.Policies[0].MatchCount)
	}
}

func TestEngine_Run_ParamsRejectedByPlainProvider(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "a", props: map[string]any{}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{Name: "bad-params", Resource: "fake.engine", Params: map[string]any{"lookback": "24h"}},
	}
	_, err := e.Run(policies, true)
	if err == nil {
		t.Error("expected error for params on a non-parameterized provider")
	}
}

func TestEngine_Run_ActionRecorded(t *testing.T) {
	registerFakeProvider([]resource.Resource{
		&fakeRes{id: "r1", props: map[string]any{}},
	})

	e := engine.New(context.Background(), nil)
	policies := []policy.Policy{
		{
			Name:     "with-action",
			Resource: "fake.engine",
			Actions:  []policy.ActionSpec{{"type": "report"}},
		},
	}

	report, err := e.Run(policies, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := report.Policies[0]
	if len(result.ActionsTaken) != 1 {
		t.Fatalf("expected 1 action record, got %d", len(result.ActionsTaken))
	}
	rec := result.ActionsTaken[0]
	if rec.ActionType != "report" {
		t.Errorf("action_type: got %q, want %q", rec.ActionType, "report")
	}
	if !rec.Success {
		t.Error("expected action success=true")
	}
	if !rec.DryRun {
		t.Error("expected action dry_run=true")
	}
}
