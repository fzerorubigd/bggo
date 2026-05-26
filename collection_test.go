package bggo_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetCollection_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	username := os.Getenv("BGG_USERNAME")
	if apiKey == "" || username == "" {
		t.Skip("BGG_API_KEY or BGG_USERNAME not set")
	}

	c := bggo.NewClient(apiKey)

	items, err := c.GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: username,
		Statuses: []bggo.CollectionStatus{bggo.CollectionOwn},
	})
	require.NoError(t, err)
	require.NotEmpty(t, items)

	for _, item := range items {
		t.Logf("ID=%d Name=%q Type=%s Year=%d Plays=%d Status=%v",
			item.ID, item.Name, item.Type, item.YearPublished, item.NumPlays, item.Status)
	}

	assert.Contains(t, items[0].Status, bggo.CollectionOwn)
}

// collectionFixture serves a canned XML body for /xmlapi2/collection
// + captures the requested query string so tests can verify both
// the response decoding AND the request parameters.
type collectionFixture struct {
	server       *httptest.Server
	lastRawQuery string
	body         string
	status       int
}

func newCollectionFixture(t *testing.T, body string) *collectionFixture {
	t.Helper()
	f := &collectionFixture{body: body, status: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastRawQuery = r.URL.RawQuery
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *collectionFixture) client(t *testing.T) *bggo.Client {
	t.Helper()
	u, err := url.Parse(f.server.URL)
	require.NoError(t, err)
	return bggo.NewClient("fixture-key",
		bggo.WithHost(u.Host),
		bggo.WithScheme(u.Scheme),
	)
}

// TestGetCollection_ParsesRatingFromStatsBlock pins the
// stats=1-augmented response: <stats><rating value="8.5"/></stats>
// resolves to a non-nil pointer at the numeric value.
func TestGetCollection_ParsesRatingFromStatsBlock(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<items totalitems="1" termsofuse="https://example.invalid/terms">
  <item objectid="42" subtype="boardgame" collid="100">
    <name>Placeholder Game</name>
    <yearpublished>2099</yearpublished>
    <status own="1" prevowned="0" fortrade="0" want="0" wanttoplay="0" wanttobuy="0" wishlist="0" preordered="0" />
    <numplays>3</numplays>
    <stats><rating value="8.5"/></stats>
  </item>
</items>`
	f := newCollectionFixture(t, body)
	items, err := f.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: "operator",
		Stats:    true,
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Rating)
	assert.InDelta(t, 8.5, *items[0].Rating, 0.0001)
	assert.Contains(t, f.lastRawQuery, "stats=1")
}

// TestGetCollection_RatingNAResolvesNil pins the unrated case:
// BGG renders unrated items as value="N/A". A non-nil pointer
// here would force callers to special-case the sentinel; nil is
// the correct "no rating" representation.
func TestGetCollection_RatingNAResolvesNil(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<items totalitems="1">
  <item objectid="42" subtype="boardgame">
    <name>Placeholder</name>
    <status own="1" />
    <numplays>0</numplays>
    <stats><rating value="N/A"/></stats>
  </item>
</items>`
	f := newCollectionFixture(t, body)
	items, err := f.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: "operator",
		Stats:    true,
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].Rating, "rating value=N/A must decode to nil pointer")
}

// TestGetCollection_ParsesCommentAndPrivateInfo pins the
// showprivate=1 augmented response: comment is on the item,
// privateinfo attrs land on CollectionPrivateInfo, and the
// nested <privatecomment> child resolves.
func TestGetCollection_ParsesCommentAndPrivateInfo(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<items totalitems="1">
  <item objectid="42" subtype="boardgame">
    <name>Placeholder</name>
    <status own="1" />
    <numplays>5</numplays>
    <comment>Public comment here.</comment>
    <privateinfo pricepaid="49.99" pp_currency="USD" acquisitiondate="2099-01-15" acquiredfrom="local-shop" inventorylocation="shelf-A">
      <privatecomment>Private comment here.</privatecomment>
    </privateinfo>
  </item>
</items>`
	f := newCollectionFixture(t, body)
	items, err := f.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username:    "operator",
		ShowPrivate: true,
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Public comment here.", items[0].Comment)
	require.NotNil(t, items[0].PrivateInfo)
	assert.Equal(t, "49.99", items[0].PrivateInfo.PricePaid)
	assert.Equal(t, "USD", items[0].PrivateInfo.PriceCurrency)
	assert.Equal(t, "2099-01-15", items[0].PrivateInfo.AcquisitionDate)
	assert.Equal(t, "local-shop", items[0].PrivateInfo.AcquiredFrom)
	assert.Equal(t, "shelf-A", items[0].PrivateInfo.InventoryLocation)
	assert.Equal(t, "Private comment here.", items[0].PrivateInfo.PrivateComment)
	assert.Contains(t, f.lastRawQuery, "showprivate=1")
}

// TestGetCollection_PrivateInfoAbsentWhenWireMissing pins the
// negative path: an item without a <privateinfo> element (the
// anonymous / other-user response shape) lands as nil, not a
// zero-valued struct that callers must distinguish from a real
// privateinfo block.
func TestGetCollection_PrivateInfoAbsentWhenWireMissing(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<items totalitems="1">
  <item objectid="42" subtype="boardgame">
    <name>Placeholder</name>
    <status own="1" />
    <numplays>0</numplays>
  </item>
</items>`
	f := newCollectionFixture(t, body)
	items, err := f.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: "operator",
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].PrivateInfo)
	assert.Nil(t, items[0].Rating)
	assert.Empty(t, items[0].Comment)
}

// TestGetCollection_StatsAndShowPrivateBothEmit pins that both
// query params appear when both flags are set.
func TestGetCollection_StatsAndShowPrivateBothEmit(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?><items totalitems="0"></items>`
	f := newCollectionFixture(t, body)
	_, err := f.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username:    "operator",
		Stats:       true,
		ShowPrivate: true,
	})
	require.NoError(t, err)
	assert.Contains(t, f.lastRawQuery, "stats=1")
	assert.Contains(t, f.lastRawQuery, "showprivate=1")
}

// TestGetCollection_StatsAndShowPrivateOmittedByDefault pins the
// back-compat path: neither query param appears when the flags
// are unset, so existing callers that depend on today's wire
// shape don't see surprise params.
func TestGetCollection_StatsAndShowPrivateOmittedByDefault(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?><items totalitems="0"></items>`
	f := newCollectionFixture(t, body)
	_, err := f.client(t).GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: "operator",
	})
	require.NoError(t, err)
	assert.NotContains(t, f.lastRawQuery, "stats=")
	assert.NotContains(t, f.lastRawQuery, "showprivate=")
}

// TestClient_CookiesAccessor pins the read-after-WithCookies
// path: cookies registered via the option are visible through
// the Cookies() accessor + the returned slice is a defensive
// copy that callers can mutate without affecting the client.
func TestClient_CookiesAccessor(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "SessionID", Value: "session-value-here"},
		{Name: "bggusername", Value: "operator"},
	}
	c := bggo.NewClient("fixture-key", bggo.WithCookies("operator", cookies))

	got := c.Cookies()
	require.Len(t, got, 2)
	names := []string{got[0].Name, got[1].Name}
	assert.ElementsMatch(t, []string{"SessionID", "bggusername"}, names)
	assert.Equal(t, "operator", c.Username())

	got[0].Name = "MUTATED"
	again := c.Cookies()
	for _, ck := range again {
		assert.NotEqual(t, "MUTATED", ck.Name, "Cookies() must return a defensive copy")
	}
}

// TestClient_CookiesEmptyWhenNoSession pins that an unauthenticated
// client returns nil from Cookies() rather than an empty slice,
// matching idiomatic Go nil-vs-empty semantics for "no session."
func TestClient_CookiesEmptyWhenNoSession(t *testing.T) {
	c := bggo.NewClient("fixture-key")
	assert.Nil(t, c.Cookies())
	assert.Empty(t, c.Username())
}

// TestGetCollection_HTTPStatusError_WrapsStatus pins the typed-
// error contract: a non-200 / non-202 response from the
// collection endpoint surfaces as `*bggo.HTTPStatusError` via
// `errors.As`, carrying the integer status code so callers can
// branch on 401 (re-login + retry) vs 5xx / other 4xx.
func TestGetCollection_HTTPStatusError_WrapsStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	c := bggo.NewClient("fixture-key", bggo.WithHost(u.Host), bggo.WithScheme(u.Scheme))

	_, err = c.GetCollection(context.Background(), bggo.GetCollectionRequest{Username: "operator"})
	require.Error(t, err)
	var statusErr *bggo.HTTPStatusError
	require.True(t, errors.As(err, &statusErr), "error must unwrap to *HTTPStatusError")
	assert.Equal(t, http.StatusUnauthorized, statusErr.StatusCode)
	assert.Contains(t, statusErr.Status, "401")
	assert.Contains(t, statusErr.Error(), "unexpected status:",
		"Error() text must preserve the legacy `unexpected status:` prefix for log compatibility")
}

// TestHTTPStatusError_PreservesLegacyMessageShape pins that
// callers (logs / tests) that string-match on `unexpected
// status:` keep working under the typed-error addition.
func TestHTTPStatusError_PreservesLegacyMessageShape(t *testing.T) {
	t.Parallel()
	e := &bggo.HTTPStatusError{StatusCode: 503, Status: "503 Service Unavailable"}
	assert.Equal(t, "unexpected status: 503 Service Unavailable", e.Error())
}
