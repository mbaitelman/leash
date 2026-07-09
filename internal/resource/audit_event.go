package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/mbaitelman/leash/internal/timeutil"
)

func init() {
	Register(&auditEventProvider{})
}

const (
	defaultAuditLookback  = 24 * time.Hour
	defaultAuditMaxEvents = 1000
	maxAuditPageLimit     = 1000
)

// ---- Params ----

// auditEventParams holds the policy-level `params:` for datadog.audit_event.
type auditEventParams struct {
	Query     string        // server-side Audit Logs search query
	Lookback  time.Duration // window ending at `to`; mutually exclusive with From
	From      string        // RFC3339 or Datadog date math, passed through
	To        string        // RFC3339 or Datadog date math, defaults to "now"
	MaxEvents int           // hard cap on fetched events
}

func parseAuditEventParams(params map[string]any) (auditEventParams, error) {
	p := auditEventParams{
		Lookback:  defaultAuditLookback,
		To:        "now",
		MaxEvents: defaultAuditMaxEvents,
	}

	var haveLookback bool
	for key, raw := range params {
		switch key {
		case "query":
			s, ok := raw.(string)
			if !ok {
				return p, fmt.Errorf("'query' must be a string, got %T", raw)
			}
			p.Query = s
		case "lookback":
			s, ok := raw.(string)
			if !ok {
				return p, fmt.Errorf("'lookback' must be a duration string (e.g. \"24h\", \"7d\"), got %T", raw)
			}
			dur, err := timeutil.ParseDuration(s)
			if err != nil {
				return p, fmt.Errorf("'lookback': %w", err)
			}
			if dur <= 0 {
				return p, fmt.Errorf("'lookback' must be positive, got %q", s)
			}
			p.Lookback = dur
			haveLookback = true
		case "from":
			s, ok := raw.(string)
			if !ok {
				return p, fmt.Errorf("'from' must be a string, got %T", raw)
			}
			p.From = s
		case "to":
			s, ok := raw.(string)
			if !ok {
				return p, fmt.Errorf("'to' must be a string, got %T", raw)
			}
			p.To = s
		case "max_events":
			n, ok := toInt(raw)
			if !ok || n <= 0 {
				return p, fmt.Errorf("'max_events' must be a positive integer, got %v", raw)
			}
			p.MaxEvents = n
		default:
			return p, fmt.Errorf("unknown param %q (supported: query, lookback, from, to, max_events)", key)
		}
	}

	if haveLookback && p.From != "" {
		return p, fmt.Errorf("'lookback' and 'from' are mutually exclusive")
	}
	return p, nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

// ---- Provider ----

type auditEventProvider struct{}

func (p *auditEventProvider) ResourceType() string { return "datadog.audit_event" }

func (p *auditEventProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	return p.ListWithParams(ctx, client, nil)
}

func (p *auditEventProvider) ValidateParams(params map[string]any) error {
	_, err := parseAuditEventParams(params)
	return err
}

func (p *auditEventProvider) ListWithParams(ctx context.Context, client *datadog.APIClient, params map[string]any) ([]Resource, error) {
	parsed, err := parseAuditEventParams(params)
	if err != nil {
		return nil, err
	}

	from := parsed.From
	if from == "" {
		from = time.Now().UTC().Add(-parsed.Lookback).Format(time.RFC3339)
	}

	queryFilter := datadogV2.NewAuditLogsQueryFilter()
	queryFilter.SetFrom(from)
	queryFilter.SetTo(parsed.To)
	if parsed.Query != "" {
		queryFilter.SetQuery(parsed.Query)
	}

	pageLimit := int32(min(parsed.MaxEvents, maxAuditPageLimit))
	body := datadogV2.AuditLogsSearchEventsRequest{
		Filter: queryFilter,
		Page:   &datadogV2.AuditLogsQueryPageOptions{Limit: &pageLimit},
		Sort:   datadogV2.AUDITLOGSSORT_TIMESTAMP_ASCENDING.Ptr(),
	}

	api := datadogV2.NewAuditApi(client)
	items, cancel := api.SearchAuditLogsWithPagination(ctx,
		*datadogV2.NewSearchAuditLogsOptionalParameters().WithBody(body))
	defer cancel()

	var resources []Resource
	for item := range items {
		if item.Error != nil {
			return nil, fmt.Errorf("searching audit logs: %w", item.Error)
		}
		resources = append(resources, &auditEventResource{inner: item.Item})
		if len(resources) >= parsed.MaxEvents {
			// Stop the SDK pagination goroutine and drain its channel so it
			// can exit; post-cancel sends select on ctx.Done.
			cancel()
			for range items { //nolint:revive
			}
			break
		}
	}
	return resources, nil
}

// ---- Resource ----

type auditEventResource struct {
	inner datadogV2.AuditLogsEvent
}

func (r *auditEventResource) Type() string { return "datadog.audit_event" }

func (r *auditEventResource) ID() string { return r.inner.GetId() }

func (r *auditEventResource) Raw() any { return r.inner }

func (r *auditEventResource) Properties() map[string]any {
	props := map[string]any{}
	if r.inner.Id != nil {
		props["id"] = *r.inner.Id
	}
	attrs := r.inner.Attributes
	if attrs == nil {
		return props
	}
	if attrs.Message != nil {
		props["message"] = *attrs.Message
	}
	if attrs.Service != nil {
		props["service"] = *attrs.Service
	}
	if attrs.Tags != nil {
		props["tags"] = attrs.Tags
	}
	if attrs.Timestamp != nil {
		props["timestamp"] = *attrs.Timestamp
	}
	for k, v := range attrs.Attributes {
		flattenInto(props, "attributes."+k, v)
	}
	return props
}
