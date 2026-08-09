package person

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/llm"
)

// Fact is one extracted value with the message text that supports it.
type Fact struct {
	Value string `json:"value"`
	Quote string `json:"quote"`
}

// Found is what one extraction pass claims about a person. Empty values mean
// the messages did not say.
type Found struct {
	Birthday Fact   `json:"birthday"`
	City     Fact   `json:"city"`
	Country  Fact   `json:"country"`
	Likes    []Fact `json:"likes"`
}

// factSchema constrains one Fact in the extraction schema.
var factSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"value", "quote"},
	"properties": map[string]any{
		"value": map[string]any{"type": "string"},
		"quote": map[string]any{"type": "string"},
	},
}

// extractSchema is the JSON schema the model's reply must match.
var extractSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"birthday", "city", "country", "likes"},
	"properties": map[string]any{
		"birthday": factSchema,
		"city":     factSchema,
		"country":  factSchema,
		"likes":    map[string]any{"type": "array", "items": factSchema},
	},
}

// Extract asks the local model to pull card facts about name from msgs.
// Only facts the messages explicitly support come back; everything else is
// empty.
func Extract(ctx context.Context, lc *llm.Client, name string, msgs []api.Message) (Found, error) {
	var b strings.Builder
	for _, m := range msgs {
		if m.IsReaction || m.Text == "" {
			continue
		}
		sender := m.SenderName
		if m.IsFromMe {
			sender = "me"
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", m.Timestamp.Format("2006-01-02"), sender, m.Text)
	}
	sys := fmt.Sprintf(`Extract facts about %s from the chat transcript.
Only record what the messages explicitly state about %s: their birthday (YYYY-MM-DD or MM-DD), the city they live in, the country they live in, and things they like.
For each fact, quote the single message that states it, verbatim, in "quote".
If the messages do not state a fact, leave both value and quote empty. Never guess.`, name, name)
	out, err := lc.Complete(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: b.String()},
	}, "person_facts", extractSchema)
	if err != nil {
		return Found{}, err
	}
	var f Found
	if err := json.Unmarshal([]byte(out), &f); err != nil {
		return Found{}, fmt.Errorf("person: decode extraction: %w", err)
	}
	return f, nil
}

// Merge fills the card's empty fields from f and unions likes. Values a
// human (or earlier pass) already set are never changed. It returns the
// field names that changed and the provenance for each.
func Merge(c *Card, f Found) (changed []string, prov map[string]string) {
	prov = map[string]string{}
	set := func(dst *string, field string, fact Fact) {
		v := strings.TrimSpace(fact.Value)
		if *dst != "" || v == "" {
			return
		}
		*dst = v
		changed = append(changed, field)
		prov[field] = fact.Quote
	}
	set(&c.Birthday, "birthday", f.Birthday)
	set(&c.City, "city", f.City)
	set(&c.Country, "country", f.Country)
	have := map[string]bool{}
	for _, l := range c.Likes {
		have[strings.ToLower(strings.TrimSpace(l))] = true
	}
	for _, fact := range f.Likes {
		v := strings.TrimSpace(fact.Value)
		if v == "" || have[strings.ToLower(v)] {
			continue
		}
		have[strings.ToLower(v)] = true
		c.Likes = append(c.Likes, v)
		changed = append(changed, "likes+"+v)
		prov["like:"+strings.ToLower(v)] = fact.Quote
	}
	return changed, prov
}

// provenanceFile is the machine-owned sidecar recording which card values
// extraction wrote and the quote behind each. It is not part of the card.
const provenanceFile = ".extracted.json"

type provRecord struct {
	Quote string    `json:"quote"`
	At    time.Time `json:"at"`
}

// RecordProvenance merges prov for one person into the sidecar.
func (s *Store) RecordProvenance(name string, prov map[string]string) error {
	if len(prov) == 0 {
		return nil
	}
	path := filepath.Join(s.dir, provenanceFile)
	all := map[string]map[string]provRecord{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	slug := Slug(name)
	if all[slug] == nil {
		all[slug] = map[string]provRecord{}
	}
	for field, quote := range prov {
		all[slug][field] = provRecord{Quote: quote, At: time.Now()}
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Provenance returns the extraction records for one person, keyed by field.
func (s *Store) Provenance(name string) map[string]provRecord {
	path := filepath.Join(s.dir, provenanceFile)
	all := map[string]map[string]provRecord{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	return all[Slug(name)]
}

// Render formats a card for display, marking extracted values with the
// quote that supports them.
func (s *Store) Render(c Card) string {
	prov := s.Provenance(c.Name)
	var b strings.Builder
	b.WriteString(c.Name + "\n")
	row := func(field, label, val string) {
		if val == "" {
			return
		}
		line := fmt.Sprintf("%-9s %s", label, val)
		if p, ok := prov[field]; ok && p.Quote != "" {
			line += fmt.Sprintf("  (extracted: %q)", p.Quote)
		}
		b.WriteString(line + "\n")
	}
	row("birthday", "birthday", c.Birthday)
	row("city", "city", c.City)
	row("country", "country", c.Country)
	for _, l := range c.Likes {
		row("like:"+strings.ToLower(l), "likes", l)
	}
	if c.Body != "" {
		b.WriteString("\n" + c.Body + "\n")
	}
	return b.String()
}
