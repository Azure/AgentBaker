package scenario

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const kataCCSmokeVMSize = "Standard_DC8as_cc_v6"

var _ = Register(&Scenario{
	Name:        "AzureLinuxV3Gen2KataCC_Runc",
	Description: "Bootstraps the Azure Linux 3 Kata-CC VHD and completes a BusyBox pod using the runc handler on the test node",
	Location:    "eastus2",
	Tags:        Tags{Kata: true},
	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDAzureLinuxV3Gen2KataCC,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.AgentPoolProfile.VMSize = kataCCSmokeVMSize
		},
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr(kataCCSmokeVMSize)
		},
		// Establish host usability first; confidential execution will get its own
		// kata-cc workload validation once the guest and attestation path are ready.
		Validator: validateKataCCRuncPod,
	},
})

func validateKataCCRuncPod(ctx context.Context, s *Scenario) error {
	// A unique RuntimeClass explicitly selects runc without relying on the
	// cluster's default runtime or changing an existing shared RuntimeClass.
	runtimeClass, err := createKataRuntimeClass(ctx, s, "runc")
	if err != nil {
		return err
	}
	pod := kataCCRuncPod(s, runtimeClass)
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return err
	}
	pods := s.Runtime.Kube.Typed.CoreV1().Pods(pod.Namespace)
	created, err := pods.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create runc smoke pod: %w", err)
	}
	s.Cleanup(func(ctx context.Context) error {
		err := pods.Delete(ctx, created.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete runc smoke pod %s: %w", created.Name, err)
		}
		return nil
	})
	return waitForKataCCRuncPod(ctx, s, created)
}

func kataCCRuncPod(s *Scenario, runtimeClass string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueKubernetesResourceName(s.Runtime.VM.KubeName + "-runc"),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			RuntimeClassName:      to.Ptr(runtimeClass),
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: to.Ptr(int64(300)),
			NodeSelector:          getNodeSelectorForScenario(s),
			Tolerations:           getPodTolerations(),
			Containers: []corev1.Container{{
				Name:                     "busybox",
				Image:                    "mcr.microsoft.com/cbl-mariner/busybox:2.0",
				Command:                  []string{"sh", "-ec", "uname -a; echo azl3-cc-runc-smoke-complete"},
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
			}},
		},
	}
}

func waitForKataCCRuncPod(ctx context.Context, s *Scenario, pod *corev1.Pod) error {
	last := pod
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		current, err := s.Runtime.Kube.Typed.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		last = current
		switch current.Status.Phase {
		case corev1.PodSucceeded:
			if current.Spec.NodeName != s.Runtime.VM.KubeName {
				return false, fmt.Errorf("runc pod ran on %q instead of test node %q", current.Spec.NodeName, s.Runtime.VM.KubeName)
			}
			if len(current.Status.ContainerStatuses) != 1 || current.Status.ContainerStatuses[0].State.Terminated == nil ||
				current.Status.ContainerStatuses[0].State.Terminated.ExitCode != 0 {
				return false, fmt.Errorf("runc pod missing successful container termination: %s", formatPodDiagnostics(current))
			}
			logging.Logf(ctx, "runc BusyBox pod %s succeeded on %s with exit code 0", pod.Name, current.Spec.NodeName)
			return true, nil
		case corev1.PodFailed:
			return false, fmt.Errorf("runc smoke pod failed: %s", formatPodDiagnostics(current))
		default:
			return false, nil
		}
	})
	if err != nil {
		debugCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		logPodDebugInfo(debugCtx, s.Runtime.Kube, last)
		return fmt.Errorf("waiting for runc smoke pod %s (last phase %s): %w", pod.Name, last.Status.Phase, err)
	}
	return nil
}
