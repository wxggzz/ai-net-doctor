#!/bin/sh
# ai-net-doctor installer — downloads the prebuilt release binary for your platform.
#
#   curl -fsSL https://raw.githubusercontent.com/wxggzz/ai-net-doctor/main/scripts/install.sh | sh
#
# Env overrides:
#   VERSION=v0.1.0   install a specific tag (default: latest release)
#   BINDIR=/usr/local/bin   install location (default: $HOME/.local/bin)
set -eu

REPO="wxggzz/ai-net-doctor"
BINDIR="${BINDIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux) os=linux ;;
  darwin) os=darwin ;;
  *) echo "Unsupported OS: $os. Windows users: download the .zip from https://github.com/$REPO/releases" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac

tag="${VERSION:-latest}"
if [ "$tag" = "latest" ]; then
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')
fi
[ -n "$tag" ] || { echo "Could not resolve the latest release tag." >&2; exit 1; }

file="ai-net-doctor_${tag}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$file"

echo "Downloading ai-net-doctor $tag ($os/$arch)..."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp/$file"
tar -xzf "$tmp/$file" -C "$tmp"
mkdir -p "$BINDIR"
install -m 0755 "$tmp/ai-net-doctor" "$BINDIR/ai-net-doctor"

echo "Installed to $BINDIR/ai-net-doctor"
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *) echo "Note: $BINDIR is not on your PATH. Add this to your shell profile:"
     echo "  export PATH=\"$BINDIR:\$PATH\"" ;;
esac
"$BINDIR/ai-net-doctor" --version 2>/dev/null || true
