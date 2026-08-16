#!/usr/bin/env bash
# Cross-checks the enclave attestation without Tinfoil's verifier in the loop.
# Reads the measurement from --verify-tinfoil, then asks GitHub and Sigstore
# for the same release and compares. Needs gh, go, python3, and network.
#
# Usage: scripts/tinfoil-attest-independent.sh
set -euo pipefail
cd "$(dirname "$0")/.."

GH="${GH_BIN:-}"
if [[ -z "$GH" ]]; then
	if [[ -x /opt/homebrew/bin/gh ]]; then GH=/opt/homebrew/bin/gh; else GH=gh; fi
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

echo "attesting enclave" >&2
go run ./cmd/beeper-tui --verify-tinfoil >"$work/report.json"

read -r repo tag digest enclave_fp < <(python3 - "$work/report.json" <<-'PY'
	import json, sys
	doc = json.load(open(sys.argv[1]))["document"]
	print(doc["configRepo"], doc["releaseTag"], doc["releaseDigest"], doc["enclaveFingerprint"])
	PY
)

curl -fsSL -o "$work/tinfoil-deployment.json" \
	"https://github.com/$repo/releases/download/$tag/tinfoil-deployment.json"
local_digest=$(shasum -a 256 "$work/tinfoil-deployment.json" | cut -d' ' -f1)
if [[ "$local_digest" != "$digest" ]]; then
	echo "FAIL releaseDigest: local $local_digest, attested $digest" >&2
	exit 1
fi
echo "PASS releaseDigest $digest" >&2

"$GH" attestation verify "$work/tinfoil-deployment.json" --repo "$repo" \
	--predicate-type https://tinfoil.sh/predicate/snp-tdx-multiplatform/v1 >&2
echo "PASS sigstore signature chain" >&2

log_fp=$("$GH" api "repos/$repo/attestations/sha256:$digest" | python3 -c "
import base64, json, sys
d = json.load(sys.stdin)
p = json.loads(base64.b64decode(d['attestations'][0]['bundle']['dsseEnvelope']['payload']))
print(p['predicate']['snp_measurement'])
")
if [[ "$log_fp" != "$enclave_fp" ]]; then
	echo "FAIL measurement: log $log_fp, enclave $enclave_fp" >&2
	exit 1
fi
echo "PASS measurement $enclave_fp" >&2
echo "ok: hardware-reported measurement matches the signed public release $repo@$tag" >&2
