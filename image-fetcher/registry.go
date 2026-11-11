package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/core/remotes/docker/config"
	"github.com/containerd/errdefs"
	"github.com/pelletier/go-toml/v2"
)

type registryAuth struct {
	Username      string `toml:"username"`
	Password      string `toml:"password"`
	Auth          string `toml:"auth"`
	IdentityToken string `toml:"identitytoken"`
}

type registryHostConfig struct {
	Auth registryAuth   `toml:"auth"`
	TLS  map[string]any `toml:"tls"`
}

type registryPluginConfig struct {
	Registry struct {
		ConfigPath string                        `toml:"config_path"`
		Configs    map[string]registryHostConfig `toml:"configs"`
		Mirrors    map[string]any                `toml:"mirrors"`
		Headers    http.Header                   `toml:"headers"`
	} `toml:"registry"`
}

func registryResolverFromConfig(ctx context.Context, data []byte) (remotes.Resolver, error) {
	var effective struct {
		Plugins map[string]registryPluginConfig `toml:"plugins"`
	}
	if err := toml.Unmarshal(data, &effective); err != nil {
		return nil, errors.New("invalid effective registry configuration")
	}
	plugin, ok := effective.Plugins["io.containerd.cri.v1.images"]
	if !ok {
		plugin, ok = effective.Plugins["io.containerd.grpc.v1.cri"]
	}
	if !ok {
		return nil, errors.New("effective CRI registry configuration is missing")
	}
	registry := plugin.Registry
	if len(registry.Mirrors) != 0 {
		return nil, errors.New("legacy CRI registry mirrors require migration to hosts.toml before prefetch")
	}
	for _, host := range registry.Configs {
		if len(host.TLS) != 0 {
			return nil, errors.New("legacy CRI registry TLS requires migration to hosts.toml before prefetch")
		}
	}
	hostOptions := config.HostOptions{
		Credentials: func(host string) (string, string, error) {
			// A mirror must use its own credentials, never the source registry's.
			return registry.Configs[host].Auth.credentials()
		},
		AuthorizerOpts: []docker.AuthorizerOpt{docker.WithAuthHeader(registry.Headers)},
	}
	if paths := filepath.SplitList(registry.ConfigPath); len(paths) != 0 {
		hostOptions.HostDir = validatedRegistryHostDir(paths)
	}
	return docker.NewResolver(docker.ResolverOptions{
		Headers: registry.Headers,
		Hosts:   config.ConfigureHosts(ctx, hostOptions),
	}), nil
}

func (auth registryAuth) credentials() (string, string, error) {
	switch {
	case auth.Username != "":
		return auth.Username, auth.Password, nil
	case auth.IdentityToken != "":
		return "", auth.IdentityToken, nil
	case auth.Auth == "":
		return "", "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
	if err != nil {
		return "", "", errors.New("invalid registry authentication encoding")
	}
	user, password, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return "", "", errors.New("invalid registry authentication format")
	}
	return user, strings.TrimRight(password, "\x00"), nil
}

func validatedRegistryHostDir(paths []string) func(string) (string, error) {
	return func(host string) (string, error) {
		for _, root := range paths {
			dir, err := config.HostDirFromRoot(root)(host)
			if err != nil && !errdefs.IsNotFound(err) {
				return "", err
			}
			if dir == "" {
				continue
			}
			data, readErr := os.ReadFile(filepath.Join(dir, "hosts.toml"))
			if readErr != nil && !os.IsNotExist(readErr) {
				return "", readErr
			}
			if err = validateRegistryHosts(data); err != nil {
				return "", err
			}
			return dir, nil
		}
		return "", errdefs.ErrNotFound
	}
}

// ConfigureHosts in the pinned containerd version silently falls back after
// hosts.toml decoding errors. Validate its accepted schema before letting the
// library apply all endpoint ordering, mirror, path and TLS policy.
type registryHostSettings struct {
	Capabilities []string       `toml:"capabilities"`
	CA           any            `toml:"ca"`
	Client       any            `toml:"client"`
	SkipVerify   *bool          `toml:"skip_verify"`
	Header       map[string]any `toml:"header"`
	OverridePath bool           `toml:"override_path"`
	DialTimeout  string         `toml:"dial_timeout"`
}

func validateRegistryHosts(data []byte) error {
	var document struct {
		registryHostSettings
		Server string                          `toml:"server"`
		Host   map[string]registryHostSettings `toml:"host"`
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return errors.New("invalid registry hosts.toml")
	}
	if err := document.validate(document.Server); err != nil {
		return err
	}
	for server, settings := range document.Host {
		if err := settings.validate(server); err != nil {
			return err
		}
	}
	return nil
}

func (settings registryHostSettings) validate(server string) error {
	if server != "" {
		if !strings.HasPrefix(server, "http") {
			server = "https://" + server
		}
		if _, err := url.Parse(server); err != nil {
			return errors.New("invalid registry hosts.toml endpoint")
		}
	}
	for _, capability := range settings.Capabilities {
		switch strings.ToLower(capability) {
		case "pull", "resolve", "push", "referrers":
		default:
			return errors.New("invalid registry hosts.toml capability")
		}
	}
	if !validRegistryStringValues(settings.CA, false) || !validRegistryStringValues(settings.Client, true) {
		return errors.New("invalid registry hosts.toml certificate settings")
	}
	for _, value := range settings.Header {
		if !validRegistryStringValues(value, false) {
			return errors.New("invalid registry hosts.toml header")
		}
	}
	if settings.DialTimeout != "" {
		if _, err := time.ParseDuration(settings.DialTimeout); err != nil {
			return errors.New("invalid registry hosts.toml dial timeout")
		}
	}
	return nil
}

func validRegistryStringValues(value any, pairs bool) bool {
	switch values := value.(type) {
	case nil, string:
		return true
	case []any:
		for _, item := range values {
			if _, ok := item.(string); ok {
				continue
			}
			pair, ok := item.([]any)
			if !pairs || !ok || len(pair) != 2 || !validRegistryStringValues(pair, false) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
