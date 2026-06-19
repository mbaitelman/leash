package server

import (
	"encoding/json"
	"fmt"
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
	os.MkdirAll(dir, 0o755) //nolint:errcheck
	return &RunStore{dir: dir}
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
		var r output.FindingsReport
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
			writeError(w, http.StatusInternalServerError, err.Error())
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
	run, err := s.store.Get(id)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "run not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
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
		return nil, fmt.Errorf("loading policies: %w", err)
	}

	client, ctx, err := config.BuildClient()
	if err != nil {
		return nil, fmt.Errorf("Datadog credentials not configured: %w", err)
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
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	paths := s.policyDirs
	if req.PolicyPath != "" {
		abs, err := s.policies.Resolve(req.PolicyPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		paths = []string{abs}
	}

	report, err := s.executeRun(paths, req.DryRun)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "credentials") {
			status = http.StatusServiceUnavailable
		} else if strings.Contains(err.Error(), "loading policies") {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, report)
}
