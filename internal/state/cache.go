package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

const CurrentSchemaVersion = 1

var ErrCorruptCache = errors.New("state: cache file is corrupt")
var ErrSchemaMismatch = errors.New("state: cache schema version does not match current")

type Cache struct {
	SchemaVersion      int            `json:"schema_version"`
	LastSelectedChatID string         `json:"last_selected_chat_id"`
	Chats              []ChatSnapshot `json:"chats"`
}

type ChatSnapshot struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Account  string    `json:"account"`
	Unread   int       `json:"unread"`
	LastTs   time.Time `json:"last_ts"`
	LastBody string    `json:"last_body"`
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
