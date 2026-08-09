// Package identity resolves people by name across chats and contacts.
// Matching happens locally over plaintext the device already holds, so no
// query or name ever needs to leave the machine to find someone.
package identity

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/taziksh/beeper-tui/internal/api"
)

// ChatRef points a person at one chat they appear in.
type ChatRef struct {
	ID    string
	Title string
}

// Person is one resolvable identity and the chats it appears in.
type Person struct {
	AccountID  string
	UserID     string
	Name       string
	Username   string
	Phone      string
	Email      string
	Network    string
	Chats      []ChatRef
	LastActive time.Time
}

// Index holds identities gathered from chat titles, chat participants, and
// contacts, ready for fuzzy lookup.
type Index struct {
	people []*Person
}

// Build gathers identities from chats and contacts into one index. Entries
// that describe the same person on the same account are merged, keyed by user
// ID when known and by normalized name otherwise.
func Build(chats []api.Chat, contacts []api.Contact) *Index {
	b := &builder{
		byUser: map[string]*Person{},
		byName: map[string]*Person{},
	}
	networks := map[string]string{}
	for _, c := range chats {
		networks[c.AccountID] = c.Network
		ref := ChatRef{ID: c.ID, Title: c.Title}
		for _, p := range c.Participants {
			if p.IsSelf || p.IsBot || (p.FullName == "" && p.Username == "") {
				continue
			}
			b.add(&Person{
				AccountID: c.AccountID,
				UserID:    p.UserID,
				Name:      p.FullName,
				Username:  p.Username,
				Network:   c.Network,
			}, ref, c.LastActive)
		}
		if c.Type == "single" && c.Title != "" {
			b.add(&Person{
				AccountID: c.AccountID,
				Name:      c.Title,
				Network:   c.Network,
			}, ref, c.LastActive)
		}
	}
	for _, ct := range contacts {
		b.add(&Person{
			AccountID: ct.AccountID,
			UserID:    ct.UserID,
			Name:      ct.FullName,
			Username:  ct.Username,
			Phone:     ct.PhoneNumber,
			Email:     ct.Email,
			Network:   networks[ct.AccountID],
		}, ChatRef{}, time.Time{})
	}
	return &Index{people: b.people}
}

type builder struct {
	people []*Person
	byUser map[string]*Person
	byName map[string]*Person
}

// add merges p into the index, preferring an existing entry with the same
// user ID, then one with the same normalized name on the same account.
func (b *builder) add(p *Person, ref ChatRef, active time.Time) {
	userKey := ""
	if p.UserID != "" {
		userKey = p.AccountID + "\x00" + p.UserID
	}
	nameKey := ""
	if p.Name != "" {
		nameKey = p.AccountID + "\x00" + Normalize(p.Name)
	}
	dst := b.byUser[userKey]
	if dst == nil && nameKey != "" {
		dst = b.byName[nameKey]
	}
	if dst == nil {
		dst = p
		b.people = append(b.people, dst)
	} else {
		fillIfEmpty(&dst.UserID, p.UserID)
		fillIfEmpty(&dst.Name, p.Name)
		fillIfEmpty(&dst.Username, p.Username)
		fillIfEmpty(&dst.Phone, p.Phone)
		fillIfEmpty(&dst.Email, p.Email)
		fillIfEmpty(&dst.Network, p.Network)
	}
	if ref.ID != "" && !hasChat(dst.Chats, ref.ID) {
		dst.Chats = append(dst.Chats, ref)
	}
	if active.After(dst.LastActive) {
		dst.LastActive = active
	}
	if userKey != "" {
		b.byUser[userKey] = dst
	}
	if nameKey != "" {
		b.byName[nameKey] = dst
	}
}

func fillIfEmpty(dst *string, v string) {
	if *dst == "" {
		*dst = v
	}
}

func hasChat(refs []ChatRef, id string) bool {
	for _, r := range refs {
		if r.ID == id {
			return true
		}
	}
	return false
}

// Match tiers, best first.
const (
	tierExact = iota
	tierPrefix
	tierSubsequence
	tierNone
)

// Search returns up to max people matching query, best match first. A person
// matches when the query equals, prefixes, token-prefixes, or is an in-order
// subsequence of any of their name, handle, phone, or email. Ties break
// toward recently active chats.
func (ix *Index) Search(query string, max int) []Person {
	q := Normalize(query)
	if q == "" {
		return nil
	}
	type scored struct {
		p    *Person
		tier int
		dist int
	}
	var hits []scored
	for _, p := range ix.people {
		tier, dist := tierNone, 0
		for _, field := range []string{p.Name, p.Username, p.Phone, p.Email} {
			t, d := matchField(q, Normalize(field))
			if t < tier || (t == tier && d < dist) {
				tier, dist = t, d
			}
		}
		if tier < tierNone {
			hits = append(hits, scored{p, tier, dist})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].tier != hits[j].tier {
			return hits[i].tier < hits[j].tier
		}
		if hits[i].dist != hits[j].dist {
			return hits[i].dist < hits[j].dist
		}
		return hits[i].p.LastActive.After(hits[j].p.LastActive)
	})
	out := make([]Person, 0, max)
	for _, h := range hits {
		if len(out) >= max {
			break
		}
		out = append(out, *h.p)
	}
	return out
}

// All returns every person in the index.
func (ix *Index) All() []Person {
	out := make([]Person, 0, len(ix.people))
	for _, p := range ix.people {
		out = append(out, *p)
	}
	return out
}

// MatchStrings returns the indices of candidates matching query, best match
// first, using the same tiers as Search.
func MatchStrings(query string, candidates []string) []int {
	q := Normalize(query)
	if q == "" {
		return nil
	}
	type scored struct {
		idx  int
		tier int
		dist int
	}
	var hits []scored
	for i, c := range candidates {
		tier, dist := matchField(q, Normalize(c))
		if tier < tierNone {
			hits = append(hits, scored{i, tier, dist})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].tier != hits[j].tier {
			return hits[i].tier < hits[j].tier
		}
		return hits[i].dist < hits[j].dist
	})
	out := make([]int, len(hits))
	for i, h := range hits {
		out[i] = h.idx
	}
	return out
}

// matchField classifies how well normalized query q matches normalized field
// f, returning the tier and an edit distance used to rank within the tier.
func matchField(q, f string) (int, int) {
	if f == "" {
		return tierNone, 0
	}
	switch {
	case f == q:
		return tierExact, 0
	case strings.HasPrefix(f, q) || tokenPrefixes(q, f):
		return tierPrefix, fuzzy.LevenshteinDistance(q, f)
	case fuzzy.MatchNormalizedFold(q, f):
		return tierSubsequence, fuzzy.LevenshteinDistance(q, f)
	}
	return tierNone, 0
}

// tokenPrefixes reports whether every space-separated token of q prefixes
// some token of f, so "ali ahm" matches "alice ahmed" in any order.
func tokenPrefixes(q, f string) bool {
	qt := strings.Fields(q)
	ft := strings.Fields(f)
	if len(qt) == 0 {
		return false
	}
	for _, query := range qt {
		ok := false
		for _, field := range ft {
			if strings.HasPrefix(field, query) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// stripMarks removes combining marks so accented and plain letters compare
// equal.
var stripMarks = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Normalize lowercases s and strips diacritics for matching.
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out, _, err := transform.String(stripMarks, s)
	if err != nil {
		return s
	}
	return out
}
