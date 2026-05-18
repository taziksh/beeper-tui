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

func TestXDGCacheDir_EndsInBeeperTUI(t *testing.T) {
	got, err := config.XDGCacheDir()
	if err != nil {
		t.Fatalf("XDGCacheDir() error = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("beeper-tui")) {
		t.Errorf("XDGCacheDir() = %q, want path ending in 'beeper-tui'", got)
	}
}

func TestToken_ReadsEnvVar(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"set", "abc123", "abc123"},
		{"empty", "", ""},
		{"whitespace stripped", "  xyz  ", "xyz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEEPER_ACCESS_TOKEN", tc.env)
			if got := config.Token(); got != tc.want {
				t.Errorf("Token() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBaseURL_DefaultsToLocalhost(t *testing.T) {
	t.Setenv("BEEPER_API_BASE_URL", "")
	if got, want := config.BaseURL(), "http://127.0.0.1:23373"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

func TestBaseURL_HonorsEnvOverride(t *testing.T) {
	t.Setenv("BEEPER_API_BASE_URL", "http://192.168.1.10:9999")
	if got, want := config.BaseURL(), "http://192.168.1.10:9999"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

func TestLoad_AssemblesAllFields(t *testing.T) {
	t.Setenv("BEEPER_ACCESS_TOKEN", "tok")
	t.Setenv("BEEPER_API_BASE_URL", "http://x.test")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Token != "tok" {
		t.Errorf("Token = %q, want %q", got.Token, "tok")
	}
	if got.BaseURL != "http://x.test" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://x.test")
	}
	if got.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}
	if got.CacheDir == "" {
		t.Error("CacheDir is empty")
	}
}
