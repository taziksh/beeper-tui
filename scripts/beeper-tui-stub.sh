#!/usr/bin/env bash
# Installed by `make install` as ~/.local/bin/beeper-tui.
# Only the repo path is baked in; all other logic lives in the checkout wrapper.
set -euo pipefail

export BEEPER_TUI_REPO="${BEEPER_TUI_REPO:-@REPO@}"
if [[ ! -d "$BEEPER_TUI_REPO" ]]; then
	echo "beeper-tui: repo not found at $BEEPER_TUI_REPO" >&2
	echo "  Set BEEPER_TUI_REPO or re-run: make install" >&2
	exit 1
fi

wrapper="$BEEPER_TUI_REPO/scripts/beeper-tui-wrapper.sh"
if [[ ! -f "$wrapper" ]]; then
	echo "beeper-tui: wrapper not found at $wrapper" >&2
	exit 1
fi

exec bash "$wrapper" "$@"
