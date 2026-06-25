package resource

import (
	"context"
	"fmt"
	"strconv"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func init() {
	Register(&monitorProvider{})
}

// ---- Provider ----

type monitorProvider struct{}

func (p *monitorProvider) ResourceType() string { return "datadog.monitor" }

func (p *monitorProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	api := datadogV1.NewMonitorsApi(client)
	monitors, _, err := api.ListMonitors(ctx, *datadogV1.NewListMonitorsOptionalParameters().WithPageSize(1000))
	if err != nil {
		return nil, fmt.Errorf("listing monitors: %w", err)
	}

	resources := make([]Resource, 0, len(monitors))
	for i := range monitors {
		resources = append(resources, &monitorResource{inner: monitors[i], client: client})
	}
	return resources, nil
}

// ---- Resource ----

type monitorResource struct {
	inner  datadogV1.Monitor
	client *datadog.APIClient
}

func (r *monitorResource) Type() string { return "datadog.monitor" }

func (r *monitorResource) ID() string {
	if r.inner.Id != nil {
		return strconv.FormatInt(*r.inner.Id, 10)
	}
	return ""
}

func (r *monitorResource) Raw() any { return r.inner }

func (r *monitorResource) Properties() map[string]any {
	m := r.inner
	props := map[string]any{
		"tags":  m.GetTags(),
		"type":  string(m.GetType()),
		"query": m.GetQuery(),
	}
	if m.Id != nil {
		props["id"] = *m.Id
	}
	if m.Name != nil {
		props["name"] = *m.Name
	}
	if m.Message != nil {
		props["message"] = *m.Message
	}
	if m.Created != nil {
		props["created"] = *m.Created
	}
	if m.Modified != nil {
		props["modified"] = *m.Modified
	}
	if m.OverallState != nil {
		props["overall_state"] = string(*m.OverallState)
	}
	if opts := m.Options; opts != nil {
		if opts.NotifyNoData != nil {
			props["options.notify_no_data"] = *opts.NotifyNoData
		}
		if opts.RequireFullWindow != nil {
			props["options.require_full_window"] = *opts.RequireFullWindow
		}
		if opts.Thresholds != nil && opts.Thresholds.Critical != nil {
			props["options.thresholds.critical"] = *opts.Thresholds.Critical
		}
	}
	if c := m.Creator; c != nil {
		if c.Email != nil {
			props["creator.email"] = *c.Email
		}
		if c.Handle != nil {
			props["creator.handle"] = *c.Handle
		}
	}
	return props
}

// AddTags merges new tags into the monitor's existing tag list.
func (r *monitorResource) AddTags(ctx context.Context, tags []string) error {
	merged := mergeTags(r.inner.GetTags(), tags)
	body := datadogV1.MonitorUpdateRequest{Tags: merged}
	api := datadogV1.NewMonitorsApi(r.client)
	_, _, err := api.UpdateMonitor(ctx, r.inner.GetId(), body)
	return err
}

// RemoveTags removes the specified tags from the monitor's existing tag list.
func (r *monitorResource) RemoveTags(ctx context.Context, tags []string) error {
	filtered := removeTags(r.inner.GetTags(), tags)
	body := datadogV1.MonitorUpdateRequest{Tags: filtered}
	api := datadogV1.NewMonitorsApi(r.client)
	_, _, err := api.UpdateMonitor(ctx, r.inner.GetId(), body)
	return err
}

// Delete removes the monitor.
func (r *monitorResource) Delete(ctx context.Context) error {
	api := datadogV1.NewMonitorsApi(r.client)
	_, _, err := api.DeleteMonitor(ctx, r.inner.GetId(), *datadogV1.NewDeleteMonitorOptionalParameters())
	return err
}

func mergeTags(existing, newTags []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(newTags))
	for _, t := range existing {
		seen[t] = struct{}{}
	}
	result := append([]string{}, existing...)
	for _, t := range newTags {
		if _, ok := seen[t]; !ok {
			result = append(result, t)
			seen[t] = struct{}{}
		}
	}
	return result
}

func removeTags(existing, toRemove []string) []string {
	skip := make(map[string]struct{}, len(toRemove))
	for _, t := range toRemove {
		skip[t] = struct{}{}
	}
	result := make([]string, 0, len(existing))
	for _, t := range existing {
		if _, ok := skip[t]; !ok {
			result = append(result, t)
		}
	}
	return result
}
