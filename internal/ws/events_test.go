package ws

import (
	"testing"
	"time"
)

func TestDecodeFrame_Ready(t *testing.T) {
	got, err := decodeFrame([]byte(`{"type":"ready","version":1,"chatIDs":[]}`))
	if err != nil {
		t.Fatalf("decodeFrame() error = %v", err)
	}
	r, ok := got.(ready)
	if !ok {
		t.Fatalf("decodeFrame() = %T, want ready", got)
	}
	if r.Version != 1 {
		t.Errorf("Version = %d, want 1", r.Version)
	}
}

func TestDecodeFrame_SubscriptionsUpdated(t *testing.T) {
	got, err := decodeFrame([]byte(`{"type":"subscriptions.updated","requestID":"r2","chatIDs":["*"]}`))
	if err != nil {
		t.Fatalf("decodeFrame() error = %v", err)
	}
	s, ok := got.(subscriptionsUpdated)
	if !ok {
		t.Fatalf("decodeFrame() = %T, want subscriptionsUpdated", got)
	}
	if s.RequestID != "r2" {
		t.Errorf("RequestID = %q, want %q", s.RequestID, "r2")
	}
	if len(s.ChatIDs) != 1 || s.ChatIDs[0] != "*" {
		t.Errorf("ChatIDs = %v, want [*]", s.ChatIDs)
	}
}

func TestDecodeFrame_ServerError(t *testing.T) {
	got, err := decodeFrame([]byte(`{"type":"error","requestID":"r3","code":"INVALID_PAYLOAD","message":"bad"}`))
	if err != nil {
		t.Fatalf("decodeFrame() error = %v", err)
	}
	e, ok := got.(serverError)
	if !ok {
		t.Fatalf("decodeFrame() = %T, want serverError", got)
	}
	if e.Code != "INVALID_PAYLOAD" {
		t.Errorf("Code = %q, want INVALID_PAYLOAD", e.Code)
	}
}

func TestDecodeFrame_DomainEvents(t *testing.T) {
	tests := []struct {
		name string
		data string
		want Event
	}{
		{
			name: "message upserted",
			data: `{"type":"message.upserted","seq":42,"ts":1739320000000,"chatID":"chat_a","ids":["m1"],"entries":[{"id":"m1","text":"hello"}]}`,
			want: Event{Type: EventMessageUpserted, Seq: 42, ChatID: "chat_a", IDs: []string{"m1"}},
		},
		{
			name: "message deleted",
			data: `{"type":"message.deleted","seq":43,"ts":1739320000000,"chatID":"chat_a","ids":["m1"]}`,
			want: Event{Type: EventMessageDeleted, Seq: 43, ChatID: "chat_a", IDs: []string{"m1"}},
		},
		{
			name: "chat upserted",
			data: `{"type":"chat.upserted","seq":44,"ts":1739320000000,"chatID":"chat_b","ids":["chat_b"],"entries":[{"id":"chat_b","title":"Alice"}]}`,
			want: Event{Type: EventChatUpserted, Seq: 44, ChatID: "chat_b", IDs: []string{"chat_b"}},
		},
		{
			name: "chat deleted",
			data: `{"type":"chat.deleted","seq":45,"ts":1739320000000,"chatID":"chat_b","ids":["chat_b"]}`,
			want: Event{Type: EventChatDeleted, Seq: 45, ChatID: "chat_b", IDs: []string{"chat_b"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeFrame([]byte(tt.data))
			if err != nil {
				t.Fatalf("decodeFrame() error = %v", err)
			}
			e, ok := got.(Event)
			if !ok {
				t.Fatalf("decodeFrame() = %T, want Event", got)
			}
			if e.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", e.Type, tt.want.Type)
			}
			if e.Seq != tt.want.Seq {
				t.Errorf("Seq = %d, want %d", e.Seq, tt.want.Seq)
			}
			if e.ChatID != tt.want.ChatID {
				t.Errorf("ChatID = %q, want %q", e.ChatID, tt.want.ChatID)
			}
			if len(e.IDs) != len(tt.want.IDs) || e.IDs[0] != tt.want.IDs[0] {
				t.Errorf("IDs = %v, want %v", e.IDs, tt.want.IDs)
			}
			if want := time.UnixMilli(1739320000000); !e.TS.Equal(want) {
				t.Errorf("TS = %v, want %v", e.TS, want)
			}
		})
	}
}

// The live server sends ts as a string even though documented examples show
// an integer; decoding must accept every observed and plausible format, and
// an unparseable ts must never reject the event itself.
func TestDecodeFrame_TimestampFormats(t *testing.T) {
	tests := []struct {
		name string
		ts   string
		want time.Time
	}{
		{name: "epoch millis number", ts: `1739320000000`, want: time.UnixMilli(1739320000000)},
		{name: "epoch millis string", ts: `"1739320000000"`, want: time.UnixMilli(1739320000000)},
		{name: "RFC3339 string", ts: `"2026-06-10T12:00:00.500Z"`, want: time.Date(2026, 6, 10, 12, 0, 0, 500_000_000, time.UTC)},
		{name: "unparseable string", ts: `"next tuesday"`, want: time.Time{}},
		{name: "missing", ts: `null`, want: time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := `{"type":"message.upserted","seq":1,"ts":` + tt.ts + `,"chatID":"c","ids":["m1"]}`
			got, err := decodeFrame([]byte(data))
			if err != nil {
				t.Fatalf("decodeFrame() error = %v, want event despite ts format", err)
			}
			e, ok := got.(Event)
			if !ok {
				t.Fatalf("decodeFrame() = %T, want Event", got)
			}
			if !e.TS.Equal(tt.want) {
				t.Errorf("TS = %v, want %v", e.TS, tt.want)
			}
		})
	}
}

func TestDecodeFrame_EntriesKeptRaw(t *testing.T) {
	got, err := decodeFrame([]byte(`{"type":"message.upserted","seq":1,"ts":1,"chatID":"c","ids":["m1","m2"],"entries":[{"id":"m1"},{"id":"m2"}]}`))
	if err != nil {
		t.Fatalf("decodeFrame() error = %v", err)
	}
	e := got.(Event)
	if len(e.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(e.Entries))
	}
	if string(e.Entries[0]) != `{"id":"m1"}` {
		t.Errorf("Entries[0] = %s, want raw JSON object", e.Entries[0])
	}
}

func TestDecodeFrame_Errors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "unknown type", data: `{"type":"presence.updated"}`},
		{name: "malformed JSON", data: `{"type":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeFrame([]byte(tt.data)); err == nil {
				t.Error("decodeFrame() error = nil, want non-nil")
			}
		})
	}
}
