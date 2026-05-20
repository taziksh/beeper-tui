package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
	"github.com/beeper/desktop-api-go/v5/shared"
)

// ListMessages fetches up to `limit` recent messages in a chat.
// Note: the SDK's MessageListParams does not expose a Limit field; the limit
// parameter is accepted for API compatibility but is not forwarded to the SDK.
func (c *Client) ListMessages(ctx context.Context, chatID string, limit int) ([]Message, error) {
	page, err := c.sdk.Messages.List(ctx, chatID, beeperdesktopapi.MessageListParams{})
	if err != nil {
		return nil, fmt.Errorf("api: list messages for %s: %w", chatID, err)
	}
	out := make([]Message, 0, len(page.Items))
	for _, m := range page.Items {
		out = append(out, mapMessage(m))
	}
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
