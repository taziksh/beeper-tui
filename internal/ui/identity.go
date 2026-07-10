package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
)

// Identity field focus.
const (
	idFocusName  = 0
	idFocusNotes = 1
)

// openIdentityCard opens the person card for the current conversation chat
// (or the selected list chat). Creates a draft for a new identity when none
// is linked yet. Always links the chat (and peer user when known) on save.
func (m Model) openIdentityCard() Model {
	if m.idents == nil {
		return m
	}
	chat, ok := m.identityTargetChat()
	if !ok {
		return m
	}
	m.idReturnMode = m.mode
	if m.idReturnMode != ModeList && m.idReturnMode != ModeConversation {
		m.idReturnMode = ModeConversation
	}
	m.idChatID = chat.ID
	m.idAccountID = chat.AccountID
	m.idPeerUserID = chat.PeerUserID
	m.idNetwork = chat.Network
	m.idChatTitle = chat.Title
	m.idErr = nil
	m.idFocus = idFocusName

	if idn, found := m.idents.ResolveForChat(chat.ID, chat.AccountID, chat.PeerUserID); found {
		m.idID = idn.ID
		m.idName = idn.DisplayName
		m.idNotes = idn.Notes
	} else {
		m.idID = ""
		m.idName = chat.Title
		m.idNotes = ""
	}
	m.mode = ModeIdentity
	return m
}

func (m Model) identityTargetChat() (api.Chat, bool) {
	switch m.mode {
	case ModeConversation, ModeInsert, ModeReact:
		if idx := chatIndexByID(m.chats, m.currentChatID); idx >= 0 {
			return m.chats[idx], true
		}
	case ModeList:
		if m.selected >= 0 && m.selected < len(m.chats) {
			return m.chats[m.selected], true
		}
	}
	return api.Chat{}, false
}

// handleIdentityKey processes keys in IDENTITY mode.
// tab switches name/notes; enter inserts a newline in notes (saves on name);
// ctrl+s always saves; esc cancels; ctrl+d deletes an existing card.
func (m Model) handleIdentityKey(key, text string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m.closeIdentityCard(false), nil
	case "tab":
		if m.idFocus == idFocusName {
			m.idFocus = idFocusNotes
		} else {
			m.idFocus = idFocusName
		}
		return m, nil
	case "shift+tab":
		if m.idFocus == idFocusNotes {
			m.idFocus = idFocusName
		} else {
			m.idFocus = idFocusNotes
		}
		return m, nil
	case "ctrl+s":
		return m.saveIdentityCard()
	case "enter":
		if m.idFocus == idFocusName {
			return m.saveIdentityCard()
		}
		m.idNotes += "\n"
		return m, nil
	case "ctrl+d":
		return m.deleteIdentityCard()
	case "backspace":
		if m.idFocus == idFocusName {
			m.idName = backspaceRunes(m.idName)
		} else {
			m.idNotes = backspaceRunes(m.idNotes)
		}
		return m, nil
	default:
		if text == "" {
			return m, nil
		}
		if m.idFocus == idFocusName {
			// Names stay single-line.
			if strings.ContainsAny(text, "\n\r") {
				return m, nil
			}
			m.idName += text
		} else {
			m.idNotes += text
		}
		return m, nil
	}
}

func backspaceRunes(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func (m Model) closeIdentityCard(saved bool) Model {
	ret := m.idReturnMode
	if ret != ModeList && ret != ModeConversation {
		ret = ModeList
	}
	m.mode = ret
	m.idErr = nil
	if !saved {
		m.idID = ""
		m.idName = ""
		m.idNotes = ""
	}
	return m
}

func (m Model) saveIdentityCard() (Model, tea.Cmd) {
	if m.idents == nil {
		m.idErr = identity.ErrEmptyName
		return m, nil
	}
	name := strings.TrimSpace(m.idName)
	if name == "" {
		m.idErr = identity.ErrEmptyName
		return m, nil
	}
	var idn identity.Identity
	var err error
	if m.idID == "" {
		idn, err = m.idents.Create(name, m.idNotes)
		if err != nil {
			m.idErr = err
			return m, nil
		}
		m.idID = idn.ID
	} else {
		idn, err = m.idents.Update(m.idID, name, m.idNotes)
		if err != nil {
			m.idErr = err
			return m, nil
		}
	}
	// Manual link to the chat that opened the card.
	if m.idChatID != "" {
		if err := m.idents.LinkChat(m.idID, m.idChatID); err != nil && err != identity.ErrLinkTaken {
			m.idErr = err
			return m, nil
		}
	}
	if m.idAccountID != "" && m.idPeerUserID != "" {
		if err := m.idents.LinkUser(m.idID, m.idAccountID, m.idPeerUserID); err != nil && err != identity.ErrLinkTaken {
			m.idErr = err
			return m, nil
		}
	}
	_ = idn
	return m.closeIdentityCard(true), nil
}

func (m Model) deleteIdentityCard() (Model, tea.Cmd) {
	if m.idents == nil || m.idID == "" {
		return m.closeIdentityCard(false), nil
	}
	if err := m.idents.Delete(m.idID); err != nil {
		m.idErr = err
		return m, nil
	}
	return m.closeIdentityCard(true), nil
}

func (m Model) renderIdentity() string {
	var b strings.Builder
	title := m.idChatTitle
	if title == "" {
		title = "person"
	}
	net := m.idNetwork
	if net != "" {
		b.WriteString("IDENTITY  " + title + "  (" + net + ")\n\n")
	} else {
		b.WriteString("IDENTITY  " + title + "\n\n")
	}

	nameLine := "  Name:  " + m.idName
	notesHeader := "  Notes:"
	if m.idFocus == idFocusName {
		nameLine += "█"
		b.WriteString("> " + strings.TrimPrefix(nameLine, "  ") + "\n")
		b.WriteString(notesHeader + "\n")
	} else {
		b.WriteString(nameLine + "\n")
		b.WriteString("> " + strings.TrimPrefix(notesHeader, "  ") + "\n")
	}

	notes := m.idNotes
	if m.idFocus == idFocusNotes {
		notes += "█"
	}
	if notes == "" && m.idFocus != idFocusNotes {
		b.WriteString("    (empty)\n")
	} else {
		for _, line := range strings.Split(notes, "\n") {
			b.WriteString("    " + line + "\n")
		}
	}

	b.WriteString("\n")
	if m.idErr != nil {
		b.WriteString("  error: " + m.idErr.Error() + "\n")
	}
	b.WriteString(m.identityStatusBar())
	return b.String()
}

func (m Model) identityStatusBar() string {
	base := "IDENTITY  tab field · enter save/newline · ctrl+s save · ctrl+d delete · esc cancel"
	if m.idID == "" {
		base = "IDENTITY  new card · tab field · enter save/newline · ctrl+s save · esc cancel"
	}
	if m.idErr != nil {
		return base + " · " + m.idErr.Error()
	}
	return base
}
