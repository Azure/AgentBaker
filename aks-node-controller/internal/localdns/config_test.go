package localdns

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSources is a node with none of the provisioning files: each test writes the ones it needs.
func newTestSources(t *testing.T) *Sources {
	t.Helper()
	return &Sources{
		NodeConfigPath:    filepath.Join(t.TempDir(), "missing-aks-node-controller-config.json"),
		BakedCorefilePath: filepath.Join(t.TempDir(), "missing.corefile"),
		HostsFilePath:     filepath.Join(t.TempDir(), "missing-hosts"),
	}
}

func writeTestNodeConfig(t *testing.T, src *Sources) {
	t.Helper()
	writeTestNodeConfigWithCoreDNSIP(t, src, "")
}

func writeTestNodeConfigWithCoreDNSIP(t *testing.T, src *Sources, coreDNSServiceIP string) {
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
	src.NodeConfigPath = p
}

// writeTestBakedCorefile stands in for the localdns.corefile localdns.sh regenerates at startup,
// which the render reads bootstrap's CoreDNS service IP and hosts plugin choice back from.
func writeTestBakedCorefile(t *testing.T, src *Sources, corefile string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "localdns.corefile")
	require.NoError(t, os.WriteFile(p, []byte(corefile), 0o644))
	src.BakedCorefilePath = p
}

// bakedTestCorefile renders a baked Corefile whose cluster.local block forwards to 10.20.0.10.
func bakedTestCorefile(withHosts bool) string {
	hosts := ""
	if withHosts {
		hosts = "    hosts /etc/localdns/hosts {\n        fallthrough\n    }\n"
	}
	return ".:53 {\n    bind 169.254.10.10\n" + hosts + "    forward . 168.63.129.16 {\n    }\n}\n" +
		"cluster.local:53 {\n    bind 169.254.10.11\n    forward . 10.20.0.10 {\n    }\n}\n"
}

const testProfilePayload = `{
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

// testPoolEntry is one agent pool's entry as LPC and LPS publish it. cacheSeconds sets the root
// zone's cache duration, so entries with different values render different Corefiles.
func testPoolEntry(version string, cacheSeconds int) string {
	return fmt.Sprintf(`{"corefileVersion":%q,"localDnsProfile":{"enableLocalDns":true,`+
		`"vnetDnsOverrides":{".":{"queryLogging":"Error","protocol":"PreferUDP","forwardDestination":"VnetDNS",`+
		`"forwardPolicy":"Sequential","maxConcurrent":1000,"cacheDurationInSeconds":%d,`+
		`"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}},`+
		`"kubeDnsOverrides":{"cluster.local":{"queryLogging":"Error","protocol":"PreferUDP",`+
		`"forwardDestination":"ClusterCoreDNS","forwardPolicy":"Sequential","maxConcurrent":1000,`+
		`"cacheDurationInSeconds":3600,"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}}}}`,
		version, cacheSeconds)
}

// testPayload is the localDNS nodeConfig LPC and LPS publish: entries keyed by agent pool.
func testPayload(entries map[string]string) string {
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

func fetched(payload string) Fetcher {
	return func(context.Context) (string, error) { return payload, nil }
}

func TestApply(t *testing.T) {
	// newNode is a node in pool1 whose bootstrap Corefile forwards cluster.local to 10.20.0.10.
	newNode := func(t *testing.T) *Sources {
		t.Helper()
		src := newTestSources(t)
		writeTestNodeConfig(t, src)
		writeTestBakedCorefile(t, src, bakedTestCorefile(false))
		return src
	}
	apply := func(t *testing.T, src *Sources, payload, out string) (Outcome, error) {
		t.Helper()
		result, err := Apply(context.Background(), fetched(payload), out, "", *src)
		return result.Outcome, err
	}
	// commit stands in for the caller, which writes the .version file once the patch is in effect.
	commit := func(t *testing.T, out, version string) {
		t.Helper()
		require.NoError(t, os.WriteFile(corefileVersionPath(out), []byte(version+"\n"), 0o600))
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
		payload := testPayload(map[string]string{
			"pool1": testPoolEntry("v1", 7200),
			"pool2": testPoolEntry("v2", 60),
		})
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newNode(t), payload, out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		got := read(t, out)
		assert.Contains(t, got, "health-check.localdns.local:53")
		assert.Contains(t, got, "forward . 10.20.0.10")
		assert.Contains(t, got, "cache 7200 {", "pool1's profile, not pool2's, is rendered")
		assertNotWritten(t, corefileVersionPath(out))
	})

	t.Run("same version and content is already current", func(t *testing.T) {
		src, payload := newNode(t), testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)})
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		_, err := apply(t, src, payload, out)
		require.NoError(t, err)
		commit(t, out, "v1")

		outcome, err := apply(t, src, payload, out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeAlreadyCurrent, outcome)
		assert.Equal(t, "v1\n", read(t, corefileVersionPath(out)), "an already current run keeps the version")
	})

	t.Run("same content without a committed version is rendered again", func(t *testing.T) {
		// A run that rendered the Corefile but failed before the caller committed it.
		src, payload := newNode(t), testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)})
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		_, err := apply(t, src, payload, out)
		require.NoError(t, err)

		outcome, err := apply(t, src, payload, out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
	})

	t.Run("matching version with a stale corefile rewrites it and uncommits the version", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		require.NoError(t, os.WriteFile(out, []byte("stale-corefile"), 0o644))
		commit(t, out, "v1")

		outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)}), out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		assert.Contains(t, read(t, out), "cache 7200 {")
		assertNotWritten(t, corefileVersionPath(out))
	})

	t.Run("new version with the same corefile uncommits the old version", func(t *testing.T) {
		// e.g. only the CPU/memory limits changed: they are in the hash but not in the Corefile.
		src := newNode(t)
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		_, err := apply(t, src, testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)}), out)
		require.NoError(t, err)
		commit(t, out, "v1")

		outcome, err := apply(t, src, testPayload(map[string]string{"pool1": testPoolEntry("v2", 7200)}), out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		assertNotWritten(t, corefileVersionPath(out))
	})

	t.Run("a version file that cannot be removed fails without writing", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores the read-only directory this test relies on")
		}
		dir := t.TempDir()
		out := filepath.Join(dir, "localdns.corefile")
		require.NoError(t, os.WriteFile(out, []byte("stale-corefile"), 0o644))
		commit(t, out, "v1")
		require.NoError(t, os.Chmod(dir, 0o555))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

		outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)}), out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "removing localDNS corefile version")
		assert.Equal(t, OutcomeFailed, outcome)
		assert.Equal(t, "stale-corefile", read(t, out))
		assert.Equal(t, "v1\n", read(t, corefileVersionPath(out)))
	})

	t.Run("prints the version and the profile's limits before the outcome", func(t *testing.T) {
		entry := strings.Replace(testPoolEntry("v1", 7200), `"enableLocalDns":true,`,
			`"enableLocalDns":true,"cpuLimitInMilliCores":1505,"memoryLimitInMb":768,`, 1)
		src, payload := newNode(t), testPayload(map[string]string{"pool1": entry})
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		result, err := Apply(context.Background(), fetched(payload), out, "", *src)
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "cpuQuota: 150.5%", "memoryMax: 768M", "rendered"}, result.Lines())

		commit(t, out, "v1")
		result, err = Apply(context.Background(), fetched(payload), out, "", *src)
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "cpuQuota: 150.5%", "memoryMax: 768M", "alreadyCurrent"}, result.Lines())
	})

	t.Run("a limit the profile leaves unset is not printed", func(t *testing.T) {
		// No default is filled in: the caller falls back to the baked localdns.slice value.
		entry := strings.Replace(testPoolEntry("v1", 7200), `"enableLocalDns":true,`,
			`"enableLocalDns":true,"memoryLimitInMb":768,`, 1)
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		result, err := Apply(context.Background(), fetched(testPayload(map[string]string{"pool1": entry})), out, "", *newNode(t))
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "memoryMax: 768M", "rendered"}, result.Lines())
	})

	t.Run("outcomes without a corefile print only the outcome", func(t *testing.T) {
		for _, outcome := range []Outcome{OutcomeNotFound, OutcomeNoCorefileData, OutcomeFailed} {
			result := Result{Outcome: outcome, update: corefileUpdate{desiredVersion: "v1", cpuQuota: "150.0%"}}
			assert.Equal(t, []string{string(outcome)}, result.Lines())
		}
	})

	t.Run("a fetch that finds nothing published is not found", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		result, err := Apply(context.Background(), func(context.Context) (string, error) {
			return "", fmt.Errorf("wrapped: %w", ErrNotPublished)
		}, out, "", *newNode(t))
		require.NoError(t, err)
		assert.Equal(t, OutcomeNotFound, result.Outcome)
		assertNotWritten(t, out)
	})

	t.Run("a fetch that fails is a failure", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		result, err := Apply(context.Background(), func(context.Context) (string, error) {
			return "", assert.AnError
		}, out, "", *newNode(t))
		require.ErrorIs(t, err, assert.AnError)
		assert.Equal(t, OutcomeFailed, result.Outcome)
		assertNotWritten(t, out)
	})

	t.Run("entry without a corefileVersion fails without writing", func(t *testing.T) {
		entry := strings.Replace(testPoolEntry("v1", 7200), `"corefileVersion":"v1",`, "", 1)
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool1": entry}), out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no corefileVersion")
		assert.Equal(t, OutcomeFailed, outcome)
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
			entry:   strings.Replace(testPoolEntry("v1", 7200), `"forwardDestination"`, `"forwardDestinationX"`, 1),
			wantErr: `localDNS profile vnetDnsOverrides zone "." has no forwardDestination`,
		},
		{
			name:    "a renamed cacheDurationInSeconds",
			entry:   strings.Replace(testPoolEntry("v1", 7200), `"cacheDurationInSeconds"`, `"cacheDurationInSecondsX"`, 1),
			wantErr: `localDNS profile vnetDnsOverrides zone "." has no cacheDurationInSeconds`,
		},
		{
			name:    "a renamed override map",
			entry:   strings.Replace(testPoolEntry("v1", 7200), `"kubeDnsOverrides"`, `"kubeDnsOverridesX"`, 1),
			wantErr: "localDNS profile has no kubeDnsOverrides",
		},
		{
			// protojson refuses a null message, so this one fails while parsing. protobuf-go varies the
			// spacing of its own errors, so only the end of the message is matched.
			name:    "an empty zone",
			entry:   withWmtZone(testPoolEntry("v1", 7200), `"wmt":null,`),
			wantErr: "unexpected token null",
		},
		{
			name:    "a custom zone with a renamed field",
			entry:   withWmtZone(testPoolEntry("v1", 7200), strings.Replace(wmtZone, `"forwardDestination"`, `"forwardDestinationX"`, 1)),
			wantErr: `localDNS profile vnetDnsOverrides zone "wmt" has no forwardDestination`,
		},
	} {
		t.Run(tc.name+" fails without writing", func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "localdns.corefile")

			outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool1": tc.entry}), out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Equal(t, OutcomeFailed, outcome)
			assertNotWritten(t, out)
		})
	}

	t.Run("a complete custom zone is rendered", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool1": withWmtZone(testPoolEntry("v1", 7200), wmtZone)}), out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		assert.Contains(t, read(t, out), "wmt:53")
	})

	t.Run("zero is a value, not a missing field", func(t *testing.T) {
		entry := strings.Replace(testPoolEntry("v1", 0), `"maxConcurrent":1000`, `"maxConcurrent":0`, 1)
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool1": entry}), out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		got := read(t, out)
		assert.Contains(t, got, "max_concurrent 0")
		assert.Contains(t, got, "cache 0 {")
	})

	t.Run("entry without a localDnsProfile is no corefile data", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newNode(t), `{"agentPools":{"pool1":{"corefileVersion":"v1"}}}`, out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeNoCorefileData, outcome)
		assertNotWritten(t, out)
	})

	t.Run("another pool's entry is no corefile data", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newNode(t), testPayload(map[string]string{"pool2": testPoolEntry("v2", 60)}), out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeNoCorefileData, outcome)
		assertNotWritten(t, out)
	})

	t.Run("payload without agentPools is no corefile data, even without a known pool", func(t *testing.T) {
		// What LPS serves when no pool has LocalDNS enabled.
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := apply(t, newTestSources(t), "{}", out)
		require.NoError(t, err)
		assert.Equal(t, OutcomeNoCorefileData, outcome)
		assertNotWritten(t, out)
	})
}

// TestRenderReusesBootstrapValues pins that the node-specific values the live-patching payload does
// not carry -- the CoreDNS service IP and whether the hosts plugin is on -- are read back from what
// bootstrap rendered, so a patch only changes what the profile changes.
func TestRenderReusesBootstrapValues(t *testing.T) {
	render := func(t *testing.T, src *Sources) (string, Outcome, error) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		result, err := Apply(context.Background(), fetched(testProfilePayload), out, "", *src)
		got, _ := os.ReadFile(out)
		return string(got), result.Outcome, err
	}

	t.Run("baked corefile CoreDNS IP wins over node config", func(t *testing.T) {
		src := newTestSources(t)
		writeTestNodeConfigWithCoreDNSIP(t, src, "10.99.0.10")
		writeTestBakedCorefile(t, src, bakedTestCorefile(false))

		got, outcome, err := render(t, src)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		assert.Contains(t, got, "forward . 10.20.0.10")
		assert.NotContains(t, got, "10.99.0.10")
	})

	t.Run("node config CoreDNS IP is used without a baked corefile", func(t *testing.T) {
		src := newTestSources(t)
		writeTestNodeConfigWithCoreDNSIP(t, src, "10.99.0.10")

		got, outcome, err := render(t, src)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		assert.Contains(t, got, "forward . 10.99.0.10")
	})

	t.Run("nbc-cmd CoreDNS IP is used without a baked corefile or node config", func(t *testing.T) {
		src := newTestSources(t)
		src.NBCCmdVars = func() (map[string]string, string, error) {
			return map[string]string{
				"KUBELET_NODE_LABELS":    "kubernetes.azure.com/agentpool=pool1",
				"LOCALDNS_COREFILE_BASE": base64.StdEncoding.EncodeToString([]byte(bakedTestCorefile(false))),
			}, "nbc-cmd.sh", nil
		}

		got, outcome, err := render(t, src)
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, outcome)
		assert.Contains(t, got, "forward . 10.20.0.10")
	})

	t.Run("unknown CoreDNS IP refuses the render instead of defaulting", func(t *testing.T) {
		src := newTestSources(t)
		writeTestNodeConfig(t, src)

		got, outcome, err := render(t, src)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CoreDNS service")
		assert.Equal(t, OutcomeFailed, outcome)
		assert.Empty(t, got, "nothing may be written when the CoreDNS IP is unknown")
	})

	t.Run("hosts plugin kept when the baked corefile has it", func(t *testing.T) {
		src := newTestSources(t)
		writeTestNodeConfig(t, src)
		hostsFile := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(hostsFile, []byte("10.0.0.1 mcr.microsoft.com\n"), 0o644))
		src.HostsFilePath = hostsFile
		writeTestBakedCorefile(t, src, strings.ReplaceAll(bakedTestCorefile(true), "/etc/localdns/hosts", hostsFile))

		got, _, err := render(t, src)
		require.NoError(t, err)
		// The template always names /etc/localdns/hosts; the test path only stands in for its presence.
		assert.Contains(t, got, "hosts /etc/localdns/hosts")
	})

	t.Run("hosts plugin stays off when the baked corefile has none, whatever nbc-cmd says", func(t *testing.T) {
		src := newTestSources(t)
		writeTestNodeConfig(t, src)
		hostsFile := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(hostsFile, []byte("10.0.0.1 mcr.microsoft.com\n"), 0o644))
		src.HostsFilePath = hostsFile
		src.NBCCmdVars = func() (map[string]string, string, error) {
			return map[string]string{"SHOULD_ENABLE_HOSTS_PLUGIN": "true"}, "nbc-cmd.sh", nil
		}
		writeTestBakedCorefile(t, src, bakedTestCorefile(false))

		got, _, err := render(t, src)
		require.NoError(t, err)
		assert.NotContains(t, got, "hosts /etc/localdns/hosts")
	})
}

// TestApplyAgentPool pins how apply picks this node's entry. The runtime reconciler passes the Node
// object's agentpool label; that has to work on nodes provisioned without the scriptless files
// (classic CSE), and it has to win over those files so the reconciler's is-current check and the
// apply always agree on the pool.
func TestApplyAgentPool(t *testing.T) {
	payload := testPayload(map[string]string{
		"pool1": testPoolEntry("v1", 7200),
		"pool2": testPoolEntry("v2", 60),
	})

	// classicCSENode has neither provisioning file, as on a classic-CSE node. Its baked Corefile still
	// supplies the CoreDNS service IP for the render.
	classicCSENode := func(t *testing.T) *Sources {
		t.Helper()
		src := newTestSources(t)
		writeTestBakedCorefile(t, src, bakedTestCorefile(false))
		return src
	}
	apply := func(t *testing.T, src *Sources, agentPool string) (string, Result, error) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		result, err := Apply(context.Background(), fetched(payload), out, agentPool, *src)
		got, _ := os.ReadFile(out)
		return string(got), result, err
	}

	t.Run("agent pool selects the pool when no provisioning files exist", func(t *testing.T) {
		got, result, err := apply(t, classicCSENode(t), "pool1")
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, result.Outcome)
		assert.Equal(t, "v1", result.Version())
		assert.Contains(t, got, "cache 7200 {")
	})

	t.Run("without an agent pool a node with no provisioning files fails", func(t *testing.T) {
		got, result, err := apply(t, classicCSENode(t), "")
		require.Error(t, err)
		assert.Equal(t, OutcomeFailed, result.Outcome)
		assert.Empty(t, got)
	})

	t.Run("agent pool wins over the node config file", func(t *testing.T) {
		src := classicCSENode(t)
		writeTestNodeConfig(t, src) // names pool1

		got, result, err := apply(t, src, "pool2")
		require.NoError(t, err)
		assert.Equal(t, OutcomeRendered, result.Outcome)
		assert.Equal(t, "v2", result.Version())
		assert.Contains(t, got, "cache 60 {")
	})

	t.Run("agent pool read from nbc-cmd without a node config file", func(t *testing.T) {
		src := classicCSENode(t)
		src.NBCCmdVars = func() (map[string]string, string, error) {
			return map[string]string{"KUBELET_NODE_LABELS": "kubernetes.azure.com/mode=user,kubernetes.azure.com/agentpool=pool2"}, "nbc-cmd.sh", nil
		}

		got, result, err := apply(t, src, "")
		require.NoError(t, err)
		assert.Equal(t, "v2", result.Version())
		assert.Contains(t, got, "cache 60 {")
	})

	t.Run("agent pool missing from the payload is no corefile data", func(t *testing.T) {
		got, result, err := apply(t, classicCSENode(t), "pool3")
		require.NoError(t, err)
		assert.Equal(t, OutcomeNoCorefileData, result.Outcome)
		assert.Empty(t, got)
	})
}
