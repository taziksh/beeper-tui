package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/taziksh/beeper-tui/internal/api"
)

// CurrentSchemaVersion is 2: v1 stored a trimmed chat snapshot that predates
// the tabbed inbox. Warm-start needs every field the list view reads, so the
// cache now stores api.Chat directly.
const CurrentSchemaVersion = 2

var ErrCorruptCache = errors.New("state: cache file is corrupt")
var ErrSchemaMismatch = errors.New("state: cache schema version does not match current")

type Cache struct {
	SchemaVersion      int        `json:"schema_version"`
	LastSelectedChatID string     `json:"last_selected_chat_id"`
	Chats              []api.Chat `json:"chats"`
}

func Save(path string, c Cache) error {
	c.SchemaVersion = CurrentSchemaVersion
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func Load(path string) (Cache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cache{}, nil
		}
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cache{}, fmt.Errorf("%w: %v", ErrCorruptCache, err)
	}
	if c.SchemaVersion != CurrentSchemaVersion {
		return Cache{}, fmt.Errorf("%w: file has %d, expected %d",
			ErrSchemaMismatch, c.SchemaVersion, CurrentSchemaVersion)
	}
	return c, nil
}
