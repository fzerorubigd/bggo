package bggo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetHotness_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)

	items, err := c.GetHotness(context.Background(), bggo.GetHotnessRequest{Count: 5})
	require.NoError(t, err)
	require.Len(t, items, 5)

	for _, item := range items {
		t.Logf("ID=%d Name=%q Rank=%d Delta=%d", item.ID, item.Name, item.Rank, item.Delta)
		assert.Greater(t, item.ID, int64(0))
		assert.NotEmpty(t, item.Name)
	}
}

func TestGetGeekList_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)

	items, err := c.GetGeekList(context.Background(), bggo.GetGeekListRequest{ListID: 330393})
	require.NoError(t, err)
	require.NotEmpty(t, items)

	for _, item := range items[:3] {
		t.Logf("ID=%d Name=%q", item.ID, item.Name)
	}
}

func TestGetMostPlayed_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)

	items, err := c.GetMostPlayed(context.Background(), bggo.GetTrendRequest{
		Interval:  bggo.TrendWeek,
		StartDate: time.Now().AddDate(0, 0, -14),
	})
	require.NoError(t, err)
	require.NotEmpty(t, items)

	for _, item := range items[:3] {
		t.Logf("ID=%d Name=%q Rank=%d Delta=%d Appearances=%d",
			item.ID, item.Name, item.Rank, item.Delta, item.Appearances)
	}

	assert.Equal(t, 1, items[0].Rank)
}

func TestExtractIDs_Pipeline(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	ctx := context.Background()
	c := bggo.NewClient(apiKey)

	// Get hotness -> extract IDs -> fetch full thing details
	hotness, err := c.GetHotness(ctx, bggo.GetHotnessRequest{Count: 3})
	require.NoError(t, err)
	require.Len(t, hotness, 3)

	ids := bggo.ExtractIDs(hotness)
	require.Len(t, ids, 3)

	things, err := c.GetThings(ctx, bggo.GetThingsRequest{IDs: ids})
	require.NoError(t, err)
	require.NotEmpty(t, things)

	for _, thing := range things {
		t.Logf("ID=%d Name=%q Year=%d Rating=%.2f", thing.ID, thing.Name, thing.YearPublished, thing.AverageRate)
		assert.NotEmpty(t, thing.Name)
	}
}
