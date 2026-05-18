package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/taziksh/beeper-tui/internal/config"
)

func TestXDGConfigDir_EndsInBeeperTUI(t *testing.T) {
	got, err := config.XDGConfigDir()
	if err != nil {
		t.Fatalf("XDGConfigDir() error = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("beeper-tui")) {
		t.Errorf("XDGConfigDir() = %q, want path ending in 'beeper-tui'", got)
	}
}
