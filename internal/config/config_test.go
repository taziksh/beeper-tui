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
	t.Setenv("BEEPER_API_BASE_URL", "http://127.0.0.1:9999")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Token != "tok" {
		t.Errorf("Token = %q, want %q", got.Token, "tok")
	}
	if got.BaseURL != "http://127.0.0.1:9999" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://127.0.0.1:9999")
	}
	if got.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}
	if got.CacheDir == "" {
		t.Error("CacheDir is empty")
	}
}

func TestLoad_RefusesRemoteEndpoints(t *testing.T) {
	t.Setenv("BEEPER_TUI_ALLOW_REMOTE", "")
	t.Setenv("BEEPER_LLM_BASE_URL", "https://api.example.com/v1")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with remote LLM endpoint = nil error, want refusal")
	}
	t.Setenv("BEEPER_LLM_BASE_URL", "")
	t.Setenv("BEEPER_API_BASE_URL", "http://10.0.0.5:23373")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with remote API endpoint = nil error, want refusal")
	}
}

func TestLoad_AllowsLocalEndpoints(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:1234/v1", "http://localhost:1234/v1", "http://[::1]:1234/v1"} {
		t.Setenv("BEEPER_LLM_BASE_URL", u)
		if _, err := config.Load(); err != nil {
			t.Errorf("Load() with %s error = %v, want nil", u, err)
		}
	}
}

func TestLoad_RemoteOptOut(t *testing.T) {
	t.Setenv("BEEPER_LLM_BASE_URL", "https://api.example.com/v1")
	t.Setenv("BEEPER_TUI_ALLOW_REMOTE", "1")
	if _, err := config.Load(); err != nil {
		t.Errorf("Load() with opt-out error = %v, want nil", err)
	}
}

func TestLoad_TinfoilKeyAloneStaysLocal(t *testing.T) {
	t.Setenv("BEEPER_TUI_ALLOW_REMOTE", "")
	t.Setenv("BEEPER_LLM_PROVIDER", "")
	t.Setenv("TINFOIL_API_KEY", "k")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.LLMProvider != config.ProviderLocal {
		t.Errorf("LLMProvider = %q, want %q", got.LLMProvider, config.ProviderLocal)
	}

	t.Setenv("BEEPER_LLM_BASE_URL", "https://api.example.com/v1")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with remote LLM endpoint = nil error, want refusal even with TINFOIL_API_KEY set")
	}
}

func TestLoad_TinfoilRequiresKey(t *testing.T) {
	t.Setenv("BEEPER_LLM_PROVIDER", "tinfoil")
	t.Setenv("TINFOIL_API_KEY", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() tinfoil without key = nil error, want refusal")
	}
}

func TestLoad_TinfoilPinsEndpointAndModel(t *testing.T) {
	t.Setenv("BEEPER_TUI_ALLOW_REMOTE", "")
	t.Setenv("BEEPER_LLM_PROVIDER", "tinfoil")
	t.Setenv("TINFOIL_API_KEY", "k")
	t.Setenv("BEEPER_LLM_BASE_URL", "")
	t.Setenv("BEEPER_LLM_MODEL", "")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if want := "https://inference.tinfoil.sh/v1"; got.LLMBaseURL != want {
		t.Errorf("LLMBaseURL = %q, want %q", got.LLMBaseURL, want)
	}
	if want := "deepseek-v4-flash"; got.LLMModel != want {
		t.Errorf("LLMModel = %q, want %q", got.LLMModel, want)
	}
	if got.TinfoilAPIKey != "k" {
		t.Errorf("TinfoilAPIKey = %q, want %q", got.TinfoilAPIKey, "k")
	}
}

func TestLoad_TinfoilModelOverride(t *testing.T) {
	t.Setenv("BEEPER_LLM_PROVIDER", "tinfoil")
	t.Setenv("TINFOIL_API_KEY", "k")
	t.Setenv("BEEPER_LLM_MODEL", "glm-5-2")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.LLMModel != "glm-5-2" {
		t.Errorf("LLMModel = %q, want %q", got.LLMModel, "glm-5-2")
	}
}

func TestLoad_TinfoilRejectsConflictingBaseURL(t *testing.T) {
	t.Setenv("BEEPER_LLM_PROVIDER", "tinfoil")
	t.Setenv("TINFOIL_API_KEY", "k")
	t.Setenv("BEEPER_LLM_BASE_URL", "https://api.example.com/v1")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with conflicting base URL = nil error, want refusal")
	}
}

func TestLoad_TinfoilKeepsAPIEndpointLocal(t *testing.T) {
	t.Setenv("BEEPER_TUI_ALLOW_REMOTE", "")
	t.Setenv("BEEPER_LLM_PROVIDER", "tinfoil")
	t.Setenv("TINFOIL_API_KEY", "k")
	t.Setenv("BEEPER_API_BASE_URL", "http://10.0.0.5:23373")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() tinfoil with remote Beeper API = nil error, want refusal")
	}
}

func TestLoad_UnknownProvider(t *testing.T) {
	t.Setenv("BEEPER_LLM_PROVIDER", "openai")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with unknown provider = nil error, want refusal")
	}
}
