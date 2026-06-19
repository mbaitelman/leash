package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/google/uuid"
	"github.com/mbaitelman/leash/internal/action"
	"github.com/mbaitelman/leash/internal/filter"
	"github.com/mbaitelman/leash/internal/output"
	"github.com/mbaitelman/leash/internal/policy"
	"github.com/mbaitelman/leash/internal/resource"
)

// Engine executes policies against the Datadog API.
type Engine struct {
	client *datadog.APIClient
	ctx    context.Context
}

// New creates a new Engine with the given Datadog client and context.
func New(ctx context.Context, client *datadog.APIClient) *Engine {
	return &Engine{client: client, ctx: ctx}
}

// Run executes all policies and returns a FindingsReport.
func (e *Engine) Run(policies []policy.Policy, dryRun bool) (*output.FindingsReport, error) {
	report := &output.FindingsReport{
		RunID:       uuid.New().String(),
		GeneratedAt: time.Now().UTC(),
		DryRun:      dryRun,
	}

	for _, pol := range policies {
		slog.Info("running policy", "name", pol.Name, "resource", pol.Resource)
		result, err := e.runPolicy(pol, dryRun)
		if err != nil {
			return nil, fmt.Errorf("policy %q: %w", pol.Name, err)
		}
		report.Policies = append(report.Policies, *result)
		slog.Info("policy complete", "name", pol.Name, "matches", result.MatchCount)
	}

	return report, nil
}

func (e *Engine) runPolicy(pol policy.Policy, dryRun bool) (*output.PolicyResult, error) {
	provider, err := resource.Get(pol.Resource)
	if err != nil {
		return nil, err
	}

	resources, err := provider.List(e.ctx, e.client)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", pol.Resource, err)
	}

	filters, err := filter.BuildChain(pol.Filters)
	if err != nil {
		return nil, fmt.Errorf("building filters: %w", err)
	}

	actions, err := buildActions(pol.Actions)
	if err != nil {
		return nil, fmt.Errorf("building actions: %w", err)
	}

	result := &output.PolicyResult{
		PolicyName:   pol.Name,
		Resource:     pol.Resource,
		Matches:      []output.ResourceMatch{},
		Passing:      []output.ResourceMatch{},
		ActionsTaken: []output.ActionRecord{},
	}

	for _, r := range resources {
		matched, err := applyFilters(r, filters)
		if err != nil {
			return nil, fmt.Errorf("filtering resource %s: %w", r.ID(), err)
		}
		if !matched {
			result.Passing = append(result.Passing, output.ResourceMatch{
				ID:         r.ID(),
				Properties: r.Properties(),
			})
			continue
		}

		result.Matches = append(result.Matches, output.ResourceMatch{
			ID:         r.ID(),
			Properties: r.Properties(),
		})

		for _, act := range actions {
			record := output.ActionRecord{
				ResourceID: r.ID(),
				ActionType: act.Type(),
				DryRun:     dryRun,
			}
			if err := act.Execute(e.ctx, r, dryRun); err != nil {
				record.Success = false
				record.Error = err.Error()
				slog.Error("action failed", "action", act.Type(), "resource_id", r.ID(), "error", err)
			} else {
				record.Success = true
			}
			result.ActionsTaken = append(result.ActionsTaken, record)
		}
	}

	result.MatchCount = len(result.Matches)
	result.PassCount = len(result.Passing)
	return result, nil
}

func applyFilters(r resource.Resource, filters []filter.Filter) (bool, error) {
	for _, f := range filters {
		ok, err := f.Match(r)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func buildActions(specs []policy.ActionSpec) ([]action.Action, error) {
	actions := make([]action.Action, 0, len(specs))
	for _, spec := range specs {
		m := map[string]any(spec)
		actionType, ok := m["type"].(string)
		if !ok || actionType == "" {
			return nil, fmt.Errorf("action spec missing 'type' field")
		}
		factory, err := action.Get(actionType)
		if err != nil {
			return nil, err
		}
		act, err := factory(m)
		if err != nil {
			return nil, fmt.Errorf("building action %q: %w", actionType, err)
		}
		actions = append(actions, act)
	}
	return actions, nil
}
