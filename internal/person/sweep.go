package person

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
	"github.com/taziksh/beeper-tui/internal/llm"
)

// MessagesAPI is the slice of the Beeper API the sweep reads.
type MessagesAPI interface {
	ListChats(ctx context.Context) ([]api.Chat, error)
	ListMessages(ctx context.Context, chatID string) ([]api.Message, error)
}

// sweepFile records when each person was last swept, so relaunching does not
// re-extract everyone. Machine-owned sidecar, not part of any card.
const sweepFile = ".sweep.json"

// sweepCooldown is how long a swept person stays quiet before the sweep
// looks at their messages again.
const sweepCooldown = 24 * time.Hour

// sweepMessageCap bounds how much history one person's extraction reads.
const sweepMessageCap = 80

// Sweep fills card gaps in the background: the most recently active people
// whose cards have empty fields get one extraction pass each, sequentially.
// Errors skip the person; the sweep never surfaces failures.
func Sweep(ctx context.Context, apiClient MessagesAPI, lc *llm.Client, store *Store, max int) {
	if lc.Model() == "" {
		if _, err := lc.DetectModel(ctx); err != nil {
			return
		}
	}
	chats, err := apiClient.ListChats(ctx)
	if err != nil {
		return
	}
	var targets []identity.Person
	for _, p := range identity.Build(chats, nil).All() {
		if p.Name == "" || len(p.Chats) == 0 {
			continue
		}
		targets = append(targets, p)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].LastActive.After(targets[j].LastActive)
	})

	marks := store.loadSweepMarks()
	done := 0
	for _, t := range targets {
		if done >= max || ctx.Err() != nil {
			return
		}
		card, err := store.Load(t.Name)
		if err != nil || cardFull(card) {
			continue
		}
		if at, ok := marks[Slug(t.Name)]; ok && time.Since(at) < sweepCooldown {
			continue
		}
		var msgs []api.Message
		for i, ref := range t.Chats {
			if i == 3 || len(msgs) >= sweepMessageCap {
				break
			}
			part, err := apiClient.ListMessages(ctx, ref.ID)
			if err != nil {
				continue
			}
			if room := sweepMessageCap - len(msgs); len(part) > room {
				part = part[len(part)-room:]
			}
			msgs = append(msgs, part...)
		}
		marks[Slug(t.Name)] = time.Now()
		store.saveSweepMarks(marks)
		done++
		if len(msgs) == 0 {
			continue
		}
		found, err := Extract(ctx, lc, t.Name, msgs)
		if err != nil {
			continue
		}
		changed, prov := Merge(&card, found)
		if len(changed) == 0 {
			continue
		}
		if err := store.Save(card); err != nil {
			continue
		}
		_ = store.RecordProvenance(t.Name, prov)
	}
}

// cardFull reports whether every schema field already has a value.
func cardFull(c Card) bool {
	return c.Birthday != "" && c.City != "" && c.Country != "" && len(c.Likes) > 0
}

func (s *Store) loadSweepMarks() map[string]time.Time {
	marks := map[string]time.Time{}
	if data, err := os.ReadFile(filepath.Join(s.dir, sweepFile)); err == nil {
		_ = json.Unmarshal(data, &marks)
	}
	return marks
}

func (s *Store) saveSweepMarks(marks map[string]time.Time) {
	data, err := json.MarshalIndent(marks, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(s.dir, sweepFile), data, 0o644)
}
