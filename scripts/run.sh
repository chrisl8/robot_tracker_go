#!/bin/bash
# OS-detecting wrapper — delegates to linux/ or macos/
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ "$(uname)" == "Darwin" ]]; then
    exec "${SCRIPT_DIR}/macos/run.sh" "$@"
else
    exec "${SCRIPT_DIR}/linux/run.sh" "$@"
fi
