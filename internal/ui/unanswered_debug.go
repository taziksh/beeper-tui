package ui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/taziksh/beeper-tui/internal/api"
)

// PrintUnansweredDebug prints one line per active chat with every signal the
// unanswered filter reads, so filter bugs can be diagnosed from flags alone.
// IDs are reduced to their shape and no titles, names, or text are printed.
func PrintUnansweredDebug(ctx context.Context, client *api.Client, w io.Writer) error {
	selfIDs := client.SelfIDs(ctx)
	chats, err := client.ListChats(ctx)
	if err != nil {
		return err
	}
	waiting := 0
	for i, c := range chats {
		if c.Archived || c.Preview == "" {
			continue
		}
		bot := false
		for _, p := range c.Participants {
			if p.IsBot {
				bot = true
			}
		}
		un := isUnanswered(c)
		if un {
			waiting++
		}
		idMatch := c.PreviewSenderID != "" && c.PreviewSenderID == selfIDs[c.AccountID]
		fmt.Fprintf(w, "#%03d %-12s %-6s unread=%-3d muted=%-5v low=%-5v bot=%-5v lastFromMe=%-5v idMatch=%-5v waiting=%-5v self=%s sender=%s\n",
			i, c.Network, c.Type, c.Unread, c.Muted, c.LowPriority, bot, c.LastFromMe, idMatch, un,
			idShape(selfIDs[c.AccountID]), idShape(c.PreviewSenderID))
	}
	fmt.Fprintf(w, "waiting=%d\n", waiting)
	return nil
}

// idShape strips identifying content from an ID, keeping only its structure:
// digit runs become N, letter runs become a, punctuation stays.
func idShape(id string) string {
	if id == "" {
		return "-"
	}
	var b strings.Builder
	var last rune
	for _, r := range id {
		switch {
		case r >= '0' && r <= '9':
			if last != 'N' {
				b.WriteByte('N')
				last = 'N'
			}
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			if last != 'a' {
				b.WriteByte('a')
				last = 'a'
			}
		default:
			b.WriteRune(r)
			last = r
		}
		if b.Len() > 24 {
			break
		}
	}
	return b.String()
}
