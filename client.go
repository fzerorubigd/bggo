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
		limiter: noOpLimiter{},
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
