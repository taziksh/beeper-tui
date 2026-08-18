package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/config"
	"github.com/taziksh/beeper-tui/internal/identity"
	"github.com/taziksh/beeper-tui/internal/redact"
)

func TestSearchQueryWords(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"lisa", []string{"lisa"}},
		{"Lisa Wang", []string{"Lisa", "Wang"}},
		{"  Lisa   Wang  ", []string{"Lisa", "Wang"}},
		{"Lisa Lisa Wang", []string{"Lisa", "Wang"}},
		{"to Lisa Wang", []string{"Lisa", "Wang"}},
		{"Dana Kim Fixture", []string{"Dana", "Kim", "Fixture"}},
		{"", nil},
		{"  ", nil},
		{"a b", []string{"a b"}},
		{"ok go", []string{"ok go"}},
	} {
		if got := searchQueryWords(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("searchQueryWords(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMergeSearchResults(t *testing.T) {
	newer := time.Date(2026, 8, 18, 13, 29, 0, 0, time.UTC)
	mid := newer.Add(-time.Hour)
	older := newer.Add(-2 * time.Hour)

	t.Run("dedupes by id and newest first", func(t *testing.T) {
		got := mergeSearchResults([][]api.MessageSearchResult{
			{{Message: api.Message{ID: "m1", Text: "Hi Lisa", Timestamp: older}}},
			{
				{Message: api.Message{ID: "m1", Text: "Hi Lisa", Timestamp: older}},
				{Message: api.Message{ID: "m2", Text: "Wang?", Timestamp: newer}},
				{Message: api.Message{ID: "m3", Text: "mid", Timestamp: mid}},
			},
		}, 10)
		if len(got) != 3 || got[0].Message.ID != "m2" || got[1].Message.ID != "m3" || got[2].Message.ID != "m1" {
			t.Fatalf("got %+v", idsOf(got))
		}
	})
	t.Run("caps at limit after merge", func(t *testing.T) {
		got := mergeSearchResults([][]api.MessageSearchResult{
			{{Message: api.Message{ID: "m1", Text: "Hi Lisa", Timestamp: older}}},
			{{Message: api.Message{ID: "m2", Text: "Wang?", Timestamp: newer}}},
		}, 1)
		if len(got) != 1 || got[0].Message.ID != "m2" {
			t.Fatalf("got %+v, want newest unique m2", idsOf(got))
		}
	})
	t.Run("default limit is 20", func(t *testing.T) {
		var set []api.MessageSearchResult
		for i := 0; i < 25; i++ {
			set = append(set, api.MessageSearchResult{Message: api.Message{
				ID:        fmt.Sprintf("m%d", i),
				Timestamp: newer.Add(time.Duration(i) * time.Minute),
			}})
		}
		got := mergeSearchResults([][]api.MessageSearchResult{set}, 0)
		if len(got) != 20 {
			t.Fatalf("len = %d, want 20", len(got))
		}
	})
	t.Run("dedupes messages with no id", func(t *testing.T) {
		msg := api.Message{ChatID: "c", Text: "Hi Lisa", Timestamp: newer}
		got := mergeSearchResults([][]api.MessageSearchResult{
			{{Message: msg}},
			{{Message: msg}},
		}, 10)
		if len(got) != 1 {
			t.Fatalf("got %d, want 1", len(got))
		}
	})
	t.Run("empty sets", func(t *testing.T) {
		if got := mergeSearchResults(nil, 10); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})
}

func TestToolSearchMessages_RehydratedFullNameFindsFirstNameHit(t *testing.T) {
	// The screenshot bug: user said "lisa", vault rehydrated the tool
	// query to "Lisa Wang", Beeper AND-search missed the first-name-only body.
	vault := redact.NewVault([]identity.Person{{Name: "Lisa Wang"}})
	token := vault.Redact("lisa")
	if !strings.HasPrefix(token, "CONTACT_") {
		t.Fatalf("redact(lisa) = %q, want a token", token)
	}
	args := vault.Rehydrate(`{"query":"` + token + `"}`)
	if args != `{"query":"Lisa Wang"}` {
		t.Fatalf("rehydrated args = %s, want full display name", args)
	}

	client, got := newSearchClient(t, map[string]string{
		"Lisa": searchHitJSON("m1", "Hi Lisa, this is tazik!!", "2026-08-18T13:29:00Z"),
	})
	text, step, err := toolSearchMessages(context.Background(), client, args)
	if err != nil {
		t.Fatalf("toolSearchMessages: %v", err)
	}
	if queriesOf(*got) != "Lisa,Wang" {
		t.Errorf("queries = %s, want Lisa then Wang", queriesOf(*got))
	}
	if step.result != "1 results" {
		t.Errorf("result = %q, want 1 results", step.result)
	}
	if !strings.Contains(text, "Hi Lisa, this is tazik!!") {
		t.Errorf("text = %q, want the first-name hit", text)
	}
}

func TestToolSearchMessages_UnionsHitsFromEachWord(t *testing.T) {
	client, got := newSearchClient(t, map[string]string{
		"Lisa": searchHitJSON("m-lisa", "Hi Lisa", "2026-08-18T13:00:00Z"),
		"Wang": searchHitJSON("m-wang", "see you Wang", "2026-08-18T14:00:00Z") + "," +
			searchHitJSON("m-lisa", "Hi Lisa", "2026-08-18T13:00:00Z"),
	})
	text, step, err := toolSearchMessages(context.Background(), client, `{"query":"Lisa Wang"}`)
	if err != nil {
		t.Fatalf("toolSearchMessages: %v", err)
	}
	if queriesOf(*got) != "Lisa,Wang" {
		t.Errorf("queries = %s", queriesOf(*got))
	}
	if step.result != "2 results" {
		t.Errorf("result = %q, want 2 unique hits", step.result)
	}
	lisa := strings.Index(text, "Hi Lisa")
	wang := strings.Index(text, "see you Wang")
	if lisa < 0 || wang < 0 || wang > lisa {
		t.Errorf("text = %q, want Wang hit before Lisa (newer first)", text)
	}
}

func TestToolSearchMessages_SingleWordIsOneRequest(t *testing.T) {
	client, got := newSearchClient(t, map[string]string{
		"lisa": searchHitJSON("m1", "hi lisa", "2026-08-18T13:29:00Z"),
	})
	_, step, err := toolSearchMessages(context.Background(), client, `{"query":"lisa"}`)
	if err != nil {
		t.Fatalf("toolSearchMessages: %v", err)
	}
	if queriesOf(*got) != "lisa" {
		t.Errorf("queries = %s, want a single lisa search", queriesOf(*got))
	}
	if step.result != "1 results" {
		t.Errorf("result = %q", step.result)
	}
}

func TestToolSearchMessages_MissStaysEmpty(t *testing.T) {
	client, _ := newSearchClient(t, nil)
	text, step, err := toolSearchMessages(context.Background(), client, `{"query":"Lisa Wang"}`)
	if err != nil {
		t.Fatalf("toolSearchMessages: %v", err)
	}
	if step.result != "0 results" || text != "no messages match" {
		t.Errorf("result = %q text = %q", step.result, text)
	}
}

func TestSearchMessages_ForwardsFiltersOnEachWord(t *testing.T) {
	client, got := newSearchClient(t, nil)
	after := time.Date(2026, 8, 18, 0, 0, 0, 0, time.Local)
	before := after.Add(24 * time.Hour)
	_, err := searchMessages(context.Background(), client, api.SearchQuery{
		Query:  "Lisa Wang",
		Sender: "me",
		ChatID: "imsg##thread:1",
		After:  after,
		Before: before,
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("searchMessages: %v", err)
	}
	if len(*got) != 2 {
		t.Fatalf("requests = %d, want 2", len(*got))
	}
	for i, req := range *got {
		if req.Get("sender") != "me" {
			t.Errorf("req %d sender = %q", i, req.Get("sender"))
		}
		if req.Get("limit") != "5" {
			t.Errorf("req %d limit = %q", i, req.Get("limit"))
		}
		if req.Get("chatIDs") != "imsg##thread:1" && !strings.Contains(req.Encode(), "imsg") {
			t.Errorf("req %d missing chat id: %s", i, req.Encode())
		}
		if req.Get("dateAfter") == "" || req.Get("dateBefore") == "" {
			t.Errorf("req %d missing dates: %s", i, req.Encode())
		}
	}
}

func TestSearchMessages_FilterOnlyIsOneRequest(t *testing.T) {
	client, got := newSearchClient(t, nil)
	_, err := searchMessages(context.Background(), client, api.SearchQuery{Sender: "me"})
	if err != nil {
		t.Fatalf("searchMessages: %v", err)
	}
	if len(*got) != 1 || (*got)[0].Get("query") != "" || (*got)[0].Get("sender") != "me" {
		t.Errorf("requests = %v, want one filter-only search", *got)
	}
}

func TestSearchMessages_WordLookupError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages/search" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[],"hasMore":false}`))
			return
		}
		if r.URL.Query().Get("query") == "Wang" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"hasMore":false}`))
	}))
	t.Cleanup(srv.Close)
	client := api.New(config.Config{Token: "test", BaseURL: srv.URL})
	_, err := searchMessages(context.Background(), client, api.SearchQuery{Query: "Lisa Wang"})
	if err == nil {
		t.Fatal("want error from the Wang lookup")
	}
}

func TestToolSearchMessages_RequiresQueryOrFilter(t *testing.T) {
	client, _ := newSearchClient(t, nil)
	_, _, err := toolSearchMessages(context.Background(), client, `{}`)
	if err == nil {
		t.Fatal("want error when query and filters are empty")
	}
}

func idsOf(in []api.MessageSearchResult) []string {
	out := make([]string, len(in))
	for i, r := range in {
		out[i] = r.Message.ID
	}
	return out
}

func queriesOf(reqs []url.Values) string {
	var words []string
	for _, req := range reqs {
		words = append(words, req.Get("query"))
	}
	return strings.Join(words, ",")
}

func searchHitJSON(id, text, ts string) string {
	return fmt.Sprintf(`{"id":%q,"accountID":"acc","chatID":"imsg","senderID":"me","sortKey":"1","text":%q,"timestamp":%q,"isSender":true,"senderName":"Me"}`, id, text, ts)
}

func newSearchClient(t *testing.T, hits map[string]string) (*api.Client, *[]url.Values) {
	t.Helper()
	var got []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/messages/search" {
			_, _ = w.Write([]byte(`{"items":[],"hasMore":false}`))
			return
		}
		got = append(got, r.URL.Query())
		items := hits[r.URL.Query().Get("query")]
		if items == "" {
			items = "[]"
		} else if !strings.HasPrefix(items, "[") {
			items = "[" + items + "]"
		}
		_, _ = w.Write([]byte(`{"items":` + items + `,"hasMore":false}`))
	}))
	t.Cleanup(srv.Close)
	return api.New(config.Config{Token: "test", BaseURL: srv.URL}), &got
}

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
