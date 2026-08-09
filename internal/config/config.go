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
	Token      string
	BaseURL    string
	ConfigDir  string
	CacheDir   string
	LLMBaseURL string
	LLMModel   string
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
		Token:      Token(),
		BaseURL:    BaseURL(),
		ConfigDir:  cfgDir,
		CacheDir:   cacheDir,
		LLMBaseURL: LLMBaseURL(),
		LLMModel:   LLMModel(),
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
func checkEgress(cfg Config) error {
	if os.Getenv(allowRemoteEnv) == "1" {
		return nil
	}
	for _, raw := range []string{cfg.BaseURL, cfg.LLMBaseURL} {
		if err := checkLocalURL(raw); err != nil {
			return err
		}
	}
	return nil
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
