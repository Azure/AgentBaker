package scenario

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const cloudInitRepositoryEndpoint = "http://127.0.0.1:17885"
const cloudInitStateDir = "/var/lib/anc-cloud-integration"
const cloudInitANCPath = "/opt/azure/containers/aks-node-controller-hotfix"
const cloudInitNodeDataHeader = "cat <<'EOF' | base64 -d | gzip -d >/opt/azure/containers/nodecustomdata.yml\n"
const cloudInitLegacyPath = "/opt/azure/containers/init-aks-cloud.sh"
const cloudInitLegacyOriginalPath = "/opt/azure/containers/init-aks-cloud.anc-integration-original.sh"
const cloudInitRootOwner = "root"

const cloudInitLegacyWrapper = `#!/bin/bash
printf 'invoked\n' >> /var/lib/anc-cloud-integration/legacy-init-called
exec /bin/bash /opt/azure/containers/init-aks-cloud.anc-integration-original.sh "$@"
`

type cloudInitializationArtifacts struct {
	ANC, Proxy, Tests, SHA256 string
}

func buildCloudInitializationArtifacts(ctx context.Context, arch string) (cloudInitializationArtifacts, error) {
	root, err := findRepoRoot()
	if err != nil {
		return cloudInitializationArtifacts{}, err
	}
	artifacts := cloudInitializationArtifacts{}
	for _, build := range []struct {
		directory string
		args      []string
		url       *string
		hash      bool
	}{
		{"aks-node-controller", []string{goBuildCommand}, &artifacts.ANC, true},
		{"e2e/cmd/cloud-repo-proxy", []string{goBuildCommand}, &artifacts.Proxy, false},
		{"aks-node-controller", []string{"test", "-c", "-tags=cloudintegration"}, &artifacts.Tests, false},
	} {
		binary, err := compileLinuxGoBinary(ctx, arch, filepath.Join(root, build.directory), build.args)
		if err != nil {
			return artifacts, err
		}
		var digest string
		*build.url, digest, err = uploadCloudInitializationArtifact(ctx, binary, build.hash)
		if err != nil {
			return artifacts, err
		}
		if build.hash {
			artifacts.SHA256 = digest
		}
	}
	return artifacts, nil
}

func uploadCloudInitializationArtifact(ctx context.Context, binary *os.File, hashBinary bool) (string, string, error) {
	location, digest, uploadErr := func() (string, string, error) {
		digest := ""
		if hashBinary {
			hash := sha256.New()
			if _, err := io.Copy(hash, binary); err != nil {
				return "", "", fmt.Errorf("hash PR-built ANC: %w", err)
			}
			digest = hex.EncodeToString(hash.Sum(nil))
			if _, err := binary.Seek(0, io.SeekStart); err != nil {
				return "", "", fmt.Errorf("rewind PR-built ANC: %w", err)
			}
		}
		location, err := uploadAKSNodeController(ctx, binary)
		return location, digest, err
	}()
	cleanupErr := errors.Join(binary.Close(), os.Remove(binary.Name()), os.Remove(filepath.Dir(binary.Name())))
	return location, digest, errors.Join(uploadErr, cleanupErr)
}

func customDataWithCloudInitializationFixture(encoded string, artifacts cloudInitializationArtifacts) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode cloud initialization custom data: %w", err)
	}
	if strings.Count(string(data), hotfixMarker) != 1 {
		return "", fmt.Errorf("cloud initialization fixture requires exactly one hotfix marker")
	}
	digest, err := hex.DecodeString(artifacts.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return "", fmt.Errorf("cloud initialization fixture requires an ANC SHA256")
	}
	for _, raw := range []string{artifacts.ANC, artifacts.Proxy, artifacts.Tests} {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" ||
			strings.ContainsAny(raw, "\r\n`$\\\"") {
			return "", fmt.Errorf("invalid cloud initialization artifact URL")
		}
	}
	data, err = instrumentCloudInitializationNodeData(data)
	if err != nil {
		return "", err
	}
	script := strings.NewReplacer(
		"@ANC_URL@", strconv.Quote(artifacts.ANC),
		"@PROXY_URL@", strconv.Quote(artifacts.Proxy),
		"@TESTS_URL@", strconv.Quote(artifacts.Tests),
		"@ANC_SHA256@", artifacts.SHA256,
	).Replace(cloudInitializationBootstrap)
	result := strings.Replace(string(data), hotfixMarker, script, 1)
	if len(result) > 64*1024 {
		return "", fmt.Errorf("cloud initialization fixture exceeds the Azure CustomData limit")
	}
	return base64.StdEncoding.EncodeToString([]byte(result)), nil
}

// ANC applies nodecustomdata.yml after the boothook hook. Instrument that
// payload, rather than wrapping only the existing VHD file: materialization
// would erase that observer and invalidate a no-shell-initialization assertion.
func instrumentCloudInitializationNodeData(boothook []byte) ([]byte, error) {
	text := string(boothook)
	if strings.Count(text, cloudInitNodeDataHeader) != 1 {
		return nil, fmt.Errorf("cloud initialization fixture requires one rendered nodecustomdata payload")
	}
	start := strings.Index(text, cloudInitNodeDataHeader) + len(cloudInitNodeDataHeader)
	length := strings.Index(text[start:], "\nEOF")
	if length < 0 {
		return nil, fmt.Errorf("cloud initialization nodecustomdata payload is unterminated")
	}
	document, err := decodeCloudInitializationNodeData(text[start : start+length])
	if err != nil {
		return nil, err
	}
	files := cloudFixtureMappingValue(document.Content[0], "write_files")
	if files == nil || files.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("cloud initialization nodecustomdata requires write_files")
	}
	count := 0
	for _, file := range files.Content {
		path := cloudFixtureMappingValue(file, "path")
		if path != nil && path.Value == cloudInitLegacyOriginalPath {
			return nil, fmt.Errorf("cloud initialization nodecustomdata is already instrumented")
		}
		if path != nil && path.Value == cloudInitLegacyPath {
			path.Value = cloudInitLegacyOriginalPath
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("cloud initialization nodecustomdata requires exactly one real legacy initializer")
	}
	var wrapper yaml.Node
	if encodeErr := wrapper.Encode(scriptHotfixFixtureWriteFile{
		Path: cloudInitLegacyPath, Permissions: "0744", Owner: cloudInitRootOwner,
		Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(cloudInitLegacyWrapper)),
	}); encodeErr != nil {
		return nil, fmt.Errorf("encode legacy initialization observer: %w", encodeErr)
	}
	files.Content = append(files.Content, &wrapper)
	raw, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode instrumented nodecustomdata: %w", err)
	}
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, writeErr := writer.Write(raw); writeErr != nil {
		return nil, writeErr
	}
	if closeErr := writer.Close(); closeErr != nil {
		return nil, closeErr
	}
	payload := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return []byte(text[:start] + payload + text[start+length:]), nil
}

func decodeCloudInitializationNodeData(payload string) (*yaml.Node, error) {
	compressed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return nil, fmt.Errorf("decode cloud initialization nodecustomdata: %w", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open cloud initialization nodecustomdata: %w", err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(reader, (8<<20)+1))
	if readErr = errors.Join(readErr, reader.Close()); readErr != nil {
		return nil, fmt.Errorf("read cloud initialization nodecustomdata: %w", readErr)
	}
	if len(raw) > 8<<20 {
		return nil, fmt.Errorf("cloud initialization nodecustomdata exceeds 8 MiB")
	}
	var document yaml.Node
	if decodeErr := yaml.Unmarshal(raw, &document); decodeErr != nil {
		return nil, fmt.Errorf("decode cloud initialization nodecustomdata YAML: %w", decodeErr)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("cloud initialization nodecustomdata must be a YAML mapping")
	}
	return &document, nil
}

func cloudFixtureMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

const cloudInitializationBootstrap = `(
set -euo pipefail
state=/var/lib/anc-cloud-integration
mkdir -p "$state"
chmod 0700 "$state"
curl --fail --silent --show-error --retry 5 @ANC_URL@ -o /opt/azure/containers/aks-node-controller-hotfix
curl --fail --silent --show-error --retry 5 @PROXY_URL@ -o "$state/repository-proxy"
curl --fail --silent --show-error --retry 5 @TESTS_URL@ -o "$state/system-integration.test"
printf '%s\n' '@ANC_SHA256@' > "$state/anc.sha256"
printf '%s  %s\n' '@ANC_SHA256@' /opt/azure/containers/aks-node-controller-hotfix | sha256sum --check -
chmod 0755 /opt/azure/containers/aks-node-controller-hotfix "$state/repository-proxy" "$state/system-integration.test"
printf 'disposable AgentBaker cloud integration\n' > "$state/allow-system-tests"
cat > /etc/systemd/system/anc-cloud-repository-fixture.service <<'CLOUD_REPOSITORY_UNIT'
[Unit]
Description=AgentBaker E2E vendor repository proxy
After=network-online.target
[Service]
ExecStart=/var/lib/anc-cloud-integration/repository-proxy
Restart=on-failure
CLOUD_REPOSITORY_UNIT
systemctl daemon-reload
systemctl start anc-cloud-repository-fixture.service
for attempt in $(seq 1 30); do
    if curl --fail --silent http://127.0.0.1:17885/anc-integration/stats > /dev/null; then break; fi
    sleep 1
done
curl --fail --silent --show-error http://127.0.0.1:17885/anc-integration/stats > /dev/null
) || { echo 'cloud initialization integration setup failed' >&2; exit 1; }
`
