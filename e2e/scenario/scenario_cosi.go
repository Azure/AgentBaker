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
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Masterminds/semver/v3"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	remoteCOSIConfigPath            = "/home/azureuser/update-config.yaml"
	cosiAMD64PublishingArtifact     = "cosi-publishing-info-acl-tl-gen2"
	cosiARM64PublishingArtifact     = "cosi-publishing-info-acl-arm64-tl-gen2"
	cosiAMD64FIPSPublishingArtifact = "cosi-publishing-info-acl-fips-tl-gen2"
	cosiARM64FIPSPublishingArtifact = "cosi-publishing-info-acl-arm64-fips-tl-gen2"

	aclCOSIAMD64BaselineVHDVersion   = "1.1790005343.11420"
	aclCOSIAMD64BaselineImageVersion = "202609.21.0"

	aclUpdateRequestAnnotationKey      = "acl.azure.com/update-request"
	aclUpdateStatusAnnotationKey       = "acl.azure.com/update-status"
	aclUpdateCommitStatusAnnotationKey = "acl.azure.com/update-commit-status"
	aclUpdateSchemaVersion             = "1.0"
	aclUpdateOperationStage            = "stage"
	aclUpdateOperationFinalize         = "finalize"
	aclUpdateOperationCommit           = "commit"
	aclUpdateCodeInProgress            = "InProgress"
	aclUpdateCodeSuccess               = "Success"
	aclUpdateStatusPollInterval        = 10 * time.Second
	aclCOSIStageTimeout                = 20 * time.Minute
	aclCOSIFinalizeTimeout             = 10 * time.Minute
	aclCOSICommitTimeout               = 15 * time.Minute
)

// cosiPublishingInfo mirrors the JSON written by convert-vhd-to-cosi.sh.
type cosiPublishingInfo struct {
	CosiURL        string `json:"cosi_url"`
	ImageVersion   string `json:"image_version"`
	MetadataSHA384 string `json:"metadata_sha384"`
}

type aclUpdateRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	NodeUpdateID  string `json:"nodeUpdateId"`
	OperationID   string `json:"operationId"`
	Operation     string `json:"operation"`
	TargetVersion string `json:"targetVersion"`
	Server        string `json:"server"`
	AppID         string `json:"appId"`
	Track         string `json:"track"`
}

type aclUpdateStatus struct {
	SchemaVersion string `json:"schemaVersion"`
	NodeUpdateID  string `json:"nodeUpdateId"`
	OperationID   string `json:"operationId"`
	Operation     string `json:"operation"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	FromVersion   string `json:"fromVersion"`
	ToVersion     string `json:"toVersion"`
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
	if info.ImageVersion == "" {
		panic(fmt.Sprintf("image_version is empty in %s", infoPath))
	}
	if info.MetadataSHA384 == "" {
		panic(fmt.Sprintf("metadata_sha384 is empty in %s", infoPath))
	}
	return info, true
}

var _ = Register(cosiUpdateAMD64Scenario())
var _ = Register(cosiUpdateARM64Scenario())
var _ = Register(cosiUpdateAMD64FIPSScenario())
var _ = Register(cosiUpdateARM64FIPSScenario())

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
	nebraskaServer := strings.TrimSpace(os.Getenv("COSI_NEBRASKA_SERVER"))
	nebraskaAppID := strings.TrimSpace(os.Getenv("COSI_NEBRASKA_APP_ID"))
	if err := validateACLAnnotationUpdateInput(info.ImageVersion, nebraskaServer, nebraskaAppID); err != nil {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  fmt.Sprintf("invalid COSI update input: %v", err),
		}
	}
	baselineVHD := *config.VHDACLGen2TL
	baselineVHD.Version = aclCOSIAMD64BaselineVHDVersion

	return &Scenario{
		Name:        name,
		Description: description,
		Location:    "westus3",
		Tags: Tags{
			COSIUpdate: true,
		},
		Config: Config{
			Cluster:                 ClusterKubenet,
			VHD:                     &baselineVHD,
			SkipScriptlessNBCCSECmd: true,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, config.Config.ACLBaseImageSigned)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return validateACLAnnotationCOSIUpdate(ctx, s, info.ImageVersion, nebraskaServer, nebraskaAppID)
			},
		},
	}
}

func validateACLAnnotationCOSIUpdate(ctx context.Context, s *Scenario, rawTargetVersion, rawNebraskaServer, rawNebraskaAppID string) error {
	targetVersion := strings.TrimSpace(rawTargetVersion)
	nebraskaServer := strings.TrimSpace(rawNebraskaServer)
	nebraskaAppID := strings.TrimSpace(rawNebraskaAppID)

	currentVersion, err := requireACLUpdateAgent(ctx, s)
	if err != nil {
		return err
	}
	if currentVersion != aclCOSIAMD64BaselineImageVersion {
		return fmt.Errorf("frozen ACL COSI baseline reports image version %q, want %q", currentVersion, aclCOSIAMD64BaselineImageVersion)
	}
	if err := validateACLTargetVersion(currentVersion, targetVersion); err != nil {
		return err
	}

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

	nodes := s.Runtime.Kube.Typed.CoreV1().Nodes()
	request := aclUpdateRequest{
		SchemaVersion: aclUpdateSchemaVersion,
		NodeUpdateID:  uuid.NewString(),
		OperationID:   uuid.NewString(),
		Operation:     aclUpdateOperationStage,
		TargetVersion: targetVersion,
		Server:        nebraskaServer,
		AppID:         nebraskaAppID,
		Track:         "pin-" + targetVersion,
	}

	logging.Logf(ctx, "Requesting ACL COSI stage on node %s (operationId=%s targetVersion=%s server=%s track=%s)", beforeNode.Name, request.OperationID, targetVersion, nebraskaServer, request.Track)
	if err := patchACLUpdateRequest(ctx, nodes, beforeNode.Name, request); err != nil {
		return err
	}
	stageStatus, err := waitForACLUpdateStatus(ctx, nodes, beforeNode.Name, aclUpdateStatusAnnotationKey, request, aclCOSIStageTimeout)
	if err != nil {
		return err
	}
	if stageStatus.Code != aclUpdateCodeSuccess {
		return fmt.Errorf("ACL COSI stage reported %s: %s", stageStatus.Code, stageStatus.Message)
	}

	request.OperationID = uuid.NewString()
	request.Operation = aclUpdateOperationFinalize
	logging.Logf(ctx, "Requesting ACL COSI finalize on node %s (operationId=%s)", beforeNode.Name, request.OperationID)
	if err := patchACLUpdateRequest(ctx, nodes, beforeNode.Name, request); err != nil {
		return err
	}
	finalizeStatus, err := waitForACLUpdateStatus(ctx, nodes, beforeNode.Name, aclUpdateStatusAnnotationKey, request, aclCOSIFinalizeTimeout)
	if err != nil {
		return err
	}
	if finalizeStatus.Code != aclUpdateCodeSuccess {
		return fmt.Errorf("ACL COSI finalize reported %s: %s", finalizeStatus.Code, finalizeStatus.Message)
	}

	commitRequest := request
	commitRequest.Operation = aclUpdateOperationCommit
	commitStatus, err := waitForACLUpdateStatus(ctx, nodes, beforeNode.Name, aclUpdateCommitStatusAnnotationKey, commitRequest, aclCOSICommitTimeout)
	if err != nil {
		return err
	}
	if commitStatus.Code != aclUpdateCodeSuccess {
		return fmt.Errorf("ACL COSI commit reported %s: %s", commitStatus.Code, commitStatus.Message)
	}
	if commitStatus.FromVersion != currentVersion {
		return fmt.Errorf("ACL COSI commit reported fromVersion %q, want %q", commitStatus.FromVersion, currentVersion)
	}
	if commitStatus.ToVersion != targetVersion {
		return fmt.Errorf("ACL COSI commit reported toVersion %q, want %q", commitStatus.ToVersion, targetVersion)
	}

	if err := waitForSameNodeReadyAfterACLAnnotationUpdate(ctx, s, beforeNode, beforeBootID); err != nil {
		return err
	}
	if err := waitForSSHAfterReboot(ctx, s, beforeBootID); err != nil {
		return fmt.Errorf("reconnect SSH after ACL annotation update: %w", err)
	}
	if err := requireTridentStatus(ctx, s, "provisioned", "volume-b"); err != nil {
		return fmt.Errorf("verify Trident status after ACL annotation update: %w", err)
	}
	logging.Logf(ctx, "Verified ACL COSI update: fromVersion=%s toVersion=%s servicingState=provisioned activeVolume=volume-b", commitStatus.FromVersion, commitStatus.ToVersion)
	postUpdatePod := podHTTPServerLinux(s)
	postUpdatePod.Name += "-cosi-post-update"
	return ValidatePodRunning(ctx, s, postUpdatePod)
}

func validateACLAnnotationUpdateInput(targetVersion, nebraskaServer, nebraskaAppID string) error {
	if strings.TrimSpace(targetVersion) == "" {
		return fmt.Errorf("COSI publishing info image_version is empty")
	}
	if strings.TrimSpace(nebraskaAppID) == "" {
		return fmt.Errorf("COSI Nebraska app ID is empty")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(nebraskaServer))
	if err != nil {
		return fmt.Errorf("parse COSI Nebraska server: %w", err)
	}
	if !strings.EqualFold(parsedURL.Scheme, "https") || parsedURL.Host == "" {
		return fmt.Errorf("COSI Nebraska server must be an absolute HTTPS URL")
	}
	if parsedURL.Path == "" || parsedURL.Path == "/" {
		return fmt.Errorf("COSI Nebraska server must include the Omaha path")
	}
	return nil
}

func validateACLTargetVersion(currentVersion, targetVersion string) error {
	currentSemver, err := semver.StrictNewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("parse ACL baseline image version %q: %w", currentVersion, err)
	}
	targetSemver, err := semver.StrictNewVersion(targetVersion)
	if err != nil {
		return fmt.Errorf("parse COSI target image version %q: %w", targetVersion, err)
	}
	if !targetSemver.GreaterThan(currentSemver) {
		return fmt.Errorf("COSI annotation target %q must be newer than baseline %q", targetVersion, currentVersion)
	}
	return nil
}

func requireACLUpdateAgent(ctx context.Context, s *Scenario) (string, error) {
	preflight := "sudo systemctl is-active --quiet trident-acl-agent.path && sudo systemctl is-active --quiet trident-acl-agent.service && sudo test -S /run/trident/trident.sock && sudo test -f /opt/azure/containers/image-version"
	if _, err := runCOSICommand(ctx, s, preflight); err != nil {
		return "", fmt.Errorf("Trident ACL Agent precondition failed: %w", err)
	}
	rawVersion, err := runCOSICommand(ctx, s, "sudo cat /opt/azure/containers/image-version")
	if err != nil {
		return "", fmt.Errorf("read ACL node image version: %w", err)
	}
	for _, line := range strings.Split(rawVersion, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "IMAGE_VERSION" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", fmt.Errorf("ACL node image version file has no IMAGE_VERSION")
}

func patchACLUpdateRequest(ctx context.Context, nodes corev1client.NodeInterface, nodeName string, request aclUpdateRequest) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal ACL update request: %w", err)
	}
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				aclUpdateRequestAnnotationKey: string(payload),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal ACL update request patch: %w", err)
	}
	if _, err := nodes.Patch(ctx, nodeName, k8stypes.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patch ACL update request on node %s: %w", nodeName, err)
	}
	return nil
}

func waitForACLUpdateStatus(ctx context.Context, nodes corev1client.NodeInterface, nodeName, annotationKey string, expected aclUpdateRequest, timeout time.Duration) (*aclUpdateStatus, error) {
	var terminalStatus *aclUpdateStatus
	lastLoggedCode := ""
	err := wait.PollUntilContextTimeout(ctx, aclUpdateStatusPollInterval, timeout, true, func(pollCtx context.Context) (bool, error) {
		node, err := nodes.Get(pollCtx, nodeName, metav1.GetOptions{})
		if err != nil {
			logging.Logf(pollCtx, "Waiting for ACL update %s status on node %s: %v", expected.OperationID, nodeName, err)
			return false, nil
		}
		status, terminal, err := matchACLUpdateStatus(node.Annotations[annotationKey], expected)
		if err != nil {
			return false, err
		}
		if status != nil && status.Code != lastLoggedCode {
			logging.Logf(pollCtx, "ACL update %s %s on node %s reported %s: %s", status.OperationID, status.Operation, nodeName, status.Code, status.Message)
			lastLoggedCode = status.Code
		}
		if terminal {
			terminalStatus = status
		}
		return terminal, nil
	})
	if err != nil {
		return nil, fmt.Errorf("wait for ACL update %s %s status on node %s: %w", expected.OperationID, expected.Operation, nodeName, err)
	}
	return terminalStatus, nil
}

func matchACLUpdateStatus(raw string, expected aclUpdateRequest) (*aclUpdateStatus, bool, error) {
	if raw == "" {
		return nil, false, nil
	}
	status := &aclUpdateStatus{}
	if err := json.Unmarshal([]byte(raw), status); err != nil {
		return nil, false, fmt.Errorf("parse ACL update %s status: %w", expected.OperationID, err)
	}
	if status.SchemaVersion != aclUpdateSchemaVersion {
		return nil, false, fmt.Errorf("ACL update %s status schemaVersion is %q, want %q", expected.OperationID, status.SchemaVersion, aclUpdateSchemaVersion)
	}
	if status.OperationID != expected.OperationID {
		return nil, false, nil
	}
	if status.NodeUpdateID != expected.NodeUpdateID {
		return nil, false, fmt.Errorf("ACL update %s status nodeUpdateId is %q, want %q", expected.OperationID, status.NodeUpdateID, expected.NodeUpdateID)
	}
	if status.Operation != expected.Operation {
		return nil, false, fmt.Errorf("ACL update %s status operation is %q, want %q", expected.OperationID, status.Operation, expected.Operation)
	}
	return status, status.Code != "" && status.Code != aclUpdateCodeInProgress, nil
}

func waitForSameNodeReadyAfterACLAnnotationUpdate(ctx context.Context, s *Scenario, beforeNode *corev1.Node, beforeBootID string) error {
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(pollCtx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(pollCtx, beforeNode.Name, metav1.GetOptions{})
		if err != nil {
			logging.Logf(pollCtx, "waiting for node %s after COSI update: %v", beforeNode.Name, err)
			return false, nil
		}
		if node.UID != beforeNode.UID {
			return false, fmt.Errorf("node %s was replaced during the COSI update", beforeNode.Name)
		}
		if node.Status.NodeInfo.BootID == "" || strings.EqualFold(node.Status.NodeInfo.BootID, beforeBootID) {
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

// cosiUpdateARM64Scenario builds the ACL_COSIUpdate_ARM64 scenario. Same
// shape as cosiUpdateAMD64Scenario, but on ARM64 (Cobalt 100 / Standard_D2pds_v6,
// the only currently-supported ARM64 SKU for ACL -- see ACL_ARM64 in scenario.go).
func cosiUpdateARM64Scenario() *Scenario {
	const name = "ACL_COSIUpdate_ARM64"
	const description = "Tests that an ARM64 ACL node remains Ready after a COSI A/B update"

	info, ok := loadCOSIPublishingInfo(cosiARM64PublishingArtifact)
	if !ok {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  "COSI artifact not available for acl-arm64-tl-gen2, skipping COSI update test",
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
			Cluster: ClusterKubenet,
			VHD:     config.VHDACLArm64Gen2TL,
			// v6 (Cobalt 100) only supports NVMe disk controllers, not ResourceDisk
			UseNVMe:                 true,
			SkipScriptlessNBCCSECmd: true,
			WaitForSSHAfterReboot:   5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				// Ampere Altra (v5) doesn't support TrustedLaunch; Cobalt 100 (v6) does
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v6"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, config.Config.ACLBaseImageSigned)
				vmss.SKU.Name = to.Ptr("Standard_D2pds_v6")
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return validateACLCOSIUpdate(ctx, s, info.CosiURL, info.MetadataSHA384)
			},
		},
	}
}

// cosiUpdateAMD64FIPSScenario builds the ACL_COSIUpdate_AMD64_FIPS scenario.
// Same shape as cosiUpdateAMD64Scenario, but on the FIPS VHD (LocalDNS isn't
// currently supported on FIPS-enabled VHDs; mirrors ACLGen2FIPSTL in scenario.go).
func cosiUpdateAMD64FIPSScenario() *Scenario {
	const name = "ACL_COSIUpdate_AMD64_FIPS"
	const description = "Tests that an AMD64 FIPS ACL node remains Ready after a COSI A/B update"

	info, ok := loadCOSIPublishingInfo(cosiAMD64FIPSPublishingArtifact)
	if !ok {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  "COSI artifact not available for acl-fips-tl-gen2, skipping COSI update test",
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
			VHD:                     config.VHDACLGen2FIPSTL,
			SkipScriptlessNBCCSECmd: true,
			WaitForSSHAfterReboot:   5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				// LocalDNS isn't currently supported on FIPS-enabled VHDs; mirror ACLGen2FIPSTL.
				nbc.AgentPoolProfile.LocalDNSProfile = nil
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, config.Config.ACLBaseImageSigned)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return validateACLCOSIUpdate(ctx, s, info.CosiURL, info.MetadataSHA384)
			},
		},
	}
}

// cosiUpdateARM64FIPSScenario builds the ACL_COSIUpdate_ARM64_FIPS scenario,
// combining the ARM64 (Cobalt 100 / Standard_D2pds_v6) and FIPS (no LocalDNS)
// requirements of cosiUpdateARM64Scenario and cosiUpdateAMD64FIPSScenario.
func cosiUpdateARM64FIPSScenario() *Scenario {
	const name = "ACL_COSIUpdate_ARM64_FIPS"
	const description = "Tests that an ARM64 FIPS ACL node remains Ready after a COSI A/B update"

	info, ok := loadCOSIPublishingInfo(cosiARM64FIPSPublishingArtifact)
	if !ok {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  "COSI artifact not available for acl-arm64-fips-tl-gen2, skipping COSI update test",
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
			Cluster: ClusterKubenet,
			VHD:     config.VHDACLArm64Gen2FIPSTL,
			// v6 (Cobalt 100) only supports NVMe disk controllers, not ResourceDisk
			UseNVMe:                 true,
			SkipScriptlessNBCCSECmd: true,
			WaitForSSHAfterReboot:   5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				// Ampere Altra (v5) doesn't support TrustedLaunch; Cobalt 100 (v6) does
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v6"
				nbc.IsARM64 = true
				// LocalDNS isn't currently supported on FIPS-enabled VHDs; mirror ACLGen2FIPSTL.
				nbc.AgentPoolProfile.LocalDNSProfile = nil
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, config.Config.ACLBaseImageSigned)
				vmss.SKU.Name = to.Ptr("Standard_D2pds_v6")
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return validateACLCOSIUpdate(ctx, s, info.CosiURL, info.MetadataSHA384)
			},
		},
	}
}

func validateACLCOSIUpdate(ctx context.Context, s *Scenario, rawCosiURL, rawMetadataHash string) error {
	cosiURL := strings.TrimSpace(rawCosiURL)
	metadataHash := strings.TrimSpace(rawMetadataHash)

	// Diagnostic-only: capture trident-acl-agent's systemd status and
	// relevant files before and after the update, to help root-cause why it
	// enters a failed state after a COSI update. The test never disables or
	// masks the agent -- it drives Trident directly via CLI/gRPC (stage,
	// finalize, commit below) alongside whatever the agent does on its own.
	logACLAgentDiagnostics(ctx, s, "before COSI update")
	defer logACLAgentDiagnostics(ctx, s, "after COSI update")

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

// logACLAgentDiagnostics runs a fixed set of read-only diagnostic commands --
// systemd status, the unit's full journal history (not just the short tail
// systemctl status shows), and relevant config files -- and logs their
// output, ignoring any errors or non-zero exit codes. It never fails the
// test; it exists purely to help root-cause why trident-acl-agent enters a
// failed state after a COSI update.
func logACLAgentDiagnostics(ctx context.Context, s *Scenario, label string) {
	commands := []struct {
		desc string
		cmd  string
	}{
		{"systemctl status trident-acl-agent.path/.service", "sudo systemctl status trident-acl-agent.path trident-acl-agent.service"},
		{"journalctl -u trident-acl-agent.path/.service (all available logs)", "sudo journalctl -u trident-acl-agent.path -u trident-acl-agent.service --no-pager"},
		{"/var/lib/kubelet/kubeconfig", "sudo cat /var/lib/kubelet/kubeconfig"},
		{"/opt/azure/containers/image-version", "sudo cat /opt/azure/containers/image-version"},
	}
	for _, c := range commands {
		result, err := runSSHCommand(ctx, s.Runtime.VM.SSHClient, c.cmd, false)
		if err != nil {
			logging.Logf(ctx, "[acl-agent-diagnostics %s] %s: SSH error: %v", label, c.desc, err)
			continue
		}
		logging.Logf(ctx, "[acl-agent-diagnostics %s] %s: exit=%s\nstdout:\n%s\nstderr:\n%s", label, c.desc, result.exitCode, result.stdout, result.stderr)
	}
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
