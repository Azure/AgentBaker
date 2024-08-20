package common

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/env"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/exec"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

func GetTarball(path, fallbackURL string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := DownloadWithCurl(path, fallbackURL); err != nil {
			return err
		}
	}
	return RunCommand(fmt.Sprintf("sudo tar -tzf %s", path), nil)
}

func ExtractTarball(path, targetPath string) error {
	return RunCommand(fmt.Sprintf("sudo tar -zxf %s -C %s", path, targetPath), nil)
}

func DownloadWithCurl(outputPath, downloadURL string) error {
	return RunCommand(fmt.Sprintf("curl -fsSLv %s -o %s", downloadURL, outputPath), &exec.CommandConfig{
		MaxRetries: 10,
		Wait:       to.Ptr(3 * time.Second),
		Timeout:    to.Ptr(time.Minute),
	})
}

func EnsureDirectory(dir string) error {
	return RunCommand(fmt.Sprintf("mkdir -p %s", dir), nil)
}

func Remove(path string) error {
	return RunCommand(fmt.Sprintf("rm -rf %s", path), nil)
}

func RunCommand(cmdString string, cmdConfig *exec.CommandConfig) error {
	cmd, err := exec.NewCommand(cmdString, cmdConfig)
	if err != nil {
		return fmt.Errorf("constructing command: %w", err)
	}
	res, err := cmd.Execute()
	if err != nil {
		return fmt.Errorf("executing command: %w", err)
	}
	if err := res.AsError(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// TODO: actually do this correctly
func GetRelevantDownloadURI(pkg *model.Package) *model.ReleaseDownloadURI {
	switch {
	case env.IsMariner():
		return getMarinerURI(pkg)
	case env.IsUbuntu():
		return getUbuntuURI(pkg)
	default:
		return pkg.DownloadURIs.Default.Current
	}
}

func getMarinerURI(pkg *model.Package) *model.ReleaseDownloadURI {
	if pkg.DownloadURIs.Mariner == nil {
		return getDefaultURI(pkg)
	}
	return pkg.DownloadURIs.Mariner.Current
}

func getUbuntuURI(pkg *model.Package) *model.ReleaseDownloadURI {
	if pkg.DownloadURIs.Ubuntu == nil {
		return getDefaultURI(pkg)
	}
	switch env.UbuntuRelease() {
	case "18.04":
		return pkg.DownloadURIs.Ubuntu.R1804
	case "20.04":
		return pkg.DownloadURIs.Ubuntu.R2004
	case "22.04":
		return pkg.DownloadURIs.Ubuntu.R2204
	case "24.04":
		return pkg.DownloadURIs.Ubuntu.R2404
	default:
		return pkg.DownloadURIs.Ubuntu.Current
	}
}

func getDefaultURI(pkg *model.Package) *model.ReleaseDownloadURI {
	if pkg.DownloadURIs.Default == nil {
		return nil
	}
	return pkg.DownloadURIs.Default.Current
}

func EvaluateDownloadURL(url, version string) string {
	url = strings.ReplaceAll(url, "${CPU_ARCH}", env.GetArchString())
	url = strings.ReplaceAll(url, "${version}", version)
	return url
}
