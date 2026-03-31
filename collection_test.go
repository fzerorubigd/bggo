package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetCollection_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	username := os.Getenv("BGG_USERNAME")
	if apiKey == "" || username == "" {
		t.Skip("BGG_API_KEY or BGG_USERNAME not set")
	}

	c := bggo.NewClient(apiKey)

	items, err := c.GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: username,
		Statuses: []bggo.CollectionStatus{bggo.CollectionOwn},
	})
	require.NoError(t, err)
	require.NotEmpty(t, items)

	for _, item := range items {
		t.Logf("ID=%d Name=%q Type=%s Year=%d Plays=%d Status=%v",
			item.ID, item.Name, item.Type, item.YearPublished, item.NumPlays, item.Status)
	}

	assert.Contains(t, items[0].Status, bggo.CollectionOwn)
}
