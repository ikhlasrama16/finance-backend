#!/usr/bin/env bash
# finance_bot_adapter.sh - Shell wrapper for OpenClaw tools
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_BIN="$(which python3 || echo "python")"

exec "$PYTHON_BIN" "$SCRIPT_DIR/finance_bot_adapter.py" "$@"
