package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mbaitelman/leash/internal/config"
	"github.com/mbaitelman/leash/internal/engine"
	"github.com/mbaitelman/leash/internal/output"
	"github.com/mbaitelman/leash/internal/policy"
)

// errPolicyLoad marks failures to load or parse policy files so HTTP handlers
// can map them to a 400 response via errors.Is.
var errPolicyLoad = errors.New("loading policies")

// RunSummary is a lightweight row for the runs list.
type RunSummary struct {
	RunID        string    `json:"run_id"`
	GeneratedAt  time.Time `json:"generated_at"`
	DryRun       bool      `json:"dry_run"`
	PolicyCount  int       `json:"policy_count"`
	TotalMatches int       `json:"total_matches"`
}

// RunStore persists FindingsReports as JSON files in a directory.
type RunStore struct{ dir string }

func NewRunStore(dir string) *RunStore {
	return &RunStore{dir: dir}
}

// ensureDir creates the store's directory if it does not exist. Called from
// Server.Start so the server fails fast when runs cannot be persisted.
func (s *RunStore) ensureDir() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("creating runs directory %q: %w", s.dir, err)
	}
	return nil
}

func (s *RunStore) Save(r *output.FindingsReport) error {
	f, err := os.Create(filepath.Join(s.dir, r.RunID+".json"))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// runListFields is a minimal decode target for List — skips large Matches/Passing/ActionsTaken arrays.
type runListFields struct {
	RunID       string    `json:"run_id"`
	GeneratedAt time.Time `json:"generated_at"`
	DryRun      bool      `json:"dry_run"`
	Policies    []struct {
		MatchCount int `json:"match_count"`
	} `json:"policies"`
}

func (s *RunStore) List() ([]RunSummary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunSummary{}, nil
		}
		return nil, err
	}
	var out []RunSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var r runListFields
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		total := 0
		for _, p := range r.Policies {
			total += p.MatchCount
		}
		out = append(out, RunSummary{
			RunID:        r.RunID,
			GeneratedAt:  r.GeneratedAt,
			DryRun:       r.DryRun,
			PolicyCount:  len(r.Policies),
			TotalMatches: total,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GeneratedAt.After(out[j].GeneratedAt)
	})
	return out, nil
}

func (s *RunStore) Get(id string) (*output.FindingsReport, error) {
	if strings.ContainsAny(id, `/\`) {
		return nil, fmt.Errorf("invalid run ID")
	}
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var r output.FindingsReport
	return &r, json.Unmarshal(data, &r)
}

// handleRuns handles GET /api/runs (list) and POST /api/runs (trigger).
func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		summaries, err := s.store.List()
		if err != nil {
			writeServerError(w, err)
			return
		}
		writeJSON(w, summaries)
	case http.MethodPost:
		s.triggerRun(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleRunByID handles GET /api/runs/{id}.
func (s *Server) handleRunByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	if id == "" {
		s.handleRuns(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	run, err := s.store.Get(id)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "run not found")
		} else {
			writeServerError(w, err)
		}
		return
	}
	writeJSON(w, run)
}

// executeRun loads policies from the given paths (or s.policyDirs if empty),
// runs the engine, saves the report, and returns it. Used by both the HTTP
// handler and the scheduled runner.
func (s *Server) executeRun(policyPaths []string, dryRun bool) (*output.FindingsReport, error) {
	policies, err := policy.LoadPaths(policyPaths)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPolicyLoad, err)
	}

	client, ctx, err := config.BuildClient()
	if err != nil {
		return nil, err // wraps config.ErrCredentials
	}

	report, err := engine.New(ctx, client).Run(policies, dryRun)
	if err != nil {
		return nil, err
	}

	if err := s.store.Save(report); err != nil {
		slog.Warn("failed to save run", "error", err)
	}
	return report, nil
}

func (s *Server) triggerRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DryRun     bool   `json:"dry_run"`
		PolicyPath string `json:"policy_path"` // optional: run a single file
	}
	req.DryRun = s.dryRun
	// An empty body means "no overrides"; anything else must be valid JSON.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	paths := s.policyDirs
	if req.PolicyPath != "" {
		abs, err := s.policies.Resolve(req.PolicyPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		paths = []string{abs}
	}

	// Only one run (scheduled or HTTP-triggered) may execute at a time.
	if !s.runMu.TryLock() {
		writeError(w, http.StatusConflict, "a run is already in progress")
		return
	}
	defer s.runMu.Unlock()

	report, err := s.executeRun(paths, req.DryRun)
	if err != nil {
		switch {
		case errors.Is(err, config.ErrCredentials):
			writeError(w, http.StatusServiceUnavailable, err.Error())
		case errors.Is(err, errPolicyLoad):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeServerError(w, err)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, report)
}
