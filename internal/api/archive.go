package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ArchiveChat moves a chat into or out of the archived chats collection.
func (c *Client) ArchiveChat(ctx context.Context, chatID string, archived bool) error {
	err := c.sdk.Chats.Archive(ctx, escapeChatID(chatID), beeperdesktopapi.ChatArchiveParams{
		Archived: beeperdesktopapi.Bool(archived),
	})
	if err != nil {
		return fmt.Errorf("api: archive chat %s: %w", chatID, err)
	}
	return nil
}
