package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForOS(t *testing.T) {
	require.Equal(t, Windows(), ForOS("windows"))
	require.Equal(t, Linux(), ForOS("linux"))
	require.Equal(t, Linux(), ForOS("darwin"))
	require.Empty(t, Windows().ReadyReportMarkerPath)
	require.NotEmpty(t, Linux().ReadyReportMarkerPath)
}
