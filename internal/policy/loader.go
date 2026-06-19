package policy

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadPaths loads all policies from the given file or directory paths.
// Directories are scanned recursively for *.yaml and *.yml files.
func LoadPaths(paths []string) ([]Policy, error) {
	var all []Policy
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("policy path %q: %w", p, err)
		}
		var policies []Policy
		if info.IsDir() {
			policies, err = loadDir(p)
		} else {
			policies, err = loadFile(p)
		}
		if err != nil {
			return nil, err
		}
		all = append(all, policies...)
	}
	return all, nil
}

func loadDir(dir string) ([]Policy, error) {
	var all []Policy
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		policies, err := loadFile(path)
		if err != nil {
			return err
		}
		all = append(all, policies...)
		return nil
	})
	return all, err
}

func loadFile(path string) ([]Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy file %q: %w", path, err)
	}

	var pf PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing policy file %q: %w", path, err)
	}

	for i, p := range pf.Policies {
		if p.Name == "" {
			return nil, fmt.Errorf("policy at index %d in %q is missing required field 'name'", i, path)
		}
		if p.Resource == "" {
			return nil, fmt.Errorf("policy %q in %q is missing required field 'resource'", p.Name, path)
		}
	}

	return pf.Policies, nil
}
