package identity

import (
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

var base = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

func testChats() []api.Chat {
	return []api.Chat{
		{
			ID: "c-alice", AccountID: "ig", Network: "Instagram", Title: "Alicia Ahmed",
			Type: "single", LastActive: base,
		},
		{
			ID: "c-group", AccountID: "wa", Network: "WhatsApp", Title: "Ski Trip", Type: "group",
			LastActive: base.Add(-time.Hour),
			Participants: []api.Participant{
				{UserID: "u-self", FullName: "Me", IsSelf: true},
				{UserID: "u-bob", FullName: "Bob Ramírez", Username: "@bobr"},
			},
		},
		{
			ID: "c-bob", AccountID: "wa", Network: "WhatsApp", Title: "Bob Ramírez", Type: "single",
			LastActive: base.Add(-2 * time.Hour),
			Participants: []api.Participant{
				{UserID: "u-bob", FullName: "Bob Ramírez", Username: "@bobr"},
			},
		},
	}
}

func testContacts() []api.Contact {
	return []api.Contact{
		{AccountID: "wa", UserID: "u-bob", FullName: "Bob Ramírez", PhoneNumber: "+15550000009"},
		{AccountID: "wa", UserID: "u-carol", FullName: "Carol Chen", Email: "carol@example.test"},
	}
}

func search(t *testing.T, query string) []Person {
	t.Helper()
	return Build(testChats(), testContacts()).Search(query, 10)
}

func TestSearchFullName(t *testing.T) {
	got := search(t, "Alicia Ahmed")
	if len(got) == 0 || got[0].Name != "Alicia Ahmed" {
		t.Fatalf("search full name = %+v, want Alicia Ahmed first", got)
	}
	if got[0].Chats[0].ID != "c-alice" {
		t.Errorf("chat ref = %+v, want c-alice", got[0].Chats)
	}
}

func TestSearchPartialAndCase(t *testing.T) {
	for _, q := range []string{"alicia", "ali", "ALICIA AHM", "ali ahm"} {
		got := search(t, q)
		if len(got) == 0 || got[0].Name != "Alicia Ahmed" {
			t.Errorf("search %q first = %+v, want Alicia Ahmed", q, got)
		}
	}
}

func TestSearchTokenOrder(t *testing.T) {
	got := search(t, "ahmed alicia")
	if len(got) == 0 || got[0].Name != "Alicia Ahmed" {
		t.Fatalf("out-of-order tokens = %+v, want Alicia Ahmed", got)
	}
}

func TestSearchDiacritics(t *testing.T) {
	got := search(t, "ramirez")
	if len(got) == 0 || got[0].Name != "Bob Ramírez" {
		t.Fatalf("search ramirez = %+v, want Bob Ramírez", got)
	}
}

func TestSearchByHandleAndEmail(t *testing.T) {
	if got := search(t, "@bobr"); len(got) == 0 || got[0].Name != "Bob Ramírez" {
		t.Errorf("search handle = %+v, want Bob Ramírez", got)
	}
	if got := search(t, "carol@example.test"); len(got) == 0 || got[0].Name != "Carol Chen" {
		t.Errorf("search email = %+v, want Carol Chen", got)
	}
}

func TestMergeAcrossSources(t *testing.T) {
	got := search(t, "bob")
	if len(got) != 1 {
		t.Fatalf("bob entries = %d (%+v), want 1 merged", len(got), got)
	}
	p := got[0]
	if p.Phone != "+15550000009" || p.Username != "@bobr" {
		t.Errorf("merged person = %+v, want contact phone and participant handle", p)
	}
	if len(p.Chats) != 2 {
		t.Errorf("chats = %+v, want group and dm", p.Chats)
	}
}

func TestContactOnlyPerson(t *testing.T) {
	got := search(t, "carol")
	if len(got) != 1 || len(got[0].Chats) != 0 {
		t.Fatalf("contact-only = %+v, want one person with no chats", got)
	}
}

func TestNoMatch(t *testing.T) {
	if got := search(t, "zzzqqq"); len(got) != 0 {
		t.Fatalf("search zzzqqq = %+v, want none", got)
	}
	if got := search(t, ""); got != nil {
		t.Fatalf("empty query = %+v, want nil", got)
	}
}

func TestRecencyBreaksTies(t *testing.T) {
	chats := []api.Chat{
		{ID: "c-old", AccountID: "a", Title: "Sam Old", Type: "single", LastActive: base.Add(-48 * time.Hour)},
		{ID: "c-new", AccountID: "a", Title: "Sam New", Type: "single", LastActive: base},
	}
	got := Build(chats, nil).Search("sam", 10)
	if len(got) != 2 || got[0].Name != "Sam New" {
		t.Fatalf("recency order = %+v, want Sam New first", got)
	}
}
