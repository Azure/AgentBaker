package components

import (
	"fmt"
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
		// check if versionsV2 exists
		if packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release)).Exists() {
			versions := packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release))
			for _, version := range versions.Array() {
				// get versions.latestVersion and append to expectedVersions
				expectedVersions = append(expectedVersions, version.Get("latestVersion").String())
				// get versions.previousLatestVersion (if exists) and append to expectedVersions
				if version.Get("previousLatestVersion").Exists() {
					expectedVersions = append(expectedVersions, version.Get("previousLatestVersion").String())
				}
			}
		}
	}
	return expectedVersions
}

func GetWindowsContainerImages(containerName string, windowsVersion string) []string {
	return toolkit.Map(getWindowsContainerImageTags(containerName, windowsVersion), func(tag string) string {
		return strings.Replace(containerName, "*", tag, 1)
	})
}

// TODO: expand this logic to support linux container images as well
func getWindowsContainerImageTags(containerName string, windowsVersion string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here

	// Get the project root dynamically
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename))) // Go up 3 levels from e2e/components/
	componentsPath := filepath.Join(projectRoot, "parts", "common", "components.json")

	jsonBytes, _ := os.ReadFile(componentsPath)

	containerImages := gjson.GetBytes(jsonBytes, "ContainerImages") //fmt.Sprintf("ContainerImages", containerName))

	for _, containerImage := range containerImages.Array() {
		imageDownloadUrl := containerImage.Get("downloadURL").String()
		if strings.EqualFold(imageDownloadUrl, containerName) {
			packages := containerImage.Get("windowsVersions")
			//t.Logf("got packages: %s", packages.String())

			for _, packageItem := range packages.Array() {
				// check if versionsV2 exists
				if packageItem.Get("windowsSkuMatch").Exists() {
					windowsSkuMatch := packageItem.Get("windowsSkuMatch").String()
					matched, err := filepath.Match(windowsSkuMatch, windowsVersion)
					if matched && err == nil {

						// get versions.latestVersion and append to expectedVersions
						expectedVersions = append(expectedVersions, packageItem.Get("latestVersion").String())
						// get versions.previousLatestVersion (if exists) and append to expectedVersions
						if packageItem.Get("previousLatestVersion").Exists() {
							expectedVersions = append(expectedVersions, packageItem.Get("previousLatestVersion").String())
						}
					}
				} else {
					// get versions.latestVersion and append to expectedVersions
					expectedVersions = append(expectedVersions, packageItem.Get("latestVersion").String())
					// get versions.previousLatestVersion (if exists) and append to expectedVersions
					if packageItem.Get("previousLatestVersion").Exists() {
						expectedVersions = append(expectedVersions, packageItem.Get("previousLatestVersion").String())
					}
				}
			}
		}
	}

	return expectedVersions
}

func getWindowsEnvVarForName(vhd *config.Image) string {
	return strings.TrimPrefix(vhd.Name, "windows-")
}

func GetServercoreImagesForVHD(vhd *config.Image) []string {
	return GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", getWindowsEnvVarForName(vhd))
}

func GetNanoserverImagesForVhd(vhd *config.Image) []string {
	return GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", getWindowsEnvVarForName(vhd))
}

func RemoveLeadingV(version string) string {
	if len(version) > 0 && version[0] == 'v' {
		return version[1:]
	}
	return version
}
