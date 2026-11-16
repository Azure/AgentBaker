package scenario

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeepCachedImages(t *testing.T) {
	cached := map[string]bool{
		"mcr.microsoft.com/windows/servercore:ltsc2022": true,
		"mcr.microsoft.com/windows/servercore:ltsc2025": true,
	}

	t.Run("drops pinned tags missing from a pre-release VHD", func(t *testing.T) {
		kept, err := keepCachedImages(t.Context(), []string{
			"mcr.microsoft.com/windows/servercore:10.0.20348.5622",
			"mcr.microsoft.com/windows/servercore:ltsc2022",
			"mcr.microsoft.com/windows/servercore:10.0.26100.33438",
			"mcr.microsoft.com/windows/servercore:ltsc2025",
		}, cached)
		require.NoError(t, err)
		require.Equal(t, []string{
			"mcr.microsoft.com/windows/servercore:ltsc2022",
			"mcr.microsoft.com/windows/servercore:ltsc2025",
		}, kept)
	})

	t.Run("keeps every image on a regular VHD", func(t *testing.T) {
		images := []string{"mcr.microsoft.com/windows/servercore:ltsc2022", "mcr.microsoft.com/windows/servercore:ltsc2025"}
		kept, err := keepCachedImages(t.Context(), images, cached)
		require.NoError(t, err)
		require.Equal(t, images, kept)
	})

	t.Run("fails when nothing is cached", func(t *testing.T) {
		_, err := keepCachedImages(t.Context(), []string{"mcr.microsoft.com/windows/nanoserver:ltsc2025"}, cached)
		require.ErrorContains(t, err, "none of the base images")
	})
}
