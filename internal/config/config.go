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
