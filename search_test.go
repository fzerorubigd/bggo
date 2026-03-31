package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestSearch_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)

	results, err := c.Search(context.Background(), bggo.SearchRequest{
		Query: "Catan",
		Types: []bggo.ItemType{bggo.BoardGameType},
		Exact: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)

	ids := make([]int64, len(results))
	for i, r := range results {
		ids[i] = r.ID
		t.Logf("ID=%d Name=%q Type=%s Year=%d", r.ID, r.Name, r.Type, r.YearPublished)
	}

	assert.Contains(t, ids, int64(13), "expected Catan (ID=13) in results")
}
