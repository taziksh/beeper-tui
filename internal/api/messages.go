package api

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
	"github.com/beeper/desktop-api-go/v5/shared"
)

// ListMessages fetches recent messages in a chat.
func (c *Client) ListMessages(ctx context.Context, chatID string) ([]Message, error) {
	page, err := c.sdk.Messages.List(ctx, escapeChatID(chatID), beeperdesktopapi.MessageListParams{})
	if err != nil {
		return nil, fmt.Errorf("api: list messages for %s: %w", chatID, err)
	}
	out := make([]Message, 0, len(page.Items))
	for _, m := range page.Items {
		// Reaction events double what Message.Reactions already carries;
		// Desktop hides them too.
		if m.Type == shared.MessageTypeReaction {
			continue
		}
		out = append(out, mapMessage(m))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

// SearchMessages searches message contents across chats.
func (c *Client) SearchMessages(ctx context.Context, query string) ([]MessageSearchResult, error) {
	page, err := c.sdk.Messages.Search(ctx, beeperdesktopapi.MessageSearchParams{
		Query: beeperdesktopapi.String(query),
		Limit: beeperdesktopapi.Int(20),
	})
	if err != nil {
		return nil, fmt.Errorf("api: search messages: %w", err)
	}
	out := make([]MessageSearchResult, 0, len(page.Items))
	for _, m := range page.Items {
		if m.Type == shared.MessageTypeReaction {
			continue
		}
		out = append(out, MessageSearchResult{Message: mapMessage(m)})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Message.Timestamp.After(out[j].Message.Timestamp)
	})
	return out, nil
}

// MessageFromJSON decodes one message object from a WebSocket event entry,
// which carries the same schema as REST messages.
func MessageFromJSON(raw []byte) (Message, error) {
	var m shared.Message
	if err := m.UnmarshalJSON(raw); err != nil {
		return Message{}, fmt.Errorf("api: decode event message: %w", err)
	}
	return mapMessage(m), nil
}

func mapMessage(m shared.Message) Message {
	var reactions []Reaction
	for _, r := range m.Reactions {
		reactions = append(reactions, Reaction{Key: r.ReactionKey, Emoji: r.Emoji, ParticipantID: r.ParticipantID})
	}
	return Message{
		ID:         m.ID,
		ChatID:     m.ChatID,
		SenderName: m.SenderName,
		Text:       renderText(m),
		Timestamp:  m.Timestamp,
		IsFromMe:   m.IsSender,
		IsUnread:   m.IsUnread,
		IsReaction: m.Type == shared.MessageTypeReaction,
		Reactions:  reactions,
	}
}

// renderText decodes HTML entities and substitutes templated placeholders
// (e.g. the {{sender}} used in reaction text) into the resolved sender name,
// or "You" for the authenticated user's own messages.
func renderText(m shared.Message) string {
	sender := m.SenderName
	if m.IsSender {
		sender = "You"
	}
	text := strings.ReplaceAll(m.Text, "{{sender}}", sender)
	return html.UnescapeString(text)
}
