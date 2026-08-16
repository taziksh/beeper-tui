#!/usr/bin/env bash
# Installed by `make install` as ~/.local/bin/beeper-tui.
# Rebuilds from the checkout when source is newer than the binary, then execs it.
# So the PATH command never falls behind the repo.
set -euo pipefail

REPO="${BEEPER_TUI_REPO:-@REPO@}"
BIN="${BEEPER_TUI_BIN:-$REPO/beeper-tui}"

if [[ ! -d "$REPO" ]]; then
	echo "beeper-tui: repo not found at $REPO" >&2
	echo "  Set BEEPER_TUI_REPO or re-run: make install" >&2
	exit 1
fi

if ! command -v go >/dev/null 2>&1; then
	echo "beeper-tui: go not found on PATH (needed to rebuild)" >&2
	exit 1
fi

needs_build=0
if [[ ! -x "$BIN" ]]; then
	needs_build=1
else
	# Rebuild if any tracked source is newer than the binary.
	# shellcheck disable=SC2044
	if [[ -n "$(find "$REPO/cmd" "$REPO/internal" \
		\( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) \
		-newer "$BIN" -print 2>/dev/null | head -n 1)" ]]; then
		needs_build=1
	elif [[ -f "$REPO/go.mod" && "$REPO/go.mod" -nt "$BIN" ]] ||
		[[ -f "$REPO/go.sum" && "$REPO/go.sum" -nt "$BIN" ]]; then
		needs_build=1
	fi
fi

# Keys and provider settings live in the checkout's .env, gitignored.
if [[ -f "$REPO/.env" ]]; then
	set -a
	# shellcheck disable=SC1091
	source "$REPO/.env"
	set +a
fi

if [[ "$needs_build" -eq 1 ]]; then
	# Silent by default so the TUI isn't noisy; set BEEPER_TUI_VERBOSE=1 to see rebuilds.
	if [[ "${BEEPER_TUI_VERBOSE:-}" == "1" ]]; then
		echo "beeper-tui: rebuilding from $REPO" >&2
	fi
	(cd "$REPO" && go build -o "$BIN" ./cmd/beeper-tui)
fi

exec "$BIN" "$@"
