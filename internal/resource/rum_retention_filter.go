package resource

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func init() {
	Register(&rumRetentionFilterProvider{})
}

// ---- Provider ----

type rumRetentionFilterProvider struct{}

func (p *rumRetentionFilterProvider) ResourceType() string { return "datadog.rum_retention_filter" }

func (p *rumRetentionFilterProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	rumAPI := datadogV2.NewRUMApi(client)
	appsResp, _, err := rumAPI.GetRUMApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing RUM applications for retention filters: %w", err)
	}

	filterAPI := datadogV2.NewRumRetentionFiltersApi(client)
	var resources []Resource
	for _, app := range appsResp.GetData() {
		appID := app.GetId()
		resp, _, err := filterAPI.ListRetentionFilters(ctx, appID)
		if err != nil {
			slog.Warn("failed to list retention filters", "app_id", appID, "error", err)
			continue
		}
		for i := range resp.GetData() {
			resources = append(resources, &rumRetentionFilterResource{
				inner: resp.GetData()[i],
				appID: appID,
			})
		}
	}
	return resources, nil
}

// ---- Resource ----

type rumRetentionFilterResource struct {
	inner datadogV2.RumRetentionFilterData
	appID string
}

func (r *rumRetentionFilterResource) Type() string { return "datadog.rum_retention_filter" }
func (r *rumRetentionFilterResource) ID() string   { return r.inner.GetId() }
func (r *rumRetentionFilterResource) Raw() any     { return r.inner }

func (r *rumRetentionFilterResource) Properties() map[string]any {
	attrs := r.inner.GetAttributes()
	props := map[string]any{
		"app_id": r.appID,
	}
	if r.inner.Id != nil {
		props["id"] = *r.inner.Id
	}
	if attrs.Name != nil {
		props["name"] = *attrs.Name
	}
	if attrs.Enabled != nil {
		props["enabled"] = *attrs.Enabled
	}
	if attrs.EventType != nil {
		props["event_type"] = string(*attrs.EventType)
	}
	if attrs.Query != nil {
		props["query"] = *attrs.Query
	}
	if attrs.SampleRate != nil {
		props["sample_rate"] = *attrs.SampleRate
	}
	return props
}
