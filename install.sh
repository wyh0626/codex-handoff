#!/usr/bin/env bash
# Install the latest codex-handoff release on macOS or Linux.
set -euo pipefail

REPO="${CODEX_HANDOFF_REPO:-wyh0626/codex-handoff}"
INSTALL_DIR="${CODEX_HANDOFF_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux) OS="linux" ;;
  *) echo "error: use the Windows archive from the Releases page" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

ASSET="codex-handoff_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/latest/download"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/codex-handoff-install.XXXXXX")"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

echo "Downloading $ASSET from $REPO..."
curl --proto '=https' --tlsv1.2 -fsSL "$BASE_URL/$ASSET" -o "$TMP/$ASSET"
curl --proto '=https' --tlsv1.2 -fsSL "$BASE_URL/SHA256SUMS.txt" -o "$TMP/SHA256SUMS.txt"

EXPECTED="$(awk -v file="$ASSET" '$2 == file { print $1 }' "$TMP/SHA256SUMS.txt")"
if [ -z "$EXPECTED" ]; then
  echo "error: release checksum does not list $ASSET" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$TMP/$ASSET" | awk '{print $1}')"
else
  ACTUAL="$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')"
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "error: SHA-256 verification failed" >&2
  exit 1
fi

mkdir -p "$TMP/unpack" "$INSTALL_DIR"
tar -xzf "$TMP/$ASSET" -C "$TMP/unpack"
BIN="$(find "$TMP/unpack" -type f -name codex-handoff -print -quit)"
if [ -z "$BIN" ]; then
  echo "error: release archive contains no codex-handoff binary" >&2
  exit 1
fi
install -m 0755 "$BIN" "$INSTALL_DIR/codex-handoff"

echo "Installed: $INSTALL_DIR/codex-handoff"
"$INSTALL_DIR/codex-handoff" version
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add $INSTALL_DIR to PATH before running codex-handoff." ;;
esac
