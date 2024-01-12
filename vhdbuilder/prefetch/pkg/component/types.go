package component

// List represents a component list (e.g. vhdbuilder/packer/components.json).
type List struct {
	ContainerImages []ContainerImage `json:"ContainerImages,omitempty"`
}

// ContainerImage represents a container image component within a component list.
type ContainerImage struct {
	DownloadURL       string   `json:"downloadURL,omitempty"`
	AMD64OnlyVersions []string `json:"amd64OnlyVersions,omitempty"`
	MultiArchVersions []string `json:"multiArchVersions,omitempty"`
}
