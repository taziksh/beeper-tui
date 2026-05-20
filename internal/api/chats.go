package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ListChats fetches the first page of chats and returns them as domain Chats.
// (Pagination across all pages is added in the next task.)
func (c *Client) ListChats(ctx context.Context) ([]Chat, error) {
	page, err := c.sdk.Chats.List(ctx, beeperdesktopapi.ChatListParams{})
	if err != nil {
		return nil, fmt.Errorf("api: list chats: %w", err)
	}
	out := make([]Chat, 0, len(page.Items))
	for _, item := range page.Items {
		out = append(out, mapChat(item))
	}
	return out, nil
}

func mapChat(c beeperdesktopapi.ChatListResponse) Chat {
	return Chat{
		ID:         c.ID,
		AccountID:  c.AccountID,
		Network:    c.Network,
		Title:      c.Title,
		Type:       string(c.Type),
		Unread:     int(c.UnreadCount),
		LastActive: c.LastActivity,
		Preview:    c.Preview.Text,
	}
}
