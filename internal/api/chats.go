package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ListChats fetches all pages of chats and returns them as domain Chats.
func (c *Client) ListChats(ctx context.Context) ([]Chat, error) {
	selfIDs := c.SelfIDs(ctx)
	iter := c.sdk.Chats.ListAutoPaging(ctx, beeperdesktopapi.ChatListParams{})
	var out []Chat
	for iter.Next() {
		cur := iter.Current()
		ch := mapChat(cur)
		// Some bridges never set isSender on previews; the sender ID is the
		// reliable signal that the last message is the user's own.
		if !ch.LastFromMe && cur.Preview.SenderID != "" && cur.Preview.SenderID == selfIDs[ch.AccountID] {
			ch.LastFromMe = true
		}
		out = append(out, ch)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("api: list chats: %w", err)
	}
	return out, nil
}

// GetChat fetches one chat by ID. The single-chat endpoint returns no preview
// text, so Preview is empty and callers keep any value they already have.
func (c *Client) GetChat(ctx context.Context, chatID string) (Chat, error) {
	ch, err := c.sdk.Chats.Get(ctx, escapeChatID(chatID), beeperdesktopapi.ChatGetParams{})
	if err != nil {
		return Chat{}, fmt.Errorf("api: get chat %s: %w", chatID, err)
	}
	return mapChatCore(*ch), nil
}

// SearchChats asks the server to find chats matching query across every
// inbox, including low priority and archive. The server matches participant
// names as well as titles, so it finds chats the local list may miss.
func (c *Client) SearchChats(ctx context.Context, query string) ([]Chat, error) {
	iter := c.sdk.Chats.SearchAutoPaging(ctx, beeperdesktopapi.ChatSearchParams{
		Query: beeperdesktopapi.String(query),
	})
	var out []Chat
	for iter.Next() && len(out) < 100 {
		out = append(out, mapChatCore(iter.Current()))
	}
	if err := iter.Err(); err != nil {
		return nil, compactErr("search chats", err)
	}
	return out, nil
}

// mapChatCore maps the SDK chat fields shared by every chat endpoint.
func mapChatCore(ch beeperdesktopapi.Chat) Chat {
	return Chat{
		ID:           ch.ID,
		AccountID:    ch.AccountID,
		Network:      ch.Network,
		Title:        ch.Title,
		Type:         string(ch.Type),
		Unread:       int(ch.UnreadCount),
		Mentions:     int(ch.UnreadMentionsCount),
		Muted:        ch.IsMuted,
		LowPriority:  ch.IsLowPriority,
		Pinned:       ch.IsPinned,
		Archived:     ch.IsArchived,
		MarkedUnread: ch.IsMarkedUnread,
		LastActive:   ch.LastActivity,

		AllowedReactions: ch.Capabilities.AllowedReactions,
		Participants:     mapParticipants(ch.Participants),
	}
}

func mapChat(c beeperdesktopapi.ChatListResponse) Chat {
	ch := mapChatCore(c.Chat)
	ch.Preview = c.Preview.Text
	ch.PreviewSenderID = c.Preview.SenderID
	ch.LastFromMe = c.Preview.IsSender
	ch.LastSender = c.Preview.SenderName
	return ch
}

func mapParticipants(p beeperdesktopapi.ChatParticipants) []Participant {
	var out []Participant
	for _, u := range p.Items {
		out = append(out, Participant{
			UserID:   u.ID,
			FullName: u.FullName,
			Username: u.Username,
			IsSelf:   u.IsSelf,
			IsBot:    u.IsNetworkBot,
		})
	}
	return out
}
