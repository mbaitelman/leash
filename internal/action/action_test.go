package action_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mbaitelman/leash/internal/action"
)

// fakeResource implements resource.Resource for testing.
type fakeResource struct {
	id       string
	resType  string
	props    map[string]any
	addTagFn func([]string) error
	deleteFn func() error
}

func (r *fakeResource) Type() string               { return r.resType }
func (r *fakeResource) ID() string                 { return r.id }
func (r *fakeResource) Properties() map[string]any { return r.props }
func (r *fakeResource) Raw() any                   { return nil }

// AddTags makes fakeResource implement resource.Taggable.
func (r *fakeResource) AddTags(_ context.Context, tags []string) error {
	if r.addTagFn != nil {
		return r.addTagFn(tags)
	}
	return nil
}

// Delete makes fakeResource implement resource.Deletable.
func (r *fakeResource) Delete(_ context.Context) error {
	if r.deleteFn != nil {
		return r.deleteFn()
	}
	return nil
}

func resource(id string, props map[string]any) *fakeResource {
	return &fakeResource{id: id, resType: "fake.resource", props: props}
}

// ── Tag action ────────────────────────────────────────────────────────────────

func TestTagAction_MissingTagsField(t *testing.T) {
	factory, err := action.Get("tag")
	if err != nil {
		t.Fatal(err)
	}
	_, err = factory(map[string]any{"type": "tag"})
	if err == nil {
		t.Error("expected error when 'tags' field is missing")
	}
}

func TestTagAction_DryRun_NoAPICall(t *testing.T) {
	factory, _ := action.Get("tag")
	act, err := factory(map[string]any{"type": "tag", "tags": []any{"leash:flagged"}})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	called := false
	r := resource("abc", map[string]any{})
	r.addTagFn = func(_ []string) error {
		called = true
		return nil
	}

	if err := act.Execute(context.Background(), r, true); err != nil {
		t.Fatalf("Execute dry-run: %v", err)
	}
	if called {
		t.Error("AddTags should not be called in dry-run mode")
	}
}

func TestTagAction_SkipsAlreadyPresentTags(t *testing.T) {
	factory, _ := action.Get("tag")
	act, _ := factory(map[string]any{
		"type": "tag",
		"tags": []any{"env:prod", "leash:flagged"},
	})

	var received []string
	r := resource("abc", map[string]any{"tags": []string{"env:prod"}})
	r.addTagFn = func(tags []string) error {
		received = tags
		return nil
	}

	if err := act.Execute(context.Background(), r, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(received) != 1 || received[0] != "leash:flagged" {
		t.Errorf("expected only missing tag 'leash:flagged', got %v", received)
	}
}

func TestTagAction_AllTagsPresent_NoCall(t *testing.T) {
	factory, _ := action.Get("tag")
	act, _ := factory(map[string]any{
		"type": "tag",
		"tags": []any{"env:prod"},
	})

	called := false
	r := resource("abc", map[string]any{"tags": []string{"env:prod"}})
	r.addTagFn = func(_ []string) error {
		called = true
		return nil
	}

	if err := act.Execute(context.Background(), r, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called {
		t.Error("AddTags should not be called when all tags already present")
	}
}

func TestTagAction_NonTaggableResource(t *testing.T) {
	factory, _ := action.Get("tag")
	act, _ := factory(map[string]any{"type": "tag", "tags": []any{"leash:x"}})

	// plain fakeResource without AddTags — but it does implement Taggable.
	// Use a non-taggable type to test the error path.
	r := &nonTaggable{id: "x"}
	err := act.Execute(context.Background(), r, false)
	if err == nil {
		t.Error("expected error for non-taggable resource")
	}
}

// nonTaggable only implements resource.Resource, not resource.Taggable.
type nonTaggable struct{ id string }

func (r *nonTaggable) Type() string               { return "nt.resource" }
func (r *nonTaggable) ID() string                 { return r.id }
func (r *nonTaggable) Properties() map[string]any { return nil }
func (r *nonTaggable) Raw() any                   { return nil }

// ── Delete action ─────────────────────────────────────────────────────────────

func TestDeleteAction_UnconfirmedReturnsError(t *testing.T) {
	factory, _ := action.Get("delete")
	act, err := factory(map[string]any{"type": "delete"}) // no confirm
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	r := resource("abc", nil)
	err = act.Execute(context.Background(), r, false)
	if err == nil {
		t.Error("expected error when confirm is not set")
	}
}

func TestDeleteAction_DryRun_NoAPICall(t *testing.T) {
	factory, _ := action.Get("delete")
	act, _ := factory(map[string]any{"type": "delete", "confirm": true})

	called := false
	r := resource("abc", nil)
	r.deleteFn = func() error {
		called = true
		return nil
	}

	if err := act.Execute(context.Background(), r, true); err != nil {
		t.Fatalf("Execute dry-run: %v", err)
	}
	if called {
		t.Error("Delete should not be called in dry-run mode")
	}
}

func TestDeleteAction_CallsDelete(t *testing.T) {
	factory, _ := action.Get("delete")
	act, _ := factory(map[string]any{"type": "delete", "confirm": true})

	called := false
	r := resource("abc", nil)
	r.deleteFn = func() error {
		called = true
		return nil
	}

	if err := act.Execute(context.Background(), r, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("Delete was not called")
	}
}

func TestDeleteAction_NonDeletableResource(t *testing.T) {
	factory, _ := action.Get("delete")
	act, _ := factory(map[string]any{"type": "delete", "confirm": true})

	r := &nonTaggable{id: "x"}
	err := act.Execute(context.Background(), r, false)
	if err == nil {
		t.Error("expected error for non-deletable resource")
	}
}

func TestDeleteAction_PropagatesDeleteError(t *testing.T) {
	factory, _ := action.Get("delete")
	act, _ := factory(map[string]any{"type": "delete", "confirm": true})

	boom := errors.New("API error")
	r := resource("abc", nil)
	r.deleteFn = func() error { return boom }

	err := act.Execute(context.Background(), r, false)
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped boom error, got %v", err)
	}
}

// ── Report action ─────────────────────────────────────────────────────────────

func TestReportAction_Execute_NoError(t *testing.T) {
	factory, err := action.Get("report")
	if err != nil {
		t.Fatal(err)
	}
	act, err := factory(map[string]any{"type": "report"})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	r := resource("abc", map[string]any{"name": "my-monitor"})
	if err := act.Execute(context.Background(), r, true); err != nil {
		t.Errorf("Execute dry-run: %v", err)
	}
	if err := act.Execute(context.Background(), r, false); err != nil {
		t.Errorf("Execute live: %v", err)
	}
}

// ── Notify action ─────────────────────────────────────────────────────────────

func TestNotifyAction_DryRun_NoHTTP(t *testing.T) {
	factory, _ := action.Get("notify")
	act, _ := factory(map[string]any{"type": "notify", "webhook_url": "http://invalid.invalid"})

	r := resource("abc", map[string]any{"name": "my-monitor"})
	if err := act.Execute(context.Background(), r, true); err != nil {
		t.Errorf("Execute dry-run: %v", err)
	}
}

func TestNotifyAction_MissingWebhookLive(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK_URL", "")
	factory, _ := action.Get("notify")
	act, _ := factory(map[string]any{"type": "notify"})

	r := resource("abc", nil)
	err := act.Execute(context.Background(), r, false)
	if err == nil {
		t.Error("expected error when no webhook configured")
	}
}

func TestNotifyAction_PayloadIsSlackAttachment(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	factory, _ := action.Get("notify")
	act, _ := factory(map[string]any{"type": "notify", "webhook_url": srv.URL})

	res := resource("mon-123", map[string]any{
		"name":          "High error rate",
		"overall_state": "Alert",
		"tags":          []string{"env:prod", "team:platform"},
	})
	res.resType = "datadog.monitor"

	if err := act.Execute(context.Background(), res, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, body)
	}

	attachments, ok := payload["attachments"].([]any)
	if !ok || len(attachments) == 0 {
		t.Fatalf("expected attachments array, got: %s", body)
	}
	att := attachments[0].(map[string]any)

	if att["color"] != "warning" {
		t.Errorf("expected color=warning, got %v", att["color"])
	}
	text, _ := att["text"].(string)
	if !strings.Contains(text, "mon-123") {
		t.Errorf("expected resource ID in text, got: %s", text)
	}
	if !strings.Contains(text, "High error rate") {
		t.Errorf("expected resource name in text, got: %s", text)
	}

	fields, _ := att["fields"].([]any)
	var fieldTitles []string
	for _, f := range fields {
		fm := f.(map[string]any)
		fieldTitles = append(fieldTitles, fm["title"].(string))
	}
	for _, want := range []string{"Resource", "Status", "Tags"} {
		found := false
		for _, got := range fieldTitles {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected field %q in attachment fields %v", want, fieldTitles)
		}
	}
}

func TestNotifyAction_NoNameFallsBackGracefully(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	factory, _ := action.Get("notify")
	act, _ := factory(map[string]any{"type": "notify", "webhook_url": srv.URL})

	res := resource("dash-456", map[string]any{}) // no name/title/email
	if err := act.Execute(context.Background(), res, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty payload")
	}
}

func TestNotifyAction_WebhookNon2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	factory, _ := action.Get("notify")
	act, _ := factory(map[string]any{"type": "notify", "webhook_url": srv.URL})

	r := resource("abc", nil)
	if err := act.Execute(context.Background(), r, false); err == nil {
		t.Error("expected error for 5xx response")
	}
}

// ── Registry ──────────────────────────────────────────────────────────────────

func TestGet_UnknownType(t *testing.T) {
	_, err := action.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}
