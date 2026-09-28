package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	aclIPETestVMSS  = "/subscriptions/test/resourceGroups/nodes/providers/Microsoft.Compute/virtualMachineScaleSets/scenario52"
	aclIPETestImage = "/subscriptions/test/resourceGroups/images/providers/Microsoft.Compute/galleries/pr52/images/aclgen2TL/versions/1.2.3"
	aclIPETestVMID  = "e7b560b4-621d-4831-8224-fb759f961de3"
)

func aclIPEFixture() (*Scenario, armcompute.VirtualMachineScaleSet) {
	model := armcompute.VirtualMachineScaleSet{
		ID: to.Ptr(aclIPETestVMSS), Name: to.Ptr("scenario52"), Etag: to.Ptr(`"etag-1"`),
		SKU: &armcompute.SKU{Capacity: to.Ptr[int64](1)},
		Tags: map[string]*string{"owner": to.Ptr("e2e"), "preserve": to.Ptr("unchanged"),
			aclIPECreationTag: to.Ptr("test-creation-token")},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			Overprovision: to.Ptr(false),
			UpgradePolicy: &armcompute.UpgradePolicy{Mode: to.Ptr(armcompute.UpgradeModeManual)},
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &armcompute.ImageReference{ID: to.Ptr(aclIPETestImage)},
					OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{
						DiffDiskSettings: &armcompute.DiffDiskSettings{Option: to.Ptr(armcompute.DiffDiskOptionsLocal)},
					},
				},
			},
		},
	}
	s := &Scenario{Name: "ACL", Runtime: &ScenarioRuntime{
		VMSSName: "scenario52",
		Cluster: &Cluster{Model: &armcontainerservice.ManagedCluster{
			ID:         to.Ptr("/subscriptions/test/resourceGroups/rg/providers/Microsoft.ContainerService/managedClusters/test"),
			Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("nodes")},
		}},
		VM: &ScenarioVM{VMSS: &model, ipeCreationAttempted: true,
			ipeCreationReceipt: &aclIPECreationReceipt{resourceID: aclIPETestVMSS, token: "test-creation-token"},
			VM: &armcompute.VirtualMachineScaleSetVM{
				ID: to.Ptr(aclIPETestVMSS + "/virtualMachines/0"), InstanceID: to.Ptr("0"),
				Properties: &armcompute.VirtualMachineScaleSetVMProperties{VMID: to.Ptr(aclIPETestVMID)},
			}},
	}}
	return s, model
}

func TestACLIPETransitionGateAndCreation(t *testing.T) {
	s, model := aclIPEFixture()
	t.Setenv(aclIPETransitionEnv, "")
	t.Setenv(aclIPEModeEnv, "")
	oldKeep := config.Config.KeepVMSS
	config.Config.KeepVMSS = false
	t.Cleanup(func() { config.Config.KeepVMSS = oldKeep })
	require.NoError(t, aclIPETransitionGate(s))
	require.NoError(t, ValidateACLIPETransition(t.Context(), s))
	for _, registered := range List() {
		if registered.Name == "ACL" {
			require.NotNil(t, registered.AKSNodeConfigMutator, "ordinary ACL must keep its original ANC provisioning path")
			ordinary := *registered
			require.NotNil(t, ordinary.AKSNodeConfigMutator)
			nbc := &datamodel.NodeBootstrappingConfiguration{KubeletConfig: make(map[string]string)}
			ordinary.BootstrapConfigMutator(nil, nbc)
			require.NotContains(t, nbc.KubeletConfig, "--register-with-taints")
			anc := &aksnodeconfigv1.Configuration{KubeletConfig: baseKubeletConfig()}
			ordinary.AKSNodeConfigMutator(nil, anc)
			require.NotContains(t, anc.KubeletConfig.KubeletFlags, "--register-with-taints")
			base := &armcompute.VirtualMachineScaleSet{Properties: &armcompute.VirtualMachineScaleSetProperties{
				UpgradePolicy: &armcompute.UpgradePolicy{Mode: to.Ptr(armcompute.UpgradeModeAutomatic)},
			}}
			registered.VMConfigMutator(base)
			require.Equal(t, armcompute.UpgradeModeAutomatic, *base.Properties.UpgradePolicy.Mode)
		}
	}
	t.Setenv(aclIPETransitionEnv, "off-to-audit")
	require.ErrorContains(t, aclIPETransitionGate(s), "requires ACL_IPE_EXPECTED_MODE=off")
	t.Setenv(aclIPEModeEnv, "off")
	require.ErrorContains(t, aclIPETransitionGate(s), aclIPEApprovedClusterEnv)
	t.Setenv(aclIPEApprovedClusterEnv, *s.Runtime.Cluster.Model.ID)
	require.NoError(t, aclIPETransitionGate(s))
	s.VHDCaching = true
	require.ErrorContains(t, aclIPETransitionGate(s), "directly provisioned")
	s.VHDCaching = false
	config.Config.KeepVMSS = true
	require.ErrorContains(t, aclIPETransitionGate(s), "KEEP_VMSS=false")
	config.Config.KeepVMSS = false
	for _, registered := range List() {
		if registered.Name == "ACL" {
			transition := *registered
			require.NotNil(t, transition.AKSNodeConfigMutator)
			nbc := &datamodel.NodeBootstrappingConfiguration{KubeletConfig: make(map[string]string)}
			transition.BootstrapConfigMutator(nil, nbc)
			require.Equal(t, aclIPETransitionTaint, nbc.KubeletConfig["--register-with-taints"])
			anc := &aksnodeconfigv1.Configuration{KubeletConfig: baseKubeletConfig()}
			transition.AKSNodeConfigMutator(nil, anc)
			require.Equal(t, aclIPETransitionTaint, anc.KubeletConfig.KubeletFlags["--register-with-taints"])
			base := &armcompute.VirtualMachineScaleSet{Properties: &armcompute.VirtualMachineScaleSetProperties{
				UpgradePolicy: &armcompute.UpgradePolicy{Mode: to.Ptr(armcompute.UpgradeModeAutomatic)},
			}}
			registered.VMConfigMutator(base)
			require.Equal(t, armcompute.UpgradeModeManual, *base.Properties.UpgradePolicy.Mode)
			pod := podHTTPServerLinux(s)
			require.Equal(t, s.Runtime.VM.KubeName, pod.Spec.NodeSelector["kubernetes.io/hostname"])
			require.Contains(t, pod.Spec.Tolerations, corev1.Toleration{
				Key: aclIPETransitionTaintKey, Operator: corev1.TolerationOpEqual,
				Value: "true", Effect: corev1.TaintEffectNoSchedule,
			})
		}
	}
	require.NoError(t, aclIPEOwnedModel(s, &model))
	*model.SKU.Capacity = 2
	require.ErrorContains(t, aclIPEOwnedModel(s, &model), "one instance")
	*model.SKU.Capacity = 1
	model.Tags["aks-managed-poolName"] = to.Ptr("nodepool1")
	require.ErrorContains(t, aclIPEOwnedModel(s, &model), "AKS-managed pool")
}

func TestACLIPETransitionChecksApprovedExistingClusterBeforePreparation(t *testing.T) {
	previousConfig, previousAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = previousConfig, previousAzure })
	config.Config.SubscriptionID = "00000000-0000-0000-0000-000000000052"
	config.Config.KeepVMSS = false
	config.Config.TestPreProvision = false
	location := "eastus"
	id := "/subscriptions/" + config.Config.SubscriptionID +
		"/resourceGroups/" + config.ResourceGroupName(location) +
		"/providers/Microsoft.ContainerService/managedClusters/abe2e-kubenet-v5-abcde"
	t.Setenv(aclIPETransitionEnv, "off-to-audit")
	t.Setenv(aclIPEModeEnv, "off")
	t.Setenv(aclIPEApprovedClusterEnv, id)
	s := &Scenario{Name: "ACL", Location: location, Config: Config{
		Cluster: func(context.Context, ClusterRequest) (*Cluster, error) {
			t.Fatal("generic create/reconcile fallback must never be called")
			return nil, nil
		},
	}}
	parsed, err := aclIPEApprovedClusterID(s)
	require.NoError(t, err)
	require.Equal(t, "abe2e-kubenet-v5-abcde", parsed.Name)
	t.Setenv(aclIPEApprovedClusterEnv, strings.Replace(id, "managedClusters", "virtualMachineScaleSets", 1))
	_, err = aclIPEApprovedClusterID(s)
	require.ErrorContains(t, err, "existing E2E kubenet")
	t.Setenv(aclIPEApprovedClusterEnv, id)
	aks, err := armcontainerservice.NewManagedClustersClient(config.Config.SubscriptionID, nil,
		&arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.Equal(t, http.MethodGet, req.Method, "no image, cluster, or resource group preparation before approved GET")
				return gcResponse(req, http.StatusNotFound, `{"error":{"code":"ResourceNotFound"}}`)
			}),
		}}})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{AKS: aks}
	require.ErrorContains(t, runScenario(t.Context(), "ACL", s), "no create/reconcile fallback")
	_, err = aclIPESelectCluster(t.Context(), s, nil, ClusterRequest{})
	require.ErrorContains(t, err, "refuses generic cluster create/reconcile fallback")
	approved := &Cluster{}
	selected, err := aclIPESelectCluster(t.Context(), s, approved, ClusterRequest{})
	require.NoError(t, err)
	require.Same(t, approved, selected)
}

func TestACLIPETransitionRightsAndWorkloads(t *testing.T) {
	action := "Microsoft.Compute/virtualMachineScaleSets/virtualMachines/restart/action"
	permissions := []*armauthorization.Permission{
		{Actions: []*string{to.Ptr("Microsoft.Compute/*")}, NotActions: []*string{to.Ptr(action)}},
	}
	require.False(t, aclIPEAllows(permissions, action))
	permissions = append(permissions, &armauthorization.Permission{Actions: []*string{to.Ptr(action)}})
	require.True(t, aclIPEAllows(permissions, action))
	permissions = []*armauthorization.Permission{{Actions: []*string{to.Ptr("*")}, Condition: to.Ptr("resource condition")}}
	require.False(t, aclIPEAllows(permissions, action))

	node := "scenario52-000000"
	ds := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: "kube-proxy",
			OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "kube-proxy", Controller: to.Ptr(true)}}},
		Spec: corev1.PodSpec{NodeName: node},
	}
	require.ErrorContains(t, aclIPEAllowedWorkloads([]corev1.Pod{ds}, node, nil), "unexpected workload")
	require.NoError(t, aclIPEAllowedWorkloads([]corev1.Pod{ds}, node, map[string]bool{"kube-proxy": true}))
	unexpected := ds.DeepCopy()
	unexpected.Namespace, unexpected.Name, unexpected.OwnerReferences = "default", "other", nil
	require.ErrorContains(t, aclIPEAllowedWorkloads([]corev1.Pod{ds, *unexpected}, node,
		map[string]bool{"kube-proxy": true}), "default/other")
	t.Setenv(aclIPEAllowedDaemonsetsEnv, "kube-proxy,cloud-node-manager")
	allowed, err := aclIPEAllowedDaemonsets()
	require.NoError(t, err)
	require.True(t, allowed["cloud-node-manager"])
	t.Setenv(aclIPEAllowedDaemonsetsEnv, "kube-proxy,*")
	_, err = aclIPEAllowedDaemonsets()
	require.ErrorContains(t, err, "without wildcards")
}

func TestACLIPETransitionLeaseIdentityAndReboot(t *testing.T) {
	s, _ := aclIPEFixture()
	s.Runtime.VM.KubeName = "scenario52-000000"
	leaseTime := metav1.NewMicroTime(time.Now().Add(-15 * time.Second))
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: s.Runtime.VM.KubeName, UID: types.UID("node-uid")},
		Spec: corev1.NodeSpec{ProviderID: "azure://" + aclIPETestVMSS + "/virtualMachines/0",
			Taints: []corev1.Taint{{Key: aclIPETransitionTaintKey, Value: "true", Effect: corev1.TaintEffectNoSchedule}}},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
	}
	lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: node.Name, Namespace: "kube-node-lease"},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: to.Ptr(node.Name), RenewTime: &leaseTime}}
	s.Runtime.Kube = &Kubeclient{Typed: fake.NewClientset(node, lease)}
	before := aclIPEIdentity{resourceID: aclIPETestVMSS + "/virtualMachines/0",
		instanceID: "0", vmID: aclIPETestVMID, imageID: aclIPETestImage,
		diskConfig: []byte(`{"diffDiskSettings":{"option":"Local"}}`),
		nonce:      "7cf46a59-5740-4220-b5d0-906a65a456cd",
		bootID:     "0b1ddc3c-e6f5-4cb5-aa1a-644f18d7a1b1"}
	require.NoError(t, aclIPENode(t.Context(), s, &before))
	after := before
	after.bootID = "23e197e3-11ee-4c9b-846d-070700c546e8"
	after.leaseRenew = before.leaseRenew.Add(10 * time.Second)
	require.NoError(t, aclIPEInstanceUnchanged(before, after))
	after.nonce = "f747d201-ce09-4d94-9589-78fd427a9741"
	require.ErrorContains(t, aclIPEInstanceUnchanged(before, after), "guest nonce")
	after = before
	after.bootID = "23e197e3-11ee-4c9b-846d-070700c546e8"
	require.ErrorContains(t, aclIPEInstanceUnchanged(before, after), "fresh Kubernetes Node Lease")
	after.leaseRenew = before.leaseRenew.Add(10 * time.Second)
	after.nodeUID = "replacement"
	require.ErrorContains(t, aclIPEInstanceUnchanged(before, after), "Kubernetes Node identity")
	after = before
	after.bootID = "23e197e3-11ee-4c9b-846d-070700c546e8"
	after.leaseRenew = before.leaseRenew.Add(10 * time.Second)
	after.diskConfig = []byte(`{"diffDiskSettings":{"option":"None"}}`)
	require.ErrorContains(t, aclIPEInstanceUnchanged(before, after), "OS disk configuration")
	after = before
	after.bootID = "23e197e3-11ee-4c9b-846d-070700c546e8"
	after.leaseRenew = before.leaseRenew.Add(10 * time.Second)
	after.imageID = strings.Replace(before.imageID, "1.2.3", "1.2.4", 1)
	require.ErrorContains(t, aclIPEInstanceUnchanged(before, after), "image")
}

func TestACLIPETransitionWaitsForPostBootReadyAndLease(t *testing.T) {
	s, _ := aclIPEFixture()
	s.Runtime.VM.KubeName = "scenario52-000000"
	bootObservedAt := time.Now()
	before := aclIPEIdentity{
		resourceID: aclIPETestVMSS + "/virtualMachines/0", nodeUID: types.UID("same-node"),
		providerID: "azure://" + aclIPETestVMSS + "/virtualMachines/0",
		leaseRenew: bootObservedAt.Add(-20 * time.Second),
	}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: s.Runtime.VM.KubeName, UID: before.nodeUID},
		Spec: corev1.NodeSpec{ProviderID: before.providerID,
			Taints: []corev1.Taint{{Key: aclIPETransitionTaintKey, Value: "true", Effect: corev1.TaintEffectNoSchedule}}}}
	oldRenew := metav1.NewMicroTime(before.leaseRenew.Add(5 * time.Second))
	lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: node.Name, Namespace: "kube-node-lease"},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: to.Ptr(node.Name), RenewTime: &oldRenew}}
	client := fake.NewClientset(node, lease)
	s.Runtime.Kube = &Kubeclient{Typed: client}
	after := aclIPEIdentity{resourceID: before.resourceID}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	require.ErrorContains(t, aclIPEPollNodeAfterBoot(ctx, s, before, bootObservedAt, &after, time.Millisecond, 30*time.Millisecond), "not Ready")
	node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
	_, err := client.CoreV1().Nodes().Update(t.Context(), node, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.ErrorContains(t, aclIPEPollNodeAfterBoot(t.Context(), s, before, bootObservedAt, &after, time.Millisecond, 30*time.Millisecond), "must be later than observed new boot")
	newRenew := metav1.NewMicroTime(bootObservedAt.Add(time.Second))
	lease.Spec.RenewTime = &newRenew
	_, err = client.CoordinationV1().Leases("kube-node-lease").Update(t.Context(), lease, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, aclIPEPollNodeAfterBoot(t.Context(), s, before, bootObservedAt, &after, time.Millisecond, 30*time.Millisecond))
	require.Equal(t, before.nodeUID, after.nodeUID)
	node.UID = "replacement"
	_, err = client.CoreV1().Nodes().Update(t.Context(), node, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.ErrorContains(t, aclIPEPollNodeAfterBoot(t.Context(), s, before, bootObservedAt, &after, time.Millisecond, 30*time.Millisecond), "differs from original")
}

func TestACLIPETransitionCleanupArmedBeforeProbes(t *testing.T) {
	s, model := aclIPEFixture()
	s.cleanup = &scenarioCleanup{}
	tags, err := aclIPETaggedModel(model.Tags)
	require.NoError(t, err)
	require.ErrorContains(t, aclIPEArmTagRestore(s, model, tags), "without armed")
	aclIPEArmOwnedCleanup(s, s.Runtime.VM)
	require.Equal(t, aclIPETransitionCleanTime, s.cleanup.timeout)
	require.NotNil(t, s.Runtime.VM.ipeTransitionCleanup)
	require.NoError(t, s.Runtime.VM.ipeTransitionCleanup(t.Context()))
	require.NoError(t, aclIPEArmTagRestore(s, model, tags))
	require.NotNil(t, s.Runtime.VM.ipeTransitionCleanup)
}

func TestACLIPETransitionFreshTargetedPod(t *testing.T) {
	now := time.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{UID: types.UID("new-pod"), CreationTimestamp: metav1.NewTime(now.Add(time.Second))},
		Spec:       corev1.PodSpec{NodeName: "owned-node"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	created := pod.DeepCopy()
	created.Status = corev1.PodStatus{}
	uid, err := aclIPEProbePodEvidence(created, pod, "owned-node", now)
	require.NoError(t, err)
	require.Equal(t, types.UID("new-pod"), uid)
	pod.CreationTimestamp = metav1.NewTime(now.Add(-time.Second))
	created.CreationTimestamp = pod.CreationTimestamp
	_, err = aclIPEProbePodEvidence(created, pod, "owned-node", now)
	require.ErrorContains(t, err, "creation time")
	pod.CreationTimestamp = metav1.NewTime(now.Add(time.Second))
	created.CreationTimestamp = pod.CreationTimestamp
	pod.Spec.NodeName = "nodepool1"
	_, err = aclIPEProbePodEvidence(created, pod, "owned-node", now)
	require.ErrorContains(t, err, "exact node")
	pod.Spec.NodeName = "owned-node"
	pod.Status.Conditions = nil
	_, err = aclIPEProbePodEvidence(created, pod, "owned-node", now)
	require.ErrorContains(t, err, "fresh targeted")
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	pod.UID = "stale-pod"
	_, err = aclIPEProbePodEvidence(created, pod, "owned-node", now)
	require.ErrorContains(t, err, "UID")
	pod.UID = created.UID
	created.CreationTimestamp = metav1.NewTime(now.Truncate(time.Second))
	pod.CreationTimestamp = created.CreationTimestamp
	_, err = aclIPEProbePodEvidence(created, pod, "owned-node", now)
	require.NoError(t, err, "Kubernetes creation timestamps are serialized to seconds")
}

func TestACLIPETransitionOwnedSoleInstanceAndImage(t *testing.T) {
	s, model := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	vmID := aclIPETestVMID
	instances := 1
	client, err := armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				if strings.HasSuffix(req.URL.Path, "/virtualMachines") {
					value := make([]map[string]string, instances)
					for i := range value {
						value[i] = map[string]string{"instanceId": fmt.Sprint(i)}
					}
					data, err := json.Marshal(map[string]any{"value": value})
					require.NoError(t, err)
					return gcResponse(req, 200, string(data))
				}
				require.Equal(t, http.MethodGet, req.Method)
				require.True(t, strings.HasSuffix(req.URL.Path, "/virtualMachines/0"))
				data, err := json.Marshal(map[string]any{
					"id": aclIPETestVMSS + "/virtualMachines/0", "instanceId": "0",
					"properties": map[string]any{
						"vmId": vmID,
						"storageProfile": map[string]any{
							"imageReference": map[string]any{"id": aclIPETestImage},
							"osDisk":         map[string]any{"caching": "ReadOnly", "diffDiskSettings": map[string]string{"option": "Local"}},
						},
					},
				})
				require.NoError(t, err)
				return gcResponse(req, 200, string(data))
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSSVM: client}
	id, err := aclIPEVM(t.Context(), s, model)
	require.NoError(t, err)
	require.Equal(t, aclIPETestImage, id.imageID)
	require.NotEmpty(t, id.diskConfig)
	vmID = "23e197e3-11ee-4c9b-846d-070700c546e8"
	_, err = aclIPEVM(t.Context(), s, model)
	require.ErrorContains(t, err, "differs from provisioned pinned image")
	vmID = aclIPETestVMID
	instances = 2
	_, err = aclIPEVM(t.Context(), s, model)
	require.ErrorContains(t, err, "exactly one owned VMSS instance")
}

func aclIPEModelJSON(t *testing.T, model armcompute.VirtualMachineScaleSet) string {
	t.Helper()
	data, err := json.Marshal(model)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	body["etag"] = *model.Etag
	data, err = json.Marshal(body)
	require.NoError(t, err)
	return string(data)
}

func TestACLIPETransitionPatchRestoreAndConflict(t *testing.T) {
	s, original := aclIPEFixture()
	tagged, err := aclIPETaggedModel(original.Tags)
	require.NoError(t, err)
	require.Equal(t, "unchanged", *tagged["preserve"])
	require.NotContains(t, original.Tags, aclIPESecurityProfileTag)
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	current := original
	writes := 0
	conflict := false
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
		vmssCreationTestPolicy(func(req *http.Request) *http.Response {
			if req.Method == http.MethodGet {
				return gcResponse(req, 200, aclIPEModelJSON(t, current))
			}
			require.Equal(t, http.MethodPatch, req.Method)
			require.Equal(t, *current.Etag, req.Header.Get("If-Match"))
			var patch map[string]json.RawMessage
			require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
			require.Len(t, patch, 1, "PATCH must contain tags only")
			var tags map[string]*string
			require.NoError(t, json.Unmarshal(patch["tags"], &tags))
			if conflict {
				return gcResponse(req, http.StatusPreconditionFailed, `{"error":{"code":"PreconditionFailed"}}`)
			}
			writes++
			current.Tags = tags
			current.Etag = to.Ptr(fmt.Sprintf(`"etag-%d"`, writes+1))
			return gcResponse(req, 200, aclIPEModelJSON(t, current))
		}),
	}}}
	client, err := armcompute.NewVirtualMachineScaleSetsClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSS: client}
	require.NoError(t, aclIPEPatch(t.Context(), s, original, tagged))
	require.NoError(t, aclIPEVerifyModel(t.Context(), s, original, tagged))
	conflict = true
	require.ErrorContains(t, aclIPERestoreModel(t.Context(), s, original, tagged), "412")
	require.Equal(t, 1, writes)
	conflict = false
	require.NoError(t, aclIPERestoreModel(t.Context(), s, original, tagged))
	require.True(t, reflect.DeepEqual(current.Tags, original.Tags))
	require.Equal(t, 2, writes)
	current.Tags = aclIPECopyTags(tagged)
	current.Tags["concurrent"] = to.Ptr("change")
	require.ErrorContains(t, aclIPERestoreModel(t.Context(), s, original, tagged), "concurrent tag changes")
	require.Equal(t, 2, writes)
}

func TestACLIPETransitionCleanupFailureStillDeletes(t *testing.T) {
	s, model := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	deleted := false
	client, err := armcompute.NewVirtualMachineScaleSetsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				if req.URL.Path == "/operations/delete" {
					return gcResponse(req, 200, `{"status":"Succeeded"}`)
				}
				if deleted {
					return gcResponse(req, http.StatusNotFound, `{}`)
				}
				if req.Method == http.MethodDelete {
					deleted = true
					response := gcResponse(req, http.StatusAccepted, "")
					response.Header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/delete")
					return response
				}
				return gcResponse(req, 200, aclIPEModelJSON(t, model))
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSS: client}
	oldKeep := config.Config.KeepVMSS
	config.Config.KeepVMSS = false
	t.Cleanup(func() { config.Config.KeepVMSS = oldKeep })
	s.Runtime.VM.ipeTransitionCleanup = func(context.Context) error { return errors.New("tag restoration refused") }
	err = aclIPEFinishCleanup(t.Context(), s, s.Runtime.VM)
	require.ErrorContains(t, err, "tag restoration refused")
	require.True(t, deleted)
}

func TestACLIPETransitionNoPropagatedInstanceTag(t *testing.T) {
	s, _ := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	client, err := armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.Equal(t, http.MethodGet, req.Method)
				return gcResponse(req, 200, `{"id":"`+aclIPETestVMSS+`/virtualMachines/0","instanceId":"0","tags":{"owner":"e2e"}}`)
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSSVM: client}
	require.NoError(t, aclIPEReadUntaggedInstance(t.Context(), s, "0"))
	require.ErrorContains(t, aclIPEReadAuditInstance(t.Context(), s), "did not propagate")
}

func TestACLIPETransitionPropagatedInstanceTag(t *testing.T) {
	s, _ := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	client, err := armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				return gcResponse(req, 200, `{"tags":{"owner":"e2e","acl-node-security-profile":"ipe=audit"}}`)
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSSVM: client}
	require.NoError(t, aclIPEReadAuditInstance(t.Context(), s))
}

func TestACLIPETransitionRejectsTaggedFirstBootInstance(t *testing.T) {
	s, _ := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	client, err := armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.Equal(t, http.MethodGet, req.Method)
				return gcResponse(req, 200, `{"tags":{"acl-node-security-profile":"ipe=off"}}`)
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSSVM: client}
	require.ErrorContains(t, aclIPEReadUntaggedInstance(t.Context(), s, "0"), "unexpected ACL IPE security profile tag")
}

func TestACLIPETransitionDeletionFailureReported(t *testing.T) {
	s, model := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	client, err := armcompute.NewVirtualMachineScaleSetsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				if req.Method == http.MethodDelete {
					return gcResponse(req, http.StatusForbidden, `{"error":{"code":"AuthorizationFailed"}}`)
				}
				require.Equal(t, http.MethodGet, req.Method)
				return gcResponse(req, 200, aclIPEModelJSON(t, model))
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSS: client}
	oldKeep := config.Config.KeepVMSS
	config.Config.KeepVMSS = false
	t.Cleanup(func() { config.Config.KeepVMSS = oldKeep })
	s.Runtime.VM.ipeTransitionCleanup = func(context.Context) error { return errors.New("restore failed") }
	err = aclIPEFinishCleanup(t.Context(), s, s.Runtime.VM)
	require.ErrorContains(t, err, "restore failed")
	require.ErrorContains(t, err, "AuthorizationFailed")
}

func TestACLIPETransitionRefusesDeleteAfterOwnershipChange(t *testing.T) {
	s, model := aclIPEFixture()
	azure := config.Azure
	t.Cleanup(func() { config.Azure = azure })
	model.Tags = aclIPECopyTags(model.Tags)
	model.Tags["owner"] = to.Ptr("another-user")
	client, err := armcompute.NewVirtualMachineScaleSetsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.Equal(t, http.MethodGet, req.Method, "must not delete a VMSS after owner changes")
				return gcResponse(req, 200, aclIPEModelJSON(t, model))
			}),
		}},
	})
	require.NoError(t, err)
	config.Azure = &config.AzureClient{VMSS: client}
	oldKeep := config.Config.KeepVMSS
	config.Config.KeepVMSS = false
	t.Cleanup(func() { config.Config.KeepVMSS = oldKeep })
	require.ErrorContains(t, deleteOwnedIPEVMSSAndWait(t.Context(), s, s.Runtime.VM), "matching creation receipt")
}

func TestACLIPEPartialCreationDeletionRequiresReceipt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		foreign bool
		readErr bool
	}{
		{name: "partial model is owned"},
		{name: "foreign creation marker", foreign: true},
		{name: "cannot read ownership", readErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, model := aclIPEFixture()
			model.SKU = nil
			model.Properties = &armcompute.VirtualMachineScaleSetProperties{}
			if tc.foreign {
				model.Tags[aclIPECreationTag] = to.Ptr("foreign-token")
			}
			oldAzure, oldKeep := config.Azure, config.Config.KeepVMSS
			t.Cleanup(func() { config.Azure, config.Config.KeepVMSS = oldAzure, oldKeep })
			config.Config.KeepVMSS = false
			deleted := false
			client, err := armcompute.NewVirtualMachineScaleSetsClient("test", nil, &arm.ClientOptions{
				ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
					vmssCreationTestPolicy(func(req *http.Request) *http.Response {
						switch {
						case req.URL.Path == "/operations/delete":
							return gcResponse(req, http.StatusOK, `{"status":"Succeeded"}`)
						case req.Method == http.MethodDelete:
							deleted = true
							response := gcResponse(req, http.StatusAccepted, "")
							response.Header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/delete")
							return response
						case tc.readErr:
							return gcResponse(req, http.StatusForbidden, `{"error":{"code":"AuthorizationFailed"}}`)
						case deleted:
							return gcResponse(req, http.StatusNotFound, `{"error":{"code":"ResourceNotFound"}}`)
						default:
							return gcResponse(req, http.StatusOK, aclIPEModelJSON(t, model))
						}
					}),
				}},
			})
			require.NoError(t, err)
			config.Azure = &config.AzureClient{VMSS: client}
			err = deleteOwnedIPEVMSSAndWait(t.Context(), s, s.Runtime.VM)
			if tc.foreign || tc.readErr {
				require.Error(t, err)
				require.False(t, deleted)
			} else {
				require.NoError(t, err)
				require.True(t, deleted, "provisioning must not require a completed model for owned teardown")
			}
		})
	}
}

func TestACLIPETransitionAuditRebootEvidence(t *testing.T) {
	hash := strings.Repeat("a", 64)
	before := aclIPEBootEvidence{bootID: "0b1ddc3c-e6f5-4cb5-aa1a-644f18d7a1b1", cmdline: "acl.ipe.policy_sha256=" + hash}
	after := aclIPEBootEvidence{bootID: "23e197e3-11ee-4c9b-846d-070700c546e8", cmdline: before.cmdline,
		cache: aclIPEAuditTagValue, journal: strings.Join([]string{
			"acl-ipe-load: Credential SHA-256 verified: " + hash,
			"acl-ipe-load: Loaded policy acl_ipe_boot_policy into kernel IPE.",
			"acl-ipe-load: Using IPE mode 'audit'.",
			"acl-ipe-load: Activated policy acl_ipe_boot_policy.",
		}, "\n")}
	require.NoError(t, aclIPEAuditReboot(before, after))
	after.cache = ""
	require.ErrorContains(t, aclIPEAuditReboot(before, after), "initrd IMDS profile cache")
}
