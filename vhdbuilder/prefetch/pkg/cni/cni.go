package cni

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/pkg/component"
)

var (
	//go:embed cni-prefetch.sh.gtpl
	prefetchScript         string
	prefetchScriptTemplate = template.Must(template.New("cniprefetch").Parse(prefetchScript))

	prefetchList = CNIPrefetchList{
		"mcr.microsoft.com/containernetworking/cni-dropgz:*": []string{"dropgz"},
		"mcr.microsoft.com/oss/calico/pod2daemon-flexvol:*":  []string{"usr/local/bin/flexvol"},
		"mcr.microsoft.com/oss/calico/cni:*": []string{
			"opt/cni/bin/bandwidth",
			"opt/cni/bin/calico",
			"opt/cni/bin/calico-ipam",
			"opt/cni/bin/flannel",
			"opt/cni/bin/host-local",
			"opt/cni/bin/install",
			"opt/cni/bin/loopback",
			"opt/cni/bin/portmap",
			"opt/cni/bin/tuning",
		},
	}
)

// can later generalize this to an interface if we need to prefetch other non-CNI related images/binaries
func Generate(components *component.List, dest string) error {
	var templateArgs PrefetchTemplateArgs
	for _, image := range components.ContainerImages {
		if binaries, shouldPrefetch := prefetchList[image.DownloadURL]; shouldPrefetch {
			// only prefetch mutli-arch versions for now
			for _, tag := range image.MultiArchVersions {
				fullTag := fmt.Sprintf("%s%s", strings.TrimSuffix(image.DownloadURL, "*"), tag)
				templateArgs.Images = append(templateArgs.Images, CNIContainerImage{
					FullyQualifiedTag: fullTag,
					Binaries:          binaries,
				})
			}
		}
	}

	if len(templateArgs.Images) < 1 {
		return fmt.Errorf("no container images found to prefetch")
	}

	var buf bytes.Buffer
	if err := prefetchScriptTemplate.Execute(&buf, templateArgs); err != nil {
		return fmt.Errorf("unable to execute CNI prefetch template: %w", err)
	}

	fmt.Printf("generated CNI prefetch script:\n%s\n", buf.String())

	if err := os.WriteFile(dest, buf.Bytes(), os.ModePerm); err != nil {
		return fmt.Errorf("unable to write generated prefetch script to dest %s: %w", dest, err)
	}

	return nil
}
