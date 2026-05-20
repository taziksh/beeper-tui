package api

import (
	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
	"github.com/beeper/desktop-api-go/v5/option"

	"github.com/taziksh/beeper-tui/internal/config"
)

// Client wraps the Beeper Desktop SDK with intention-revealing methods that
// return our own domain types.
type Client struct {
	sdk beeperdesktopapi.Client
}

// New constructs a Client from resolved config.
func New(cfg config.Config) *Client {
	sdk := beeperdesktopapi.NewClient(
		option.WithAccessToken(cfg.Token),
		option.WithBaseURL(cfg.BaseURL),
	)
	return &Client{sdk: sdk}
}
