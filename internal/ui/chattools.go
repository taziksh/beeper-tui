package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
	"github.com/taziksh/beeper-tui/internal/identity"
	"github.com/taziksh/beeper-tui/internal/llm"
	"github.com/taziksh/beeper-tui/internal/person"
)

// chatTools declares the read-only tools the assistant may call. v1 exposes
// no write operations; drafting and sending are later tiers.
var chatTools = []llm.Tool{
	{Type: "function", Function: llm.ToolSpec{
		Name:        "list_chats",
		Description: "List the user's chats with unread counts, network, and last-message preview. Use filter=unread to see only chats with unread messages.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filter": map[string]any{
					"type":        "string",
					"enum":        []string{"all", "unread", "inbox"},
					"description": "all chats, only unread ones, or the active inbox (default inbox)",
				},
				"limit": map[string]any{"type": "integer", "description": "max chats to return, default 40"},
			},
		},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "list_messages",
		Description: "Read the recent messages of one chat, oldest to newest. Identify the chat by its title (fuzzy) or id.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chat":  map[string]any{"type": "string", "description": "chat title or id"},
				"limit": map[string]any{"type": "integer", "description": "max messages, default 30"},
			},
			"required": []string{"chat"},
		},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "search_messages",
		Description: "Search message contents across chats. Literal word matching: use single words people actually type, not phrases or concepts. Filters narrow by chat, sender, and date.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "description": "words to search for; omit to filter only"},
				"chat":   map[string]any{"type": "string", "description": "restrict to one chat, by title or id"},
				"sender": map[string]any{"type": "string", "description": "'me', 'others', or a user id"},
				"after":  map[string]any{"type": "string", "description": "only messages after this date, YYYY-MM-DD"},
				"before": map[string]any{"type": "string", "description": "only messages before this date, YYYY-MM-DD"},
				"limit":  map[string]any{"type": "integer", "description": "max results, default 20"},
			},
		},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "unanswered_chats",
		Description: "List chats waiting on the user's reply: the other person spoke last. Use this first for questions about owed replies or follow-ups.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "search_contacts",
		Description: "Find a person across all messaging accounts by name, handle, phone, or email.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "name or identifier to look up"},
			},
			"required": []string{"query"},
		},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "person_card",
		Description: "What the user knows about a person: birthday, city, country, likes, and their own notes. Call for any question about who someone is, what they like, where they live, or when their birthday is.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "the person's name"},
			},
			"required": []string{"name"},
		},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "update_person_card",
		Description: "Learn about a person from their recent messages: extracts birthday, city, country, and likes into their saved card. Call when person_card is empty or missing the fact the user wants. Fills gaps only; never overwrites saved values.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "the person's name"},
			},
			"required": []string{"name"},
		},
	}},
	{Type: "function", Function: llm.ToolSpec{
		Name:        "set_reminder",
		Description: "Set a Beeper reminder on a chat, e.g. to reply later. The reminder fires as a notification at the given time.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chat":               map[string]any{"type": "string", "description": "chat title or id"},
				"at":                 map[string]any{"type": "string", "description": "when to remind, ISO 8601 like 2026-08-08T09:00"},
				"dismiss_on_message": map[string]any{"type": "boolean", "description": "cancel if the chat gets a new message first, default false"},
			},
			"required": []string{"chat", "at"},
		},
	}},
}

// toolResultLimit caps one tool result so a chatty tool cannot flood the
// model's context.
const toolResultLimit = 6000

// toolEnv carries what tool implementations may touch: the Beeper API, the
// local model for extraction, and the person-card store.
type toolEnv struct {
	client *api.Client
	llm    *llm.Client
	people *person.Store
}

// resolvePerson fuzzy-matches name against the identity index and returns
// the best match.
func resolvePerson(ctx context.Context, client *api.Client, name string) (identity.Person, error) {
	chats, err := client.ListChats(ctx)
	if err != nil {
		return identity.Person{}, err
	}
	// The server search sees every inbox and matches participants, catching
	// chats the list endpoint omits.
	if searched, err := client.SearchChats(ctx, name); err == nil {
		chats = append(chats, searched...)
	}
	contacts, _ := client.ListContacts(ctx)
	people := identity.Build(chats, contacts).Search(name, 1)
	if len(people) == 0 {
		return identity.Person{}, fmt.Errorf("no person matches %q", name)
	}
	return people[0], nil
}

func toolPersonCard(ctx context.Context, env toolEnv, args string) (string, toolStep, error) {
	var p struct {
		Name string `json:"name"`
	}
	parseToolArgs(args, &p)
	step := toolStep{name: "person_card", args: truncate(p.Name, 30)}
	if env.people == nil {
		return "", step, fmt.Errorf("person store unavailable")
	}
	if strings.TrimSpace(p.Name) == "" {
		return "", step, fmt.Errorf("name is required")
	}
	name := p.Name
	if who, err := resolvePerson(ctx, env.client, p.Name); err == nil {
		name = who.Name
	}
	step.args = truncate(name, 30)
	card, err := env.people.Load(name)
	if err != nil {
		return "", step, err
	}
	if card.Birthday == "" && card.City == "" && card.Country == "" && len(card.Likes) == 0 && card.Body == "" {
		step.result = "empty"
		return fmt.Sprintf("no card yet for %s; update_person_card can extract one from their messages", name), step, nil
	}
	step.result = "shown"
	return env.people.Render(card), step, nil
}

// updateCardMessageCap bounds how much history one extraction pass reads.
const updateCardMessageCap = 80

func toolUpdatePersonCard(ctx context.Context, env toolEnv, args string) (string, toolStep, error) {
	var p struct {
		Name string `json:"name"`
	}
	parseToolArgs(args, &p)
	step := toolStep{name: "update_person_card", args: truncate(p.Name, 30)}
	if env.people == nil || env.llm == nil {
		return "", step, fmt.Errorf("person store unavailable")
	}
	if strings.TrimSpace(p.Name) == "" {
		return "", step, fmt.Errorf("name is required")
	}
	who, err := resolvePerson(ctx, env.client, p.Name)
	if err != nil {
		return "", step, err
	}
	if len(who.Chats) == 0 {
		return "", step, fmt.Errorf("no chats with %s to extract from", who.Name)
	}
	var msgs []api.Message
	for i, ref := range who.Chats {
		if i == 3 || len(msgs) >= updateCardMessageCap {
			break
		}
		part, err := env.client.ListMessages(ctx, ref.ID)
		if err != nil {
			continue
		}
		if room := updateCardMessageCap - len(msgs); len(part) > room {
			part = part[len(part)-room:]
		}
		msgs = append(msgs, part...)
	}
	found, err := person.Extract(ctx, env.llm, who.Name, msgs)
	if err != nil {
		return "", step, err
	}
	card, err := env.people.Load(who.Name)
	if err != nil {
		return "", step, err
	}
	changed, prov := person.Merge(&card, found)
	step.args = truncate(who.Name, 30)
	if len(changed) == 0 {
		step.result = "nothing new"
		return "nothing new to add\n\n" + env.people.Render(card), step, nil
	}
	if err := env.people.Save(card); err != nil {
		return "", step, err
	}
	if err := env.people.RecordProvenance(who.Name, prov); err != nil {
		return "", step, err
	}
	step.result = fmt.Sprintf("%d added", len(changed))
	return "added: " + strings.Join(changed, ", ") + "\n\n" + env.people.Render(card), step, nil
}

// execChatTool runs one tool call and returns the text handed back to the
// model plus the finished display step for the transcript trace.
func execChatTool(ctx context.Context, env toolEnv, call llm.ToolCall) (string, toolStep) {
	client := env.client
	step := toolStep{name: call.Function.Name}
	var result string
	var err error
	switch call.Function.Name {
	case "list_chats":
		result, step, err = toolListChats(ctx, client, call.Function.Arguments)
	case "list_messages":
		result, step, err = toolListMessages(ctx, client, call.Function.Arguments)
	case "search_messages":
		result, step, err = toolSearchMessages(ctx, client, call.Function.Arguments)
	case "unanswered_chats":
		result, step, err = toolUnansweredChats(ctx, client)
	case "search_contacts":
		result, step, err = toolSearchContacts(ctx, env, call.Function.Arguments)
	case "set_reminder":
		result, step, err = toolSetReminder(ctx, client, call.Function.Arguments)
	case "person_card":
		result, step, err = toolPersonCard(ctx, env, call.Function.Arguments)
	case "update_person_card":
		result, step, err = toolUpdatePersonCard(ctx, env, call.Function.Arguments)
	default:
		err = fmt.Errorf("unknown tool %q", call.Function.Name)
	}
	if err != nil {
		step.result = "error: " + truncate(err.Error(), 60)
		return "error: " + err.Error(), step
	}
	if len(result) > toolResultLimit {
		result = result[:toolResultLimit] + "\n(truncated)"
	}
	return result, step
}

func toolListChats(ctx context.Context, client *api.Client, args string) (string, toolStep, error) {
	var p struct {
		Filter string `json:"filter"`
		Limit  int    `json:"limit"`
	}
	parseToolArgs(args, &p)
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 40
	}
	if p.Filter == "" {
		p.Filter = "inbox"
	}
	step := toolStep{name: "list_chats", args: p.Filter}
	chats, err := client.ListChats(ctx)
	if err != nil {
		return "", step, err
	}
	sort.Slice(chats, func(i, j int) bool { return chats[i].LastActive.After(chats[j].LastActive) })
	var b strings.Builder
	b.WriteString("title | network | unread | mentions | last_active | last_message\n")
	n := 0
	for _, c := range chats {
		switch p.Filter {
		case "unread":
			if c.Unread == 0 && !c.MarkedUnread {
				continue
			}
		case "inbox":
			if c.Archived || c.Muted || c.LowPriority {
				continue
			}
		}
		fmt.Fprintf(&b, "%s | %s | %d | %d | %s | %s\n",
			c.Title, c.Network, c.Unread, c.Mentions,
			formatToolTime(c.LastActive), truncate(c.Preview, 120))
		n++
		if n >= p.Limit {
			break
		}
	}
	step.result = fmt.Sprintf("%d chats", n)
	if n == 0 {
		return "no chats match", step, nil
	}
	return b.String(), step, nil
}

func toolListMessages(ctx context.Context, client *api.Client, args string) (string, toolStep, error) {
	var p struct {
		Chat  string `json:"chat"`
		Limit int    `json:"limit"`
	}
	parseToolArgs(args, &p)
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 30
	}
	step := toolStep{name: "list_messages", args: truncate(p.Chat, 30)}
	if strings.TrimSpace(p.Chat) == "" {
		return "", step, fmt.Errorf("chat is required")
	}
	chats, err := client.ListChats(ctx)
	if err != nil {
		return "", step, err
	}
	chat, candidates := resolveChatFull(ctx, client, chats, p.Chat)
	if chat == nil {
		if len(candidates) > 0 {
			step.result = "ambiguous"
			return "ambiguous chat, candidates: " + strings.Join(candidates, " | "), step, nil
		}
		return "", step, fmt.Errorf("no chat matches %q", p.Chat)
	}
	step.args = truncate(chat.Title, 30)
	msgs, err := client.ListMessages(ctx, chat.ID)
	if err != nil {
		return "", step, err
	}
	if len(msgs) > p.Limit {
		msgs = msgs[len(msgs)-p.Limit:]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "chat: %s (%s)\ntime | sender | text\n", chat.Title, chat.Network)
	for _, msg := range msgs {
		if msg.IsReaction {
			continue
		}
		fmt.Fprintf(&b, "%s | %s | %s\n", formatToolTime(msg.Timestamp), senderLabel(msg), messageText(msg))
	}
	step.result = fmt.Sprintf("%d msgs", len(msgs))
	step.sources = messageSources(msgs, chat.Title, 3)
	return b.String(), step, nil
}

func toolSearchMessages(ctx context.Context, client *api.Client, args string) (string, toolStep, error) {
	var p struct {
		Query  string `json:"query"`
		Chat   string `json:"chat"`
		Sender string `json:"sender"`
		After  string `json:"after"`
		Before string `json:"before"`
		Limit  int    `json:"limit"`
	}
	parseToolArgs(args, &p)
	step := toolStep{name: "search_messages", args: truncate(p.Query, 30)}
	q := api.SearchQuery{Query: p.Query, Sender: p.Sender, Limit: p.Limit}
	if p.Chat != "" {
		chats, err := client.ListChats(ctx)
		if err != nil {
			return "", step, err
		}
		chat, candidates := resolveChatFull(ctx, client, chats, p.Chat)
		if chat == nil {
			if len(candidates) > 0 {
				step.result = "ambiguous"
				return "ambiguous chat, candidates: " + strings.Join(candidates, " | "), step, nil
			}
			return "", step, fmt.Errorf("no chat matches %q", p.Chat)
		}
		q.ChatID = chat.ID
		if step.args == "" {
			step.args = truncate(chat.Title, 30)
		}
	}
	var err error
	if q.After, err = parseToolDate(p.After, false); err != nil {
		return "", step, err
	}
	if q.Before, err = parseToolDate(p.Before, true); err != nil {
		return "", step, err
	}
	if p.Query == "" && q.ChatID == "" && q.Sender == "" && q.After.IsZero() && q.Before.IsZero() {
		return "", step, fmt.Errorf("give a query or at least one filter")
	}
	results, err := client.SearchMessagesFiltered(ctx, q)
	if err != nil {
		return "", step, err
	}
	step.result = fmt.Sprintf("%d results", len(results))
	if len(results) == 0 {
		return "no messages match", step, nil
	}
	titles := map[string]string{}
	if chats, err := client.ListChats(ctx); err == nil {
		for _, c := range chats {
			titles[c.ID] = c.Title
		}
	}
	var b strings.Builder
	b.WriteString("chat | time | sender | text\n")
	sources := make([]chatSource, 0, 5)
	for _, r := range results {
		msg := r.Message
		fmt.Fprintf(&b, "%s | %s | %s | %s\n",
			chatTitleIn(titles, msg.ChatID), formatToolTime(msg.Timestamp), senderLabel(msg), messageText(msg))
		if len(sources) < 5 && !msg.IsReaction && msg.Text != "" {
			sources = append(sources, chatSource{
				chatID:    msg.ChatID,
				chatTitle: chatTitleIn(titles, msg.ChatID),
				sender:    senderLabel(msg),
				snippet:   truncate(msg.Text, 60),
				ts:        msg.Timestamp,
			})
		}
	}
	step.sources = sources
	return b.String(), step, nil
}

// toolUnansweredChats lists chats whose last message is from someone else:
// DMs unconditionally, groups only when they mention the user. Muted,
// low-priority, and archived chats are noise, not owed replies.
func toolUnansweredChats(ctx context.Context, client *api.Client) (string, toolStep, error) {
	step := toolStep{name: "unanswered_chats"}
	chats, err := client.ListChats(ctx)
	if err != nil {
		return "", step, err
	}
	sort.Slice(chats, func(i, j int) bool { return chats[i].LastActive.After(chats[j].LastActive) })
	var b strings.Builder
	b.WriteString("chat | network | last_sender | waiting_since | last_message\n")
	n := 0
	for _, c := range chats {
		if !isUnanswered(c) {
			continue
		}
		fmt.Fprintf(&b, "%s | %s | %s | %s | %s\n",
			c.Title, c.Network, c.LastSender, formatToolTime(c.LastActive), truncate(c.Preview, 120))
		n++
		if n >= 40 {
			break
		}
	}
	step.result = fmt.Sprintf("%d waiting", n)
	if n == 0 {
		return "no chats are waiting on a reply", step, nil
	}
	return b.String(), step, nil
}

// isUnanswered reports whether a chat is waiting on the user's reply.
func isUnanswered(c api.Chat) bool {
	if c.Archived || c.Muted || c.LowPriority || c.LastFromMe || c.Preview == "" {
		return false
	}
	if c.Type == "single" {
		// A DM with a bot or notification channel is not owed a reply.
		for _, p := range c.Participants {
			if p.IsBot {
				return false
			}
		}
		return true
	}
	return c.Mentions > 0
}

func toolSearchContacts(ctx context.Context, env toolEnv, args string) (string, toolStep, error) {
	client := env.client
	var p struct {
		Query string `json:"query"`
	}
	parseToolArgs(args, &p)
	step := toolStep{name: "search_contacts", args: truncate(p.Query, 30)}
	if strings.TrimSpace(p.Query) == "" {
		return "", step, fmt.Errorf("query is required")
	}
	chats, err := client.ListChats(ctx)
	if err != nil {
		return "", step, err
	}
	if searched, err := client.SearchChats(ctx, p.Query); err == nil {
		chats = append(chats, searched...)
	}
	contacts, _ := client.ListContacts(ctx)
	remote, _ := client.SearchContacts(ctx, p.Query)
	people := identity.Build(chats, append(contacts, remote...)).Search(p.Query, 20)
	step.result = fmt.Sprintf("%d found", len(people))
	if len(people) == 0 {
		return "no contacts match", step, nil
	}
	var b strings.Builder
	b.WriteString("name | handle | network | phone | email | chats | user_id\n")
	for _, person := range people {
		titles := make([]string, 0, 2)
		for _, ref := range person.Chats {
			if len(titles) == 2 {
				break
			}
			titles = append(titles, ref.Title)
		}
		fmt.Fprintf(&b, "%s | %s | %s | %s | %s | %s | %s\n",
			person.Name, person.Username, person.Network, person.Phone, person.Email,
			strings.Join(titles, "; "), person.UserID)
	}
	appendCardFacts(&b, env, people)
	return b.String(), step, nil
}

// appendCardFacts joins saved person-card facts onto contact results, so the
// model sees what the user already knows without asking for it.
func appendCardFacts(b *strings.Builder, env toolEnv, people []identity.Person) {
	if env.people == nil {
		return
	}
	wrote := false
	for i, p := range people {
		if i == 5 {
			break
		}
		card, err := env.people.Load(p.Name)
		if err != nil {
			continue
		}
		var facts []string
		if card.Birthday != "" {
			facts = append(facts, "birthday "+card.Birthday)
		}
		if card.City != "" {
			facts = append(facts, "city "+card.City)
		}
		if card.Country != "" {
			facts = append(facts, "country "+card.Country)
		}
		if len(card.Likes) > 0 {
			facts = append(facts, "likes "+strings.Join(card.Likes, ", "))
		}
		if len(facts) == 0 {
			continue
		}
		if !wrote {
			b.WriteString("\nknown facts:\n")
			wrote = true
		}
		fmt.Fprintf(b, "%s: %s\n", card.Name, strings.Join(facts, "; "))
	}
}

func toolSetReminder(ctx context.Context, client *api.Client, args string) (string, toolStep, error) {
	var p struct {
		Chat             string `json:"chat"`
		At               string `json:"at"`
		DismissOnMessage bool   `json:"dismiss_on_message"`
	}
	parseToolArgs(args, &p)
	step := toolStep{name: "set_reminder", args: truncate(p.Chat, 30)}
	chats, err := client.ListChats(ctx)
	if err != nil {
		return "", step, err
	}
	chat, candidates := resolveChatFull(ctx, client, chats, p.Chat)
	if chat == nil {
		if len(candidates) > 0 {
			step.result = "ambiguous"
			return "ambiguous chat, candidates: " + strings.Join(candidates, " | "), step, nil
		}
		return "", step, fmt.Errorf("no chat matches %q", p.Chat)
	}
	at, err := parseToolTimestamp(p.At)
	if err != nil {
		return "", step, err
	}
	if err := client.SetReminder(ctx, chat.ID, at, p.DismissOnMessage); err != nil {
		return "", step, err
	}
	step.args = truncate(chat.Title, 30)
	step.result = at.Local().Format("Jan 2 15:04")
	return fmt.Sprintf("reminder set on %s for %s", chat.Title, at.Local().Format("2006-01-02 15:04")), step, nil
}

// parseToolDate reads a YYYY-MM-DD date in local time. end pushes the moment
// to the end of that day, so before=2026-08-07 includes the 7th.
func parseToolDate(s string, end bool) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("bad date %q, want YYYY-MM-DD", s)
	}
	if end {
		t = t.AddDate(0, 0, 1)
	}
	return t, nil
}

// parseToolTimestamp reads the timestamp formats models produce for
// reminders, requiring an explicit time of day.
func parseToolTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad time %q, want ISO 8601 like 2026-08-08T09:00", s)
}

// resolveChat matches a title or id against the chat list: exact id, then
// exact title, then substring. One substring hit resolves; several return
// candidates for the model to disambiguate.
// resolveChatFull resolves like resolveChat, then falls back to the server's
// chat search, which sees every inbox and matches participant names the
// local list may not carry.
func resolveChatFull(ctx context.Context, client *api.Client, chats []api.Chat, key string) (*api.Chat, []string) {
	chat, candidates := resolveChat(chats, key)
	if chat != nil || len(candidates) > 0 {
		return chat, candidates
	}
	searched, err := client.SearchChats(ctx, key)
	if err != nil || len(searched) == 0 {
		return nil, nil
	}
	if chat, candidates := resolveChat(searched, key); chat != nil || len(candidates) > 0 {
		return chat, candidates
	}
	if len(searched) == 1 {
		return &searched[0], nil
	}
	names := make([]string, 0, 8)
	for _, c := range searched {
		names = append(names, c.Title)
		if len(names) == 8 {
			break
		}
	}
	return nil, names
}

func resolveChat(chats []api.Chat, key string) (*api.Chat, []string) {
	fold := strings.ToLower(strings.TrimSpace(key))
	var sub []*api.Chat
	for i := range chats {
		c := &chats[i]
		if c.ID == key {
			return c, nil
		}
		title := strings.ToLower(c.Title)
		if title == fold {
			return c, nil
		}
		if strings.Contains(title, fold) {
			sub = append(sub, c)
		}
	}
	if len(sub) == 1 {
		return sub[0], nil
	}
	if len(sub) == 0 {
		titles := make([]string, len(chats))
		for i := range chats {
			titles[i] = chats[i].Title
		}
		for _, idx := range identity.MatchStrings(key, titles) {
			sub = append(sub, &chats[idx])
		}
		if len(sub) == 1 {
			return sub[0], nil
		}
	}
	names := make([]string, 0, len(sub))
	for _, c := range sub {
		names = append(names, c.Title)
		if len(names) >= 8 {
			break
		}
	}
	return nil, names
}

// messageSources keeps the newest n non-reaction messages as trace sources.
func messageSources(msgs []api.Message, chatTitle string, n int) []chatSource {
	out := make([]chatSource, 0, n)
	for i := len(msgs) - 1; i >= 0 && len(out) < n; i-- {
		msg := msgs[i]
		if msg.IsReaction || msg.Text == "" {
			continue
		}
		title := chatTitle
		if title == "" {
			title = msg.ChatID
		}
		out = append(out, chatSource{
			chatID:    msg.ChatID,
			chatTitle: title,
			sender:    senderLabel(msg),
			snippet:   truncate(msg.Text, 60),
			ts:        msg.Timestamp,
		})
	}
	return out
}

func senderLabel(msg api.Message) string {
	if msg.IsFromMe {
		return "me"
	}
	if msg.SenderName == "" {
		return "?"
	}
	return msg.SenderName
}

// messageText is the message body as shown to the model, with attachments
// noted since their content is not readable.
func messageText(msg api.Message) string {
	text := strings.ReplaceAll(msg.Text, "\n", " ")
	if len(msg.Attachments) > 0 {
		text = strings.TrimSpace(text + fmt.Sprintf(" [%d attachment(s)]", len(msg.Attachments)))
	}
	return truncate(text, 200)
}

// chatTitleIn resolves a chat id to its title, falling back to the raw id,
// which still lets the model call list_messages on it.
func chatTitleIn(titles map[string]string, chatID string) string {
	if t, ok := titles[chatID]; ok {
		return t
	}
	return chatID
}

func formatToolTime(ts time.Time) string {
	if ts.IsZero() {
		return "?"
	}
	return ts.Local().Format("2006-01-02 15:04")
}

// parseToolArgs decodes model-provided JSON arguments, tolerating an empty
// string. Bad JSON leaves the zero value; validation happens per tool.
func parseToolArgs(args string, v any) {
	if strings.TrimSpace(args) == "" {
		return
	}
	_ = json.Unmarshal([]byte(args), v)
}
