package bundle

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ahmojo/codex-claude-transfer/internal/codexhome"
)

func fakeHome(t *testing.T) codexhome.Home {
	t.Helper()
	home, err := codexhome.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	return home
}

func writeSession(t *testing.T, dir, name, threadID, cwd string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cwdJSON, _ := json.Marshal(cwd)
	body := `{"timestamp":"x","type":"session_meta","payload":{"id":"` + threadID + `","cwd":` + string(cwdJSON) + `,"source":"cli","model_provider":"openai","cli_version":"1.2.3"}}` + "\n" +
		`{"timestamp":"y","type":"event_msg","payload":{"type":"user_message","message":"hello"}}` + "\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readBundle opens a .codexbundle ZIP and returns a map of file name -> bytes.
func readBundle(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open bundle: %v", err)
	}
	defer zr.Close()
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		out[f.Name] = data
	}
	return out
}

func TestExportCreatesValidBundle(t *testing.T) {
	home := fakeHome(t)
	project := "/Users/example/dev/project"
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-22-01-aaaa1111-2222-3333-4444-555566667777.jsonl",
		"aaaa1111-2222-3333-4444-555566667777", project)

	out := filepath.Join(t.TempDir(), "project.codexbundle")
	result, err := Export(home, ExportOptions{ProjectPath: project, OutputPath: out})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.IncludedCount != 1 {
		t.Fatalf("included = %d, want 1", result.IncludedCount)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("bundle not written: %v", err)
	}

	files := readBundle(t, out)
	if _, ok := files[ManifestName]; !ok {
		t.Errorf("missing %s", ManifestName)
	}
	if _, ok := files[ChecksumsName]; !ok {
		t.Errorf("missing %s", ChecksumsName)
	}
	wantSession := "sessions/2026/06/13/rollout-2026-06-13T18-22-01-aaaa1111-2222-3333-4444-555566667777.jsonl"
	if _, ok := files[wantSession]; !ok {
		t.Errorf("missing session file %q; have %v", wantSession, keys(files))
	}
}

func TestExportManifestCorrect(t *testing.T) {
	home := fakeHome(t)
	project := "/proj/x"
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-22-01-bbbb1111-2222-3333-4444-555566667777.jsonl",
		"bbbb1111-2222-3333-4444-555566667777", project)

	out := filepath.Join(t.TempDir(), "x.codexbundle")
	if _, err := Export(home, ExportOptions{ProjectPath: project, OutputPath: out}); err != nil {
		t.Fatalf("export: %v", err)
	}

	files := readBundle(t, out)
	var m Manifest
	if err := json.Unmarshal(files[ManifestName], &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if m.FormatVersion != FormatVersion {
		t.Errorf("format version = %q", m.FormatVersion)
	}
	if m.SourceCodexHome != home.Root {
		t.Errorf("source codex home = %q", m.SourceCodexHome)
	}
	if m.SourceProjectPath != project {
		t.Errorf("source project = %q", m.SourceProjectPath)
	}
	if m.CodexVersion != "1.2.3" {
		t.Errorf("codex version = %q", m.CodexVersion)
	}
	if len(m.Sessions) != 1 {
		t.Fatalf("sessions = %d", len(m.Sessions))
	}
	s := m.Sessions[0]
	if s.ThreadID != "bbbb1111-2222-3333-4444-555566667777" {
		t.Errorf("thread id = %q", s.ThreadID)
	}
	if s.OriginalCWD != project {
		t.Errorf("original cwd = %q", s.OriginalCWD)
	}
	if s.ModelProvider != "openai" {
		t.Errorf("model provider = %q", s.ModelProvider)
	}
	if s.SHA256 == "" || s.SizeBytes == 0 {
		t.Errorf("missing checksum/size: %+v", s)
	}
}

func TestExportChecksumsCorrect(t *testing.T) {
	home := fakeHome(t)
	project := "/proj/y"
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	name := "rollout-2026-06-13T18-22-01-cccc1111-2222-3333-4444-555566667777.jsonl"
	writeSession(t, day, name, "cccc1111-2222-3333-4444-555566667777", project)

	out := filepath.Join(t.TempDir(), "y.codexbundle")
	if _, err := Export(home, ExportOptions{ProjectPath: project, OutputPath: out}); err != nil {
		t.Fatalf("export: %v", err)
	}

	files := readBundle(t, out)
	var sums Checksums
	if err := json.Unmarshal(files[ChecksumsName], &sums); err != nil {
		t.Fatalf("unmarshal checksums: %v", err)
	}
	// checksums.json must not reference itself.
	if _, ok := sums[ChecksumsName]; ok {
		t.Errorf("checksums.json should not include itself")
	}
	// Every other bundle file must have a matching, correct checksum.
	for name, data := range files {
		if name == ChecksumsName {
			continue
		}
		want, ok := sums[name]
		if !ok {
			t.Errorf("checksums missing entry for %q", name)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("checksum mismatch for %q: got %s want %s", name, got, want)
		}
	}
}

func TestExportCwdFilterExcludesOtherProjects(t *testing.T) {
	home := fakeHome(t)
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-22-01-dddd1111-2222-3333-4444-555566667777.jsonl",
		"dddd1111-2222-3333-4444-555566667777", "/proj/wanted")
	writeSession(t, day, "rollout-2026-06-13T19-00-00-eeee1111-2222-3333-4444-555566667777.jsonl",
		"eeee1111-2222-3333-4444-555566667777", "/proj/other")

	out := filepath.Join(t.TempDir(), "wanted.codexbundle")
	result, err := Export(home, ExportOptions{ProjectPath: "/proj/wanted", OutputPath: out})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.IncludedCount != 1 {
		t.Fatalf("included = %d, want 1 (cwd filter)", result.IncludedCount)
	}
	if result.Manifest.Sessions[0].OriginalCWD != "/proj/wanted" {
		t.Errorf("wrong session exported: %q", result.Manifest.Sessions[0].OriginalCWD)
	}
}

func TestExportSelectedMultipleProjects(t *testing.T) {
	home := fakeHome(t)
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-00-00-aaaa1111-2222-3333-4444-555566667777.jsonl",
		"aaaa1111-2222-3333-4444-555566667777", "/proj/alpha")
	writeSession(t, day, "rollout-2026-06-13T19-00-00-bbbb1111-2222-3333-4444-555566667777.jsonl",
		"bbbb1111-2222-3333-4444-555566667777", "/proj/beta")
	writeSession(t, day, "rollout-2026-06-13T20-00-00-cccc1111-2222-3333-4444-555566667777.jsonl",
		"cccc1111-2222-3333-4444-555566667777", "/proj/not-selected")

	out := filepath.Join(t.TempDir(), "handoff.codexbundle")
	result, err := Export(home, ExportOptions{
		ProjectPaths: []string{"/proj/alpha", "/proj/beta"},
		OutputPath:   out,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.IncludedCount != 2 {
		t.Fatalf("included = %d, want 2", result.IncludedCount)
	}
	if result.Manifest.SourceProjectPath != "" {
		t.Errorf("legacy single project path = %q, want empty", result.Manifest.SourceProjectPath)
	}
	if got := result.Manifest.SourceProjectPaths; len(got) != 2 || got[0] != "/proj/alpha" || got[1] != "/proj/beta" {
		t.Errorf("source project paths = %#v", got)
	}
	for _, s := range result.Manifest.Sessions {
		if s.OriginalCWD == "/proj/not-selected" {
			t.Fatalf("unselected project was exported: %+v", s)
		}
	}
}

func TestExportMultipleProjectsRecordsSanitizedGitMapping(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	projects := []string{filepath.Join(root, "alpha"), filepath.Join(root, "beta")}
	remotes := []string{
		"https://oauth2:sender-secret@git.example.com/team/alpha.git",
		"git@git.example.com:team/beta.git",
	}
	for i, project := range projects {
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatalf("mkdir project: %v", err)
		}
		mustRunGit(t, project, "init")
		if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("test\n"), 0o644); err != nil {
			t.Fatalf("write README: %v", err)
		}
		mustRunGit(t, project, "add", "README.md")
		mustRunGit(t, project, "-c", "user.name=cct-test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
		mustRunGit(t, project, "remote", "add", "origin", remotes[i])
	}

	home := fakeHome(t)
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-00-00-aaaa1111-2222-3333-4444-555566667777.jsonl",
		"aaaa1111-2222-3333-4444-555566667777", projects[0])
	writeSession(t, day, "rollout-2026-06-13T19-00-00-bbbb1111-2222-3333-4444-555566667777.jsonl",
		"bbbb1111-2222-3333-4444-555566667777", projects[1])
	out := filepath.Join(t.TempDir(), "handoff.codexbundle")
	if _, err := Export(home, ExportOptions{ProjectPaths: projects, OutputPath: out}); err != nil {
		t.Fatalf("export: %v", err)
	}
	res, err := Inspect(out)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if res.Manifest.Git != nil {
		t.Fatalf("legacy single-project git should be nil for a multi-project bundle: %+v", res.Manifest.Git)
	}
	if got := len(res.Manifest.Projects); got != 2 {
		t.Fatalf("project mappings = %d, want 2: %+v", got, res.Manifest.Projects)
	}
	wantRemotes := []string{
		"https://git.example.com/team/alpha.git",
		"git@git.example.com:team/beta.git",
	}
	for i, project := range res.Manifest.Projects {
		if project.Path != projects[i] {
			t.Errorf("project[%d].Path = %q, want %q", i, project.Path, projects[i])
		}
		if project.GitURL != wantRemotes[i] {
			t.Errorf("project[%d] remote = %q, want %q", i, project.GitURL, wantRemotes[i])
		}
		if project.Git != nil {
			t.Errorf("project[%d] should record only git_url, got legacy metadata: %+v", i, project.Git)
		}
	}
}

func TestExportProjectGitURLOverridesMissingSourceFolder(t *testing.T) {
	home := fakeHome(t)
	project := "/missing/project-delta"
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-00-00-aaaa1111-2222-3333-4444-555566667777.jsonl",
		"aaaa1111-2222-3333-4444-555566667777", project)
	out := filepath.Join(t.TempDir(), "handoff.codexbundle")
	res, err := Export(home, ExportOptions{
		ProjectPath:    project,
		ProjectGitURLs: map[string]string{project: "https://oauth2:secret@git.example.com/team/project-delta.git?token=hidden"},
		OutputPath:     out,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(res.Manifest.Projects) != 1 || res.Manifest.Projects[0].GitURL == "" {
		t.Fatalf("missing project git mapping: %+v", res.Manifest.Projects)
	}
	if got, want := res.Manifest.Projects[0].GitURL, "https://git.example.com/team/project-delta.git"; got != want {
		t.Errorf("remote = %q, want %q", got, want)
	}
}

func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestExportNoMatchingSessionsErrors(t *testing.T) {
	home := fakeHome(t)
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	writeSession(t, day, "rollout-2026-06-13T18-22-01-ffff1111-2222-3333-4444-555566667777.jsonl",
		"ffff1111-2222-3333-4444-555566667777", "/proj/a")

	out := filepath.Join(t.TempDir(), "none.codexbundle")
	_, err := Export(home, ExportOptions{ProjectPath: "/proj/does-not-match", OutputPath: out})
	if err == nil {
		t.Fatalf("expected error when no sessions match")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("no bundle should be written when nothing matches")
	}
}

func TestExportCompressedSkippedByCwdFilter(t *testing.T) {
	home := fakeHome(t)
	day := filepath.Join(home.SessionsDir, "2026", "06", "13")
	// A plain session that matches, plus a compressed one (cwd unknown).
	writeSession(t, day, "rollout-2026-06-13T18-22-01-11110000-2222-3333-4444-555566667777.jsonl",
		"11110000-2222-3333-4444-555566667777", "/proj/z")
	zst := filepath.Join(day, "rollout-2026-06-13T19-00-00-22220000-2222-3333-4444-555566667777.jsonl.zst")
	if err := os.WriteFile(zst, []byte("opaque zstd bytes"), 0o644); err != nil {
		t.Fatalf("write zst: %v", err)
	}

	out := filepath.Join(t.TempDir(), "z.codexbundle")
	result, err := Export(home, ExportOptions{ProjectPath: "/proj/z", OutputPath: out})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.IncludedCount != 1 {
		t.Errorf("included = %d, want 1", result.IncludedCount)
	}
	if result.CompressedSkipped != 1 {
		t.Errorf("compressed skipped = %d, want 1", result.CompressedSkipped)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
