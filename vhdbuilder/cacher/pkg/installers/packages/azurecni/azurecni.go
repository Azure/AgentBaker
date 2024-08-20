package azurecni

import (
	"fmt"
	"path/filepath"
	"strings"

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
		if err := g.unpackTarball(tarName, pkg.DownloadLocation); err != nil {
			return fmt.Errorf("unpacking tarball %q to temp dir: %w", tarName, err)
		}
	}
	return nil
}

func (g *g) unpackTarball(tarName, downloadDirName string) error {
	tarPath := filepath.Join(downloadDirName, tarName)
	tempDir := filepath.Join(downloadDirName, strings.TrimSuffix(tarName, ".tgz"))
	if err := common.EnsureDirectory(tempDir); err != nil {
		return fmt.Errorf("ensuring directory %q exists: %w", tempDir, err)
	}
	if err := common.ExtractTarball(tarPath, tempDir); err != nil {
		return fmt.Errorf("extracting tarball %q to %q: %w", tarPath, tempDir, err)
	}
	if err := common.Remove(tarPath); err != nil {
		return fmt.Errorf("removing %q: %w", tarPath, err)
	}
	return nil
}
