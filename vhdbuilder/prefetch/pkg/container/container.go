package container

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
)

var (
	//go:embed templates/prefetch.sh.gtpl
	prefetchScript         string
	prefetchScriptTemplate = template.Must(template.New("containerimageprefetch").Parse(prefetchScript))
)

// ParseComponents parses the named component list JSON and returns its content as a ComponentList.
func ParseComponents(name string) (*ComponentList, error) {
	bytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read component list %s: %w", name, err)
	}

	var list ComponentList
	if err = json.Unmarshal(bytes, &list); err != nil {
		return nil, fmt.Errorf("unable to unnmarshal component list content: %w", err)
	}

	if len(list.Images) < 1 {
		return nil, fmt.Errorf("parsed list of container images from %s is empty", name)
	}

	return &list, nil
}

// Generate generates and saves the container image prefetch script to disk based on the specified component list and destination path.
func Generate(components *ComponentList, dest string) error {
	var args TemplateArgs
	for _, image := range components.Images {
		if !strings.HasSuffix(image.DownloadURL, ":*") {
			return fmt.Errorf("download URL of container image is malformed: %q must end with ':*'; unable to generate prefetch script", image.DownloadURL)
		}
		if len(image.PrefetchOptimizations) > 0 {
			for _, optimization := range image.PrefetchOptimizations {
				if !image.IsKnownVersion(optimization.Tag) {
					return fmt.Errorf("%q is not a known version of container image %q, unable to generate prefetch script", optimization.Tag, image.DownloadURL)
				}
				args.Images = append(args.Images, TemplateImage{
					FullyQualifiedTag: fmt.Sprintf("%s%s", strings.TrimSuffix(image.DownloadURL, "*"), optimization.Tag),
					Binaries:          optimization.Binaries,
				})
			}
		}
	}

	if len(args.Images) < 1 {
		return fmt.Errorf("no container images found to prefetch")
	}

	var buf bytes.Buffer
	if err := prefetchScriptTemplate.Execute(&buf, args); err != nil {
		return fmt.Errorf("unable to execute container image prefetch template: %w", err)
	}

	fmt.Printf("generated the following container image prefetch script:\n%s\n", buf.String())

	if err := os.WriteFile(dest, buf.Bytes(), os.ModePerm); err != nil {
		return fmt.Errorf("unable to write container image prefetch script to dest %s: %w", dest, err)
	}

	return nil
}
