// Package cache provides types and functionality for reasoning about the content cached on a particular VHD version
// through both components.json and manifest.json.
package cache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/Azure/agentbaker/parts"
)

const (
	manifestFilePartPath   = "linux/cloud-init/artifacts/manifest.json"
	componentsFilePartPath = "linux/cloud-init/artifacts/components.json"
)

//nolint:gochecknoglobals
var onVHD *OnVHD

//nolint:gochecknoinits
func init() {
	var err error
	onVHD, err = loadOnVHD()
	if err != nil {
		panic(err)
	}
	if onVHD == nil {
		panic("onVHD is nil after initialization")
	}
	if onVHD.FromManifest == nil {
		panic("onVHD.FromManifest is nil after initialization")
	}
	if onVHD.FromComponentContainerImages == nil {
		panic("onVHD.FromComponentContainerImages is nil after initialization")
	}
	if onVHD.FromComponentDownloadedFiles == nil {
		panic("onVHD.FromComponentDownloadedFiles is nil after initialization")
	}
}

// GetOnVHD returns the set of components and binaries that have been cached on the
// particular VHD corresponding to the given agentbakersvc version.
func GetOnVHD() *OnVHD {
	return onVHD
}

func loadOnVHD() (*OnVHD, error) {
	// init manifest content
	manifest, err := getManifest()
	if err != nil {
		return nil, fmt.Errorf("initializing manifest.json content: %w", err)
	}

	// init components content
	components, err := getComponents()
	if err != nil {
		return nil, fmt.Errorf("initializing components.json content: %w", err)
	}
	componentContainerImages := make(map[string]ContainerImage)
	for _, image := range components.ContainerImages {
		imageName, nameErr := getContainerImageNameFromURL(image.DownloadURL)
		if nameErr != nil {
			return nil, fmt.Errorf("error getting component name from URL: %w", nameErr)
		}
		componentContainerImages[imageName] = image
	}
	componentDownloadFiles := make(map[string]DownloadFile)
	for _, file := range components.DownloadFiles {
		fileName, nameErr := getFileNameFromURL(file.DownloadURL)
		if nameErr != nil {
			return nil, fmt.Errorf("error getting component name from URL: %w", nameErr)
		}
		componentDownloadFiles[fileName] = file
	}

	return &OnVHD{
		FromManifest:                 manifest,
		FromComponentContainerImages: componentContainerImages,
		FromComponentDownloadedFiles: componentDownloadFiles,
	}, nil
}

func getManifest() (*Manifest, error) {
	manifestContent, err := parts.Templates.ReadFile(manifestFilePartPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest.json file part: %w", err)
	}
	manifestContent = bytes.ReplaceAll(manifestContent, []byte("#EOF"), []byte(""))
	var manifest Manifest
	if err = json.Unmarshal(manifestContent, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshalling manifest.json file part content: %w", err)
	}
	return &manifest, nil
}

func getComponents() (*Components, error) {
	componentsContent, err := parts.Templates.ReadFile(componentsFilePartPath)
	if err != nil {
		return nil, fmt.Errorf("reading components.json file part: %w", err)
	}
	componentsContent = bytes.ReplaceAll(componentsContent, []byte("#EOF"), []byte(""))
	var components Components
	if err = json.Unmarshal(componentsContent, &components); err != nil {
		return nil, fmt.Errorf("unmarshalling components.json file part content: %w", err)
	}
	return &components, nil
}

func getContainerImageNameFromURL(downloadURL string) (string, error) {
	// example URL: "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*",
	// getting the data between the last / and the last :
	parts := strings.Split(downloadURL, "/")
	if len(parts) == 0 || len(parts[len(parts)-1]) == 0 {
		return "", fmt.Errorf("container image component URL is not in the expected format: %s", downloadURL)
	}
	lastPart := parts[len(parts)-1]
	component := strings.TrimSuffix(lastPart, ":*")
	return component, nil
}

func getFileNameFromURL(downloadURL string) (string, error) {
	// example URL: "https://acs-mirror.azureedge.net/cni-plugins/v*/binaries",
	url, err := url.Parse(downloadURL) // /cni-plugins/v*/binaries
	if err != nil {
		return "", fmt.Errorf("download file image URL is not in the expected format: %s", downloadURL)
	}
	urlSplit := strings.Split(url.Path, "/") // ["", cni-plugins, v*, binaries]
	componentIndx, minURLSplit := 1, 2
	if len(urlSplit) < minURLSplit {
		return "", fmt.Errorf("download file image URL is not in the expected format: %s", downloadURL)
	}
	componentName := urlSplit[componentIndx]
	if componentName == "" {
		return "", fmt.Errorf("component name is empty in the URL: %s", downloadURL)
	}
	return componentName, nil
}
