package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type imageComponent struct {
	DownloadURL         string   `json:"downloadURL"`
	AMD64OnlyVersions   []string `json:"amd64OnlyVersions"`
	MultiArchVersions   []string `json:"multiArchVersions"`
	MultiArchVersionsV2 []struct {
		LatestVersion         string `json:"latestVersion"`
		PreviousLatestVersion string `json:"previousLatestVersion"`
	} `json:"multiArchVersionsV2"`
}

func resolveImageReferences(data []byte, arch string) ([]string, error) {
	if arch != "amd64" && arch != "arm64" {
		return nil, fmt.Errorf("unsupported architecture %q", arch)
	}
	var components struct {
		ContainerImages []imageComponent `json:"ContainerImages"`
	}
	if err := json.Unmarshal(data, &components); err != nil {
		return nil, fmt.Errorf("decode components: %w", err)
	}
	if components.ContainerImages == nil {
		return nil, fmt.Errorf("ContainerImages must be an array")
	}

	var refs []string
	for i, image := range components.ContainerImages {
		var versions []string
		if arch == "amd64" {
			versions = append(versions, image.AMD64OnlyVersions...)
		}
		// An explicitly empty V2 array suppresses legacy versions; null or absent falls back.
		if image.MultiArchVersionsV2 != nil {
			for _, version := range image.MultiArchVersionsV2 {
				versions = append(versions, version.LatestVersion, version.PreviousLatestVersion)
			}
		} else {
			versions = append(versions, image.MultiArchVersions...)
		}
		for _, version := range versions {
			if version == "" {
				continue
			}
			if strings.TrimSpace(image.DownloadURL) == "" {
				return nil, fmt.Errorf("ContainerImages[%d] has versions but no downloadURL", i)
			}
			refs = append(refs, strings.ReplaceAll(image.DownloadURL, "*", version))
		}
	}
	return refs, nil
}
