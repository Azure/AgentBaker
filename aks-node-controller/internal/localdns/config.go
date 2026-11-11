// Package localdns renders a node's LocalDNS Corefile from its live-patching payload: the localDNS
// component's nodeConfig, which LPS serves at bootstrap and LPC hands the reconciler at runtime. It
// picks this node's agent pool entry, checks the profile, renders the Corefile with the same
// template CSE uses, and writes it next to its .version file. aks-node-controller's
// `localdns fetch-config` and `localdns apply-config` commands are thin wrappers around Apply.
package localdns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// ComponentName is the localDNS component's name in the live-patching payloads.
	ComponentName = "localDNS"
	// DefaultCorefilePath is where the live-patched Corefile is written when no output path is given.
	DefaultCorefilePath = "/opt/azure/containers/localdns/livepatched.localdns.corefile"
	// DefaultBakedCorefilePath is the Corefile localdns.sh regenerates from CustomData on every
	// start. It records what bootstrap decided for this node, so node-specific values the
	// live-patching payload does not carry are read back from it.
	DefaultBakedCorefilePath = "/opt/azure/containers/localdns/localdns.corefile"
	// DefaultHostsFilePath is the hosts file the hosts plugin serves.
	DefaultHostsFilePath = "/etc/localdns/hosts"
)

// Outcome is the machine-readable result of a fetch or apply, printed as the last line of output.
type Outcome string

const (
	OutcomeRendered       Outcome = "rendered"
	OutcomeAlreadyCurrent Outcome = "alreadyCurrent"
	OutcomeNotFound       Outcome = "notFound"
	OutcomeNoCorefileData Outcome = "noCorefileData"
	OutcomeFailed         Outcome = "failed"
)

// Fetcher returns the localDNS payload: from LPS at bootstrap, or the payload the reconciler
// dispatched at runtime.
type Fetcher func(context.Context) (string, error)

// ErrNotPublished, returned (or wrapped) by a Fetcher, means LPS has no localDNS config for this
// node: the notFound outcome rather than a failure.
var ErrNotPublished = errors.New("localDNS config is not published")

// Sources says where this node's bootstrap values are read from: the agent pool, the CoreDNS service
// IP and whether the hosts plugin is on. Empty paths fall back to the defaults.
type Sources struct {
	// NodeConfigPath is the AKSNodeConfig JSON (scriptless provisioning only).
	NodeConfigPath string
	// BakedCorefilePath is the Corefile localdns.sh rendered from CustomData.
	BakedCorefilePath string
	// HostsFilePath is the hosts plugin's hosts file.
	HostsFilePath string
	// NBCCmdVars returns the variables nbc-cmd.sh sets and the file's path. It is the fallback when
	// the AKSNodeConfig JSON is absent; aks-node-controller's nbc-cmd parser supplies it.
	NBCCmdVars func() (map[string]string, string, error)
}

func (s Sources) bakedCorefilePath() string {
	if s.BakedCorefilePath != "" {
		return s.BakedCorefilePath
	}
	return DefaultBakedCorefilePath
}

func (s Sources) hostsFilePath() string {
	if s.HostsFilePath != "" {
		return s.HostsFilePath
	}
	return DefaultHostsFilePath
}

func (s Sources) nbcCmdVars() (map[string]string, string, error) {
	if s.NBCCmdVars == nil {
		return nil, "", fmt.Errorf("no nbc-cmd source")
	}
	return s.NBCCmdVars()
}

// payload is the localDNS component's nodeConfig as LPC and LPS publish it: one entry per agent pool
// that has LocalDNS enabled. LPS serves "{}" when no pool has it enabled.
type payload struct {
	AgentPools map[string]agentPoolConfig `json:"agentPools"`
}

// agentPoolConfig is one agent pool's entry: the RP's hash of the pool's LocalDNS profile, and the
// profile this node renders its Corefile from.
type agentPoolConfig struct {
	CorefileVersion string          `json:"corefileVersion"`
	LocalDNSProfile json.RawMessage `json:"localDnsProfile"`
}

type corefileUpdate struct {
	corefile       string
	desiredVersion string
	hasCorefile    bool
	// cpuQuota and memoryMax are the profile's limits in the units localdns.slice takes (CPUQuota=,
	// MemoryMax=). Empty when the profile leaves a limit unset: the caller then uses the baked value.
	cpuQuota  string
	memoryMax string
}

// Result is the outcome of a fetch or apply, plus what the caller needs to finish the patch when a
// Corefile was rendered or is already current.
type Result struct {
	Outcome Outcome
	update  corefileUpdate
}

// Version is the corefileVersion of the entry that was rendered or is already current.
func (r Result) Version() string {
	return r.update.desiredVersion
}

// Lines returns the output lines for this result. The outcome is always last, since both shell
// callers read it with tail -n 1. For rendered and alreadyCurrent the lines before it carry the
// version to commit and the limits to apply; aks-node-controller no longer writes the .version file
// itself, because the version stands for the Corefile and the limits, and only the caller applies
// the limits.
func (r Result) Lines() []string {
	var lines []string
	if r.Outcome == OutcomeRendered || r.Outcome == OutcomeAlreadyCurrent {
		lines = append(lines, "version: "+r.update.desiredVersion)
		if r.update.cpuQuota != "" {
			lines = append(lines, "cpuQuota: "+r.update.cpuQuota)
		}
		if r.update.memoryMax != "" {
			lines = append(lines, "memoryMax: "+r.update.memoryMax)
		}
	}
	return append(lines, string(r.Outcome))
}

// WriteResult prints a result the way the localDNS handler parses it: the outcome is always the last
// line, and on failure the reason is printed on the line before it as "error: <reason>", which the
// handler reports as the component's message in live-patching-status.
func WriteResult(w io.Writer, result Result, err error) {
	if err != nil {
		_, _ = fmt.Fprintf(w, "error: %s\n", err)
	}
	for _, line := range result.Lines() {
		_, _ = fmt.Fprintln(w, line)
	}
}

// ReadConfig reads a payload from configPath, or from stdin when configPath is empty or "-".
func ReadConfig(configPath string) (string, error) {
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

// Apply renders this node's Corefile from the fetched payload and writes it to outputPath. agentPool
// selects this node's entry; empty reads it from the provisioning files. It never writes the
// .version file next to the Corefile: the caller writes that once it has also applied the limits and
// CoreDNS serves the Corefile. Before a changed Corefile is written the old .version is removed, so a
// node is never left claiming a version whose Corefile it does not have.
func Apply(ctx context.Context, fetch Fetcher, outputPath, agentPool string, src Sources) (Result, error) {
	failed := Result{Outcome: OutcomeFailed}
	if outputPath == "" {
		outputPath = DefaultCorefilePath
	}
	config, err := fetch(ctx)
	if err != nil {
		if errors.Is(err, ErrNotPublished) {
			return Result{Outcome: OutcomeNotFound}, nil
		}
		return failed, err
	}
	update, err := corefileUpdateFromConfig(config, agentPool, src)
	if err != nil {
		return failed, err
	}
	versionPath := corefileVersionPath(outputPath)
	if !update.hasCorefile {
		return Result{Outcome: OutcomeNoCorefileData}, nil
	}
	current, readErr := os.ReadFile(outputPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return failed, fmt.Errorf("reading localDNS corefile %s: %w", outputPath, readErr)
	}
	contentMatches := readErr == nil && bytes.Equal(current, []byte(update.corefile))
	currentVersion, err := readCorefileVersion(versionPath)
	if err != nil {
		return failed, err
	}
	if currentVersion == update.desiredVersion && contentMatches {
		return Result{Outcome: OutcomeAlreadyCurrent, update: update}, nil
	}
	if err := os.Remove(versionPath); err != nil && !os.IsNotExist(err) {
		return failed, fmt.Errorf("removing localDNS corefile version %s: %w", versionPath, err)
	}
	if err := writeCorefile(outputPath, update.corefile); err != nil {
		return failed, err
	}
	return Result{Outcome: OutcomeRendered, update: update}, nil
}

func corefileUpdateFromConfig(config, agentPool string, src Sources) (corefileUpdate, error) {
	config = strings.TrimSpace(config)
	if config == "" {
		return corefileUpdate{}, nil
	}

	var p payload
	if err := json.Unmarshal([]byte(config), &p); err != nil {
		return corefileUpdate{}, fmt.Errorf("parsing localDNS LPS config: %w", err)
	}

	selected, found, err := selectAgentPoolConfig(p, agentPool, src)
	if err != nil || !found {
		return corefileUpdate{}, err
	}
	// The RP stamps a corefileVersion on every pool it publishes; the node records it with the patch
	// and compares against it, so an entry without one cannot be applied consistently.
	desiredVersion := strings.TrimSpace(selected.CorefileVersion)
	if desiredVersion == "" {
		return corefileUpdate{}, fmt.Errorf("localDNS config for this node's agent pool has no corefileVersion")
	}

	update := corefileUpdate{desiredVersion: desiredVersion}
	return corefileUpdateFromAgentPoolConfig(selected, update, src)
}

// selectAgentPoolConfig returns this node's entry. A payload without entries, or without one for
// this pool, is not found: no LocalDNS config applies to this node.
func selectAgentPoolConfig(p payload, agentPool string, src Sources) (agentPoolConfig, bool, error) {
	if len(p.AgentPools) == 0 {
		return agentPoolConfig{}, false, nil
	}
	if agentPool == "" {
		var err error
		if agentPool, err = nodeAgentPoolName(src); err != nil {
			return agentPoolConfig{}, false, err
		}
	}
	selected, ok := p.AgentPools[agentPool]
	return selected, ok, nil
}

func corefileUpdateFromAgentPoolConfig(selected agentPoolConfig, update corefileUpdate, src Sources) (corefileUpdate, error) {
	if len(selected.LocalDNSProfile) == 0 {
		slog.Info("localDNS config has a corefileVersion but no localDnsProfile to render",
			"corefileVersion", selected.CorefileVersion)
		return update, nil
	}
	profile := &aksnodeconfigv1.LocalDnsProfile{}
	unmarshalOptions := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOptions.Unmarshal(selected.LocalDNSProfile, profile); err != nil {
		return corefileUpdate{}, fmt.Errorf("parsing localDNS profile: %w", err)
	}
	if !profile.GetEnableLocalDns() {
		return update, nil
	}
	if err := checkProfileComplete(profile); err != nil {
		return corefileUpdate{}, err
	}
	// The Corefile template reads exactly one field off Configuration outside of LocalDnsProfile:
	// ClusterConfig.ClusterNetworkConfig.CoreDnsServiceIp. Everything else comes from the profile.
	nodeConfig := &aksnodeconfigv1.Configuration{
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
				CoreDnsServiceIp: coreDNSServiceIP(src),
			},
		},
		LocalDnsProfile: profile,
	}
	includeHostsPlugin := includeHostsPluginForCorefile(profile, src)
	if includeHostsPlugin {
		if _, statErr := os.Stat(src.hostsFilePath()); statErr != nil {
			includeHostsPlugin = false
		}
	}
	// The same functions CSE uses to write localdns.slice, so both paths format limits identically.
	// Only for a set limit: for an unset one they return the provisioning defaults (200%, 128M),
	// but an unset limit must fall back to the baked localdns.slice value instead.
	if profile.GetCpuLimitInMilliCores() != 0 {
		update.cpuQuota = parser.GetLocalDnsCpuLimitInPercentage(nodeConfig)
	}
	if profile.GetMemoryLimitInMb() != 0 {
		update.memoryMax = parser.GetLocalDnsMemoryLimitInMb(nodeConfig)
	}
	corefile, err := parser.GenerateLocalDnsCorefileFromAKSNodeConfig(nodeConfig, includeHostsPlugin)
	if err != nil {
		return corefileUpdate{}, err
	}
	if strings.Contains(corefile, unresolvedCoreDNSServiceIP) {
		return corefileUpdate{}, fmt.Errorf("localDNS profile forwards to the cluster CoreDNS service, " +
			"but no bootstrap source on this node carries its IP")
	}
	update.corefile = corefile
	update.hasCorefile = true
	return update, nil
}

// checkProfileComplete refuses a profile that lacks a field the RP always sets (ValidateLocalDNSProfile
// in aks-rp requires all of them on every zone, custom zones included). The payload is parsed with
// DiscardUnknown so that a field the RP adds later does not break older nodes, which also means a
// field renamed on either side arrives unset instead of failing the parse. An unset field can render
// a Corefile CoreDNS accepts but serves differently: "cache <nil>" turns caching off, and no
// forwardDestination sends ClusterFirst traffic to VNet DNS. Zero is a valid value; only an absent
// field is refused. This ties the node to the fields the RP requires today: if the RP stops sending
// one, this node keeps its current config and reports which field is missing.
func checkProfileComplete(profile *aksnodeconfigv1.LocalDnsProfile) error {
	for _, m := range []struct {
		name      string
		overrides map[string]*aksnodeconfigv1.LocalDnsOverrides
	}{
		{name: "vnetDnsOverrides", overrides: profile.GetVnetDnsOverrides()},
		{name: "kubeDnsOverrides", overrides: profile.GetKubeDnsOverrides()},
	} {
		if len(m.overrides) == 0 {
			return fmt.Errorf("localDNS profile has no %s", m.name)
		}
		for _, zone := range slices.Sorted(maps.Keys(m.overrides)) {
			override := m.overrides[zone]
			if override == nil {
				return fmt.Errorf("localDNS profile %s zone %q is empty", m.name, zone)
			}
			if missing := missingOverrideFields(override); len(missing) > 0 {
				return fmt.Errorf("localDNS profile %s zone %q has no %s", m.name, zone, strings.Join(missing, ", "))
			}
		}
	}
	return nil
}

// missingOverrideFields names the fields the RP always sets that this override lacks.
func missingOverrideFields(override *aksnodeconfigv1.LocalDnsOverrides) []string {
	var missing []string
	for _, field := range []struct {
		name string
		set  bool
	}{
		{name: "queryLogging", set: override.GetQueryLogging() != ""},
		{name: "protocol", set: override.GetProtocol() != ""},
		{name: "forwardDestination", set: override.GetForwardDestination() != ""},
		{name: "forwardPolicy", set: override.GetForwardPolicy() != ""},
		{name: "maxConcurrent", set: override.MaxConcurrent != nil},
		{name: "cacheDurationInSeconds", set: override.CacheDurationInSeconds != nil},
		{name: "serveStaleDurationInSeconds", set: override.ServeStaleDurationInSeconds != nil},
		{name: "serveStale", set: override.GetServeStale() != ""},
	} {
		if !field.set {
			missing = append(missing, field.name)
		}
	}
	return missing
}

func writeCorefile(path string, corefile string) error {
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

func corefileVersionPath(corefilePath string) string {
	return corefilePath + ".version"
}

func readCorefileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading localDNS corefile version %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
