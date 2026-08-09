package api

import (
	"context"
	"fmt"
	"time"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// SetReminder schedules a Beeper reminder on a chat. dismissOnMessage cancels
// it if someone messages in the chat first.
func (c *Client) SetReminder(ctx context.Context, chatID string, at time.Time, dismissOnMessage bool) error {
	err := c.sdk.Chats.Reminders.New(ctx, escapeChatID(chatID), beeperdesktopapi.ChatReminderNewParams{
		Reminder: beeperdesktopapi.ChatReminderNewParamsReminder{
			RemindAt:                 at,
			DismissOnIncomingMessage: beeperdesktopapi.Bool(dismissOnMessage),
		},
	})
	if err != nil {
		return fmt.Errorf("api: set reminder on %s: %w", chatID, err)
	}
	return nil
}

// ClearReminder removes a chat's reminder.
func (c *Client) ClearReminder(ctx context.Context, chatID string) error {
	if err := c.sdk.Chats.Reminders.Delete(ctx, escapeChatID(chatID)); err != nil {
		return fmt.Errorf("api: clear reminder on %s: %w", chatID, err)
	}
	return nil
}
