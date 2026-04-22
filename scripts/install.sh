#!/usr/bin/env bash
#
# One-liner install for HermesManager binary.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/MackDing/hermes-manager/main/scripts/install.sh | bash
#   VERSION=v1.1.0 bash install.sh
#
set -euo pipefail

VERSION="${VERSION:-latest}"
GITHUB_REPO="MackDing/hermes-manager"

# Resolve "latest" to actual tag
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest release." >&2
    exit 1
  fi
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture '$ARCH'. Only amd64 and arm64 are supported." >&2
    exit 1
    ;;
esac

URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/hermesmanager-${OS}-${ARCH}"
echo "Downloading hermesmanager ${VERSION} (${OS}/${ARCH})..."
curl -fsSLo hermesmanager "$URL"
chmod +x hermesmanager

echo ""
echo "Downloaded to: $(pwd)/hermesmanager"
./hermesmanager --version 2>/dev/null || echo "(binary ready)"
echo ""
echo "Move to your PATH to use globally:"
echo "  sudo mv hermesmanager /usr/local/bin/"
