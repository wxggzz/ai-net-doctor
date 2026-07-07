#!/bin/bash
# Double-click to run a diagnostic and open a visual HTML report in your browser.
# All diagnosis lives in the CLI; this only launches it.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$ROOT/bin/ai-net-doctor"
[ -x "$BIN" ] || BIN="$(command -v ai-net-doctor)"

if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
  echo "找不到 ai-net-doctor。请先安装/构建："
  echo "  https://github.com/wxggzz/ai-net-doctor#install"
  echo
  echo "按任意键关闭…"
  read -r -n 1 -s
  exit 1
fi

echo "正在诊断并打开可视化报告…"
"$BIN" --html
