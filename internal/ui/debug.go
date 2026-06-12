package ui

import (
	"log"
	"os"
	"path/filepath"
	"time"
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
