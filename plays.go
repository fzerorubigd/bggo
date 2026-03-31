package bggo

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	playsPath     = "xmlapi2/plays"
	bggTimeFormat = "2006-01-02"
)

// GetPlaysRequest is the request for the GetPlays API.
// At least one of Username or GameID must be set.
type GetPlaysRequest struct {
	Username string
	GameID   int64
	MinDate  time.Time
	MaxDate  time.Time
	Page     int
}

// PlaysResult is the paginated result from the plays API.
type PlaysResult struct {
	Total    int64  `json:"total"`
	Page     int64  `json:"page"`
	Username string `json:"username"`
	UserID   int64  `json:"user_id"`
	Plays    []Play `json:"plays"`
}

// Play is a single logged play.
type Play struct {
	ID         int64         `json:"id"`
	Date       time.Time     `json:"date"`
	Quantity   int           `json:"quantity"`
	Length     time.Duration `json:"length"`
	Incomplete bool          `json:"incomplete"`
	NowInStats bool          `json:"now_in_stats"`
	Location   string        `json:"location,omitempty"`
	Comment    string        `json:"comment,omitempty"`
	Item       PlayItem      `json:"item"`
	Players    []Player      `json:"players,omitempty"`
}

// PlayItem identifies the game that was played.
type PlayItem struct {
	ID   int64    `json:"id"`
	Name string   `json:"name"`
	Type ItemType `json:"type"`
}

// Player is a participant in a play.
type Player struct {
	Username      string `json:"username,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	Name          string `json:"name,omitempty"`
	StartPosition string `json:"start_position,omitempty"`
	Color         string `json:"color,omitempty"`
	Score         int64  `json:"score"`
	New           bool   `json:"new"`
	Rating        string `json:"rating,omitempty"`
	Win           bool   `json:"win"`
}

// GetPlays fetches play records from BGG.
func (c *Client) GetPlays(ctx context.Context, req GetPlaysRequest) (*PlaysResult, error) {
	if req.Username == "" && req.GameID == 0 {
		return nil, errors.New("at least one of Username or GameID is required")
	}

	args := map[string]string{}
	if req.Username != "" {
		args["username"] = req.Username
	}
	if req.GameID > 0 {
		args["id"] = fmt.Sprint(req.GameID)
	}
	if req.Page > 0 {
		args["page"] = fmt.Sprint(req.Page)
	}
	if !req.MinDate.IsZero() {
		args["mindate"] = req.MinDate.Format(bggTimeFormat)
	}
	if !req.MaxDate.IsZero() {
		args["maxdate"] = req.MaxDate.Format(bggTimeFormat)
	}

	u := c.buildURL(playsPath, args)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	var raw xmlPlaysResponse
	if err = decodeXML(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	result := &PlaysResult{
		Total:    safeInt(raw.Total),
		Page:     safeInt(raw.Page),
		Username: raw.Username,
		UserID:   safeInt(raw.UserID),
		Plays:    make([]Play, len(raw.Play)),
	}

	for i, p := range raw.Play {
		play := Play{
			ID:         safeInt(p.ID),
			Date:       safeDate(p.Date),
			Quantity:   int(safeInt(p.Quantity)),
			Length:     time.Duration(safeInt(p.Length)) * time.Minute,
			Incomplete: p.Incomplete == "1",
			NowInStats: p.NowInStats == "1",
			Location:   p.Location,
			Comment:    p.Comments,
			Item: PlayItem{
				ID:   safeInt(p.Item.ObjectID),
				Name: p.Item.Name,
				Type: ItemType(p.Item.ObjectType),
			},
			Players: make([]Player, len(p.Players.Player)),
		}

		for j, plr := range p.Players.Player {
			play.Players[j] = Player{
				Username:      plr.Username,
				UserID:        plr.UserID,
				Name:          plr.Name,
				StartPosition: plr.StartPosition,
				Color:         plr.Color,
				Score:         safeInt(plr.Score),
				New:           plr.New == "1",
				Rating:        plr.Rating,
				Win:           plr.Win == "1",
			}
		}

		result.Plays[i] = play
	}

	return result, nil
}

// --- XML structures (private) ---

type xmlPlayPlayer struct {
	Username      string `xml:"username,attr"`
	UserID        string `xml:"userid,attr"`
	Name          string `xml:"name,attr"`
	StartPosition string `xml:"startposition,attr"`
	Color         string `xml:"color,attr"`
	Score         string `xml:"score,attr"`
	New           string `xml:"new,attr"`
	Rating        string `xml:"rating,attr"`
	Win           string `xml:"win,attr"`
}

type xmlPlay struct {
	ID         string `xml:"id,attr"`
	Date       string `xml:"date,attr"`
	Quantity   string `xml:"quantity,attr"`
	Length     string `xml:"length,attr"`
	Incomplete string `xml:"incomplete,attr"`
	NowInStats string `xml:"nowinstats,attr"`
	Location   string `xml:"location,attr"`
	Item       struct {
		Name       string `xml:"name,attr"`
		ObjectType string `xml:"objecttype,attr"`
		ObjectID   string `xml:"objectid,attr"`
	} `xml:"item"`
	Players struct {
		Player []xmlPlayPlayer `xml:"player"`
	} `xml:"players"`
	Comments string `xml:"comments"`
}

type xmlPlaysResponse struct {
	XMLName  xml.Name  `xml:"plays"`
	Username string    `xml:"username,attr"`
	UserID   string    `xml:"userid,attr"`
	Total    string    `xml:"total,attr"`
	Page     string    `xml:"page,attr"`
	Play     []xmlPlay `xml:"play"`
}

func safeDate(s string) time.Time {
	t, err := time.Parse(bggTimeFormat, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
