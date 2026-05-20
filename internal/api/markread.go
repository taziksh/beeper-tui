package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// MarkRead marks an entire chat as read.
func (c *Client) MarkRead(ctx context.Context, chatID string) error {
	_, err := c.sdk.Chats.MarkRead(ctx, chatID, beeperdesktopapi.ChatMarkReadParams{})
	if err != nil {
		return fmt.Errorf("api: mark read %s: %w", chatID, err)
	}
	return nil
}
