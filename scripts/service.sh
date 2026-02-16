#!/bin/bash
# OS-detecting wrapper — delegates to linux/ or macos/
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ "$(uname)" == "Darwin" ]]; then
    exec "${SCRIPT_DIR}/macos/service.sh" "$@"
else
    echo "LaunchAgent service management is macOS-only."
    echo "On Linux, use systemd: systemctl --user start robot-tracker"
    exit 1
fi
