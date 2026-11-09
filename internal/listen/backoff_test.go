package listen

import (
	"errors"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	cases := map[int]time.Duration{
		-1: time.Second, 0: time.Second, 1: 2 * time.Second, 2: 4 * time.Second,
		5: 32 * time.Second, 6: 60 * time.Second, 7: 60 * time.Second, 100: 60 * time.Second,
	}
	for attempt, want := range cases {
		if got := Backoff(attempt); got != want {
			t.Errorf("Backoff(%d)=%s want %s", attempt, got, want)
		}
	}
}

func TestIsAuthError(t *testing.T) {
	for _, msg := range []string{"Slack API error: invalid_auth", "Slack API error: not_authed", "Slack API error: token_revoked"} {
		if !IsAuthError(errors.New(msg)) {
			t.Errorf("%q should be auth error", msg)
		}
	}
	for _, err := range []error{nil, errors.New("dial tcp: timeout"), errors.New("Slack API error: ratelimited")} {
		if IsAuthError(err) {
			t.Errorf("%v should not be auth error", err)
		}
	}
}
