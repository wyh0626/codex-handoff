#!/usr/bin/env bash
# End-to-end test against a disposable Codex home. Nothing is written to the
# user's real ~/.codex directory.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/codex-handoff}"
BUNDLE="${2:-$ROOT/codex-project-handoff.codexbundle}"
EXPECTED="${EXPECTED_SESSIONS:-}"

if [ ! -x "$BIN" ]; then
  echo "error: executable not found: $BIN" >&2
  echo "build it with: go build -o codex-handoff ./cmd/codex-handoff" >&2
  exit 2
fi
if [ ! -f "$BUNDLE" ]; then
  echo "error: bundle not found: $BUNDLE" >&2
  echo "usage: $0 [binary] [bundle]" >&2
  exit 2
fi

TEST_HOME="$(mktemp -d "${TMPDIR:-/tmp}/codex-handoff-test.XXXXXX")"
cleanup() { rm -rf "$TEST_HOME"; }
trap cleanup EXIT INT TERM
# Import also writes an undo journal. Keep that out of the user's normal cct
# configuration directory so the entire test is disposable.
export CCT_CONFIG_DIR="$TEST_HOME/cct-config"

count_sessions() {
  local count=0
  if [ -d "$TEST_HOME/sessions" ] || [ -d "$TEST_HOME/archived_sessions" ]; then
    count="$(find "$TEST_HOME/sessions" "$TEST_HOME/archived_sessions" \
      -type f \( -name 'rollout-*.jsonl' -o -name 'rollout-*.jsonl.zst' \) \
      2>/dev/null | wc -l | tr -d ' ')"
  fi
  printf '%s' "$count"
}

echo "Test Codex home: $TEST_HOME"
echo "== binary =="
"$BIN" version

echo "== dry run =="
"$BIN" import "$BUNDLE" --dry-run --codex-home "$TEST_HOME"
if [ "$(count_sessions)" != "0" ]; then
  echo "error: dry-run unexpectedly wrote session files" >&2
  exit 1
fi

echo "== isolated real import =="
"$BIN" import "$BUNDLE" --codex-home "$TEST_HOME"
ACTUAL="$(count_sessions)"
if [ "$ACTUAL" -eq 0 ]; then
  echo "error: import completed without writing any sessions" >&2
  exit 1
fi
if [ -n "$EXPECTED" ] && [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "error: imported $ACTUAL sessions, expected $EXPECTED" >&2
  exit 1
fi

echo "PASS: imported $ACTUAL sessions into an isolated Codex home."
echo "PASS: your real ~/.codex was not used."
