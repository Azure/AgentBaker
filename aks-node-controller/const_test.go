package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigForOS(t *testing.T) {
	require.Equal(t, windowsConfig(), configForOS("windows"))
	require.Equal(t, linuxConfig(), configForOS("linux"))
	require.Equal(t, linuxConfig(), configForOS("darwin"))
	require.Empty(t, windowsConfig().readyReportMarkerPath)
	require.NotEmpty(t, linuxConfig().readyReportMarkerPath)
}
