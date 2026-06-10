package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ListChats fetches all pages of chats and returns them as domain Chats.
func (c *Client) ListChats(ctx context.Context) ([]Chat, error) {
	iter := c.sdk.Chats.ListAutoPaging(ctx, beeperdesktopapi.ChatListParams{})
	var out []Chat
	for iter.Next() {
		out = append(out, mapChat(iter.Current()))
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("api: list chats: %w", err)
	}
	return out, nil
}

func mapChat(c beeperdesktopapi.ChatListResponse) Chat {
	return Chat{
		ID:           c.ID,
		AccountID:    c.AccountID,
		Network:      c.Network,
		Title:        c.Title,
		Type:         string(c.Type),
		Unread:       int(c.UnreadCount),
		Mentions:     int(c.UnreadMentionsCount),
		Muted:        c.IsMuted,
		LowPriority:  c.IsLowPriority,
		Pinned:       c.IsPinned,
		Archived:     c.IsArchived,
		MarkedUnread: c.IsMarkedUnread,
		LastActive:   c.LastActivity,
		Preview:      c.Preview.Text,
	}
}
