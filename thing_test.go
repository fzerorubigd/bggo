package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetThings_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)
	// Catan = 13, Pandemic = 30549
	results, err := c.GetThings(context.Background(), bggo.GetThingsRequest{
		IDs: []int64{13, 30549},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	for _, r := range results {
		t.Logf("ID=%d Name=%q Type=%s Year=%d MinPlayers=%d MaxPlayers=%d Rank=%d Rating=%.2f",
			r.ID, r.Name, r.Type, r.YearPublished, r.MinPlayers, r.MaxPlayers, r.Rank, r.AverageRate)
	}

	assert.Equal(t, "Catan", results[0].Name)
	assert.Equal(t, int64(13), results[0].ID)
	assert.Equal(t, bggo.BoardGameType, results[0].Type)
	assert.Nil(t, results[0].RankBreakDown)

	assert.Equal(t, "Pandemic", results[1].Name)
	assert.Equal(t, int64(30549), results[1].ID)

	// Suggested player count should be populated
	require.NotEmpty(t, results[0].SuggestedPlayerCount)
	spc := results[0].SuggestedPlayerCount[0]
	rec, votes, pct := spc.Suggestion()
	t.Logf("SuggestedPlayerCount[0]: NumPlayers=%s Suggestion=%s Votes=%d Pct=%.1f%%",
		spc.NumPlayers, rec, votes, pct)
	assert.Greater(t, votes, 0)
	assert.Greater(t, pct, float32(0))
}

func TestGetThings_WithRankBreakDown(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)
	results, err := c.GetThings(context.Background(), bggo.GetThingsRequest{
		IDs:           []int64{13},
		RankBreakDown: true,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	rbd := results[0].RankBreakDown
	require.NotNil(t, rbd)
	assert.Greater(t, rbd.Total(), int64(0))

	t.Logf("RankBreakDown: %v Total=%d Avg=%.2f Bayes(100)=%.2f",
		rbd, rbd.Total(), rbd.Average(), rbd.BayesianAverage(100))
}
