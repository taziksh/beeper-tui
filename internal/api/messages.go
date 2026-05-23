package api

import (
	"context"
	"fmt"
	"sort"

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
		Text:       m.Text,
		Timestamp:  m.Timestamp,
		IsFromMe:   m.IsSender,
	}
}
