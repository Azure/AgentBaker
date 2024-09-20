package components

// List represents a container image component list (e.g. parts/linux/cloud-init/artifacts/components.json).
type List struct {
	Images []Image `json:"ContainerImages,omitempty"`
}

// Image represents a container image component. Note that these are the only fields
// we need from container image component definitions to generate prefetch scripts.
type Image struct {
	DownloadURL       string `json:"downloadURL"`
	MultiArchVersions []struct {
		LatestVersion         string `json:"latestVersion"`
		PreviousLatestVersion string `json:"previousLatestVersion,omitempty"`
		PrefetchOptimizations struct {
			LatestVersion         ContainerImagePrefetchOptimization `json:"latestVersion"`
			PreviousLatestVersion ContainerImagePrefetchOptimization `json:"previousLatestVersion,omitempty"`
		} `json:"containerImagePrefetch,omitempty"`
	} `json:"multiArchVersionsV2,omitempty"`
}

// ContainerImagePrefetchOptimization represents a container image prefetch optimization.
type ContainerImagePrefetchOptimization struct {
	Binaries []string `json:"binaries,omitempty"`
}
