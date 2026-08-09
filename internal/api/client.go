package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
	"github.com/beeper/desktop-api-go/v5/option"

	"github.com/taziksh/beeper-tui/internal/config"
)

// compactErr rewrites SDK HTTP errors to operation + status, dropping the
// request URL, which is noisy and can carry search text.
func compactErr(op string, err error) error {
	var apierr *beeperdesktopapi.Error
	if errors.As(err, &apierr) {
		return fmt.Errorf("api: %s: HTTP %d %s", op, apierr.StatusCode, apierr.RawJSON())
	}
	return fmt.Errorf("api: %s: %w", op, err)
}

// Client wraps the Beeper Desktop SDK with intention-revealing methods that
// return our own domain types.
type Client struct {
	sdk beeperdesktopapi.Client

	contactsMu sync.Mutex
	contacts   []Contact
	contactsAt time.Time

	selfMu  sync.Mutex
	selfIDs map[string]string // accountID -> the user's own user ID there
}

// SelfIDs returns the user's own user ID per account, cached for the
// session. Some bridges do not set isSender on messages, so comparing sender
// IDs against these is the reliable way to spot the user's own messages.
func (c *Client) SelfIDs(ctx context.Context) map[string]string {
	c.selfMu.Lock()
	defer c.selfMu.Unlock()
	if c.selfIDs != nil {
		return c.selfIDs
	}
	accounts, err := c.sdk.Accounts.List(ctx)
	if err != nil {
		return map[string]string{}
	}
	ids := map[string]string{}
	for _, a := range *accounts {
		ids[a.AccountID] = a.User.ID
	}
	c.selfIDs = ids
	return ids
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
