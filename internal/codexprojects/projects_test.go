package codexprojects

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPreservesProjectOrderAndRoots(t *testing.T) {
	root := t.TempDir()
	data := []byte(`{
  "project-order": ["two", "one"],
  "local-projects": {
    "one": {"id":"one","name":"Alpha","rootPaths":["/work/a", "/work/a"]},
    "two": {"id":"two","name":"Beta","rootPaths":["/work/b"]},
    "three": {"id":"three","name":"Gamma","rootPaths":["/work/c"]},
    "empty": {"id":"empty","name":"Empty","rootPaths":[]}
  }
}`)
	if err := os.WriteFile(filepath.Join(root, globalStateName), data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	projects, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("projects = %d, want 3", len(projects))
	}
	if projects[0].ID != "two" || projects[1].ID != "one" || projects[2].ID != "three" {
		t.Fatalf("unexpected order: %+v", projects)
	}
	if len(projects[1].RootPaths) != 1 || projects[1].RootPaths[0] != "/work/a" {
		t.Fatalf("roots were not de-duplicated: %+v", projects[1].RootPaths)
	}
}

func TestLoadMissingCatalogErrors(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected missing catalog error")
	}
}
