package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetUser_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	username := os.Getenv("BGG_USERNAME")
	if apiKey == "" || username == "" {
		t.Skip("BGG_API_KEY or BGG_USERNAME not set")
	}

	c := bggo.NewClient(apiKey)

	user, err := c.GetUser(context.Background(), bggo.GetUserRequest{
		Username: username,
	})
	require.NoError(t, err)

	assert.Equal(t, username, user.Username)

	t.Logf("ID=%d Username=%q Name=%q %q Year=%d Country=%q",
		user.ID, user.Username, user.FirstName, user.LastName, user.YearRegistered, user.Country)
}
