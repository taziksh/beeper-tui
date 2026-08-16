package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const appName = "beeper-tui"

func XDGConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

func XDGCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

func Token() string {
	return strings.TrimSpace(os.Getenv("BEEPER_ACCESS_TOKEN"))
}

const defaultBaseURL = "http://127.0.0.1:23373"

func BaseURL() string {
	if v := strings.TrimSpace(os.Getenv("BEEPER_API_BASE_URL")); v != "" {
		return v
	}
	return defaultBaseURL
}

// Providers the assistant can talk to. Local is an LM Studio or Ollama
// server on this machine. Tinfoil is confidential inference in an attested
// enclave; see checkEgress for how it interacts with the egress guard.
const (
	ProviderLocal   = "local"
	ProviderTinfoil = "tinfoil"
)

// TinfoilEnclave is the attested enclave host the tinfoil provider pins to.
const TinfoilEnclave = "inference.tinfoil.sh"

const tinfoilLLMBaseURL = "https://" + TinfoilEnclave + "/v1"

// defaultTinfoilModel favors latency: the chat loop pays one round trip
// per tool call, and the catalog's reasoning models take 5 to 20 seconds
// per trivial reply. Measured August 2026: deepseek-v4-flash 0.9s,
// glm-5-2 5.6s, kimi-k3 22s.
const defaultTinfoilModel = "deepseek-v4-flash"

// LLMProvider reads BEEPER_LLM_PROVIDER. Empty means local. The Tinfoil
// API key alone never selects a provider: keys are commonly exported
// machine-wide, and switching on one would silently send data remote.
func LLMProvider() string {
	v := strings.TrimSpace(os.Getenv("BEEPER_LLM_PROVIDER"))
	if v == "" {
		return ProviderLocal
	}
	return v
}

func TinfoilAPIKey() string {
	return strings.TrimSpace(os.Getenv("TINFOIL_API_KEY"))
}

const defaultLLMBaseURL = "http://127.0.0.1:1234/v1"

// LLMBaseURL is the OpenAI-compatible endpoint the chat tab talks to. The
// default is LM Studio's local server; Ollama works via its /v1 path.
func LLMBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("BEEPER_LLM_BASE_URL")); v != "" {
		return v
	}
	return defaultLLMBaseURL
}

// LLMModel is the model id for chat completions. Empty means autodetect the
// first loaded chat model on the server.
func LLMModel() string {
	return strings.TrimSpace(os.Getenv("BEEPER_LLM_MODEL"))
}

type Config struct {
	Token         string
	BaseURL       string
	ConfigDir     string
	CacheDir      string
	LLMProvider   string
	LLMBaseURL    string
	LLMModel      string
	TinfoilAPIKey string
}

func Load() (Config, error) {
	cfgDir, err := XDGConfigDir()
	if err != nil {
		return Config{}, err
	}
	cacheDir, err := XDGCacheDir()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Token:         Token(),
		BaseURL:       BaseURL(),
		ConfigDir:     cfgDir,
		CacheDir:      cacheDir,
		LLMProvider:   LLMProvider(),
		LLMBaseURL:    LLMBaseURL(),
		LLMModel:      LLMModel(),
		TinfoilAPIKey: TinfoilAPIKey(),
	}
	switch cfg.LLMProvider {
	case ProviderLocal:
	case ProviderTinfoil:
		if cfg.TinfoilAPIKey == "" {
			return Config{}, fmt.Errorf("config: BEEPER_LLM_PROVIDER=tinfoil requires TINFOIL_API_KEY")
		}
		if v := strings.TrimSpace(os.Getenv("BEEPER_LLM_BASE_URL")); v != "" && v != tinfoilLLMBaseURL {
			return Config{}, fmt.Errorf("config: BEEPER_LLM_BASE_URL=%q conflicts with BEEPER_LLM_PROVIDER=tinfoil", v)
		}
		cfg.LLMBaseURL = tinfoilLLMBaseURL
		if cfg.LLMModel == "" {
			cfg.LLMModel = defaultTinfoilModel
		}
	default:
		return Config{}, fmt.Errorf("config: unknown BEEPER_LLM_PROVIDER %q", cfg.LLMProvider)
	}
	if err := checkEgress(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// allowRemoteEnv opts out of the local-only egress guarantee. Without it the
// TUI refuses to start when any configured endpoint leaves this machine.
const allowRemoteEnv = "BEEPER_TUI_ALLOW_REMOTE"

// checkEgress enforces that every endpoint the app talks to is local, so
// messages, names, and contacts cannot leave the device by configuration.
// The one exemption is the tinfoil provider: Load pins its LLM URL to the
// attested enclave, and the tinfoil client refuses to send anything unless
// attestation verifies.
func checkEgress(cfg Config) error {
	if os.Getenv(allowRemoteEnv) == "1" {
		return nil
	}
	if err := checkLocalURL(cfg.BaseURL); err != nil {
		return err
	}
	if cfg.LLMProvider == ProviderTinfoil {
		return nil
	}
	return checkLocalURL(cfg.LLMBaseURL)
}

// checkLocalURL reports an error unless raw points at this machine.
func checkLocalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("config: invalid endpoint %q: %v", raw, err)
	}
	host := u.Hostname()
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("config: endpoint %q is not local; personal data stays on this machine unless %s=1", raw, allowRemoteEnv)
}
