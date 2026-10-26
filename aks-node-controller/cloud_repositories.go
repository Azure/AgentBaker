package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
)

const cloudRepoSchemeHTTP = "http"

const cloudRepoSchemeHTTPS = "https"

const ubuntuCloudSource = `
Types: deb
URIs: %s/ubuntu
Suites: %s %s-updates %s-backports %s-security
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
`

const microsoftCloudSource = `
Types: deb
URIs: %s
Suites: %s
Components: main
Architectures: %s
Signed-By: %s
`

const azureLinuxCloudRepo = `[azurelinux-official-%s]
name=Azure Linux Official %s $releasever $basearch
baseurl=%s/azurelinux/$releasever/prod/%s/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
`

var errCloudRepositoryCacheRefresh = errors.New("refresh RepoDepot package cache")

func (c *cloudInitializer) initializeRepositories(ctx context.Context, platform cloudPlatform, endpoint string) error {
	if endpoint == "" {
		slog.Info("RepoDepot endpoint is empty, skipping repository initialization")
		return nil
	}
	endpoint = strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/ubuntu")
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Scheme != cloudRepoSchemeHTTP && parsed.Scheme != cloudRepoSchemeHTTPS) || strings.ContainsAny(endpoint, "\r\n\t ") {
		return errors.New("invalid RepoDepot endpoint")
	}
	switch platform.ID {
	case osReleaseIDUbuntu:
		if err := c.initializeUbuntuRepositories(ctx, platform, endpoint); err != nil {
			return err
		}
		return c.updateRepositoryCache(ctx, "apt-get", "update")
	case osReleaseIDMariner:
		if err := c.initializeMarinerRepositories(endpoint); err != nil {
			return err
		}
		return c.updateRepositoryCache(ctx, "dnf", "makecache", "-y")
	case osReleaseIDAzureLinux:
		if err := c.initializeAzureLinuxRepositories(endpoint); err != nil {
			return err
		}
		return c.updateRepositoryCache(ctx, "dnf", "makecache", "-y")
	default:
		slog.Info("RepoDepot initialization is not needed on immutable OS", "OS", platform.ID)
		return nil
	}
}

func (c *cloudInitializer) repositoryGET(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.repoClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := readCloudResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("RepoDepot request returned HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func (c *cloudInitializer) initializeUbuntuRepositories(ctx context.Context, platform cloudPlatform, endpoint string) error {
	if platform.Version == "" || platform.Codename == "" ||
		strings.ContainsAny(platform.Version+platform.Codename, "\r\n\t ") {
		return errors.New("missing valid Ubuntu VERSION_ID or VERSION_CODENAME")
	}
	if err := c.copyOpenSSLBundle(); err != nil {
		return fmt.Errorf("copy Ubuntu CA bundle: %w", err)
	}
	if err := c.backupUbuntuSources(); err != nil {
		return err
	}
	mainSource := fmt.Sprintf(ubuntuCloudSource,
		endpoint, platform.Codename, platform.Codename, platform.Codename, platform.Codename)
	if err := c.writeFile(defaultAptSourcesDir+"/ubuntu.sources", []byte(mainSource)); err != nil {
		return err
	}
	microsoftSource := endpoint + "/microsoft/ubuntu/" + platform.Version + "/prod"
	if _, err := c.repositoryGET(ctx, microsoftSource); err != nil {
		return fmt.Errorf("check Ubuntu Microsoft repository: %w", err)
	}
	keyPaths, err := c.ubuntuRepositoryKeyrings(ctx, endpoint)
	if err != nil {
		return err
	}
	microsoftConfig := fmt.Sprintf(microsoftCloudSource,
		microsoftSource, platform.Codename, runtime.GOARCH, strings.Join(keyPaths, " "))
	for _, name := range []string{"microsoft-prod", "microsoft-prod-testing"} {
		if err := c.writeFile(defaultAptSourcesDir+"/"+name+".sources", []byte(microsoftConfig)); err != nil {
			return err
		}
	}
	return nil
}

func (c *cloudInitializer) backupUbuntuSources() error {
	sourcesDir, backupDir := c.path(defaultAptSourcesDir), c.path("/etc/apt/backup")
	if err := os.MkdirAll(sourcesDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	sources, err := filepath.Glob(filepath.Join(sourcesDir, "*"))
	if err != nil {
		return err
	}
	sources = append(sources, c.path("/etc/apt/sources.list"))
	for _, source := range sources {
		info, err := os.Stat(source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if err := os.Rename(source, filepath.Join(backupDir, filepath.Base(source))); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *cloudInitializer) ubuntuRepositoryKeyrings(ctx context.Context, endpoint string) ([]string, error) {
	var keyPaths []string
	for _, key := range []string{"microsoft.asc", "msopentech.asc"} {
		body, err := c.repositoryGET(ctx, endpoint+"/keys/"+key)
		if err != nil {
			return nil, fmt.Errorf("download RepoDepot key %s: %w", key, err)
		}
		keyring, err := c.command(ctx, body, "gpg", "--dearmor")
		if err != nil {
			return nil, err
		}
		if len(keyring) == 0 {
			return nil, fmt.Errorf("RepoDepot key %s produced an empty keyring", key)
		}
		path := "/etc/apt/keyrings/" + key + ".gpg"
		if err := c.writeFile(path, keyring); err != nil {
			return nil, err
		}
		keyPaths = append(keyPaths, path)
	}
	return keyPaths, nil
}

func (c *cloudInitializer) initializeMarinerRepositories(endpoint string) error {
	source, err := os.ReadFile(c.path(defaultYumReposDir + "/mariner-extras.repo"))
	if err != nil {
		return err
	}
	for _, repo := range []struct{ id, title string }{
		{"extended", "Extended"}, {"nvidia", "Nvidia"}, {"cloud-native", "Cloud-Native"},
	} {
		content := strings.ReplaceAll(string(source), "extras", repo.id)
		content = strings.ReplaceAll(content, "Extras", repo.title)
		if writeErr := c.writeFile(defaultYumReposDir+"/mariner-"+repo.id+".repo", []byte(content)); writeErr != nil {
			return writeErr
		}
	}
	paths, err := filepath.Glob(c.path(defaultYumReposDir + "/*.repo"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = []byte(strings.ReplaceAll(string(content), "https://packages.microsoft.com", endpoint+"/mariner/packages.microsoft.com"))
		if err := writeCloudFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

func (c *cloudInitializer) initializeAzureLinuxRepositories(endpoint string) error {
	paths, err := filepath.Glob(c.path(defaultYumReposDir + "/azurelinux*"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for _, repo := range []string{"amd", "base", "cloud-native", "extended", "ms-non-oss", "ms-oss", "nvidia"} {
		content := fmt.Sprintf(azureLinuxCloudRepo,
			repo, repo, endpoint, repo)
		if err := c.writeFile(defaultYumReposDir+"/azurelinux-"+repo+".repo", []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

func (c *cloudInitializer) updateRepositoryCache(ctx context.Context, name string, args ...string) error {
	attempt := 0
	dnfError := regexp.MustCompile(`(?m)^([WE]:.*)|([eE]rr.*)$`)
	_, err := common.RetryFetch(ctx, cloudRequestAttempts, nil, func(ctx context.Context) (struct{}, error) {
		if attempt > 0 {
			if err := c.wait(ctx, 5*time.Second); err != nil {
				return struct{}{}, err
			}
		}
		attempt++
		output, err := c.command(ctx, nil, name, args...)
		slog.Info("refreshing RepoDepot package cache", "command", name, "attempt", attempt, "output", string(output))
		if err == nil && name == "dnf" && dnfError.Match(output) {
			err = errors.New("dnf reported repository metadata errors")
		}
		return struct{}{}, err
	})
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return fmt.Errorf("refresh RepoDepot package cache: %w", contextErr)
		}
		return fmt.Errorf("%w: %w", errCloudRepositoryCacheRefresh, err)
	}
	return nil
}
