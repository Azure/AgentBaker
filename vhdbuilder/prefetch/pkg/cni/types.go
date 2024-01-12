package cni

type CNIContainerImage struct {
	FullyQualifiedTag string
	Binaries          []string
}

type PrefetchTemplateArgs struct {
	Images []CNIContainerImage
}

type CNIPrefetchList map[string][]string
