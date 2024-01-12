package cni

type CNIContainerImage struct {
	FullyQualifiedTag string
	Binaries          []string
}

type CNIPrefetchList map[string][]string

type PrefetchTemplateArgs struct {
	Images []CNIContainerImage
}
