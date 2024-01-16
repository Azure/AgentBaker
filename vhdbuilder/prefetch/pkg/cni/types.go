package cni

// CNIContainerImage represents a container image used for CNI-related functionality.
type CNIContainerImage struct {
	// FullyQualifiedTag is the fully qualified tag identifier of the image (e.g. mcr.microsoft.com/containernetworking/cni-dropgz:v0.0.4.2).
	FullyQualifiedTag string
	// Binaries is a list of absolute paths to binaries which should be prefetched from within the container image.
	Binaries []string
}

// PrefetchTemplateArgs represents the arguments required by the prefetch script template.
type PrefetchTemplateArgs struct {
	Images []CNIContainerImage
}

// CNIPrefetchList represents a mapping between container images and their respective binaries that need to be prefetched.
type CNIPrefetchList map[string][]string
