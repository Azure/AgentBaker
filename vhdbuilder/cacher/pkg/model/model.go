package model

type Components struct {
	ContainerImages []*ContainerImage `json:"containerImages"`
	Packages        []*Package        `json:"packages"`
}

type ContainerImagePrefetchOptimization struct {
	Version  string   `json:"version"`
	Binaries []string `json:"binaries"`
}

type ContainerImage struct {
	Repo                  string                                `json:"downloadURL"`
	AMD64OnlyTags         []string                              `json:"amd64OnlyVersions"`
	MultiArchTags         []string                              `json:"multiArchVersions"`
	PrefetchOptimizations []*ContainerImagePrefetchOptimization `json:"prefetchOptimizations"`
}

type ReleaseDownloadURI struct {
	Versions    []string `json:"versions"`
	DownloadURL string   `json:"downloadURL"`
}

type DefaultOSDistro struct {
	Current *ReleaseDownloadURI `json:"current,omitempty"`
}

type UbuntuOSDistro struct {
	DefaultOSDistro
	R1804 *ReleaseDownloadURI `json:"r1804,omitempty"`
	R2004 *ReleaseDownloadURI `json:"r2004,omitempty"`
	R2204 *ReleaseDownloadURI `json:"r2204,omitempty"`
}

type MarinerOSDistro struct {
	DefaultOSDistro
}

type DownloadURICollection struct {
	Default *DefaultOSDistro `json:"default,omitempty"`
	Ubuntu  *DefaultOSDistro `json:"ubuntu,omitempty"`
	Mariner *DefaultOSDistro `json:"mariner,omitempty"`
}

type Package struct {
	Name                   string                   `json:"name"`
	DownloadLocation       string                   `json:"downloadLocation"`
	DownloadURICollections []*DownloadURICollection `json:"downloadURIs,omitempty"`
}
