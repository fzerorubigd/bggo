package bggo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PostPlayRequest is the request for logging a new play.
type PostPlayRequest struct {
	GameID   int64
	GameType ItemType
	Date     time.Time
	Length   time.Duration
	Location string
	Comment  string
	Players  []PostPlayPlayer
}

// PostPlayPlayer is a player entry when posting a play.
type PostPlayPlayer struct {
	Username string
	UserID   int64
	Name     string
	Color    string
	Score    string
	Win      bool
	New      bool
}

// PostPlayResult is the response from logging a play.
type PostPlayResult struct {
	PlayID   int64 `json:"play_id"`
	NumPlays int   `json:"num_plays"`
}

// PostPlay logs a new play record. The client must be logged in first via Login.
func (c *Client) PostPlay(ctx context.Context, req PostPlayRequest) (*PostPlayResult, error) {
	c.lock.RLock()
	hasCookies := len(c.cookies) > 0
	c.lock.RUnlock()

	if !hasCookies {
		return nil, fmt.Errorf("call Login first")
	}

	payload := postPlayPayload{
		Playdate:   req.Date.Format(bggTimeFormat),
		Comments:   req.Comment,
		Length:     int(req.Length.Minutes()),
		Minutes:    int(req.Length.Minutes()),
		Location:   req.Location,
		ObjectID:   fmt.Sprint(req.GameID),
		ObjectType: string(req.GameType),
		Quantity:   1,
		Action:     "save",
		Date:       time.Now(),
		Ajax:       1,
	}

	for _, p := range req.Players {
		payload.Players = append(payload.Players, postPayloadPlayer{
			Name:     p.Name,
			Username: p.Username,
			UserID:   p.UserID,
			Color:    p.Color,
			Score:    p.Score,
			Win:      p.Win,
			New:      p.New,
		})
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	u := c.buildURL("geekplay.php", nil)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("post play failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var raw postPlayResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("bgg error: %s", raw.Error)
	}

	return &PostPlayResult{
		PlayID:   safeInt(raw.PlayID),
		NumPlays: raw.NumPlays,
	}, nil
}

// --- JSON structures (private) ---

type postPayloadPlayer struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	UserID   int64  `json:"userid,omitempty"`
	Color    string `json:"color"`
	Score    string `json:"score"`
	Win      bool   `json:"win,omitempty"`
	New      bool   `json:"new,omitempty"`
	Selected bool   `json:"selected"`
}

type postPlayPayload struct {
	Players    []postPayloadPlayer `json:"players"`
	Quantity   int                 `json:"quantity"`
	Date       time.Time           `json:"date"`
	Twitter    bool                `json:"twitter"`
	Location   string              `json:"location"`
	Minutes    int                 `json:"minutes"`
	Hours      int                 `json:"hours"`
	Incomplete bool                `json:"incomplete"`
	Comments   string              `json:"comments"`
	ObjectType string              `json:"objecttype"`
	ObjectID   string              `json:"objectid"`
	Playdate   string              `json:"playdate"`
	Length     int                 `json:"length"`
	Ajax       int                 `json:"ajax"`
	Action     string              `json:"action"`
}

type postPlayResponse struct {
	PlayID   string `json:"playid,omitempty"`
	NumPlays int    `json:"numplays,omitempty"`
	HTML     string `json:"html,omitempty"`
	Error    string `json:"error,omitempty"`
}
