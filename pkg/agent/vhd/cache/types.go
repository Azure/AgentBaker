package cache

// OnVHD represents the cached components on the VHD.
type OnVHD struct {
	FromManifest                 *Manifest                 `json:"cachedFromManifest"`
	FromComponentContainerImages map[string]ContainerImage `json:"cachedFromComponentContainerImages"`
	FromComponentPackages        map[string]Package        `json:"cachedFromComponentPackages"`
}

// Manifest represents the manifest.json file.
type Manifest struct {
	NvidiaContainerRuntime Dependency `json:"nvidia-container-runtime"`
	NvidiaDrivers          Dependency `json:"nvidia-drivers"`
	Kubernetes             Dependency `json:"kubernetes"`
}

// Dependency represents fields that occur on manifest.json.
type Dependency struct {
	Versions  []string          `json:"versions"`
	Installed map[string]string `json:"installed"`
	Pinned    map[string]string `json:"pinned"`
	Edge      string            `json:"edge"`
}

// Components represents the components.json file.
type Components struct {
	ContainerImages []ContainerImage `json:"containerImages"`
	Packages        []Package        `json:"Packages"`
}

// ContainerImage represents fields that occur on components.json.
type ContainerImage struct {
	DownloadURL           string                 `json:"downloadURL"`
	MultiArchVersions     []string               `json:"multiArchVersions"`
	Amd64OnlyVersions     []string               `json:"amd64OnlyVersions"`
	PrefetchOptimizations []PrefetchOptimization `json:"prefetchOptimizations"`
}

// PrefetchOptimization represents fields that occur on components.json.
type PrefetchOptimization struct {
	Version  string   `json:"version"`
	Binaries []string `json:"binaries"`
}

// Release represents Release struct that occur on components.json.
type Release struct {
	Versions    []string `json:"versions"`
	DownloadURL string   `json:"downloadURL"`
}

// Package represents Package metadata struct that occur on components.json.
type Package struct {
	Name             string       `json:"name"`
	DownloadLocation string       `json:"downloadLocation"`
	DownloadURIs     DownloadURIs `json:"downloadURIs"`
}

// DownloadURIs represents DownloadURIs struct that occur on components.json.
type DownloadURIs struct {
	Ubuntu  *Ubuntu    `json:"ubuntu,omitempty"`
	Mariner *Mariner   `json:"mariner,omitempty"`
	Default *DefaultOS `json:"default,omitempty"`
}

// DefaultOS represents DefaultOS struct that occur on components.json.
type DefaultOS struct {
	Current *Release `json:"current"`
	// unspecified OS (defaultOS) is not expected to have specific release versions.
	// so we only have `Current`` now.
}

// Mariner represents Mariner struct that occur on components.json.
type Mariner struct {
	Current *Release `json:"current"`
	// additional release versions can be added here, if any
}

// Ubuntu represents Ubuntu struct that occur on components.json.
type Ubuntu struct {
	R1804   *Release `json:"r1804,omitempty"`
	R2004   *Release `json:"r2004,omitempty"`
	R2204   *Release `json:"r2204,omitempty"`
	Current *Release `json:"current,omitempty"`
}
