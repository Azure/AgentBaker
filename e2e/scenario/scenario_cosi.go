package scenario

import (
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
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	remoteCOSIConfigPath            = "/home/azureuser/update-config.yaml"
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
// not built for this run). This runs at package init time (via Register),
// so a malformed artifact -- as opposed to a missing one -- is a hard
// failure and panics, matching the invariant-violation convention used
// elsewhere in this package (see registry.go).
func loadCOSIPublishingInfo(artifactName string) (cosiPublishingInfo, bool) {
	dir := os.Getenv("COSI_ARTIFACTS_DIR")
	if dir == "" {
		return cosiPublishingInfo{}, false
	}
	infoPath := filepath.Join(dir, artifactName, "cosi-publishing-info.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cosiPublishingInfo{}, false
		}
		panic(fmt.Sprintf("reading %s: %v", infoPath, err))
	}
	var info cosiPublishingInfo
	if err := json.Unmarshal(data, &info); err != nil {
		panic(fmt.Sprintf("parsing %s: %v", infoPath, err))
	}
	if info.CosiURL == "" {
		panic(fmt.Sprintf("cosi_url is empty in %s", infoPath))
	}
	if info.MetadataSHA384 == "" {
		panic(fmt.Sprintf("metadata_sha384 is empty in %s", infoPath))
	}
	return info, true
}

var _ = Register(cosiUpdateAMD64Scenario())

// cosiUpdateAMD64Scenario builds the ACL_COSIUpdate_AMD64 scenario. If the
// COSI publishing artifact isn't available (or its update inputs are
// invalid), the scenario is still registered but with a SkipReason, so
// registry listings and validation still see it -- it just doesn't run.
func cosiUpdateAMD64Scenario() *Scenario {
	const name = "ACL_COSIUpdate_AMD64"
	const description = "Tests that an AMD64 ACL node remains Ready after a COSI A/B update"

	info, ok := loadCOSIPublishingInfo(cosiAMD64PublishingArtifact)
	if !ok {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  "COSI artifact not available for acl-tl-gen2, skipping COSI update test",
		}
	}
	if err := validateCOSIUpdateInput(info.CosiURL, info.MetadataSHA384); err != nil {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  fmt.Sprintf("invalid COSI update input: %v", err),
		}
	}

	return &Scenario{
		Name:        name,
		Description: description,
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
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, info.SkipSecureBoot)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return validateACLAMD64COSIUpdate(ctx, s, info.CosiURL, info.MetadataSHA384)
			},
		},
	}
}

func validateACLAMD64COSIUpdate(ctx context.Context, s *Scenario, rawCosiURL, rawMetadataHash string) error {
	cosiURL := strings.TrimSpace(rawCosiURL)
	metadataHash := strings.TrimSpace(rawMetadataHash)

	beforeBootID, err := runCOSICommand(ctx, s, "cat /proc/sys/kernel/random/boot_id")
	if err != nil {
		return fmt.Errorf("get boot ID before COSI update: %w", err)
	}
	beforeBootID = strings.TrimSpace(beforeBootID)
	if beforeBootID == "" {
		return fmt.Errorf("boot ID before COSI update is empty")
	}

	beforeNode, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node before COSI update: %w", err)
	}
	if !strings.EqualFold(beforeBootID, beforeNode.Status.NodeInfo.BootID) {
		return fmt.Errorf("host and Kubernetes boot IDs must match before the update: host=%s, k8s=%s", beforeBootID, beforeNode.Status.NodeInfo.BootID)
	}

	// Trident's Host Configuration expects the image SHA384 as a 96-character
	// lowercase hex string, but the COSI publishing artifact stores it base64
	// encoded (see convert-vhd-to-cosi.sh). Convert before embedding it below.
	metadataHashBytes, err := base64.StdEncoding.DecodeString(metadataHash)
	if err != nil {
		return fmt.Errorf("decode COSI metadata SHA-384: %w", err)
	}
	metadataHashHex := hex.EncodeToString(metadataHashBytes)

	updateConfig := fmt.Sprintf("image:\n  url: %s\n  sha384: %s\ninternalParams:\n  forceAbUpdate: true\n  noTransition: true\n", strconv.Quote(cosiURL), metadataHashHex)
	logging.Logf(ctx, "Trident update host configuration:\n%s", updateConfig)
	encodedConfig := base64.StdEncoding.EncodeToString([]byte(updateConfig))
	writeConfigCommand := fmt.Sprintf("printf '%%s' %s | base64 --decode > %s && chmod 0600 %s", shellQuote(encodedConfig), remoteCOSIConfigPath, remoteCOSIConfigPath)
	if _, err := runCOSICommand(ctx, s, writeConfigCommand); err != nil {
		return fmt.Errorf("write Trident update host configuration: %w", err)
	}

	version, err := runCOSICommand(ctx, s, "sudo trident --version")
	if err != nil {
		return fmt.Errorf("get trident version: %w", err)
	}
	logging.Logf(ctx, "Trident version: %s", strings.TrimSpace(version))

	stageCommand := fmt.Sprintf("sudo trident update -v trace %s --allowed-operations=stage", remoteCOSIConfigPath)
	logging.Logf(ctx, "Trident update (stage) SSH command: %s", stageCommand)
	stageResult, stageErr := runSSHCommand(ctx, s.Runtime.VM.SSHClient, stageCommand, false)
	logTridentUpdateResult(ctx, "stage", stageResult, stageErr)

	finalizeCommand := fmt.Sprintf("sudo trident update -v trace %s --allowed-operations=finalize", remoteCOSIConfigPath)
	logging.Logf(ctx, "Trident update (finalize) SSH command: %s", finalizeCommand)
	finalizeResult, finalizeErr := runSSHCommand(ctx, s.Runtime.VM.SSHClient, finalizeCommand, false)
	logTridentUpdateResult(ctx, "finalize", finalizeResult, finalizeErr)

	if err := RebootVMAndWaitForSSH(ctx, s); err != nil {
		return fmt.Errorf("reboot VM after COSI update: %w", err)
	}

	afterBootIDRaw, err := runCOSICommand(ctx, s, "cat /proc/sys/kernel/random/boot_id")
	if err != nil {
		return fmt.Errorf("get boot ID after COSI update: %w", err)
	}
	afterBootID := strings.TrimSpace(afterBootIDRaw)
	if afterBootID == "" {
		return fmt.Errorf("boot ID after COSI update is empty")
	}
	if beforeBootID == afterBootID {
		return fmt.Errorf("boot ID must change after the COSI update reboot")
	}

	if err := requireTridentStatus(ctx, s, "ab-update-finalized", ""); err != nil {
		return err
	}

	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo trident grpc-client commit -v trace", 0, "failed to commit the COSI update"); err != nil {
		return err
	}
	if err := requireTridentStatus(ctx, s, "provisioned", "volume-b"); err != nil {
		return err
	}
	if err := waitForSameNodeReadyAfterCOSIUpdate(ctx, s, beforeNode, afterBootID); err != nil {
		return err
	}
	postUpdatePod := podHTTPServerLinux(s)
	postUpdatePod.Name += "-cosi-post-update"
	if err := ValidatePodRunning(ctx, s, postUpdatePod); err != nil {
		return err
	}
	return nil
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

func runCOSICommand(ctx context.Context, s *Scenario, command string) (string, error) {
	result, err := runSSHCommand(ctx, s.Runtime.VM.SSHClient, command, false)
	if err != nil {
		return "", err
	}
	if result.exitCode != "0" {
		return "", fmt.Errorf("command failed with exit code %s: %s", result.exitCode, result.stderr)
	}
	return result.stdout, nil
}

func logTridentUpdateResult(ctx context.Context, step string, result *podExecResult, err error) {
	if err != nil {
		logging.Logf(ctx, "Trident update (%s) SSH command error: %v", step, err)
		return
	}
	if result == nil {
		return
	}
	logging.Logf(ctx, "Trident update (%s) exited with code %s", step, result.exitCode)
	logging.Logf(ctx, "Trident update (%s) stdout: %s", step, result.stdout)
	logging.Logf(ctx, "Trident update (%s) stderr: %s", step, result.stderr)
}

func requireTridentStatus(ctx context.Context, s *Scenario, servicingState, activeVolume string) error {
	status, err := runCOSICommand(ctx, s, "sudo trident get status")
	if err != nil {
		return fmt.Errorf("get trident status: %w", err)
	}
	if !strings.Contains(status, "servicingState: "+servicingState) {
		return fmt.Errorf("expected trident status to contain servicingState: %s, got: %s", servicingState, status)
	}
	if activeVolume != "" && !strings.Contains(status, "abActiveVolume: "+activeVolume) {
		return fmt.Errorf("expected trident status to contain abActiveVolume: %s, got: %s", activeVolume, status)
	}
	return nil
}

func waitForSameNodeReadyAfterCOSIUpdate(ctx context.Context, s *Scenario, beforeNode *corev1.Node, hostBootID string) error {
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(pollCtx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(pollCtx, beforeNode.Name, metav1.GetOptions{})
		if err != nil {
			logging.Logf(pollCtx, "waiting for node %s after COSI update: %v", beforeNode.Name, err)
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
	if err != nil {
		return fmt.Errorf("same Kubernetes node did not return Ready after the COSI update: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
