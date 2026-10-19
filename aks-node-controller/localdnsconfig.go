package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	akslivepatchingv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/parser"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	localDNSLivePatchingComponentName = "localDNS"
	defaultLocalDNSCorefilePath       = "/opt/azure/containers/localdns/livepatched.localdns.corefile"
	// defaultLocalDNSBakedCorefilePath is the Corefile localdns.sh regenerates from CustomData on
	// every start. It records what bootstrap decided for this node, so node-specific values the
	// live-patching payload does not carry are read back from it.
	defaultLocalDNSBakedCorefilePath = "/opt/azure/containers/localdns/localdns.corefile"
	defaultLocalDNSHostsFilePath     = "/etc/localdns/hosts"
	localDNSAgentPoolLabel           = "kubernetes.azure.com/agentpool"
	// unresolvedCoreDNSServiceIP stands in for the CoreDNS service IP when no bootstrap source on
	// the node carries it. The parser would otherwise substitute its 10.0.0.10 default, which is
	// wrong for any cluster with a custom service CIDR; rendering this marker instead lets a profile
	// that actually forwards to CoreDNS be refused rather than silently misrouted.
	unresolvedCoreDNSServiceIP = "unresolved-coredns-service-ip"
)

type localDNSConfigFetcher func(context.Context) (string, error)

type localDNSConfigOutcome string

const (
	outcomeLocalDNSConfigApplied        localDNSConfigOutcome = "applied"
	outcomeLocalDNSConfigAlreadyCurrent localDNSConfigOutcome = "alreadyCurrent"
	outcomeLocalDNSConfigNotFound       localDNSConfigOutcome = "notFound"
	outcomeLocalDNSConfigNoCorefileData localDNSConfigOutcome = "noCorefileData"
	outcomeLocalDNSConfigFailed         localDNSConfigOutcome = "failed"
)

// localDNSConfigPayload is the localDNS component's nodeConfig as LPC and LPS publish it: one entry
// per agent pool that has LocalDNS enabled. LPS serves "{}" when no pool has it enabled.
type localDNSConfigPayload struct {
	AgentPools map[string]localDNSAgentPoolConfig `json:"agentPools"`
}

// localDNSAgentPoolConfig is one agent pool's entry: the RP's hash of the pool's LocalDNS profile, and
// the profile this node renders its Corefile from.
type localDNSAgentPoolConfig struct {
	CorefileVersion string          `json:"corefileVersion"`
	LocalDNSProfile json.RawMessage `json:"localDnsProfile"`
}

type localDNSCorefileUpdate struct {
	corefile       string
	desiredVersion string
	hasCorefile    bool
}

// runApplyLocalDNSConfigCommand applies a payload the reconciler already has in hand. agentPool is
// the node's kubernetes.azure.com/agentpool label as the reconciler read it off the Node object; it
// selects this node's entry on every provisioning mode, including classic CSE, where neither file
// nodeAgentPoolName reads exists. Empty falls back to nodeAgentPoolName.
//
// The outcome is always the last line of output. On failure the reason is printed on the line before
// it as "error: <reason>": Run discards returned errors, and the localDNS handler reports this reason
// as the component's message in live-patching-status.
func (a *App) runApplyLocalDNSConfigCommand(ctx context.Context, configPath, outputPath, agentPool string, writer io.Writer) error {
	outcome := outcomeLocalDNSConfigFailed
	config, err := readLocalDNSConfigInput(configPath)
	if err == nil {
		outcome, err = a.applyLocalDNSConfig(ctx, config, outputPath, agentPool)
	}
	if writer != nil {
		if err != nil {
			_, _ = fmt.Fprintf(writer, "error: %s\n", err)
		}
		_, _ = fmt.Fprintf(writer, "%s\n", outcome)
	}
	return err
}

func readLocalDNSConfigInput(configPath string) (string, error) {
	if configPath == "" || configPath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading localDNS config from stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("reading localDNS config %s: %w", configPath, err)
	}
	return string(data), nil
}

func (a *App) applyLocalDNSConfig(ctx context.Context, config, outputPath, agentPool string) (localDNSConfigOutcome, error) {
	return a.fetchAndApplyLocalDNSConfig(ctx, outputPath, agentPool, func(context.Context) (string, error) {
		return config, nil
	})
}

func (a *App) runFetchLocalDNSConfigCommand(ctx context.Context, outputPath string) (err error) {
	slog.Info("aks-node-controller fetch-localdns-config started", "outputPath", outputPath)
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("fetch-localdns-config panicked (fail-open)", "panic", r)
			if a.eventLogger != nil {
				a.eventLogger.LogEvent("FetchLocalDNSConfig",
					fmt.Sprintf("fetch-localdns-config outcome=%s panic=%v", outcomeLocalDNSConfigFailed, r),
					helpers.EventLevelError, startTime, time.Now())
			}
			err = nil
		}
	}()

	// There is no Node object before the node joins, so the agent pool comes from the provisioning
	// files. Reaching LPS needs those same files, so a node without them never gets this far.
	outcome, err := a.fetchAndApplyLocalDNSConfig(ctx, outputPath, "", a.fetchLocalDNSConfig)
	level := helpers.EventLevelInformational
	if outcome == outcomeLocalDNSConfigFailed {
		level = helpers.EventLevelError
	}
	message := fmt.Sprintf("fetch-localdns-config outcome=%s", outcome)
	if err != nil {
		message = fmt.Sprintf("%s error=%s", message, err.Error())
		slog.Warn("fetch-localdns-config completed with error (fail-open)", "outcome", outcome, "error", err)
	} else {
		slog.Info("fetch-localdns-config completed", "outcome", outcome)
	}
	if a.eventLogger != nil {
		a.eventLogger.LogEvent("FetchLocalDNSConfig", message, level, startTime, time.Now())
	}
	// The bootstrap shell caller is intentionally fail-open, so it cannot use the
	// process exit status to distinguish an applied config from a fallback. Keep
	// the machine-readable outcome as the final stdout line.
	fmt.Fprintln(os.Stdout, outcome)
	return nil
}

func (a *App) fetchAndApplyLocalDNSConfig(ctx context.Context, outputPath, agentPool string, fetcher localDNSConfigFetcher) (localDNSConfigOutcome, error) {
	if outputPath == "" {
		outputPath = defaultLocalDNSCorefilePath
	}
	config, err := fetcher(ctx)
	if err != nil {
		if isLPSUnavailable(err) {
			return outcomeLocalDNSConfigNotFound, nil
		}
		return outcomeLocalDNSConfigFailed, err
	}
	update, err := a.localDNSCorefileUpdateFromConfig(config, agentPool)
	if err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	versionPath := localDNSCorefileVersionPath(outputPath)
	if !update.hasCorefile {
		return outcomeLocalDNSConfigNoCorefileData, nil
	}
	current, readErr := os.ReadFile(outputPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return outcomeLocalDNSConfigFailed, fmt.Errorf("reading localDNS corefile %s: %w", outputPath, readErr)
	}
	contentMatches := readErr == nil && bytes.Equal(current, []byte(update.corefile))
	currentVersion, err := readLocalDNSCorefileVersion(versionPath)
	if err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	if currentVersion == update.desiredVersion && contentMatches {
		return outcomeLocalDNSConfigAlreadyCurrent, nil
	}
	if err := writeLocalDNSCorefile(outputPath, update.corefile); err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	if err := writeLocalDNSCorefileVersion(versionPath, update.desiredVersion); err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	return outcomeLocalDNSConfigApplied, nil
}

func (a *App) fetchLocalDNSConfig(ctx context.Context) (string, error) {
	if a.fetchLocalDNSConfigFn != nil {
		return a.fetchLocalDNSConfigFn(ctx)
	}
	return a.fetchLocalDNSConfigFromLPS(ctx)
}

func (a *App) fetchLocalDNSConfigFromLPS(ctx context.Context) (string, error) {
	fqdn, caPEM, err := a.lpsTargetFromNodeConfig()
	if err != nil {
		return "", fmt.Errorf("resolving LPS endpoint from node config: %w", err)
	}
	token, err := a.attestedToken(ctx)
	if err != nil {
		return "", fmt.Errorf("imds attested token: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, lpsFetchTimeout)
	defer cancel()

	// Share the check-hotfix dialer rather than building a second TLS config: it targets the same
	// apiserver FQDN with the same ALPN protocols and the same cluster CA, so LPS presents a
	// certificate issued for the dialed hostname and grpc-go's standard verification covers both
	// the chain and the hostname. Reusing it also picks up grpc.WithNoProxy(), which this path was
	// missing, and keeps the two LPS clients from drifting.
	conn, err := a.dialLPSGRPC(fqdn, caPEM)
	if err != nil {
		return "", fmt.Errorf("creating LPS client: %w", err)
	}
	defer conn.Close()

	client := akslivepatchingv1.NewLivePatchingServiceClient(conn)
	rpcCtx := metadata.AppendToOutgoingContext(ctx, "authorization", token)
	resp, err := client.GetComponentConfig(rpcCtx, &akslivepatchingv1.GetComponentConfigRequest{
		ComponentName: localDNSLivePatchingComponentName,
	})
	if err != nil {
		// mapGRPCError is the shared LPS classifier: it folds NotFound/PermissionDenied/
		// Unauthenticated into errLPSUnavailable -- which the caller tests via isLPSUnavailable
		// -- while keeping the gRPC code and message, and passes non-status errors through
		// untouched. %w keeps that chain intact through this wrap.
		return "", fmt.Errorf("get %s component config: %w",
			localDNSLivePatchingComponentName, mapGRPCError(err))
	}
	return resp.GetConfig(), nil
}

func (a *App) localDNSCorefileUpdateFromConfig(config, agentPool string) (localDNSCorefileUpdate, error) {
	config = strings.TrimSpace(config)
	if config == "" {
		return localDNSCorefileUpdate{}, nil
	}

	var payload localDNSConfigPayload
	if err := json.Unmarshal([]byte(config), &payload); err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("parsing localDNS LPS config: %w", err)
	}

	selected, found, err := a.selectLocalDNSAgentPoolConfig(payload, agentPool)
	if err != nil || !found {
		return localDNSCorefileUpdate{}, err
	}
	// The RP stamps a corefileVersion on every pool it publishes; the node records it with the patch
	// and compares against it, so an entry without one cannot be applied consistently.
	desiredVersion := strings.TrimSpace(selected.CorefileVersion)
	if desiredVersion == "" {
		return localDNSCorefileUpdate{}, fmt.Errorf("localDNS config for this node's agent pool has no corefileVersion")
	}

	update := localDNSCorefileUpdate{desiredVersion: desiredVersion}
	return a.localDNSCorefileUpdateFromAgentPoolConfig(selected, update)
}

// selectLocalDNSAgentPoolConfig returns this node's entry. A payload without entries, or without one
// for this pool, is not found: no LocalDNS config applies to this node.
func (a *App) selectLocalDNSAgentPoolConfig(payload localDNSConfigPayload, agentPool string) (localDNSAgentPoolConfig, bool, error) {
	if len(payload.AgentPools) == 0 {
		return localDNSAgentPoolConfig{}, false, nil
	}
	if agentPool == "" {
		var err error
		if agentPool, err = a.nodeAgentPoolName(); err != nil {
			return localDNSAgentPoolConfig{}, false, err
		}
	}
	selected, ok := payload.AgentPools[agentPool]
	return selected, ok, nil
}

func (a *App) localDNSCorefileUpdateFromAgentPoolConfig(selected localDNSAgentPoolConfig, update localDNSCorefileUpdate) (localDNSCorefileUpdate, error) {
	if len(selected.LocalDNSProfile) == 0 {
		slog.Info("localDNS config has a corefileVersion but no localDnsProfile to render",
			"corefileVersion", selected.CorefileVersion)
		return update, nil
	}
	profile := &aksnodeconfigv1.LocalDnsProfile{}
	unmarshalOptions := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOptions.Unmarshal(selected.LocalDNSProfile, profile); err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("parsing localDNS profile: %w", err)
	}
	if !profile.GetEnableLocalDns() {
		return update, nil
	}
	// The Corefile template reads exactly one field off Configuration outside of LocalDnsProfile:
	// ClusterConfig.ClusterNetworkConfig.CoreDnsServiceIp. Everything else comes from the profile.
	nodeConfig := &aksnodeconfigv1.Configuration{
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
				CoreDnsServiceIp: a.localDNSCoreDNSServiceIP(),
			},
		},
		LocalDnsProfile: profile,
	}
	includeHostsPlugin := a.includeHostsPluginForCorefile(profile)
	if includeHostsPlugin {
		if _, statErr := os.Stat(a.getLocalDNSHostsFilePath()); statErr != nil {
			includeHostsPlugin = false
		}
	}
	corefile, err := parser.GenerateLocalDnsCorefileFromAKSNodeConfig(nodeConfig, includeHostsPlugin)
	if err != nil {
		return localDNSCorefileUpdate{}, err
	}
	if strings.Contains(corefile, unresolvedCoreDNSServiceIP) {
		return localDNSCorefileUpdate{}, fmt.Errorf("localDNS profile forwards to the cluster CoreDNS service, " +
			"but no bootstrap source on this node carries its IP")
	}
	update.corefile = corefile
	update.hasCorefile = true
	return update, nil
}

func (a *App) nodeAgentPoolName() (string, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("node config not found, trying nbc-cmd.sh fallback", "path", path)
			return a.nodeAgentPoolNameFromNBCCmd()
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
	agentPool := cfg.GetKubeletConfig().GetKubeletNodeLabels()[localDNSAgentPoolLabel]
	if agentPool == "" {
		return "", fmt.Errorf("node config has no %s kubelet node label", localDNSAgentPoolLabel)
	}
	return agentPool, nil
}

// localDNSCoreDNSServiceIP returns the CoreDNS service IP for the cluster.local blocks, taken from
// what bootstrap already rendered for this node rather than recomputed.
//
// The baked Corefile comes first because it exists in every provisioning mode and is exactly what
// this node has been serving. The AKSNodeConfig JSON and nbc-cmd.sh only exist for scriptless
// provisioning, so they are fallbacks. When none of them carries the IP the marker is returned and
// the caller refuses the render if the profile needs it.
func (a *App) localDNSCoreDNSServiceIP() string {
	bakedPath := a.getLocalDNSBakedCorefilePath()
	if baked, err := os.ReadFile(bakedPath); err == nil {
		if ip := coreDNSServiceIPFromCorefile(string(baked)); ip != "" {
			slog.Info("loaded CoreDNS service IP from the baked localDNS corefile", "coreDnsServiceIp", ip, "path", bakedPath)
			return ip
		}
	}
	if ip := a.coreDNSServiceIPFromNodeConfig(); ip != "" {
		slog.Info("loaded CoreDNS service IP from node config", "coreDnsServiceIp", ip)
		return ip
	}
	if vars, path, err := a.nbcCmdEnvVars(); err == nil {
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
func (a *App) coreDNSServiceIPFromNodeConfig() string {
	raw, err := os.ReadFile(a.getNodeConfigPath())
	if err != nil {
		return ""
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	return cfg.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp()
}

func (a *App) getLocalDNSBakedCorefilePath() string {
	if a.localDNSBakedCorefilePath != "" {
		return a.localDNSBakedCorefilePath
	}
	return defaultLocalDNSBakedCorefilePath
}

func (a *App) getLocalDNSHostsFilePath() string {
	if a.localDNSHostsFilePath != "" {
		return a.localDNSHostsFilePath
	}
	return defaultLocalDNSHostsFilePath
}

func writeLocalDNSCorefile(path string, corefile string) error {
	if strings.TrimSpace(corefile) == "" {
		return fmt.Errorf("localDNS corefile is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".localdns-corefile-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.WriteString(tmp, corefile); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func localDNSCorefileVersionPath(corefilePath string) string {
	return corefilePath + ".version"
}

func readLocalDNSCorefileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading localDNS corefile version %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func writeLocalDNSCorefileVersion(path string, version string) error {
	if strings.TrimSpace(version) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(version)+"\n"), 0600)
}

// nodeAgentPoolNameFromNBCCmd reads the agent pool name from the existing nbc-cmd.sh file.
// This is the fallback path when the full AKSNodeConfig JSON is not present (e.g. Staging
// Phase 2), mirroring lpsTargetFromNBCCmd. The pool name is carried in KUBELET_NODE_LABELS
// as the kubernetes.azure.com/agentpool label.
func (a *App) nodeAgentPoolNameFromNBCCmd() (string, error) {
	vars, path, err := a.nbcCmdEnvVars()
	if err != nil {
		return "", err
	}
	labels := vars["KUBELET_NODE_LABELS"]
	if labels == "" {
		return "", fmt.Errorf("nbc-cmd %s has no KUBELET_NODE_LABELS", path)
	}
	agentPool := labelValueFromCSV(labels, localDNSAgentPoolLabel)
	if agentPool == "" {
		return "", fmt.Errorf("nbc-cmd %s KUBELET_NODE_LABELS has no %s label", path, localDNSAgentPoolLabel)
	}
	slog.Info("loaded agent pool from nbc-cmd.sh fallback", "agentPool", agentPool, "path", path)
	return agentPool, nil
}

// nbcCmdEnvVars reads and parses the command-scoped environment variables out of nbc-cmd.sh.
func (a *App) nbcCmdEnvVars() (map[string]string, string, error) {
	path := a.getNBCCmdPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("reading nbc-cmd %s: %w", path, err)
	}
	return parseEnvVarsFromNBCCmdContent(string(raw)), path, nil
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
func (a *App) includeHostsPluginForCorefile(profile *aksnodeconfigv1.LocalDnsProfile) bool {
	bakedPath := a.getLocalDNSBakedCorefilePath()
	if baked, err := os.ReadFile(bakedPath); err == nil && len(bytes.TrimSpace(baked)) > 0 {
		return corefileHasHostsPlugin(string(baked), a.getLocalDNSHostsFilePath())
	}
	vars, path, err := a.nbcCmdEnvVars()
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
