// Package identity stores local notes-first person cards linked to Beeper
// profiles/chats. Data is private and local-only (see design spec).
package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion bumps when the on-disk format changes incompatibly.
const SchemaVersion = 1

// FileName is the identities file under the app config directory.
const FileName = "identities.json"

// Link kinds.
const (
	LinkUser = "user"
	LinkChat = "chat"
)

var (
	ErrNotFound   = errors.New("identity: not found")
	ErrLinkTaken  = errors.New("identity: link already belongs to another identity")
	ErrEmptyName  = errors.New("identity: display name is required")
	ErrBadLink    = errors.New("identity: invalid link")
)

// Link associates a Beeper profile or chat with an identity.
type Link struct {
	Kind      string    `json:"kind"`
	AccountID string    `json:"account_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	ChatID    string    `json:"chat_id,omitempty"`
	LinkedAt  time.Time `json:"linked_at"`
}

// Identity is a local person card: a name you use and freeform notes.
type Identity struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Notes       string    `json:"notes"`
	Links       []Link    `json:"links"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type fileData struct {
	SchemaVersion int        `json:"schema_version"`
	Identities    []Identity `json:"identities"`
}

// Store is an in-memory identity book backed by a JSON file.
type Store struct {
	path string
	data fileData
}

// Path returns the durable file path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load reads identities from path. A missing file yields an empty store.
func Load(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: fileData{SchemaVersion: SchemaVersion},
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("identity: read: %w", err)
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("identity: parse: %w", err)
	}
	if data.SchemaVersion != 0 && data.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("identity: unsupported schema version %d (want %d)",
			data.SchemaVersion, SchemaVersion)
	}
	data.SchemaVersion = SchemaVersion
	if data.Identities == nil {
		data.Identities = []Identity{}
	}
	s.data = data
	return s, nil
}

// Save writes the store to disk with mode 0600.
func (s *Store) Save() error {
	if s == nil || s.path == "" {
		return errors.New("identity: no path")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("identity: mkdir: %w", err)
	}
	s.data.SchemaVersion = SchemaVersion
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("identity: marshal: %w", err)
	}
	raw = append(raw, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("identity: write: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("identity: rename: %w", err)
	}
	// Rename preserves mode on some systems; re-chmod for the final path.
	_ = os.Chmod(s.path, 0o600)
	return nil
}

// List returns a copy of all identities.
func (s *Store) List() []Identity {
	if s == nil {
		return nil
	}
	out := make([]Identity, len(s.data.Identities))
	copy(out, s.data.Identities)
	return out
}

// Get returns an identity by id.
func (s *Store) Get(id string) (Identity, bool) {
	if s == nil {
		return Identity{}, false
	}
	for _, idn := range s.data.Identities {
		if idn.ID == id {
			return idn, true
		}
	}
	return Identity{}, false
}

// Create adds a new identity and persists it.
func (s *Store) Create(displayName, notes string) (Identity, error) {
	if s == nil {
		return Identity{}, errors.New("identity: nil store")
	}
	name := trimName(displayName)
	if name == "" {
		return Identity{}, ErrEmptyName
	}
	now := time.Now().UTC()
	idn := Identity{
		ID:          newID(),
		DisplayName: name,
		Notes:       notes,
		Links:       []Link{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.data.Identities = append(s.data.Identities, idn)
	if err := s.Save(); err != nil {
		s.data.Identities = s.data.Identities[:len(s.data.Identities)-1]
		return Identity{}, err
	}
	return idn, nil
}

// Update changes display name and notes.
func (s *Store) Update(id, displayName, notes string) (Identity, error) {
	idx, ok := s.index(id)
	if !ok {
		return Identity{}, ErrNotFound
	}
	name := trimName(displayName)
	if name == "" {
		return Identity{}, ErrEmptyName
	}
	s.data.Identities[idx].DisplayName = name
	s.data.Identities[idx].Notes = notes
	s.data.Identities[idx].UpdatedAt = time.Now().UTC()
	if err := s.Save(); err != nil {
		return Identity{}, err
	}
	return s.data.Identities[idx], nil
}

// Delete removes an identity and all of its links.
func (s *Store) Delete(id string) error {
	idx, ok := s.index(id)
	if !ok {
		return ErrNotFound
	}
	s.data.Identities = append(s.data.Identities[:idx], s.data.Identities[idx+1:]...)
	return s.Save()
}

// LinkChat attaches a chat id to the identity (manual only).
func (s *Store) LinkChat(id, chatID string) error {
	if chatID == "" {
		return ErrBadLink
	}
	return s.addLink(id, Link{
		Kind:     LinkChat,
		ChatID:   chatID,
		LinkedAt: time.Now().UTC(),
	})
}

// LinkUser attaches a (accountID, userID) profile to the identity.
func (s *Store) LinkUser(id, accountID, userID string) error {
	if accountID == "" || userID == "" {
		return ErrBadLink
	}
	return s.addLink(id, Link{
		Kind:      LinkUser,
		AccountID: accountID,
		UserID:    userID,
		LinkedAt:  time.Now().UTC(),
	})
}

// UnlinkChat removes a chat link from the identity.
func (s *Store) UnlinkChat(id, chatID string) error {
	return s.removeLink(id, func(l Link) bool {
		return l.Kind == LinkChat && l.ChatID == chatID
	})
}

// UnlinkUser removes a user link from the identity.
func (s *Store) UnlinkUser(id, accountID, userID string) error {
	return s.removeLink(id, func(l Link) bool {
		return l.Kind == LinkUser && l.AccountID == accountID && l.UserID == userID
	})
}

// ResolveByChat finds the identity linked to a chat id.
func (s *Store) ResolveByChat(chatID string) (Identity, bool) {
	if s == nil || chatID == "" {
		return Identity{}, false
	}
	for _, idn := range s.data.Identities {
		for _, l := range idn.Links {
			if l.Kind == LinkChat && l.ChatID == chatID {
				return idn, true
			}
		}
	}
	return Identity{}, false
}

// ResolveByUser finds the identity linked to a profile.
func (s *Store) ResolveByUser(accountID, userID string) (Identity, bool) {
	if s == nil || accountID == "" || userID == "" {
		return Identity{}, false
	}
	for _, idn := range s.data.Identities {
		for _, l := range idn.Links {
			if l.Kind == LinkUser && l.AccountID == accountID && l.UserID == userID {
				return idn, true
			}
		}
	}
	return Identity{}, false
}

// ResolveForChat prefers a chat link, then a user link for the peer.
func (s *Store) ResolveForChat(chatID, accountID, peerUserID string) (Identity, bool) {
	if idn, ok := s.ResolveByChat(chatID); ok {
		return idn, true
	}
	return s.ResolveByUser(accountID, peerUserID)
}

func (s *Store) addLink(id string, link Link) error {
	idx, ok := s.index(id)
	if !ok {
		return ErrNotFound
	}
	// Already on this identity: no-op success.
	for _, l := range s.data.Identities[idx].Links {
		if sameLink(l, link) {
			return nil
		}
	}
	// Owned by another identity.
	if owner, ok := s.ownerOf(link); ok && owner != id {
		return ErrLinkTaken
	}
	s.data.Identities[idx].Links = append(s.data.Identities[idx].Links, link)
	s.data.Identities[idx].UpdatedAt = time.Now().UTC()
	if err := s.Save(); err != nil {
		// roll back append
		links := s.data.Identities[idx].Links
		s.data.Identities[idx].Links = links[:len(links)-1]
		return err
	}
	return nil
}

func (s *Store) removeLink(id string, match func(Link) bool) error {
	idx, ok := s.index(id)
	if !ok {
		return ErrNotFound
	}
	links := s.data.Identities[idx].Links
	out := links[:0]
	found := false
	for _, l := range links {
		if match(l) {
			found = true
			continue
		}
		out = append(out, l)
	}
	if !found {
		return ErrNotFound
	}
	// Avoid retaining leftover capacity with old elements.
	s.data.Identities[idx].Links = append([]Link(nil), out...)
	s.data.Identities[idx].UpdatedAt = time.Now().UTC()
	return s.Save()
}

func (s *Store) ownerOf(link Link) (string, bool) {
	for _, idn := range s.data.Identities {
		for _, l := range idn.Links {
			if sameLink(l, link) {
				return idn.ID, true
			}
		}
	}
	return "", false
}

func (s *Store) index(id string) (int, bool) {
	if s == nil {
		return -1, false
	}
	for i, idn := range s.data.Identities {
		if idn.ID == id {
			return i, true
		}
	}
	return -1, false
}

func sameLink(a, b Link) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case LinkChat:
		return a.ChatID != "" && a.ChatID == b.ChatID
	case LinkUser:
		return a.AccountID != "" && a.UserID != "" &&
			a.AccountID == b.AccountID && a.UserID == b.UserID
	default:
		return false
	}
}

func trimName(s string) string {
	// Keep it simple: strip surrounding whitespace only.
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to timestamp bits.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
