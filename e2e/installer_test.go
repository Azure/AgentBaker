package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
)

// test installer binary without rebuilding VHD images.
func Test_ubuntu2204Installer(t *testing.T) {
	if _, ok := os.LookupEnv("ENABLE_INSTALLER_TEST"); !ok {
		t.Skip("ENABLE_INSTALLER_TEST is not set")
	}
	// TODO: figure out how to properly parallelize test, maybe move t.Parallel to the top of each test?
	cluster, err := ClusterKubenet(context.TODO(), t)
	require.NoError(t, err)
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator("1.7.20-1"),
				runcVersionValidator("1.1.12-1"),
			},
			CSEOverride:        installerScript(context.TODO(), t, cluster),
			CustomDataOverride: to.Ptr(""),
		},
	})
}

func installerScript(ctx context.Context, t *testing.T, cluster *Cluster) string {
	installerConfigJSON, err := json.Marshal(cluster.NodeBootstrappingConfiguration)
	require.NoError(t, err)

	binary := compileInstaller(t)
	url, err := config.Azure.UploadAndGetLink(ctx, "installer-"+hashFile(t, binary.Name()), binary)
	require.NoError(t, err)
	return fmt.Sprintf(`bash -c "(echo '%s' | base64 -d > config.json && curl -L -o ./installer '%s' && chmod +x ./installer && mkdir -p /var/log/azure && ./installer) > /var/log/azure/installer.log 2>&1"`, base64.StdEncoding.EncodeToString(installerConfigJSON), url)
}

func compileInstaller(t *testing.T) *os.File {
	cmd := exec.Command("go", "build", "-o", "installer", "-v")
	cmd.Dir = "../installer"
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=amd64",
	)
	log, err := cmd.CombinedOutput()
	require.NoError(t, err, string(log))
	t.Logf("Compiled %s", "../installer")
	f, err := os.Open("../installer/installer")
	require.NoError(t, err)
	return f
}

func hashFile(t *testing.T, filePath string) string {
	// Open the file
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	// Create a SHA-256 hasher
	hasher := sha256.New()

	// Copy the file content to the hasher
	_, err = io.Copy(hasher, file)
	require.NoError(t, err)

	// Compute the hash
	hashSum := hasher.Sum(nil)

	// Encode the hash using base32
	encodedHash := base32.StdEncoding.EncodeToString(hashSum)

	// Return the first 5 characters of the encoded hash
	return encodedHash[:5]
}
