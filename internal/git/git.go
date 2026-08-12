// Package git provides best-effort discovery of git metadata for a project
// directory. Everything here is optional: if git is not installed or the
// directory is not a repository, the functions return an empty Info and no
// error, so callers can include git metadata when available without failing.
package git

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// hexOIDRe matches a git object id (a 7–64 char hex string).
var hexOIDRe = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

// helperTransportRe matches git's "remote helper" transport syntax
// `<name>::<address>` (e.g. ext::, fd::). The ext helper can run an arbitrary
// command, so these are blocked. Standard URLs use `<scheme>://`, not `::`, and a
// local path / scp-style remote contains no `::`, so they are unaffected.
var helperTransportRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*::`)

// validateRemoteURL guards against a malicious bundle's git remote. It rejects
// values that look like a command-line flag and git remote-helper transports
// (ext::/fd::/…), which are the command-execution vector. Ordinary transports
// (https/http/ssh/git, scp-style user@host:path, file://, and local paths) are
// allowed, since they cannot execute commands. See SEC-1 in docs/security/audit.md.
// ValidateRemoteURL is the exported form of validateRemoteURL, so callers that
// accept a git remote from somewhere other than a bundle — a reference file
// committed to a project repo, for example — apply exactly the same rule.
func ValidateRemoteURL(remote string) error { return validateRemoteURL(remote) }

func validateRemoteURL(remote string) error {
	if remote == "" {
		return fmt.Errorf("empty git remote")
	}
	if strings.HasPrefix(remote, "-") {
		return fmt.Errorf("refusing git remote that looks like a flag: %q", remote)
	}
	if helperTransportRe.MatchString(remote) {
		return fmt.Errorf("refusing git remote-helper transport (e.g. ext::/fd::): %q", remote)
	}
	return nil
}

// Info holds the git facts we record in a bundle manifest.
type Info struct {
	Branch    string `json:"branch,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
	RemoteURL string `json:"remote_url,omitempty"`
	// Dirty reports that the working tree had uncommitted changes when the
	// bundle was exported, so HEAD does not capture the exact state.
	Dirty bool `json:"dirty,omitempty"`
	// Unpushed reports that the HEAD commit is not reachable from any
	// remote-tracking branch, so the other machine cannot fetch it yet.
	Unpushed bool `json:"unpushed,omitempty"`
}

// Empty reports whether no git information was discovered.
func (i Info) Empty() bool {
	return i.Branch == "" && i.CommitSHA == "" && i.RemoteURL == ""
}

// Available reports whether the git executable is on PATH.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo reports whether dir is inside a git working tree. It is best-effort:
// a missing git binary or any failure yields false.
func IsRepo(dir string) bool {
	if dir == "" || !Available() {
		return false
	}
	return isRepo(dir)
}

// Discover returns best-effort git metadata for dir. It never returns an error;
// missing git or a non-repository simply yields an empty Info.
func Discover(dir string) Info {
	if dir == "" {
		return Info{}
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Info{}
	}
	if !isRepo(dir) {
		return Info{}
	}
	return Info{
		Branch:    run(dir, "rev-parse", "--abbrev-ref", "HEAD"),
		CommitSHA: run(dir, "rev-parse", "HEAD"),
		RemoteURL: SanitizeRemoteURL(run(dir, "config", "--get", "remote.origin.url")),
		Dirty:     isDirty(dir),
		Unpushed:  isUnpushed(dir),
	}
}

// SanitizeRemoteURL removes credentials and query/fragment data before a git
// remote is written into a portable bundle. A remote can legitimately contain
// an HTTPS access token or password in userinfo; recipients need the repository
// location, never the sender's credential. SSH usernames are retained when
// they are ordinary usernames, but passwords are always removed. For scp-style
// remotes, suspicious/nonstandard usernames are replaced with the conventional
// "git" user so token-shaped userinfo cannot leak into the manifest.
func SanitizeRemoteURL(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if u, err := url.Parse(remote); err == nil && u.Scheme != "" && strings.Contains(remote, "://") {
		if u.User != nil {
			username := u.User.Username()
			switch strings.ToLower(u.Scheme) {
			case "http", "https":
				u.User = nil
			default:
				if username == "" {
					u.User = nil
				} else {
					u.User = url.User(username)
				}
			}
		}
		// Git remotes do not need URL query parameters or fragments for repository
		// identity, while these are common places for access tokens to hide.
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		return u.String()
	}

	// scp-style SSH: [user@]host:path. Preserve conventional public transport
	// users; replace anything else because it may itself be an access token.
	if at := strings.LastIndex(remote, "@"); at > 0 {
		if colon := strings.Index(remote[at+1:], ":"); colon >= 0 {
			user := remote[:at]
			switch user {
			case "git", "hg", "svn", "ssh":
				// Ordinary transport username; safe and needed for cloning.
			default:
				remote = "git@" + remote[at+1:]
			}
		}
	}
	if i := strings.IndexAny(remote, "?#"); i >= 0 {
		remote = remote[:i]
	}
	return remote
}

// Clone clones remote into dir and, when commit is non-empty, checks it out.
// It is only ever invoked when the user explicitly opts in (e.g. --clone), so
// it surfaces errors rather than swallowing them like Discover does.
func Clone(remote, dir, commit string) error {
	if remote == "" {
		return fmt.Errorf("no remote URL to clone")
	}
	// The remote and commit come from the (untrusted) bundle manifest. Validate the
	// transport and the commit shape before handing them to git. See SEC-1 in
	// docs/security/audit.md.
	if err := validateRemoteURL(remote); err != nil {
		return err
	}
	if commit != "" && !hexOIDRe.MatchString(commit) {
		return fmt.Errorf("refusing to check out non-hex commit from bundle: %q", commit)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	// Belt-and-suspenders alongside validateRemoteURL: explicitly forbid the
	// command-executing helper transports at the git-config level, and `--` stops
	// a stray leading-dash remote from being read as an option.
	if err := runErr("", "-c", "protocol.ext.allow=never", "-c", "protocol.fd.allow=never",
		"clone", "--", remote, dir); err != nil {
		return fmt.Errorf("git clone %s: %w", remote, err)
	}
	if commit != "" {
		if err := runErr(dir, "checkout", commit); err != nil {
			return fmt.Errorf("git checkout %s: %w", commit, err)
		}
	}
	return nil
}

// Push pushes the current branch of the repo at dir to its remote. It is only
// invoked when the user explicitly opts in (--git-push), so it surfaces errors
// rather than swallowing them. It is deliberately conservative: it never
// force-pushes, never pushes tags, and never creates a remote — it runs a plain
// `git push <remote> <branch>`, so a diverged remote is rejected as a
// non-fast-forward rather than overwritten. It returns the remote name and
// branch that were pushed.
func Push(dir string) (remote, branch string, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", fmt.Errorf("git not found in PATH")
	}
	if !isRepo(dir) {
		return "", "", fmt.Errorf("%s is not a git repository", dir)
	}
	branch = run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" || branch == "HEAD" {
		return "", "", fmt.Errorf("cannot push a detached HEAD; check out a branch first")
	}
	remote = pushRemote(dir, branch)
	if remote == "" {
		return "", "", fmt.Errorf("no git remote is configured to push to")
	}
	if err := runErr(dir, "push", remote, branch); err != nil {
		return "", "", fmt.Errorf("git push %s %s: %w", remote, branch, err)
	}
	return remote, branch, nil
}

// pushRemote returns the remote to push branch to: the branch's configured
// upstream remote if set, otherwise the first configured remote.
func pushRemote(dir, branch string) string {
	if up := run(dir, "rev-parse", "--abbrev-ref", branch+"@{upstream}"); up != "" {
		if i := strings.Index(up, "/"); i > 0 {
			return up[:i]
		}
	}
	if remotes := run(dir, "remote"); remotes != "" {
		return strings.Fields(remotes)[0]
	}
	return ""
}

func isRepo(dir string) bool {
	return run(dir, "rev-parse", "--is-inside-work-tree") == "true"
}

// isDirty reports whether the working tree has staged or unstaged changes.
func isDirty(dir string) bool {
	return run(dir, "status", "--porcelain") != ""
}

// isUnpushed reports whether HEAD has commits not reachable from any
// remote-tracking branch. With no remotes configured, every commit counts as
// unpushed (the other machine has nowhere to fetch from).
func isUnpushed(dir string) bool {
	return run(dir, "rev-list", "HEAD", "--not", "--remotes", "--max-count=1") != ""
}

// run executes a git command in dir and returns its trimmed stdout, or "" on
// any failure. A short timeout guards against a hung git process.
func run(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runErr executes a git command and returns its error, with a longer timeout
// suited to network operations like clone. Unlike run, it does not swallow
// failures: callers that opt into an action want to know when it fails.
func runErr(dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
