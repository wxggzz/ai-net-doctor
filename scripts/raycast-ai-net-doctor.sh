#!/bin/bash
# Raycast Script Command — runs ai-net-doctor and prints a short report.
# It only calls the CLI; no diagnosis logic lives here.
#
# @raycast.schemaVersion 1
# @raycast.title AI Net Doctor
# @raycast.mode fullOutput
# @raycast.packageName Network
# @raycast.icon 🩺
# @raycast.description Check whether the network can reach Codex and Claude Code.

# Point this at your built binary. Adjust if you install it elsewhere.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$ROOT/bin/ai-net-doctor"

if [ ! -x "$BIN" ]; then
  # Fall back to a binary on PATH (e.g. after `go install`).
  BIN="$(command -v ai-net-doctor)"
fi

if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
  echo "ai-net-doctor 未构建。请运行: go build -o ./bin/ai-net-doctor ./cmd/ai-net-doctor"
  exit 1
fi

"$BIN" --target all
