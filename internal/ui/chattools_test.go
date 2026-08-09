package ui

import (
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

func TestIsUnanswered(t *testing.T) {
	base := api.Chat{Type: "single", Preview: "hey", LastSender: "Ada"}
	for _, tc := range []struct {
		name string
		mod  func(c api.Chat) api.Chat
		want bool
	}{
		{"dm waiting", func(c api.Chat) api.Chat { return c }, true},
		{"i replied last", func(c api.Chat) api.Chat { c.LastFromMe = true; return c }, false},
		{"muted", func(c api.Chat) api.Chat { c.Muted = true; return c }, false},
		{"low priority", func(c api.Chat) api.Chat { c.LowPriority = true; return c }, false},
		{"archived", func(c api.Chat) api.Chat { c.Archived = true; return c }, false},
		{"no preview", func(c api.Chat) api.Chat { c.Preview = ""; return c }, false},
		{"group without mention", func(c api.Chat) api.Chat { c.Type = "group"; return c }, false},
		{"group with mention", func(c api.Chat) api.Chat { c.Type = "group"; c.Mentions = 1; return c }, true},
		{"bot dm", func(c api.Chat) api.Chat {
			c.Participants = []api.Participant{{UserID: "u-bot", FullName: "Notifier", IsBot: true}}
			return c
		}, false},
		{"human dm with participants", func(c api.Chat) api.Chat {
			c.Participants = []api.Participant{{UserID: "u-ada", FullName: "Ada Testface"}}
			return c
		}, true},
	} {
		if got := isUnanswered(tc.mod(base)); got != tc.want {
			t.Errorf("%s: isUnanswered = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseToolDate(t *testing.T) {
	after, err := parseToolDate("2026-08-01", false)
	if err != nil || after.Day() != 1 {
		t.Fatalf("after = %v, %v", after, err)
	}
	before, err := parseToolDate("2026-08-07", true)
	if err != nil || before.Day() != 8 {
		t.Fatalf("before should land on next day, got %v, %v", before, err)
	}
	if _, err := parseToolDate("aug 7", false); err == nil {
		t.Error("want error for non-ISO date")
	}
	if d, err := parseToolDate("", false); err != nil || !d.IsZero() {
		t.Errorf("empty date should be zero, got %v, %v", d, err)
	}
}

func TestParseToolTimestamp(t *testing.T) {
	for _, ok := range []string{"2026-08-08T09:00", "2026-08-08 09:00", "2026-08-08T09:00:00Z"} {
		if _, err := parseToolTimestamp(ok); err != nil {
			t.Errorf("parseToolTimestamp(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"tomorrow", "2026-08-08", ""} {
		if _, err := parseToolTimestamp(bad); err == nil {
			t.Errorf("parseToolTimestamp(%q): want error", bad)
		}
	}
}

func TestResolveChat(t *testing.T) {
	chats := []api.Chat{
		{ID: "c1", Title: "Ada Testface", LastActive: time.Now()},
		{ID: "c2", Title: "Fixture Friends"},
		{ID: "c3", Title: "Fixture Family"},
	}
	if c, _ := resolveChat(chats, "c2"); c == nil || c.ID != "c2" {
		t.Error("exact id lookup failed")
	}
	if c, _ := resolveChat(chats, "ada testface"); c == nil || c.ID != "c1" {
		t.Error("case-folded title lookup failed")
	}
	if c, _ := resolveChat(chats, "friends"); c == nil || c.ID != "c2" {
		t.Error("unique substring lookup failed")
	}
	if c, cands := resolveChat(chats, "fixture"); c != nil || len(cands) != 2 {
		t.Errorf("ambiguous lookup: chat %v candidates %v", c, cands)
	}
	if c, cands := resolveChat(chats, "zzz"); c != nil || len(cands) != 0 {
		t.Error("miss should return nothing")
	}
}
