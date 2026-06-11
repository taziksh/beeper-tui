// Package launch starts Beeper Desktop when its API is not reachable.
package launch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

const (
	pollInterval = 500 * time.Millisecond
	startTimeout = 30 * time.Second
)

// EnsureRunning returns once the Beeper Desktop API at baseURL accepts
// connections. If the API is down and baseURL points at this machine, it
// launches Beeper Desktop hidden in the background and waits for the API
// to come up.
func EnsureRunning(ctx context.Context, baseURL string) error {
	probe := func(ctx context.Context) bool { return reachable(ctx, baseURL) }
	return ensureRunning(ctx, baseURL, probe, startApp, pollInterval, startTimeout)
}

func ensureRunning(ctx context.Context, baseURL string, probe func(context.Context) bool, start func() error, interval, timeout time.Duration) error {
	if probe(ctx) {
		return nil
	}
	if !isLoopback(baseURL) {
		return fmt.Errorf("launch: Beeper Desktop API at %s is not reachable", baseURL)
	}
	if err := start(); err != nil {
		return fmt.Errorf("launch: start Beeper Desktop: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
		if probe(ctx) {
			return nil
		}
	}
	return fmt.Errorf("launch: Beeper Desktop API did not come up within %s", timeout)
}

// reachable reports whether anything is serving HTTP at baseURL. Any
// response counts, including errors like 401 or 404; only a failed
// connection means the app is down.
func reachable(ctx context.Context, baseURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func isLoopback(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// startApp launches Beeper Desktop without raising a window: -g keeps it
// in the background and -j launches it hidden.
func startApp() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("automatic launch is only supported on macOS; start Beeper Desktop manually")
	}
	return exec.Command("open", "-g", "-j", "-a", "Beeper Desktop").Run()
}
