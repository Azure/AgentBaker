package scenario

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuncUsesImageManifestAndExactInstalledRevision(t *testing.T) {
	// An older published VHD must be validated against its own pin, even after
	// Renovate advances the checkout. A different installed revision still fails.
	for _, release := range []string{"r2004", "r2204", "r2404", "r2604"} {
		t.Run(release, func(t *testing.T) {
			manifest := fmt.Sprintf(`{"Packages":[{"name":"runc","downloadURIs":{"ubuntu":{"%s":{"versionsV2":[{"latestVersion":"1.4.3-ubuntu24.04u17"}]}}}}]}`, release)
			expected, err := runcVersionFromImageManifest(manifest, release)
			require.NoError(t, err)
			require.NoError(t, validateInstalledRunc("install ok installed\t1.4.3-ubuntu24.04u17\n", expected))
			for _, output := range []string{
				"install ok installed\t1.4.3-ubuntu24.04u18",  // wrong revision
				"install ok installed\t1.4.3-ubuntu24.04u170", // substring is insufficient
				"deinstall ok config-files\t1.4.3-ubuntu24.04u17", "", "1.4.3-ubuntu24.04u17",
			} {
				require.Error(t, validateInstalledRunc(output, expected))
			}
		})
	}
}

func TestRuncImageManifestFailsClosed(t *testing.T) {
	for _, manifest := range []string{
		"not json", `{}`, `{"Packages":[]}`,
		`{"Packages":[{"name":"runc","downloadURIs":{"ubuntu":{"r2404":{"versionsV2":[]}}}}]}`,
		`{"Packages":[{"name":"runc","downloadURIs":{"ubuntu":{"r2404":{"versionsV2":[{},{}]}}}}]}`,
	} {
		_, err := runcVersionFromImageManifest(manifest, "r2404")
		require.Error(t, err)
	}
	for _, version := range []string{"", "garbage", "1.1.9-ubuntu24.04u1", "1.0.99-ubuntu24.04u1"} {
		manifest := fmt.Sprintf(`{"Packages":[{"name":"runc","downloadURIs":{"ubuntu":{"r2404":{"versionsV2":[{"latestVersion":%q}]}}}}]}`, version)
		_, err := runcVersionFromImageManifest(manifest, "r2404")
		require.Error(t, err)
	}
	_, err := runcVersionFromImageManifest(`{}`, "r9999")
	require.ErrorContains(t, err, "unsupported")
}
