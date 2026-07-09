package resource

import (
	"reflect"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

// ── Tag helpers ───────────────────────────────────────────────────────────────

func TestMergeTags(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		newTags  []string
		want     []string
	}{
		{
			name:     "adds new tags",
			existing: []string{"env:prod"},
			newTags:  []string{"team:sre"},
			want:     []string{"env:prod", "team:sre"},
		},
		{
			name:     "skips duplicates",
			existing: []string{"env:prod", "team:sre"},
			newTags:  []string{"env:prod"},
			want:     []string{"env:prod", "team:sre"},
		},
		{
			name:     "deduplicates within new tags",
			existing: []string{"env:prod"},
			newTags:  []string{"team:sre", "team:sre"},
			want:     []string{"env:prod", "team:sre"},
		},
		{
			name:     "empty existing",
			existing: nil,
			newTags:  []string{"env:prod"},
			want:     []string{"env:prod"},
		},
		{
			name:     "empty new tags",
			existing: []string{"env:prod"},
			newTags:  nil,
			want:     []string{"env:prod"},
		},
		{
			name:     "both empty",
			existing: nil,
			newTags:  nil,
			want:     []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeTags(tc.existing, tc.newTags)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mergeTags(%v, %v) = %v, want %v", tc.existing, tc.newTags, got, tc.want)
			}
		})
	}
}

func TestRemoveTags(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		toRemove []string
		want     []string
	}{
		{
			name:     "removes matching tags",
			existing: []string{"env:prod", "team:sre"},
			toRemove: []string{"team:sre"},
			want:     []string{"env:prod"},
		},
		{
			name:     "ignores tags not present",
			existing: []string{"env:prod"},
			toRemove: []string{"team:sre"},
			want:     []string{"env:prod"},
		},
		{
			name:     "removes all",
			existing: []string{"env:prod", "team:sre"},
			toRemove: []string{"env:prod", "team:sre"},
			want:     []string{},
		},
		{
			name:     "empty existing",
			existing: nil,
			toRemove: []string{"env:prod"},
			want:     []string{},
		},
		{
			name:     "empty removal list",
			existing: []string{"env:prod"},
			toRemove: nil,
			want:     []string{"env:prod"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := removeTags(tc.existing, tc.toRemove)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("removeTags(%v, %v) = %v, want %v", tc.existing, tc.toRemove, got, tc.want)
			}
		})
	}
}

// ── monitorResource ───────────────────────────────────────────────────────────

func TestMonitorResource_Properties(t *testing.T) {
	m := datadogV1.Monitor{
		Id:    datadog.PtrInt64(42),
		Name:  datadog.PtrString("High error rate"),
		Type:  datadogV1.MONITORTYPE_METRIC_ALERT,
		Query: "avg(last_5m):avg:system.load.1{*} > 4",
		Tags:  []string{"env:prod", "team:sre"},
		Creator: &datadogV1.Creator{
			Email:  datadog.PtrString("alice@example.com"),
			Handle: datadog.PtrString("alice"),
		},
	}
	r := &monitorResource{inner: m}

	if got, want := r.Type(), "datadog.monitor"; got != want {
		t.Errorf("Type() = %q, want %q", got, want)
	}
	if got, want := r.ID(), "42"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}

	props := r.Properties()
	if got, want := props["id"], int64(42); got != want {
		t.Errorf("props[id] = %v, want %v", got, want)
	}
	if got, want := props["name"], "High error rate"; got != want {
		t.Errorf("props[name] = %v, want %v", got, want)
	}
	if got, want := props["type"], "metric alert"; got != want {
		t.Errorf("props[type] = %v, want %v", got, want)
	}
	if got, want := props["query"], "avg(last_5m):avg:system.load.1{*} > 4"; got != want {
		t.Errorf("props[query] = %v, want %v", got, want)
	}
	if got, want := props["tags"], []string{"env:prod", "team:sre"}; !reflect.DeepEqual(got, want) {
		t.Errorf("props[tags] = %v, want %v", got, want)
	}
	if got, want := props["creator.email"], "alice@example.com"; got != want {
		t.Errorf("props[creator.email] = %v, want %v", got, want)
	}
	if got, want := props["creator.handle"], "alice"; got != want {
		t.Errorf("props[creator.handle] = %v, want %v", got, want)
	}
	if _, ok := props["message"]; ok {
		t.Error("props[message] should be absent when Message is nil")
	}
	if _, ok := props["overall_state"]; ok {
		t.Error("props[overall_state] should be absent when OverallState is nil")
	}
}

func TestMonitorResource_ID_Empty(t *testing.T) {
	r := &monitorResource{inner: datadogV1.Monitor{}}
	if got := r.ID(); got != "" {
		t.Errorf("ID() = %q, want empty string for monitor without ID", got)
	}
}

// ── userResource ──────────────────────────────────────────────────────────────

func TestUserResource_Properties(t *testing.T) {
	u := datadogV2.User{
		Id: datadog.PtrString("abc-123"),
		Attributes: &datadogV2.UserAttributes{
			Email:    datadog.PtrString("bob@example.com"),
			Name:     *datadog.NewNullableString(datadog.PtrString("Bob Builder")),
			Disabled: datadog.PtrBool(false),
		},
	}
	r := &userResource{inner: u}

	if got, want := r.Type(), "datadog.user"; got != want {
		t.Errorf("Type() = %q, want %q", got, want)
	}
	if got, want := r.ID(), "abc-123"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}

	props := r.Properties()
	if got, want := props["id"], "abc-123"; got != want {
		t.Errorf("props[id] = %v, want %v", got, want)
	}
	if got, want := props["email"], "bob@example.com"; got != want {
		t.Errorf("props[email] = %v, want %v", got, want)
	}
	if got, want := props["name"], "Bob Builder"; got != want {
		t.Errorf("props[name] = %v, want %v", got, want)
	}
	if got, want := props["disabled"], false; got != want {
		t.Errorf("props[disabled] = %v, want %v", got, want)
	}
	if _, ok := props["title"]; ok {
		t.Error("props[title] should be absent when Title is unset")
	}
	if _, ok := props["status"]; ok {
		t.Error("props[status] should be absent when Status is nil")
	}
}
