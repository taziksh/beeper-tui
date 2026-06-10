package ws

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/taziksh/beeper-tui/internal/config"
)

var defaultBackoff = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	30 * time.Second,
}

const (
	defaultPingInterval = 30 * time.Second
	dialTimeout         = 10 * time.Second
	writeTimeout        = 10 * time.Second

	// readLimit must fit a full event frame; upserts can carry several
	// complete message objects in entries.
	readLimit = 8 << 20
)

// Client maintains a connection to the Beeper Desktop events WebSocket,
// reconnecting with exponential backoff until Close is called.
type Client struct {
	url   string
	token string

	backoff      []time.Duration
	pingInterval time.Duration

	events  chan Event
	retryCh chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}

	mu      sync.Mutex
	conn    *websocket.Conn
	chatIDs []string
	reqSeq  int
}

// Option adjusts a Client at construction time.
type Option func(*Client)

// WithBackoff overrides the reconnect delay schedule. After the schedule is
// exhausted the final delay repeats.
func WithBackoff(schedule []time.Duration) Option {
	return func(c *Client) { c.backoff = schedule }
}

// WithPingInterval overrides how often the client pings to detect dead
// connections.
func WithPingInterval(d time.Duration) Option {
	return func(c *Client) { c.pingInterval = d }
}

// New constructs a Client from resolved config and starts connecting in the
// background. It subscribes to all chats until SetSubscriptions narrows the
// set.
func New(cfg config.Config, opts ...Option) *Client {
	c := &Client{
		url:          cfg.BaseURL + "/v1/ws",
		token:        cfg.Token,
		backoff:      defaultBackoff,
		pingInterval: defaultPingInterval,
		events:       make(chan Event, 64),
		retryCh:      make(chan struct{}, 1),
		done:         make(chan struct{}),
		chatIDs:      []string{"*"},
	}
	for _, opt := range opts {
		opt(c)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.run(ctx)
	return c
}

// Events returns the stream of domain events and connection-state changes.
// The channel is closed after Close.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Retry skips any pending backoff delay and reconnects immediately, resetting
// the backoff schedule. Safe to call at any time.
func (c *Client) Retry() {
	select {
	case c.retryCh <- struct{}{}:
	default:
	}
}

// SetSubscriptions changes which chats produce events: ["*"] for all chats,
// nil or empty to pause. The set is sent immediately if connected and re-sent
// after every reconnect.
func (c *Client) SetSubscriptions(chatIDs []string) {
	c.mu.Lock()
	c.chatIDs = append([]string{}, chatIDs...)
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	// Best effort: if the write fails the read loop notices the broken
	// connection and the reconnect re-sends the stored set.
	_ = c.sendSubscriptions(ctx, conn)
}

// Close shuts the client down and waits for the background goroutine to
// exit. The Events channel is closed before Close returns.
func (c *Client) Close() {
	c.cancel()
	<-c.done
}

// run is the reconnect loop: one session per iteration, then a backoff delay
// that Retry can cut short.
func (c *Client) run(ctx context.Context) {
	defer close(c.done)
	defer close(c.events)
	attempt := 0
	for {
		c.emit(ctx, Event{Type: EventConnecting})
		connected, err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if connected {
			attempt = 0
		}
		delay := c.backoff[min(attempt, len(c.backoff)-1)]
		attempt++
		c.emit(ctx, Event{Type: EventDisconnected, Err: err, RetryIn: delay})
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		case <-c.retryCh:
			attempt = 0
		}
	}
}

// session dials and reads frames until the connection fails. It reports
// whether the connection became fully established (ready received and
// subscriptions sent), which resets the backoff schedule.
func (c *Client) session(ctx context.Context) (connected bool, err error) {
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	conn, _, err := websocket.Dial(dialCtx, c.url, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + c.token}},
	})
	cancel()
	if err != nil {
		return false, fmt.Errorf("ws: dial %s: %w", c.url, err)
	}
	conn.SetReadLimit(readLimit)
	defer func() { _ = conn.CloseNow() }()

	c.setConn(conn)
	defer c.setConn(nil)

	sessionCtx, stop := context.WithCancel(ctx)
	defer stop()
	go c.pingLoop(sessionCtx, conn)

	for {
		_, data, err := conn.Read(sessionCtx)
		if err != nil {
			return connected, fmt.Errorf("ws: read: %w", err)
		}
		frame, err := decodeFrame(data)
		if err != nil {
			// Skip unknown or malformed frames rather than dropping a
			// healthy connection.
			continue
		}
		switch f := frame.(type) {
		case ready:
			if err := c.sendSubscriptions(sessionCtx, conn); err != nil {
				return connected, err
			}
			connected = true
			c.emit(sessionCtx, Event{Type: EventConnected})
		case subscriptionsUpdated:
			// Ack only; nothing to do.
		case serverError:
			// A rejected command is not a connection failure.
		case Event:
			c.emit(sessionCtx, f)
		}
	}
}

// pingLoop detects silently-dead connections. A failed ping closes the
// connection, which unblocks the session read loop into a reconnect.
func (c *Client) pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				_ = conn.CloseNow()
				return
			}
		}
	}
}

func (c *Client) sendSubscriptions(ctx context.Context, conn *websocket.Conn) error {
	c.mu.Lock()
	ids := c.chatIDs
	c.reqSeq++
	requestID := fmt.Sprintf("r%d", c.reqSeq)
	c.mu.Unlock()
	cmd := subscriptionsSet{Type: "subscriptions.set", RequestID: requestID, ChatIDs: ids}
	if err := wsjson.Write(ctx, conn, cmd); err != nil {
		return fmt.Errorf("ws: send subscriptions.set: %w", err)
	}
	return nil
}

func (c *Client) setConn(conn *websocket.Conn) {
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
}

func (c *Client) emit(ctx context.Context, e Event) {
	select {
	case c.events <- e:
	case <-ctx.Done():
	}
}
