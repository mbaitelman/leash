package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func init() {
	Register(&rumApplicationProvider{})
}

// ---- Provider ----

type rumApplicationProvider struct{}

func (p *rumApplicationProvider) ResourceType() string { return "datadog.rum_application" }

func (p *rumApplicationProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	api := datadogV2.NewRUMApi(client)
	// GetRUMApplications is not paginated: the endpoint takes no page
	// parameters and returns all applications in a single response.
	resp, _, err := api.GetRUMApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing RUM applications: %w", err)
	}

	apps := resp.GetData()
	resources := make([]Resource, 0, len(apps))
	for i := range apps {
		resources = append(resources, &rumApplicationResource{inner: apps[i]})
	}
	return resources, nil
}

// ---- Resource ----

type rumApplicationResource struct {
	inner datadogV2.RUMApplicationList
}

func (r *rumApplicationResource) Type() string { return "datadog.rum_application" }
func (r *rumApplicationResource) ID() string   { return r.inner.GetId() }
func (r *rumApplicationResource) Raw() any     { return r.inner }

func (r *rumApplicationResource) Properties() map[string]any {
	attrs := r.inner.GetAttributes()
	props := map[string]any{
		"name": attrs.Name,
		"type": attrs.Type,
	}
	if r.inner.Id != nil {
		props["id"] = *r.inner.Id
	}
	props["creator.email"] = attrs.CreatedByHandle
	props["created"] = time.UnixMilli(attrs.CreatedAt).UTC()
	props["updated"] = time.UnixMilli(attrs.UpdatedAt).UTC()
	props["updated_by_handle"] = attrs.UpdatedByHandle
	if attrs.IsActive != nil {
		props["is_active"] = *attrs.IsActive
	}
	if ps := attrs.ProductScales; ps != nil {
		if eps := ps.RumEventProcessingScale; eps != nil && eps.State != nil {
			props["product_scales.rum_processing_state"] = string(*eps.State)
		}
		if ars := ps.ProductAnalyticsRetentionScale; ars != nil && ars.State != nil {
			props["product_scales.analytics_retention_state"] = string(*ars.State)
		}
	}
	return props
}
