package e2e

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
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
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	aclCOSIAMD64ImageVersion    = "0.20260827.1192019"
	aclCOSIAMD64ImageID         = "/SharedGalleries/035db282-f1c8-4ce7-b78f-2a7265d5398c-ACLDEVEL/Images/acldevel/Versions/0.20260827.1192019"
	remoteCOSIConfigPath        = "/home/azureuser/update-config.yaml"
	cosiAMD64PublishingArtifact = "cosi-publishing-info-acl-tl-gen2"
)

// cosiPublishingInfo mirrors the JSON written by convert-vhd-to-cosi.sh.
type cosiPublishingInfo struct {
	CosiURL        string `json:"cosi_url"`
	MetadataSHA384 string `json:"metadata_sha384"`
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
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
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
	requireTridentStatus(ctx, scenario, "provisioned", "volume-a")

	preflightCommand := fmt.Sprintf("curl --fail --location --silent --show-error --range 0-511 --output /dev/null %s", shellQuote(cosiURL))
	_, err = runCOSICommand(ctx, scenario, preflightCommand)
	require.NoError(scenario.T, err, "COSI URL is not reachable from the node")

	updateConfig := fmt.Sprintf("image:\n  url: %s\n  sha384: %s\ninternalParams:\n  forceAbUpdate: true\n", strconv.Quote(cosiURL), metadataHash)
	encodedConfig := base64.StdEncoding.EncodeToString([]byte(updateConfig))
	writeConfigCommand := fmt.Sprintf("printf '%%s' %s | base64 --decode > %s && chmod 0600 %s", shellQuote(encodedConfig), remoteCOSIConfigPath, remoteCOSIConfigPath)
	_, err = runCOSICommand(ctx, scenario, writeConfigCommand)
	require.NoError(scenario.T, err)

	updateResult, updateErr := runSSHCommand(ctx, scenario.Runtime.VM.SSHClient, "sudo trident update -v trace "+remoteCOSIConfigPath, false)
	if updateErr != nil {
		scenario.Logger.Logf("Trident update disconnected SSH for reboot: %v", updateErr)
	} else if updateResult != nil {
		scenario.Logger.Logf("Trident update SSH command exited with code %s", updateResult.exitCode)
	}

	afterBootID, err := reconnectAfterCOSIReboot(ctx, scenario, beforeBootID)
	require.NoError(scenario.T, err)
	requireTridentStatus(ctx, scenario, "ab-update-finalized", "")

	execScriptOnVMForScenarioValidateExitCode(ctx, scenario, "sudo trident grpc-client commit -v trace", 0, "failed to commit the COSI update")
	requireTridentStatus(ctx, scenario, "provisioned", "volume-b")
	waitForSameNodeReadyAfterCOSIUpdate(ctx, scenario, beforeNode, afterBootID)
	postUpdatePod := podHTTPServerLinux(scenario)
	postUpdatePod.Name += "-cosi-post-update"
	ValidatePodRunning(ctx, scenario, postUpdatePod)
	return nil
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

func requireTridentStatus(ctx context.Context, scenario *Scenario, servicingState, activeVolume string) {
	status, err := runCOSICommand(ctx, scenario, "sudo trident get status")
	require.NoError(scenario.T, err)
	require.Contains(scenario.T, status, "servicingState: "+servicingState)
	if activeVolume != "" {
		require.Contains(scenario.T, status, "abActiveVolume: "+activeVolume)
	}
}

func reconnectAfterCOSIReboot(ctx context.Context, scenario *Scenario, beforeBootID string) (string, error) {
	cleanupBastionTunnel(scenario.Runtime.VM.SSHClient)
	scenario.Runtime.VM.SSHClient = nil

	var afterBootID string
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(pollCtx context.Context) (bool, error) {
		client, err := DialSSHOverBastion(pollCtx, scenario.Runtime.Cluster.Bastion, scenario.Runtime.VM.PrivateIP, config.VMSSHPrivateKey)
		if err != nil {
			scenario.Logger.Logf("waiting for SSH after COSI reboot: %v", err)
			return false, nil
		}

		result, err := runSSHCommand(pollCtx, client, "cat /proc/sys/kernel/random/boot_id", false)
		if err != nil || result.exitCode != "0" {
			cleanupBastionTunnel(client)
			return false, nil
		}

		afterBootID = strings.TrimSpace(result.stdout)
		if afterBootID == "" || strings.EqualFold(afterBootID, beforeBootID) {
			cleanupBastionTunnel(client)
			return false, nil
		}

		scenario.Runtime.VM.SSHClient = client
		scenario.T.Cleanup(func() {
			cleanupBastionTunnel(client)
		})
		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("wait for node SSH with a new boot ID: %w", err)
	}
	return afterBootID, nil
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
