//go:build integration

package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/taziksh/beeper-tui/internal/config"
)

// TestIntegration_FrameTypes speaks the protocol raw, logs every frame's
// type, size, and control-message details, and runs decodeFrame on each
// frame so decoder mismatches with the real wire format surface as logged
// errors. It never logs entries, chat IDs, or any other content.
func TestIntegration_FrameTypes(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Token == "" {
		t.Skip("BEEPER_ACCESS_TOKEN not set; skipping integration test")
	}
	window := 5 * time.Second
	if d, err := time.ParseDuration(os.Getenv("BEEPER_TUI_WS_DRAIN")); err == nil {
		window = d
	}

	ctx, cancel := context.WithTimeout(context.Background(), window+15*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, cfg.BaseURL+"/v1/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + cfg.Token}},
	})
	if err != nil {
		t.Fatalf("dial error = %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(8 << 20)

	subscribed := false
	stop := time.After(window)
	for {
		select {
		case <-stop:
			t.Log("window elapsed")
			return
		default:
		}
		readCtx, readCancel := context.WithTimeout(ctx, window)
		_, data, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Logf("read ended: %v", err)
			return
		}
		var f struct {
			Type      string            `json:"type"`
			RequestID string            `json:"requestID"`
			Code      string            `json:"code"`
			Message   string            `json:"message"`
			Seq       int64             `json:"seq"`
			Entries   []json.RawMessage `json:"entries"`
		}
		if err := json.Unmarshal(data, &f); err != nil {
			t.Logf("frame: %d bytes, undecodable: %v", len(data), err)
			continue
		}
		switch f.Type {
		case "ready", "subscriptions.updated", "error":
			t.Logf("frame: type=%q requestID=%q code=%q message=%q", f.Type, f.RequestID, f.Code, f.Message)
		default:
			t.Logf("frame: type=%q seq=%d entries=%d size=%dB", f.Type, f.Seq, len(f.Entries), len(data))
		}
		if _, err := decodeFrame(data); err != nil {
			t.Logf("  decodeFrame REJECTS this frame: %v", err)
		} else {
			t.Log("  decodeFrame ok")
		}
		if f.Type == "ready" && !subscribed {
			subscribed = true
			cmd := map[string]any{"type": "subscriptions.set", "requestID": "r1", "chatIDs": []string{"*"}}
			if err := wsjson.Write(ctx, conn, cmd); err != nil {
				t.Fatalf("send subscriptions.set: %v", err)
			}
			t.Log("sent subscriptions.set [*]")
		}
	}
}
