package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestRCV1PRefreshProductionRender(t *testing.T) {
	source, err := parts.Templates.ReadFile("linux/cloud-init/artifacts/init-aks-cloud.sh")
	require.NoError(t, err)
	rawHash := fmt.Sprintf("%x", sha256.Sum256(source))
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2, datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSUbuntuMinimalContainerd2604Gen2, datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSACLGen2TL,
	} {
		for _, mode := range []string{"scripted", "scriptless", "scriptless-nbc"} {
			t.Run(fmt.Sprintf("%s/%s", distro, mode), func(t *testing.T) {
				nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
				require.NoError(t, err)
				nbc.AgentPoolProfile.Distro = distro
				nbc.EnableScriptlessCSECmd = mode != "scripted"
				nbc.EnableScriptlessNBCCSECmd = mode == "scriptless-nbc"
				baker, err := agent.NewAgentBaker()
				require.NoError(t, err)
				// Exercise the real renderer, including comment removal,
				// compression and ACL's Ignition tar packaging. No Azure calls.
				payload, err := baker.GetNodeBootstrapping(context.Background(), nbc)
				require.NoError(t, err)
				want, err := expectedRCV1PRefreshArtifact(payload.CustomData)
				require.NoError(t, err)
				if mode != "scripted" {
					require.Equal(t, rawHash, want.sha256)
					require.Contains(t, want.origin, "not delivered by customData")
					require.Error(t, validateRCV1PRefreshHash(context.Background(),
						strings.Repeat("0", 64)+"  "+installedRCV1PScript, want))
				} else {
					script, err := rcv1pRefreshPayload(payload.CustomData)
					require.NoError(t, err)
					require.Contains(t, string(script), "update_containerd_ca")
					require.NotEqual(t, rawHash, want.sha256, "normal production comment removal must be honored")
					require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(script)), want.sha256)
					require.Equal(t, "production-rendered customData", want.origin)
					t.Logf("production-rendered refresh SHA256=%s", want.sha256)
					// Raw-source comparison was the hosted-run regression.
					require.Error(t, validateRCV1PRefreshHash(context.Background(),
						rawHash+"  "+installedRCV1PScript, want))
					modified := append(append([]byte(nil), script...), []byte("\n# changed\n")...)
					require.Error(t, validateRCV1PRefreshHash(context.Background(),
						fmt.Sprintf("%x  %s", sha256.Sum256(modified), installedRCV1PScript), want))
				}
				require.NoError(t, validateRCV1PRefreshHash(context.Background(),
					want.sha256+"  "+installedRCV1PScript+"\n", want))
				require.Error(t, validateRCV1PRefreshHash(context.Background(),
					want.sha256+"  /tmp/substitute.sh", want))
				require.Error(t, validateRCV1PRefreshHash(context.Background(), "missing-hash", want))
			})
		}
	}
}

func TestRCV1PRefreshMissingProvenance(t *testing.T) {
	s := &Scenario{Runtime: &ScenarioRuntime{}}
	require.ErrorContains(t, validateInstalledRCV1PScript(context.Background(), s), "not recorded")
	s.Runtime.RCV1PRefreshArtifactErr = errors.New("render decode failure")
	require.ErrorContains(t, validateInstalledRCV1PScript(context.Background(), s), "render decode failure")
}

func TestRCV1PRefreshArtifactRejectsMalformedPayload(t *testing.T) {
	encode := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	compress := func(value string) string {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		_, err := writer.Write([]byte(value))
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		return base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	entry := fmt.Sprintf("\n- path: %s\n  encoding: gzip\n  content: !!binary |\n    %s\n",
		installedRCV1PScript, compress("#!/bin/bash\ntrue\n"))
	for name, customData := range map[string]string{
		"bad base64":              "not base64",
		"empty":                   "",
		"unknown format":          encode("not cloud-init"),
		"invalid gzip":            encode("\x1f\x8bcorrupt"),
		"duplicate script":        encode("#cloud-config\nwrite_files:" + entry + entry),
		"bad encoding":            encode("#cloud-config\nwrite_files:" + strings.Replace(entry, "encoding: gzip", "encoding: text", 1)),
		"missing binary tag":      encode("#cloud-config\nwrite_files:" + strings.Replace(entry, "!!binary", "!!str", 1)),
		"invalid script gzip":     encode("#cloud-config\nwrite_files:" + strings.Replace(entry, compress("#!/bin/bash\ntrue\n"), encode("not gzip"), 1)),
		"empty script":            encode("#cloud-config\nwrite_files:" + strings.Replace(entry, compress("#!/bin/bash\ntrue\n"), compress(""), 1)),
		"invalid yaml":            encode("#cloud-config\nwrite_files: ["),
		"unknown boothook writer": encode("#cloud-boothook\nwrite " + installedRCV1PScript),
		"invalid ignition":        encode(`{"storage":{}}`),
		"external ignition":       encode(`{"ignition":{"version":"3.4.0"},"storage":{"files":[{"path":"/var/lib/ignition/ignition-files.tar","contents":{"source":"https://example.invalid/payload","compression":"gzip"}}]}}`),
		"direct ignition writer":  encode(`{"ignition":{"version":"3.4.0"},"storage":{"files":[{"path":"` + installedRCV1PScript + `"}]}}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := expectedRCV1PRefreshArtifact(customData)
			require.Error(t, err)
		})
	}
	for name, customData := range map[string]string{
		"gzip cloud-config":  compress("#cloud-config\nwrite_files:" + entry),
		"plain cloud-config": encode("#cloud-config\nwrite_files:" + entry),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := rcv1pRefreshPayload(customData)
			require.NoError(t, err)
			require.Equal(t, "#!/bin/bash\ntrue\n", string(got))
		})
	}
}
