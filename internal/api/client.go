package api

import (
	"net/url"

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

// escapeChatID percent-encodes a chat ID for safe interpolation into a request
// path. The SDK builds paths with fmt.Sprintf and no encoding, so an ID
// containing '#' (iMessage IDs look like "imsg##thread:...") is otherwise parsed
// as a URL fragment and truncated, and the server only sees "imsg". Encoding
// '#' -> %23 keeps the whole ID in the path.
func escapeChatID(id string) string {
	return url.PathEscape(id)
}
