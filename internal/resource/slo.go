package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func init() {
	Register(&sloProvider{})
}

// ---- Provider ----

type sloProvider struct{}

func (p *sloProvider) ResourceType() string { return "datadog.slo" }

func (p *sloProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(client)
	resp, _, err := api.ListSLOs(ctx, *datadogV1.NewListSLOsOptionalParameters())
	if err != nil {
		return nil, fmt.Errorf("listing SLOs: %w", err)
	}

	data := resp.GetData()
	resources := make([]Resource, 0, len(data))
	for i := range data {
		resources = append(resources, &sloResource{inner: data[i], client: client})
	}
	return resources, nil
}

// ---- Resource ----

type sloResource struct {
	inner  datadogV1.ServiceLevelObjective
	client *datadog.APIClient
}

func (r *sloResource) Type() string { return "datadog.slo" }
func (r *sloResource) ID() string   { return r.inner.GetId() }
func (r *sloResource) Raw() any     { return r.inner }

func (r *sloResource) Properties() map[string]any {
	s := r.inner
	props := map[string]any{
		"name": s.GetName(),
		"tags": s.GetTags(),
		"type": string(s.GetType()),
	}
	if s.Id != nil {
		props["id"] = *s.Id
	}
	if v := s.Description.Get(); v != nil {
		props["description"] = *v
	}
	if s.CreatedAt != nil {
		props["created"] = time.Unix(*s.CreatedAt, 0).UTC()
	}
	if s.ModifiedAt != nil {
		props["modified"] = time.Unix(*s.ModifiedAt, 0).UTC()
	}
	if s.Creator != nil && s.Creator.Email != nil {
		props["creator.email"] = *s.Creator.Email
	}
	return props
}

// AddTags merges new tags into the SLO's existing tag list.
func (r *sloResource) AddTags(ctx context.Context, tags []string) error {
	merged := mergeTags(r.inner.GetTags(), tags)
	// Copy the full SLO and update only the tags to avoid losing other fields.
	body := r.inner
	body.Tags = merged
	api := datadogV1.NewServiceLevelObjectivesApi(r.client)
	_, _, err := api.UpdateSLO(ctx, r.inner.GetId(), body)
	return err
}
