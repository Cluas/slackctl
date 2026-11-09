package listen

import "time"

const (
	BackoffBase = 1 * time.Second
	BackoffMax  = 60 * time.Second
)

// Backoff returns the delay before reconnect attempt n (0-based):
// 1s, 2s, 4s, ... capped at 60s.
func Backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := BackoffBase
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= BackoffMax {
			return BackoffMax
		}
	}
	return d
}
