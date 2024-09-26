package containerimage

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/components"
)

var (
	//go:embed templates/prefetch.sh.gtpl
	prefetchScript         string
	prefetchScriptTemplate = template.Must(template.New("containerimageprefetch").Parse(prefetchScript))
)

// GeneratePrefetchScript generates the container image prefetch script based on the specified component list.
func GeneratePrefetchScript(list *components.List) ([]byte, error) {
	if list == nil {
		return nil, fmt.Errorf("components list generate opt must be non-nil")
	}
	var args TemplateArgs
	for _, image := range list.Images {
		if !strings.HasSuffix(image.DownloadURL, ":*") {
			return nil, fmt.Errorf("download URL of container image is malformed: %q must end with ':*'; unable to generate prefetch script", image.DownloadURL)
		}
		for _, version := range image.MultiArchVersions {
			if len(version.PrefetchOptimizations.LatestVersion.Binaries) > 0 {
				if version.LatestVersion == "" {
					return nil, fmt.Errorf("container image %q specifies a latestVersion prefetch optimization, but does not have a latestVersion", image.DownloadURL)
				}
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
