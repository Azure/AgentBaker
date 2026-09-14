package components

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/tidwall/gjson"
)

func GetKubeletVersionByMinorVersion(minorVersion string) string {
	allCachedKubeletVersions := GetExpectedPackageVersions("kubernetes-binaries", "default", "current")
	rightVersions := toolkit.Filter(allCachedKubeletVersions, func(v string) bool { return strings.HasPrefix(v, minorVersion) })
	rightVersion := toolkit.Reduce(rightVersions, "", func(sum string, next string) string {
		if sum == "" {
			return next
		}
		if next > sum {
			return next
		}
		return sum
	})
	return rightVersion
}

func GetExpectedPackageVersions(packageName, distro, release string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here

	// Get the project root dynamically
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename))) // Go up 3 levels from e2e/components/
	componentsPath := filepath.Join(projectRoot, "parts", "common", "components.json")

	jsonBytes, _ := os.ReadFile(componentsPath)
	packages := gjson.GetBytes(jsonBytes, fmt.Sprintf("Packages.#(name=%s).downloadURIs", packageName))

	// If there is a dot in the "release" then we need to escape it for the json path
	release = strings.ReplaceAll(release, ".", "\\.")

	for _, packageItem := range packages.Array() {
		// Check if versionsV2 exists. Assume the DEFAULT OS variant.
		versions := packageItem.Get(fmt.Sprintf("%s.DEFAULT/%s.versionsV2", distro, release)).Array()
		if len(versions) == 0 {
			versions = packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release)).Array()
		}

		for _, version := range versions {
			// get versions.latestVersion and append to expectedVersions
			expectedVersions = append(expectedVersions, version.Get("latestVersion").String())
			// get versions.previousLatestVersion (if exists) and append to expectedVersions
			if version.Get("previousLatestVersion").Exists() {
				expectedVersions = append(expectedVersions, version.Get("previousLatestVersion").String())
			}
		}
	}
	return expectedVersions
}

func GetWindowsContainerImages(containerName string, windowsVersion string) ([]string, error) {
	tags, err := getWindowsContainerImageTags(containerName, windowsVersion)
	if err != nil {
		return nil, err
	}
	return toolkit.Map(tags, func(tag string) string {
		return strings.Replace(containerName, "*", tag, 1)
	}), nil
}

// TODO: expand this logic to support linux container images as well
func getWindowsContainerImageTags(containerName string, windowsVersion string) ([]string, error) {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	return getWindowsContainerImageTagsFromFS(os.DirFS(projectRoot), containerName, windowsVersion)
}

func getWindowsContainerImageTagsFromFS(fsys fs.FS, containerName, windowsVersion string) ([]string, error) {
	const componentsPath = "parts/common/components.json"
	jsonBytes, err := fs.ReadFile(fsys, componentsPath)
	if err != nil {
		return nil, fmt.Errorf("read %s for Windows image %q (%s): %w", componentsPath, containerName, windowsVersion, err)
	}
	if !gjson.ValidBytes(jsonBytes) {
		return nil, fmt.Errorf("parse %s for Windows image %q (%s): invalid JSON", componentsPath, containerName, windowsVersion)
	}
	containerImages := gjson.GetBytes(jsonBytes, "ContainerImages")
	if !containerImages.IsArray() {
		return nil, fmt.Errorf("%s: ContainerImages must be an array", componentsPath)
	}
	var expectedVersions []string
	for _, containerImage := range containerImages.Array() {
		if !strings.EqualFold(containerImage.Get("downloadURL").String(), containerName) {
			continue
		}
		packages := containerImage.Get("windowsVersions")
		if !packages.IsArray() {
			return nil, fmt.Errorf("%s: windowsVersions for %q must be an array", componentsPath, containerName)
		}
		for _, packageItem := range packages.Array() {
			if pattern := packageItem.Get("windowsSkuMatch"); pattern.Exists() {
				if pattern.Type != gjson.String {
					return nil, fmt.Errorf("%s: windowsSkuMatch for %q must be a string", componentsPath, containerName)
				}
				matched, err := filepath.Match(pattern.String(), windowsVersion)
				if err != nil {
					return nil, fmt.Errorf("%s: invalid windowsSkuMatch %q for %q: %w", componentsPath, pattern.String(), containerName, err)
				}
				if !matched {
					continue
				}
			}
			for _, field := range []string{"latestVersion", "previousLatestVersion"} {
				tag := packageItem.Get(field)
				if field == "previousLatestVersion" && !tag.Exists() {
					continue
				}
				if tag.Type != gjson.String || strings.TrimSpace(tag.String()) == "" {
					return nil, fmt.Errorf("%s: %s for Windows image %q (%s) must be a non-empty string", componentsPath, field, containerName, windowsVersion)
				}
				expectedVersions = append(expectedVersions, tag.String())
			}
		}
	}
	if len(expectedVersions) == 0 {
		return nil, fmt.Errorf("%s: no Windows image tags for %q matching %q", componentsPath, containerName, windowsVersion)
	}
	return expectedVersions, nil
}

func getWindowsEnvVarForName(vhd *config.Image) string {
	return strings.TrimPrefix(vhd.Name, "windows-")
}

func GetServercoreImagesForVHD(vhd *config.Image) ([]string, error) {
	return GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", getWindowsEnvVarForName(vhd))
}

func GetNanoserverImagesForVhd(vhd *config.Image) ([]string, error) {
	return GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", getWindowsEnvVarForName(vhd))
}

func RemoveLeadingV(version string) string {
	if len(version) > 0 && version[0] == 'v' {
		return version[1:]
	}
	return version
}
