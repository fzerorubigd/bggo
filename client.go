package bggo

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultHost   = "boardgamegeek.com"
	defaultScheme = "https"
)

// Limiter is a rate limiter interface compatible with go.uber.org/ratelimit.
// The package itself is not required, but can be used with this client.
type Limiter interface {
	// Take should block to make sure that the RPS is met.
	Take() time.Time
}

type noOpLimiter struct{}

func (noOpLimiter) Take() time.Time {
	return time.Time{}
}

// Client is the BGG API client.
type Client struct {
	apiKey  string
	host    string
	scheme  string
	client  *http.Client
	limiter Limiter

	cookies  []*http.Cookie
	username string

	// queuedDelay is how long to wait before the attempt-th retry of a
	// request answered with 202 Accepted. Tests shorten it.
	queuedDelay func(attempt int) time.Duration
	// queuedRetries is how many times a request answered with 202 Accepted
	// is sent again before giving up.
	queuedRetries int

	lock sync.RWMutex
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

// WithHost overrides the default host (boardgamegeek.com).
func WithHost(host string) Option {
	return func(c *Client) {
		c.host = host
	}
}

// WithScheme overrides the default scheme (https).
func WithScheme(scheme string) Option {
	return func(c *Client) {
		c.scheme = scheme
	}
}

// WithCookies sets pre-existing session cookies for an already logged-in user.
func WithCookies(username string, cookies []*http.Cookie) Option {
	return func(c *Client) {
		c.cookies = cookies
		c.username = username
	}
}

// Cookies returns a deep-copy snapshot of the session cookies currently
// held by the client. Callers that persist a cookie jar across daemon
// restarts should snapshot here after a successful Login and restore via
// WithCookies on the next Client construction. The returned slice and
// each *http.Cookie are independent of the client's state — mutating
// either does not affect the client.
func (c *Client) Cookies() []*http.Cookie {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if len(c.cookies) == 0 {
		return nil
	}
	out := make([]*http.Cookie, len(c.cookies))
	for i, src := range c.cookies {
		cp := *src
		out[i] = &cp
	}
	return out
}

// Username returns the username currently associated with the session,
// populated by Login or WithCookies. Empty when the client has no session.
func (c *Client) Username() string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.username
}

// DefaultQueuedRetries is how many times a request answered with 202
// Accepted is retried unless WithQueuedRetries says otherwise. With the
// default backoff the retries wait about three minutes in total.
const DefaultQueuedRetries = 10

// WithQueuedRetries sets how many times a GET request answered with 202
// Accepted (the response is queued and still being generated) is sent again
// before the method gives up with an *HTTPStatusError carrying 202. Zero
// means no retry. A negative value keeps retrying until the context is done.
func WithQueuedRetries(n int) Option {
	return func(c *Client) {
		c.queuedRetries = n
	}
}

// WithLimiter sets a rate limiter to throttle API calls.
func WithLimiter(limiter Limiter) Option {
	return func(c *Client) {
		c.limiter = limiter
	}
}

// NewClient creates a new BGG API client. The apiKey is required by BGG.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey: apiKey,
		host:   defaultHost,
		scheme: defaultScheme,
		client: &http.Client{
			Transport: http.DefaultTransport,
		},
		limiter:       noOpLimiter{},
		queuedDelay:   queuedBackoff,
		queuedRetries: DefaultQueuedRetries,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) buildURL(path string, args map[string]string) string {
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   path,
	}

	q := u.Query()
	for k, v := range args {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

func (c *Client) buildFullURL(rawURL string, args map[string]string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := u.Query()
	for k, v := range args {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	c.limiter.Take()

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.lock.RLock()
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	c.lock.RUnlock()

	return c.client.Do(req)
}

// doQueued sends a GET request and, while BGG answers 202 Accepted (the
// response is queued and still being generated), waits and sends it again,
// up to the client's queuedRetries times or until the request's context is
// done. Running out of retries returns an *HTTPStatusError with the 202; any
// other response is returned with checking its status left to the caller.
// Each attempt sends a fresh clone, so headers and cookies are not added twice.
func (c *Client) doQueued(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for attempt := 1; ; attempt++ {
		resp, err := c.do(req.Clone(ctx))
		if err != nil {
			return nil, fmt.Errorf("http call: %w", err)
		}
		if resp.StatusCode != http.StatusAccepted {
			return resp, nil
		}
		resp.Body.Close()
		if c.queuedRetries >= 0 && attempt > c.queuedRetries {
			return nil, newHTTPStatusError(resp)
		}

		timer := time.NewTimer(c.queuedDelay(attempt))
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
}

// queuedBackoff waits 2s before the first retry and grows by one second more
// each time (2s, 4s, 7s, 11s, ...), capped at 30s.
func queuedBackoff(attempt int) time.Duration {
	return min(time.Duration(1+attempt*(attempt+1)/2)*time.Second, 30*time.Second)
}

// HTTPStatusError wraps a non-success BGG API response, carrying
// the HTTP status code + status text so callers can branch on the
// numeric code with `errors.As`. Methods that previously returned
// an unwrapped `unexpected status: ...` string now return this
// type wrapped via fmt.Errorf so existing error-message readers
// keep working and new callers can do:
//
//	var statusErr *bggo.HTTPStatusError
//	if errors.As(err, &statusErr) && statusErr.StatusCode == 401 {
//	    // re-login + retry
//	}
type HTTPStatusError struct {
	// StatusCode is the integer HTTP status (e.g. 401, 503).
	StatusCode int
	// Status is the textual status line from the response
	// (e.g. "401 Unauthorized"). Matches http.Response.Status.
	Status string
}

// Error implements the error interface with the same message
// shape callers saw pre-#-typed-error so logs / tests that
// string-match on "unexpected status:" keep working.
func (e *HTTPStatusError) Error() string {
	return "unexpected status: " + e.Status
}

// newHTTPStatusError constructs an HTTPStatusError from an
// http.Response. Callers should use this at the non-success
// branch in each API method so the typed wrapping is uniform.
func newHTTPStatusError(resp *http.Response) *HTTPStatusError {
	return &HTTPStatusError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
}

type bggError struct {
	XMLName xml.Name `xml:"error"`
	Message string   `xml:"message"`
}

func decodeXML(r io.Reader, dst any) error {
	buf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if err = xml.Unmarshal(buf, dst); err != nil {
		var bggErr bggError
		if xml.Unmarshal(buf, &bggErr) == nil && bggErr.Message != "" {
			return fmt.Errorf("bgg api error: %s", bggErr.Message)
		}
		return fmt.Errorf("xml decode: %w", err)
	}

	return nil
}
