package api

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
	"github.com/beeper/desktop-api-go/v5/shared"
)

// ListMessages fetches recent messages in a chat.
func (c *Client) ListMessages(ctx context.Context, chatID string) ([]Message, error) {
	page, err := c.sdk.Messages.List(ctx, escapeChatID(chatID), beeperdesktopapi.MessageListParams{})
	if err != nil {
		return nil, compactErr("list messages", err)
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

// SearchQuery filters a message search. Zero fields are omitted from the
// request. Sender takes "me", "others", or a user ID.
type SearchQuery struct {
	Query  string
	ChatID string
	Sender string
	After  time.Time
	Before time.Time
	Limit  int
}

// SearchMessages searches message contents across chats.
func (c *Client) SearchMessages(ctx context.Context, query string) ([]MessageSearchResult, error) {
	return c.SearchMessagesFiltered(ctx, SearchQuery{Query: query})
}

// SearchMessagesFiltered searches messages with the full filter set Beeper's
// index supports: date range, sender, and chat scoping.
func (c *Client) SearchMessagesFiltered(ctx context.Context, q SearchQuery) ([]MessageSearchResult, error) {
	params := beeperdesktopapi.MessageSearchParams{}
	if q.Query != "" {
		params.Query = beeperdesktopapi.String(q.Query)
	}
	if q.ChatID != "" {
		params.ChatIDs = []string{q.ChatID}
	}
	if q.Sender != "" {
		params.Sender = beeperdesktopapi.String(q.Sender)
	}
	if !q.After.IsZero() {
		params.DateAfter = beeperdesktopapi.Time(q.After)
	}
	if !q.Before.IsZero() {
		params.DateBefore = beeperdesktopapi.Time(q.Before)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	params.Limit = beeperdesktopapi.Int(int64(limit))
	page, err := c.sdk.Messages.Search(ctx, params)
	if err != nil {
		return nil, compactErr("search messages", err)
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
	var attachments []Attachment
	for _, a := range m.Attachments {
		attachments = append(attachments, Attachment{
			Type:          string(a.Type),
			ID:            a.ID,
			SrcURL:        a.SrcURL,
			FileName:      a.FileName,
			FileSize:      int64(a.FileSize),
			Duration:      a.Duration,
			MimeType:      a.MimeType,
			Width:         int(a.Size.Width),
			Height:        int(a.Size.Height),
			IsVoiceNote:   a.IsVoiceNote,
			IsGif:         a.IsGif,
			IsSticker:     a.IsSticker,
			Transcription: a.Transcription.Transcription,
		})
	}
	return Message{
		ID:          m.ID,
		ChatID:      m.ChatID,
		SenderName:  m.SenderName,
		Text:        renderText(m),
		Timestamp:   m.Timestamp,
		IsFromMe:    m.IsSender,
		IsUnread:    m.IsUnread,
		IsReaction:  m.Type == shared.MessageTypeReaction,
		Reactions:   reactions,
		Attachments: attachments,
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
