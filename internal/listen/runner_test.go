package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/cluas/slackctl/internal/slack"
)

type fakeConnector struct {
	mu       sync.Mutex
	url      string
	err      error
	connects int
}

func (f *fakeConnector) RTMConnect() (slack.RTMInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connects++
	if f.err != nil {
		return slack.RTMInfo{}, f.err
	}
	return slack.RTMInfo{URL: f.url, SelfID: "U0SELF", SelfName: "me"}, nil
}
func (f *fakeConnector) WebsocketHTTPClient() *http.Client { return http.DefaultClient }
func (f *fakeConnector) WebsocketHeaders() http.Header     { return http.Header{} }
func (f *fakeConnector) ConversationIsMPIM(string) (bool, error) {
	return false, nil
}

// safeBuffer collects output lines from the listener goroutine.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) lines() []map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(l), &m)
		out = append(out, m)
	}
	return out
}

// newFakeRTMServer: first connection sends hello, a DM, a filtered channel
// message, then goodbye; later connections send hello and stay open while
// answering pings with pongs.
func newFakeRTMServer(t *testing.T) *httptest.Server {
	var mu sync.Mutex
	conns := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		mu.Lock()
		conns++
		n := conns
		mu.Unlock()
		ctx := r.Context()
		send := func(s string) { _ = c.Write(ctx, websocket.MessageText, []byte(s)) }
		send(`{"type":"hello"}`)
		if n == 1 {
			send(`{"type":"message","channel":"D1","user":"U9","text":"hi","ts":"1.0"}`)
			send(`{"type":"message","channel":"C1","user":"U9","text":"noise","ts":"2.0"}`)
			send(`{"type":"goodbye"}`)
			return
		}
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var ping struct {
				Type string `json:"type"`
				ID   int64  `json:"id"`
			}
			if json.Unmarshal(data, &ping) == nil && ping.Type == "ping" {
				b, _ := json.Marshal(map[string]any{"type": "pong", "reply_to": ping.ID})
				_ = c.Write(ctx, websocket.MessageText, b)
			}
		}
	}))
}

func TestRunReconnectsAfterGoodbye(t *testing.T) {
	srv := newFakeRTMServer(t)
	defer srv.Close()
	conn := &fakeConnector{url: "ws" + strings.TrimPrefix(srv.URL, "http")}
	out := &safeBuffer{}
	l := NewListener(conn, Filter{Types: ParseTypes("message"), MentionsOrDMs: true, IncludeBots: true}, out, &safeBuffer{})
	l.sleep = func(context.Context, time.Duration) {}
	l.SetPingInterval(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(out.lines()) < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond) // let a few ping/pong cycles pass
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned %v", err)
	}

	lines := out.lines()
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0]["type"] != "_connected" || lines[1]["channel"] != "D1" || lines[2]["type"] != "_reconnected" {
		t.Fatalf("unexpected sequence: %v", lines)
	}
	if lines[1]["is_dm"] != true {
		t.Fatalf("dm flag missing: %v", lines[1])
	}
}

func TestRunAuthErrorExits(t *testing.T) {
	conn := &fakeConnector{err: errors.New("Slack API error: invalid_auth")}
	out := &safeBuffer{}
	l := NewListener(conn, Filter{}, out, &safeBuffer{})
	err := l.Run(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	lines := out.lines()
	if len(lines) != 1 || lines[0]["type"] != "_auth_error" {
		t.Fatalf("want one _auth_error line, got %v", lines)
	}
}

func TestRunRetriesTransientConnectErrors(t *testing.T) {
	conn := &fakeConnector{err: errors.New("dial tcp: i/o timeout")}
	var slept []time.Duration
	ctx, cancel := context.WithCancel(context.Background())
	l := NewListener(conn, Filter{}, &safeBuffer{}, &safeBuffer{})
	l.sleep = func(_ context.Context, d time.Duration) {
		slept = append(slept, d)
		if len(slept) == 4 {
			cancel()
		}
	}
	if err := l.Run(ctx); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for i, d := range want {
		if slept[i] != d {
			t.Fatalf("sleep[%d]=%s want %s (all %v)", i, slept[i], d, slept)
		}
	}
}
