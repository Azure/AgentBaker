package cache

// CachedOnVHD represents the cached components on VHD.
type OnVHD struct {
	FromManifest                 *Manifest                 `json:"cachedFromManifest"`
	FromComponentContainerImages map[string]ContainerImage `json:"cachedFromComponentContainerImages"`
	FromComponentDownloadedFiles map[string]DownloadFile   `json:"cachedFromComponentDownloadedFiles"`
}

// Manifest represents the manifest.json file.
type Manifest struct {
	Containerd             Dependency `json:"containerd"`
	Runc                   Dependency `json:"runc"`
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

// Versions of components on manifest.json.
type Versions struct {
	Versions []string `json:"versions"`
}

// Components represents the components.json file.
type Components struct {
	ContainerImages []ContainerImage `json:"containerImages"`
	DownloadFiles   []DownloadFile   `json:"downloadFiles"`
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

// DownloadFile represents DownloadFile fields that occur on components.json.
type DownloadFile struct {
	FileName         string   `json:"fileName"`
	DownloadLocation string   `json:"downloadLocation"`
	DownloadURL      string   `json:"downloadURL"`
	Versions         []string `json:"versions"`
}
