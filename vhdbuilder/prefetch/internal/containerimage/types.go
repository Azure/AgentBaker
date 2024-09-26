package containerimage

// TemplateImage represents a container image in terms of its fully-qualified tag,
// as well as the list of binaries within it that are in-scope for prefetch optimization.
// This is used to execute the prefetch template for script generation.
type TemplateImage struct {
	Tag      string
	Binaries []string
}

// PrefetchTemplateArgs represents the arguments required by the prefetch script template.
type TemplateArgs struct {
	Images []TemplateImage
}
