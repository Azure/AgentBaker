package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstaller(t *testing.T) {
	if _, ok := os.LookupEnv("ENABLE_INSTALLER_TEST"); !ok {
		t.Skip("ENABLE_INSTALLER_TEST is not set")
	}
	t.Parallel()
	ctx := newTestCtx(t)
	script := installerScript(ctx, t)
	t.Log(script)
	err := ensureResourceGroup(ctx)
	require.NoError(t, err)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)
	if err := writeToFile(t, "sshkey", string(privateKeyBytes)); err != nil {
		t.Logf("failed to private ssh key to disk: %s", err)
	}
	vmssName := getVmssName(t)
	vm := createInstallerVMSSS(ctx, t, vmssName, privateKeyBytes, publicKeyBytes, script)
	time.Sleep(2 * time.Minute)
	t.Logf("VMSS %q created", *vm.ID)
}

func installerScript(ctx context.Context, t *testing.T) string {
	binary := compileInstaller(t)
	url, err := config.Azure.UploadAndGetLink(ctx, "installer-"+hashFile(t, binary.Name()), binary)
	require.NoError(t, err)
	// content of /var/log/azure/cluster-provision-cse-output.log is automatically exported from the VM by the test helpers
	// we want to check the installer stderr
	return fmt.Sprintf(`bash -c "(curl -L -o ./installer '%s' && chmod +x ./installer && mkdir -p /var/log/azure && ./installer) > /var/log/azure/installer.log 2>&1"`, url)
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

func createInstallerVMSSS(ctx context.Context, t *testing.T, vmssName string, privateKeyBytes, publicKeyBytes []byte, script string) *armcompute.VirtualMachineScaleSet {
	clusterConfig, err := ClusterKubenet(ctx, t)
	require.NoError(t, err)
	t.Logf("creating VMSS %q in resource group %q", vmssName, *clusterConfig.Model.Properties.NodeResourceGroup)
	model := getBaseVMSSModel(vmssName, string(publicKeyBytes), "", script, clusterConfig)
	imageID, err := config.VHDUbuntu2204Gen2Containerd.VHDResourceID(ctx, t)
	require.NoError(t, err)
	model.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
		ID: to.Ptr(string(imageID)),
	}

	operation, err := config.Azure.VMSS.BeginCreateOrUpdate(
		ctx,
		*clusterConfig.Model.Properties.NodeResourceGroup,
		vmssName,
		model,
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupVMSS(ctx, t, vmssName, clusterConfig, privateKeyBytes)
	})

	vmssResp, err := operation.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
		Frequency: 10 * time.Second,
	})
	// fail test, but continue to extract debug information
	require.NoError(t, err, "create vmss %q, check %s for vm logs", vmssName, testDir(t))
	return &vmssResp.VirtualMachineScaleSet
}
