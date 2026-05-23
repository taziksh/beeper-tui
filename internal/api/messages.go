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
		out = append(out, mapMessage(m))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

func mapMessage(m shared.Message) Message {
	return Message{
		ID:         m.ID,
		ChatID:     m.ChatID,
		SenderName: m.SenderName,
		Text:       renderText(m),
		Timestamp:  m.Timestamp,
		IsFromMe:   m.IsSender,
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
