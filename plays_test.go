package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetPlays_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	username := os.Getenv("BGG_USERNAME")
	if apiKey == "" || username == "" {
		t.Skip("BGG_API_KEY or BGG_USERNAME not set")
	}

	c := bggo.NewClient(apiKey)

	result, err := c.GetPlays(context.Background(), bggo.GetPlaysRequest{
		Username: username,
	})
	require.NoError(t, err)

	assert.Equal(t, username, result.Username)
	assert.Greater(t, result.Total, int64(0))
	require.NotEmpty(t, result.Plays)

	for _, p := range result.Plays {
		t.Logf("ID=%d Date=%s Game=%q (ID=%d) Quantity=%d Players=%d",
			p.ID, p.Date.Format("2006-01-02"), p.Item.Name, p.Item.ID, p.Quantity, len(p.Players))
	}
}
