package scenario

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed validate_acl_ipe.sh
var aclIPEAuditScript string

//go:embed validate_acl_overlayfs.sh
var aclDefaultSnapshotterScript string

// IPE (Integrity Policy Enforcement) applies a kernel policy to executable code.
// In audit mode, denied execution is logged but still allowed. These scenarios
// compare the same program on signed /usr and on writable storage.
var (
	_ = Register(newACLIPEAuditScenario(false))
	_ = Register(newACLIPEAuditScenario(true))
)

// The ACL image reads the audit tag during boot, so these checks need separate
// nodes from the ordinary ACL scenarios. Image selection and reporting stay
// with the standard E2E runner.
func newACLIPEAuditScenario(arm64 bool) *Scenario {
	name, image := "ACL_IPEAudit", config.VHDACLGen2TL
	if arm64 {
		name, image = "ACL_ARM64_IPEAudit", config.VHDACLArm64Gen2TL
	}
	return &Scenario{
		Name:        name,
		Description: "Boot an ACL node with IPE audit enabled; verify policy, signed /usr, EROFS, audit behavior for base and writable executables, and signed-container execution",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     image,
			UseNVMe: arm64,
			// Match the existing ACL ARM64 setup: a Trusted Launch-capable v6 VM,
			// with the same SKU in both bootstrap formats and the VMSS model.
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				if arm64 {
					nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v6"
					nbc.IsARM64 = true
				}
			},
			AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
				if arm64 {
					cfg.VmSize = "Standard_D2pds_v6"
				}
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, config.Config.ACLBaseImageSigned)
				if arm64 {
					vmss.SKU.Name = to.Ptr("Standard_D2pds_v6")
				}
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["acl-node-security-profile"] = to.Ptr(ipeAuditProfile(lo.FromPtr(vmss.Tags["acl-node-security-profile"])))
			},
			Validator: validateACLIPEAudit,
		},
	}
}

// The tag is shared with settings such as SELinux. Replace only the IPE entry.
func ipeAuditProfile(profile string) string {
	var fields []string
	for _, field := range strings.Split(profile, ",") {
		if field != "" && strings.SplitN(field, "=", 2)[0] != "ipe" {
			fields = append(fields, field)
		}
	}
	return strings.Join(append(fields, "ipe=audit"), ",")
}

func validateACLIPEAudit(ctx context.Context, s *Scenario) error {
	// Inspect the host kernel and its journal over SSH, not from a container.
	// The requested VM tag alone does not prove that the policy became active.
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo bash -s <<'ACL_IPE_VALIDATION'\n"+aclIPEAuditScript+"\nACL_IPE_VALIDATION\n",
		0, "ACL IPE audit validation failed")
	if result != nil {
		logging.Logf(ctx, "IPE guest evidence:\n%s", result.String())
	}
	if err != nil {
		return err
	}

	const signedContainerName = "signed"
	// Define a pod that uses a container image that has a dmverity referrer.
	signedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "acl-ipe-signed-container",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeSelector:  getNodeSelectorForScenario(s),
			Tolerations:   getPodTolerations(),
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    signedContainerName,
					Image:   "mcr.microsoft.com/oss/v2/inspektor-gadget/kubectl-gadget:v0.58.0-4@sha256:c75f7b821627af9ab6f7f9bae5b93d66981ff75439cfad1d4ab0599dc5eb105a",
					Command: []string{"/usr/bin/sleep", "infinity"},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"/usr/bin/kubectl-gadget", "--help"},
							},
						},
						TimeoutSeconds: 10,
					},
				},
			},
		},
	}

	// Callback function to validate the signed container
	validateSignedContainer := func(pod *corev1.Pod) error {
		containerID, err := getContainerdID(pod, s.Runtime.VM.KubeName, signedContainerName)
		if err != nil {
			return err
		}
		result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			fmt.Sprintf("sudo bash -s -- %q <<'ACL_IPE_CONTAINER_VALIDATION'\n%s\nACL_IPE_CONTAINER_VALIDATION\n",
				containerID, aclIPEAuditScript),
			0, "ACL signed-container IPE validation failed")
		if result != nil {
			logging.Logf(ctx, "IPE container evidence:\n%s", result.String())
		}
		return err
	}

	return deployACLSignedPod(ctx, s, signedPod, validateSignedContainer)
}

func deployACLSignedPod(ctx context.Context, s *Scenario, pod *corev1.Pod, validate func(*corev1.Pod) error) error {
	pod = pod.DeepCopy()
	pod.Name = uniqueKubernetesResourceName(pod.Name)
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return err
	}

	kube := s.Runtime.Kube
	pods := kube.Typed.CoreV1().Pods(pod.Namespace)
	logging.Logf(ctx, "creating signed IPE pod %s/%s", pod.Namespace, pod.Name)
	created, err := pods.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create signed IPE pod %q: %w", pod.Name, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := pods.Delete(cleanupCtx, created.Name, metav1.DeleteOptions{
			GracePeriodSeconds: to.Ptr(int64(0)),
			Preconditions:      &metav1.Preconditions{UID: &created.UID},
		}); err != nil && !apierrors.IsNotFound(err) {
			logging.Logf(ctx, "could not delete signed IPE pod %s/%s: %v", created.Namespace, created.Name, err)
		}
	}()

	observed, err := kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name)
	if err != nil {
		return fmt.Errorf("wait for signed IPE pod %s/%s: %w", created.Namespace, created.Name, err)
	}
	if observed.UID != created.UID {
		return fmt.Errorf("signed IPE pod %s/%s was replaced while waiting for readiness", created.Namespace, created.Name)
	}
	return validate(observed)
}

func getContainerdID(pod *corev1.Pod, nodeName, containerName string) (string, error) {
	if pod.Spec.NodeName != nodeName {
		return "", fmt.Errorf("test pod %q is on node %q, want %q", pod.Name, pod.Spec.NodeName, nodeName)
	}
	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return "", fmt.Errorf("test pod %q is not live (phase %s, deleting %t)", pod.Name, pod.Status.Phase, pod.DeletionTimestamp != nil)
	}
	status, found := lo.Find(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool {
		return status.Name == containerName
	})
	if !found || !status.Ready || status.State.Running == nil {
		return "", fmt.Errorf("test pod %q has no running, ready container named %q", pod.Name, containerName)
	}
	id, ok := strings.CutPrefix(status.ContainerID, "containerd://")
	if !ok || len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		return "", fmt.Errorf("test pod %q container %q has invalid containerd ID %q", pod.Name, containerName, status.ContainerID)
	}
	return id, nil
}

// Baseline ACL nodes must keep overlayfs when the IPE profile is not requested.
// Read the running daemon's config, including imports and plugin defaults.
func validateACLDefaultSnapshotter(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo bash -s <<'ACL_OVERLAYFS_VALIDATION'\n"+aclDefaultSnapshotterScript+"\nACL_OVERLAYFS_VALIDATION\n",
		0, "ACL default snapshotter validation failed")
	if result != nil {
		logging.Logf(ctx, "ACL default snapshotter:\n%s", result.String())
	}
	return err
}
