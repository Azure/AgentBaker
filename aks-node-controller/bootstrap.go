package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // Azure GoalState identifies certificates with SHA-1 thumbprints.
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"golang.org/x/sync/errgroup"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

const (
	imdsUserDataURL = "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text"
	imdsComputeURL  = "http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01"
	goalStateURL    = "http://168.63.129.16/machine/?comp=goalstate"
	vmSettingsURL   = "http://168.63.129.16:32526/vmSettings"

	wireProtocolVersion     = "2012-11-30"
	hostPluginAPIVersion    = "2015-09-01"
	customScriptHandlerName = "Microsoft.Azure.Extensions.CustomScript"
	expectedCertFormat      = "Pkcs7BlobWithPfxContents"

	defaultBootstrapTimeout = 180 * time.Second
	userDataPath            = "/opt/azure/containers/userdata.sh"
	boothookPath            = "/opt/azure/containers/boothook.sh"
	waagentProvisionedPath  = "/var/lib/waagent/provisioned"
	productUUIDPath         = "/sys/class/dmi/id/product_uuid"
)

var (
	userDataPrefix = []byte("#!/bin/bash\nset -euo pipefail\n")
	hostnameRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)
	usernameRE     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,31}$`)
	boothookRE     = regexp.MustCompile(`^echo '([A-Za-z0-9+/=\s]+)' \| base64 -d \| gzip -d > /opt/azure/containers/boothook\.sh`)
)

type bootstrapConfig struct {
	timeout         time.Duration
	userDataPath    string
	boothookPath    string
	hostnamePath    string
	hostsPath       string
	productUUIDPath string
	provisionedPath string
	httpClient      *http.Client
	runCommand      func(context.Context, string, ...string) error
}

type goalState struct {
	containerID         string
	roleConfigName      string
	extensionsConfigURL string
	certificatesURL     string
}

type protectedSettings struct {
	protected  string
	thumbprint string
}

type certificateMaterial struct {
	keys  []crypto.PrivateKey
	certs []*x509.Certificate
}

type certificateIdentity struct {
	key  crypto.PrivateKey
	cert *x509.Certificate
}

type imdsProfile struct {
	hostname string
	username string
	sshKeys  []string
}

type protectedSettingsDocument struct {
	CommandToExecute string `json:"commandToExecute"`
}

func defaultBootstrapConfig() bootstrapConfig {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return bootstrapConfig{
		timeout:         defaultBootstrapTimeout,
		userDataPath:    userDataPath,
		boothookPath:    boothookPath,
		hostnamePath:    "/etc/hostname",
		hostsPath:       "/etc/hosts",
		productUUIDPath: productUUIDPath,
		provisionedPath: waagentProvisionedPath,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
		runCommand: runBootstrapCommand,
	}
}

func runBootstrapCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *App) runBootstrapCommand(ctx context.Context) error {
	slog.Info("aks-node-controller started", "task", "Bootstrap")
	start := time.Now()
	a.eventLogger.LogEvent("Bootstrap", "Starting", helpers.EventLevelInformational, start, start)
	run := a.bootstrapFn
	if run == nil {
		run = func(ctx context.Context) error {
			return bootstrap(ctx, defaultBootstrapConfig())
		}
	}
	err := run(ctx)
	end := time.Now()
	if err != nil {
		a.eventLogger.LogEvent("Bootstrap", err.Error(), helpers.EventLevelError, start, end)
		slog.Error("aks-node-controller bootstrap failed", "error", err)
		return err
	}
	a.eventLogger.LogEvent("Bootstrap", "Completed", helpers.EventLevelInformational, start, end)
	slog.Info("aks-node-controller bootstrap finished")
	return nil
}

func bootstrap(ctx context.Context, cfg bootstrapConfig) error {
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()
	start := time.Now()

	if err := markWaagentProvisioned(ctx, cfg); err != nil {
		return err
	}
	slog.Info("bootstrap milestone", "stage", "waagent-marker-complete", "elapsed", time.Since(start))

	userDataResult := make(chan error, 1)
	var group errgroup.Group
	group.Go(func() error {
		err := fetchAndRunUserData(ctx, cfg)
		if err == nil {
			slog.Info("bootstrap milestone", "stage", "userdata-complete", "elapsed", time.Since(start))
		}
		userDataResult <- err
		return err
	})
	group.Go(func() error {
		err := fetchAndApplyIMDSProfile(ctx, cfg)
		if err == nil {
			slog.Info("bootstrap milestone", "stage", "imds-profile-complete", "elapsed", time.Since(start))
		}
		return err
	})
	group.Go(func() error {
		boothook, err := fetchBoothook(ctx, cfg.httpClient)
		if err != nil {
			return err
		}
		slog.Info("bootstrap milestone", "stage", "protected-settings-ready", "elapsed", time.Since(start))
		if err := <-userDataResult; err != nil {
			return err
		}
		if err := writeAtomic(cfg.boothookPath, boothook, 0o600); err != nil {
			return fmt.Errorf("write boothook: %w", err)
		}
		if err := cfg.runCommand(ctx, "/bin/bash", cfg.boothookPath); err != nil {
			return fmt.Errorf("materialize protected CSE settings: %w", err)
		}
		slog.Info("bootstrap milestone", "stage", "boothook-complete", "elapsed", time.Since(start))
		return nil
	})
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

func fetchAndRunUserData(ctx context.Context, cfg bootstrapConfig) error {
	body, err := retry(ctx, 100*time.Millisecond, func() ([]byte, error) {
		return httpGet(ctx, cfg.httpClient, imdsUserDataURL, map[string]string{"Metadata": "true"}, false)
	})
	if err != nil {
		return fmt.Errorf("fetch IMDS UserData: %w", err)
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(body)))
	if err != nil {
		return fmt.Errorf("decode IMDS UserData: %w", err)
	}
	if !bytes.HasPrefix(decoded, userDataPrefix) {
		return errors.New("IMDS UserData does not match the AKS bootstrap format")
	}
	if err := writeAtomic(cfg.userDataPath, decoded, 0o600); err != nil {
		return fmt.Errorf("write UserData: %w", err)
	}
	if err := cfg.runCommand(ctx, "/bin/bash", cfg.userDataPath); err != nil {
		return fmt.Errorf("execute UserData: %w", err)
	}
	return nil
}

func fetchAndApplyIMDSProfile(ctx context.Context, cfg bootstrapConfig) error {
	profile, err := retry(ctx, 100*time.Millisecond, func() (imdsProfile, error) {
		body, getErr := httpGet(ctx, cfg.httpClient, imdsComputeURL, map[string]string{"Metadata": "true"}, false)
		if getErr != nil {
			return imdsProfile{}, getErr
		}
		return parseIMDSProfile(body)
	})
	if err != nil {
		return fmt.Errorf("fetch compute profile from IMDS: %w", err)
	}
	if err := provisionSSH(ctx, cfg, profile.username, profile.sshKeys); err != nil {
		return fmt.Errorf("provision SSH access: %w", err)
	}
	slog.Info("bootstrap milestone", "stage", "ssh-ready")
	if err := writeAtomic(cfg.hostnamePath, []byte(profile.hostname+"\n"), 0o644); err != nil {
		return fmt.Errorf("write hostname: %w", err)
	}
	hosts, err := os.ReadFile(cfg.hostsPath)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}
	updated := updateHostsFile(string(hosts), profile.hostname)
	if err := writeAtomic(cfg.hostsPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write hosts file: %w", err)
	}
	if err := cfg.runCommand(ctx, "hostnamectl", "set-hostname", profile.hostname); err != nil {
		return fmt.Errorf("set hostname: %w", err)
	}
	return nil
}

func parseIMDSProfile(body []byte) (imdsProfile, error) {
	var compute struct {
		OSProfile struct {
			AdminUsername string `json:"adminUsername"`
			ComputerName  string `json:"computerName"`
		} `json:"osProfile"`
		PublicKeys []struct {
			KeyData string `json:"keyData"`
		} `json:"publicKeys"`
	}
	if err := json.Unmarshal(body, &compute); err != nil {
		return imdsProfile{}, fmt.Errorf("parse IMDS compute metadata: %w", err)
	}
	hostname := strings.TrimSpace(compute.OSProfile.ComputerName)
	username := strings.TrimSpace(compute.OSProfile.AdminUsername)
	if !hostnameRE.MatchString(hostname) {
		return imdsProfile{}, fmt.Errorf("invalid hostname %q", hostname)
	}
	if !usernameRE.MatchString(username) {
		return imdsProfile{}, fmt.Errorf("invalid admin username %q", username)
	}
	keys := make([]string, 0, len(compute.PublicKeys))
	for _, publicKey := range compute.PublicKeys {
		key := strings.TrimSpace(publicKey.KeyData)
		if !validSSHKey(key) {
			return imdsProfile{}, errors.New("IMDS returned an invalid SSH public key")
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return imdsProfile{}, errors.New("IMDS returned no SSH public keys")
	}
	return imdsProfile{hostname: hostname, username: username, sshKeys: keys}, nil
}

func updateHostsFile(contents, hostname string) string {
	lines := strings.Split(strings.TrimSuffix(contents, "\n"), "\n")
	replaced := false
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "127.0.1.1" {
			lines[i] = "127.0.1.1 " + hostname
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, "127.0.1.1 "+hostname)
	}
	return strings.Join(lines, "\n") + "\n"
}

func fetchBoothook(ctx context.Context, client *http.Client) ([]byte, error) {
	settings, state, err := waitForProtectedSettings(ctx, client)
	if err != nil {
		return nil, err
	}
	material, err := fetchCertificateMaterial(ctx, client, state.certificatesURL)
	if err != nil {
		return nil, err
	}
	identity, err := identityForThumbprint(material, settings.thumbprint)
	if err != nil {
		return nil, fmt.Errorf("resolve protected-settings certificate: %w", err)
	}

	plain, err := decryptCMS(ctx, settings.protected, identity)
	if err != nil {
		return nil, fmt.Errorf("decrypt protected settings: %w", err)
	}
	var document protectedSettingsDocument
	if err := json.Unmarshal(plain, &document); err != nil {
		return nil, fmt.Errorf("parse protected settings: %w", err)
	}
	if document.CommandToExecute == "" {
		return nil, errors.New("protected settings have no commandToExecute")
	}
	return extractBoothook(document.CommandToExecute)
}

func waitForProtectedSettings(ctx context.Context, client *http.Client) (protectedSettings, goalState, error) {
	type result struct {
		settings protectedSettings
		state    goalState
	}
	found, err := retry(ctx, 100*time.Millisecond, func() (result, error) {
		state, fetchErr := fetchGoalState(ctx, client)
		if fetchErr != nil {
			return result{}, fetchErr
		}
		settings, fetchErr := settingsFromHostPlugin(ctx, client, state)
		if fetchErr == nil && settings.protected != "" {
			return result{settings: settings, state: state}, nil
		}
		settings, fetchErr = settingsFromWireServer(ctx, client, state)
		if fetchErr != nil {
			return result{}, fetchErr
		}
		if settings.protected == "" {
			return result{}, errors.New("CustomScript protected settings are not published")
		}
		return result{settings: settings, state: state}, nil
	})
	if err != nil {
		return protectedSettings{}, goalState{}, fmt.Errorf("fetch protected CSE settings: %w", err)
	}
	return found.settings, found.state, nil
}

func fetchGoalState(ctx context.Context, client *http.Client) (goalState, error) {
	body, err := httpGet(ctx, client, goalStateURL, wireHeaders(), false)
	if err != nil {
		return goalState{}, err
	}
	var document struct {
		Container struct {
			ContainerID      string `xml:"ContainerId"`
			RoleInstanceList struct {
				RoleInstance struct {
					Configuration struct {
						ConfigName       string `xml:"ConfigName"`
						ExtensionsConfig string `xml:"ExtensionsConfig"`
						Certificates     string `xml:"Certificates"`
					} `xml:"Configuration"`
				} `xml:"RoleInstance"`
			} `xml:"RoleInstanceList"`
		} `xml:"Container"`
	}
	if err := xml.Unmarshal(body, &document); err != nil {
		return goalState{}, fmt.Errorf("parse goal state: %w", err)
	}
	configuration := document.Container.RoleInstanceList.RoleInstance.Configuration
	return goalState{
		containerID:         strings.TrimSpace(document.Container.ContainerID),
		roleConfigName:      strings.TrimSpace(configuration.ConfigName),
		extensionsConfigURL: strings.TrimSpace(configuration.ExtensionsConfig),
		certificatesURL:     strings.TrimSpace(configuration.Certificates),
	}, nil
}

func settingsFromHostPlugin(ctx context.Context, client *http.Client, state goalState) (protectedSettings, error) {
	if state.containerID == "" || state.roleConfigName == "" {
		return protectedSettings{}, errors.New("goal state is missing ContainerId or ConfigName")
	}
	headers := map[string]string{
		"x-ms-version":          hostPluginAPIVersion,
		"x-ms-containerid":      state.containerID,
		"x-ms-host-config-name": state.roleConfigName,
		"x-ms-correlationid":    randomCorrelationID(),
	}
	body, err := httpGet(ctx, client, vmSettingsURL, headers, false)
	if err != nil {
		return protectedSettings{}, err
	}
	var document struct {
		ExtensionGoalStates []struct {
			Name     string `json:"name"`
			Settings []struct {
				ProtectedSettings               string `json:"protectedSettings"`
				ProtectedSettingsCertThumbprint string `json:"protectedSettingsCertThumbprint"`
			} `json:"settings"`
		} `json:"extensionGoalStates"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return protectedSettings{}, fmt.Errorf("parse HostGAPlugin vmSettings: %w", err)
	}
	for _, extension := range document.ExtensionGoalStates {
		if extension.Name != customScriptHandlerName {
			continue
		}
		for _, settings := range extension.Settings {
			if settings.ProtectedSettings != "" && settings.ProtectedSettingsCertThumbprint != "" {
				return protectedSettings{
					protected:  settings.ProtectedSettings,
					thumbprint: normalizeThumbprint(settings.ProtectedSettingsCertThumbprint),
				}, nil
			}
		}
	}
	return protectedSettings{}, nil
}

func settingsFromWireServer(ctx context.Context, client *http.Client, state goalState) (protectedSettings, error) {
	if state.extensionsConfigURL == "" {
		return protectedSettings{}, errors.New("goal state has no ExtensionsConfig URL")
	}
	body, err := httpGet(ctx, client, state.extensionsConfigURL, wireHeaders(), true)
	if err != nil {
		return protectedSettings{}, err
	}
	var document struct {
		PluginSettings []struct {
			Plugin struct {
				Name            string `xml:"name,attr"`
				RuntimeSettings struct {
					Text string `xml:",chardata"`
				} `xml:"RuntimeSettings"`
			} `xml:"Plugin"`
		} `xml:"PluginSettings"`
	}
	if err := xml.Unmarshal(body, &document); err != nil {
		return protectedSettings{}, fmt.Errorf("parse ExtensionsConfig: %w", err)
	}
	for _, pluginSettings := range document.PluginSettings {
		if pluginSettings.Plugin.Name != customScriptHandlerName {
			continue
		}
		var runtime struct {
			RuntimeSettings []struct {
				HandlerSettings struct {
					ProtectedSettings               string `json:"protectedSettings"`
					ProtectedSettingsCertThumbprint string `json:"protectedSettingsCertThumbprint"`
				} `json:"handlerSettings"`
			} `json:"runtimeSettings"`
		}
		if err := json.Unmarshal([]byte(pluginSettings.Plugin.RuntimeSettings.Text), &runtime); err != nil {
			return protectedSettings{}, fmt.Errorf("parse CustomScript runtime settings: %w", err)
		}
		for _, entry := range runtime.RuntimeSettings {
			if entry.HandlerSettings.ProtectedSettings != "" && entry.HandlerSettings.ProtectedSettingsCertThumbprint != "" {
				return protectedSettings{
					protected:  entry.HandlerSettings.ProtectedSettings,
					thumbprint: normalizeThumbprint(entry.HandlerSettings.ProtectedSettingsCertThumbprint),
				}, nil
			}
		}
	}
	return protectedSettings{}, nil
}

func fetchCertificateMaterial(ctx context.Context, client *http.Client, certificatesURL string) (certificateMaterial, error) {
	if certificatesURL == "" {
		return certificateMaterial{}, errors.New("goal state has no Certificates URL")
	}
	transportKey, transportCert, err := mintTransportIdentity()
	if err != nil {
		return certificateMaterial{}, err
	}
	certHeader := base64.StdEncoding.EncodeToString(transportCert.Raw)
	var lastErr error
	for _, cipherName := range []string{"AES128_CBC", "DES_EDE3_CBC"} {
		headers := wireHeaders()
		headers["x-ms-guest-agent-public-x509-cert"] = certHeader
		headers["x-ms-cipher-name"] = cipherName
		body, getErr := httpGet(ctx, client, certificatesURL, headers, true)
		if getErr != nil {
			lastErr = getErr
			continue
		}
		var response struct {
			Data   string `xml:"Data"`
			Format string `xml:"Format"`
		}
		if unmarshalErr := xml.Unmarshal(body, &response); unmarshalErr != nil {
			lastErr = fmt.Errorf("parse Certificates response: %w", unmarshalErr)
			continue
		}
		if strings.TrimSpace(response.Format) != "" && strings.TrimSpace(response.Format) != expectedCertFormat {
			return certificateMaterial{}, fmt.Errorf("unexpected Certificates format %q", response.Format)
		}
		pfx, decryptErr := decryptCMS(ctx, response.Data, certificateIdentity{key: transportKey, cert: transportCert})
		if decryptErr != nil {
			lastErr = decryptErr
			continue
		}
		material, decodeErr := decodePFX(pfx)
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}
		return material, nil
	}
	return certificateMaterial{}, fmt.Errorf("download GoalState certificates: %w", lastErr)
}

func mintTransportIdentity() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate transport key: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: "LinuxTransport"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(730 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create transport certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("parse transport certificate: %w", err)
	}
	return key, cert, nil
}

func randomSerialNumber() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	value, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return value
}

func decodePFX(pfx []byte) (certificateMaterial, error) {
	blocks, err := pkcs12.ToPEM(pfx, "")
	if err != nil {
		return certificateMaterial{}, fmt.Errorf("decode PKCS#12 certificate bundle: %w", err)
	}
	material := certificateMaterial{}
	for _, block := range blocks {
		switch block.Type {
		case "CERTIFICATE":
			cert, parseErr := x509.ParseCertificate(block.Bytes)
			if parseErr != nil {
				return certificateMaterial{}, fmt.Errorf("parse PFX certificate: %w", parseErr)
			}
			material.certs = append(material.certs, cert)
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
			key, parseErr := parsePrivateKey(block.Bytes)
			if parseErr != nil {
				return certificateMaterial{}, parseErr
			}
			material.keys = append(material.keys, key)
		}
	}
	if len(material.keys) == 0 || len(material.certs) == 0 {
		return certificateMaterial{}, errors.New("PKCS#12 bundle has no private keys or certificates")
	}
	return material, nil
}

func parsePrivateKey(raw []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(raw); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(raw); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse PFX private key: %w", err)
	}
	return key, nil
}

func identityForThumbprint(material certificateMaterial, thumbprint string) (certificateIdentity, error) {
	for _, cert := range material.certs {
		if certificateThumbprint(cert) != normalizeThumbprint(thumbprint) {
			continue
		}
		for _, key := range material.keys {
			if publicKeysEqual(cert.PublicKey, publicKey(key)) {
				return certificateIdentity{key: key, cert: cert}, nil
			}
		}
		return certificateIdentity{}, fmt.Errorf("certificate %s has no matching private key", thumbprint)
	}
	return certificateIdentity{}, fmt.Errorf("certificate %s is absent from GoalState", thumbprint)
}

func decryptCMS(ctx context.Context, encoded string, identity certificateIdentity) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	return decryptCMSDER(ctx, raw, identity)
}

func decryptCMSDER(ctx context.Context, raw []byte, identity certificateIdentity) ([]byte, error) {
	workDir, err := os.MkdirTemp("", "aks-cms-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	keyDER, err := x509.MarshalPKCS8PrivateKey(identity.key)
	if err != nil {
		return nil, fmt.Errorf("marshal CMS private key: %w", err)
	}
	keyPath := filepath.Join(workDir, "recipient-key.pem")
	certPath := filepath.Join(workDir, "recipient-cert.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return nil, fmt.Errorf("write CMS private key: %w", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: identity.cert.Raw}), 0o600); err != nil {
		return nil, fmt.Errorf("write CMS certificate: %w", err)
	}

	cmd := exec.CommandContext(ctx, "openssl", "cms", "-decrypt", "-binary", "-inform", "DER", "-inkey", keyPath, "-recip", certPath)
	cmd.Stdin = bytes.NewReader(raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("openssl cms decrypt failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func extractBoothook(command string) ([]byte, error) {
	match := boothookRE.FindStringSubmatch(strings.TrimSpace(command))
	if len(match) != 2 {
		return nil, errors.New("commandToExecute does not carry a scriptless-phase2 boothook payload")
	}
	payload := strings.Join(strings.Fields(match[1]), "")
	compressed, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode boothook payload: %w", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open boothook gzip payload: %w", err)
	}
	defer reader.Close()
	boothook, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("decompress boothook payload: %w", err)
	}
	return boothook, nil
}

func provisionSSH(ctx context.Context, cfg bootstrapConfig, username string, keys []string) error {
	account, err := ensureAdminUser(ctx, cfg, username)
	if err != nil {
		return err
	}
	uid, err := parseID(account.Uid)
	if err != nil {
		return err
	}
	gid, err := parseID(account.Gid)
	if err != nil {
		return err
	}
	sshDir := filepath.Join(account.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.Chown(sshDir, uid, gid); err != nil {
		return err
	}
	authorizedKeys := filepath.Join(sshDir, "authorized_keys")
	if err := writeAtomic(authorizedKeys, []byte(strings.Join(keys, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Chown(authorizedKeys, uid, gid); err != nil {
		return err
	}
	if err := writeAtomic("/etc/sudoers.d/90-azure-"+username, []byte(username+" ALL=(ALL) NOPASSWD:ALL\n"), 0o440); err != nil {
		return err
	}
	return startSSHService(ctx, cfg)
}

func startSSHService(ctx context.Context, cfg bootstrapConfig) error {
	if err := cfg.runCommand(ctx, "ssh-keygen", "-A"); err != nil {
		return fmt.Errorf("generate SSH host keys: %w", err)
	}
	// waagent deprovisioning removes host keys. ssh.service may race this bootstrap,
	// fail its initial configuration check, and remain failed after the keys exist.
	if err := cfg.runCommand(ctx, "systemctl", "restart", "ssh.service"); err != nil {
		return fmt.Errorf("restart SSH service: %w", err)
	}
	return nil
}

func validSSHKey(value string) bool {
	return (strings.HasPrefix(value, "ssh-") || strings.HasPrefix(value, "ecdsa-") || strings.HasPrefix(value, "sk-")) &&
		!strings.ContainsAny(value, "\r\n")
}

func ensureAdminUser(ctx context.Context, cfg bootstrapConfig, username string) (*user.User, error) {
	account, err := user.Lookup(username)
	if err != nil {
		if _, ok := err.(user.UnknownUserError); !ok {
			return nil, err
		}
		if err := cfg.runCommand(ctx, "useradd", "--create-home", "--shell", "/bin/bash", "--groups", "adm,sudo", username); err != nil {
			return nil, err
		}
		account, err = user.Lookup(username)
		if err != nil {
			return nil, err
		}
	} else if err := cfg.runCommand(ctx, "usermod", "--append", "--groups", "adm,sudo", username); err != nil {
		return nil, err
	}
	if err := cfg.runCommand(ctx, "usermod", "--password", "*", "--shell", "/bin/bash", username); err != nil {
		return nil, err
	}
	return account, nil
}

func markWaagentProvisioned(ctx context.Context, cfg bootstrapConfig) error {
	uuid, err := retry(ctx, 100*time.Millisecond, func() ([]byte, error) {
		value, readErr := os.ReadFile(cfg.productUUIDPath)
		if readErr != nil {
			return nil, readErr
		}
		if len(bytes.TrimSpace(value)) == 0 {
			return nil, errors.New("product UUID is empty")
		}
		return value, nil
	})
	if err != nil {
		return fmt.Errorf("read product UUID: %w", err)
	}
	if err := writeAtomic(cfg.provisionedPath, uuid, 0o644); err != nil {
		return fmt.Errorf("mark waagent provisioned: %w", err)
	}
	return nil
}

func httpGet(ctx context.Context, client *http.Client, url string, headers map[string]string, redactQuery bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		shownURL := url
		if redactQuery {
			shownURL = strings.SplitN(url, "?", 2)[0] + "?<redacted>"
		}
		return nil, fmt.Errorf("GET %s returned %s", shownURL, response.Status)
	}
	return io.ReadAll(response.Body)
}

func retry[T any](ctx context.Context, interval time.Duration, operation func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	nextInterval := interval
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return zero, fmt.Errorf("%w: %v", ctx.Err(), lastErr)
			}
			return zero, ctx.Err()
		case <-timer.C:
			value, err := operation()
			if err == nil {
				return value, nil
			}
			lastErr = err
			timer.Reset(nextInterval)
			if nextInterval < 2*time.Second {
				nextInterval *= 2
			}
		}
	}
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".aks-bootstrap-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func wireHeaders() map[string]string {
	return map[string]string{
		"x-ms-agent-name": "WALinuxAgent",
		"x-ms-version":    wireProtocolVersion,
		"Content-Type":    "text/xml;charset=utf-8",
	}
}

func certificateThumbprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw)
	return strings.ToUpper(fmt.Sprintf("%x", sum[:]))
}

func normalizeThumbprint(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
}

func publicKey(key crypto.PrivateKey) crypto.PublicKey {
	if signer, ok := key.(crypto.Signer); ok {
		return signer.Public()
	}
	return nil
}

func publicKeysEqual(left, right crypto.PublicKey) bool {
	leftDER, leftErr := x509.MarshalPKIXPublicKey(left)
	rightDER, rightErr := x509.MarshalPKIXPublicKey(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftDER, rightDER)
}

func randomCorrelationID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func parseID(value string) (int, error) {
	var id int
	if _, err := fmt.Sscanf(value, "%d", &id); err != nil {
		return 0, fmt.Errorf("parse account id %q: %w", value, err)
	}
	return id, nil
}
