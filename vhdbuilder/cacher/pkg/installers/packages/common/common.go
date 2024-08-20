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
	untar := fmt.Sprintf("tar -tzf %s", path)
	return runCommand(untar, &exec.CommandConfig{
		MaxRetries: 3,
		Wait:       to.Ptr(time.Second),
		Timeout:    to.Ptr(time.Minute),
	})
}

func DownloadWithCurl(outputPath, downloadURL string) error {
	download := fmt.Sprintf("curl -fsSLv %s -o %s", downloadURL, outputPath)
	return runCommand(download, &exec.CommandConfig{
		MaxRetries: 10,
		Wait:       to.Ptr(3 * time.Second),
		Timeout:    to.Ptr(time.Minute),
	})
}

func runCommand(cmdString string, cmdConfig *exec.CommandConfig) error {
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
	return pkg.DownloadURIs.Mariner.Current
}

func getUbuntuURI(pkg *model.Package) *model.ReleaseDownloadURI {
	if pkg.DownloadURIs.Ubuntu == nil {
		return pkg.DownloadURIs.Default.Current
	}
	return pkg.DownloadURIs.Ubuntu.Current
}

func GetURLBase(url string) string {
	if !strings.Contains(url, "/") {
		return url
	}
	parts := strings.Split(strings.TrimSpace(url), "/")
	return parts[len(parts)-1]
}

func EnsureDirectory(dir string) error {
	return runCommand(fmt.Sprintf("mkdir -p %s", dir), &exec.CommandConfig{
		MaxRetries: 3,
		Timeout:    to.Ptr(time.Second),
		Wait:       to.Ptr(time.Second),
	})
}
