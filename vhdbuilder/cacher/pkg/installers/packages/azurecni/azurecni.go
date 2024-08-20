package azurecni

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
		tarName := filepath.Base(url)
		path := filepath.Join(pkg.DownloadLocation, tarName)
		if err := common.EnsureDirectory(pkg.DownloadLocation); err != nil {
			return fmt.Errorf("ensuring directory %q exists: %w", pkg.DownloadLocation, err)
		}
		if err := common.GetTarball(path, url); err != nil {
			return fmt.Errorf("getting tarball: %w", err)
		}
		if err := unpackTarballToDownloadsDIR(tarName, pkg.DownloadLocation); err != nil {
			return fmt.Errorf("unpacking tarball %q to temp dir: %w", tarName, err)
		}
	}
	return nil
}

func unpackTarballToDownloadsDIR(tarName, downloadDirName string) error {
	tempDir := filepath.Join(downloadDirName, strings.TrimSuffix(tarName, ".tgz"))
	if err := common.EnsureDirectory(tempDir); err != nil {
		return fmt.Errorf("ensuring directory %q exists: %w", tempDir, err)
	}
	tarPath := filepath.Join(downloadDirName, tarName)
	untar := fmt.Sprintf("tar -xzf %s -C %s", tarPath, tempDir)
	if err := common.RunCommand(untar, nil); err != nil {
		return err
	}
	removeTarBall := fmt.Sprintf("rm -rf %s", tarPath)
	if err := common.RunCommand(removeTarBall, nil); err != nil {
		return err
	}
	return nil
}
