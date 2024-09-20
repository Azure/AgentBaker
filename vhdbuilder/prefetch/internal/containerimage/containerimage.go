package containerimage

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
	raw, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read component list %s: %w", name, err)
	}
	var components ComponentList
	if err = json.Unmarshal(raw, &components); err != nil {
		return nil, fmt.Errorf("unable to unnmarshal component list content: %w", err)
	}
	if len(components.Images) < 1 {
		return nil, fmt.Errorf("parsed list of container images from %s is empty", name)
	}
	return &components, nil
}

// Generate generates and saves the container image prefetch script to disk based on the specified component list and destination path.
func Generate(components *ComponentList) ([]byte, error) {
	if components == nil {
		return nil, fmt.Errorf("components list generate opt must be non-nil")
	}
	var args TemplateArgs
	for _, image := range components.Images {
		if !strings.HasSuffix(image.DownloadURL, ":*") {
			return nil, fmt.Errorf("download URL of container image is malformed: %q must end with ':*'; unable to generate prefetch script", image.DownloadURL)
		}
		for _, version := range image.MultiArchVersions {
			if len(version.PrefetchOptimizations.LatestVersion.Binaries) > 0 {
				args.Images = append(args.Images, TemplateImage{
					Tag:      fmt.Sprintf("%s%s", strings.TrimSuffix(image.DownloadURL, "*"), version.LatestVersion),
					Binaries: version.PrefetchOptimizations.LatestVersion.Binaries,
				})
			}
			if len(version.PrefetchOptimizations.PreviousLatestVersion.Binaries) > 0 {
				if version.PreviousLatestVersion == "" {
					return nil, fmt.Errorf("container image %q specifies a previousLatestVersion prefetch optimization, but does not have a previousLatestVersion", image.DownloadURL)
				}
				args.Images = append(args.Images, TemplateImage{
					Tag:      fmt.Sprintf("%s%s", strings.TrimSuffix(image.DownloadURL, "*"), version.PreviousLatestVersion),
					Binaries: version.PrefetchOptimizations.PreviousLatestVersion.Binaries,
				})
			}
		}
	}
	if len(args.Images) < 1 {
		return nil, fmt.Errorf("no container images found to prefetch")
	}
	var buf bytes.Buffer
	if err := prefetchScriptTemplate.Execute(&buf, args); err != nil {
		return nil, fmt.Errorf("unable to execute container image prefetch template: %w", err)
	}
	return buf.Bytes(), nil
}
