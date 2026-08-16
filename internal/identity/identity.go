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

// AccountRef locates one person on one account.
type AccountRef struct {
	AccountID string
	UserID    string
	Network   string
}

// Person is one resolvable human, possibly spanning several accounts.
type Person struct {
	Name       string
	AltNames   []string
	Usernames  []string
	Phones     []string
	Emails     []string
	Accounts   []AccountRef
	Chats      []ChatRef
	LastActive time.Time
}

// Networks lists the networks the person appears on, in account order.
func (p Person) Networks() []string {
	var out []string
	for _, a := range p.Accounts {
		if a.Network != "" && !containsString(out, a.Network) {
			out = append(out, a.Network)
		}
	}
	return out
}

// Index holds identities gathered from chat titles, chat participants, and
// contacts, ready for fuzzy lookup.
type Index struct {
	people []*Person
}

// Build gathers identities from chats and contacts into one index with a
// nil merge policy. See BuildWithPolicy for the merge rules.
func Build(chats []api.Chat, contacts []api.Contact) *Index {
	return BuildWithPolicy(chats, contacts, nil)
}

// BuildWithPolicy gathers identities and merges entries that describe the
// same human. Strong keys merge unconditionally: same account and user ID,
// same phone digits, same email. A shared full name merges within one
// account always, and across accounts only when the policy allows it: the
// name needs two or more words, and common names wait for user approval.
func BuildWithPolicy(chats []api.Chat, contacts []api.Contact, policy *MergePolicy) *Index {
	b := &builder{
		byAccount:  map[string]*Person{},
		byAcctName: map[string]*Person{},
		byName:     map[string]*Person{},
		byPhone:    map[string]*Person{},
		byEmail:    map[string]*Person{},
		parent:     map[*Person]*Person{},
		policy:     policy,
	}
	networks := map[string]string{}
	for _, c := range chats {
		networks[c.AccountID] = c.Network
		ref := ChatRef{ID: c.ID, Title: c.Title}
		for _, p := range c.Participants {
			if p.IsSelf || p.IsBot || (p.FullName == "" && p.Username == "") {
				continue
			}
			b.add(AccountRef{c.AccountID, p.UserID, c.Network}, p.FullName, p.Username, "", "", ref, c.LastActive)
		}
		if c.Type == "single" && c.Title != "" {
			b.add(AccountRef{c.AccountID, "", c.Network}, c.Title, "", "", "", ref, c.LastActive)
		}
	}
	for _, ct := range contacts {
		b.add(AccountRef{ct.AccountID, ct.UserID, networks[ct.AccountID]},
			ct.FullName, ct.Username, ct.PhoneNumber, ct.Email, ChatRef{}, time.Time{})
	}
	var people []*Person
	for _, p := range b.people {
		if b.parent[p] == nil {
			people = append(people, p)
		}
	}
	return &Index{people: people}
}

type builder struct {
	people     []*Person
	byAccount  map[string]*Person // accountID \x00 userID
	byAcctName map[string]*Person // accountID \x00 normalized name
	byName     map[string]*Person // normalized name, any account
	byPhone    map[string]*Person // digits only
	byEmail    map[string]*Person // lowercased
	parent     map[*Person]*Person
	policy     *MergePolicy
}

// find follows merge links to the surviving person.
func (b *builder) find(p *Person) *Person {
	for p != nil && b.parent[p] != nil {
		p = b.parent[p]
	}
	return p
}

// add merges one observed entry into the index under the merge rules.
func (b *builder) add(acct AccountRef, name, username, phone, email string, ref ChatRef, active time.Time) {
	accountKey := ""
	if acct.UserID != "" {
		accountKey = acct.AccountID + "\x00" + acct.UserID
	}
	acctNameKey, nameKey := "", ""
	if n := Normalize(name); n != "" {
		acctNameKey = acct.AccountID + "\x00" + n
		nameKey = n
	}
	phoneKey := digitsOnly(phone)
	emailKey := strings.ToLower(strings.TrimSpace(email))

	var dst *Person
	var extra []*Person
	consider := func(p *Person) {
		p = b.find(p)
		if p == nil {
			return
		}
		if dst == nil {
			dst = p
		} else if p != dst && !containsPerson(extra, p) {
			extra = append(extra, p)
		}
	}
	consider(b.byAccount[accountKey])
	consider(b.byAcctName[acctNameKey])
	consider(b.byPhone[phoneKey])
	consider(b.byEmail[emailKey])
	if p := b.find(b.byName[nameKey]); p != nil && p != dst && !containsPerson(extra, p) {
		// A cross-account name match is the one weak key: gate it.
		if b.policy.allowNameMerge(name, append(p.Networks(), acct.Network)) {
			consider(p)
		}
	}

	if dst == nil {
		dst = &Person{Name: name, LastActive: time.Time{}}
		b.people = append(b.people, dst)
	}
	for _, p := range extra {
		b.absorb(dst, p)
		b.parent[p] = dst
	}
	b.absorbFields(dst, name, username, phone, email, acct)
	if ref.ID != "" && !hasChat(dst.Chats, ref.ID) {
		dst.Chats = append(dst.Chats, ref)
	}
	if active.After(dst.LastActive) {
		dst.LastActive = active
	}

	for key, m := range map[string]map[string]*Person{
		accountKey:  b.byAccount,
		acctNameKey: b.byAcctName,
		phoneKey:    b.byPhone,
		emailKey:    b.byEmail,
	} {
		if key != "" {
			m[key] = dst
		}
	}
	if nameKey != "" && b.byName[nameKey] == nil {
		b.byName[nameKey] = dst
	}
}

// absorb folds src's identity into dst after a merge.
func (b *builder) absorb(dst, src *Person) {
	b.absorbFields(dst, src.Name, "", "", "", AccountRef{})
	for _, n := range src.AltNames {
		b.absorbFields(dst, n, "", "", "", AccountRef{})
	}
	dst.Usernames = appendUnique(dst.Usernames, src.Usernames...)
	dst.Phones = appendUnique(dst.Phones, src.Phones...)
	dst.Emails = appendUnique(dst.Emails, src.Emails...)
	for _, a := range src.Accounts {
		addAccount(dst, a)
	}
	for _, ref := range src.Chats {
		if !hasChat(dst.Chats, ref.ID) {
			dst.Chats = append(dst.Chats, ref)
		}
	}
	if src.LastActive.After(dst.LastActive) {
		dst.LastActive = src.LastActive
	}
}

// absorbFields folds one observation's fields into dst.
func (b *builder) absorbFields(dst *Person, name, username, phone, email string, acct AccountRef) {
	switch {
	case dst.Name == "":
		dst.Name = name
	case name != "" && Normalize(name) != Normalize(dst.Name) && !containsNormalized(dst.AltNames, name):
		dst.AltNames = append(dst.AltNames, name)
	}
	if username != "" {
		dst.Usernames = appendUnique(dst.Usernames, username)
	}
	if phone != "" {
		dst.Phones = appendUnique(dst.Phones, phone)
	}
	if email != "" {
		dst.Emails = appendUnique(dst.Emails, email)
	}
	if acct.AccountID != "" {
		addAccount(dst, acct)
	}
}

// addAccount records acct on dst, upgrading a userless entry from the same
// account instead of duplicating it.
func addAccount(dst *Person, acct AccountRef) {
	for i, a := range dst.Accounts {
		if a.AccountID != acct.AccountID {
			continue
		}
		if a.UserID == acct.UserID {
			return
		}
		if a.UserID == "" {
			dst.Accounts[i].UserID = acct.UserID
			return
		}
		if acct.UserID == "" {
			return
		}
	}
	dst.Accounts = append(dst.Accounts, acct)
}

func appendUnique(dst []string, values ...string) []string {
	for _, v := range values {
		if v == "" {
			continue
		}
		found := false
		for _, have := range dst {
			if strings.EqualFold(have, v) {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, v)
		}
	}
	return dst
}

func containsNormalized(list []string, v string) bool {
	n := Normalize(v)
	for _, have := range list {
		if Normalize(have) == n {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, have := range list {
		if have == v {
			return true
		}
	}
	return false
}

func containsPerson(list []*Person, p *Person) bool {
	for _, have := range list {
		if have == p {
			return true
		}
	}
	return false
}

// digitsOnly reduces a phone number to its digits for matching.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
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
		for _, field := range p.matchFields() {
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

// matchFields lists every string a search query may hit.
func (p *Person) matchFields() []string {
	fields := []string{p.Name}
	fields = append(fields, p.AltNames...)
	fields = append(fields, p.Usernames...)
	fields = append(fields, p.Phones...)
	fields = append(fields, p.Emails...)
	return fields
}

// Candidates returns people whose full name appears as a word-bounded span
// of query. It rescues queries that stitch several names together, where
// Search finds nothing because no single person matches every word.
func (ix *Index) Candidates(query string) []Person {
	q := " " + strippedWords(query) + " "
	var out []Person
	for _, p := range ix.people {
		for _, name := range append([]string{p.Name}, p.AltNames...) {
			n := strippedWords(name)
			if n != "" && strings.Contains(q, " "+n+" ") {
				out = append(out, *p)
				break
			}
		}
	}
	return out
}

// strippedWords normalizes s and reduces punctuation to single spaces.
func strippedWords(s string) string {
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, Normalize(s))
	return strings.Join(strings.Fields(mapped), " ")
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
