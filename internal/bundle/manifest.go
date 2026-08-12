// Package bundle creates and (later) reads .codexbundle archives: a ZIP
// containing a manifest, a checksum map, and the copied Codex rollout files.
//
// A .codexbundle is a portable, hosting-free way to move local Codex sessions
// between devices. It never includes or modifies Codex's SQLite state DB.
package bundle

import "github.com/ahmojo/codex-claude-transfer/internal/git"

// FormatVersion identifies the bundle layout. Bump this on breaking changes.
const FormatVersion = "codex-sync-bundle-v1"

// Bundle file names.
const (
	ManifestName  = "manifest.json"
	ChecksumsName = "checksums.json"
)

// Manifest describes a bundle and everything inside it. It is written as
// manifest.json at the root of the ZIP.
type Manifest struct {
	FormatVersion string `json:"format_version"`
	// Tool records which coding agent the sessions came from ("codex" or
	// "claude"). It is omitted for legacy Codex bundles, which are treated as
	// "codex" on read, so older bundles remain importable unchanged.
	Tool              string `json:"tool,omitempty"`
	CreatedAt         string `json:"created_at"`
	CreatedByDevice   string `json:"created_by_device,omitempty"`
	SourceOS          string `json:"source_os"`
	SourceCodexHome   string `json:"source_codex_home"`
	SourceProjectPath string `json:"source_project_path,omitempty"`
	// SourceProjectPaths records an explicitly selected multi-project export.
	// Older cct versions ignore this additive field and still import sessions by
	// each ManifestSession.OriginalCWD.
	SourceProjectPaths []string `json:"source_project_paths,omitempty"`
	// Projects maps every explicitly selected source folder to its sanitized Git
	// repository URL. It is additive and optional: older readers ignore it, and legacy
	// single-project bundles continue to use Git below.
	Projects     []ManifestProject `json:"projects,omitempty"`
	Git          *git.Info         `json:"git,omitempty"`
	CodexVersion string            `json:"codex_version,omitempty"`
	Sessions     []ManifestSession `json:"sessions"`
	// Memory lists the Claude Code auto-memory files carried alongside the
	// sessions. It is only ever populated by an explicit `--with-memory` export;
	// a cct that predates the field ignores it and skips the entries, so an older
	// version can still read the bundle.
	Memory []ManifestMemory `json:"memory,omitempty"`
}

// ManifestProject connects one recorded Codex project folder to the repository
// the recipient can clone before remapping imported sessions to a local folder.
type ManifestProject struct {
	Path   string `json:"path"`
	GitURL string `json:"git_url,omitempty"`
	// Git reads bundles produced by the short-lived format that stored the full
	// git.Info per project. New bundles leave it empty and write GitURL only.
	Git *git.Info `json:"git,omitempty"`
}

// GitForProject returns the git metadata recorded for path. The legacy Git
// field is used as a fallback for older single-project bundles.
func (m Manifest) GitForProject(path string) *git.Info {
	want := normalizeCWD(path)
	for i := range m.Projects {
		if normalizeCWD(m.Projects[i].Path) == want {
			if m.Projects[i].GitURL != "" {
				return &git.Info{RemoteURL: m.Projects[i].GitURL}
			}
			return m.Projects[i].Git
		}
	}
	if m.Git != nil && (m.SourceProjectPath == "" || normalizeCWD(m.SourceProjectPath) == want) {
		return m.Git
	}
	return nil
}

// ManifestMemory is one Claude Code auto-memory file recorded in the manifest.
// The project it belongs to is named by its cwd, not by the encoded folder, so
// an import that remaps the cwd can place the file under the right project.
type ManifestMemory struct {
	ProjectCWD   string `json:"project_cwd"`
	Rel          string `json:"rel"`
	OriginalPath string `json:"original_path"`
	BundlePath   string `json:"bundle_path"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}

// ManifestSession is one rollout file recorded in the manifest.
type ManifestSession struct {
	ThreadID         string `json:"thread_id,omitempty"`
	OriginalPath     string `json:"original_path"`
	BundlePath       string `json:"bundle_path"`
	OriginalCWD      string `json:"original_cwd,omitempty"`
	Preview          string `json:"preview,omitempty"`
	FirstUserMessage string `json:"first_user_message,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	Source           string `json:"source,omitempty"`
	ModelProvider    string `json:"model_provider,omitempty"`
	GitBranch        string `json:"git_branch,omitempty"`
	GitSHA           string `json:"git_sha,omitempty"`
	Archived         bool   `json:"archived"`
	Compressed       bool   `json:"compressed"`
	SizeBytes        int64  `json:"size_bytes"`
	SHA256           string `json:"sha256"`
}
