package api

import (
	"context"
	"fmt"
)

// SelfUserIDs returns the authenticated user's own user ID for each account,
// keyed by account ID. Reactions carry only a participant ID, so this map is
// how callers recognize the user's own reactions.
func (c *Client) SelfUserIDs(ctx context.Context) (map[string]string, error) {
	accounts, err := c.sdk.Accounts.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("api: list accounts: %w", err)
	}
	out := make(map[string]string, len(*accounts))
	for _, a := range *accounts {
		out[a.AccountID] = a.User.ID
	}
	return out, nil
}
