#!/bin/bash
# Double-clickable macOS entry point for ai-net-doctor.
# It ONLY runs the CLI and prints the report — all diagnosis lives in the CLI.

# Resolve project root from this script's location (scripts/ -> repo root).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$ROOT/bin/ai-net-doctor"

echo "=== AI Net Doctor ==="
echo

if [ ! -x "$BIN" ]; then
  echo "找不到已构建的二进制：$BIN"
  echo
  echo "请先在项目根目录构建："
  echo "  cd \"$ROOT\""
  echo "  go build -o ./bin/ai-net-doctor ./cmd/ai-net-doctor"
  echo
else
  "$BIN" --target all
fi

echo
echo "诊断完成。按任意键关闭窗口..."
# -n 1 reads a single key; works in the Terminal window opened by double-click.
read -r -n 1 -s
