package cni

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/env"
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
		url := strings.ReplaceAll(uri.DownloadURL, "${CPU_ARCH}", env.GetArchString())
		url = strings.ReplaceAll(url, "${version}", version)
		name := common.GetURLBase(url)
		path := filepath.Join(pkg.DownloadLocation, name)
		if err := common.EnsureDirectory(pkg.DownloadLocation); err != nil {
			return fmt.Errorf("ensuring directory %q exists: %w", pkg.DownloadLocation, err)
		}
		if err := common.GetTarball(path, url); err != nil {
			return fmt.Errorf("getting tarball: %w", err)
		}
	}
	return nil
}
