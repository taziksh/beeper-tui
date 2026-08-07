package ui

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/taziksh/beeper-tui/internal/api"
)

// debugLog appends timing lines for slow update and render passes to
// beeper-tui-debug.log in the temp dir when BEEPER_TUI_DEBUG is set. Only
// message type names are logged, never chat content.
var debugLog = func() *log.Logger {
	if os.Getenv("BEEPER_TUI_DEBUG") == "" {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "beeper-tui-debug.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	return log.New(f, "", log.LstdFlags|log.Lmicroseconds)
}()

// slowThreshold is two dropped frames at 60Hz; anything above it is felt as lag.
const slowThreshold = 32 * time.Millisecond

func logSlow(what string, start time.Time) {
	if d := time.Since(start); d > slowThreshold {
		debugLog.Printf("%s took %v", what, d)
	}
}

// logAPIResult records an API call's outcome by shape only: operation name,
// elapsed time, Go error type, and HTTP status. Never the error string, which
// embeds request URLs and therefore chat IDs.
func logAPIResult(op string, start time.Time, err error) {
	if debugLog == nil {
		return
	}
	if err == nil {
		debugLog.Printf("%s ok in %v", op, time.Since(start))
		return
	}
	debugLog.Printf("%s failed in %v: %T status=%d", op, time.Since(start), err, api.ErrorStatus(err))
}
