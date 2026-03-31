package bggo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	hotnessURL      = "https://api.geekdo.com/api/hotness"
	geekListURL     = "https://api.geekdo.com/api/listitems"
	trendOwnership  = "https://api.geekdo.com/api/trends/ownership"
	trendPlays      = "https://api.geekdo.com/api/trends/plays"
	trendPlaysDelta = "https://api.geekdo.com/api/trends/plays_delta"
)

// TrendInterval is the time interval for trend queries.
type TrendInterval string

const (
	TrendWeek  TrendInterval = "week"
	TrendMonth TrendInterval = "month"
)

// ListItem is an item from a BGG geek list.
type ListItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GetID implements IDGetter.
func (l ListItem) GetID() int64 { return l.ID }

// HotnessItem is an item from the BGG hotness list.
type HotnessItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Rank        int    `json:"rank"`
	Delta       int    `json:"delta"`
}

// GetID implements IDGetter.
func (h HotnessItem) GetID() int64 { return h.ID }

// TrendItem is an item from a BGG trend list (best sellers, most played, trending).
type TrendItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Rank        int    `json:"rank"`
	Delta       int    `json:"delta"`
	Appearances int    `json:"appearances"`
}

// GetID implements IDGetter.
func (t TrendItem) GetID() int64 { return t.ID }

// GetHotnessRequest is the request for the Hotness API.
type GetHotnessRequest struct {
	Count int // 1-50, defaults to 50
}

// GetHotness returns the current hotness list from BGG.
func (c *Client) GetHotness(ctx context.Context, req GetHotnessRequest) ([]HotnessItem, error) {
	count := req.Count
	if count < 1 || count > 50 {
		count = 50
	}

	body, err := c.fetchJSON(ctx, hotnessURL, map[string]string{
		"geeksite":   "boardgame",
		"objecttype": "thing",
		"showcount":  fmt.Sprint(count),
	})
	if err != nil {
		return nil, fmt.Errorf("hotness: %w", err)
	}

	var raw jsonHotness
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("hotness decode: %w", err)
	}

	items := make([]HotnessItem, len(raw.Items))
	for i, h := range raw.Items {
		items[i] = HotnessItem{
			ID:          safeInt(h.ID),
			Name:        h.Name,
			Description: h.Description,
			Rank:        int(safeInt(h.Rank)),
			Delta:       h.Delta,
		}
	}
	return items, nil
}

// GetGeekListRequest is the request for the GeekList API.
type GetGeekListRequest struct {
	ListID int64
}

// GetGeekList returns all items from a BGG geek list.
func (c *Client) GetGeekList(ctx context.Context, req GetGeekListRequest) ([]ListItem, error) {
	var items []ListItem
	page := 1
	for {
		body, err := c.fetchJSON(ctx, geekListURL, map[string]string{
			"listid": fmt.Sprint(req.ListID),
			"page":   fmt.Sprint(page),
		})
		if err != nil {
			return nil, fmt.Errorf("geeklist: %w", err)
		}

		var raw jsonGeekList
		if err = json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("geeklist decode: %w", err)
		}

		if len(raw.Data) == 0 {
			break
		}

		for _, d := range raw.Data {
			items = append(items, ListItem{
				ID:          safeInt(d.Item.ID),
				Name:        d.Item.Name,
				Description: d.Body,
			})
		}
		page++
	}
	return items, nil
}

// GetTrendRequest is the request for trend APIs (BestSellers, MostPlayed, TrendingPlays).
type GetTrendRequest struct {
	Interval  TrendInterval
	StartDate time.Time
}

// GetBestSellers returns the best-selling games for the week containing StartDate.
func (c *Client) GetBestSellers(ctx context.Context, req GetTrendRequest) ([]TrendItem, error) {
	start := previousWeekday(req.StartDate, time.Monday)
	return c.fetchTrend(ctx, trendOwnership, TrendWeek, start)
}

// GetMostPlayed returns the most-played games for the given interval.
func (c *Client) GetMostPlayed(ctx context.Context, req GetTrendRequest) ([]TrendItem, error) {
	start := alignDate(req.StartDate, req.Interval)
	return c.fetchTrend(ctx, trendPlays, req.Interval, start)
}

// GetTrendingPlays returns trending plays (biggest delta) for the given interval.
func (c *Client) GetTrendingPlays(ctx context.Context, req GetTrendRequest) ([]TrendItem, error) {
	start := alignDate(req.StartDate, req.Interval)
	return c.fetchTrend(ctx, trendPlaysDelta, req.Interval, start)
}

func (c *Client) fetchTrend(ctx context.Context, baseURL string, interval TrendInterval, start time.Time) ([]TrendItem, error) {
	body, err := c.fetchJSON(ctx, baseURL, map[string]string{
		"interval":  string(interval),
		"startDate": start.Format("2006-01-02"),
	})
	if err != nil {
		return nil, fmt.Errorf("trend: %w", err)
	}

	var raw jsonTrends
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("trend decode: %w", err)
	}

	items := make([]TrendItem, len(raw.Items))
	for i, t := range raw.Items {
		items[i] = TrendItem{
			ID:          safeInt(t.Item.ID),
			Name:        t.Item.Name,
			Description: t.Description,
			Rank:        t.Rank,
			Delta:       t.Delta,
			Appearances: t.Appearances,
		}
	}
	return items, nil
}

// fetchJSON is a helper that GETs a URL and returns the raw body bytes.
func (c *Client) fetchJSON(ctx context.Context, baseURL string, params map[string]string) ([]byte, error) {
	u := c.buildFullURL(baseURL, params)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func previousWeekday(t time.Time, day time.Weekday) time.Time {
	diff := int(day) - int(t.Weekday())
	if diff > 0 {
		diff -= 7
	}
	return t.AddDate(0, 0, diff)
}

func alignDate(t time.Time, interval TrendInterval) time.Time {
	switch interval {
	case TrendMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return previousWeekday(t, time.Monday)
	}
}

// --- JSON structures (private) ---

type jsonHotnessItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Delta       int    `json:"delta"`
	Rank        string `json:"rank"`
}

type jsonHotness struct {
	Items []jsonHotnessItem `json:"items"`
}

type jsonGeekListEntry struct {
	Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"item"`
	Body string `json:"body"`
}

type jsonGeekList struct {
	Data []jsonGeekListEntry `json:"data"`
}

type jsonTrendItem struct {
	Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"item"`
	Rank        int    `json:"rank"`
	Description string `json:"description"`
	Delta       int    `json:"delta"`
	Appearances int    `json:"appearances"`
}

type jsonTrends struct {
	Items []jsonTrendItem `json:"items"`
}
