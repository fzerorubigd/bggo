package bggo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
)

const thingPath = "xmlapi2/thing"

// GetThingsRequest is the request for the GetThings API.
type GetThingsRequest struct {
	IDs            []int64
	RankBreakDown  bool // fetch rating distribution (extra API call per thing)
}

// Recommendation represents a suggested player count rating.
type Recommendation int

const (
	NotRecommended Recommendation = iota
	RecommendedCount
	BestCount
)

func (r Recommendation) String() string {
	switch r {
	case BestCount:
		return "Best"
	case RecommendedCount:
		return "Recommended"
	case NotRecommended:
		return "Not Recommended"
	default:
		return fmt.Sprint(int(r))
	}
}

// SuggestedPlayerCount holds poll results for a given player count.
type SuggestedPlayerCount struct {
	NumPlayers     string `json:"num_players"`
	Best           int    `json:"best"`
	Recommended    int    `json:"recommended"`
	NotRecommended int    `json:"not_recommended"`
}

func percent(a, b, c int) float32 {
	sum := float32(a + b + c)
	if sum <= 0 {
		return 0
	}
	return (float32(a) / sum) * 100
}

// Suggestion returns the winning recommendation, its vote count, and percentage.
func (s *SuggestedPlayerCount) Suggestion() (Recommendation, int, float32) {
	if s.Recommended >= s.Best && s.Recommended > s.NotRecommended {
		return RecommendedCount, s.Recommended, percent(s.Recommended, s.Best, s.NotRecommended)
	}
	if s.Best > s.Recommended && s.Best > s.NotRecommended {
		return BestCount, s.Best, percent(s.Best, s.Recommended, s.NotRecommended)
	}
	return NotRecommended, s.NotRecommended, percent(s.NotRecommended, s.Best, s.Recommended)
}

// BestPercent returns the percentage of "Best" votes.
func (s *SuggestedPlayerCount) BestPercent() float32 {
	return percent(s.Best, s.Recommended, s.NotRecommended)
}

// RecommendedPercent returns the percentage of "Recommended" votes.
func (s *SuggestedPlayerCount) RecommendedPercent() float32 {
	return percent(s.Recommended, s.Best, s.NotRecommended)
}

// NotRecommendedPercent returns the percentage of "Not Recommended" votes.
func (s *SuggestedPlayerCount) NotRecommendedPercent() float32 {
	return percent(s.NotRecommended, s.Best, s.Recommended)
}

// RankBreakDown is the rating distribution (votes for ratings 1-10).
type RankBreakDown [10]int64

// Total returns the total number of votes.
func (rb RankBreakDown) Total() int64 {
	var total int64
	for _, v := range rb {
		total += v
	}
	return total
}

// Average returns the arithmetic average rating.
func (rb RankBreakDown) Average() float64 {
	var total int64
	var sum float64
	for i, v := range rb {
		total += v
		sum += float64(int64(i+1) * v)
	}
	return sum / float64(total)
}

// BayesianAverage returns the Bayesian average with the given number of dummy votes at 5.5.
func (rb RankBreakDown) BayesianAverage(added int64) float64 {
	m := float64(added) * 5.5
	total := added
	for i, v := range rb {
		m += float64(i+1) * float64(v)
		total += v
	}
	return m / float64(total)
}

// FamilyRank is the ranking within a BGG family (e.g. strategy games).
type FamilyRank struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	FriendlyName string  `json:"friendly_name"`
	Rank         int     `json:"rank"`
	BayesAverage float64 `json:"bayes_average"`
}

// ThingResult is the parsed result for a single thing from the BGG API.
type ThingResult struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	AlternateNames []string `json:"alternate_names,omitempty"`
	Type           ItemType `json:"type"`
	YearPublished  int      `json:"year_published"`

	Thumbnail string `json:"thumbnail,omitempty"`
	Image     string `json:"image,omitempty"`

	Description string `json:"description,omitempty"`

	MinPlayers int `json:"min_players"`
	MaxPlayers int `json:"max_players"`
	MinAge     int `json:"min_age"`

	PlayingTime int `json:"playing_time"`
	MinPlayTime int `json:"min_play_time"`
	MaxPlayTime int `json:"max_play_time"`

	SuggestedPlayerCount []SuggestedPlayerCount `json:"suggested_player_count,omitempty"`

	Links map[string][]Link `json:"links,omitempty"`

	UsersRated    int     `json:"users_rated"`
	AverageRate   float64 `json:"average_rate"`
	BayesAverage  float64 `json:"bayes_average"`
	AverageWeight float64 `json:"average_weight"`

	Rank          int                   `json:"rank"`
	Family        map[string]FamilyRank `json:"family,omitempty"`
	RankBreakDown *RankBreakDown        `json:"rank_break_down,omitempty"`
}

// Categories returns the board game category links.
func (t *ThingResult) Categories() []Link { return t.Links[LinkCategory] }

// Mechanics returns the board game mechanic links.
func (t *ThingResult) Mechanics() []Link { return t.Links[LinkMechanic] }

// Families returns the board game family links.
func (t *ThingResult) Families() []Link { return t.Links[LinkFamily] }

// Designers returns the board game designer links.
func (t *ThingResult) Designers() []Link { return t.Links[LinkDesigner] }

// Artists returns the board game artist links.
func (t *ThingResult) Artists() []Link { return t.Links[LinkArtist] }

// Publishers returns the board game publisher links.
func (t *ThingResult) Publishers() []Link { return t.Links[LinkPublisher] }

// GetThings fetches one or more things by their BGG IDs.
func (c *Client) GetThings(ctx context.Context, req GetThingsRequest) ([]ThingResult, error) {
	if len(req.IDs) == 0 {
		return nil, errors.New("at least one ID is required")
	}
	if len(req.IDs) > 20 {
		return nil, errors.New("BGG limits requests to 20 items at a time")
	}

	ids := make([]string, len(req.IDs))
	for i, id := range req.IDs {
		ids[i] = fmt.Sprint(id)
	}

	u := c.buildURL(thingPath, map[string]string{
		"id":    strings.Join(ids, ","),
		"stats": "1",
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	var raw xmlThingItems
	if err = decodeXML(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := convertThings(raw.Item)

	if req.RankBreakDown {
		for i := range results {
			rbd, err := c.fetchRankBreakDown(ctx, results[i].ID)
			if err != nil {
				return nil, fmt.Errorf("rank breakdown for %d: %w", results[i].ID, err)
			}
			results[i].RankBreakDown = &rbd
		}
	}

	return results, nil
}

// --- XML structures (private) ---

type xmlSimpleString struct {
	Value string `xml:"value,attr"`
}

type xmlNameStruct struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

type xmlLinkStruct struct {
	Type  string `xml:"type,attr"`
	ID    int64  `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

type xmlPollResult struct {
	Value    string `xml:"value,attr"`
	NumVotes int    `xml:"numvotes,attr"`
}

type xmlPollResults struct {
	NumPlayers string          `xml:"numplayers,attr"`
	Result     []xmlPollResult `xml:"result"`
}

type xmlPoll struct {
	Name    string           `xml:"name,attr"`
	Results []xmlPollResults `xml:"results"`
}

type xmlRank struct {
	Type         string `xml:"type,attr"`
	ID           string `xml:"id,attr"`
	Name         string `xml:"name,attr"`
	FriendlyName string `xml:"friendlyname,attr"`
	Value        string `xml:"value,attr"`
	BayesAverage string `xml:"bayesaverage,attr"`
}

type xmlStatistics struct {
	Ratings struct {
		UsersRated    xmlSimpleString `xml:"usersrated"`
		Average       xmlSimpleString `xml:"average"`
		BayesAverage  xmlSimpleString `xml:"bayesaverage"`
		Ranks         struct {
			Rank []xmlRank `xml:"rank"`
		} `xml:"ranks"`
		AverageWeight xmlSimpleString `xml:"averageweight"`
	} `xml:"ratings"`
}

type xmlThingItem struct {
	Type          string          `xml:"type,attr"`
	ID            int64           `xml:"id,attr"`
	Thumbnail     string          `xml:"thumbnail"`
	Image         string          `xml:"image"`
	Name          []xmlNameStruct `xml:"name"`
	Description   string          `xml:"description"`
	YearPublished xmlSimpleString `xml:"yearpublished"`
	MinPlayers    xmlSimpleString `xml:"minplayers"`
	MaxPlayers    xmlSimpleString `xml:"maxplayers"`
	MinAge        xmlSimpleString `xml:"minage"`
	PlayingTime   xmlSimpleString `xml:"playingtime"`
	MinPlayTime   xmlSimpleString `xml:"minplaytime"`
	MaxPlayTime   xmlSimpleString `xml:"maxplaytime"`
	Poll          []xmlPoll       `xml:"poll"`
	Link          []xmlLinkStruct `xml:"link"`
	Statistics    xmlStatistics   `xml:"statistics"`
}

type xmlThingItems struct {
	XMLName xml.Name       `xml:"items"`
	Item    []xmlThingItem `xml:"item"`
}

// --- conversion helpers ---

func convertThings(items []xmlThingItem) []ThingResult {
	out := make([]ThingResult, len(items))
	for i, item := range items {
		name, alts := extractNames(item.Name)
		out[i] = ThingResult{
			ID:                   item.ID,
			Name:                 name,
			AlternateNames:       alts,
			Type:                 ItemType(item.Type),
			YearPublished:        int(safeInt(item.YearPublished.Value)),
			Description:          html.UnescapeString(item.Description),
			Thumbnail:            item.Thumbnail,
			Image:                item.Image,
			MinPlayers:           int(safeInt(item.MinPlayers.Value)),
			MaxPlayers:           int(safeInt(item.MaxPlayers.Value)),
			MinAge:               int(safeInt(item.MinAge.Value)),
			PlayingTime:          int(safeInt(item.PlayingTime.Value)),
			MinPlayTime:          int(safeInt(item.MinPlayTime.Value)),
			MaxPlayTime:          int(safeInt(item.MaxPlayTime.Value)),
			SuggestedPlayerCount: extractSuggestedPlayers(item.Poll),
			Links:                extractLinks(item.Link),
			UsersRated:           int(safeInt(item.Statistics.Ratings.UsersRated.Value)),
			AverageRate:          safeFloat(item.Statistics.Ratings.Average.Value),
			BayesAverage:         safeFloat(item.Statistics.Ratings.BayesAverage.Value),
			AverageWeight:        safeFloat(item.Statistics.Ratings.AverageWeight.Value),
			Family:               make(map[string]FamilyRank),
		}

		for _, r := range item.Statistics.Ratings.Ranks.Rank {
			if r.Type == "subtype" && r.Name == "boardgame" {
				out[i].Rank = int(safeInt(r.Value))
				continue
			}
			if r.Type == "family" {
				out[i].Family[r.Name] = FamilyRank{
					ID:           safeInt(r.ID),
					Name:         r.Name,
					FriendlyName: r.FriendlyName,
					Rank:         int(safeInt(r.Value)),
					BayesAverage: safeFloat(r.BayesAverage),
				}
			}
		}
	}
	return out
}

func extractNames(names []xmlNameStruct) (string, []string) {
	var primary string
	var alts []string
	for _, n := range names {
		switch n.Type {
		case "primary":
			primary = n.Value
		case "alternate":
			alts = append(alts, n.Value)
		}
	}
	return primary, alts
}

func extractLinks(links []xmlLinkStruct) map[string][]Link {
	m := make(map[string][]Link)
	for _, l := range links {
		m[l.Type] = append(m[l.Type], Link{ID: l.ID, Name: l.Value})
	}
	return m
}

const rankPath = "api/collectionstatsgraph"

type rankBreakDownResponse struct {
	Data struct {
		Rows []struct {
			C []struct {
				V any `json:"v"`
			} `json:"c"`
		} `json:"rows"`
	} `json:"data"`
}

func (c *Client) fetchRankBreakDown(ctx context.Context, gameID int64) (RankBreakDown, error) {
	var rbd RankBreakDown

	u := c.buildURL(rankPath, map[string]string{
		"objectid":   fmt.Sprint(gameID),
		"objecttype": "thing",
		"type":       "BarChart",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return rbd, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return rbd, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return rbd, fmt.Errorf("read response: %w", err)
	}

	var raw rankBreakDownResponse
	if err = json.Unmarshal(data, &raw); err != nil {
		return rbd, fmt.Errorf("json decode: %w", err)
	}

	for _, row := range raw.Data.Rows {
		if len(row.C) < 2 {
			continue
		}
		num := safeIntInterface(row.C[0].V)
		val := safeIntInterface(row.C[1].V)
		if num >= 1 && num <= 10 {
			rbd[num-1] = val
		}
	}

	return rbd, nil
}

func extractSuggestedPlayers(polls []xmlPoll) []SuggestedPlayerCount {
	var out []SuggestedPlayerCount
	for _, p := range polls {
		if p.Name != "suggested_numplayers" {
			continue
		}
		for _, r := range p.Results {
			spc := SuggestedPlayerCount{NumPlayers: r.NumPlayers}
			for _, v := range r.Result {
				switch v.Value {
				case "Best":
					spc.Best = v.NumVotes
				case "Recommended":
					spc.Recommended = v.NumVotes
				case "Not Recommended":
					spc.NotRecommended = v.NumVotes
				}
			}
			out = append(out, spc)
		}
	}
	return out
}
