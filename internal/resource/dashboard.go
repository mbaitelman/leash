package resource

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func init() {
	Register(&dashboardProvider{})
}

// ---- Provider ----

type dashboardProvider struct{}

func (p *dashboardProvider) ResourceType() string { return "datadog.dashboard" }

func (p *dashboardProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	api := datadogV1.NewDashboardsApi(client)
	resp, _, err := api.ListDashboards(ctx, *datadogV1.NewListDashboardsOptionalParameters())
	if err != nil {
		return nil, fmt.Errorf("listing dashboards: %w", err)
	}

	dashboards := resp.GetDashboards()
	resources := make([]Resource, 0, len(dashboards))
	for i := range dashboards {
		resources = append(resources, &dashboardResource{inner: dashboards[i], client: client})
	}
	return resources, nil
}

// ---- Resource ----

type dashboardResource struct {
	inner  datadogV1.DashboardSummaryDefinition
	client *datadog.APIClient
}

func (r *dashboardResource) Type() string { return "datadog.dashboard" }
func (r *dashboardResource) ID() string   { return r.inner.GetId() }
func (r *dashboardResource) Raw() any     { return r.inner }

func (r *dashboardResource) Properties() map[string]any {
	d := r.inner
	props := map[string]any{
		"title":       d.GetTitle(),
		"layout_type": string(d.GetLayoutType()),
	}
	if d.Id != nil {
		props["id"] = *d.Id
	}
	if d.AuthorHandle != nil {
		props["author_handle"] = *d.AuthorHandle
	}
	if v := d.Description.Get(); v != nil {
		props["description"] = *v
	}
	if d.CreatedAt != nil {
		props["created"] = *d.CreatedAt
	}
	if d.ModifiedAt != nil {
		props["modified"] = *d.ModifiedAt
	}
	if d.Url != nil {
		props["url"] = *d.Url
	}
	return props
}
