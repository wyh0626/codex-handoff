// Package codexprojects reads Codex Desktop's saved local project catalog.
// It is intentionally read-only and treats the global-state schema as optional:
// callers can fall back to session cwd discovery when the file is absent or has
// changed in a future Codex release.
package codexprojects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const globalStateName = ".codex-global-state.json"

type Project struct {
	ID        string
	Name      string
	RootPaths []string
}

type projectRecord struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	RootPaths []string `json:"rootPaths"`
}

type globalState struct {
	ProjectOrder  []string                 `json:"project-order"`
	LocalProjects map[string]projectRecord `json:"local-projects"`
}

// Load returns saved local projects in the same order Codex Desktop records.
// Projects without a usable root path are ignored.
func Load(codexRoot string) ([]Project, error) {
	data, err := os.ReadFile(filepath.Join(codexRoot, globalStateName))
	if err != nil {
		return nil, fmt.Errorf("read Codex project catalog: %w", err)
	}
	var state globalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse Codex project catalog: %w", err)
	}
	if len(state.LocalProjects) == 0 {
		return nil, fmt.Errorf("Codex project catalog contains no local projects")
	}

	seen := map[string]bool{}
	projects := make([]Project, 0, len(state.LocalProjects))
	appendProject := func(key string, record projectRecord) {
		if seen[key] {
			return
		}
		seen[key] = true
		roots := uniqueNonEmpty(record.RootPaths)
		if len(roots) == 0 {
			return
		}
		id := record.ID
		if id == "" {
			id = key
		}
		name := record.Name
		if name == "" {
			name = filepath.Base(roots[0])
		}
		projects = append(projects, Project{ID: id, Name: name, RootPaths: roots})
	}

	for _, id := range state.ProjectOrder {
		if record, ok := state.LocalProjects[id]; ok {
			appendProject(id, record)
		}
	}
	var remaining []string
	for id := range state.LocalProjects {
		if !seen[id] {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(remaining)
	for _, id := range remaining {
		appendProject(id, state.LocalProjects[id])
	}
	return projects, nil
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
