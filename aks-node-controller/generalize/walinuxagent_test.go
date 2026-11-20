package generalize

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/agentbaker/aks-node-controller/config"
)

const testWALinuxAgentVersion = "2.16.0.2"

const testWALinuxAgentComponentsJSON = `{
	"Packages": [
		{"name": "other", "downloadURIs": {"default": {"current": {"versionsV2": [{"latestVersion": "9.9"}]}}}},
		{"name": "walinuxagent", "downloadURIs": {"default": {"current": {"versionsV2": [{"latestVersion": "2.16.0.2"}]}}}}
	]
}`

type fakeWireserver struct {
	t               *testing.T
	server          *httptest.Server
	manifestVersion string
	zip             []byte
	requests        atomic.Int32
	// failGoalState makes the goal state endpoint fail this many times.
	failGoalState atomic.Int32
}

func newFakeWireserver(t *testing.T) *fakeWireserver {
	t.Helper()
	w := &fakeWireserver{t: t, manifestVersion: testWALinuxAgentVersion, zip: testZip(t, map[string]string{
		"HandlerManifest.json": "{}",
		"bin/agent.egg":        "egg",
	})}
	w.server = httptest.NewServer(http.HandlerFunc(w.serve))
	t.Cleanup(w.server.Close)
	return w
}

func (w *fakeWireserver) installer() *walinuxagentInstaller {
	return &walinuxagentInstaller{client: w.server.Client(), wireserverURL: w.server.URL, retryDelay: time.Millisecond}
}

func (w *fakeWireserver) serve(rw http.ResponseWriter, r *http.Request) {
	w.requests.Add(1)
	wireHeaders := r.Header.Get("x-ms-agent-name") == "WALinuxAgent" && r.Header.Get("x-ms-version") == "2012-11-30"
	switch r.URL.Path {
	case "/machine/":
		if !wireHeaders {
			http.Error(rw, "missing headers", http.StatusBadRequest)
			return
		}
		if w.failGoalState.Load() > 0 {
			w.failGoalState.Add(-1)
			http.Error(rw, "busy", http.StatusServiceUnavailable)
			return
		}
		// The wireserver XML-escapes the URL; "%3F"-style escapes are also seen in the field.
		fmt.Fprintf(rw, "<GoalState><ExtensionsConfig>%s/ext?a=1&amp;sig=secret</ExtensionsConfig></GoalState>", w.server.URL)
	case "/ext":
		if !wireHeaders {
			http.Error(rw, "missing headers", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(rw, `<Extensions><GAFamilies><GAFamily><Name>Prod</Name><Uris><Uri>%s/manifest?sig=secret&amp;b=2</Uri></Uris></GAFamily></GAFamilies></Extensions>`, w.server.URL)
	case "/manifest":
		fmt.Fprintf(rw, `<?xml version="1.0"?>
<PluginVersionManifest xmlns="http://example.com/ns">
  <Plugins>
    <Plugin><Version>1.0.0.0</Version><Uris><Uri>%[1]s/wrong.zip</Uri></Uris></Plugin>
    <Plugin><Version>%[2]s</Version><Uris><Uri></Uri></Uris></Plugin>
    <Plugin><Version>%[2]s</Version><Uris><Uri> %[1]s/agent.zip?sig=secret </Uri><Uri>%[1]s/second.zip</Uri></Uris></Plugin>
  </Plugins>
</PluginVersionManifest>`, w.server.URL, w.manifestVersion)
	case "/agent.zip":
		if r.URL.Query().Get("sig") != "secret" {
			http.Error(rw, "bad sig", http.StatusForbidden)
			return
		}
		_, _ = rw.Write(w.zip)
	default:
		http.NotFound(rw, r)
	}
}

func testZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := zw.Create(name)
		require.NoError(t, err)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestWALinuxAgentInstaller(t *testing.T) {
	t.Run("downloads the pinned version and retries", func(t *testing.T) {
		ws := newFakeWireserver(t)
		ws.failGoalState.Store(2)
		zipPath := filepath.Join(t.TempDir(), "downloads", "agent.zip")

		require.NoError(t, ws.installer().download(context.Background(), testWALinuxAgentVersion, zipPath))
		got, err := os.ReadFile(zipPath)
		require.NoError(t, err)
		assert.Equal(t, ws.zip, got)
	})

	t.Run("errors do not contain SAS tokens", func(t *testing.T) {
		ws := newFakeWireserver(t)
		ws.zip = []byte("not a zip")
		zipPath := filepath.Join(t.TempDir(), "agent.zip")
		err := ws.installer().download(context.Background(), testWALinuxAgentVersion, zipPath)
		require.Error(t, err)
		assert.NoFileExists(t, zipPath, "an invalid zip must not be saved")

		ws = newFakeWireserver(t)
		installer := ws.installer()
		ws.server.Close()
		err = installer.download(context.Background(), testWALinuxAgentVersion, filepath.Join(t.TempDir(), "agent.zip"))
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret")
	})

	t.Run("stops retrying when the context is done", func(t *testing.T) {
		ws := newFakeWireserver(t)
		ws.failGoalState.Store(100)
		installer := ws.installer()
		installer.retryDelay = time.Hour
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)
		err := installer.download(ctx, testWALinuxAgentVersion, filepath.Join(t.TempDir(), "agent.zip"))
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestRedactURLQueries(t *testing.T) {
	assert.Equal(t, `Get "https://x.blob/a?<redacted>": EOF`, redactURLQueries(`Get "https://x.blob/a?sig=s&se=1": EOF`))
	assert.Equal(t, "https://x.blob/a", stripURLQuery("https://x.blob/a?sig=s"))
}

func TestFindWALinuxAgentZipURL(t *testing.T) {
	manifest := []byte(`<PluginVersionManifest><Plugins>
<Plugin><Version>2.1</Version><Uris><Uri>u21</Uri></Uris></Plugin>
<Plugin><Name><Version>2.2</Version></Name><Uris><Uri>nested</Uri></Uris></Plugin>
<Plugin><Version>2.2</Version><Uris><Uri>u22</Uri></Uris></Plugin>
</Plugins></PluginVersionManifest>`)
	got, err := findWALinuxAgentZipURL(manifest, "2.2")
	require.NoError(t, err)
	assert.Equal(t, "u22", got, "Version must be a direct child of Plugin")

	_, err = findWALinuxAgentZipURL(manifest, "2.21")
	require.Error(t, err, "version match must be exact")
	_, err = findWALinuxAgentZipURL(manifest, "2.")
	require.Error(t, err, "version match must be exact")
	_, err = findWALinuxAgentZipURL([]byte("<bad"), "2.2")
	require.Error(t, err)
}

func TestExtractZip(t *testing.T) {
	writeZip := func(t *testing.T, files map[string]string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "agent.zip")
		require.NoError(t, os.WriteFile(p, testZip(t, files), 0o644))
		return p
	}

	t.Run("replaces the directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "agent")
		old := writeUnderRoot(t, dir, "old.txt", "old")
		require.NoError(t, extractZip(writeZip(t, map[string]string{"bin/agent.egg": "egg"}), dir))
		got, err := os.ReadFile(filepath.Join(dir, "bin", "agent.egg"))
		require.NoError(t, err)
		assert.Equal(t, "egg", string(got))
		assert.NoFileExists(t, old)
	})

	t.Run("writes nothing outside the directory", func(t *testing.T) {
		for _, name := range []string{"../evil", "a/../../evil", "/abs"} {
			base := t.TempDir()
			dir := filepath.Join(base, "agent")
			_ = extractZip(writeZip(t, map[string]string{name: "x"}), dir)
			require.NoError(t, filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && !strings.HasPrefix(p, dir+string(filepath.Separator)) {
					t.Errorf("%s wrote %s", name, p)
				}
				return err
			}))
		}
	})
}

func TestPinnedWALinuxAgentVersion(t *testing.T) {
	for _, tc := range []struct {
		name, components, want string
	}{
		{name: "pinned", components: testWALinuxAgentComponentsJSON, want: "2.16.0.2"},
		{name: "missing package", components: `{"Packages":[]}`},
		{name: "empty", components: `{"Packages":[{"name":"walinuxagent","downloadURIs":{"default":{"current":{"versionsV2":[{"latestVersion":""}]}}}}]}`},
		{name: "null", components: `{"Packages":[{"name":"walinuxagent","downloadURIs":{"default":{"current":{"versionsV2":[{"latestVersion":null}]}}}}]}`},
		{name: "skip", components: `{"Packages":[{"name":"walinuxagent","downloadURIs":{"default":{"current":{"versionsV2":[{"latestVersion":"<SKIP>"}]}}}}]}`},
		{name: "unsafe", components: `{"Packages":[{"name":"walinuxagent","downloadURIs":{"default":{"current":{"versionsV2":[{"latestVersion":"2.1/../x"}]}}}}]}`},
		{name: "invalid JSON", components: `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeUnderRoot(t, root, config.ComponentsFilePath, tc.components)
			got, err := (&Generalizer{rootDir: root}).pinnedWALinuxAgentVersion(map[string]string{"ID": "ubuntu"})
			if tc.want == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPruneStaleWALinuxAgentDirs(t *testing.T) {
	require.Error(t, pruneStaleWALinuxAgentDirs(t.TempDir(), ""))
	require.NoError(t, pruneStaleWALinuxAgentDirs(filepath.Join(t.TempDir(), "missing"), "2.2"))

	state := t.TempDir()
	keep := []string{
		writeUnderRoot(t, state, "WALinuxAgent-2.2/bin", "x"),
		writeUnderRoot(t, state, "WALinuxAgent-2.1.zip", "x"),
		writeUnderRoot(t, state, "WALinuxAgent-plain", "x"),
		writeUnderRoot(t, state, "Microsoft.Azure.Extensions.CustomScript-2.1/bin", "x"),
		writeUnderRoot(t, state, "nested/WALinuxAgent-2.0/bin", "x"),
	}
	stale := []string{
		filepath.Join(state, "WALinuxAgent-2.1"),
		filepath.Join(state, "WALinuxAgent-2.2.1"),
	}
	for _, p := range stale {
		writeUnderRoot(t, p, "bin", "x")
	}
	target := filepath.Join(t.TempDir(), "target")
	writeUnderRoot(t, target, "bin", "x")
	link := filepath.Join(state, "WALinuxAgent-link")
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, pruneStaleWALinuxAgentDirs(state, "2.2"))
	for _, p := range keep {
		assert.FileExists(t, p)
	}
	for _, p := range stale {
		assert.NoDirExists(t, p)
	}
	assert.FileExists(t, filepath.Join(target, "bin"), "symlink targets must not be removed")

	// Without the pinned directory, every versioned directory goes.
	writeUnderRoot(t, state, "WALinuxAgent-2.0/bin", "x")
	require.NoError(t, pruneStaleWALinuxAgentDirs(state, "3.0"))
	assert.NoDirExists(t, filepath.Join(state, "WALinuxAgent-2.0"))
	assert.NoDirExists(t, filepath.Join(state, "WALinuxAgent-2.2"))
}

func TestConfigureWALinuxAgentAutoUpdate(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{name: "flips both", in: "A=1\nAutoUpdate.Enabled=n\nAutoUpdate.UpdateToLatestVersion=y\n",
			want: "A=1\nAutoUpdate.Enabled=y\nAutoUpdate.UpdateToLatestVersion=n\n"},
		{name: "appends missing", in: "A=1", want: "A=1\nAutoUpdate.Enabled=y\nAutoUpdate.UpdateToLatestVersion=n\n"},
		{name: "commented lines do not count", in: "# AutoUpdate.Enabled=n\n",
			want: "# AutoUpdate.Enabled=y\nAutoUpdate.Enabled=y\nAutoUpdate.UpdateToLatestVersion=n\n"},
		{name: "already set", in: "AutoUpdate.Enabled=y\nAutoUpdate.UpdateToLatestVersion=n\n",
			want: "AutoUpdate.Enabled=y\nAutoUpdate.UpdateToLatestVersion=n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "waagent.conf")
			require.NoError(t, os.WriteFile(p, []byte(tc.in), 0o600))
			require.NoError(t, configureWALinuxAgentAutoUpdate(p))
			got, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
			info, err := os.Stat(p)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}
	require.Error(t, configureWALinuxAgentAutoUpdate(filepath.Join(t.TempDir(), "missing")))
}
