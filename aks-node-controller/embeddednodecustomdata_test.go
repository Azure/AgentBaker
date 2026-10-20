package main

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEmbeddedHotfixCloudEligibility(t *testing.T) {
	tests := []struct {
		name, config, command, wantError string
		wantCustom                       bool
	}{
		{name: "custom", config: `{"custom_cloud_config":{"custom_cloud_env_name":"akscustom"}}`, wantCustom: true},
		{name: "case insensitive", config: `{"customCloudConfig":{"customCloudEnvName":"AKSCustom"}}`, wantCustom: true},
		{name: "public", config: `{}`},
		{name: "other cloud", config: `{"custom_cloud_config":{"custom_cloud_env_name":"AzureChinaCloud"}}`},
		{name: "command authoritative", config: `{}`, command: `IS_CUSTOM_CLOUD=true`, wantCustom: true},
		{name: "command excludes custom config", config: `{"custom_cloud_config":{"custom_cloud_env_name":"akscustom"}}`, command: `IS_CUSTOM_CLOUD=false`},
		{name: "command ignores invalid config", config: `{`, command: `IS_CUSTOM_CLOUD=true`, wantCustom: true},
		{name: "invalid config", config: `{`, wantError: "decode node config"},
		{name: "invalid command does not fall back", config: `{"custom_cloud_config":{"custom_cloud_env_name":"akscustom"}}`, command: `echo test`, wantError: "missing or invalid IS_CUSTOM_CLOUD"},
		{name: "legacy custom", command: `IS_CUSTOM_CLOUD="true" /bin/bash provision.sh`, wantCustom: true},
		{name: "legacy public", command: `IS_CUSTOM_CLOUD=false /bin/bash provision.sh`},
		{name: "missing marker", command: `echo IS_CUSTOM_CLOUD`, wantError: "missing or invalid IS_CUSTOM_CLOUD"},
		{name: "invalid marker", command: `IS_CUSTOM_CLOUD=TRUE`, wantError: "missing or invalid IS_CUSTOM_CLOUD"},
		{name: "missing inputs", wantError: "read node config"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			app := &App{
				nodeConfigPath: filepath.Join(directory, "config.json"),
				nbcCmdPath:     filepath.Join(directory, "nbc-cmd.sh"),
			}
			if test.config != "" {
				require.NoError(t, os.WriteFile(app.nodeConfigPath, []byte(test.config), 0o600))
			}
			if test.command != "" {
				require.NoError(t, os.WriteFile(app.nbcCmdPath, []byte(test.command), 0o600))
			}
			actual, err := app.isEmbeddedHotfixCustomCloud()
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.wantCustom, actual)
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomDataSelectsCloudVariant(t *testing.T) {
	for _, platform := range []string{"ubuntu", "mariner"} {
		for _, cloud := range []string{"public", "custom", "unknown"} {
			t.Run(platform+"/"+cloud, func(t *testing.T) {
				directory := t.TempDir()
				script := filepath.Join(directory, "init-aks-cloud.sh")
				common := filepath.Join(directory, "provision_source_distro.sh")
				require.NoError(t, os.WriteFile(script, []byte("original"), 0o600))
				publicFiles := []nodeCustomDataWriteFile{{Path: common, Content: platform}}
				customFiles := append(append([]nodeCustomDataWriteFile{}, publicFiles...), nodeCustomDataWriteFile{
					Path: script, Encoding: encodingBase64, Content: base64.StdEncoding.EncodeToString([]byte("hotfix")),
				})
				publicYAML, err := yaml.Marshal(nodeCustomData{WriteFiles: publicFiles})
				require.NoError(t, err)
				customYAML, err := yaml.Marshal(nodeCustomData{WriteFiles: customFiles})
				require.NoError(t, err)
				name := "generated/rendered_nodecustomdata_" + platform
				files := fstest.MapFS{
					name + ".yml":           &fstest.MapFile{Data: publicYAML},
					name + "_akscustom.yml": &fstest.MapFile{Data: customYAML},
				}
				releasePath := filepath.Join(directory, "os-release")
				id := platform
				if platform == "mariner" {
					id = "azurelinux"
				}
				require.NoError(t, os.WriteFile(releasePath, []byte("ID="+id), 0o600))
				outputPath := filepath.Join(directory, "embedded.yml")
				err = applyEmbeddedNodeCustomData(files, releasePath, outputPath, func() (bool, error) {
					if cloud == "unknown" {
						return false, errors.New("unavailable config")
					}
					return cloud == "custom", nil
				})
				if cloud == "unknown" {
					require.ErrorContains(t, err, "resolve embedded hotfix cloud eligibility")
					assert.NoFileExists(t, common)
					assert.NoFileExists(t, outputPath)
				} else {
					require.NoError(t, err)
					actual, err := os.ReadFile(common)
					require.NoError(t, err)
					assert.Equal(t, platform, string(actual))
					retained, err := os.ReadFile(outputPath)
					require.NoError(t, err)
					if cloud == "custom" {
						assert.Equal(t, customYAML, retained)
					} else {
						assert.Equal(t, publicYAML, retained)
					}
				}
				actual, err := os.ReadFile(script)
				require.NoError(t, err)
				if cloud == "custom" {
					assert.Equal(t, "hotfix", string(actual))
				} else {
					assert.Equal(t, "original", string(actual), "ineligible nodes must not overwrite the script")
				}
			})
		}
	}
}

func TestClassifyNodeCustomDataPlatform(t *testing.T) {
	tests := []struct {
		name     string
		release  string
		expected nodeCustomDataPlatform
	}{
		{"Ubuntu", "ID=ubuntu\n", nodeCustomDataPlatformUbuntu},
		{"Azure Linux", "ID=azurelinux\n", nodeCustomDataPlatformMariner},
		{"OS Guard", "ID=azurelinux\nVARIANT_ID=osguard\n", nodeCustomDataPlatformUnsupported},
		{"ACL variant", "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n", nodeCustomDataPlatformUnsupported},
		{"ACL ID", "ID=azurecontainerlinux\n", nodeCustomDataPlatformUnsupported},
		{"Flatcar", "ID=flatcar\n", nodeCustomDataPlatformUnsupported},
		{"Quoted OS Guard", "ID=\"AZURELINUX\"\nVARIANT_ID=\"OSGUARD\"\n", nodeCustomDataPlatformUnsupported},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			releasePath := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))
			actual, err := classifyNodeCustomDataPlatform(releasePath)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
			if actual == nodeCustomDataPlatformUnsupported {
				outputPath := filepath.Join(t.TempDir(), "embedded-nodecustomdata.yml")
				require.NoError(t, applyEmbeddedNodeCustomData(fstest.MapFS{}, releasePath, outputPath, nil),
					"unsupported platforms must skip without reading any payload")
				assert.NoFileExists(t, outputPath)
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomData(t *testing.T) {
	for _, id := range []string{"ubuntu", "azurelinux"} {
		t.Run(id, func(t *testing.T) {
			directory := t.TempDir()
			outputPath := filepath.Join(directory, "containers", "embedded-nodecustomdata.yml")
			legacyPath := filepath.Join(directory, "containers", "nodecustomdata.yml")
			require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0o755))
			require.NoError(t, os.WriteFile(legacyPath, []byte("legacy payload"), 0o600))
			require.NoError(t, os.WriteFile(outputPath, []byte("previous embedded payload"), 0o600))
			releasePath := filepath.Join(directory, "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte("ID="+id+"\n"), 0o600))
			existing := filepath.Join(directory, "existing.sh")
			missing := filepath.Join(directory, "new", "script.sh")
			require.NoError(t, os.WriteFile(existing, []byte("old"), 0o600))
			payload := "echo hotfixed\n"
			data, err := yaml.Marshal(nodeCustomData{WriteFiles: []nodeCustomDataWriteFile{
				{Path: existing, Encoding: encodingBase64, Content: base64.StdEncoding.EncodeToString([]byte(payload))},
				{Path: missing, Content: payload},
			}})
			require.NoError(t, err)
			platform := id
			if id == osReleaseIDAzureLinux {
				platform = string(nodeCustomDataPlatformMariner)
			}
			files := fstest.MapFS{
				"generated/rendered_nodecustomdata_" + platform + ".yml": &fstest.MapFile{Data: data},
			}
			require.NoError(t, applyEmbeddedNodeCustomData(files, releasePath, outputPath, nil))
			for _, destination := range []string{existing, missing} {
				actual, readErr := os.ReadFile(destination)
				require.NoError(t, readErr)
				assert.Equal(t, payload, string(actual))
			}
			retained, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, data, retained)
			legacy, err := os.ReadFile(legacyPath)
			require.NoError(t, err)
			assert.Equal(t, "legacy payload", string(legacy))
			require.NoError(t, applyEmbeddedNodeCustomData(fstest.MapFS{}, releasePath, outputPath, nil))
			retained, err = os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, data, retained, "a skipped application must preserve the last payload")
			if runtime.GOOS != "windows" {
				info, statErr := os.Stat(missing)
				require.NoError(t, statErr)
				assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomDataErrorsAndRetention(t *testing.T) {
	tests := []struct {
		name      string
		release   string
		payload   string
		missing   string
		wantError string
		retained  bool
	}{
		{name: "missing release", missing: "release", wantError: "read OS release"},
		{name: "unknown OS", release: "ID=other", wantError: "unsupported OS ID"},
		{name: "legacy mariner", release: "ID=mariner", wantError: "unsupported OS ID"},
		{name: "missing ID", release: "VERSION_ID=3.0", wantError: "ID is missing"},
		{name: "missing payload", release: "ID=ubuntu", missing: "payload"},
		{name: "other platform payload only", release: "ID=azurelinux", payload: "write_files: ["},
		{name: "unreadable payload directory", release: "ID=ubuntu", missing: "payload-file", wantError: "read embedded nodecustomdata"},
		{name: "malformed YAML", release: "ID=ubuntu", payload: "write_files: [", wantError: "unmarshal nodecustomdata", retained: true},
		{name: "invalid entry", release: "ID=ubuntu", payload: "write_files:\n- content: invalid\n", wantError: "path is required", retained: true},
		{name: "empty payload", release: "ID=ubuntu", payload: "write_files: []\n", retained: true},
		{name: "output directory blocked", release: "ID=ubuntu", payload: "write_files: []\n", missing: "output-dir", wantError: "create embedded nodecustomdata directory"},
		{name: "output file blocked", release: "ID=ubuntu", payload: "write_files: []\n", missing: "output-file", wantError: "write embedded nodecustomdata"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			outputPath := filepath.Join(directory, "containers", "embedded-nodecustomdata.yml")
			if test.missing == "output-dir" {
				require.NoError(t, os.WriteFile(filepath.Dir(outputPath), []byte("blocked"), 0o600))
			}
			if test.missing == "output-file" {
				require.NoError(t, os.MkdirAll(outputPath, 0o755))
			}
			releasePath := filepath.Join(directory, "os-release")
			if test.missing != "release" {
				require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))
			}
			files := fstest.MapFS{
				"generated/README": &fstest.MapFile{Data: []byte("placeholder")},
			}
			if test.missing != "payload" {
				files["generated/rendered_nodecustomdata_ubuntu.yml"] = &fstest.MapFile{Data: []byte(test.payload)}
			}
			if test.missing == "payload-file" {
				files["generated/rendered_nodecustomdata_ubuntu.yml"].Mode = fs.ModeDir
			}
			err := applyEmbeddedNodeCustomData(files, releasePath, outputPath, nil)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
			assertRetainedEmbeddedPayload(t, outputPath, test.payload, test.retained)
		})
	}
}

func assertRetainedEmbeddedPayload(t *testing.T, outputPath, payload string, retained bool) {
	t.Helper()
	if !retained {
		assert.NoFileExists(t, outputPath)
		return
	}
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, payload, string(data))
	if runtime.GOOS != "windows" {
		info, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}
