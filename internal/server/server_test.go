package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbaitelman/leash/internal/server"
)

// newTestServer creates a Server on an OS-assigned port (":0") so tests never
// conflict with each other or with the running dev server.
func newTestServer(t *testing.T, schedule string) *server.Server {
	t.Helper()
	return server.New("0", t.TempDir(), []string{t.TempDir()}, true, schedule)
}

// newTestHandler returns the Server's HTTP handler plus the Server itself, with
// policies rooted at policyDir.
func newTestHandler(t *testing.T, policyDir string) (http.Handler, *server.Server) {
	t.Helper()
	srv := server.New("0", t.TempDir(), []string{policyDir}, true, "")
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler() error: %v", err)
	}
	return h, srv
}

// do performs a request against the handler and returns the recorded response.
func do(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// startAndStop calls Start with an already-cancelled context so the HTTP server
// shuts down immediately after binding. Returns the error Start produced.
func startAndStop(srv *server.Server) error {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return srv.Start(ctx)
}

func TestServer_InvalidCronSchedule_ReturnsError(t *testing.T) {
	srv := newTestServer(t, "not-a-cron-expression")
	err := startAndStop(srv)
	if err == nil {
		t.Fatal("expected error for invalid cron expression, got nil")
	}
	if !strings.Contains(err.Error(), "invalid cron schedule") {
		t.Errorf("error should mention 'invalid cron schedule', got: %v", err)
	}
}

func TestServer_ValidCronSchedule_NoError(t *testing.T) {
	srv := newTestServer(t, "* * * * *") // every minute — valid
	if err := startAndStop(srv); err != nil {
		t.Errorf("unexpected error with valid cron expression: %v", err)
	}
}

func TestServer_NoSchedule_NoError(t *testing.T) {
	srv := newTestServer(t, "")
	if err := startAndStop(srv); err != nil {
		t.Errorf("unexpected error with no schedule: %v", err)
	}
}

func TestTriggerRun_MalformedBody_Returns400(t *testing.T) {
	h, _ := newTestHandler(t, t.TempDir())
	w := do(h, http.MethodPost, "/api/runs", `{"dry_run": tru`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed body, got %d: %s", w.Code, w.Body)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !strings.Contains(resp["error"], "invalid request body") {
		t.Errorf("error should mention 'invalid request body', got: %q", resp["error"])
	}
}

func TestTriggerRun_RunInProgress_Returns409(t *testing.T) {
	h, srv := newTestHandler(t, t.TempDir())
	unlock := srv.LockRunMu() // simulate a run already executing
	defer unlock()

	w := do(h, http.MethodPost, "/api/runs", "")
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 while a run is in progress, got %d: %s", w.Code, w.Body)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp["error"] != "a run is already in progress" {
		t.Errorf("unexpected error message: %q", resp["error"])
	}
}

func TestTriggerRun_MissingCredentials_Returns503(t *testing.T) {
	t.Setenv("DD_API_KEY", "")
	t.Setenv("DD_APP_KEY", "")
	h, _ := newTestHandler(t, t.TempDir())
	w := do(h, http.MethodPost, "/api/runs", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for missing credentials, got %d: %s", w.Code, w.Body)
	}
}

func TestTriggerRun_InvalidPolicyFile_Returns400(t *testing.T) {
	dir := t.TempDir()
	// Written directly to disk, bypassing save-time validation.
	bad := "policies:\n  - resource: monitors\n" // missing 'name'
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, dir)
	w := do(h, http.MethodPost, "/api/runs", `{"policy_path":"bad.yaml"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid policy file, got %d: %s", w.Code, w.Body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h, _ := newTestHandler(t, t.TempDir())
	cases := []struct {
		method string
		target string
	}{
		{http.MethodPost, "/api/logs"},
		{http.MethodPost, "/api/logs/stream"},
		{http.MethodPost, "/api/config"},
		{http.MethodDelete, "/api/runs/some-id"},
		{http.MethodPut, "/api/runs"},
		{http.MethodPost, "/health"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			w := do(h, c.method, c.target, "")
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d: %s", w.Code, w.Body)
			}
		})
	}
}

func TestPolicySave_SchemaValidation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("policies: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, dir)

	put := func(content string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"content": content})
		return do(h, http.MethodPut, "/api/policies?path=test.yaml", string(body))
	}

	t.Run("missing name rejected", func(t *testing.T) {
		w := put("policies:\n  - resource: monitors\n")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body)
		}
		if !strings.Contains(w.Body.String(), "name") {
			t.Errorf("error should mention missing 'name', got: %s", w.Body)
		}
	})

	t.Run("missing resource rejected", func(t *testing.T) {
		w := put("policies:\n  - name: my-policy\n")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body)
		}
		if !strings.Contains(w.Body.String(), "resource") {
			t.Errorf("error should mention missing 'resource', got: %s", w.Body)
		}
	})

	t.Run("malformed structure rejected", func(t *testing.T) {
		w := put("just a string, not a policy file")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body)
		}
	})

	t.Run("valid policy accepted", func(t *testing.T) {
		w := put("policies:\n  - name: my-policy\n    resource: monitors\n")
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body)
		}
	})

	t.Run("empty policies list accepted", func(t *testing.T) {
		w := put("policies: []\n")
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body)
		}
	})
}

func TestServer_CommonCronExpressions(t *testing.T) {
	exprs := []string{
		"0 * * * *",      // hourly
		"*/30 * * * *",   // every 30 minutes
		"0 6 * * *",      // daily at 06:00
		"0 0 * * 0",      // weekly on Sunday
		"0 0 1 * *",      // monthly on the 1st
	}
	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			srv := newTestServer(t, expr)
			if err := startAndStop(srv); err != nil {
				t.Errorf("schedule %q should be valid, got error: %v", expr, err)
			}
		})
	}
}
