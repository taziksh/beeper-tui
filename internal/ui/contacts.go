package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
)

type contactsLoadedMsg struct{ contacts []api.Contact }

// loadContactsCmd fetches contacts silently; a failed fetch changes
// nothing, like the chat poll.
func (m Model) loadContactsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		contacts, err := client.ListContacts(ctx)
		if err != nil {
			return nil
		}
		return contactsLoadedMsg{contacts: contacts}
	}
}

// applyContactsLoaded refreshes contact-derived state: number-titled chats
// pick up saved names, and the redaction vault learns identities that
// appeared or were renamed after startup.
func (m Model) applyContactsLoaded(contacts []api.Contact) Model {
	m.contacts = contacts
	m.chats = resolveChatTitles(m.chats, contacts)
	if m.vault != nil {
		m.vault.Update(identity.BuildWithPolicy(m.chats, contacts, m.merges).All())
	}
	return m
}

// resolveChatTitles swaps number-like single-chat titles for the matching
// contact's saved name, the way Beeper Desktop displays them.
func resolveChatTitles(chats []api.Chat, contacts []api.Contact) []api.Chat {
	if len(contacts) == 0 {
		return chats
	}
	byHandle := map[string]string{}
	type numbered struct{ digits, name string }
	var numbers []numbered
	for _, ct := range contacts {
		if ct.FullName == "" || identity.IsHandleLike(ct.FullName) {
			continue
		}
		if d := identity.DigitsOnly(ct.PhoneNumber); len(d) >= 7 {
			numbers = append(numbers, numbered{d, ct.FullName})
		}
		for _, h := range []string{ct.Email, ct.Username} {
			if h != "" {
				byHandle[strings.ToLower(h)] = ct.FullName
			}
		}
	}
	out := make([]api.Chat, len(chats))
	for i, c := range chats {
		out[i] = c
		if c.Type != "single" || !identity.IsHandleLike(c.Title) {
			continue
		}
		if name, ok := byHandle[strings.ToLower(c.Title)]; ok {
			out[i].Title = name
			continue
		}
		d := identity.DigitsOnly(c.Title)
		if len(d) < 7 {
			continue
		}
		for _, n := range numbers {
			if sameNumber(d, n.digits) {
				out[i].Title = n.name
				break
			}
		}
	}
	return out
}

// sameNumber compares phone digits, tolerating one side carrying a country
// code the other omits.
func sameNumber(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) < 10 || len(b) < 10 {
		return false
	}
	return strings.HasSuffix(a, b) || strings.HasSuffix(b, a)
}
