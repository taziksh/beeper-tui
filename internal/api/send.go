package api

import (
	"context"
	"fmt"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// SendMessage sends a plain-text message to a chat. The SDK confirms the send
// asynchronously (returning a pending id we don't need here); only success or
// failure matters — the UI shows the message optimistically.
func (c *Client) SendMessage(ctx context.Context, chatID, text string) error {
	_, err := c.sdk.Messages.Send(ctx, escapeChatID(chatID), beeperdesktopapi.MessageSendParams{
		Text: beeperdesktopapi.String(text),
	})
	if err != nil {
		return fmt.Errorf("api: send message to %s: %w", chatID, err)
	}
	return nil
}
