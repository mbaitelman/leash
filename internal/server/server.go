package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mbaitelman/leash/internal/output"
	"github.com/robfig/cron/v3"
)

//go:embed web
var webFiles embed.FS

// Server serves the Leash web UI and REST API.
type Server struct {
	port       string
	dryRun     bool
	policyDirs []string
	schedule   string // cron expression, empty = no scheduled runs
	store      *RunStore
	policies   *PolicyStore
	logBuf     *LogBuffer
	logLevel   slog.LevelVar // adjustable at runtime via PUT /api/log-level
}

func New(port, runsDir string, policyDirs []string, dryRun bool, schedule string) *Server {
	return &Server{
		port:       port,
		dryRun:     dryRun,
		policyDirs: policyDirs,
		schedule:   schedule,
		store:      NewRunStore(runsDir),
		policies:   NewPolicyStore(policyDirs),
		logBuf:     newLogBuffer(),
	}
}

// statusRecorder captures the HTTP status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// httpLogger is a middleware that logs each API request with ANSI colors in the
// terminal and also pushes an entry to the in-memory log buffer for the UI.
func (s *Server) httpLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip SSE stream (long-lived) and static file requests.
		if r.URL.Path == "/api/logs/stream" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, code: 200}
		start := time.Now()
		next.ServeHTTP(rec, r)
		dur := time.Since(start)

		color := "\033[32m" // green  — 2xx
		if rec.code >= 500 {
			color = "\033[31m" // red    — 5xx
		} else if rec.code >= 400 {
			color = "\033[33m" // yellow — 4xx
		}
		fmt.Fprintf(os.Stderr, "%s %3d \033[0m| %8s | %-6s %s\n",
			color, rec.code, dur.Round(time.Millisecond), r.Method, r.URL.Path)

		s.logBuf.append(LogEntry{
			Time:    time.Now(),
			Level:   "INFO",
			Message: "request",
			Attrs: []Attr{
				{Key: "method", Value: r.Method},
				{Key: "path",   Value: r.URL.Path},
				{Key: "status", Value: strconv.Itoa(rec.code)},
				{Key: "dur",    Value: dur.Round(time.Millisecond).String()},
			},
		})
	})
}

func (s *Server) Start(ctx context.Context) error {
	if s.schedule != "" {
		c := cron.New()
		if _, err := c.AddFunc(s.schedule, func() {
			slog.Info("scheduled run starting", "schedule", s.schedule)
			report, err := s.executeRun(s.policyDirs, s.dryRun)
			if err != nil {
				slog.Error("scheduled run failed", "error", err)
				return
			}
			slog.Info("scheduled run complete", "run_id", report.RunID, "matches", totalMatches(report))
		}); err != nil {
			return fmt.Errorf("invalid cron schedule %q: %w", s.schedule, err)
		}
		c.Start()
		defer c.Stop()
		slog.Info("scheduled runs enabled", "schedule", s.schedule)
	}

	mux := http.NewServeMux()

	// API routes — order matters: specific prefix before generic
	mux.HandleFunc("/api/runs/", s.handleRunByID)
	mux.HandleFunc("/api/runs", s.handleRuns)
	mux.HandleFunc("/api/policies", s.handlePolicies)
	mux.HandleFunc("/api/logs/stream", s.handleLogStream)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/log-level", s.handleLogLevel)

	// Static SPA — serve index.html for any path without a file extension
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		last := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if !strings.Contains(last, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	srv := &http.Server{Addr: ":" + s.port, Handler: s.httpLogger(mux)}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func totalMatches(r *output.FindingsReport) int {
	n := 0
	for _, p := range r.Policies {
		n += p.MatchCount
	}
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}
