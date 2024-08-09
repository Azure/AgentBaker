package container

// List represents a component list (e.g. parts/linux/cloud-init/artifacts/components.json).
type ComponentList struct {
	Images []Image `json:"ContainerImages,omitempty"`
}

// Image represents a container image within a component list.
type Image struct {
	DownloadURL           string   `json:"downloadURL,omitempty"`
	AMD64OnlyVersions     []string `json:"amd64OnlyVersions,omitempty"`
	MultiArchVersions     []string `json:"multiArchVersions,omitempty"`
	PrefetchOptimizations []struct {
		Tag      string   `json:"version,omitempty"`
		Binaries []string `json:"binaries,omitempty"`
	} `json:"prefetchOptimizations,omitempty"`
}

// IsImageVersion returns true iff the specified version is contained within the
// image's multi-arch or amd64-only versions, false otherwise.
func (i *Image) IsKnownVersion(version string) bool {
	for _, v := range i.MultiArchVersions {
		if v == version {
			return true
		}
	}
	for _, v := range i.AMD64OnlyVersions {
		if v == version {
			return true
		}
	}
	return false
}

// TemplateImage represents a container image in terms of its fully-qualified tag,
// as well as the list of binaries within it that are in-scope for prefetch optimization.
// This is used to execute the prefetch template for script generation.
type TemplateImage struct {
	FullyQualifiedTag string
	Binaries          []string
}

// PrefetchTemplateArgs represents the arguments required by the prefetch script template.
type TemplateArgs struct {
	Images []TemplateImage
}
