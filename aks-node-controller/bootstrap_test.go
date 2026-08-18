package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func responseClient(body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
}

func TestExtractBoothook(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte("#!/bin/bash\necho bootstrap\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	command := fmt.Sprintf(
		"echo '%s' | base64 -d | gzip -d > /opt/azure/containers/boothook.sh && ignored",
		base64.StdEncoding.EncodeToString(compressed.Bytes()),
	)
	boothook, err := extractBoothook(command)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\necho bootstrap\n", string(boothook))
}

func TestExtractBoothookRejectsUnexpectedCommand(t *testing.T) {
	_, err := extractBoothook("curl example.invalid | bash")
	require.Error(t, err)
}

func TestUpdateHostsFile(t *testing.T) {
	assert.Equal(t,
		"127.0.0.1 localhost\n127.0.1.1 node-1\n",
		updateHostsFile("127.0.0.1 localhost\n", "node-1"))
	assert.Equal(t,
		"127.0.0.1 localhost\n127.0.1.1 node-2\n",
		updateHostsFile("127.0.0.1 localhost\n127.0.1.1 old\n", "node-2"))
}

func TestParseIMDSProfile(t *testing.T) {
	profile, err := parseIMDSProfile([]byte(`{
		"osProfile": {
			"adminUsername": "azureuser",
			"computerName": "node-1"
		},
		"publicKeys": [{
			"path": "/home/azureuser/.ssh/authorized_keys",
			"keyData": "ssh-rsa AAAA test"
		}]
	}`))
	require.NoError(t, err)
	assert.Equal(t, "azureuser", profile.username)
	assert.Equal(t, "node-1", profile.hostname)
	assert.Equal(t, []string{"ssh-rsa AAAA test"}, profile.sshKeys)
}

func TestStartSSHServiceGeneratesKeysBeforeRestart(t *testing.T) {
	var commands []string
	cfg := bootstrapConfig{
		runCommand: func(_ context.Context, name string, args ...string) error {
			commands = append(commands, strings.Join(append([]string{name}, args...), " "))
			return nil
		},
	}

	require.NoError(t, startSSHService(context.Background(), cfg))
	assert.Equal(t, []string{
		"ssh-keygen -A",
		"systemctl restart ssh.service",
	}, commands)
}

func TestStartSSHServiceDoesNotRestartWhenKeyGenerationFails(t *testing.T) {
	var commands []string
	cfg := bootstrapConfig{
		runCommand: func(_ context.Context, name string, args ...string) error {
			commands = append(commands, strings.Join(append([]string{name}, args...), " "))
			if name == "ssh-keygen" {
				return errors.New("key generation failed")
			}
			return nil
		},
	}

	err := startSSHService(context.Background(), cfg)
	require.ErrorContains(t, err, "generate SSH host keys")
	assert.Equal(t, []string{"ssh-keygen -A"}, commands)
}

func TestFetchGoalState(t *testing.T) {
	client := responseClient(`<GoalState><Container>
<ContainerId>container-id</ContainerId><RoleInstanceList><RoleInstance><Configuration>
<ExtensionsConfig>http://settings</ExtensionsConfig>
<Certificates>http://certificates</Certificates><ConfigName>config-name</ConfigName>
</Configuration></RoleInstance></RoleInstanceList></Container></GoalState>`)

	state, err := fetchGoalState(context.Background(), client)
	require.NoError(t, err)
	assert.Equal(t, "container-id", state.containerID)
	assert.Equal(t, "config-name", state.roleConfigName)
	assert.Equal(t, "http://settings", state.extensionsConfigURL)
	assert.Equal(t, "http://certificates", state.certificatesURL)
}

func TestSettingsFromWireServer(t *testing.T) {
	client := responseClient(`<Extensions><PluginSettings>
<Plugin name="Microsoft.Azure.Extensions.CustomScript">
<RuntimeSettings seqNo="0">{"runtimeSettings":[{"handlerSettings":{
"protectedSettings":"encrypted","protectedSettingsCertThumbprint":"AA:BB"}}]}</RuntimeSettings>
</Plugin></PluginSettings></Extensions>`)

	settings, err := settingsFromWireServer(context.Background(), client, goalState{
		extensionsConfigURL: "http://settings",
	})
	require.NoError(t, err)
	assert.Equal(t, "encrypted", settings.protected)
	assert.Equal(t, "AABB", settings.thumbprint)
}

func TestCMSAndPKCS12RoundTrip(t *testing.T) {
	key, cert, err := mintTransportIdentity()
	require.NoError(t, err)

	pfx, err := pkcs12.Modern.Encode(key, cert, nil, "")
	require.NoError(t, err)
	material, err := decodePFX(pfx)
	require.NoError(t, err)
	identity, err := identityForThumbprint(material, certificateThumbprint(cert))
	require.NoError(t, err)

	opensslPath, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("openssl is not installed")
	}
	workDir := t.TempDir()
	certPath := filepath.Join(workDir, "recipient-cert.pem")
	plainPath := filepath.Join(workDir, "plain")
	require.NoError(t, os.WriteFile(
		certPath,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}),
		0o600))
	require.NoError(t, os.WriteFile(plainPath, []byte("protected settings"), 0o600))
	cmd := exec.Command(opensslPath, "cms", "-encrypt", "-binary", "-in", plainPath, "-outform", "DER", "-aes128", "-keyid", "-recip", certPath)
	encrypted, err := cmd.Output()
	require.NoError(t, err)

	decrypted, err := decryptCMS(context.Background(), base64.StdEncoding.EncodeToString(encrypted), identity)
	require.NoError(t, err)
	assert.Equal(t, "protected settings", string(decrypted))
}
