// Package ws implements a client for the Beeper Desktop live-events
// WebSocket. It connects to /v1/ws, subscribes to chat events, and exposes
// them as a channel of typed Events. The package does not import bubbletea;
// the UI layer bridges the channel into tea.Msg values.
package ws

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// EventType discriminates the values delivered on the Events channel.
type EventType string

// Domain events pushed by the server.
const (
	EventMessageUpserted EventType = "message.upserted"
	EventMessageDeleted  EventType = "message.deleted"
	EventChatUpserted    EventType = "chat.upserted"
	EventChatDeleted     EventType = "chat.deleted"
)

// Connection-state changes synthesized by the client so the UI can show
// connection status without watching the socket itself.
const (
	EventConnecting   EventType = "connecting"
	EventConnected    EventType = "connected"
	EventDisconnected EventType = "disconnected"
)

// Event is one value on the Events channel: either a domain event decoded
// from the wire or a connection-state change.
type Event struct {
	Type EventType

	// Domain-event fields. Seq is a monotonic per-connection counter; a jump
	// indicates missed events. Message upserts carry full objects in Entries;
	// chat upserts and all deletes carry only IDs. Entries stay raw JSON for
	// the UI layer to map onto api.Message and api.Chat.
	Seq       int64
	TS        time.Time
	AccountID string
	ChatID    string
	IDs       []string
	Entries   []json.RawMessage

	// Disconnected-event fields.
	Err     error         // why the connection dropped, nil otherwise
	RetryIn time.Duration // delay before the next automatic reconnect
}

// subscriptionsSet is the only client-to-server command we send.
// ChatIDs of ["*"] subscribes to all chats; [] pauses delivery.
type subscriptionsSet struct {
	Type      string   `json:"type"`
	RequestID string   `json:"requestID"`
	ChatIDs   []string `json:"chatIDs"`
}

// ready is the server's initial handshake after the connection opens.
type ready struct {
	Version int
	ChatIDs []string
}

// subscriptionsUpdated acknowledges a subscriptions.set command.
type subscriptionsUpdated struct {
	RequestID string
	ChatIDs   []string
}

// serverError reports a rejected command.
type serverError struct {
	RequestID string
	Code      string
	Message   string
}

// wireFrame is the superset of fields across all server frames; Type
// discriminates which are populated.
type wireFrame struct {
	Type      string            `json:"type"`
	RequestID string            `json:"requestID"`
	Version   int               `json:"version"`
	ChatIDs   []string          `json:"chatIDs"`
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Seq       int64             `json:"seq"`
	TS        flexTime          `json:"ts"`
	AccountID string            `json:"accountID"`
	ChatID    string            `json:"chatID"`
	IDs       []string          `json:"ids"`
	Entries   []json.RawMessage `json:"entries"`
}

// flexTime decodes the wire timestamp leniently. The live server sends ts as
// a string while the documented examples show an integer, so accept epoch
// milliseconds as a number or string, or an RFC 3339 string. An unknown
// format decodes to the zero time rather than erroring: a bad timestamp must
// not cost us the whole event.
type flexTime struct {
	time.Time
}

func (ft *flexTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var millis int64
	if err := json.Unmarshal(data, &millis); err == nil {
		ft.Time = time.UnixMilli(millis)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		ft.Time = t
		return nil
	}
	if millis, err := strconv.ParseInt(s, 10, 64); err == nil {
		ft.Time = time.UnixMilli(millis)
	}
	return nil
}

// decodeFrame decodes one server frame into a ready, subscriptionsUpdated,
// serverError, or domain Event. Unknown frame types are an error so callers
// can skip them without dropping the connection.
func decodeFrame(data []byte) (any, error) {
	var f wireFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("ws: decode frame: %w", err)
	}
	switch f.Type {
	case "ready":
		return ready{Version: f.Version, ChatIDs: f.ChatIDs}, nil
	case "subscriptions.updated":
		return subscriptionsUpdated{RequestID: f.RequestID, ChatIDs: f.ChatIDs}, nil
	case "error":
		return serverError{RequestID: f.RequestID, Code: f.Code, Message: f.Message}, nil
	case string(EventMessageUpserted), string(EventMessageDeleted),
		string(EventChatUpserted), string(EventChatDeleted):
		return Event{
			Type:      EventType(f.Type),
			Seq:       f.Seq,
			TS:        f.TS.Time,
			AccountID: f.AccountID,
			ChatID:    f.ChatID,
			IDs:       f.IDs,
			Entries:   f.Entries,
		}, nil
	default:
		return nil, fmt.Errorf("ws: unknown frame type %q", f.Type)
	}
}
