package bggo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fzerorubigd/bggo"
)

func TestGetPerson_Integration(t *testing.T) {
	apiKey := os.Getenv("BGG_API_KEY")
	if apiKey == "" {
		t.Skip("BGG_API_KEY not set")
	}

	c := bggo.NewClient(apiKey)

	// Klaus Teuber (Catan designer) = ID 11
	person, err := c.GetPerson(context.Background(), bggo.GetPersonRequest{ID: 11})
	require.NoError(t, err)

	assert.Equal(t, int64(11), person.ID)
	assert.Equal(t, bggo.ItemType("boardgamedesigner"), person.Type)
	assert.NotEmpty(t, person.Thumbnail)
	assert.NotEmpty(t, person.Image)

	t.Logf("ID=%d Type=%s Thumbnail=%s", person.ID, person.Type, person.Thumbnail)
}
