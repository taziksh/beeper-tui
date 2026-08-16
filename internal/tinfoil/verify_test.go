package tinfoil

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tinfoilsh/tinfoil-go/verifier/client"
)

func TestDocumentOK(t *testing.T) {
	good := &client.VerificationDocument{
		SecurityVerified: true,
		EnclaveHost:      "inference.tinfoil.sh",
		ConfigRepo:       "tinfoilsh/confidential-model-router",
		ReleaseDigest:    "abc",
	}
	if err := documentOK(good, "inference.tinfoil.sh"); err != nil {
		t.Fatalf("good document: %v", err)
	}
	if err := documentOK(nil, "inference.tinfoil.sh"); err == nil {
		t.Fatal("nil document = nil error")
	}
	bad := *good
	bad.SecurityVerified = false
	if err := documentOK(&bad, "inference.tinfoil.sh"); err == nil {
		t.Fatal("unverified document = nil error")
	}
	bad = *good
	bad.EnclaveHost = "other.example"
	if err := documentOK(&bad, "inference.tinfoil.sh"); err == nil {
		t.Fatal("wrong host = nil error")
	}
	bad = *good
	bad.ReleaseDigest = ""
	if err := documentOK(&bad, "inference.tinfoil.sh"); err == nil {
		t.Fatal("empty digest = nil error")
	}
}

func TestFingerprintsMatch(t *testing.T) {
	doc := &client.VerificationDocument{
		CodeFingerprint:    "aaa",
		EnclaveFingerprint: "aaa",
	}
	if err := fingerprintsMatch(doc); err != nil {
		t.Fatalf("matching fingerprints: %v", err)
	}
	doc.EnclaveFingerprint = "bbb"
	if err := fingerprintsMatch(doc); err == nil {
		t.Fatal("mismatched fingerprints = nil error")
	}
	doc.CodeFingerprint = ""
	doc.EnclaveFingerprint = ""
	if err := fingerprintsMatch(doc); err == nil {
		t.Fatal("empty fingerprints = nil error")
	}
}

func TestIsHostBoundError(t *testing.T) {
	if isHostBoundError(nil) {
		t.Fatal("nil is host-bound")
	}
	if !isHostBoundError(errors.New(`Get "https://example.com/": refusing to send request to "https://example.com": client is bound to enclave "inference.tinfoil.sh"`)) {
		t.Fatal("SDK bind error not recognized")
	}
	if isHostBoundError(errors.New("connection refused")) {
		t.Fatal("network error treated as host bind")
	}
}

func TestWriteProbeLines(t *testing.T) {
	var buf bytes.Buffer
	writeProbeLines(&buf, Check{
		OK: true,
		Checks: map[string]probe{
			"attest":       {OK: true},
			"fingerprints": {OK: true},
			"failClosed":   {OK: true, Detail: "attestation failed"},
			"hostBind":     {Detail: "reached example.com"},
		},
		Document: &client.VerificationDocument{
			ConfigRepo:    "tinfoilsh/confidential-model-router",
			ReleaseTag:    "v0.0.141",
			ReleaseDigest: "deadbeef",
		},
	})
	got := buf.String()
	for _, want := range []string{
		"PASS attest",
		"PASS fingerprints",
		"PASS failClosed",
		"FAIL hostBind: reached example.com",
		"ok repo=tinfoilsh/confidential-model-router tag=v0.0.141 digest=deadbeef",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestPrintVerificationEncodesReport(t *testing.T) {
	var out, errw bytes.Buffer
	report := Check{
		Checks: map[string]probe{"attest": {Detail: "boom"}},
	}
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		t.Fatal(err)
	}
	writeProbeLines(&errw, report)
	if !strings.Contains(out.String(), `"ok": false`) {
		t.Errorf("stdout = %s", out.String())
	}
	if !strings.Contains(errw.String(), "FAIL attest") {
		t.Errorf("stderr = %s", errw.String())
	}
}
