package server_test

import (
	"context"
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
