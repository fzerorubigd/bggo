# bggo

[![BoardGameGeek](https://upload.wikimedia.org/wikipedia/commons/b/bb/BoardGameGeek_Logo.svg)](https://boardgamegeek.com)

A Go client library and MCP server for the [BoardGameGeek](https://boardgamegeek.com) API.

> This is a complete rewrite of [gobgg](https://github.com/fzerorubigd/gobgg) with a cleaner API design, struct-based requests/responses, and additional features.

## MCP Server

The included MCP server lets AI assistants (Claude, etc.) interact with BoardGameGeek — search for games, browse collections, check hotness, log plays, and more.

### Prerequisites

- Go 1.22+
- A [BGG API key](https://boardgamegeek.com/wiki/page/BGG_XML_API2)

### Build

```bash
go install github.com/fzerorubigd/bggo/cmd/bgg-mcp@latest
```

Or from a local clone:

```bash
git clone https://github.com/fzerorubigd/bggo.git
cd bggo
go build -o bgg-mcp ./cmd/bgg-mcp/
```

### Configuration

The server reads configuration from environment variables:

| Variable | Required | Description |
|---|---|---|
| `BGG_API_KEY` | Yes | Your BoardGameGeek API key |
| `BGG_USERNAME` | No | BGG username for authenticated features (e.g. logging plays) |
| `BGG_PASSWORD` | No | BGG password for authenticated features |

### Add to Claude Code

```bash
claude mcp add bgg-mcp -e BGG_API_KEY=your-key -- /path/to/bgg-mcp
```

Or with authenticated features:

```bash
claude mcp add bgg-mcp \
  -e BGG_API_KEY=your-key \
  -e BGG_USERNAME=your-user \
  -e BGG_PASSWORD=your-pass \
  -- /path/to/bgg-mcp
```

### Add to Claude Desktop

Add this to your Claude Desktop config:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "bgg-mcp": {
      "command": "/path/to/bgg-mcp",
      "env": {
        "BGG_API_KEY": "your-key",
        "BGG_USERNAME": "your-user",
        "BGG_PASSWORD": "your-pass"
      }
    }
  }
}
```

### Available Tools

| Tool | Description |
|---|---|
| `search` | Search for board games by name |
| `get_things` | Get detailed game info by BGG IDs (max 20) |
| `get_collection` | Browse a user's board game collection |
| `get_plays` | View play history for a user or game |
| `post_play` | Log a new play (requires login) |
| `get_hotness` | Current trending games on BGG |
| `get_geek_list` | Get items from a geek list |
| `get_best_sellers` | Weekly best-selling games |
| `get_most_played` | Most played games for a time period |
| `get_trending_plays` | Games with the biggest play count increase |

---

## Go Library

The MCP server is built on top of the `bggo` Go library, which you can also use directly in your own projects.

### Install

```bash
go get github.com/fzerorubigd/bggo
```

### Usage

```go
client := bggo.NewClient("your-bgg-api-key")
```

The API key is required and used as a Bearer token for all requests.

### Search

```go
results, err := client.Search(ctx, bggo.SearchRequest{
    Query: "Catan",
    Types: []bggo.ItemType{bggo.BoardGameType},
    Exact: true,
})
```

### Get Things (Game Details)

```go
things, err := client.GetThings(ctx, bggo.GetThingsRequest{
    IDs: []int64{13, 30549}, // Catan, Pandemic
})

// With rating breakdown (extra API call per thing)
things, err := client.GetThings(ctx, bggo.GetThingsRequest{
    IDs:           []int64{13},
    RankBreakDown: true,
})
```

### User Profile

```go
user, err := client.GetUser(ctx, bggo.GetUserRequest{
    Username: "someone",
})
```

### Collection

```go
items, err := client.GetCollection(ctx, bggo.GetCollectionRequest{
    Username: "someone",
    Statuses: []bggo.CollectionStatus{bggo.CollectionOwn},
})
```

### Plays

```go
plays, err := client.GetPlays(ctx, bggo.GetPlaysRequest{
    Username: "someone",
})
```

### Login & Post Play

```go
err := client.Login(ctx, bggo.LoginRequest{
    Username: "user",
    Password: "pass",
})

result, err := client.PostPlay(ctx, bggo.PostPlayRequest{
    GameID:   23383,
    GameType: "thing",
    Date:     time.Now(),
    Length:   20 * time.Minute,
    Players: []bggo.PostPlayPlayer{
        {Username: "player1", Name: "Alice", Win: true},
    },
})
```

### Hotness & Trends

```go
hot, err := client.GetHotness(ctx, bggo.GetHotnessRequest{Count: 10})

trend, err := client.GetMostPlayed(ctx, bggo.GetTrendRequest{
    Interval:  bggo.TrendWeek,
    StartDate: time.Now().AddDate(0, 0, -7),
})
```

### Geek Lists

```go
items, err := client.GetGeekList(ctx, bggo.GetGeekListRequest{
    ListID: 330393,
})
```

### Piping Lists into GetThings

All list types implement `IDGetter`. Use `ExtractIDs` to pipe results into `GetThings`:

```go
hot, _ := client.GetHotness(ctx, bggo.GetHotnessRequest{Count: 5})
things, _ := client.GetThings(ctx, bggo.GetThingsRequest{
    IDs: bggo.ExtractIDs(hot),
})
```

### Client Options

```go
client := bggo.NewClient("api-key",
    bggo.WithHTTPClient(customClient),
    bggo.WithHost("boardgamegeek.com"),
    bggo.WithScheme("https"),
    bggo.WithLimiter(rateLimiter), // compatible with go.uber.org/ratelimit
    bggo.WithCookies("user", cookies),
)
```

## License

MIT
