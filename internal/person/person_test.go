package person

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/llm"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCardRoundTrip(t *testing.T) {
	s := testStore(t)
	in := Card{
		Name: "Dana Fixture", Birthday: "1998-03-14", City: "Toronto",
		Country: "Canada", Likes: []string{"climbing", "jazz"},
		Body: "met at the co-op\nprefers tea",
	}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("Dana Fixture")
	if err != nil {
		t.Fatal(err)
	}
	if got.Birthday != in.Birthday || got.City != in.City || got.Country != in.Country {
		t.Errorf("fields = %+v, want %+v", got, in)
	}
	if len(got.Likes) != 2 || got.Likes[1] != "jazz" {
		t.Errorf("likes = %v", got.Likes)
	}
	if got.Body != in.Body {
		t.Errorf("body = %q, want %q", got.Body, in.Body)
	}
}

func TestLoadMissingCard(t *testing.T) {
	s := testStore(t)
	got, err := s.Load("Nobody Yet")
	if err != nil || got.Name != "Nobody Yet" || got.City != "" {
		t.Fatalf("missing card = %+v, %v", got, err)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Dana Fixture": "dana-fixture",
		"  Bob  ":      "bob",
		"Renée C.":     "rene-c",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMergeFillsOnlyEmpty(t *testing.T) {
	c := Card{Name: "Dana", City: "Toronto", Likes: []string{"jazz"}}
	changed, prov := Merge(&c, Found{
		City:     Fact{Value: "Vancouver", Quote: "i moved"},
		Country:  Fact{Value: "Canada", Quote: "in canada"},
		Likes:    []Fact{{Value: "Jazz", Quote: "love jazz"}, {Value: "climbing", Quote: "gym tonight"}},
		Birthday: Fact{},
	})
	if c.City != "Toronto" {
		t.Errorf("hand-set city overwritten: %q", c.City)
	}
	if c.Country != "Canada" {
		t.Errorf("country = %q", c.Country)
	}
	if len(c.Likes) != 2 || c.Likes[1] != "climbing" {
		t.Errorf("likes = %v", c.Likes)
	}
	want := []string{"country", "likes+climbing"}
	if strings.Join(changed, ",") != strings.Join(want, ",") {
		t.Errorf("changed = %v, want %v", changed, want)
	}
	if prov["country"] != "in canada" {
		t.Errorf("prov = %v", prov)
	}
}

func TestProvenanceAndRender(t *testing.T) {
	s := testStore(t)
	c := Card{Name: "Dana", City: "Toronto", Likes: []string{"climbing"}}
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordProvenance("Dana", map[string]string{"city": "im in toronto now", "like:climbing": "gym tonight"}); err != nil {
		t.Fatal(err)
	}
	out := s.Render(c)
	if !strings.Contains(out, `city      Toronto  (extracted: "im in toronto now")`) {
		t.Errorf("render missing extraction marker:\n%s", out)
	}
	if !strings.Contains(out, "climbing") {
		t.Errorf("render missing like:\n%s", out)
	}
}

func TestExtract(t *testing.T) {
	reply := Found{
		City:  Fact{Value: "Toronto", Quote: "i live in toronto"},
		Likes: []Fact{{Value: "climbing", Quote: "climbing gym tonight?"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ResponseFormat map[string]any `json:"response_format"`
			Messages       []llm.Message  `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.ResponseFormat["type"] != "json_schema" {
			t.Errorf("response_format = %v", req.ResponseFormat)
		}
		if !strings.Contains(req.Messages[1].Content, "climbing gym tonight?") {
			t.Errorf("transcript missing message: %s", req.Messages[1].Content)
		}
		content, _ := json.Marshal(reply)
		resp := map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": string(content)}}}}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	lc := llm.New(srv.URL, "test-model")
	got, err := Extract(context.Background(), lc, "Dana", []api.Message{
		{SenderName: "Dana", Text: "climbing gym tonight?", Timestamp: time.Now()},
		{SenderName: "Dana", Text: "i live in toronto", Timestamp: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.City.Value != "Toronto" || len(got.Likes) != 1 {
		t.Errorf("extract = %+v", got)
	}
}
