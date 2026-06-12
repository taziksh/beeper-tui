package api

import (
	"context"
	"fmt"
	"net/url"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// AddReaction adds the authenticated user's reaction to a message. key is an
// emoji or a network shortcode.
func (c *Client) AddReaction(ctx context.Context, chatID, messageID, key string) error {
	_, err := c.sdk.Chats.Messages.Reactions.Add(ctx, url.PathEscape(messageID),
		beeperdesktopapi.ChatMessageReactionAddParams{
			ChatID:      escapeChatID(chatID),
			ReactionKey: key,
		})
	if err != nil {
		return fmt.Errorf("api: add reaction %s to %s: %w", key, messageID, err)
	}
	return nil
}

// RemoveReaction removes the authenticated user's reaction from a message.
func (c *Client) RemoveReaction(ctx context.Context, chatID, messageID, key string) error {
	_, err := c.sdk.Chats.Messages.Reactions.Delete(ctx, url.PathEscape(key),
		beeperdesktopapi.ChatMessageReactionDeleteParams{
			ChatID:    escapeChatID(chatID),
			MessageID: url.PathEscape(messageID),
		})
	if err != nil {
		return fmt.Errorf("api: remove reaction %s from %s: %w", key, messageID, err)
	}
	return nil
}
