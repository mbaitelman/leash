package resource

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

// ── Params parsing ────────────────────────────────────────────────────────────

func TestParseAuditEventParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]any
		want    auditEventParams
		wantErr string
	}{
		{
			name:   "nil params applies defaults",
			params: nil,
			want:   auditEventParams{Lookback: 24 * time.Hour, To: "now", MaxEvents: 1000},
		},
		{
			name:   "lookback in days",
			params: map[string]any{"lookback": "7d"},
			want:   auditEventParams{Lookback: 7 * 24 * time.Hour, To: "now", MaxEvents: 1000},
		},
		{
			name:   "query and max_events",
			params: map[string]any{"query": "@evt.name:Dashboard", "max_events": 50},
			want:   auditEventParams{Query: "@evt.name:Dashboard", Lookback: 24 * time.Hour, To: "now", MaxEvents: 50},
		},
		{
			name:   "from and to passthrough",
			params: map[string]any{"from": "now-2d", "to": "now-1d"},
			want:   auditEventParams{Lookback: 24 * time.Hour, From: "now-2d", To: "now-1d", MaxEvents: 1000},
		},
		{
			name:    "lookback and from conflict",
			params:  map[string]any{"lookback": "24h", "from": "now-1d"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "unknown key rejected",
			params:  map[string]any{"lookbak": "24h"},
			wantErr: "unknown param",
		},
		{
			name:    "bad duration",
			params:  map[string]any{"lookback": "24x"},
			wantErr: "lookback",
		},
		{
			name:    "non-positive max_events",
			params:  map[string]any{"max_events": 0},
			wantErr: "max_events",
		},
		{
			name:    "non-integer max_events",
			params:  map[string]any{"max_events": "many"},
			wantErr: "max_events",
		},
		{
			name:    "non-string query",
			params:  map[string]any{"query": 5},
			wantErr: "query",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAuditEventParams(tc.params)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// ── Resource properties ───────────────────────────────────────────────────────

func TestAuditEventResource_Properties(t *testing.T) {
	ts := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	ev := datadogV2.AuditLogsEvent{
		Id: datadog.PtrString("evt-123"),
		Attributes: &datadogV2.AuditLogsEventAttributes{
			Message:   datadog.PtrString("Dashboard deleted"),
			Service:   datadog.PtrString("dashboard"),
			Tags:      []string{"source:audit"},
			Timestamp: &ts,
			Attributes: map[string]any{
				"action": "deleted",
				"evt":    map[string]any{"name": "Dashboard"},
				"usr":    map[string]any{"email": "x@y.com"},
			},
		},
	}
	r := &auditEventResource{inner: ev}

	if r.Type() != "datadog.audit_event" {
		t.Errorf("type: got %q", r.Type())
	}
	if r.ID() != "evt-123" {
		t.Errorf("id: got %q", r.ID())
	}

	props := r.Properties()
	checks := map[string]any{
		"id":                   "evt-123",
		"message":              "Dashboard deleted",
		"service":              "dashboard",
		"attributes.action":    "deleted",
		"attributes.evt.name":  "Dashboard",
		"attributes.usr.email": "x@y.com",
	}
	for key, want := range checks {
		if got, ok := props[key]; !ok || got != want {
			t.Errorf("props[%q]: got %v (present=%v), want %v", key, got, ok, want)
		}
	}
	if got, ok := props["timestamp"].(time.Time); !ok || !got.Equal(ts) {
		t.Errorf("timestamp: got %v (time.Time=%v)", props["timestamp"], ok)
	}
	if tags, ok := props["tags"].([]string); !ok || len(tags) != 1 || tags[0] != "source:audit" {
		t.Errorf("tags: got %v", props["tags"])
	}
}

func TestAuditEventResource_Properties_NilAttributes(t *testing.T) {
	r := &auditEventResource{inner: datadogV2.AuditLogsEvent{Id: datadog.PtrString("evt-1")}}
	props := r.Properties()
	if props["id"] != "evt-1" {
		t.Errorf("id: got %v", props["id"])
	}
	if len(props) != 1 {
		t.Errorf("expected only id, got %v", props)
	}
}

// ── Provider against a mock API server ────────────────────────────────────────

func auditPage(ids []string, afterCursor string) map[string]any {
	data := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		data = append(data, map[string]any{
			"id":   id,
			"type": "audit",
			"attributes": map[string]any{
				"service":    "dashboard",
				"attributes": map[string]any{"evt": map[string]any{"name": "Dashboard"}},
			},
		})
	}
	page := map[string]any{}
	if afterCursor != "" {
		page["after"] = afterCursor
	}
	return map[string]any{"data": data, "meta": map[string]any{"page": page}}
}

func newAuditTestClient(t *testing.T, handler http.HandlerFunc) *datadog.APIClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := datadog.NewConfiguration()
	cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
	return datadog.NewAPIClient(cfg)
}

func TestAuditEventProvider_ListWithParams(t *testing.T) {
	var requests []map[string]any
	client := newAuditTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		requests = append(requests, req)

		w.Header().Set("Content-Type", "application/json")
		var resp map[string]any
		if len(requests) == 1 {
			resp = auditPage([]string{"evt-1", "evt-2"}, "cursor-1")
		} else {
			resp = auditPage([]string{"evt-3"}, "")
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	p := &auditEventProvider{}
	resources, err := p.ListWithParams(context.Background(), client, map[string]any{
		"query":      "@evt.name:Dashboard",
		"from":       "now-2d",
		"to":         "now",
		"max_events": 100,
	})
	if err != nil {
		t.Fatalf("ListWithParams: %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}
	if resources[0].ID() != "evt-1" || resources[2].ID() != "evt-3" {
		t.Errorf("unexpected ids: %s, %s", resources[0].ID(), resources[2].ID())
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(requests))
	}

	filter, _ := requests[0]["filter"].(map[string]any)
	if filter["query"] != "@evt.name:Dashboard" || filter["from"] != "now-2d" || filter["to"] != "now" {
		t.Errorf("first request filter: got %v", filter)
	}
	page2, _ := requests[1]["page"].(map[string]any)
	if page2["cursor"] != "cursor-1" {
		t.Errorf("second request should carry cursor-1, got %v", page2)
	}
}

func TestAuditEventProvider_MaxEventsStopsEarly(t *testing.T) {
	var requestCount int
	client := newAuditTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		// Every page advertises another cursor: without the cap this would
		// paginate forever.
		_ = json.NewEncoder(w).Encode(auditPage([]string{"evt-a", "evt-b"}, "next"))
	})

	p := &auditEventProvider{}
	done := make(chan struct{})
	var resources []Resource
	var err error
	go func() {
		resources, err = p.ListWithParams(context.Background(), client, map[string]any{"max_events": 3})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("ListWithParams hung: max_events early-stop did not terminate pagination")
	}
	if err != nil {
		t.Fatalf("ListWithParams: %v", err)
	}
	if len(resources) != 3 {
		t.Errorf("expected 3 resources (capped), got %d", len(resources))
	}
	if requestCount < 2 {
		t.Errorf("expected at least 2 requests before cap, got %d", requestCount)
	}
}

func TestAuditEventProvider_APIError(t *testing.T) {
	client := newAuditTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":["forbidden"]}`))
	})

	p := &auditEventProvider{}
	_, err := p.ListWithParams(context.Background(), client, nil)
	if err == nil {
		t.Fatal("expected error from API failure")
	}
}

func TestAuditEventProvider_ValidateParams(t *testing.T) {
	p := &auditEventProvider{}
	if err := p.ValidateParams(nil); err != nil {
		t.Errorf("nil params should validate: %v", err)
	}
	if err := p.ValidateParams(map[string]any{"lookback": "bogus"}); err == nil {
		t.Error("expected error for bad lookback")
	}
}
