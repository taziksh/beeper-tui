package api_test

import (
	"testing"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
)

func TestNew_ReturnsNonNilClient(t *testing.T) {
	c := api.New(config.Config{
		Token:   "test-token",
		BaseURL: "http://127.0.0.1:23373",
	})
	if c == nil {
		t.Fatal("New() returned nil")
	}
}
