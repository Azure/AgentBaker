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
		return nil, fmt.Errorf("components list must be non-nil")
	}
	var args TemplateArgs
	for _, image := range list.Images {
		for _, version := range image.MultiArchVersions {
			hasLatestPrefetch := len(version.PrefetchOptimizations.LatestVersion.Binaries) > 0
			hasPreviousPrefetch := len(version.PrefetchOptimizations.PreviousLatestVersion.Binaries) > 0
			// Only entries that opt into prefetch optimization need a tag-shaped
			// downloadURL. Skip everything else (e.g. digest-pinned test images
			// using the `name@*` + `sha256:...` shape) so they pre-pull cleanly
			// without forcing the strict suffix on unrelated entries.
			if !hasLatestPrefetch && !hasPreviousPrefetch {
				continue
			}
			if !strings.HasSuffix(image.DownloadURL, ":*") {
				return nil, fmt.Errorf("download URL of container image is malformed: %q must end with ':*' to participate in prefetch optimization; unable to generate prefetch script", image.DownloadURL)
			}
			if hasLatestPrefetch {
				if version.LatestVersion == "" {
					return nil, fmt.Errorf("container image %q specifies a latestVersion prefetch optimization, but does not have a latestVersion", image.DownloadURL)
				}
				args.Images = append(args.Images, TemplateImage{
					Tag:      fmt.Sprintf("%s%s", strings.TrimSuffix(image.DownloadURL, "*"), version.LatestVersion),
					Binaries: version.PrefetchOptimizations.LatestVersion.Binaries,
				})
			}
			if hasPreviousPrefetch {
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
