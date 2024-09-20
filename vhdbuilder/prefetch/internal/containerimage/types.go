package containerimage

// List represents a container image component list (e.g. parts/linux/cloud-init/artifacts/components.json).
type ComponentList struct {
	Images []Image `json:"ContainerImages,omitempty"`
}

// PrefetchOptimization represents a container image prefetch optimization.
type PrefetchOptimization struct {
	Binaries []string `json:"binaries,omitempty"`
}

// Image represents a container image component. Note that these are the only fields
// we need from container image component definitions to generate prefetch scripts.
type Image struct {
	DownloadURL       string `json:"downloadURL"`
	MultiArchVersions []struct {
		LatestVersion         string `json:"latestVersion"`
		PreviousLatestVersion string `json:"previousLatestVersion"`
		PrefetchOptimizations struct {
			LatestVersion         PrefetchOptimization `json:"latestVersion"`
			PreviousLatestVersion PrefetchOptimization `json:"previousLatestVersion"`
		} `json:"prefetch"`
	} `json:"multiArchVersionsV2,omitempty"`
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
