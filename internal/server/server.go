package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mbaitelman/leash/internal/output"
	"github.com/robfig/cron/v3"
)

//go:embed web
var webFiles embed.FS

// Server serves the Leash web UI and REST API.
type Server struct {
	host       string // bind address, empty = all interfaces
	port       string
	dryRun     bool
	policyDirs []string
	schedule   string // cron expression, empty = no scheduled runs
	store      *RunStore
	policies   *PolicyStore
	logBuf     *LogBuffer
	logLevel   slog.LevelVar // adjustable at runtime via PUT /api/log-level
	runMu      sync.Mutex    // serializes runs: scheduled and HTTP-triggered runs never overlap
}

func New(host, port, runsDir string, policyDirs []string, dryRun bool, schedule string) *Server {
	return &Server{
		host:       host,
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
		// Skip SSE stream (long-lived), health probes, and static file requests.
		if r.URL.Path == "/api/logs/stream" || r.URL.Path == "/health" || !strings.HasPrefix(r.URL.Path, "/api/") {
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

// csrfProtect rejects state-changing cross-origin requests to the API.
// Browsers attach an Origin header to cross-origin requests; if it is present
// and does not match the request's host, the request came from another site
// and is refused. Bodied requests must also declare a JSON content type, which
// stops "simple" cross-origin form/text POSTs that skip the CORS preflight.
func csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
		if safe || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host == "" || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "cross-origin request rejected")
				return
			}
		}
		if r.ContentLength != 0 {
			ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || ct != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Handler builds the HTTP handler serving the API and the embedded web UI.
func (s *Server) Handler() (http.Handler, error) {
	mux := http.NewServeMux()

	// API routes — order matters: specific prefix before generic
	mux.HandleFunc("/health", s.handleHealth)
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
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		last := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if !strings.Contains(last, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	return s.httpLogger(csrfProtect(mux)), nil
}

func (s *Server) Start(ctx context.Context) error {
	// Fail fast if run results cannot be persisted.
	if err := s.store.ensureDir(); err != nil {
		return err
	}

	if s.schedule != "" {
		c := cron.New()
		if _, err := c.AddFunc(s.schedule, func() {
			if !s.runMu.TryLock() {
				slog.Warn("skipping scheduled run: a run is already in progress", "schedule", s.schedule)
				return
			}
			defer s.runMu.Unlock()
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

	handler, err := s.Handler()
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:    s.host + ":" + s.port,
		Handler: handler,
		// No WriteTimeout/ReadTimeout: the SSE log stream is long-lived.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
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

// writeServerError logs the full error server-side and returns a generic 500
// so internal details (filesystem paths, upstream API errors) never reach the
// client.
func writeServerError(w http.ResponseWriter, err error) {
	slog.Error("internal server error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
