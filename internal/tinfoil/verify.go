package tinfoil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tinfoilsh/tinfoil-go/verifier/client"

	"github.com/taziksh/beeper-tui/internal/config"
)

// failClosedHost is a public site that is not a Tinfoil enclave. Attesting
// it must fail so we know Dial does not accept an arbitrary HTTPS server.
const failClosedHost = "example.com"

// foreignURL is an origin the pinned client must refuse.
const foreignURL = "https://example.com/"

// Check is the JSON report --verify-tinfoil prints. Document is the SDK
// attestation record; Checks are the extra sanity probes this binary runs.
type Check struct {
	OK       bool                         `json:"ok"`
	Enclave  string                       `json:"enclave"`
	Checks   map[string]probe             `json:"checks"`
	Document *client.VerificationDocument `json:"document,omitempty"`
}

type probe struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// PrintVerification attests the pinned enclave, runs fail-closed and
// host-bind probes, and writes a JSON report to out. Status lines go to
// errw. Returns a non-nil error when any probe fails.
func PrintVerification(out, errw io.Writer) error {
	report, err := Verify()
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(report); encErr != nil {
		return encErr
	}
	writeProbeLines(errw, report)
	return err
}

// Verify attests config.TinfoilEnclave and runs the probes a human uses
// to check that this binary's Tinfoil path is wired correctly.
func Verify() (Check, error) {
	report := Check{
		Enclave: config.TinfoilEnclave,
		Checks:  map[string]probe{},
	}

	tc, err := dialClient(config.TinfoilEnclave)
	if err != nil {
		report.Checks["attest"] = probe{Detail: err.Error()}
		return report, err
	}
	doc := tc.VerificationDocument()
	report.Document = doc
	if err := documentOK(doc, config.TinfoilEnclave); err != nil {
		report.Checks["attest"] = probe{Detail: err.Error()}
		return report, err
	}
	report.Checks["attest"] = probe{OK: true}

	if err := fingerprintsMatch(doc); err != nil {
		report.Checks["fingerprints"] = probe{Detail: err.Error()}
		return report, err
	}
	report.Checks["fingerprints"] = probe{OK: true}

	if _, err := dialClient(failClosedHost); err == nil {
		err := fmt.Errorf("tinfoil: attested %s; expected refusal", failClosedHost)
		report.Checks["failClosed"] = probe{Detail: err.Error()}
		return report, err
	} else {
		report.Checks["failClosed"] = probe{OK: true, Detail: err.Error()}
	}

	if err := refuseOtherHosts(tc.HTTPClient(), foreignURL); err != nil {
		report.Checks["hostBind"] = probe{Detail: err.Error()}
		return report, err
	}
	report.Checks["hostBind"] = probe{OK: true}

	report.OK = true
	return report, nil
}

func documentOK(doc *client.VerificationDocument, enclave string) error {
	if doc == nil {
		return fmt.Errorf("tinfoil: no verification document")
	}
	if !doc.SecurityVerified {
		return fmt.Errorf("tinfoil: securityVerified is false")
	}
	if doc.EnclaveHost != enclave {
		return fmt.Errorf("tinfoil: enclave host %q, want %q", doc.EnclaveHost, enclave)
	}
	if doc.ConfigRepo == "" {
		return fmt.Errorf("tinfoil: empty config repo")
	}
	if doc.ReleaseDigest == "" {
		return fmt.Errorf("tinfoil: empty release digest")
	}
	return nil
}

func fingerprintsMatch(doc *client.VerificationDocument) error {
	if doc.CodeFingerprint == "" || doc.EnclaveFingerprint == "" {
		return fmt.Errorf("tinfoil: missing measurement fingerprint")
	}
	if doc.CodeFingerprint != doc.EnclaveFingerprint {
		return fmt.Errorf("tinfoil: code fingerprint %s != enclave fingerprint %s",
			doc.CodeFingerprint, doc.EnclaveFingerprint)
	}
	return nil
}

func refuseOtherHosts(hc *http.Client, rawURL string) error {
	resp, err := hc.Get(rawURL)
	if err == nil {
		resp.Body.Close()
		return fmt.Errorf("tinfoil: pinned client reached %s", rawURL)
	}
	if !isHostBoundError(err) {
		return fmt.Errorf("tinfoil: pinned client error for %s was not a host bind: %w", rawURL, err)
	}
	return nil
}

func isHostBoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "refusing to send request") ||
		strings.Contains(msg, "bound to enclave")
}

func writeProbeLines(w io.Writer, report Check) {
	if w == nil {
		return
	}
	order := []string{"attest", "fingerprints", "failClosed", "hostBind"}
	for _, name := range order {
		p, ok := report.Checks[name]
		if !ok {
			continue
		}
		if p.OK {
			fmt.Fprintf(w, "PASS %s\n", name)
		} else {
			fmt.Fprintf(w, "FAIL %s: %s\n", name, p.Detail)
		}
	}
	if report.OK && report.Document != nil {
		fmt.Fprintf(w, "ok repo=%s tag=%s digest=%s\n",
			report.Document.ConfigRepo, report.Document.ReleaseTag, report.Document.ReleaseDigest)
	}
}
