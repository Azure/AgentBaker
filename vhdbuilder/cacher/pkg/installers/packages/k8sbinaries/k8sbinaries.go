package k8sbinaries

import (
	"fmt"
	"path/filepath"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/common"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/getter"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

var _ getter.Getter = (*g)(nil)

type g struct{}

func Getter() getter.Getter {
	return &g{}
}

func (g *g) Get(pkg *model.Package) error {
	uri := common.GetRelevantDownloadURI(pkg)
	for _, version := range uri.Versions {
		url := common.EvaluateDownloadURL(uri.DownloadURL, version)
		tarName := filepath.Base(url)
		tarPath := filepath.Join(pkg.DownloadLocation, tarName)
		if err := common.EnsureDirectory(pkg.DownloadLocation); err != nil {
			return fmt.Errorf("ensuring directory %q exists: %w", pkg.DownloadLocation, err)
		}
		if err := common.GetTarball(tarPath, url); err != nil {
			return fmt.Errorf("getting tarball: %w", err)
		}
		if err := g.extractBinaries(tarPath, version); err != nil {
			return fmt.Errorf("extracting kube binaries: %w", err)
		}
	}
	return nil
}

func (g *g) extractBinaries(tarPath, version string) error {
	extract := fmt.Sprintf(`tar --transform="s|.*|&-%s|" \
--show-transformed-names -xzvf "%s" \
--strip-components=3 -C /usr/local/bin kubernetes/node/bin/kubelet kubernetes/node/bin/kubectl`,
		version, tarPath)
	if err := common.RunCommand(extract, nil); err != nil {
		return fmt.Errorf("extracing kube binaries for version %q: %w", version, err)
	}
	if err := common.Remove(tarPath); err != nil {
		return fmt.Errorf("removing %q: %w", tarPath, err)
	}
	return nil
}
