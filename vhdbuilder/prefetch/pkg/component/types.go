package component

// vhdbuilder/packer/components.json
type List struct {
	ContainerImages []ContainerImage `json:"ContainerImages,omitempty"`
}

type ContainerImage struct {
	DownloadURL       string   `json:"downloadURL,omitempty"`
	AMD64OnlyVersions []string `json:"amd64OnlyVersions,omitempty"`
	MultiArchVersions []string `json:"multiArchVersions,omitempty"`
}
