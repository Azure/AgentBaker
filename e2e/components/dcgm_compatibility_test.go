//go:build dcgmcompat

package components

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blakesmith/ar"
	"github.com/cavaliergopher/rpm"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestDCGMExporterCompatibility(t *testing.T) {
	type testCase struct {
		name       string
		os         string
		osVersion  string
		packageDir string
		packageRE  string
		parseDeps  func(t *testing.T, path string) (coreVersion, propVersion string)
	}

	testCases := []testCase{
		{
			name:       "Ubuntu2204",
			os:         "ubuntu",
			osVersion:  "r2204",
			packageDir: "https://packages.microsoft.com/repos/microsoft-ubuntu-jammy-prod/pool/main/d/dcgm-exporter",
			packageRE:  `dcgm-exporter_%s-ubuntu22\.04u([0-9]+)_amd64\.deb`,
			parseDeps:  parseDebDeps,
		},
		{
			name:       "Ubuntu2404",
			os:         "ubuntu",
			osVersion:  "r2404",
			packageDir: "https://packages.microsoft.com/repos/microsoft-ubuntu-noble-prod/pool/main/d/dcgm-exporter",
			packageRE:  `dcgm-exporter_%s-ubuntu24\.04u([0-9]+)_amd64\.deb`,
			parseDeps:  parseDebDeps,
		},
		{
			name:       "Ubuntu2604",
			os:         "ubuntu",
			osVersion:  "r2604",
			packageDir: "https://packages.microsoft.com/ubuntu/26.04/prod/pool/main/d/dcgm-exporter",
			packageRE:  `dcgm-exporter_%s-ubuntu26\.04u([0-9]+)_amd64\.deb`,
			parseDeps:  parseDebDeps,
		},
		{
			name:       "AzureLinux3",
			os:         "azurelinux",
			osVersion:  "v3.0",
			packageDir: "https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/Packages/d",
			packageRE:  `dcgm-exporter-%s-([0-9]+)\.azl3\.x86_64\.rpm`,
			parseDeps:  parseRPMDeps,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dcgmExporterVersions := GetExpectedPackageVersions("dcgm-exporter", tc.os, tc.osVersion)
			require.NotEmpty(t, dcgmExporterVersions, "dcgm-exporter not found in components.json")
			dcgmExporterVersion := dcgmExporterVersions[0]

			coreVersions := GetExpectedPackageVersions("datacenter-gpu-manager-4-core", tc.os, tc.osVersion)
			require.NotEmpty(t, coreVersions, "datacenter-gpu-manager-4-core not found in components.json")
			expectedCoreVersion := coreVersions[0]

			propVersions := GetExpectedPackageVersions("datacenter-gpu-manager-4-proprietary", tc.os, tc.osVersion)
			require.NotEmpty(t, propVersions, "datacenter-gpu-manager-4-proprietary not found in components.json")
			expectedPropVersion := propVersions[0]

			t.Logf("Expected versions from components.json:")
			t.Logf("  dcgm-exporter: %s", dcgmExporterVersion)
			t.Logf("  datacenter-gpu-manager-4-core: %s", expectedCoreVersion)
			t.Logf("  datacenter-gpu-manager-4-proprietary: %s", expectedPropVersion)

			url := resolveLatestPackageURL(t, tc.packageDir, tc.packageRE, dcgmExporterVersion)
			t.Logf("Downloading dcgm-exporter package from %s", url)

			tmpFile, err := os.CreateTemp("", "dcgm-exporter-*")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			resp := downloadWithRetry(t, url, 3)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to download dcgm-exporter package from %s", url)

			_, err = io.Copy(tmpFile, resp.Body)
			require.NoError(t, err)
			require.NoError(t, tmpFile.Close())

			actualCoreVersion, actualPropVersion := tc.parseDeps(t, tmpFile.Name())

			t.Logf("Actual versions from dcgm-exporter package:")
			t.Logf("  datacenter-gpu-manager-4-core: %s", actualCoreVersion)
			t.Logf("  datacenter-gpu-manager-4-proprietary: %s", actualPropVersion)

			require.Equalf(t, extractUpstreamPackageVersion(expectedCoreVersion), extractUpstreamPackageVersion(actualCoreVersion),
				"datacenter-gpu-manager-4-core version mismatch: components.json has %s but dcgm-exporter requires %s",
				expectedCoreVersion, actualCoreVersion)
			require.Equalf(t, extractUpstreamPackageVersion(expectedPropVersion), extractUpstreamPackageVersion(actualPropVersion),
				"datacenter-gpu-manager-4-proprietary version mismatch: components.json has %s but dcgm-exporter requires %s",
				expectedPropVersion, actualPropVersion)

			t.Logf("✅ Version compatibility verified: dcgm-exporter %s is compatible with DCGM packages %s",
				dcgmExporterVersion, expectedCoreVersion)
		})
	}
}

func resolveLatestPackageURL(t *testing.T, packageDir, packagePattern, upstreamVersion string) string {
	t.Helper()

	resp := downloadWithRetry(t, packageDir+"/", 3)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to list packages from %s", packageDir)

	index, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	pattern := regexp.MustCompile(fmt.Sprintf(packagePattern, regexp.QuoteMeta(upstreamVersion)))
	matches := pattern.FindAllStringSubmatch(string(index), -1)
	require.NotEmptyf(t, matches, "No package revision found for %s in %s", upstreamVersion, packageDir)

	latestRevision := -1
	latestFilename := ""
	for _, match := range matches {
		require.Len(t, match, 2, "Package pattern must capture the numeric rebuild revision")
		revision, err := strconv.Atoi(match[1])
		require.NoError(t, err)
		if revision > latestRevision {
			latestRevision = revision
			latestFilename = match[0]
		}
	}

	t.Logf("Resolved dcgm-exporter package version %s -> %s", upstreamVersion, latestFilename)
	return packageDir + "/" + latestFilename
}

func extractUpstreamPackageVersion(version string) string {
	version = regexp.MustCompile(`^\d+:`).ReplaceAllString(version, "")
	return regexp.MustCompile(`^\d+\.\d+\.\d+`).FindString(version)
}

func downloadWithRetry(t *testing.T, url string, maxRetries int) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: 60 * time.Second}
	var lastErr error
	for attempt := range maxRetries {
		resp, err := client.Get(url)
		if err == nil {
			return resp
		}
		lastErr = err
		t.Logf("Download attempt %d/%d failed: %v", attempt+1, maxRetries, err)
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	require.NoError(t, lastErr, "All %d download attempts failed for %s", maxRetries, url)
	return nil
}

func parseDebDeps(t *testing.T, path string) (string, string) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	reader := ar.NewReader(f)
	for {
		header, err := reader.Next()
		require.NoError(t, err, "control file not found in .deb package")
		if !strings.HasPrefix(header.Name, "control.tar") {
			continue
		}

		var tarReader *tar.Reader
		if strings.HasSuffix(header.Name, ".gz") {
			gz, err := gzip.NewReader(reader)
			require.NoError(t, err)
			defer gz.Close()
			tarReader = tar.NewReader(gz)
		} else if strings.HasSuffix(header.Name, ".zst") {
			zr, err := zstd.NewReader(reader)
			require.NoError(t, err)
			defer zr.Close()
			tarReader = tar.NewReader(zr)
		} else {
			tarReader = tar.NewReader(reader)
		}

		for {
			th, err := tarReader.Next()
			require.NoError(t, err, "control file not found in control.tar")
			if th.Name == "./control" || th.Name == "control" {
				data, err := io.ReadAll(tarReader)
				require.NoError(t, err)
				dependsValue := parseDebControlField(string(data), "Depends")
				require.NotEmpty(t, dependsValue, "Depends field not found in control file")

				coreMatches := regexp.MustCompile(`datacenter-gpu-manager-4-core \(= ([^)]+)\)`).FindStringSubmatch(dependsValue)
				require.Len(t, coreMatches, 2, "Failed to extract datacenter-gpu-manager-4-core version from Depends")
				propMatches := regexp.MustCompile(`datacenter-gpu-manager-4-proprietary \(= ([^)]+)\)`).FindStringSubmatch(dependsValue)
				require.Len(t, propMatches, 2, "Failed to extract datacenter-gpu-manager-4-proprietary version from Depends")
				return coreMatches[1], propMatches[1]
			}
		}
	}
}

func parseDebControlField(control, field string) string {
	prefix := field + ":"
	var result strings.Builder
	found := false
	scanner := bufio.NewScanner(strings.NewReader(control))
	for scanner.Scan() {
		line := scanner.Text()
		if found {
			if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
				result.WriteString(" ")
				result.WriteString(strings.TrimSpace(line))
				continue
			}
			break
		}
		if strings.HasPrefix(line, prefix) {
			found = true
			result.WriteString(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
		}
	}
	return result.String()
}

func parseRPMDeps(t *testing.T, path string) (string, string) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	pkg, err := rpm.Read(f)
	require.NoError(t, err)
	var coreVersion, propVersion string
	for _, req := range pkg.Requires() {
		name := req.Name()
		if name == "datacenter-gpu-manager-4-core" {
			t.Logf("RPM dependency %s: epoch=%d version=%s release=%s", name, req.Epoch(), req.Version(), req.Release())
			coreVersion = formatRPMVersion(req)
		}
		if name == "datacenter-gpu-manager-4-proprietary" {
			t.Logf("RPM dependency %s: epoch=%d version=%s release=%s", name, req.Epoch(), req.Version(), req.Release())
			propVersion = formatRPMVersion(req)
		}
	}
	require.NotEmpty(t, coreVersion, "datacenter-gpu-manager-4-core dependency not found in RPM Requires")
	require.NotEmpty(t, propVersion, "datacenter-gpu-manager-4-proprietary dependency not found in RPM Requires")
	return coreVersion, propVersion
}

func formatRPMVersion(dep rpm.Dependency) string {
	epoch := dep.Epoch()
	version := dep.Version()
	release := dep.Release()
	if epoch > 0 {
		return fmt.Sprintf("%d:%s-%s", epoch, version, release)
	}
	if release != "" {
		return fmt.Sprintf("%s-%s", version, release)
	}
	return version
}
