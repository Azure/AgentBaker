package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	localDNSLivePatchingComponentName = "localDNS"
	defaultLocalDNSCorefilePath       = "/opt/azure/containers/localdns/localdns.corefile"
	localDNSHostsFilePath             = "/etc/localdns/hosts"
	localDNSAgentPoolLabel            = "kubernetes.azure.com/agentpool"
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

type localDNSConfigPayload struct {
	Corefile           string                             `json:"corefile"`
	CorefileBase64     string                             `json:"corefileBase64"`
	CorefileBase64Alt  string                             `json:"corefile_base64"`
	CoreFile           string                             `json:"coreFile"`
	LocalDNSProfile    json.RawMessage                    `json:"localDnsProfile"`
	LocalDNSProfileAlt json.RawMessage                    `json:"local_dns_profile"`
	AgentPools         map[string]localDNSAgentPoolConfig `json:"agentPools"`
	Profiles           map[string]localDNSAgentPoolConfig `json:"profiles"`
}

type localDNSAgentPoolConfig struct {
	CorefileVersion    string          `json:"corefileVersion"`
	ConfigChecksum     string          `json:"configChecksum"`
	Corefile           string          `json:"corefile"`
	CorefileBase64     string          `json:"corefileBase64"`
	CorefileBase64Alt  string          `json:"corefile_base64"`
	CoreFile           string          `json:"coreFile"`
	LocalDNSProfile    json.RawMessage `json:"localDnsProfile"`
	LocalDNSProfileAlt json.RawMessage `json:"local_dns_profile"`
}

type localDNSCorefileUpdate struct {
	corefile       string
	desiredVersion string
	hasCorefile    bool
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

	outcome, err := a.fetchAndApplyLocalDNSConfig(ctx, outputPath)
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
	return nil
}

func (a *App) fetchAndApplyLocalDNSConfig(ctx context.Context, outputPath string) (localDNSConfigOutcome, error) {
	if outputPath == "" {
		outputPath = defaultLocalDNSCorefilePath
	}
	config, err := a.fetchLocalDNSConfig(ctx)
	if err != nil {
		if isLPSUnavailable(err) {
			return outcomeLocalDNSConfigNotFound, nil
		}
		return outcomeLocalDNSConfigFailed, err
	}
	update, err := a.localDNSCorefileUpdateFromConfig(config)
	if err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	versionPath := localDNSCorefileVersionPath(outputPath)
	if update.desiredVersion != "" {
		currentVersion, err := readLocalDNSCorefileVersion(versionPath)
		if err != nil {
			return outcomeLocalDNSConfigFailed, err
		}
		if currentVersion == update.desiredVersion {
			return outcomeLocalDNSConfigAlreadyCurrent, nil
		}
	}
	if !update.hasCorefile {
		return outcomeLocalDNSConfigNoCorefileData, nil
	}
	if err := writeLocalDNSCorefile(outputPath, update.corefile); err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	if update.desiredVersion != "" {
		if err := writeLocalDNSCorefileVersion(versionPath, update.desiredVersion); err != nil {
			return outcomeLocalDNSConfigFailed, err
		}
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
	rootCAs, err := certPoolFromPEM(caPEM)
	if err != nil {
		return "", err
	}
	host := fqdn
	if h, _, splitErr := net.SplitHostPort(fqdn); splitErr == nil {
		host = h
	}
	dialAddr := net.JoinHostPort(host, lpsAPIServerPort)
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: lpsSNIHost, RootCAs: rootCAs}
	ctx, cancel := context.WithTimeout(ctx, lpsFetchTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx,
		net.JoinHostPort(lpsSNIHost, lpsAPIServerPort),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: lpsDialTimeout}
			return dialer.DialContext(ctx, "tcp", dialAddr)
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		return "", fmt.Errorf("dial LPS: %w", err)
	}
	defer conn.Close()

	client := akslivepatchingv1.NewLivePatchingServiceClient(conn)
	rpcCtx := metadata.AppendToOutgoingContext(ctx, "authorization", token)
	resp, err := client.GetComponentConfig(rpcCtx, &akslivepatchingv1.GetComponentConfigRequest{
		ComponentName: localDNSLivePatchingComponentName,
	})
	if err != nil {
		if code := status.Code(err); code == codes.NotFound || code == codes.PermissionDenied || code == codes.Unauthenticated {
			return "", &lpsUnavailableError{statusCode: int(code)}
		}
		return "", fmt.Errorf("get %s component config: %w", localDNSLivePatchingComponentName, err)
	}
	return resp.GetConfig(), nil
}

func certPoolFromPEM(caPEM []byte) (*x509.CertPool, error) {
	if len(caPEM) == 0 {
		return nil, fmt.Errorf("cluster CA unavailable from provision-config; refusing to fetch over unverified TLS")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse cluster CA PEM")
	}
	return pool, nil
}

func (a *App) localDNSCorefileUpdateFromConfig(config string) (localDNSCorefileUpdate, error) {
	config = strings.TrimSpace(config)
	if config == "" {
		return localDNSCorefileUpdate{}, nil
	}
	var payload localDNSConfigPayload
	if err := json.Unmarshal([]byte(config), &payload); err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("parsing localDNS LPS config: %w", err)
	}

	selected := localDNSAgentPoolConfig{
		Corefile:           payload.Corefile,
		CorefileBase64:     payload.CorefileBase64,
		CorefileBase64Alt:  payload.CorefileBase64Alt,
		CoreFile:           payload.CoreFile,
		LocalDNSProfile:    payload.LocalDNSProfile,
		LocalDNSProfileAlt: payload.LocalDNSProfileAlt,
	}
	if len(payload.AgentPools) > 0 || len(payload.Profiles) > 0 {
		agentPool, err := a.nodeAgentPoolName()
		if err != nil {
			return localDNSCorefileUpdate{}, err
		}
		var ok bool
		selected, ok = payload.AgentPools[agentPool]
		if !ok {
			selected, ok = payload.Profiles[agentPool]
		}
		if !ok {
			return localDNSCorefileUpdate{}, nil
		}
	}
	update := localDNSCorefileUpdate{
		desiredVersion: firstNonEmpty(selected.CorefileVersion, selected.ConfigChecksum),
	}

	switch {
	case strings.TrimSpace(selected.Corefile) != "":
		update.corefile = selected.Corefile
		update.hasCorefile = true
		return update, nil
	case strings.TrimSpace(selected.CoreFile) != "":
		update.corefile = selected.CoreFile
		update.hasCorefile = true
		return update, nil
	case strings.TrimSpace(selected.CorefileBase64) != "":
		return update.withCorefileBase64(selected.CorefileBase64)
	case strings.TrimSpace(selected.CorefileBase64Alt) != "":
		return update.withCorefileBase64(selected.CorefileBase64Alt)
	}
	profileJSON := selected.LocalDNSProfile
	if len(profileJSON) == 0 {
		profileJSON = selected.LocalDNSProfileAlt
	}
	if len(profileJSON) == 0 {
		if selected.CorefileVersion != "" || selected.ConfigChecksum != "" {
			slog.Info("localDNS LPS config has only version/checksum; Corefile content is required for bootstrap mutation",
				"corefileVersion", selected.CorefileVersion, "configChecksum", selected.ConfigChecksum)
		}
		return update, nil
	}
	profile := &aksnodeconfigv1.LocalDnsProfile{}
	unmarshalOptions := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOptions.Unmarshal(profileJSON, profile); err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("parsing localDNS profile: %w", err)
	}
	if !profile.GetEnableLocalDns() {
		return update, nil
	}
	nodeConfig, err := a.nodeConfigWithLocalDNSProfile(profile)
	if err != nil {
		return localDNSCorefileUpdate{}, err
	}
	includeHostsPlugin := profile.GetEnableHostsPlugin()
	if includeHostsPlugin {
		if _, err := os.Stat(localDNSHostsFilePath); err != nil {
			includeHostsPlugin = false
		}
	}
	corefile, err := parser.GenerateLocalDNSCorefileFromAKSNodeConfig(nodeConfig, includeHostsPlugin)
	if err != nil {
		return localDNSCorefileUpdate{}, err
	}
	update.corefile = corefile
	update.hasCorefile = true
	return update, nil
}

func (u localDNSCorefileUpdate) withCorefileBase64(v string) (localDNSCorefileUpdate, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
	if err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("decoding localDNS corefileBase64: %w", err)
	}
	if len(strings.TrimSpace(string(decoded))) == 0 {
		return u, nil
	}
	u.corefile = string(decoded)
	u.hasCorefile = true
	return u, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (a *App) nodeAgentPoolName() (string, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
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

func (a *App) nodeConfigWithLocalDNSProfile(profile *aksnodeconfigv1.LocalDnsProfile) (*aksnodeconfigv1.Configuration, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	if cfg == nil {
		return nil, fmt.Errorf("node config %s could not be parsed", path)
	}
	cfg.LocalDnsProfile = profile
	return cfg, nil
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
	return os.WriteFile(path, []byte(strings.TrimSpace(version)+"\n"), 0644)
}
