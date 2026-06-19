package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PolicyFile is a YAML policy file discovered on disk.
type PolicyFile struct {
	Path string `json:"path"` // forward-slash relative display path
	Size int64  `json:"size"`
	abs  string // absolute path, not exposed in JSON
}

// PolicyStore discovers and manages policy YAML files under configured roots.
type PolicyStore struct{ roots []string }

func NewPolicyStore(roots []string) *PolicyStore {
	return &PolicyStore{roots: roots}
}

func (s *PolicyStore) List() []PolicyFile {
	var out []PolicyFile
	for _, root := range s.roots {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			out = append(out, PolicyFile{
				Path: filepath.ToSlash(filepath.Base(root)),
				Size: info.Size(),
				abs:  root,
			})
			continue
		}
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error { //nolint:errcheck
			if err != nil || d.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			fi, _ := d.Info()
			out = append(out, PolicyFile{
				Path: filepath.ToSlash(rel),
				Size: fi.Size(),
				abs:  path,
			})
			return nil
		})
	}
	return out
}

// Resolve converts a display path to an absolute path.
// Only paths discovered through List() are accepted, preventing path traversal.
func (s *PolicyStore) Resolve(displayPath string) (string, error) {
	// Normalise separators so Windows paths compare correctly
	displayPath = strings.ReplaceAll(displayPath, `\`, "/")
	for _, f := range s.List() {
		if f.Path == displayPath {
			return f.abs, nil
		}
	}
	return "", fmt.Errorf("policy file not found: %q", displayPath)
}

func (s *PolicyStore) Read(displayPath string) (string, error) {
	abs, err := s.Resolve(displayPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	return string(data), err
}

func (s *PolicyStore) Write(displayPath, content string) error {
	var dummy any
	if err := yaml.Unmarshal([]byte(content), &dummy); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	abs, err := s.Resolve(displayPath)
	if err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), 0o644)
}

// handlePolicies handles GET /api/policies and GET|PUT /api/policies?path=...
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	if path == "" {
		writeJSON(w, s.policies.List())
		return
	}

	switch r.Method {
	case http.MethodGet:
		content, err := s.policies.Read(path)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, map[string]string{"content": content})

	case http.MethodPut:
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := s.policies.Write(path, body.Content); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
