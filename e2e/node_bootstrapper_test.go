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

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
	"github.com/Azure/agentbakere2e/config"
	"github.com/stretchr/testify/require"
)

// test node-bootstrapper binary without rebuilding VHD images.
// it compiles the node-bootstrapper binary and uploads it to Azure Storage.
// the binary is then downloaded and executed on the VM
// the test results are unreliable, as there can be a version mismatch between the binary and the rest content of VHD image
// it's intended to be used for quick testing without rebuilding VHD images
// mostly executed locally
func Test_ubuntu2204NodeBootstrapper(t *testing.T) {
	ctx := newTestCtx(t)
	if !config.Config.EnableNodeBootstrapperTest {
		t.Skip("ENABLE_NODE_BOOTSTRAPPER_TEST is not set")
	}
	// TODO: figure out how to properly parallelize test, maybe move t.Parallel to the top of each test?
	cluster, err := ClusterKubenet(ctx, t)
	require.NoError(t, err)
	t.Cleanup(func() {
		log, err := os.ReadFile("./scenario-logs/" + t.Name() + "/node-bootstrapper.stdout.txt")
		if err != nil {
			t.Logf("failed to read node-bootstrapper log: %v", err)
		}
		t.Logf("node-bootstrapper log: %s", string(log))
	})
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			//NodeBootstrappingType: Scriptless,
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			LiveVMValidators: []*LiveVMValidator{
				mobyComponentVersionValidator("containerd", getExpectedPackageVersions("containerd", "ubuntu", "r2204")[0], "apt"),
				mobyComponentVersionValidator("runc", getExpectedPackageVersions("runc", "ubuntu", "r2204")[0], "apt"),
			},
			CSEOverride:       CSENodeBootstrapper(ctx, t, cluster),
			DisableCustomData: true,
		},
	})
}

func CSENodeBootstrapper(ctx context.Context, t *testing.T, cluster *Cluster) string {
	configJSON, err := json.Marshal(nbcontractv1.Configuration{
		Version: "v0",
	})
	require.NoError(t, err)

	binary := compileNodeBootstrapper(t)
	url, err := config.Azure.UploadAndGetLink(ctx, "node-bootstrapper-"+hashFile(t, binary.Name()), binary)
	require.NoError(t, err)
	return fmt.Sprintf(`sh -c "(mkdir -p /etc/node-bootstrapper && echo '%s' | base64 -d > /etc/node-bootstrapper/config.json && curl -L -o ./node-bootstrapper '%s' && chmod +x ./node-bootstrapper && ./node-bootstrapper provision --provision-config=/etc/node-bootstrapper/config.json)"`, base64.StdEncoding.EncodeToString(configJSON), url)
}

func compileNodeBootstrapper(t *testing.T) *os.File {
	cmd := exec.Command("go", "build", "-o", "node-bootstrapper", "-v")
	cmd.Dir = "../node-bootstrapper"
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64",
	)
	log, err := cmd.CombinedOutput()
	require.NoError(t, err, string(log))
	t.Logf("Compiled %s", "../node-bootstrapper")
	f, err := os.Open("../node-bootstrapper/node-bootstrapper")
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
