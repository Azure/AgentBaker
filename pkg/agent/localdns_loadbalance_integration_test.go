package agent

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

// TestLocalDNSLoadBalanceCacheHits exercises the rendered Corefile with a real
// CoreDNS binary. Run with COREDNS_TEST_BINARY=/path/to/coredns; dig must be on PATH.
// The negative control proves upstream ordering alone cannot satisfy the test.
func TestLocalDNSLoadBalanceCacheHits(t *testing.T) {
	binary := os.Getenv("COREDNS_TEST_BINARY")
	if binary == "" {
		t.Skip("set COREDNS_TEST_BINARY to run the DNS integration test")
	}
	_, err := exec.LookPath("dig")
	require.NoError(t, err)
	port := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		p := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
		require.NoError(t, listener.Close())
		return p
	}
	start := func(t *testing.T, corefile string) {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "Corefile")
		require.NoError(t, os.WriteFile(path, []byte(corefile), 0600))
		log, err := os.Create(filepath.Join(dir, "coredns.log"))
		require.NoError(t, err)
		cmd := exec.Command(binary, "-conf", path)
		cmd.Stdout, cmd.Stderr = log, log
		require.NoError(t, cmd.Start())
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = log.Close()
			if t.Failed() {
				output, _ := os.ReadFile(log.Name())
				t.Logf("CoreDNS output:\n%s", output)
			}
		})
	}
	query := func(p, name, rrtype string) (string, error) {
		output, err := exec.Command("dig", "@127.0.0.1", "-p", p, name, rrtype,
			"+time=1", "+tries=1", "+noall", "+answer").CombinedOutput()
		return string(output), err
	}
	upstreamPort := port()
	start(t, fmt.Sprintf(`.:%s {
    bind 127.0.0.1
    template IN A {
        answer "{{ .Name }} 300 IN A 192.0.2.1"
        answer "{{ .Name }} 300 IN A 192.0.2.2"
        answer "{{ .Name }} 300 IN A 192.0.2.3"
    }
    template IN AAAA {
        answer "{{ .Name }} 300 IN AAAA 2001:db8::1"
        answer "{{ .Name }} 300 IN AAAA 2001:db8::2"
        answer "{{ .Name }} 300 IN AAAA 2001:db8::3"
    }
}`, upstreamPort))
	wait := func(t *testing.T, p string) {
		t.Helper()
		require.Eventually(t, func() bool {
			answer, err := query(p, "ready.example.test.", "A")
			return err == nil && strings.Contains(answer, "192.0.2.1")
		}, 10*time.Second, 100*time.Millisecond)
	}
	wait(t, upstreamPort)

	for _, listener := range []string{"vnet", "kube"} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/loadbalance=%t", listener, enabled), func(t *testing.T) {
				config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
				overrides := map[string]*datamodel.LocalDNSOverrides{".": {
					ForwardDestination: "VnetDNS", ForwardPolicy: "Sequential", Protocol: "ForceTCP",
					MaxConcurrent: to.Int32Ptr(1000), CacheDurationInSeconds: to.Int32Ptr(300), ServeStale: "Disable",
				}}
				profile := &datamodel.LocalDNSProfile{EnableLocalDNS: true}
				if listener == "vnet" {
					profile.VnetDNSOverrides = overrides
				} else {
					profile.KubeDNSOverrides = overrides
				}
				config.AgentPoolProfile.LocalDNSProfile = profile
				corefile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
				require.NoError(t, err)
				localPort := port()
				// Keep the rendered forwarding/cache/plugin configuration; remap only
				// sockets to unprivileged loopback ports for the local test process.
				corefile = strings.NewReplacer(
					"169.254.10.10 169.254.10.11", "127.0.0.1",
					"169.254.10.10", "127.0.0.1", "169.254.10.11", "127.0.0.1",
					":53 {", ":"+localPort+" {", ":8181", ":"+port(), ":9253", ":"+port(),
					"168.63.129.16", "127.0.0.1:"+upstreamPort,
				).Replace(corefile)
				if !enabled {
					corefile = strings.ReplaceAll(corefile, "    loadbalance\n", "")
				}
				start(t, corefile)
				wait(t, localPort)
				for _, rrtype := range []string{"A", "AAAA"} {
					name := strings.ToLower(rrtype) + ".example.test."
					_, err := query(localPort, name, rrtype)
					require.NoError(t, err)
					time.Sleep(1100 * time.Millisecond)
					firstAddresses := map[string]bool{}
					for i := 0; i < 64; i++ {
						answer, err := query(localPort, name, rrtype)
						require.NoError(t, err)
						var addresses []string
						for _, line := range strings.Split(answer, "\n") {
							fields := strings.Fields(line)
							if len(fields) != 5 || fields[3] != rrtype {
								continue
							}
							ttl, err := strconv.Atoi(fields[1])
							require.NoError(t, err)
							require.Greater(t, ttl, 0)
							require.Less(t, ttl, 300, "TTL must prove a cache hit, not another upstream response")
							addresses = append(addresses, fields[4])
						}
						require.Len(t, addresses, 3, answer)
						firstAddresses[addresses[0]] = true
						sort.Strings(addresses)
						expected := []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"}
						if rrtype == "AAAA" {
							expected = []string{"2001:db8::1", "2001:db8::2", "2001:db8::3"}
						}
						require.Equal(t, expected, addresses)
					}
					if enabled {
						require.Greater(t, len(firstAddresses), 1, "cached responses must vary their first address")
					} else {
						require.Len(t, firstAddresses, 1, "negative control must reproduce fixed cached ordering")
					}
					t.Logf("%s: %d distinct first addresses across 64 cache hits", rrtype, len(firstAddresses))
				}
			})
		}
	}
}
