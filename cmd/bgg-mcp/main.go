package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fzerorubigd/bggo"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Input types ---

type SearchInput struct {
	Query string   `json:"query" jsonschema:"search query string"`
	Types []string `json:"types,omitempty" jsonschema:"item types to filter: boardgame, boardgameexpansion, boardgameaccessory, rpgitem, videogame"`
	Exact bool     `json:"exact,omitempty" jsonschema:"exact match only"`
}

type GetThingsInput struct {
	IDs           []int64 `json:"ids" jsonschema:"BGG thing IDs (max 20)"`
	RankBreakDown bool    `json:"rank_break_down,omitempty" jsonschema:"include rating distribution (extra API call per thing)"`
}

type GetCollectionInput struct {
	Username string   `json:"username" jsonschema:"BGG username"`
	Statuses []string `json:"statuses,omitempty" jsonschema:"filter by status: own, rated, played, comment, trade, want, wishlist, preorder, wanttoplay, wanttobuy, prevowned"`
}

type GetPlaysInput struct {
	Username string `json:"username,omitempty" jsonschema:"BGG username"`
	GameID   int64  `json:"game_id,omitempty" jsonschema:"filter by game ID"`
	MinDate  string `json:"min_date,omitempty" jsonschema:"earliest date (YYYY-MM-DD)"`
	MaxDate  string `json:"max_date,omitempty" jsonschema:"latest date (YYYY-MM-DD)"`
	Page     int    `json:"page,omitempty" jsonschema:"page number (100 plays per page)"`
}

type PostPlayInput struct {
	GameID   int64  `json:"game_id" jsonschema:"BGG game ID"`
	GameType string `json:"game_type,omitempty" jsonschema:"object type, usually thing"`
	Date     string `json:"date,omitempty" jsonschema:"play date (YYYY-MM-DD), defaults to today"`
	Length   int    `json:"length,omitempty" jsonschema:"play length in minutes"`
	Location string `json:"location,omitempty" jsonschema:"where the game was played"`
	Comment  string `json:"comment,omitempty" jsonschema:"play comment"`
	Players  []struct {
		Username string `json:"username,omitempty" jsonschema:"BGG username"`
		Name     string `json:"name,omitempty" jsonschema:"player display name"`
		Win      bool   `json:"win,omitempty" jsonschema:"whether the player won"`
		Score    string `json:"score,omitempty" jsonschema:"player score"`
		Color    string `json:"color,omitempty" jsonschema:"player color"`
		New      bool   `json:"new,omitempty" jsonschema:"first time playing"`
	} `json:"players,omitempty" jsonschema:"list of players"`
}

type GetHotnessInput struct {
	Count int `json:"count,omitempty" jsonschema:"number of items 1-50, defaults to 50"`
}

type GetGeekListInput struct {
	ListID int64 `json:"list_id" jsonschema:"BGG geek list ID"`
}

type GetTrendInput struct {
	Interval  string `json:"interval,omitempty" jsonschema:"week or month, defaults to week"`
	StartDate string `json:"start_date,omitempty" jsonschema:"start date (YYYY-MM-DD), defaults to last week"`
}

// --- Output types ---

type SearchOutput struct {
	Results []bggo.SearchResult `json:"results" jsonschema:"search results"`
}

type GetThingsOutput struct {
	Things []bggo.ThingResult `json:"things" jsonschema:"thing details"`
}

type GetCollectionOutput struct {
	Items []bggo.CollectionItem `json:"items" jsonschema:"collection items"`
}

type GetPlaysOutput struct {
	Total    int64       `json:"total" jsonschema:"total number of plays"`
	Page     int64       `json:"page" jsonschema:"current page"`
	Username string      `json:"username" jsonschema:"BGG username"`
	Plays    []bggo.Play `json:"plays" jsonschema:"play records"`
}

type PostPlayOutput struct {
	PlayID   int64 `json:"play_id" jsonschema:"the new play ID"`
	NumPlays int   `json:"num_plays" jsonschema:"total play count for this game"`
}

type GetHotnessOutput struct {
	Items []bggo.HotnessItem `json:"items" jsonschema:"hotness list items"`
}

type GetGeekListOutput struct {
	Items []bggo.ListItem `json:"items" jsonschema:"geek list items"`
}

type GetTrendOutput struct {
	Items []bggo.TrendItem `json:"items" jsonschema:"trend items"`
}

func main() {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		log.Fatal("BGG_API_KEY environment variable is required")
	}

	client := bggo.NewClient(apiKey)

	// Optional auto-login
	loggedIn := false
	if user, pass := os.Getenv("BGG_USERNAME"), os.Getenv("BGG_PASSWORD"); user != "" && pass != "" {
		if err := client.Login(context.Background(), bggo.LoginRequest{
			Username: user,
			Password: pass,
		}); err != nil {
			log.Printf("Warning: login failed: %v", err)
		} else {
			loggedIn = true
			log.Printf("Logged in as %s", user)
		}
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bgg-mcp",
		Version: "1.0.0",
	}, nil)

	// search
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search",
		Description: "Search for board games on BoardGameGeek by name",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		var types []bggo.ItemType
		for _, t := range input.Types {
			types = append(types, bggo.ItemType(t))
		}
		results, err := client.Search(ctx, bggo.SearchRequest{
			Query: input.Query,
			Types: types,
			Exact: input.Exact,
		})
		if err != nil {
			return nil, SearchOutput{}, err
		}
		return nil, SearchOutput{Results: results}, nil
	})

	// get_things
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_things",
		Description: "Get detailed information about board games by their BGG IDs (max 20 at a time). Returns name, description, player counts, play time, ratings, categories, mechanics, designers, and more.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetThingsInput) (*mcp.CallToolResult, GetThingsOutput, error) {
		things, err := client.GetThings(ctx, bggo.GetThingsRequest{
			IDs:           input.IDs,
			RankBreakDown: input.RankBreakDown,
		})
		if err != nil {
			return nil, GetThingsOutput{}, err
		}
		return nil, GetThingsOutput{Things: things}, nil
	})

	// get_collection
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_collection",
		Description: "Get a BGG user's board game collection. Can filter by status (own, wishlist, played, etc).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetCollectionInput) (*mcp.CallToolResult, GetCollectionOutput, error) {
		var statuses []bggo.CollectionStatus
		for _, s := range input.Statuses {
			statuses = append(statuses, bggo.CollectionStatus(s))
		}
		items, err := client.GetCollection(ctx, bggo.GetCollectionRequest{
			Username: input.Username,
			Statuses: statuses,
		})
		if err != nil {
			return nil, GetCollectionOutput{}, err
		}
		return nil, GetCollectionOutput{Items: items}, nil
	})

	// get_plays
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_plays",
		Description: "Get play history for a BGG user or a specific game. Returns paginated results (100 per page).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetPlaysInput) (*mcp.CallToolResult, GetPlaysOutput, error) {
		req := bggo.GetPlaysRequest{
			Username: input.Username,
			GameID:   input.GameID,
			Page:     input.Page,
		}
		if input.MinDate != "" {
			t, err := time.Parse("2006-01-02", input.MinDate)
			if err != nil {
				return nil, GetPlaysOutput{}, fmt.Errorf("invalid min_date: %w", err)
			}
			req.MinDate = t
		}
		if input.MaxDate != "" {
			t, err := time.Parse("2006-01-02", input.MaxDate)
			if err != nil {
				return nil, GetPlaysOutput{}, fmt.Errorf("invalid max_date: %w", err)
			}
			req.MaxDate = t
		}
		result, err := client.GetPlays(ctx, req)
		if err != nil {
			return nil, GetPlaysOutput{}, err
		}
		return nil, GetPlaysOutput{
			Total:    result.Total,
			Page:     result.Page,
			Username: result.Username,
			Plays:    result.Plays,
		}, nil
	})

	// post_play
	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_play",
		Description: "Log a new board game play. Requires the server to be started with BGG_USERNAME and BGG_PASSWORD.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input PostPlayInput) (*mcp.CallToolResult, PostPlayOutput, error) {
		if !loggedIn {
			return nil, PostPlayOutput{}, fmt.Errorf("not logged in — set BGG_USERNAME and BGG_PASSWORD environment variables")
		}

		date := time.Now()
		if input.Date != "" {
			t, err := time.Parse("2006-01-02", input.Date)
			if err != nil {
				return nil, PostPlayOutput{}, fmt.Errorf("invalid date: %w", err)
			}
			date = t
		}

		gameType := bggo.ItemType("thing")
		if input.GameType != "" {
			gameType = bggo.ItemType(input.GameType)
		}

		var players []bggo.PostPlayPlayer
		for _, p := range input.Players {
			players = append(players, bggo.PostPlayPlayer{
				Username: p.Username,
				Name:     p.Name,
				Win:      p.Win,
				Score:    p.Score,
				Color:    p.Color,
				New:      p.New,
			})
		}

		result, err := client.PostPlay(ctx, bggo.PostPlayRequest{
			GameID:   input.GameID,
			GameType: gameType,
			Date:     date,
			Length:   time.Duration(input.Length) * time.Minute,
			Location: input.Location,
			Comment:  input.Comment,
			Players:  players,
		})
		if err != nil {
			return nil, PostPlayOutput{}, err
		}
		return nil, PostPlayOutput{
			PlayID:   result.PlayID,
			NumPlays: result.NumPlays,
		}, nil
	})

	// get_hotness
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_hotness",
		Description: "Get the current BGG hotness list — the most trending board games right now.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetHotnessInput) (*mcp.CallToolResult, GetHotnessOutput, error) {
		items, err := client.GetHotness(ctx, bggo.GetHotnessRequest{Count: input.Count})
		if err != nil {
			return nil, GetHotnessOutput{}, err
		}
		return nil, GetHotnessOutput{Items: items}, nil
	})

	// get_geek_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_geek_list",
		Description: "Get items from a BGG geek list by its ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetGeekListInput) (*mcp.CallToolResult, GetGeekListOutput, error) {
		items, err := client.GetGeekList(ctx, bggo.GetGeekListRequest{ListID: input.ListID})
		if err != nil {
			return nil, GetGeekListOutput{}, err
		}
		return nil, GetGeekListOutput{Items: items}, nil
	})

	// get_best_sellers
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_best_sellers",
		Description: "Get the weekly best-selling board games on BGG.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetTrendInput) (*mcp.CallToolResult, GetTrendOutput, error) {
		start := time.Now().AddDate(0, 0, -7)
		if input.StartDate != "" {
			t, err := time.Parse("2006-01-02", input.StartDate)
			if err != nil {
				return nil, GetTrendOutput{}, fmt.Errorf("invalid start_date: %w", err)
			}
			start = t
		}
		items, err := client.GetBestSellers(ctx, bggo.GetTrendRequest{
			Interval:  parseTrendInterval(input.Interval),
			StartDate: start,
		})
		if err != nil {
			return nil, GetTrendOutput{}, err
		}
		return nil, GetTrendOutput{Items: items}, nil
	})

	// get_most_played
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_most_played",
		Description: "Get the most-played board games on BGG for a given time period.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetTrendInput) (*mcp.CallToolResult, GetTrendOutput, error) {
		start := time.Now().AddDate(0, 0, -7)
		if input.StartDate != "" {
			t, err := time.Parse("2006-01-02", input.StartDate)
			if err != nil {
				return nil, GetTrendOutput{}, fmt.Errorf("invalid start_date: %w", err)
			}
			start = t
		}
		items, err := client.GetMostPlayed(ctx, bggo.GetTrendRequest{
			Interval:  parseTrendInterval(input.Interval),
			StartDate: start,
		})
		if err != nil {
			return nil, GetTrendOutput{}, err
		}
		return nil, GetTrendOutput{Items: items}, nil
	})

	// get_trending_plays
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_trending_plays",
		Description: "Get board games with the biggest play count increase (trending plays) on BGG.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetTrendInput) (*mcp.CallToolResult, GetTrendOutput, error) {
		start := time.Now().AddDate(0, 0, -7)
		if input.StartDate != "" {
			t, err := time.Parse("2006-01-02", input.StartDate)
			if err != nil {
				return nil, GetTrendOutput{}, fmt.Errorf("invalid start_date: %w", err)
			}
			start = t
		}
		items, err := client.GetTrendingPlays(ctx, bggo.GetTrendRequest{
			Interval:  parseTrendInterval(input.Interval),
			StartDate: start,
		})
		if err != nil {
			return nil, GetTrendOutput{}, err
		}
		return nil, GetTrendOutput{Items: items}, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func parseTrendInterval(s string) bggo.TrendInterval {
	if strings.EqualFold(s, "month") {
		return bggo.TrendMonth
	}
	return bggo.TrendWeek
}
