#!/usr/bin/env bash
# Times one trivial chat completion per model against the Tinfoil enclave,
# printing latency, completion tokens, and whether the model burned hidden
# reasoning. No user data is sent.
#
# Usage: scripts/tinfoil-timing.sh [model ...]
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -f .env ]]; then
	set -a
	# shellcheck disable=SC1091
	source .env
	set +a
fi
: "${TINFOIL_API_KEY:?set TINFOIL_API_KEY or put it in .env}"

models=("$@")
[[ ${#models[@]} -gt 0 ]] || models=(deepseek-v4-flash gpt-oss-120b gemma4-31b glm-5-2 kimi-k3)

printf '%-20s %8s %8s %10s  %s\n' MODEL TTFB TOKENS REASONING CONTENT
for m in "${models[@]}"; do
	body=$(mktemp)
	ttfb=$(curl -sS -m 120 -o "$body" -w '%{time_starttransfer}' \
		https://inference.tinfoil.sh/v1/chat/completions \
		-H "Authorization: Bearer $TINFOIL_API_KEY" \
		-H 'Content-Type: application/json' \
		-d '{"model":"'"$m"'","messages":[{"role":"user","content":"Reply with the single word: ok"}],"max_tokens":2000}') || ttfb=fail
	python3 - "$m" "$ttfb" "$body" <<-'PY'
	import json, sys
	model, ttfb = sys.argv[1], sys.argv[2]
	try:
	    d = json.load(open(sys.argv[3]))
	    msg = d["choices"][0]["message"]
	    tokens = d["usage"]["completion_tokens"]
	    reasoning = "yes" if msg.get("reasoning_content") else "no"
	    content = repr((msg.get("content") or "")[:40])
	except Exception:
	    tokens, reasoning, content = "-", "-", "error"
	print(f"{model:<20} {ttfb:>7}s {tokens:>8} {reasoning:>10}  {content}")
	PY
	rm -f "$body"
done
