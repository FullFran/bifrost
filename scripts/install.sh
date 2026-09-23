#!/usr/bin/env bash
# ==============================================================================
# Bifrost Installer
# The Universal AI Coding Agent Bridge
# ==============================================================================

set -euo pipefail

INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"

echo "🌈 Installing Bifrost..."

# If running inside repo, compile and install directly
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -f "$SCRIPT_DIR/go.mod" && -f "$SCRIPT_DIR/cmd/bifrost/main.go" ]]; then
    echo "🔨 Building from local source..."
    cd "$SCRIPT_DIR"
    go build -ldflags="-s -w" -o "$INSTALL_DIR/bifrost" ./cmd/bifrost
    chmod +x "$INSTALL_DIR/bifrost"
else
    # Remote fallback using go install or github releases
    if command -v go >/dev/null 2>&1; then
        echo "📦 Installing via go install..."
        go install github.com/FullFran/bifrost/cmd/bifrost@latest
        if [[ -f "$(go env GOPATH)/bin/bifrost" ]]; then
            cp "$(go env GOPATH)/bin/bifrost" "$INSTALL_DIR/bifrost"
        fi
    else
        echo "Error: Go compiler is required for local build." >&2
        exit 1
    fi
fi

# Ensure ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo ""
    echo "⚠️ Notice: $INSTALL_DIR is not in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
fi

echo "✅ Bifrost installed successfully to $INSTALL_DIR/bifrost"
echo ""
"$INSTALL_DIR/bifrost" realms
