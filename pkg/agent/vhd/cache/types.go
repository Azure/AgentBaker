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

// DownloadURI represents DownloadURI struct that occur on components.json.
type DownloadURI struct {
	Current Release `json:"current"`
	R1804   Release `json:"1804,omitempty"`
}

// Package represents Package struct that occur on components.json.
type Package struct {
	Name                   string                 `json:"name"`
	DownloadLocation       string                 `json:"downloadLocation"`
	DownloadURIs           map[string]DownloadURI `json:"downloadURIs"`
	TargetContainerRuntime string                 `json:"targetContainerRuntime,omitempty"`
}
