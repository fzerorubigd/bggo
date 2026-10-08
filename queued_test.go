package bggo_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

// queuedServer answers 202 Accepted to the first `queued` requests and then
// replies with `status` and `body`. It records the Cookie header of every
// request it sees.
type queuedServer struct {
	server *httptest.Server
	queued int
	status int
	body   string

	mu      sync.Mutex
	hits    int
	cookies []string
}

func newQueuedServer(t *testing.T, queued, status int, body string) *queuedServer {
	t.Helper()
	s := &queuedServer{queued: queued, status: status, body: body}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits++
		hit := s.hits
		s.cookies = append(s.cookies, r.Header.Get("Cookie"))
		s.mu.Unlock()
		if hit <= s.queued {
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, "<message>Your request for this collection has been accepted and will be processed.</message>")
			return
		}
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.body)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *queuedServer) Hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

// redirect sends every request to the test server, whatever host the client
// built the URL for (some endpoints use a fixed host of their own).
type redirect struct{ target *url.URL }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	req.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func (s *queuedServer) client(t *testing.T, opts ...bggo.Option) *bggo.Client {
	t.Helper()
	u, err := url.Parse(s.server.URL)
	require.NoError(t, err)
	opts = append([]bggo.Option{
		bggo.WithHTTPClient(&http.Client{Transport: redirect{target: u}}),
		bggo.WithQueuedDelay(func(int) time.Duration { return time.Millisecond }),
	}, opts...)
	return bggo.NewClient("fixture-key", opts...)
}

// Every GET method, with a minimal body it decodes without error.
var queuedEndpoints = []struct {
	name string
	body string
	call func(context.Context, *bggo.Client) error
}{
	{"GetCollection", `<items totalitems="0"></items>`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.GetCollection(ctx, bggo.GetCollectionRequest{Username: "someone"})
		return err
	}},
	{"GetPlays", `<plays username="someone" userid="1" total="0" page="1"></plays>`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.GetPlays(ctx, bggo.GetPlaysRequest{Username: "someone"})
		return err
	}},
	{"GetThings", `<items></items>`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.GetThings(ctx, bggo.GetThingsRequest{IDs: []int64{1}})
		return err
	}},
	{"GetUser", `<user id="1" name="someone"></user>`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.GetUser(ctx, bggo.GetUserRequest{Username: "someone"})
		return err
	}},
	{"GetPerson", `<items><item type="boardgamedesigner" id="1"><name>x</name></item></items>`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.GetPerson(ctx, bggo.GetPersonRequest{ID: 1})
		return err
	}},
	{"Search", `<items total="0"></items>`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.Search(ctx, bggo.SearchRequest{Query: "x"})
		return err
	}},
	{"GetHotness", `{"items":[]}`, func(ctx context.Context, c *bggo.Client) error {
		_, err := c.GetHotness(ctx, bggo.GetHotnessRequest{})
		return err
	}},
}

func TestQueued_RetriesUntilReady(t *testing.T) {
	for _, ep := range queuedEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			s := newQueuedServer(t, 2, http.StatusOK, ep.body)
			require.NoError(t, ep.call(context.Background(), s.client(t)))
			assert.Equal(t, 3, s.Hits(), "two 202s, then the 200")
		})
	}
}

func TestQueued_ContextCancelledWhileWaiting(t *testing.T) {
	for _, ep := range queuedEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			s := newQueuedServer(t, 1000, http.StatusOK, ep.body)
			ctx, cancel := context.WithCancel(context.Background())
			// The wait never ends on its own; only the cancel can stop it.
			c := s.client(t, bggo.WithQueuedDelay(func(int) time.Duration {
				cancel()
				return time.Hour
			}))
			err := ep.call(ctx, c)
			require.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, 1, s.Hits())
		})
	}
}

func TestQueued_ErrorStatusAfterQueuedIsReturned(t *testing.T) {
	s := newQueuedServer(t, 1, http.StatusInternalServerError, "")
	_, err := s.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{Username: "someone"})

	var statusErr *bggo.HTTPStatusError
	require.True(t, errors.As(err, &statusErr), "got %v", err)
	assert.Equal(t, http.StatusInternalServerError, statusErr.StatusCode)
	assert.Equal(t, 2, s.Hits())
}

func TestQueued_RetrySendsCookiesOnce(t *testing.T) {
	s := newQueuedServer(t, 2, http.StatusOK, `<items totalitems="0"></items>`)
	c := s.client(t, bggo.WithCookies("someone", []*http.Cookie{{Name: "SessionID", Value: "abc"}}))
	_, err := c.GetCollection(context.Background(), bggo.GetCollectionRequest{Username: "someone"})
	require.NoError(t, err)

	require.Len(t, s.cookies, 3)
	for i, got := range s.cookies {
		assert.Equal(t, "SessionID=abc", got, "attempt %d", i+1)
	}
}

func TestQueuedBackoff(t *testing.T) {
	want := []time.Duration{2, 4, 7, 11, 16, 22, 29, 30, 30}
	for i, w := range want {
		assert.Equal(t, w*time.Second, bggo.QueuedBackoff(i+1), "attempt %d", i+1)
	}
}

func TestQueued_GivesUpAfterRetries(t *testing.T) {
	for _, ep := range queuedEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			s := newQueuedServer(t, 1000, http.StatusOK, ep.body)
			err := ep.call(context.Background(), s.client(t, bggo.WithQueuedRetries(3)))

			var statusErr *bggo.HTTPStatusError
			require.True(t, errors.As(err, &statusErr), "got %v", err)
			assert.Equal(t, http.StatusAccepted, statusErr.StatusCode)
			assert.Equal(t, 4, s.Hits(), "the first attempt and 3 retries")
		})
	}
}

func TestQueued_RetryLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    []bggo.Option
		queued  int
		wantErr bool
		want    int
	}{
		{"default gives up after DefaultQueuedRetries", nil, 1000, true, bggo.DefaultQueuedRetries + 1},
		{"default is enough for a short queue", nil, bggo.DefaultQueuedRetries, false, bggo.DefaultQueuedRetries + 1},
		{"zero means no retry", []bggo.Option{bggo.WithQueuedRetries(0)}, 1000, true, 1},
		{"negative retries until ready", []bggo.Option{bggo.WithQueuedRetries(-1)}, 25, false, 26},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newQueuedServer(t, tc.queued, http.StatusOK, `<items totalitems="0"></items>`)
			_, err := s.client(t, tc.opts...).GetCollection(context.Background(), bggo.GetCollectionRequest{Username: "someone"})
			if tc.wantErr {
				var statusErr *bggo.HTTPStatusError
				require.True(t, errors.As(err, &statusErr), "got %v", err)
				assert.Equal(t, http.StatusAccepted, statusErr.StatusCode)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, s.Hits())
		})
	}
}
