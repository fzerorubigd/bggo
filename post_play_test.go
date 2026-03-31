package bggo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestPostPlay_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	username := os.Getenv("BGG_USERNAME")
	password := os.Getenv("BGG_PASSWORD")
	if apiKey == "" || username == "" || password == "" {
		t.Skip("BGG_API_KEY, BGG_USERNAME, or BGG_PASSWORD not set")
	}

	ctx := context.Background()
	c := bggo.NewClient(apiKey)

	err := c.Login(ctx, bggo.LoginRequest{
		Username: username,
		Password: password,
	})
	require.NoError(t, err)

	result, err := c.PostPlay(ctx, bggo.PostPlayRequest{
		GameID:   23383, // Hokm
		GameType: "thing",
		Date:     time.Now(),
		Length:   20 * time.Minute,
		Location: "Testing",
		Comment:  "Testing",
		Players: []bggo.PostPlayPlayer{
			{
				Username: "fzerorubigd",
				Name:     "Forud",
				Win:      true,
			},
			{
				Username: "gobgg",
				Name:     "GoBGG",
				UserID:   3597059,
				Win:      false,
			},
		},
	})
	require.NoError(t, err)
	require.Greater(t, result.NumPlays, 0)

	t.Logf("PlayID=%d NumPlays=%d", result.PlayID, result.NumPlays)
}
