package state

import (
	"encoding/json"
	"os"
	"time"
)

const CurrentSchemaVersion = 1

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
