package e2e

import (
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/stretchr/testify/require"
)

func TestVersionConsistencyGPUManagedComponents(t *testing.T) {
	allPackageVariants := [][]packageOSVariant{
		{
			{"nvidia-device-plugin", "ubuntu", "r2404"},
			{"nvidia-device-plugin", "ubuntu", "r2204"},
			{"nvidia-device-plugin", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-4-core", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-4-core", "ubuntu", "r2204"},
			{"datacenter-gpu-manager-4-core", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-4-proprietary", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-4-proprietary", "ubuntu", "r2204"},
			{"datacenter-gpu-manager-4-proprietary", "azurelinux", "v3.0"},
		},
		{
			{"dcgm-exporter", "ubuntu", "r2404"},
			{"dcgm-exporter", "ubuntu", "r2204"},
			{"dcgm-exporter", "azurelinux", "v3.0"},
		},
	}

	for _, packageGroup := range allPackageVariants {
		expectedVersion := ""
		expectedRevision := ""
		for _, pkgVar := range packageGroup {
			componentVersions := components.GetExpectedPackageVersions(pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease)
			require.Lenf(t, componentVersions, 1,
				"Expected exactly one %s version for %s %s but got %d",
				pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease, len(componentVersions))

			pkgVersion := extractMajorMinorPatchVersion(componentVersions[0])
			require.NotEmptyf(t, pkgVersion, "Failed to extract major.minor.patch version from %s for %s %s",
				componentVersions[0], pkgVar.osName, pkgVar.osRelease)

			pkgRevision := extractPackageRevision(componentVersions[0])
			require.NotEmptyf(t, pkgRevision, "Failed to extract rebuild revision from %s for %s %s",
				componentVersions[0], pkgVar.osName, pkgVar.osRelease)

			if expectedVersion == "" {
				expectedVersion = pkgVersion
				expectedRevision = pkgRevision
				continue
			}

			require.Equalf(t, expectedVersion, pkgVersion,
				"Expected all %s versions to have the same major.minor.patch version, but found mismatch: %s vs %s for %s.%s",
				pkgVar.pkgName, expectedVersion, pkgVersion, pkgVar.osName, pkgVar.osRelease)
			require.Equalf(t, expectedRevision, pkgRevision,
				"Partial OS update detected for %s: rebuild revision %q (%s.%s) does not match %q from the first OS variant.",
				pkgVar.pkgName, pkgRevision, pkgVar.osName, pkgVar.osRelease, expectedRevision)
		}
	}
}

func TestExtractPackageRevision(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "4.8.2-ubuntu24.04u2", expected: "2"},
		{input: "4.8.2-ubuntu22.04u2", expected: "2"},
		{input: "4.8.2-1.azl3", expected: "1"},
		{input: "1:4.5.3-1", expected: "1"},
		{input: "0.19.2-ubuntu22.04u10", expected: "10"},
		{input: "4.6.0-3.azl3", expected: "3"},
		{input: "", expected: ""},
		{input: "4.8.2", expected: ""},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("extractPackageRevision(%q)=%q", test.input, test.expected), func(t *testing.T) {
			require.Equal(t, test.expected, extractPackageRevision(test.input))
		})
	}
}
