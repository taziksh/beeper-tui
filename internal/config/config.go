package config

import (
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

type Config struct {
	Token     string
	BaseURL   string
	ConfigDir string
	CacheDir  string
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
	return Config{
		Token:     Token(),
		BaseURL:   BaseURL(),
		ConfigDir: cfgDir,
		CacheDir:  cacheDir,
	}, nil
}
