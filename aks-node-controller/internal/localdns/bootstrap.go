package localdns

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

// This file reads back what bootstrap decided for this node: its agent pool, the CoreDNS service IP
// and whether it runs the hosts plugin. The live-patching payload carries none of these.

const (
	agentPoolLabel = "kubernetes.azure.com/agentpool"
	// unresolvedCoreDNSServiceIP stands in for the CoreDNS service IP when no bootstrap source on
	// the node carries it. The parser would otherwise substitute its 10.0.0.10 default, which is
	// wrong for any cluster with a custom service CIDR; rendering this marker instead lets a profile
	// that actually forwards to CoreDNS be refused rather than silently misrouted.
	unresolvedCoreDNSServiceIP = "unresolved-coredns-service-ip"
)

func nodeAgentPoolName(src Sources) (string, error) {
	path := src.NodeConfigPath
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("node config not found, trying nbc-cmd.sh fallback", "path", path)
			return nodeAgentPoolNameFromNBCCmd(src)
		}
		return "", fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	if cfg == nil {
		return "", fmt.Errorf("node config %s could not be parsed", path)
	}
	agentPool := cfg.GetKubeletConfig().GetKubeletNodeLabels()[agentPoolLabel]
	if agentPool == "" {
		return "", fmt.Errorf("node config has no %s kubelet node label", agentPoolLabel)
	}
	return agentPool, nil
}

// coreDNSServiceIP returns the CoreDNS service IP for the cluster.local blocks, taken from what
// bootstrap already rendered for this node rather than recomputed.
//
// The baked Corefile comes first because it exists in every provisioning mode and is exactly what
// this node has been serving. The AKSNodeConfig JSON and nbc-cmd.sh only exist for scriptless
// provisioning, so they are fallbacks. When none of them carries the IP the marker is returned and
// the caller refuses the render if the profile needs it.
func coreDNSServiceIP(src Sources) string {
	bakedPath := src.bakedCorefilePath()
	if baked, err := os.ReadFile(bakedPath); err == nil {
		if ip := coreDNSServiceIPFromCorefile(string(baked)); ip != "" {
			slog.Info("loaded CoreDNS service IP from the baked localDNS corefile", "coreDnsServiceIp", ip, "path", bakedPath)
			return ip
		}
	}
	if ip := coreDNSServiceIPFromNodeConfig(src); ip != "" {
		slog.Info("loaded CoreDNS service IP from node config", "coreDnsServiceIp", ip)
		return ip
	}
	if vars, path, err := src.nbcCmdVars(); err == nil {
		if ip, err := coreDNSServiceIPFromBootstrapCorefiles(vars); err == nil {
			slog.Info("loaded CoreDNS service IP from nbc-cmd.sh fallback", "coreDnsServiceIp", ip, "path", path)
			return ip
		}
	}
	slog.Warn("no bootstrap source carries the CoreDNS service IP", "bakedCorefile", bakedPath)
	return unresolvedCoreDNSServiceIP
}

// coreDNSServiceIPFromNodeConfig reads the CoreDNS service IP straight off the AKSNodeConfig JSON,
// returning "" when the file or field is absent. It deliberately does not go through the parser's
// getter, which would substitute the 10.0.0.10 default.
func coreDNSServiceIPFromNodeConfig(src Sources) string {
	raw, err := os.ReadFile(src.NodeConfigPath)
	if err != nil {
		return ""
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	return cfg.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp()
}

// nodeAgentPoolNameFromNBCCmd reads the agent pool name from the existing nbc-cmd.sh file.
// This is the fallback path when the full AKSNodeConfig JSON is not present (e.g. Staging
// Phase 2), mirroring lpsTargetFromNBCCmd. The pool name is carried in KUBELET_NODE_LABELS
// as the kubernetes.azure.com/agentpool label.
func nodeAgentPoolNameFromNBCCmd(src Sources) (string, error) {
	vars, path, err := src.nbcCmdVars()
	if err != nil {
		return "", err
	}
	labels := vars["KUBELET_NODE_LABELS"]
	if labels == "" {
		return "", fmt.Errorf("nbc-cmd %s has no KUBELET_NODE_LABELS", path)
	}
	agentPool := labelValueFromCSV(labels, agentPoolLabel)
	if agentPool == "" {
		return "", fmt.Errorf("nbc-cmd %s KUBELET_NODE_LABELS has no %s label", path, agentPoolLabel)
	}
	slog.Info("loaded agent pool from nbc-cmd.sh fallback", "agentPool", agentPool, "path", path)
	return agentPool, nil
}

// labelValueFromCSV extracts a label value from a comma-separated "key=value" list.
// KUBELET_NODE_LABELS may repeat a key; the values agree, so the first match wins.
func labelValueFromCSV(labels string, key string) string {
	for _, pair := range strings.Split(labels, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// coreDNSServiceIPFromBootstrapCorefiles recovers the CoreDNS service IP from a Corefile that
// bootstrap already rendered. In the generated Corefile the cluster.local server block forwards
// to the CoreDNS service, so the forward target there is the value we need. nbc-cmd.sh carries no
// standalone variable for it -- --cluster-dns points at the localdns cluster listener
// (169.254.10.11), not at the CoreDNS service.
func coreDNSServiceIPFromBootstrapCorefiles(vars map[string]string) (string, error) {
	for _, name := range []string{"LOCALDNS_COREFILE_BASE", "LOCALDNS_COREFILE_WITH_HOSTS", "LOCALDNS_GENERATED_COREFILE"} {
		encoded := strings.TrimSpace(vars[name])
		if encoded == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			slog.Info("bootstrap corefile variable is not valid base64, skipping", "variable", name, "error", err)
			continue
		}
		if ip := coreDNSServiceIPFromCorefile(string(decoded)); ip != "" {
			return ip, nil
		}
	}
	return "", fmt.Errorf("no bootstrap corefile carried a cluster.local forward target")
}

// coreDNSServiceIPFromCorefile returns the forward target of the cluster.local server block.
func coreDNSServiceIPFromCorefile(corefile string) string {
	inClusterLocalBlock := false
	for _, line := range strings.Split(corefile, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "{") && strings.HasPrefix(trimmed, "cluster.local:") {
			inClusterLocalBlock = true
			continue
		}
		if !inClusterLocalBlock {
			continue
		}
		if fields := strings.Fields(trimmed); len(fields) >= 3 && fields[0] == "forward" && fields[1] == "." {
			if ip := net.ParseIP(fields[2]); ip != nil {
				return fields[2]
			}
			return ""
		}
		if trimmed == "}" {
			inClusterLocalBlock = false
		}
	}
	return ""
}

// includeHostsPluginForCorefile decides whether the re-rendered Corefile keeps the hosts plugin
// block.
//
// Whether a node runs the hosts plugin is decided at bootstrap: the RP resolves a control-plane
// toggle into SHOULD_ENABLE_HOSTS_PLUGIN, and only then does CSE set up aks-localdns-hosts-setup
// and /etc/localdns/hosts. The live-patching payload cannot change that -- it carries
// enableHostsPlugin off HostsPluginConfig, which is not populated on the HCP proto, so it reads
// false, and turning the plugin on at runtime would need the hosts setup bootstrap never ran.
// Honouring the payload would silently strip the hosts block from a node that booted with it.
//
// So the baked Corefile decides: it is the variant localdns.sh selected for this node, which also
// captures bootstrap's own fallback to the no-hosts variant when the hosts file was missing. The
// nbc-cmd.sh value and then the payload are only consulted when no baked Corefile is readable.
func includeHostsPluginForCorefile(profile *aksnodeconfigv1.LocalDnsProfile, src Sources) bool {
	bakedPath := src.bakedCorefilePath()
	if baked, err := os.ReadFile(bakedPath); err == nil && len(bytes.TrimSpace(baked)) > 0 {
		return corefileHasHostsPlugin(string(baked), src.hostsFilePath())
	}
	vars, path, err := src.nbcCmdVars()
	if err != nil {
		slog.Info("could not read the baked corefile or nbc-cmd.sh for hosts plugin state, using LPS profile value",
			"bakedCorefile", bakedPath, "error", err)
		return profile.GetEnableHostsPlugin()
	}
	switch strings.TrimSpace(vars["SHOULD_ENABLE_HOSTS_PLUGIN"]) {
	case "true":
		return true
	case "false":
		return false
	}
	slog.Info("nbc-cmd.sh has no SHOULD_ENABLE_HOSTS_PLUGIN, using LPS profile value", "path", path)
	return profile.GetEnableHostsPlugin()
}

// corefileHasHostsPlugin reports whether a Corefile loads the hosts plugin from hostsPath.
func corefileHasHostsPlugin(corefile string, hostsPath string) bool {
	for _, line := range strings.Split(corefile, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "hosts" && fields[1] == hostsPath {
			return true
		}
	}
	return false
}
