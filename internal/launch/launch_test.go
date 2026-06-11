package launch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEnsureRunning_AlreadyReachable_DoesNotStart(t *testing.T) {
	probe := func(context.Context) bool { return true }
	start := func() error {
		t.Fatal("start called even though the API was reachable")
		return nil
	}
	if err := ensureRunning(context.Background(), "http://127.0.0.1:23373", probe, start, time.Millisecond, time.Second); err != nil {
		t.Fatalf("ensureRunning() = %v, want nil", err)
	}
}

func TestEnsureRunning_StartsAndWaits(t *testing.T) {
	started := false
	probe := func(context.Context) bool { return started }
	start := func() error {
		started = true
		return nil
	}
	if err := ensureRunning(context.Background(), "http://127.0.0.1:23373", probe, start, time.Millisecond, time.Second); err != nil {
		t.Fatalf("ensureRunning() = %v, want nil", err)
	}
	if !started {
		t.Fatal("start was never called")
	}
}

func TestEnsureRunning_TimesOut(t *testing.T) {
	probe := func(context.Context) bool { return false }
	start := func() error { return nil }
	err := ensureRunning(context.Background(), "http://127.0.0.1:23373", probe, start, time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not come up") {
		t.Fatalf("ensureRunning() = %v, want timeout error", err)
	}
}

func TestEnsureRunning_StartError(t *testing.T) {
	probe := func(context.Context) bool { return false }
	start := func() error { return errors.New("no such app") }
	err := ensureRunning(context.Background(), "http://127.0.0.1:23373", probe, start, time.Millisecond, time.Second)
	if err == nil || !strings.Contains(err.Error(), "no such app") {
		t.Fatalf("ensureRunning() = %v, want start error", err)
	}
}

func TestEnsureRunning_RemoteHost_DoesNotStart(t *testing.T) {
	probe := func(context.Context) bool { return false }
	start := func() error {
		t.Fatal("start called for a non-loopback base URL")
		return nil
	}
	err := ensureRunning(context.Background(), "http://192.168.1.10:23373", probe, start, time.Millisecond, time.Second)
	if err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("ensureRunning() = %v, want unreachable error", err)
	}
}

func TestEnsureRunning_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	probe := func(context.Context) bool { return false }
	start := func() error { return nil }
	err := ensureRunning(ctx, "http://127.0.0.1:23373", probe, start, time.Millisecond, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ensureRunning() = %v, want context.Canceled", err)
	}
}

func TestReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()
	if !reachable(context.Background(), srv.URL) {
		t.Errorf("reachable(%q) = false, want true: any HTTP response counts", srv.URL)
	}

	srv.Close()
	if reachable(context.Background(), srv.URL) {
		t.Errorf("reachable(%q) = true after server close, want false", srv.URL)
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		baseURL string
		want    bool
	}{
		{"http://127.0.0.1:23373", true},
		{"http://localhost:23373", true},
		{"http://[::1]:23373", true},
		{"http://192.168.1.10:9999", false},
		{"http://beeper.example.com", false},
		{"://bad", false},
	}
	for _, tt := range tests {
		if got := isLoopback(tt.baseURL); got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.baseURL, got, tt.want)
		}
	}
}
