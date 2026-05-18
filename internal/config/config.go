package config

import (
	"os"
	"path/filepath"
)

const appName = "beeper-tui"

func XDGConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}
