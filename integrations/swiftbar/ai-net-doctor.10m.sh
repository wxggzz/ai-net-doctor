#!/bin/bash
#
# SwiftBar / xbar plugin for ai-net-doctor.
# The colored dot in your menu bar shows whether Codex & Claude are reachable;
# click it for per-target status and quick actions.
#
# Setup:
#   1. Install SwiftBar (https://swiftbar.app) — free, open source.
#   2. Install the CLI (see https://github.com/wxggzz/ai-net-doctor#install).
#   3. Copy this file into your SwiftBar plugin folder, keep it executable:
#        chmod +x ai-net-doctor.10m.sh
#      The "10m" in the filename = refresh every 10 minutes; rename to taste
#      (e.g. .5m., .1h.). Set AI_NET_DOCTOR_BIN to override binary location.
#
# <xbar.title>AI Net Doctor</xbar.title>
# <xbar.version>v0.1.0</xbar.version>
# <xbar.author>wxggzz</xbar.author>
# <xbar.desc>Menu-bar status for Codex/OpenAI and Claude/Anthropic connectivity.</xbar.desc>
# <xbar.dependencies>ai-net-doctor</xbar.dependencies>
# <xbar.abouturl>https://github.com/wxggzz/ai-net-doctor</xbar.abouturl>
# <swiftbar.runInBash>true</swiftbar.runInBash>

BIN=""
for c in "$AI_NET_DOCTOR_BIN" \
         "$HOME/.local/bin/ai-net-doctor" \
         "/opt/homebrew/bin/ai-net-doctor" \
         "/usr/local/bin/ai-net-doctor" \
         "$(command -v ai-net-doctor 2>/dev/null)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then BIN="$c"; break; fi
done

if [ -z "$BIN" ]; then
  echo "⚪"
  echo "---"
  echo "ai-net-doctor not installed"
  echo "Install instructions | href=https://github.com/wxggzz/ai-net-doctor#install"
  exit 0
fi

# The CLI emits SwiftBar-formatted output directly (no jq/python needed).
exec "$BIN" --menubar --budget 12
