package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestLogin_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	username := os.Getenv("BGG_USERNAME")
	password := os.Getenv("BGG_PASSWORD")
	if apiKey == "" || username == "" || password == "" {
		t.Skip("BGG_API_KEY, BGG_USERNAME, or BGG_PASSWORD not set")
	}

	c := bggo.NewClient(apiKey)

	err := c.Login(context.Background(), bggo.LoginRequest{
		Username: username,
		Password: password,
	})
	require.NoError(t, err)

	// Verify cookies were stored by making an authenticated call
	items, err := c.GetCollection(context.Background(), bggo.GetCollectionRequest{
		Username: username,
		Statuses: []bggo.CollectionStatus{bggo.CollectionOwn},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}
