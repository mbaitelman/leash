package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// Attr is an ordered key-value pair preserved from a slog Record.
type Attr struct {
	Key   string `json:"k"`
	Value string `json:"v"`
}

// LogEntry is a single structured log event stored in the buffer.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"msg"`
	Attrs   []Attr    `json:"attrs,omitempty"`
}

const maxLogEntries = 500

// LogBuffer is a thread-safe ring buffer for log entries with SSE fan-out.
type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	subs    map[chan LogEntry]struct{}
}

func newLogBuffer() *LogBuffer {
	return &LogBuffer{subs: make(map[chan LogEntry]struct{})}
}

func (b *LogBuffer) append(e LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, e)
	if len(b.entries) > maxLogEntries {
		b.entries = b.entries[len(b.entries)-maxLogEntries:]
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default: // drop if subscriber is slow
		}
	}
}

func (b *LogBuffer) recent(n int) []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || n > len(b.entries) {
		n = len(b.entries)
	}
	out := make([]LogEntry, n)
	copy(out, b.entries[len(b.entries)-n:])
	return out
}

func (b *LogBuffer) subscribe() chan LogEntry {
	ch := make(chan LogEntry, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *LogBuffer) unsubscribe(ch chan LogEntry) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// logHandler implements slog.Handler — writes to both the buffer and an inner handler.
type logHandler struct {
	buf   *LogBuffer
	inner slog.Handler
}

func (h *logHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *logHandler) Handle(ctx context.Context, r slog.Record) error {
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	e := LogEntry{Time: t, Level: r.Level.String(), Message: r.Message}
	r.Attrs(func(a slog.Attr) bool {
		e.Attrs = append(e.Attrs, Attr{Key: a.Key, Value: a.Value.String()})
		return true
	})
	h.buf.append(e)
	return h.inner.Handle(ctx, r)
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &logHandler{buf: h.buf, inner: h.inner.WithAttrs(attrs)}
}

func (h *logHandler) WithGroup(name string) slog.Handler {
	return &logHandler{buf: h.buf, inner: h.inner.WithGroup(name)}
}

// SlogHandler sets the initial log level and returns a handler whose effective
// level can be changed at runtime via PUT /api/log-level.
func (s *Server) SlogHandler(initialLevel slog.Level) slog.Handler {
	s.logLevel.Set(initialLevel)
	return &logHandler{
		buf:   s.logBuf,
		inner: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &s.logLevel}),
	}
}

// handleLogLevel handles GET /api/log-level and PUT /api/log-level.
func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]string{"level": s.logLevel.Level().String()})
	case http.MethodPut:
		var body struct {
			Level string `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		var level slog.Level
		if err := level.UnmarshalText([]byte(body.Level)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid level: use DEBUG, INFO, WARN, or ERROR")
			return
		}
		s.logLevel.Set(level)
		slog.Info("log level changed", "level", level.String())
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleLogs handles GET /api/logs — returns recent log entries.
func (s *Server) handleLogs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.logBuf.recent(200))
}

// handleLogStream handles GET /api/logs/stream — SSE stream of log entries.
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for _, e := range s.logBuf.recent(100) {
		data, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ch := s.logBuf.subscribe()
	defer s.logBuf.unsubscribe(ch)

	for {
		select {
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleConfig handles GET /api/config — exposes runtime config to the frontend.
func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	site := os.Getenv("DD_SITE")
	if site == "" {
		site = "datadoghq.com"
	}
	writeJSON(w, map[string]string{
		"app_url":  "https://app." + site,
		"site":     site,
		"schedule": s.schedule,
	})
}
