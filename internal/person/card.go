// Package person stores per-person cards: the facts Tazik wants to remember,
// as markdown files he owns and edits. Extraction fills gaps and never
// overwrites what a human wrote; its bookkeeping lives in a sidecar file, not
// in the cards.
package person

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Card is the person schema. The fields are Tazik's design; do not add,
// remove, or rename any without his explicit approval.
type Card struct {
	Name     string   `yaml:"name"`
	Birthday string   `yaml:"birthday,omitempty"`
	City     string   `yaml:"city,omitempty"`
	Country  string   `yaml:"country,omitempty"`
	Likes    []string `yaml:"likes,omitempty"`

	// Body is the freeform text below the frontmatter. It belongs to the
	// user; extraction never writes it.
	Body string `yaml:"-"`
}

// Store reads and writes cards in one directory, one markdown file per
// person.
type Store struct {
	dir string
}

// NewStore opens (creating if needed) the card directory.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("person: create store: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Slug converts a person's name to its card filename stem.
func Slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, Slug(name)+".md")
}

// Load reads the card for name. A missing card returns an empty card with
// the name set and no error.
func (s *Store) Load(name string) (Card, error) {
	data, err := os.ReadFile(s.path(name))
	if os.IsNotExist(err) {
		return Card{Name: name}, nil
	}
	if err != nil {
		return Card{}, fmt.Errorf("person: read card: %w", err)
	}
	return parseCard(data, name)
}

// Save writes the card. The file is fully rewritten; Body is preserved as
// loaded.
func (s *Store) Save(c Card) error {
	data, err := renderCard(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path(c.Name), data, 0o644); err != nil {
		return fmt.Errorf("person: write card: %w", err)
	}
	return nil
}

func parseCard(data []byte, name string) (Card, error) {
	c := Card{Name: name}
	rest := data
	if bytes.HasPrefix(data, []byte("---\n")) {
		if end := bytes.Index(data[4:], []byte("\n---")); end >= 0 {
			front := data[4 : 4+end]
			rest = data[4+end+4:]
			rest = bytes.TrimPrefix(rest, []byte("\n"))
			if err := yaml.Unmarshal(front, &c); err != nil {
				return Card{}, fmt.Errorf("person: parse card frontmatter: %w", err)
			}
		}
	}
	c.Body = strings.TrimSpace(string(rest))
	if c.Name == "" {
		c.Name = name
	}
	return c, nil
}

func renderCard(c Card) ([]byte, error) {
	front, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("person: render card: %w", err)
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(front)
	b.WriteString("---\n")
	if c.Body != "" {
		b.WriteString("\n" + c.Body + "\n")
	}
	return b.Bytes(), nil
}
