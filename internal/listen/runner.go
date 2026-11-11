package listen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/cluas/slackctl/internal/slack"
)

const (
	DefaultPingInterval = 20 * time.Second
	dialTimeout         = 20 * time.Second
	writeTimeout        = 10 * time.Second
	readLimitBytes      = 8 << 20
	// outputQueueSize buffers events between the websocket loop and the writer
	// so a slow conversations.info lookup does not stall keepalive.
	outputQueueSize = 1024
)

// ErrAuth is returned by Run when Slack rejects the credentials.
var ErrAuth = errors.New("slack authentication failed")

// ErrUnsupported is returned by Run when Slack refuses rtm.connect for a
// reason that retrying cannot fix (e.g. an Enterprise Grid org-level session).
var ErrUnsupported = errors.New("real-time connection not available")

// ErrOutput is returned by Run when writing an event to stdout fails.
var ErrOutput = errors.New("write output failed")

// Connector is the subset of *slack.Client used by the listener.
type Connector interface {
	RTMConnect() (slack.RTMInfo, error)
	WebsocketHTTPClient() *http.Client
	WebsocketHeaders() http.Header
	ConversationIsMPIM(channelID string) (bool, error)
}

// Listener maintains an RTM websocket connection and emits JSON Lines.
type Listener struct {
	conn         Connector
	filter       Filter
	out          *json.Encoder
	log          io.Writer
	pingInterval time.Duration
	sleep        func(ctx context.Context, d time.Duration)

	self       slack.RTMInfo
	everHello  bool
	nextPingID int64

	// Owned by the writer goroutine (see writeLoop).
	ctx       context.Context
	queue     chan outItem
	mpimCache map[string]bool
}

// outItem is one unit of output: either an event still to be filtered and
// projected by the writer, or a ready-made control record.
type outItem struct {
	ev     *RawEvent
	selfID string
	line   any
}

// NewListener creates a listener writing events to out and diagnostics to log.
func NewListener(conn Connector, filter Filter, out, log io.Writer) *Listener {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	l := &Listener{
		conn:         conn,
		filter:       filter,
		out:          enc,
		log:          log,
		pingInterval: DefaultPingInterval,
		sleep:        sleepCtx,
		ctx:          context.Background(),
		mpimCache:    map[string]bool{},
	}
	if filter.MentionsOrDMs && filter.IsMPIM == nil {
		l.filter.IsMPIM = l.resolveMPIM
	}
	return l
}

// SetPingInterval overrides the keepalive interval (mainly for tests).
func (l *Listener) SetPingInterval(d time.Duration) { l.pingInterval = d }

// Run connects and streams events until ctx is cancelled (returns nil),
// authentication fails (returns ErrAuth), Slack refuses rtm.connect
// permanently (returns ErrUnsupported), or writing output fails (returns
// ErrOutput). All other errors are logged and followed by an
// exponential-backoff reconnect.
//
// The websocket loop only reads frames and answers keepalive; filtering,
// conversations.info lookups and writing happen on a separate writer
// goroutine, fed in order through a queue.
func (l *Listener) Run(parent context.Context) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	l.ctx = ctx
	l.queue = make(chan outItem, outputQueueSize)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		l.writeLoop(ctx, cancel)
	}()

	err := l.connectLoop(ctx)
	close(l.queue)
	<-writerDone
	if cause := context.Cause(ctx); errors.Is(cause, ErrOutput) {
		return cause
	}
	return err
}

func (l *Listener) connectLoop(ctx context.Context) error {
	attempt := 0
	reconnectURL := ""
	for ctx.Err() == nil {
		wsURL := reconnectURL
		reconnectURL = ""
		if wsURL == "" {
			info, err := l.conn.RTMConnect()
			if err != nil {
				if IsAuthError(err) {
					l.enqueue(ctx, outItem{line: controlRecord{Type: "_auth_error", Error: err.Error(), At: nowRFC3339()}})
					return fmt.Errorf("%w: %v", ErrAuth, err)
				}
				if IsUnsupportedError(err) {
					l.enqueue(ctx, outItem{line: controlRecord{Type: "_connect_error", Error: err.Error(), At: nowRFC3339()}})
					return fmt.Errorf("%w: %v", ErrUnsupported, err)
				}
				l.logf("rtm.connect failed: %v", err)
				l.backoff(ctx, &attempt)
				continue
			}
			l.self = info
			wsURL = info.URL
		}
		res := l.session(ctx, wsURL, attempt)
		if res.gotHello {
			attempt = 0
		}
		reconnectURL = res.reconnectURL
		if ctx.Err() != nil {
			return nil
		}
		l.logf("connection closed: %v", res.err)
		l.backoff(ctx, &attempt)
	}
	return nil
}

type sessionResult struct {
	gotHello     bool
	reconnectURL string
	err          error
}

func (l *Listener) session(ctx context.Context, wsURL string, attempt int) sessionResult {
	var res sessionResult
	dialCtx, cancelDial := context.WithTimeout(ctx, dialTimeout)
	c, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPClient: l.conn.WebsocketHTTPClient(),
		HTTPHeader: l.conn.WebsocketHeaders(),
	})
	cancelDial()
	if err != nil {
		res.err = fmt.Errorf("websocket dial: %w", err)
		return res
	}
	defer c.CloseNow()
	c.SetReadLimit(readLimitBytes)

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	frames, readErr := readLoop(sctx, c)

	ticker := time.NewTicker(l.pingInterval)
	defer ticker.Stop()
	var pendingPing int64

	for {
		select {
		case <-ctx.Done():
			_ = c.Close(websocket.StatusNormalClosure, "")
			res.err = ctx.Err()
			return res
		case err := <-readErr:
			res.err = fmt.Errorf("read: %w", err)
			return res
		case data := <-frames:
			ev, err := ParseEvent(data)
			if err != nil {
				l.logf("bad frame: %v", err)
				continue
			}
			switch ev.Type {
			case "hello":
				res.gotHello = true
				l.emitConnected(ctx, attempt)
			case "pong":
				if ev.ReplyTo != nil && *ev.ReplyTo == pendingPing {
					pendingPing = 0
				}
			case "goodbye":
				res.err = errors.New("server sent goodbye")
				return res
			case "reconnect_url":
				res.reconnectURL = ev.URL
			case "error":
				l.logf("server error event: %s", string(data))
			default:
				l.enqueue(ctx, outItem{ev: &ev, selfID: l.self.SelfID})
			}
		case <-ticker.C:
			if pendingPing != 0 {
				res.err = fmt.Errorf("no pong for ping %d within %s", pendingPing, l.pingInterval)
				return res
			}
			l.nextPingID++
			pendingPing = l.nextPingID
			if err := writeJSON(sctx, c, map[string]any{"type": "ping", "id": pendingPing}); err != nil {
				res.err = fmt.Errorf("ping: %w", err)
				return res
			}
		}
	}
}

func readLoop(ctx context.Context, c *websocket.Conn) (<-chan []byte, <-chan error) {
	frames := make(chan []byte)
	errc := make(chan error, 1)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				errc <- err
				return
			}
			select {
			case frames <- data:
			case <-ctx.Done():
				return
			}
		}
	}()
	return frames, errc
}

func writeJSON(ctx context.Context, c *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.Write(wctx, websocket.MessageText, data)
}

func (l *Listener) emitConnected(ctx context.Context, attempt int) {
	typ := "_connected"
	if l.everHello {
		typ = "_reconnected"
	}
	l.everHello = true
	self := l.self
	l.enqueue(ctx, outItem{line: controlRecord{Type: typ, Self: &self, Via: "rtm.connect", Attempts: attempt, At: nowRFC3339()}})
}

// controlRecord is a synthetic line (_connected / _reconnected / _auth_error /
// _connect_error).
type controlRecord struct {
	Type     string         `json:"type"`
	Self     *slack.RTMInfo `json:"self,omitempty"`
	Via      string         `json:"via,omitempty"`
	Attempts int            `json:"attempts,omitempty"`
	Error    string         `json:"error,omitempty"`
	At       string         `json:"at"`
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// enqueue hands an item to the writer, giving up if ctx is done (shutdown or
// a writer failure).
func (l *Listener) enqueue(ctx context.Context, it outItem) {
	select {
	case l.queue <- it:
	case <-ctx.Done():
	}
}

// writeLoop filters and writes queued items in order. A write failure is
// terminal: it cancels Run with ErrOutput so supervisors see a non-zero exit.
func (l *Listener) writeLoop(ctx context.Context, cancel context.CancelCauseFunc) {
	for it := range l.queue {
		if ctx.Err() != nil {
			continue // shutting down: drain without writing
		}
		if err := l.write(it); err != nil {
			l.logf("write stdout: %v", err)
			cancel(fmt.Errorf("%w: %v", ErrOutput, err))
		}
	}
}

func (l *Listener) write(it outItem) error {
	line := it.line
	if it.ev != nil {
		f := l.filter
		f.SelfID = it.selfID
		selected, ok := f.Select(*it.ev)
		if !ok {
			return nil
		}
		line = selected
	}
	return l.out.Encode(line)
}

func (l *Listener) logf(format string, args ...any) {
	fmt.Fprintf(l.log, "[slackctl listen] %s "+format+"\n",
		append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}

func (l *Listener) backoff(ctx context.Context, attempt *int) {
	d := Backoff(*attempt)
	*attempt++
	l.logf("reconnecting in %s (attempt %d)", d, *attempt)
	l.sleep(ctx, d)
}

// resolveMPIM looks up (and caches) whether a channel is a group DM. Runs on
// the writer goroutine only.
func (l *Listener) resolveMPIM(channel string) bool {
	if v, ok := l.mpimCache[channel]; ok {
		return v
	}
	isMPIM, err := l.lookupMPIM(channel)
	if err != nil {
		// Not cached: a transient failure must not hide the channel for the whole session.
		if l.ctx.Err() == nil {
			l.logf("conversations.info %s: %v", channel, err)
		}
		return false
	}
	l.mpimCache[channel] = isMPIM
	return isMPIM
}

// lookupMPIM calls conversations.info but returns as soon as Run is
// cancelled; the Slack client has no context and may sleep on rate limits.
func (l *Listener) lookupMPIM(channel string) (bool, error) {
	type result struct {
		isMPIM bool
		err    error
	}
	done := make(chan result, 1)
	go func() {
		isMPIM, err := l.conn.ConversationIsMPIM(channel)
		done <- result{isMPIM, err}
	}()
	select {
	case r := <-done:
		return r.isMPIM, r.err
	case <-l.ctx.Done():
		return false, l.ctx.Err()
	}
}

// IsAuthError reports whether err means the credentials are no longer valid.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, code := range []string{"invalid_auth", "not_authed", "token_revoked", "token_expired", "account_inactive"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

// IsUnsupportedError reports whether rtm.connect was refused for a reason
// that will not go away on retry. enterprise_is_restricted is what Slack
// returns for Enterprise Grid org-level sessions.
func IsUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "enterprise_is_restricted")
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
