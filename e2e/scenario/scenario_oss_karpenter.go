package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ossKarpenterImageIDAnnotation = "agentbaker.azure.com/e2e-image-id"
	ossKarpenterRunLabel          = "agentbaker.azure.com/oss-karpenter-run"
)

var ClusterOSSKarpenter = cachedFunc(clusterOSSKarpenter)

func clusterOSSKarpenter(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-oss-karpenter-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting OSS Karpenter cluster model: %w", err)
	}
	configureOSSKarpenterClusterModel(model)
	return prepareCluster(ctx, model, false, false)
}

func configureOSSKarpenterClusterModel(model *armcontainerservice.ManagedCluster) {
	azureOverlayNetworkClusterModelMutator(model)
	model.Properties.NetworkProfile.NetworkDataplane = to.Ptr(armcontainerservice.NetworkDataplaneCilium)
	model.Properties.NetworkProfile.NetworkPolicy = to.Ptr(armcontainerservice.NetworkPolicyCilium)
	model.Properties.OidcIssuerProfile = &armcontainerservice.ManagedClusterOIDCIssuerProfile{
		Enabled: to.Ptr(true),
	}
	if model.Properties.SecurityProfile == nil {
		model.Properties.SecurityProfile = &armcontainerservice.ManagedClusterSecurityProfile{}
	}
	model.Properties.SecurityProfile.WorkloadIdentity = &armcontainerservice.ManagedClusterSecurityProfileWorkloadIdentity{
		Enabled: to.Ptr(true),
	}
}

var _ = Register(&Scenario{
	Name: "Ubuntu2204_OSS_Karpenter_CSE_Compatibility",
	Description: "Builds and runs pinned upstream karpenter-provider-azure, creates an AKSNodeClass and NodePool, " +
		"forces the Karpenter node to use the selected AgentBaker Ubuntu 22.04 VHD, and verifies the node becomes " +
		"Ready and runs a workload. The upstream CSE template is used directly from the pinned source checkout.",
	Config: Config{
		Cluster:     ClusterOSSKarpenter,
		VHD:         config.VHDUbuntu2204Gen2Containerd,
		ClusterTest: runOSSKarpenterCompatibility,
	},
})

type ossKarpenterRun struct {
	Namespace     string
	NodeClassName string
	NodePoolName  string
	PodName       string
	RunLabelValue string
}

func newOSSKarpenterRun() ossKarpenterRun {
	runName := uniqueKubernetesResourceName("ab-karpenter")
	return ossKarpenterRun{
		Namespace:     uniqueKubernetesResourceName("ab-karpenter"),
		NodeClassName: runName,
		NodePoolName:  runName,
		PodName:       "inflate",
		RunLabelValue: runName,
	}
}

func runOSSKarpenterCompatibility(ctx context.Context, s *Scenario) (retErr error) {
	defer logging.LogStep(ctx, "validating OSS Karpenter CSE compatibility")()

	imageID, err := CachedPrepareVHD(ctx, GetVHDRequest{
		Image:    *s.VHD,
		Location: s.Location,
	})
	if err != nil {
		return fmt.Errorf("resolve AgentBaker VHD for OSS Karpenter: %w", err)
	}
	if imageID == "" {
		return fmt.Errorf("resolved AgentBaker VHD image ID is empty")
	}
	logging.Logf(ctx, "OSS Karpenter will provision from AgentBaker VHD %s", imageID.Short())

	build, err := cachedPrepareOSSKarpenterController(ctx, ossKarpenterAzureVersion)
	if err != nil {
		return err
	}
	if err := installOSSKarpenterCRDs(ctx, s.Runtime.Kube, build.SourceDir); err != nil {
		return err
	}

	controller, err := startOSSKarpenterController(ctx, s, build)
	if err != nil {
		return err
	}
	run := newOSSKarpenterRun()
	s.Cleanup(func(cleanupCtx context.Context) error {
		return cleanupOSSKarpenterRun(cleanupCtx, s, build, run, controller)
	})
	defer func() {
		if retErr != nil {
			collectOSSKarpenterDiagnostics(context.WithoutCancel(ctx), s, run)
		}
	}()

	if _, err := s.Runtime.Kube.Typed.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: run.Namespace},
	}, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create OSS Karpenter workload namespace: %w", err)
	}

	nodeClass := newOSSKarpenterNodeClass(run.NodeClassName, string(imageID))
	if err := s.Runtime.Kube.Dynamic.Create(ctx, nodeClass); err != nil {
		return fmt.Errorf("create AKSNodeClass %s: %w", run.NodeClassName, err)
	}
	if err := waitForOSSKarpenterNodeClassReady(ctx, s.Runtime.Kube, controller, run.NodeClassName); err != nil {
		return err
	}

	nodePool := newOSSKarpenterNodePool(run, config.Config.DefaultVMSKU)
	if err := s.Runtime.Kube.Dynamic.Create(ctx, nodePool); err != nil {
		return fmt.Errorf("create NodePool %s: %w", run.NodePoolName, err)
	}

	pod := newOSSKarpenterWorkload(run)
	if _, err := s.Runtime.Kube.Typed.CoreV1().Pods(run.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create OSS Karpenter scheduling demand: %w", err)
	}

	nodeClaim, err := waitForOSSKarpenterNodeClaim(ctx, s.Runtime.Kube, controller, run.NodePoolName, string(imageID))
	if err != nil {
		return err
	}
	node, err := waitForOSSKarpenterNodeReady(ctx, s.Runtime.Kube, controller, run)
	if err != nil {
		return err
	}
	runningPod, err := s.Runtime.Kube.WaitUntilPodRunning(ctx, run.Namespace, "app=inflate", "")
	if err != nil {
		return fmt.Errorf("wait for workload on OSS Karpenter node: %w", err)
	}
	if runningPod.Spec.NodeName != node.Name {
		return fmt.Errorf("workload scheduled to %s, expected OSS Karpenter node %s", runningPod.Spec.NodeName, node.Name)
	}
	actualImageID, _, _ := unstructured.NestedString(nodeClaim.Object, "status", "imageID")
	if !sameAzureResourceID(actualImageID, string(imageID)) {
		return fmt.Errorf("NodeClaim image ID %q does not match selected AgentBaker VHD %q", actualImageID, imageID)
	}

	logging.Logf(ctx, "OSS Karpenter provisioned Ready node %s from %s and scheduled pod %s/%s",
		node.Name, imageID.Short(), runningPod.Namespace, runningPod.Name)
	return nil
}

func newOSSKarpenterNodeClass(name, imageID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.azure.com/v1beta1",
		"kind":       "AKSNodeClass",
		"metadata": map[string]any{
			"name": name,
			"annotations": map[string]any{
				ossKarpenterImageIDAnnotation: imageID,
				"kubernetes.io/description":   "AgentBaker OSS Karpenter CSE compatibility image",
			},
		},
		"spec": map[string]any{
			"imageFamily": "Ubuntu2204",
		},
	}}
}

func newOSSKarpenterNodePool(run ossKarpenterRun, vmSize string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodePool",
		"metadata": map[string]any{
			"name": run.NodePoolName,
		},
		"spec": map[string]any{
			// The exact E2E image is intentionally outside the NodeClass' normal
			// discovered image set, so Karpenter can report image drift. Prevent
			// disruption from replacing the compatibility node during validation.
			"disruption": map[string]any{
				"consolidateAfter": "Never",
				"budgets": []any{
					map[string]any{"nodes": "0"},
				},
			},
			"limits": map[string]any{
				"cpu": "4",
			},
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						ossKarpenterRunLabel:                  run.RunLabelValue,
						"kubernetes.azure.com/ebpf-dataplane": "cilium",
					},
				},
				"spec": map[string]any{
					"expireAfter": "Never",
					"nodeClassRef": map[string]any{
						"group": "karpenter.azure.com",
						"kind":  "AKSNodeClass",
						"name":  run.NodeClassName,
					},
					"requirements": []any{
						map[string]any{
							"key":      "kubernetes.io/arch",
							"operator": "In",
							"values":   []any{"amd64"},
						},
						map[string]any{
							"key":      "kubernetes.io/os",
							"operator": "In",
							"values":   []any{"linux"},
						},
						map[string]any{
							"key":      "karpenter.sh/capacity-type",
							"operator": "In",
							"values":   []any{"on-demand"},
						},
						map[string]any{
							"key":      "node.kubernetes.io/instance-type",
							"operator": "In",
							"values":   []any{vmSize},
						},
					},
					"startupTaints": []any{
						map[string]any{
							"key":    "node.cilium.io/agent-not-ready",
							"value":  "true",
							"effect": "NoExecute",
						},
					},
				},
			},
		},
	}}
}

func newOSSKarpenterWorkload(run ossKarpenterRun) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.PodName,
			Namespace: run.Namespace,
			Labels:    map[string]string{"app": "inflate"},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				ossKarpenterRunLabel: run.RunLabelValue,
			},
			Containers: []corev1.Container{
				{
					Name:  "inflate",
					Image: "mcr.microsoft.com/oss/kubernetes/pause:3.6",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}
}

func waitForOSSKarpenterNodeClassReady(
	ctx context.Context,
	kube *Kubeclient,
	controller *ossKarpenterControllerProcess,
	name string,
) error {
	defer logging.LogStepf(ctx, "waiting for AKSNodeClass %s", name)()

	nodeClass := &unstructured.Unstructured{}
	nodeClass.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "karpenter.azure.com",
		Version: "v1beta1",
		Kind:    "AKSNodeClass",
	})
	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := controller.RunningError(); err != nil {
			return false, err
		}
		if err := kube.Dynamic.Get(ctx, client.ObjectKey{Name: name}, nodeClass); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return unstructuredConditionTrue(nodeClass, "Ready"), nil
	})
	if err != nil {
		return fmt.Errorf("wait for AKSNodeClass %s Ready=True: %w", name, err)
	}
	return nil
}

func waitForOSSKarpenterNodeClaim(
	ctx context.Context,
	kube *Kubeclient,
	controller *ossKarpenterControllerProcess,
	nodePoolName, expectedImageID string,
) (*unstructured.Unstructured, error) {
	defer logging.LogStepf(ctx, "waiting for NodeClaim from NodePool %s", nodePoolName)()

	var matched *unstructured.Unstructured
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := controller.RunningError(); err != nil {
			return false, err
		}
		claims := &unstructured.UnstructuredList{}
		claims.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "karpenter.sh",
			Version: "v1",
			Kind:    "NodeClaimList",
		})
		if err := kube.Dynamic.List(ctx, claims, client.MatchingLabels{"karpenter.sh/nodepool": nodePoolName}); err != nil {
			return false, err
		}
		for i := range claims.Items {
			imageID, found, err := unstructured.NestedString(claims.Items[i].Object, "status", "imageID")
			if err != nil {
				return false, err
			}
			if found && sameAzureResourceID(imageID, expectedImageID) {
				matched = claims.Items[i].DeepCopy()
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("wait for NodeClaim using AgentBaker VHD %q: %w", expectedImageID, err)
	}
	return matched, nil
}

func waitForOSSKarpenterNodeReady(
	ctx context.Context,
	kube *Kubeclient,
	controller *ossKarpenterControllerProcess,
	run ossKarpenterRun,
) (*corev1.Node, error) {
	defer logging.LogStepf(ctx, "waiting for OSS Karpenter node from NodePool %s", run.NodePoolName)()

	var matched *corev1.Node
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := controller.RunningError(); err != nil {
			return false, err
		}
		nodes, err := kube.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", ossKarpenterRunLabel, run.RunLabelValue),
		})
		if err != nil {
			return false, err
		}
		for i := range nodes.Items {
			if nodeReady(&nodes.Items[i]) {
				matched = nodes.Items[i].DeepCopy()
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("wait for OSS Karpenter node Ready=True: %w", err)
	}
	return matched, nil
}

func unstructuredConditionTrue(object *unstructured.Unstructured, conditionType string) bool {
	conditions, found, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]any)
		if ok && condition["type"] == conditionType && condition["status"] == "True" {
			return true
		}
	}
	return false
}

func sameAzureResourceID(left, right string) bool {
	return strings.EqualFold(strings.TrimRight(left, "/"), strings.TrimRight(right, "/"))
}

func cleanupOSSKarpenterRun(
	ctx context.Context,
	s *Scenario,
	build ossKarpenterControllerBuild,
	run ossKarpenterRun,
	controller *ossKarpenterControllerProcess,
) error {
	defer logging.LogStep(ctx, "cleaning up OSS Karpenter scenario")()

	var errs []error
	kube := s.Runtime.Kube
	cleanupController := controller
	if controllerErr := controller.RunningError(); controllerErr != nil {
		// NodeClaim finalizers invoke Karpenter's Azure deletion path. If the
		// original controller died, restart the pinned controller before deleting
		// the NodePool so the VM, NIC, and disk are not orphaned.
		logging.Logf(ctx, "OSS Karpenter controller stopped before cleanup; restarting it to finalize capacity: %v", controllerErr)
		errs = append(errs, controllerErr, controller.Stop(ctx))
		cleanupController = nil
		restarted, err := startOSSKarpenterController(ctx, s, build)
		if err != nil {
			errs = append(errs, fmt.Errorf("restart Karpenter controller for cleanup: %w", err))
		} else {
			cleanupController = restarted
		}
	}
	if err := kube.Typed.CoreV1().Namespaces().Delete(ctx, run.Namespace, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete workload namespace: %w", err))
	}

	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	nodePool.SetName(run.NodePoolName)
	if err := kube.Dynamic.Delete(ctx, nodePool); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete NodePool: %w", err))
	}
	if err := deleteOSSKarpenterNodeClaims(ctx, kube, run.NodePoolName); err != nil {
		errs = append(errs, err)
	}
	if err := waitForOSSKarpenterCapacityDeleted(ctx, kube, run); err != nil {
		errs = append(errs, err)
	}

	nodeClass := &unstructured.Unstructured{}
	nodeClass.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.azure.com", Version: "v1beta1", Kind: "AKSNodeClass"})
	nodeClass.SetName(run.NodeClassName)
	if err := kube.Dynamic.Delete(ctx, nodeClass); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete AKSNodeClass: %w", err))
	}
	errs = append(errs, cleanupController.Stop(ctx))
	return errors.Join(errs...)
}

func deleteOSSKarpenterNodeClaims(ctx context.Context, kube *Kubeclient, nodePoolName string) error {
	nodeClaim := &unstructured.Unstructured{}
	nodeClaim.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodeClaim"})
	if err := kube.Dynamic.DeleteAllOf(ctx, nodeClaim, client.MatchingLabels{"karpenter.sh/nodepool": nodePoolName}); err != nil {
		return fmt.Errorf("delete NodeClaims for NodePool %s: %w", nodePoolName, err)
	}
	return nil
}

func waitForOSSKarpenterCapacityDeleted(ctx context.Context, kube *Kubeclient, run ossKarpenterRun) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		claims := &unstructured.UnstructuredList{}
		claims.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodeClaimList"})
		if err := kube.Dynamic.List(ctx, claims, client.MatchingLabels{"karpenter.sh/nodepool": run.NodePoolName}); err != nil {
			if !apierrors.IsNotFound(err) {
				return false, err
			}
		}
		nodes, err := kube.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", ossKarpenterRunLabel, run.RunLabelValue),
		})
		if err != nil {
			return false, err
		}
		return len(claims.Items) == 0 && len(nodes.Items) == 0, nil
	})
}

func collectOSSKarpenterDiagnostics(ctx context.Context, s *Scenario, run ossKarpenterRun) {
	diagnosticCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	writeObject := func(name string, value any) {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			logging.Logf(diagnosticCtx, "failed to marshal %s diagnostics: %v", name, err)
			return
		}
		if err := writeToFile(s.artifactName, name, string(data)); err != nil {
			logging.Logf(diagnosticCtx, "failed to write %s diagnostics: %v", name, err)
		}
	}

	nodeClass := &unstructured.Unstructured{}
	nodeClass.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.azure.com", Version: "v1beta1", Kind: "AKSNodeClass"})
	if err := s.Runtime.Kube.Dynamic.Get(diagnosticCtx, client.ObjectKey{Name: run.NodeClassName}, nodeClass); err == nil {
		writeObject("oss-karpenter-nodeclass.json", nodeClass.Object)
	}
	nodePool := &unstructured.Unstructured{}
	nodePool.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	if err := s.Runtime.Kube.Dynamic.Get(diagnosticCtx, client.ObjectKey{Name: run.NodePoolName}, nodePool); err == nil {
		writeObject("oss-karpenter-nodepool.json", nodePool.Object)
	}
	claims := &unstructured.UnstructuredList{}
	claims.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodeClaimList"})
	if err := s.Runtime.Kube.Dynamic.List(diagnosticCtx, claims, client.MatchingLabels{"karpenter.sh/nodepool": run.NodePoolName}); err == nil {
		writeObject("oss-karpenter-nodeclaims.json", claims.Object)
	}
	if nodes, err := s.Runtime.Kube.Typed.CoreV1().Nodes().List(diagnosticCtx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", ossKarpenterRunLabel, run.RunLabelValue),
	}); err == nil {
		writeObject("oss-karpenter-nodes.json", nodes)
	}
	if pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(run.Namespace).List(diagnosticCtx, metav1.ListOptions{}); err == nil {
		writeObject("oss-karpenter-workload-pods.json", pods)
	}
	if events, err := s.Runtime.Kube.Typed.CoreV1().Events("").List(diagnosticCtx, metav1.ListOptions{}); err == nil {
		writeObject("oss-karpenter-events.json", events)
	}
}
