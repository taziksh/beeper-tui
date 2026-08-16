// Package redact keeps known identities out of text that leaves the
// machine. A session vault mints random per-session tokens for every
// identity string the local index knows; outbound text is rewritten to
// tokens and model output is rehydrated back to real names for display.
// Tokens are random, never derived from the identity, so they carry nothing
// and link nothing across sessions.
package redact

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
)

// SessionVault fetches known chats and contacts and registers every
// identity for this session. Best-effort: on fetch errors the vault
// simply covers less.
func SessionVault(ctx context.Context, client *api.Client) *Vault {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	chats, _ := client.ListChats(ctx)
	contacts, _ := client.ListContacts(ctx)
	return NewVault(identity.Build(chats, contacts).All())
}

// Vault holds one session's identity-to-token mapping. Issuance is strict:
// only strings registered from the identity index get tokens, and only
// tokens the vault issued ever rehydrate.
type Vault struct {
	mu       sync.Mutex
	byValue  map[string]string // normalized identity string -> token
	byToken  map[string]string // token -> display string
	replacer []replacement     // longest-first scan order
}

type replacement struct {
	value string // original form as it appears in text
	token string
}

// tokenPattern matches issued tokens tolerantly: models re-emit CONTACT_3
// as "Contact 3" or "CONTACT-3". Issuance always uses CONTACT_N.
var tokenPattern = regexp.MustCompile(`(?i)\bCONTACT[ _-](\d+)\b`)

// NewVault builds a session vault from the identity index, expanding each
// person into the variants that appear in real text: full name, name
// tokens, possessives, handle, phone, and email.
func NewVault(people []identity.Person) *Vault {
	v := &Vault{
		byValue: map[string]string{},
		byToken: map[string]string{},
	}
	for _, p := range people {
		token := v.mint(p.Name)
		v.register(p.Name, token)
		// The possessive keeps its suffix on the token, so "Dana's plan"
		// reads "CONTACT_N's plan" and grammar survives redaction.
		v.register(p.Name+"'s", token+"'s")
		for _, part := range strings.Fields(p.Name) {
			if len(part) >= 3 {
				v.register(part, token)
				v.register(part+"'s", token+"'s")
			}
		}
		v.register(p.Username, token)
		v.register(p.Phone, token)
		v.register(p.Email, token)
	}
	sort.SliceStable(v.replacer, func(i, j int) bool {
		return len(v.replacer[i].value) > len(v.replacer[j].value)
	})
	return v
}

// mint issues a fresh random token for one person and records the display
// name it rehydrates to.
func (v *Vault) mint(display string) string {
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(100000))
		if err != nil {
			// crypto/rand failing is unrecoverable; fall back to counting,
			// which is still unlinkable across sessions per re-mint order.
			n = big.NewInt(int64(len(v.byToken) + 1))
		}
		token := fmt.Sprintf("CONTACT_%d", n.Int64())
		if _, taken := v.byToken[token]; taken {
			continue
		}
		v.byToken[token] = display
		return token
	}
}

// register maps one identity string to a token. Empty and short strings
// are skipped: replacing them would mangle ordinary text.
func (v *Vault) register(value, token string) {
	value = strings.TrimSpace(value)
	if len(value) < 3 {
		return
	}
	key := strings.ToLower(value)
	if _, dup := v.byValue[key]; dup {
		return
	}
	v.byValue[key] = token
	v.replacer = append(v.replacer, replacement{value: value, token: token})
}

// Redact replaces every registered identity string in text with its token,
// longest match first, case-insensitively at word boundaries.
func (v *Vault) Redact(text string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, r := range v.replacer {
		re, err := regexp.Compile(valuePattern(r.value))
		if err != nil {
			continue
		}
		text = re.ReplaceAllString(text, r.token)
	}
	return text
}

// valuePattern builds the match for one identity string. Word boundaries
// only exist next to word characters, so values like "+1555..." and
// "@dana" get them conditionally.
func valuePattern(value string) string {
	pat := "(?i)"
	if isWordByte(value[0]) {
		pat += `\b`
	}
	pat += regexp.QuoteMeta(value)
	if isWordByte(value[len(value)-1]) {
		pat += `\b`
	}
	return pat
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}

// Rehydrate replaces issued tokens in model output with display names.
// Tokens the vault never issued pass through untouched: resolving invented
// tokens would let the model fabricate attributions.
func (v *Vault) Rehydrate(text string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return tokenPattern.ReplaceAllStringFunc(text, func(match string) string {
		sub := tokenPattern.FindStringSubmatch(match)
		if display, ok := v.byToken["CONTACT_"+sub[1]]; ok {
			return display
		}
		return match
	})
}

// HoldBack lets a vault satisfy interfaces that need the package function.
func (v *Vault) HoldBack(text string) (emit, hold string) {
	return HoldBack(text)
}

// HoldBack splits streamed text into a safe-to-emit head and a tail that
// must wait for the next chunk because it could be the start of a token.
func HoldBack(text string) (emit, hold string) {
	for i := len(text) - 1; i >= 0 && len(text)-i <= 14; i-- {
		if tokenPrefix(text[i:]) {
			return text[:i], text[i:]
		}
	}
	return text, ""
}

// tokenPrefix reports whether s could grow into a token match.
func tokenPrefix(s string) bool {
	const word = "CONTACT"
	up := strings.ToUpper(s)
	for i := 0; i < len(up) && i < len(word); i++ {
		if up[i] != word[i] {
			return false
		}
	}
	if len(up) <= len(word) {
		return true
	}
	rest := up[len(word):]
	if rest[0] != ' ' && rest[0] != '_' && rest[0] != '-' {
		return false
	}
	for _, r := range rest[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
