#!/usr/bin/env bash
# ccpill one-click installer (macOS / Linux / Git Bash)
# Usage:  curl -fsSL https://raw.githubusercontent.com/cass-2003/ccpill/main/scripts/install.sh | bash
# Flow:   GitHub Releases prebuilt binary -> fallback to `go install` -> write Claude Code settings.json
set -euo pipefail

REPO="cass-2003/ccpill"
CLAUDE_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
BIN_DIR="$CLAUDE_DIR/ccpill/bin"
mkdir -p "$BIN_DIR"

case "$(uname -s)" in
  Darwin) OS=darwin ;;
  Linux)  OS=linux ;;
  MINGW*|MSYS*|CYGWIN*) OS=windows ;;
  *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac
EXT=""; [ "$OS" = "windows" ] && EXT=".exe"
EXE="$BIN_DIR/ccpill$EXT"
ASSET="ccpill-$OS-$ARCH$EXT"

installed=""

# 1) Prebuilt binary from GitHub Releases (available since V0.3; skip silently if none)
url=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
      | grep -o "\"browser_download_url\": *\"[^\"]*$ASSET\"" | grep -o 'https://[^"]*' | head -1 || true)
if [ -n "$url" ]; then
  echo "Downloading prebuilt binary: $url"
  curl -fL "$url" -o "$EXE"
  chmod +x "$EXE"
  installed=1
fi

# 2) Local Go toolchain: install straight from source, no clone needed
if [ -z "$installed" ] && command -v go >/dev/null 2>&1; then
  echo "No prebuilt release yet, building from source via go install ..."
  GOBIN="$BIN_DIR" go install "github.com/$REPO@latest"
  installed=1
fi

if [ -z "$installed" ]; then
  echo "No prebuilt release and no local Go. Install Go first (https://go.dev/dl/) then rerun this script." >&2
  exit 1
fi

# 3) Write Claude Code settings.json (ccpill backs up and writes atomically)
"$EXE" --install

# 4) Symlink onto PATH so `ccpill --config` works everywhere
LINK_DIR="$HOME/.local/bin"
mkdir -p "$LINK_DIR"
ln -sf "$EXE" "$LINK_DIR/ccpill"
echo ""
case ":$PATH:" in
  *":$LINK_DIR:"*) echo "Web config center:   ccpill --config" ;;
  *) echo "Linked $LINK_DIR/ccpill (add $LINK_DIR to your PATH, then run: ccpill --config)" ;;
esac
