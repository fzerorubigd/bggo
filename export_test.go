package bggo

import "time"

// WithQueuedDelay replaces the wait between retries of a 202 response, so
// tests do not sleep for seconds.
func WithQueuedDelay(delay func(attempt int) time.Duration) Option {
	return func(c *Client) {
		c.queuedDelay = delay
	}
}

// QueuedBackoff exposes the default wait for tests.
var QueuedBackoff = queuedBackoff
