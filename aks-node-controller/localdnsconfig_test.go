package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func writeLocalDNSTestNodeConfig(t *testing.T, app *App) {
	t.Helper()
	writeLocalDNSTestNodeConfigWithCoreDNSIP(t, app, "")
}

func writeLocalDNSTestNodeConfigWithCoreDNSIP(t *testing.T, app *App, coreDNSServiceIP string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
	require.NoError(t, os.WriteFile(p, []byte(fmt.Sprintf(`{
  "version": "v1",
  "cluster_config": {
    "cluster_network_config": {
      "core_dns_service_ip": %q
    }
  },
  "kubelet_config": {
    "kubelet_node_labels": {
      "kubernetes.azure.com/agentpool": %q
    }
  }
}`, coreDNSServiceIP, "pool1")), 0o600))
	app.nodeConfigPath = p
}

// writeLocalDNSTestBakedCorefile stands in for the localdns.corefile localdns.sh regenerates at
// startup, which the render reads bootstrap's CoreDNS service IP and hosts plugin choice back from.
func writeLocalDNSTestBakedCorefile(t *testing.T, app *App, corefile string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "localdns.corefile")
	require.NoError(t, os.WriteFile(p, []byte(corefile), 0o644))
	app.localDNSBakedCorefilePath = p
}

// bakedLocalDNSTestCorefile renders a baked Corefile whose cluster.local block forwards to 10.20.0.10.
func bakedLocalDNSTestCorefile(withHosts bool) string {
	hosts := ""
	if withHosts {
		hosts = "    hosts /etc/localdns/hosts {\n        fallthrough\n    }\n"
	}
	return ".:53 {\n    bind 169.254.10.10\n" + hosts + "    forward . 168.63.129.16 {\n    }\n}\n" +
		"cluster.local:53 {\n    bind 169.254.10.11\n    forward . 10.20.0.10 {\n    }\n}\n"
}

const localDNSTestProfilePayload = `{
  "agentPools": {
    "pool1": {
      "corefileVersion": "profile-hash",
      "localDnsProfile": {
        "enableLocalDns": true,
        "vnetDnsOverrides": {
          ".": {
            "queryLogging": "Error",
            "protocol": "PreferUDP",
            "forwardDestination": "VnetDNS",
            "forwardPolicy": "Sequential",
            "maxConcurrent": 1000,
            "cacheDurationInSeconds": 3600,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"
          }
        },
        "kubeDnsOverrides": {
          "cluster.local": {
            "queryLogging": "Error",
            "protocol": "PreferUDP",
            "forwardDestination": "ClusterCoreDNS",
            "forwardPolicy": "Sequential",
            "maxConcurrent": 1000,
            "cacheDurationInSeconds": 3600,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"
          }
        }
      }
    }
  }
}`

// localDNSTestPoolEntry is one agent pool's entry as LPC and LPS publish it. cacheSeconds sets the
// root zone's cache duration, so entries with different values render different Corefiles.
func localDNSTestPoolEntry(version string, cacheSeconds int) string {
	return fmt.Sprintf(`{"corefileVersion":%q,"localDnsProfile":{"enableLocalDns":true,`+
		`"vnetDnsOverrides":{".":{"queryLogging":"Error","protocol":"PreferUDP","forwardDestination":"VnetDNS",`+
		`"forwardPolicy":"Sequential","maxConcurrent":1000,"cacheDurationInSeconds":%d,`+
		`"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}},`+
		`"kubeDnsOverrides":{"cluster.local":{"queryLogging":"Error","protocol":"PreferUDP",`+
		`"forwardDestination":"ClusterCoreDNS","forwardPolicy":"Sequential","maxConcurrent":1000,`+
		`"cacheDurationInSeconds":3600,"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}}}}`,
		version, cacheSeconds)
}

// localDNSTestPayload is the localDNS nodeConfig LPC and LPS publish: entries keyed by agent pool.
func localDNSTestPayload(entries map[string]string) string {
	pools := make([]string, 0, len(entries))
	for pool := range entries {
		pools = append(pools, pool)
	}
	sort.Strings(pools)
	parts := make([]string, 0, len(pools))
	for _, pool := range pools {
		parts = append(parts, fmt.Sprintf("%q:%s", pool, entries[pool]))
	}
	return `{"agentPools":{` + strings.Join(parts, ",") + `}}`
}

func TestFetchAndApplyLocalDNSConfig(t *testing.T) {
	// newApp is a node in pool1 whose bootstrap Corefile forwards cluster.local to 10.20.0.10.
	newApp := func(t *testing.T, payload string) *App {
		t.Helper()
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return payload, nil
		}
		return tt.App
	}
	apply := func(t *testing.T, app *App, out string) (localDNSConfigOutcome, error) {
		t.Helper()
		result, err := app.fetchAndApplyLocalDNSConfig(context.Background(), out, "", app.fetchLocalDNSConfig)
		return result.outcome, err
	}
	// commit stands in for the caller, which writes the .version file once the patch is in effect.
	commit := func(t *testing.T, out, version string) {
		t.Helper()
		require.NoError(t, os.WriteFile(localDNSCorefileVersionPath(out), []byte(version+"\n"), 0o600))
	}
	read := func(t *testing.T, path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
	assertNotWritten := func(t *testing.T, out string) {
		t.Helper()
		_, statErr := os.Stat(out)
		assert.True(t, os.IsNotExist(statErr))
	}

	t.Run("renders this pool's profile and leaves the version to the caller", func(t *testing.T) {
		app := newApp(t, localDNSTestPayload(map[string]string{
			"pool1": localDNSTestPoolEntry("v1", 7200),
			"pool2": localDNSTestPoolEntry("v2", 60),
		}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		got := read(t, out)
		assert.Contains(t, got, "health-check.localdns.local:53")
		assert.Contains(t, got, "forward . 10.20.0.10")
		assert.Contains(t, got, "cache 7200 {", "pool1's profile, not pool2's, is rendered")
		assertNotWritten(t, localDNSCorefileVersionPath(out))
	})

	t.Run("same version and content is already current", func(t *testing.T) {
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v1", 7200)}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		_, err := apply(t, app, out)
		require.NoError(t, err)
		commit(t, out, "v1")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigAlreadyCurrent, outcome)
		assert.Equal(t, "v1\n", read(t, localDNSCorefileVersionPath(out)), "an already current run keeps the version")
	})

	t.Run("same content without a committed version is rendered again", func(t *testing.T) {
		// A run that rendered the Corefile but failed before the caller committed it.
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v1", 7200)}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		_, err := apply(t, app, out)
		require.NoError(t, err)

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
	})

	t.Run("matching version with a stale corefile rewrites it and uncommits the version", func(t *testing.T) {
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v1", 7200)}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		require.NoError(t, os.WriteFile(out, []byte("stale-corefile"), 0o644))
		commit(t, out, "v1")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		assert.Contains(t, read(t, out), "cache 7200 {")
		assertNotWritten(t, localDNSCorefileVersionPath(out))
	})

	t.Run("new version with the same corefile uncommits the old version", func(t *testing.T) {
		// e.g. only the CPU/memory limits changed: they are in the hash but not in the Corefile.
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		_, err := apply(t, newApp(t, localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v1", 7200)})), out)
		require.NoError(t, err)
		commit(t, out, "v1")

		outcome, err := apply(t, newApp(t, localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v2", 7200)})), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		assertNotWritten(t, localDNSCorefileVersionPath(out))
	})

	t.Run("a version file that cannot be removed fails without writing", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores the read-only directory this test relies on")
		}
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v1", 7200)}))
		dir := t.TempDir()
		out := filepath.Join(dir, "localdns.corefile")
		require.NoError(t, os.WriteFile(out, []byte("stale-corefile"), 0o644))
		commit(t, out, "v1")
		require.NoError(t, os.Chmod(dir, 0o555))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

		outcome, err := apply(t, app, out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "removing localDNS corefile version")
		assert.Equal(t, outcomeLocalDNSConfigFailed, outcome)
		assert.Equal(t, "stale-corefile", read(t, out))
		assert.Equal(t, "v1\n", read(t, localDNSCorefileVersionPath(out)))
	})

	t.Run("prints the version and the profile's limits before the outcome", func(t *testing.T) {
		entry := strings.Replace(localDNSTestPoolEntry("v1", 7200), `"enableLocalDns":true,`,
			`"enableLocalDns":true,"cpuLimitInMilliCores":1505,"memoryLimitInMb":768,`, 1)
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": entry}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		result, err := app.fetchAndApplyLocalDNSConfig(context.Background(), out, "", app.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "cpuQuota: 150.5%", "memoryMax: 768M", "rendered"}, result.lines())

		commit(t, out, "v1")
		result, err = app.fetchAndApplyLocalDNSConfig(context.Background(), out, "", app.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "cpuQuota: 150.5%", "memoryMax: 768M", "alreadyCurrent"}, result.lines())
	})

	t.Run("a limit the profile leaves unset is not printed", func(t *testing.T) {
		// No default is filled in: the caller falls back to the baked localdns.slice value.
		entry := strings.Replace(localDNSTestPoolEntry("v1", 7200), `"enableLocalDns":true,`,
			`"enableLocalDns":true,"memoryLimitInMb":768,`, 1)
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": entry}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		result, err := app.fetchAndApplyLocalDNSConfig(context.Background(), out, "", app.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "memoryMax: 768M", "rendered"}, result.lines())
	})

	t.Run("outcomes without a corefile print only the outcome", func(t *testing.T) {
		for _, outcome := range []localDNSConfigOutcome{outcomeLocalDNSConfigNotFound, outcomeLocalDNSConfigNoCorefileData, outcomeLocalDNSConfigFailed} {
			result := localDNSConfigResult{outcome: outcome, update: localDNSCorefileUpdate{desiredVersion: "v1", cpuQuota: "150.0%"}}
			assert.Equal(t, []string{string(outcome)}, result.lines())
		}
	})

	t.Run("entry without a corefileVersion fails without writing", func(t *testing.T) {
		entry := strings.Replace(localDNSTestPoolEntry("v1", 7200), `"corefileVersion":"v1",`, "", 1)
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": entry}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, app, out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no corefileVersion")
		assert.Equal(t, outcomeLocalDNSConfigFailed, outcome)
		assertNotWritten(t, out)
	})

	// A field renamed on either side arrives unset, since the payload is parsed with DiscardUnknown.
	// Every field the RP always sets has to be present on every zone, custom zones included.
	const wmtZone = `"wmt":{"queryLogging":"Error","protocol":"PreferUDP","forwardDestination":"VnetDNS",` +
		`"forwardPolicy":"Sequential","maxConcurrent":1000,"cacheDurationInSeconds":3600,` +
		`"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"},`
	withWmtZone := func(entry, zone string) string {
		return strings.Replace(entry, `"vnetDnsOverrides":{`, `"vnetDnsOverrides":{`+zone, 1)
	}
	for _, tc := range []struct {
		name    string
		entry   string
		wantErr string
	}{
		{
			name:    "a renamed forwardDestination",
			entry:   strings.Replace(localDNSTestPoolEntry("v1", 7200), `"forwardDestination"`, `"forwardDestinationX"`, 1),
			wantErr: `localDNS profile vnetDnsOverrides zone "." has no forwardDestination`,
		},
		{
			name:    "a renamed cacheDurationInSeconds",
			entry:   strings.Replace(localDNSTestPoolEntry("v1", 7200), `"cacheDurationInSeconds"`, `"cacheDurationInSecondsX"`, 1),
			wantErr: `localDNS profile vnetDnsOverrides zone "." has no cacheDurationInSeconds`,
		},
		{
			name:    "a renamed override map",
			entry:   strings.Replace(localDNSTestPoolEntry("v1", 7200), `"kubeDnsOverrides"`, `"kubeDnsOverridesX"`, 1),
			wantErr: "localDNS profile has no kubeDnsOverrides",
		},
		{
			// protojson refuses a null message, so this one fails while parsing. protobuf-go varies the
			// spacing of its own errors, so only the end of the message is matched.
			name:    "an empty zone",
			entry:   withWmtZone(localDNSTestPoolEntry("v1", 7200), `"wmt":null,`),
			wantErr: "unexpected token null",
		},
		{
			name:    "a custom zone with a renamed field",
			entry:   withWmtZone(localDNSTestPoolEntry("v1", 7200), strings.Replace(wmtZone, `"forwardDestination"`, `"forwardDestinationX"`, 1)),
			wantErr: `localDNS profile vnetDnsOverrides zone "wmt" has no forwardDestination`,
		},
	} {
		t.Run(tc.name+" fails without writing", func(t *testing.T) {
			app := newApp(t, localDNSTestPayload(map[string]string{"pool1": tc.entry}))
			out := filepath.Join(t.TempDir(), "localdns.corefile")

			outcome, err := apply(t, app, out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Equal(t, outcomeLocalDNSConfigFailed, outcome)
			assertNotWritten(t, out)
		})
	}

	t.Run("a complete custom zone is rendered", func(t *testing.T) {
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": withWmtZone(localDNSTestPoolEntry("v1", 7200), wmtZone)}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		assert.Contains(t, read(t, out), "wmt:53")
	})

	t.Run("zero is a value, not a missing field", func(t *testing.T) {
		entry := strings.Replace(localDNSTestPoolEntry("v1", 0), `"maxConcurrent":1000`, `"maxConcurrent":0`, 1)
		app := newApp(t, localDNSTestPayload(map[string]string{"pool1": entry}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		got := read(t, out)
		assert.Contains(t, got, "max_concurrent 0")
		assert.Contains(t, got, "cache 0 {")
	})

	t.Run("entry without a localDnsProfile is no corefile data", func(t *testing.T) {
		app := newApp(t, `{"agentPools":{"pool1":{"corefileVersion":"v1"}}}`)
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome)
		assertNotWritten(t, out)
	})

	t.Run("another pool's entry is no corefile data", func(t *testing.T) {
		app := newApp(t, localDNSTestPayload(map[string]string{"pool2": localDNSTestPoolEntry("v2", 60)}))
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, app, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome)
		assertNotWritten(t, out)
	})

	t.Run("payload without agentPools is no corefile data, even without a known pool", func(t *testing.T) {
		// What LPS serves when no pool has LocalDNS enabled.
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "missing-aks-node-controller-config.json")
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "missing-nbc-cmd.sh")
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) { return "{}", nil }
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, tt.App, out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome)
		assertNotWritten(t, out)
	})

	t.Run("cli action fails open", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return "", assert.AnError
		}
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "fetch-localdns-config", "--output", filepath.Join(t.TempDir(), "localdns.corefile")})
		assert.Equal(t, 0, exitCode)
	})
}

// TestLocalDNSRenderReusesBootstrapValues pins that the node-specific values the live-patching
// payload does not carry -- the CoreDNS service IP and whether the hosts plugin is on -- are read
// back from what bootstrap rendered, so a patch only changes what the profile changes.
func TestLocalDNSRenderReusesBootstrapValues(t *testing.T) {
	render := func(t *testing.T, tt *TestApp) (string, localDNSConfigOutcome, error) {
		t.Helper()
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return localDNSTestProfilePayload, nil
		}
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		result, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, "", tt.App.fetchLocalDNSConfig)
		got, _ := os.ReadFile(out)
		return string(got), result.outcome, err
	}

	t.Run("baked corefile CoreDNS IP wins over node config", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfigWithCoreDNSIP(t, tt.App, "10.99.0.10")
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))

		got, outcome, err := render(t, tt)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		assert.Contains(t, got, "forward . 10.20.0.10")
		assert.NotContains(t, got, "10.99.0.10")
	})

	t.Run("node config CoreDNS IP is used without a baked corefile", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfigWithCoreDNSIP(t, tt.App, "10.99.0.10")
		tt.App.localDNSBakedCorefilePath = filepath.Join(t.TempDir(), "missing.corefile")

		got, outcome, err := render(t, tt)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome)
		assert.Contains(t, got, "forward . 10.99.0.10")
	})

	t.Run("unknown CoreDNS IP refuses the render instead of defaulting", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		tt.App.localDNSBakedCorefilePath = filepath.Join(t.TempDir(), "missing.corefile")
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "missing-nbc-cmd.sh")

		got, outcome, err := render(t, tt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CoreDNS service")
		assert.Equal(t, outcomeLocalDNSConfigFailed, outcome)
		assert.Empty(t, got, "nothing may be written when the CoreDNS IP is unknown")
	})

	t.Run("hosts plugin kept when the baked corefile has it", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		hostsFile := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(hostsFile, []byte("10.0.0.1 mcr.microsoft.com\n"), 0o644))
		tt.App.localDNSHostsFilePath = hostsFile
		writeLocalDNSTestBakedCorefile(t, tt.App,
			strings.ReplaceAll(bakedLocalDNSTestCorefile(true), "/etc/localdns/hosts", hostsFile))

		got, _, err := render(t, tt)
		require.NoError(t, err)
		// The template always names /etc/localdns/hosts; the test path only stands in for its presence.
		assert.Contains(t, got, "hosts /etc/localdns/hosts")
	})

	t.Run("hosts plugin stays off when the baked corefile has none, whatever nbc-cmd says", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		hostsFile := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(hostsFile, []byte("10.0.0.1 mcr.microsoft.com\n"), 0o644))
		tt.App.localDNSHostsFilePath = hostsFile
		nbcCmd := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		require.NoError(t, os.WriteFile(nbcCmd, []byte("SHOULD_ENABLE_HOSTS_PLUGIN=true /opt/azure/containers/provision_start.sh\n"), 0o600))
		tt.App.nbcCmdPath = nbcCmd
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))

		got, _, err := render(t, tt)
		require.NoError(t, err)
		assert.NotContains(t, got, "hosts /etc/localdns/hosts")
	})
}

// TestLocalDNSApplyAgentPoolFlag pins how apply-localdns-config picks this node's entry. The runtime
// reconciler passes the Node object's agentpool label as --agent-pool; that has to work on nodes
// provisioned without the scriptless files (classic CSE), and it has to win over those files so the
// reconciler's is-current check and the apply always agree on the pool.
func TestLocalDNSApplyAgentPoolFlag(t *testing.T) {
	payload := localDNSTestPayload(map[string]string{
		"pool1": localDNSTestPoolEntry("v1", 7200),
		"pool2": localDNSTestPoolEntry("v2", 60),
	})

	// classicCSENode points both provisioning-file lookups at paths that do not exist, as on a
	// classic-CSE node. Its baked Corefile still supplies the CoreDNS service IP for the render.
	classicCSENode := func(t *testing.T, app *App) {
		t.Helper()
		app.nodeConfigPath = filepath.Join(t.TempDir(), "missing-aks-node-controller-config.json")
		app.nbcCmdPath = filepath.Join(t.TempDir(), "missing-nbc-cmd.sh")
		writeLocalDNSTestBakedCorefile(t, app, bakedLocalDNSTestCorefile(false))
	}
	apply := func(t *testing.T, app *App, agentPool string) (string, localDNSConfigResult, error) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		result, err := app.applyLocalDNSConfig(context.Background(), payload, out, agentPool)
		got, _ := os.ReadFile(out)
		return string(got), result, err
	}

	t.Run("flag selects the pool when no provisioning files exist", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		classicCSENode(t, tt.App)

		got, outcome, err := apply(t, tt.App, "pool1")
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome.outcome)
		assert.Equal(t, "v1", outcome.update.desiredVersion)
		assert.Contains(t, got, "cache 7200 {")
	})

	t.Run("without the flag a node with no provisioning files fails", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		classicCSENode(t, tt.App)

		got, outcome, err := apply(t, tt.App, "")
		require.Error(t, err)
		assert.Equal(t, outcomeLocalDNSConfigFailed, outcome.outcome)
		assert.Empty(t, got)
	})

	t.Run("flag wins over the node config file", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App) // names pool1
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))

		got, outcome, err := apply(t, tt.App, "pool2")
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigRendered, outcome.outcome)
		assert.Equal(t, "v2", outcome.update.desiredVersion)
		assert.Contains(t, got, "cache 60 {")
	})

	t.Run("flag for a pool missing from the payload is no corefile data", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		classicCSENode(t, tt.App)

		got, outcome, err := apply(t, tt.App, "pool3")
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome.outcome)
		assert.Empty(t, got)
	})

	t.Run("cli passes the flag through", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		classicCSENode(t, tt.App)
		configFile := filepath.Join(t.TempDir(), "localdns-config.json")
		require.NoError(t, os.WriteFile(configFile, []byte(payload), 0o600))
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")

		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "apply-localdns-config",
			"--config-file", configFile, "--output", out, "--agent-pool", "pool1"})
		assert.Equal(t, 0, exitCode)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "cache 7200 {")
	})
}

// TestLocalDNSApplyPrintsErrorBeforeOutcome pins the output contract the localDNS handler parses:
// the outcome is always the last line, and on failure the reason precedes it as "error: <reason>".
func TestLocalDNSApplyPrintsErrorBeforeOutcome(t *testing.T) {
	payload := localDNSTestPayload(map[string]string{"pool1": localDNSTestPoolEntry("v1", 7200)})
	run := func(t *testing.T, app *App, agentPool string) ([]string, error) {
		t.Helper()
		configFile := filepath.Join(t.TempDir(), "localdns-config.json")
		require.NoError(t, os.WriteFile(configFile, []byte(payload), 0o600))
		var out bytes.Buffer
		err := app.runApplyLocalDNSConfigCommand(context.Background(), configFile,
			filepath.Join(t.TempDir(), "livepatched.localdns.corefile"), agentPool, &out)
		return strings.Split(strings.TrimRight(out.String(), "\n"), "\n"), err
	}

	t.Run("failure prints the reason before the outcome", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "missing-aks-node-controller-config.json")
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "missing-nbc-cmd.sh")

		lines, err := run(t, tt.App, "")
		require.Error(t, err)
		require.Len(t, lines, 2)
		assert.Equal(t, "error: "+err.Error(), lines[0])
		assert.Equal(t, string(outcomeLocalDNSConfigFailed), lines[1])
	})

	t.Run("success prints the version before the outcome", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))

		lines, err := run(t, tt.App, "pool1")
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", string(outcomeLocalDNSConfigRendered)}, lines)
	})
}

// TestLocalDNSLPSUnavailable pins the contract the localDNS fetch path depends on: the shared
// mapGRPCError classifier must fold exactly the benign LPS codes into errLPSUnavailable, which is
// what turns into the notFound outcome. It guards against a change to mapGRPCError silently
// reclassifying one of these into a hard failure for localDNS.
func TestLocalDNSLPSUnavailable(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		ok   bool
	}{
		{name: "not found", code: codes.NotFound, ok: true},
		{name: "permission denied", code: codes.PermissionDenied, ok: true},
		{name: "unauthenticated", code: codes.Unauthenticated, ok: true},
		{name: "internal", code: codes.Internal, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapGRPCError(status.Error(tt.code, "from LPS"))
			assert.Equal(t, tt.ok, isLPSUnavailable(mapped))
		})
	}
}
