package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTagsMatchesFiltersAMDV7(t *testing.T) {
	matches, err := Tags{AMDV7: true}.MatchesFilters("amdV7=true")
	require.NoError(t, err)
	require.True(t, matches)
}
