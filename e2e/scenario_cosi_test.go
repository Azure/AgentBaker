package e2e

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	aclCOSIAMD64ImageVersion        = "0.20260827.1192019"
	aclCOSIAMD64ImageID             = "/SharedGalleries/035db282-f1c8-4ce7-b78f-2a7265d5398c-ACLDEVEL/Images/acldevel/Versions/0.20260827.1192019"
	remoteCOSIConfigPath            = "/home/azureuser/update-config.yaml"
	remoteCOSIImagePath             = "/home/azureuser/update.cosi"
	cosiAMD64PublishingArtifact     = "cosi-publishing-info-acl-tl-gen2"
	cosiARM64PublishingArtifact     = "cosi-publishing-info-acl-arm64-tl-gen2"
	cosiAMD64FIPSPublishingArtifact = "cosi-publishing-info-acl-fips-tl-gen2"
	cosiARM64FIPSPublishingArtifact = "cosi-publishing-info-acl-arm64-fips-tl-gen2"
)

// cosiPublishingInfo mirrors the JSON written by convert-vhd-to-cosi.sh.
type cosiPublishingInfo struct {
	CosiURL        string `json:"cosi_url"`
	MetadataSHA384 string `json:"metadata_sha384"`
	// SkipSecureBoot is true only when the source VHD came from the
	// unsigned acldevel pipeline (SKIP_SECURE_BOOT=true in
	// .acl-base-image-vars.yaml), which cannot pass UEFI Secure Boot
	// signature verification. Absent/false means Secure Boot stays enforced.
	SkipSecureBoot bool `json:"skip_secure_boot"`
}

// loadCOSIPublishingInfo reads cosi-publishing-info.json from a downloaded
// pipeline artifact directory. artifactName is the pipeline artifact name
// (e.g., "cosi-publishing-info-acl-tl-gen2"). Returns ok=false if
// COSI_ARTIFACTS_DIR is not set or the artifact was not downloaded (variant
// not built for this run).
func loadCOSIPublishingInfo(t *testing.T, artifactName string) (cosiPublishingInfo, bool) {
	t.Helper()
	dir := os.Getenv("COSI_ARTIFACTS_DIR")
	if dir == "" {
		return cosiPublishingInfo{}, false
	}
	infoPath := filepath.Join(dir, artifactName, "cosi-publishing-info.json")
	data, err := os.ReadFile(infoPath)
	if os.IsNotExist(err) {
		return cosiPublishingInfo{}, false
	}
	require.NoError(t, err, "reading %s", infoPath)
	var info cosiPublishingInfo
	require.NoError(t, json.Unmarshal(data, &info), "parsing %s", infoPath)
	require.NotEmpty(t, info.CosiURL, "cosi_url is empty in %s", infoPath)
	require.NotEmpty(t, info.MetadataSHA384, "metadata_sha384 is empty in %s", infoPath)
	return info, true
}

// Test_ACL_COSI validates the contents of the AMD64 ACL COSI artifact
// (tar structure, metadata.json, and image SHA-384 hashes) without
// provisioning a VM.
func Test_ACL_COSI(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(t, cosiAMD64PublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

// Test_ACL_COSI_ARM64 validates the contents of the ARM64 ACL COSI artifact.
func Test_ACL_COSI_ARM64(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(t, cosiARM64PublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-arm64-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

// Test_ACL_COSI_FIPS validates the contents of the AMD64 FIPS ACL COSI artifact.
func Test_ACL_COSI_FIPS(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(t, cosiAMD64FIPSPublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-fips-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

// Test_ACL_COSI_ARM64_FIPS validates the contents of the ARM64 FIPS ACL COSI artifact.
func Test_ACL_COSI_ARM64_FIPS(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(t, cosiARM64FIPSPublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-arm64-fips-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

func Test_ACL_COSIUpdate_AMD64(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(t, cosiAMD64PublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-tl-gen2, skipping COSI update test")
	}
	require.NoError(t, validateCOSIUpdateInput(info.CosiURL, info.MetadataSHA384))

	RunScenario(t, &Scenario{
		Description: "Tests that an AMD64 ACL node remains Ready after a COSI A/B update",
		Location:    "westus3",
		Tags: Tags{
			COSIUpdate: true,
		},
		Config: Config{
			Cluster:                 ClusterKubenet,
			VHD:                     config.VHDACLGen2TL,
			SkipScriptlessNBCCSECmd: true,
			WaitForSSHAfterReboot:   5 * time.Minute,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if info.SkipSecureBoot {
					vmss.Properties = addTrustedLaunchNoSecureBootToVMSS(vmss.Properties)
				} else {
					vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				}
			},
			Validator: func(ctx context.Context, scenario *Scenario) error {
				return validateACLAMD64COSIUpdate(ctx, scenario, info.CosiURL, info.MetadataSHA384)
			},
		},
	})
}

func validateACLAMD64COSIUpdate(ctx context.Context, scenario *Scenario, rawCosiURL, rawMetadataHash string) error {
	cosiURL := strings.TrimSpace(rawCosiURL)
	metadataHash := strings.TrimSpace(rawMetadataHash)

	beforeBootID, err := runCOSICommand(ctx, scenario, "cat /proc/sys/kernel/random/boot_id")
	require.NoError(scenario.T, err)
	beforeBootID = strings.TrimSpace(beforeBootID)
	require.NotEmpty(scenario.T, beforeBootID)

	beforeNode, err := scenario.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, scenario.Runtime.VM.KubeName, metav1.GetOptions{})
	require.NoError(scenario.T, err)
	require.True(scenario.T, strings.EqualFold(beforeBootID, beforeNode.Status.NodeInfo.BootID), "host and Kubernetes boot IDs must match before the update")

	// Trident's Host Configuration expects the image SHA384 as a 96-character
	// lowercase hex string, but the COSI publishing artifact stores it base64
	// encoded (see convert-vhd-to-cosi.sh). Convert before embedding it below.
	metadataHashBytes, err := base64.StdEncoding.DecodeString(metadataHash)
	require.NoError(scenario.T, err)
	metadataHashHex := hex.EncodeToString(metadataHashBytes)

	// The node's network path to the COSI publishing endpoint is not always
	// reachable/supported, but the test runner's is (see Test_ACL_COSI).
	// Download the COSI file here, then stage it directly on the VM over the
	// existing SSH connection and reference it via a local file:// URL so
	// Trident never has to reach the original endpoint itself.
	imageURL, err := downloadAndStageCOSIFile(ctx, scenario, cosiURL)
	require.NoError(scenario.T, err)

	updateConfig := fmt.Sprintf("image:\n  url: %s\n  sha384: %s\ninternalParams:\n  forceAbUpdate: true\n  noTransition: true\n", strconv.Quote(imageURL), metadataHashHex)
	scenario.Logger.Logf("Trident update host configuration:\n%s", updateConfig)
	encodedConfig := base64.StdEncoding.EncodeToString([]byte(updateConfig))
	writeConfigCommand := fmt.Sprintf("printf '%%s' %s | base64 --decode > %s && chmod 0600 %s", shellQuote(encodedConfig), remoteCOSIConfigPath, remoteCOSIConfigPath)
	_, err = runCOSICommand(ctx, scenario, writeConfigCommand)
	require.NoError(scenario.T, err)

	version, err := runCOSICommand(ctx, scenario, "sudo trident --version")
	require.NoError(scenario.T, err)
	scenario.Logger.Logf("Trident version: %s", strings.TrimSpace(version))

	stageCommand := fmt.Sprintf("sudo trident update -v trace %s --allowed-operations=stage", remoteCOSIConfigPath)
	scenario.Logger.Logf("Trident update (stage) SSH command: %s", stageCommand)
	stageResult, stageErr := runSSHCommand(ctx, scenario.Runtime.VM.SSHClient, stageCommand, false)
	logTridentUpdateResult(scenario, "stage", stageResult, stageErr)

	finalizeCommand := fmt.Sprintf("sudo trident update -v trace %s --allowed-operations=finalize", remoteCOSIConfigPath)
	scenario.Logger.Logf("Trident update (finalize) SSH command: %s", finalizeCommand)
	finalizeResult, finalizeErr := runSSHCommand(ctx, scenario.Runtime.VM.SSHClient, finalizeCommand, false)
	logTridentUpdateResult(scenario, "finalize", finalizeResult, finalizeErr)

	require.NoError(scenario.T, RebootVMAndWaitForSSH(ctx, scenario))

	afterBootIDRaw, err := runCOSICommand(ctx, scenario, "cat /proc/sys/kernel/random/boot_id")
	require.NoError(scenario.T, err)
	afterBootID := strings.TrimSpace(afterBootIDRaw)
	require.NotEmpty(scenario.T, afterBootID)
	require.NotEqual(scenario.T, beforeBootID, afterBootID, "boot ID must change after the COSI update reboot")

	requireTridentStatus(ctx, scenario, "ab-update-finalized", "")

	execScriptOnVMForScenarioValidateExitCode(ctx, scenario, "sudo trident grpc-client commit -v trace", 0, "failed to commit the COSI update")
	requireTridentStatus(ctx, scenario, "provisioned", "volume-b")
	waitForSameNodeReadyAfterCOSIUpdate(ctx, scenario, beforeNode, afterBootID)
	postUpdatePod := podHTTPServerLinux(scenario)
	postUpdatePod.Name += "-cosi-post-update"
	ValidatePodRunning(ctx, scenario, postUpdatePod)
	return nil
}

// downloadAndStageCOSIFile downloads the COSI file at cosiURL onto the test
// runner (reusing the same download logic as Test_ACL_COSI), then copies it
// onto the scenario VM over the existing SSH connection. It returns a
// file:// URL that Trident can use to apply the update entirely from local
// disk, avoiding the VM having to reach the COSI publishing endpoint itself.
func downloadAndStageCOSIFile(ctx context.Context, scenario *Scenario, cosiURL string) (string, error) {
	localPath := filepath.Join(scenario.T.TempDir(), "update.cosi")
	scenario.Logger.Logf("downloading COSI to test runner: %s -> %s", sanitizeURL(cosiURL), localPath)
	if err := DownloadCOSIFile(ctx, cosiURL, localPath); err != nil {
		return "", fmt.Errorf("download COSI file to test runner: %w", err)
	}

	scpClient, err := scp.NewClientBySSH(scenario.Runtime.VM.SSHClient)
	if err != nil {
		return "", fmt.Errorf("create SCP client: %w", err)
	}
	defer scpClient.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open downloaded COSI file: %w", err)
	}
	defer localFile.Close()

	scenario.Logger.Logf("copying COSI file to VM: %s", remoteCOSIImagePath)
	if err := scpClient.CopyFromFile(ctx, *localFile, remoteCOSIImagePath, "0644"); err != nil {
		return "", fmt.Errorf("copy COSI file to VM: %w", err)
	}

	return "file://" + remoteCOSIImagePath, nil
}

func TestValidateCOSIUpdateInput(t *testing.T) {
	validHash := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xab}, sha512.Size384))
	require.NoError(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", validHash))
	require.ErrorContains(t, validateCOSIUpdateInput("http://download.example.com/acl.cosi", validHash), "HTTPS")
	require.ErrorContains(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", "abcd"), "48 bytes")
}

func TestResolveCOSISharedGalleryImageReference(t *testing.T) {
	imageReference, err := resolveImageReference(context.Background(), &config.Image{
		SharedGalleryImageID: aclCOSIAMD64ImageID,
		Version:              "would-trigger-gallery-lookup-without-shared-id",
	}, "westus2")

	require.NoError(t, err)
	require.Nil(t, imageReference.ID)
	require.Equal(t, aclCOSIAMD64ImageID, *imageReference.SharedGalleryImageID)
}

func TestCOSIUpdateTagFilter(t *testing.T) {
	matches, err := (Tags{COSIUpdate: true}).MatchesFilters("cosiupdate=true")
	require.NoError(t, err)
	require.True(t, matches)
}

func validateCOSIUpdateInput(rawURL, metadataSHA384 string) error {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("parse COSI update URL: %w", err)
	}
	if !strings.EqualFold(parsedURL.Scheme, "https") || parsedURL.Host == "" {
		return fmt.Errorf("COSI update URL must be an absolute HTTPS URL")
	}

	metadataHash, err := base64.StdEncoding.DecodeString(strings.TrimSpace(metadataSHA384))
	if err != nil {
		return fmt.Errorf("decode COSI metadata SHA-384: %w", err)
	}
	if len(metadataHash) != sha512.Size384 {
		return fmt.Errorf("COSI metadata SHA-384 must decode to %d bytes, got %d", sha512.Size384, len(metadataHash))
	}
	return nil
}

func runCOSICommand(ctx context.Context, scenario *Scenario, command string) (string, error) {
	result, err := runSSHCommand(ctx, scenario.Runtime.VM.SSHClient, command, false)
	if err != nil {
		return "", err
	}
	if result.exitCode != "0" {
		return "", fmt.Errorf("command failed with exit code %s: %s", result.exitCode, result.stderr)
	}
	return result.stdout, nil
}

func logTridentUpdateResult(scenario *Scenario, step string, result *podExecResult, err error) {
	if err != nil {
		scenario.Logger.Logf("Trident update (%s) SSH command error: %v", step, err)
		return
	}
	if result == nil {
		return
	}
	scenario.Logger.Logf("Trident update (%s) exited with code %s", step, result.exitCode)
	scenario.Logger.Logf("Trident update (%s) stdout: %s", step, result.stdout)
	scenario.Logger.Logf("Trident update (%s) stderr: %s", step, result.stderr)
}

func requireTridentStatus(ctx context.Context, scenario *Scenario, servicingState, activeVolume string) {
	status, err := runCOSICommand(ctx, scenario, "sudo trident get status")
	require.NoError(scenario.T, err)
	require.Contains(scenario.T, status, "servicingState: "+servicingState)
	if activeVolume != "" {
		require.Contains(scenario.T, status, "abActiveVolume: "+activeVolume)
	}
}

func waitForSameNodeReadyAfterCOSIUpdate(ctx context.Context, scenario *Scenario, beforeNode *corev1.Node, hostBootID string) {
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(pollCtx context.Context) (bool, error) {
		node, err := scenario.Runtime.Kube.Typed.CoreV1().Nodes().Get(pollCtx, beforeNode.Name, metav1.GetOptions{})
		if err != nil {
			scenario.Logger.Logf("waiting for node %s after COSI update: %v", beforeNode.Name, err)
			return false, nil
		}
		if node.UID != beforeNode.UID {
			return false, fmt.Errorf("node %s was replaced during the COSI update", beforeNode.Name)
		}
		if !strings.EqualFold(node.Status.NodeInfo.BootID, hostBootID) {
			return false, nil
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	require.NoError(scenario.T, err, "same Kubernetes node did not return Ready after the COSI update")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
