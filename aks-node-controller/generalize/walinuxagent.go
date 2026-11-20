package generalize

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/config"
)

// The pinned agent is downloaded before "waagent -deprovision", while DNS still works, and extracted
// after it, because deprovision removes /var/lib/waagent. The daemon then starts this version from
// disk, so a new node does not download the agent.
const (
	walinuxagentStateDir      = "/var/lib/waagent"
	walinuxagentFetchAttempts = 10
)

var (
	walinuxagentVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)
	extensionsConfigPattern    = regexp.MustCompile(`<ExtensionsConfig>([^<]+)</ExtensionsConfig>`)
	gaFamilyURIPattern         = regexp.MustCompile(`(?s)<GAFamily>.*?<Uri>([^<]+)</Uri>`)
	urlQueryPattern            = regexp.MustCompile(`(https?://[^\s"'<>]*?)\?[^\s"'>]*`)
)

// walinuxagentInstaller downloads the agent from the wireserver manifest. Manifest and blob URLs
// carry SAS tokens, so it never logs a URL query.
type walinuxagentInstaller struct {
	client        *http.Client
	wireserverURL string
	retryDelay    time.Duration
}

func defaultWALinuxAgentInstaller() *walinuxagentInstaller {
	return &walinuxagentInstaller{
		client:        &http.Client{Timeout: 60 * time.Second},
		wireserverURL: "http://168.63.129.16",
		retryDelay:    5 * time.Second,
	}
}

// downloadPinnedWALinuxAgent saves the pinned agent zip. A failed download stops the build before
// any other step changes the VM.
func (g *Generalizer) downloadPinnedWALinuxAgent(ctx context.Context, osRelease map[string]string) error {
	version, err := g.pinnedWALinuxAgentVersion(osRelease)
	if err != nil || version == "" {
		return err
	}
	installer := g.installer
	if installer == nil {
		installer = defaultWALinuxAgentInstaller()
	}
	return installer.download(ctx, version, g.walinuxagentZipPath(version))
}

// installPinnedWALinuxAgent extracts the zip that downloadPinnedWALinuxAgent saved.
func (g *Generalizer) installPinnedWALinuxAgent(ctx context.Context, osRelease map[string]string) error {
	version, err := g.pinnedWALinuxAgentVersion(osRelease)
	if err != nil || version == "" {
		return err
	}
	installDir := g.path(filepath.Join(walinuxagentStateDir, "WALinuxAgent-"+version))
	if err = extractZip(g.walinuxagentZipPath(version), installDir); err != nil {
		return fmt.Errorf("extract WALinuxAgent %s: %w", version, err)
	}

	// Stop the agent so that it does not create WALinuxAgent-* directories again during the prune.
	// "waagent -deprovision" stops the daemon but can leave the unit active.
	if _, stopErr := g.runCmd(ctx, "systemctl", "stop", "walinuxagent.service", "waagent.service"); stopErr != nil {
		slog.Info("stop WALinuxAgent services", "error", stopErr)
	}

	// The build VM agent can update itself from the wireserver before deprovision stops it.
	if err = pruneStaleWALinuxAgentDirs(g.path(walinuxagentStateDir), version); err != nil {
		return err
	}
	// AutoUpdate.Enabled=y makes the daemon run the newest agent on disk.
	// AutoUpdate.UpdateToLatestVersion=n prevents downloads from the network.
	if err = configureWALinuxAgentAutoUpdate(g.path("/etc/waagent.conf")); err != nil {
		return err
	}
	manifest := g.path("/opt/azure/vhd-install.complete")
	f, err := os.OpenFile(manifest, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", manifest, err)
	}
	_, err = fmt.Fprintf(f, "  - WALinuxAgent version %s\n", version)
	if err = errors.Join(err, f.Close()); err != nil {
		return fmt.Errorf("write %s: %w", manifest, err)
	}
	slog.Info("installed pinned WALinuxAgent", "version", version)
	return nil
}

func (g *Generalizer) walinuxagentZipPath(version string) string {
	return g.path(filepath.Join("/opt/walinuxagent/downloads", "WALinuxAgent-"+version+".zip"))
}

// pinnedWALinuxAgentVersion returns the WALinuxAgent version from components.json. It returns "" on
// Flatcar, ACL, and OSGuard, which use the agent that their OS package provides.
func (g *Generalizer) pinnedWALinuxAgentVersion(osRelease map[string]string) (string, error) {
	id := strings.ToLower(osRelease["ID"])
	variant := strings.ToLower(osRelease["VARIANT_ID"])
	if id == config.OSReleaseIDFlatcar || id == config.OSReleaseIDAzureContainerLinux ||
		variant == config.OSVariantIDOSGuard || variant == config.OSReleaseIDAzureContainerLinux {
		slog.Info("skipping pinned WALinuxAgent; the OS provides the agent", "id", id, "variantID", variant)
		return "", nil
	}
	componentsPath := g.path(config.ComponentsFilePath)
	data, err := os.ReadFile(componentsPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", componentsPath, err)
	}
	var components struct {
		Packages []struct {
			Name         string `json:"name"`
			DownloadURIs struct {
				Default struct {
					Current struct {
						VersionsV2 []struct {
							LatestVersion string `json:"latestVersion"`
						} `json:"versionsV2"`
					} `json:"current"`
				} `json:"default"`
			} `json:"downloadURIs"`
		} `json:"Packages"`
	}
	if err = json.Unmarshal(data, &components); err != nil {
		return "", fmt.Errorf("parse %s: %w", componentsPath, err)
	}
	for _, p := range components.Packages {
		if p.Name != "walinuxagent" {
			continue
		}
		versions := p.DownloadURIs.Default.Current.VersionsV2
		if len(versions) == 0 {
			break
		}
		v := versions[0].LatestVersion
		if v == "" || v == "<SKIP>" {
			break
		}
		if !walinuxagentVersionPattern.MatchString(v) {
			return "", fmt.Errorf("walinuxagent version %q in %s contains unexpected characters", v, componentsPath)
		}
		return v, nil
	}
	return "", fmt.Errorf("could not read walinuxagent version from %s", componentsPath)
}

func (w *walinuxagentInstaller) download(ctx context.Context, version, zipPath string) error {
	wireHeaders := map[string]string{"x-ms-agent-name": "WALinuxAgent", "x-ms-version": "2012-11-30"}

	goalStateURL := w.wireserverURL + "/machine/?comp=goalstate"
	slog.Info("fetching goal state", "url", goalStateURL)
	goalState, err := w.fetch(ctx, goalStateURL, wireHeaders)
	if err != nil {
		return err
	}
	match := extensionsConfigPattern.FindSubmatch(goalState)
	if match == nil {
		return errors.New("no <ExtensionsConfig> element found in goal state")
	}
	extensionsConfigURL := strings.TrimSpace(string(match[1]))
	if unescaped, unescapeErr := url.PathUnescape(extensionsConfigURL); unescapeErr == nil {
		extensionsConfigURL = unescaped
	}
	extensionsConfigURL = html.UnescapeString(extensionsConfigURL)

	slog.Info("fetching extensions config", "url", stripURLQuery(extensionsConfigURL))
	extensionsConfig, err := w.fetch(ctx, extensionsConfigURL, wireHeaders)
	if err != nil {
		return err
	}
	match = gaFamilyURIPattern.FindSubmatch(extensionsConfig)
	if match == nil {
		return errors.New("no GAFamily manifest URI found in extensions config")
	}
	manifestURL := html.UnescapeString(strings.TrimSpace(string(match[1])))

	slog.Info("fetching WALinuxAgent manifest", "url", stripURLQuery(manifestURL))
	manifest, err := w.fetch(ctx, manifestURL, nil)
	if err != nil {
		return err
	}
	zipURL, err := findWALinuxAgentZipURL(manifest, version)
	if err != nil {
		return err
	}

	slog.Info("downloading WALinuxAgent", "version", version, "url", stripURLQuery(zipURL))
	zipData, err := w.fetch(ctx, zipURL, nil)
	if err != nil {
		return err
	}
	if _, err = zip.NewReader(bytes.NewReader(zipData), int64(len(zipData))); err != nil {
		return fmt.Errorf("open WALinuxAgent %s zip: %w", version, err)
	}
	// The zip copy also records which artifact the image ships.
	if err = os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(zipPath), err)
	}
	//nolint:gosec // matches the mode of the other image artifacts
	if err = os.WriteFile(zipPath, zipData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", zipPath, err)
	}
	slog.Info("downloaded WALinuxAgent", "version", version, "path", zipPath)
	return nil
}

// fetch is the only exit for request errors, so it redacts every URL query that they contain.
func (w *walinuxagentInstaller) fetch(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	for attempt := 1; ; attempt++ {
		body, err := w.fetchOnce(ctx, rawURL, headers)
		if err == nil {
			return body, nil
		}
		if attempt == walinuxagentFetchAttempts {
			return nil, fmt.Errorf("fetch %s after %d attempts: %s", stripURLQuery(rawURL), attempt,
				redactURLQueries(err.Error()))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(w.retryDelay):
		}
	}
}

func (w *walinuxagentInstaller) fetchOnce(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// findWALinuxAgentZipURL returns the first Uri of the first Plugin whose Version is version. The
// layout is the one WALinuxAgent reads; element names match in any namespace.
func findWALinuxAgentZipURL(manifest []byte, version string) (string, error) {
	var m struct {
		Plugins []struct {
			Version string   `xml:"Version"`
			URIs    []string `xml:"Uris>Uri"`
		} `xml:"Plugins>Plugin"`
	}
	if err := xml.Unmarshal(manifest, &m); err != nil {
		return "", fmt.Errorf("parse WALinuxAgent manifest: %w", err)
	}
	for _, p := range m.Plugins {
		if p.Version == version && len(p.URIs) > 0 && strings.TrimSpace(p.URIs[0]) != "" {
			return strings.TrimSpace(p.URIs[0]), nil
		}
	}
	return "", fmt.Errorf("version %s not found in WALinuxAgent manifest", version)
}

// extractZip replaces dir with the zip content. zip.Reader exposes only local names, and os.CopyFS
// rejects symlink entries, so nothing is written outside dir.
func extractZip(zipPath, dir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	if err = os.RemoveAll(dir); err != nil {
		return err
	}
	return os.CopyFS(dir, zr)
}

// pruneStaleWALinuxAgentDirs removes top-level WALinuxAgent-* directories, except the pinned version.
// It does not touch symlinks, regular files, or other agent state.
func pruneStaleWALinuxAgentDirs(stateDir, version string) error {
	if version == "" {
		return errors.New("pinned WALinuxAgent version is required")
	}
	entries, err := os.ReadDir(stateDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", stateDir, err)
	}
	keep := "WALinuxAgent-" + version
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "WALinuxAgent-") || e.Name() == keep {
			continue
		}
		p := filepath.Join(stateDir, e.Name())
		if err = os.RemoveAll(p); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}
		slog.Info("removed stale WALinuxAgent directory", "path", p)
	}
	return nil
}

func configureWALinuxAgentAutoUpdate(confPath string) error {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", confPath, err)
	}
	conf := string(data)
	conf = setWAAgentConfOption(conf, "AutoUpdate.Enabled", "n", "y")
	conf = setWAAgentConfOption(conf, "AutoUpdate.UpdateToLatestVersion", "y", "n")
	if conf == string(data) {
		return nil
	}
	// The file exists, so WriteFile keeps its mode.
	//nolint:gosec // fixed image path
	if err = os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", confPath, err)
	}
	return nil
}

// setWAAgentConfOption replaces key=from with key=to everywhere, and appends key=to if no line sets key.
func setWAAgentConfOption(conf, key, from, to string) string {
	conf = strings.ReplaceAll(conf, key+"="+from, key+"="+to)
	for _, line := range strings.Split(conf, "\n") {
		if strings.HasPrefix(line, key+"=") {
			return conf
		}
	}
	if conf != "" && !strings.HasSuffix(conf, "\n") {
		conf += "\n"
	}
	return conf + key + "=" + to + "\n"
}

func stripURLQuery(rawURL string) string {
	before, _, _ := strings.Cut(rawURL, "?")
	return before
}

func redactURLQueries(s string) string {
	return urlQueryPattern.ReplaceAllString(s, "$1?<redacted>")
}
