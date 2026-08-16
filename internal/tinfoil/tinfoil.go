// Package tinfoil dials the Tinfoil confidential-inference enclave. It
// isolates the vendor SDK behind one function so the rest of the app only
// ever sees a standard http.Client.
package tinfoil

import (
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	tinfoil "github.com/tinfoilsh/tinfoil-go"

	"github.com/taziksh/beeper-tui/internal/config"
)

// Dial verifies the enclave attestation and returns an HTTP client bound
// to it. Construction fails closed: on any verification error there is no
// client, so no request can carry data out. The returned client encrypts
// request bodies to the attested enclave key and refuses other hosts.
func Dial() (*http.Client, error) {
	// The SDK logs through the global logrus, which would corrupt the
	// terminal UI.
	logrus.SetOutput(io.Discard)
	tc, err := tinfoil.NewClientWithOptions(tinfoil.WithEnclave(config.TinfoilEnclave))
	if err != nil {
		return nil, fmt.Errorf("tinfoil: enclave attestation failed, nothing was sent: %w", err)
	}
	return tc.HTTPClient(), nil
}
