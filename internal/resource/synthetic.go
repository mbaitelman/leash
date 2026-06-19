package resource

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func init() {
	Register(&syntheticProvider{})
}

// ---- Provider ----

type syntheticProvider struct{}

func (p *syntheticProvider) ResourceType() string { return "datadog.synthetic" }

func (p *syntheticProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	api := datadogV1.NewSyntheticsApi(client)
	resp, _, err := api.ListTests(ctx, *datadogV1.NewListTestsOptionalParameters())
	if err != nil {
		return nil, fmt.Errorf("listing synthetics: %w", err)
	}

	// Pre-fetch SLO monitor IDs so each synthetic knows if it's covered.
	linkedMonitors := sloLinkedMonitorIDs(ctx, client)

	tests := resp.GetTests()
	resources := make([]Resource, 0, len(tests))
	for i := range tests {
		resources = append(resources, &syntheticResource{
			inner:          tests[i],
			client:         client,
			linkedMonitors: linkedMonitors,
		})
	}
	return resources, nil
}

// sloLinkedMonitorIDs returns the set of monitor IDs that are referenced by at
// least one SLO. Used to determine whether a synthetic test is SLO-backed.
func sloLinkedMonitorIDs(ctx context.Context, client *datadog.APIClient) map[int64]struct{} {
	api := datadogV1.NewServiceLevelObjectivesApi(client)
	resp, _, err := api.ListSLOs(ctx, *datadogV1.NewListSLOsOptionalParameters())
	if err != nil {
		slog.Warn("failed to fetch SLOs for synthetic linkage check", "error", err)
		return map[int64]struct{}{}
	}
	ids := make(map[int64]struct{})
	for _, slo := range resp.GetData() {
		for _, mid := range slo.GetMonitorIds() {
			ids[mid] = struct{}{}
		}
	}
	return ids
}

// ---- Resource ----

type syntheticResource struct {
	inner          datadogV1.SyntheticsTestDetails
	client         *datadog.APIClient
	linkedMonitors map[int64]struct{}
}

func (r *syntheticResource) Type() string { return "datadog.synthetic" }
func (r *syntheticResource) ID() string   { return r.inner.GetPublicId() }
func (r *syntheticResource) Raw() any     { return r.inner }

func (r *syntheticResource) Properties() map[string]any {
	s := r.inner
	props := map[string]any{
		"tags": s.GetTags(),
	}
	if s.PublicId != nil {
		props["public_id"] = *s.PublicId
	}
	if s.Name != nil {
		props["name"] = *s.Name
	}
	if s.Type != nil {
		props["type"] = string(*s.Type)
	}
	if s.Status != nil {
		props["status"] = string(*s.Status)
	}
	if s.Creator != nil && s.Creator.Email != nil {
		props["creator.email"] = *s.Creator.Email
	}
	if s.MonitorId != nil {
		props["monitor_id"] = *s.MonitorId
		if _, ok := r.linkedMonitors[*s.MonitorId]; ok {
			props["synthetic.slo_linked"] = true
		}
	}
	return props
}

// Note: Synthetics tagging requires separate handling for API vs Browser test types.
// Taggable support for synthetics will be added in a future version.
