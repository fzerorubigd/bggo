package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// get_things with no ids returns a model-actionable error that steers the model
// to search first, without touching the BGG client (the empty path returns before
// any call, so a nil client is safe).
func TestGetThings_RejectsEmptyIDs(t *testing.T) {
	_, _, err := getThings(context.Background(), nil, GetThingsInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ids")
	assert.Contains(t, err.Error(), "search", "steers the model to resolve a name to an id first")
}
