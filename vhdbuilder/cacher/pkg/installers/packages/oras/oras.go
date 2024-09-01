package oras

import (
	"fmt"
	"path/filepath"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/common"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/getter"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

const (
	downloadDir = "/opt/oras/downloads"
)

var _ getter.Getter = (*g)(nil)

type g struct{}

func Getter() getter.Getter {
	return &g{}
}

func (g *g) Get(pkg *model.Package) error {
	if err := common.EnsureDirectory(downloadDir); err != nil {
		return fmt.Errorf("ensuring directory %q exists: %w", downloadDir, err)
	}
	if err := common.EnsureDirectory(pkg.DownloadLocation); err != nil {
		return fmt.Errorf("ensuring directory %q exists: %w", pkg.DownloadLocation, err)
	}

	uri := common.GetRelevantDownloadURI(pkg)
	for _, version := range uri.Versions {
		url := common.EvaluateDownloadURL(uri.DownloadURL, version)
		tarName := filepath.Base(url)
		tarPath := filepath.Join(downloadDir, tarName)
		if err := common.GetTarball(tarPath, url); err != nil {
			return fmt.Errorf("getting tarball: %w", err)
		}
		if err := common.ExtractTarball(tarPath, pkg.DownloadLocation); err != nil {
			return fmt.Errorf("extracting tarball: %w", err)
		}
	}

	if err := common.Remove(downloadDir); err != nil {
		return fmt.Errorf("removing directory: %w", err)
	}

	return nil
}
