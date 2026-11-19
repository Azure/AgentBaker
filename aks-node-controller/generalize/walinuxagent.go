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
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/config"
)

// The pinned agent is installed after "waagent -deprovision", which removes /var/lib/waagent. The
// daemon then starts this version from disk, so a new node does not download the agent.
const (
	walinuxagentStateDir = "/var/lib/waagent"
	wireServerIP         = "168.63.129.16"

	walinuxagentFetchAttempts = 10
	walinuxagentMaxZipEntry   = 256 << 20
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
	transport, _ := http.DefaultTransport.(*http.Transport)
	return &walinuxagentInstaller{
		client:        &http.Client{Transport: transport.Clone(), Timeout: 60 * time.Second},
		wireserverURL: "http://" + wireServerIP + ":80",
		retryDelay:    5 * time.Second,
	}
}

// installPinnedWALinuxAgent installs the WALinuxAgent version from components.json.
// Flatcar, ACL, and OSGuard use the agent that their OS package provides.
func (g *Generalizer) installPinnedWALinuxAgent(ctx context.Context, osRelease map[string]string) (err error) {
	id := strings.ToLower(osRelease["ID"])
	variant := strings.ToLower(osRelease["VARIANT_ID"])
	if id == config.OSReleaseIDFlatcar || id == config.OSReleaseIDAzureContainerLinux ||
		variant == config.OSVariantIDOSGuard || variant == config.OSReleaseIDAzureContainerLinux {
		slog.Info("skipping WALinuxAgent install; the OS provides the agent", "id", id, "variantID", variant)
		return nil
	}

	version, err := g.pinnedWALinuxAgentVersion()
	if err != nil {
		return err
	}
	slog.Info("installing pinned WALinuxAgent", "version", version)

	restore, err := g.ensureWireserverDNS()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, restore())
	}()

	installer := g.installer
	if installer == nil {
		installer = defaultWALinuxAgentInstaller()
	}
	if err = installer.install(ctx, version, g.path(walinuxagentStateDir), g.path("/opt/walinuxagent/downloads")); err != nil {
		return err
	}

	// Stop the agent so that it does not create WALinuxAgent-* directories again during the prune.
	// "waagent -deprovision" stops the daemon but can leave the unit active.
	if stopErr := g.runCmd(ctx, "systemctl", "stop", "walinuxagent.service", "waagent.service"); stopErr != nil {
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
	if err = appendFile(g.path("/opt/azure/vhd-install.complete"), fmt.Sprintf("  - WALinuxAgent version %s\n", version)); err != nil {
		return err
	}
	slog.Info("installed pinned WALinuxAgent", "version", version)
	return nil
}

func (g *Generalizer) pinnedWALinuxAgentVersion() (string, error) {
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

// ensureWireserverDNS points DNS at the Azure resolver if resolv.conf has no nameserver.
// "waagent -deprovision" clears resolv.conf on some distributions, and the manifest is on Azure Storage.
// The returned function restores the exact earlier state, so the image does not ship the change.
func (g *Generalizer) ensureWireserverDNS() (func() error, error) {
	noop := func() error { return nil }
	conf := g.path("/etc/resolv.conf")
	content, err := os.ReadFile(conf)
	if err == nil && bytes.Contains(content, []byte("nameserver")) {
		return noop, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", conf, err)
	}
	nameserver := []byte("nameserver " + wireServerIP + "\n")

	info, err := os.Lstat(conf)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		//nolint:gosec // resolv.conf must be world-readable
		if err = os.WriteFile(conf, nameserver, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", conf, err)
		}
		slog.Info("temporarily set DNS to Azure DNS", "path", conf)
		return func() error { return removeIfExists(conf) }, nil
	case err != nil:
		return nil, fmt.Errorf("stat %s: %w", conf, err)
	case info.Mode()&fs.ModeSymlink != 0:
		return g.ensureWireserverDNSInSymlinkTarget(conf, nameserver)
	default:
		if err = os.WriteFile(conf, nameserver, info.Mode().Perm()); err != nil {
			return nil, fmt.Errorf("write %s: %w", conf, err)
		}
		slog.Info("temporarily set DNS to Azure DNS", "path", conf)
		return func() error { return writeFileKeepMode(conf, content, info.Mode().Perm()) }, nil
	}
}

// ensureWireserverDNSInSymlinkTarget writes to the symlink target, so the link stays in place.
func (g *Generalizer) ensureWireserverDNSInSymlinkTarget(conf string, nameserver []byte) (func() error, error) {
	raw, err := os.Readlink(conf)
	if err != nil {
		return nil, fmt.Errorf("read link %s: %w", conf, err)
	}
	target := raw
	if filepath.IsAbs(target) {
		target = g.path(target)
	} else {
		target = filepath.Join(filepath.Dir(conf), target)
	}
	targetInfo, statErr := os.Stat(target)
	original, readErr := os.ReadFile(target)
	targetExists := statErr == nil && readErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat %s: %w", target, statErr)
	}
	if statErr == nil && readErr != nil {
		return nil, fmt.Errorf("read %s: %w", target, readErr)
	}
	mode := fs.FileMode(0o644)
	if targetExists {
		mode = targetInfo.Mode().Perm()
	}
	if err = os.WriteFile(target, nameserver, mode); err != nil {
		return nil, fmt.Errorf("write %s: %w", target, err)
	}
	slog.Info("temporarily set DNS to Azure DNS", "path", target)
	return func() error {
		var restoreErr error
		if targetExists {
			restoreErr = writeFileKeepMode(target, original, mode)
		} else {
			restoreErr = removeIfExists(target)
		}
		// Recreate the link with its raw value in case a tool replaced it during the install.
		if current, linkErr := os.Readlink(conf); linkErr != nil || current != raw {
			restoreErr = errors.Join(restoreErr, removeIfExists(conf), os.Symlink(raw, conf))
		}
		return restoreErr
	}, nil
}

func (w *walinuxagentInstaller) install(ctx context.Context, version, stateDir, downloadDir string) error {
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
	installDir := filepath.Join(stateDir, "WALinuxAgent-"+version)
	if err = extractZip(zipData, installDir); err != nil {
		return fmt.Errorf("extract WALinuxAgent %s: %w", version, err)
	}
	// The zip copy records which artifact the image ships.
	if err = os.MkdirAll(downloadDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", downloadDir, err)
	}
	zipPath := filepath.Join(downloadDir, "WALinuxAgent-"+version+".zip")
	//nolint:gosec // matches the mode of the other image artifacts
	if err = os.WriteFile(zipPath, zipData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", zipPath, err)
	}
	slog.Info("installed WALinuxAgent", "version", version, "path", installDir)
	return nil
}

func (w *walinuxagentInstaller) fetch(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= walinuxagentFetchAttempts; attempt++ {
		var body []byte
		body, lastErr = w.fetchOnce(ctx, rawURL, headers)
		if lastErr == nil {
			return body, nil
		}
		if attempt == walinuxagentFetchAttempts {
			break
		}
		timer := time.NewTimer(w.retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("fetch %s after %d attempts: %s", stripURLQuery(rawURL), walinuxagentFetchAttempts,
		redactURLQueries(lastErr.Error()))
}

func (w *walinuxagentInstaller) fetchOnce(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, unwrapURLError(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, unwrapURLError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, unwrapURLError(err)
	}
	return body, nil
}

// xmlNode is a generic element tree. The manifest is matched by local element names.
type xmlNode struct {
	XMLName xml.Name
	Text    string    `xml:",chardata"`
	Nodes   []xmlNode `xml:",any"`
}

func (n *xmlNode) walk(fn func(*xmlNode) bool) bool {
	for i := range n.Nodes {
		if !fn(&n.Nodes[i]) || !n.Nodes[i].walk(fn) {
			return false
		}
	}
	return true
}

// findWALinuxAgentZipURL returns the first Uri of the first Plugin whose direct Version child is version.
func findWALinuxAgentZipURL(manifest []byte, version string) (string, error) {
	var root xmlNode
	if err := xml.Unmarshal(manifest, &root); err != nil {
		return "", fmt.Errorf("parse WALinuxAgent manifest: %w", err)
	}
	var found string
	root.walk(func(plugin *xmlNode) bool {
		if plugin.XMLName.Local != "Plugin" || !pluginHasVersion(plugin, version) {
			return true
		}
		var uri string
		plugin.walk(func(n *xmlNode) bool {
			if n.XMLName.Local == "Uri" {
				uri = strings.TrimSpace(n.Text)
				return false
			}
			return true
		})
		if uri == "" {
			return true
		}
		found = uri
		return false
	})
	if found == "" {
		return "", fmt.Errorf("version %s not found in WALinuxAgent manifest", version)
	}
	return found, nil
}

func pluginHasVersion(plugin *xmlNode, version string) bool {
	for _, child := range plugin.Nodes {
		if child.XMLName.Local == "Version" {
			return child.Text == version
		}
	}
	return false
}

// extractZip writes files 0644 and directories 0755, and writes symlink entries as regular files.
// os.Root rejects entries that resolve outside dir.
func extractZip(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if !filepath.IsLocal(filepath.FromSlash(f.Name)) {
			return fmt.Errorf("zip entry %q would extract outside %s", f.Name, dir)
		}
	}
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		if strings.HasSuffix(f.Name, "/") {
			if err = root.MkdirAll(name, 0o755); err != nil {
				return err
			}
			continue
		}
		if parent := path.Dir(name); parent != "." {
			if err = root.MkdirAll(parent, 0o755); err != nil {
				return err
			}
		}
		if err = extractZipFile(root, f, name); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(root *os.Root, f *zip.File, name string) error {
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer src.Close()
	dst, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	n, err := io.CopyN(dst, src, walinuxagentMaxZipEntry+1)
	closeErr := dst.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("extract zip entry %s: %w", f.Name, err)
	}
	if n > walinuxagentMaxZipEntry {
		return fmt.Errorf("zip entry %s is larger than %d bytes", f.Name, walinuxagentMaxZipEntry)
	}
	return closeErr
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
	info, err := os.Stat(confPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", confPath, err)
	}
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
	return writeFileKeepMode(confPath, []byte(conf), info.Mode().Perm())
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

func appendFile(p, content string) error {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", p, err)
	}
	_, err = f.WriteString(content)
	return errors.Join(err, f.Close())
}

func writeFileKeepMode(p string, content []byte, mode fs.FileMode) error {
	//nolint:gosec // p is a fixed image path; mode is the existing file mode
	if err := os.WriteFile(p, content, mode); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

func removeIfExists(p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", p, err)
	}
	return nil
}

func stripURLQuery(rawURL string) string {
	before, _, _ := strings.Cut(rawURL, "?")
	return before
}

func redactURLQueries(s string) string {
	return urlQueryPattern.ReplaceAllString(s, "$1?<redacted>")
}

// unwrapURLError drops the full request URL from net/http errors, because it can contain a SAS token.
func unwrapURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
