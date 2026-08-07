// Command beeper-stub is a fake Beeper Desktop API server with synthetic
// data, for developing and end-to-end testing beeper-tui without touching a
// real account. Point the TUI at it with:
//
//	BEEPER_API_BASE_URL=http://127.0.0.1:23374 BEEPER_ACCESS_TOKEN=stub beeper-tui
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// message mirrors the JSON shape of the real API's shared.Message, limited to
// the fields the TUI reads.
type message struct {
	ID          string       `json:"id"`
	ChatID      string       `json:"chatID"`
	AccountID   string       `json:"accountID"`
	SenderID    string       `json:"senderID"`
	SenderName  string       `json:"senderName"`
	Text        string       `json:"text"`
	Timestamp   time.Time    `json:"timestamp"`
	IsSender    bool         `json:"isSender"`
	IsUnread    bool         `json:"isUnread"`
	Type        string       `json:"type"`
	Attachments []attachment `json:"attachments,omitempty"`
	Reactions   []any        `json:"reactions"`
	SortKey     string       `json:"sortKey"`
}

type attachment struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	SrcURL   string `json:"srcURL"`
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	FileSize int64  `json:"fileSize"`
}

type chat struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"accountID"`
	Network      string    `json:"network"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	UnreadCount  int       `json:"unreadCount"`
	LastActivity time.Time `json:"lastActivity"`
	Preview      *message  `json:"preview,omitempty"`
}

type server struct {
	mu      sync.Mutex
	chats   []chat
	msgs    map[string][]message
	uploads map[string]attachment
	seq     int
}

func seed() *server {
	now := time.Now()
	s := &server{
		chats: []chat{
			{ID: "stub-dm", AccountID: "stubacct", Network: "stubnet", Title: "Ada Testface", Type: "single", LastActivity: now},
			{ID: "stub-group", AccountID: "stubacct", Network: "stubnet", Title: "Fixture Friends", Type: "group", UnreadCount: 2, LastActivity: now.Add(-time.Hour)},
		},
		msgs:    map[string][]message{},
		uploads: map[string]attachment{},
	}
	s.append("stub-dm", message{SenderID: "u-ada", SenderName: "Ada Testface", Text: "hello from the stub", Timestamp: now.Add(-2 * time.Minute)})
	s.append("stub-group", message{SenderID: "u-ada", SenderName: "Ada Testface", Text: "synthetic group chatter", Timestamp: now.Add(-time.Hour)})
	return s
}

func (s *server) append(chatID string, m message) message {
	s.seq++
	m.ID = fmt.Sprintf("stub-msg-%d", s.seq)
	m.ChatID = chatID
	m.AccountID = "stubacct"
	m.Type = "text"
	m.SortKey = fmt.Sprintf("%020d", s.seq)
	if m.Reactions == nil {
		m.Reactions = []any{}
	}
	s.msgs[chatID] = append(s.msgs[chatID], m)
	return m
}

func page(items any) map[string]any {
	return map[string]any{"items": items, "hasMore": false, "oldestCursor": "", "newestCursor": ""}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:23374", "listen address")
	flag.Parse()
	s := seed()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{{
			"accountID": "stubacct",
			"network":   "stubnet",
			"user":      map[string]any{"id": "u-self", "fullName": "Stub Self"},
		}})
	})

	mux.HandleFunc("GET /v1/chats", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		items := make([]map[string]any, 0, len(s.chats))
		for _, c := range s.chats {
			if ms := s.msgs[c.ID]; len(ms) > 0 {
				last := ms[len(ms)-1]
				c.Preview = &last
				c.LastActivity = last.Timestamp
			}
			var m map[string]any
			b, _ := json.Marshal(c)
			if err := json.Unmarshal(b, &m); err != nil {
				continue
			}
			m["participants"] = page([]any{})
			m["capabilities"] = map[string]any{}
			items = append(items, m)
		}
		writeJSON(w, page(items))
	})

	mux.HandleFunc("GET /v1/chats/{chat}/messages", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, page(s.msgs[r.PathValue("chat")]))
	})

	mux.HandleFunc("POST /v1/chats/{chat}/messages", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text       string `json:"text"`
			Attachment struct {
				UploadID string `json:"uploadID"`
			} `json:"attachment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		chatID := r.PathValue("chat")
		m := message{SenderID: "u-self", SenderName: "Stub Self", Text: body.Text, Timestamp: time.Now(), IsSender: true}
		if att, ok := s.uploads[body.Attachment.UploadID]; ok {
			m.Attachments = []attachment{att}
		}
		m = s.append(chatID, m)
		writeJSON(w, map[string]any{"chatID": chatID, "pendingMessageID": m.ID})
	})

	mux.HandleFunc("POST /v1/chats/{chat}/read", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /v1/assets/upload", func(w http.ResponseWriter, r *http.Request) {
		f, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		s.mu.Lock()
		s.seq++
		id := fmt.Sprintf("stub-upload-%d", s.seq)
		s.mu.Unlock()
		path := filepath.Join(os.TempDir(), "beeper-stub-"+id+"-"+filepath.Base(hdr.Filename))
		out, err := os.Create(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		n, err := io.Copy(out, f)
		if err == nil {
			err = out.Close()
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		att := attachment{
			Type: "img", ID: id, SrcURL: "file://" + path,
			FileName: hdr.Filename, MimeType: hdr.Header.Get("Content-Type"), FileSize: n,
		}
		s.mu.Lock()
		s.uploads[id] = att
		s.mu.Unlock()
		writeJSON(w, map[string]any{"uploadID": id, "srcURL": att.SrcURL, "fileName": att.FileName, "fileSize": n})
	})

	mux.HandleFunc("/v1/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx := r.Context()
		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"ready","version":1,"chatIDs":[]}`)); err != nil {
			return
		}
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	})

	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s", r.Method, r.URL.Path)
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	log.Printf("beeper-stub listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, auth(mux)))
}
