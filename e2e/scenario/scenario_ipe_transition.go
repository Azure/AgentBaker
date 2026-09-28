package scenario

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/google/uuid"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	aclIPETransitionEnv        = "ACL_IPE_TRANSITION"
	aclIPEApprovedClusterEnv   = "ACL_IPE_TRANSITION_APPROVED_CLUSTER_ID"
	aclIPEAllowedDaemonsetsEnv = "ACL_IPE_TRANSITION_ALLOWED_SYSTEM_DAEMONSETS"
	aclIPETransitionTaintKey   = "e2e.agentbaker.azure.com/ipe-transition"
	aclIPETransitionTaint      = aclIPETransitionTaintKey + "=true:NoSchedule"
	aclIPEAuditTagValue        = "ipe=audit"
	aclIPECreationTag          = "agentbaker-ipe-creation-id"
	aclIPENoncePath            = "/var/lib/acl-ipe-e2e/transition-nonce"
	aclIPETransitionCleanTime  = 15 * time.Minute
)

type aclIPECreationReceipt struct {
	resourceID string
	token      string
}

type aclIPEIdentity struct {
	resourceID, instanceID, vmID, imageID string
	diskConfig                            []byte
	nodeUID                               types.UID
	providerID                            string
	bootID, nonce                         string
	leaseRenew                            time.Time
}

func aclIPETransitionEnabled() bool {
	return os.Getenv(aclIPETransitionEnv) == "off-to-audit" && os.Getenv(aclIPEModeEnv) == "off"
}

func aclIPETransitionGate(s *Scenario) error {
	if s == nil || s.Name != "ACL" || os.Getenv(aclIPETransitionEnv) == "" {
		return nil
	}
	if os.Getenv(aclIPETransitionEnv) != "off-to-audit" || os.Getenv(aclIPEModeEnv) != "off" {
		return fmt.Errorf("%s=off-to-audit requires %s=off on ACL", aclIPETransitionEnv, aclIPEModeEnv)
	}
	if config.Config.KeepVMSS {
		return fmt.Errorf("%s requires KEEP_VMSS=false", aclIPETransitionEnv)
	}
	if config.Config.TestPreProvision || s.VHDCaching {
		return fmt.Errorf("%s requires a directly provisioned ACL VMSS, not a VHD-caching/pre-provision stage", aclIPETransitionEnv)
	}
	if os.Getenv(aclIPEApprovedClusterEnv) == "" {
		return fmt.Errorf("%s requires explicit approval for the exact reused AKS cluster resource ID", aclIPEApprovedClusterEnv)
	}
	if _, err := aclIPEAllowedDaemonsets(); err != nil {
		return err
	}
	return nil
}

func aclIPESelectCluster(ctx context.Context, s *Scenario, approved *Cluster, request ClusterRequest) (*Cluster, error) {
	if s.Name == "ACL" && (approved != nil || os.Getenv(aclIPETransitionEnv) != "") {
		if approved == nil {
			return nil, fmt.Errorf("ACL IPE transition refuses generic cluster create/reconcile fallback")
		}
		return approved, nil
	}
	return s.Config.Cluster(ctx, request)
}

func aclIPEApprovedClusterID(s *Scenario) (*arm.ResourceID, error) {
	id, err := arm.ParseResourceID(os.Getenv(aclIPEApprovedClusterEnv))
	if err != nil {
		return nil, fmt.Errorf("parse approved ACL cluster ID: %w", err)
	}
	if !strings.EqualFold(id.SubscriptionID, config.Config.SubscriptionID) ||
		!strings.EqualFold(id.ResourceGroupName, config.ResourceGroupName(s.Location)) ||
		!regexp.MustCompile(`^abe2e-kubenet-v5-[0-9a-f]{5}$`).MatchString(id.Name) ||
		!strings.EqualFold(os.Getenv(aclIPEApprovedClusterEnv),
			fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s",
				id.SubscriptionID, id.ResourceGroupName, id.Name)) {
		return nil, fmt.Errorf("approved cluster ID must identify the existing E2E kubenet cluster in this subscription, location and resource group")
	}
	return id, nil
}

func aclIPEPrepareApprovedCluster(ctx context.Context, s *Scenario) (*Cluster, error) {
	id, err := aclIPEApprovedClusterID(s)
	if err != nil {
		return nil, err
	}
	if config.Azure == nil || config.Azure.AKS == nil {
		return nil, fmt.Errorf("ACL IPE transition requires the Azure AKS read client")
	}
	response, err := config.Azure.AKS.Get(ctx, id.ResourceGroupName, id.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("read approved existing AKS cluster (no create/reconcile fallback): %w", err)
	}
	model := &response.ManagedCluster
	if model.ID == nil || !strings.EqualFold(*model.ID, os.Getenv(aclIPEApprovedClusterEnv)) ||
		model.Name == nil || *model.Name != id.Name ||
		model.Location == nil || !strings.EqualFold(*model.Location, s.Location) ||
		model.Properties == nil || model.Properties.ProvisioningState == nil ||
		*model.Properties.ProvisioningState != "Succeeded" ||
		model.Properties.NodeResourceGroup == nil || *model.Properties.NodeResourceGroup == "" ||
		model.Properties.CurrentKubernetesVersion == nil ||
		model.Properties.NetworkProfile == nil || model.Properties.NetworkProfile.NetworkPlugin == nil ||
		*model.Properties.NetworkProfile.NetworkPlugin != armcontainerservice.NetworkPluginKubenet {
		return nil, fmt.Errorf("approved AKS cluster identity, location, network, or Succeeded state is not usable")
	}
	subnetID, err := getClusterSubnetID(model)
	if err != nil {
		return nil, err
	}
	vnet, err := getClusterVNet(ctx, model)
	if err != nil {
		return nil, err
	}
	if vnet.name != SharedVNetName || !strings.EqualFold(vnet.resourceGroup, id.ResourceGroupName) {
		return nil, fmt.Errorf("approved AKS cluster is not attached to the expected shared E2E VNet")
	}
	kubeletIdentity, err := getClusterKubeletIdentity(model)
	if err != nil {
		return nil, err
	}
	if kubeletIdentity.ClientID == nil || kubeletIdentity.ResourceID == nil {
		return nil, fmt.Errorf("approved AKS cluster has no kubelet identity credentials")
	}
	identityID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s",
		id.SubscriptionID, id.ResourceGroupName, SharedClusterIdentity)
	found := false
	if model.Identity != nil {
		for key := range model.Identity.UserAssignedIdentities {
			found = found || strings.EqualFold(key, identityID)
		}
	}
	if !found {
		return nil, fmt.Errorf("approved AKS cluster lacks expected E2E managed identity")
	}
	identity, err := config.Azure.UserAssignedIdentities.Get(ctx, id.ResourceGroupName, SharedClusterIdentity, nil)
	if err != nil {
		return nil, fmt.Errorf("read existing E2E cluster identity: %w", err)
	}
	if identity.ID == nil || !strings.EqualFold(*identity.ID, identityID) ||
		identity.Properties == nil || identity.Properties.TenantID == nil || *identity.Properties.TenantID == "" {
		return nil, fmt.Errorf("existing E2E cluster identity is incomplete")
	}
	bastion, err := config.Azure.BastionHosts.Get(ctx, id.ResourceGroupName, SharedBastionName, nil)
	if err != nil {
		return nil, fmt.Errorf("read existing E2E bastion: %w", err)
	}
	if bastion.Properties == nil || bastion.Properties.DNSName == nil || *bastion.Properties.DNSName == "" {
		return nil, fmt.Errorf("existing E2E bastion lacks its DNS name")
	}
	kube, err := getClusterKubeClient(ctx, model)
	if err != nil {
		return nil, err
	}
	params, err := extractClusterParameters(ctx, model, kube)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		Model: model, kubeconfig: kube.KubeConfig, KubeletIdentity: kubeletIdentity,
		SubnetID: subnetID, VNetResourceGUID: vnet.resourceGUID, ClusterParams: params,
		Bastion: NewBastion(config.Azure.Credential, config.Config.SubscriptionID,
			id.ResourceGroupName, *bastion.Properties.DNSName),
		TenantID: *identity.Properties.TenantID,
	}, nil
}

func aclIPEAllowedDaemonsets() (map[string]bool, error) {
	allowed := make(map[string]bool)
	value := os.Getenv(aclIPEAllowedDaemonsetsEnv)
	if value == "" {
		return allowed, nil
	}
	for _, name := range strings.Split(value, ",") {
		if name == "" || strings.TrimSpace(name) != name || !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(name) {
			return nil, fmt.Errorf("%s requires comma-separated exact kube-system DaemonSet names without wildcards", aclIPEAllowedDaemonsetsEnv)
		}
		allowed[name] = true
	}
	return allowed, nil
}

func aclIPEOwnedModel(s *Scenario, model *armcompute.VirtualMachineScaleSet) error {
	if s == nil || s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.VMSS == nil ||
		s.Runtime.VM.VMSS.ID == nil || s.Runtime.VM.VMSS.Tags == nil ||
		s.Runtime.VM.ipeCreationReceipt == nil ||
		s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		model == nil || model.ID == nil || model.Name == nil || model.Etag == nil ||
		!strings.EqualFold(*model.ID, *s.Runtime.VM.VMSS.ID) || *model.Name != s.Runtime.VMSSName ||
		!strings.EqualFold(*model.ID, s.Runtime.VM.ipeCreationReceipt.resourceID) ||
		strings.HasPrefix(strings.ToLower(*model.Name), "aks-") ||
		model.Tags == nil || model.Tags["owner"] == nil ||
		model.Tags[aclIPECreationTag] == nil ||
		*model.Tags[aclIPECreationTag] != s.Runtime.VM.ipeCreationReceipt.token ||
		s.Runtime.VM.VMSS.Tags["owner"] == nil ||
		*model.Tags["owner"] != *s.Runtime.VM.VMSS.Tags["owner"] ||
		*model.Tags["owner"] == "" || *model.Tags["owner"] == "unknown" {
		return fmt.Errorf("ACL IPE transition requires the exact newly created, owned scenario VMSS and ETag")
	}
	if model.SKU == nil || model.SKU.Capacity == nil || *model.SKU.Capacity != 1 ||
		model.Properties == nil || model.Properties.Overprovision == nil || *model.Properties.Overprovision ||
		model.Properties.UpgradePolicy == nil || model.Properties.UpgradePolicy.Mode == nil ||
		*model.Properties.UpgradePolicy.Mode != armcompute.UpgradeModeManual {
		return fmt.Errorf("ACL IPE transition requires one instance, no overprovisioning, and effective Manual upgrade policy")
	}
	for key := range model.Tags {
		if strings.EqualFold(key, "aks-managed-poolName") {
			return fmt.Errorf("ACL IPE transition refuses an AKS-managed pool VMSS")
		}
	}
	if model.Properties.VirtualMachineProfile == nil ||
		model.Properties.VirtualMachineProfile.StorageProfile == nil ||
		model.Properties.VirtualMachineProfile.StorageProfile.ImageReference == nil ||
		model.Properties.VirtualMachineProfile.StorageProfile.ImageReference.ID == nil ||
		!strings.Contains(strings.ToLower(*model.Properties.VirtualMachineProfile.StorageProfile.ImageReference.ID), "/versions/") ||
		model.Properties.VirtualMachineProfile.StorageProfile.OSDisk == nil ||
		model.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings == nil ||
		model.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings.Option == nil ||
		*model.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings.Option != armcompute.DiffDiskOptionsLocal {
		return fmt.Errorf("ACL IPE transition requires pinned image and known local ephemeral OS disk configuration")
	}
	return nil
}

func aclIPEActionMatches(pattern, action string) bool {
	pattern = regexp.QuoteMeta(strings.ToLower(pattern))
	pattern = strings.ReplaceAll(pattern, `\*`, ".*")
	return regexp.MustCompile("^" + pattern + "$").MatchString(strings.ToLower(action))
}

func aclIPEAllows(permissions []*armauthorization.Permission, action string) bool {
	for _, permission := range permissions {
		if permission == nil || permission.Condition != nil {
			continue
		}
		allowed := false
		for _, pattern := range permission.Actions {
			if pattern != nil && aclIPEActionMatches(*pattern, action) {
				allowed = true
			}
		}
		for _, pattern := range permission.NotActions {
			if pattern != nil && aclIPEActionMatches(*pattern, action) {
				allowed = false
			}
		}
		if allowed {
			return true
		}
	}
	return false
}

func aclIPECheckComputeRights(ctx context.Context, s *Scenario) error {
	if config.Azure == nil || config.Azure.Credential == nil || config.Config.SubscriptionID == "" {
		return fmt.Errorf("ACL IPE transition cannot establish effective Compute permissions without Azure CLI credential/subscription")
	}
	client, err := armauthorization.NewPermissionsClient(config.Config.SubscriptionID, config.Azure.Credential, config.Azure.ArmOptions)
	if err != nil {
		return fmt.Errorf("create effective permissions client: %w", err)
	}
	pager := client.NewListForResourcePager(*s.Runtime.Cluster.Model.Properties.NodeResourceGroup, "Microsoft.Compute", "",
		"virtualMachineScaleSets", s.Runtime.VMSSName, nil)
	var permissions []*armauthorization.Permission
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("read effective Compute permissions for owned VMSS: %w", err)
		}
		permissions = append(permissions, page.Value...)
	}
	for _, action := range []string{
		"Microsoft.Compute/virtualMachineScaleSets/read",
		"Microsoft.Compute/virtualMachineScaleSets/write",
		"Microsoft.Compute/virtualMachineScaleSets/delete",
		"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read",
		"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/restart/action",
	} {
		if !aclIPEAllows(permissions, action) {
			return fmt.Errorf("caller lacks effective %s on owned VMSS", action)
		}
	}
	return nil
}

func aclIPECheckKubeRights(ctx context.Context, s *Scenario) error {
	for _, access := range []authorizationv1.ResourceAttributes{
		{Resource: "nodes", Verb: "get"}, {Resource: "nodes", Verb: "list"},
		{Resource: "leases", Group: "coordination.k8s.io", Namespace: "kube-node-lease", Verb: "get"},
		{Resource: "pods", Verb: "list"},
		{Resource: "pods", Namespace: defaultNamespace, Verb: "create"},
		{Resource: "pods", Namespace: defaultNamespace, Verb: "get"},
		{Resource: "pods", Namespace: defaultNamespace, Verb: "delete"},
	} {
		response, err := s.Runtime.Kube.Typed.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx,
			&authorizationv1.SelfSubjectAccessReview{Spec: authorizationv1.SelfSubjectAccessReviewSpec{ResourceAttributes: &access}},
			metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("verify Kubernetes %s %s/%s: %w", access.Verb, access.Group, access.Resource, err)
		}
		if !response.Status.Allowed || response.Status.Denied || response.Status.EvaluationError != "" {
			return fmt.Errorf("caller lacks effective Kubernetes %s %s/%s: %s", access.Verb, access.Group, access.Resource, response.Status.Reason)
		}
	}
	return nil
}

func aclIPEPreflight(ctx context.Context, s *Scenario, model *armcompute.VirtualMachineScaleSet) error {
	if err := aclIPETransitionGate(s); err != nil {
		return err
	}
	if s.Runtime.Cluster.Model.ID == nil ||
		!strings.EqualFold(os.Getenv(aclIPEApprovedClusterEnv), *s.Runtime.Cluster.Model.ID) {
		return fmt.Errorf("approved cluster ID does not match the reused AKS cluster")
	}
	if err := aclIPEOwnedModel(s, model); err != nil {
		return err
	}
	caller, err := getLoggedInAzUser(ctx)
	if err != nil {
		return fmt.Errorf("read Azure CLI caller for scenario VMSS ownership: %w", err)
	}
	if strings.TrimSpace(caller) == "" || !strings.EqualFold(strings.TrimSpace(caller), strings.TrimSpace(*model.Tags["owner"])) {
		return fmt.Errorf("Azure CLI caller does not match scenario VMSS owner tag")
	}
	if err := aclIPECheckComputeRights(ctx, s); err != nil {
		return err
	}
	return aclIPECheckKubeRights(ctx, s)
}

func aclIPECopyTags(tags map[string]*string) map[string]*string {
	copy := make(map[string]*string, len(tags))
	for key, value := range tags {
		if value == nil {
			copy[key] = nil
		} else {
			copy[key] = to.Ptr(*value)
		}
	}
	return copy
}

func aclIPETaggedModel(tags map[string]*string) (map[string]*string, error) {
	if err := validateACLIPEVMSSNoProfileTag(tags); err != nil {
		return nil, err
	}
	copy := aclIPECopyTags(tags)
	copy[aclIPESecurityProfileTag] = to.Ptr(aclIPEAuditTagValue)
	return copy, nil
}

func aclIPEModelSameExceptTags(before, after armcompute.VirtualMachineScaleSet) (bool, error) {
	before.Tags, after.Tags = nil, nil
	before.Etag, after.Etag = nil, nil
	oldJSON, err := json.Marshal(before)
	if err != nil {
		return false, err
	}
	newJSON, err := json.Marshal(after)
	if err != nil {
		return false, err
	}
	return bytes.Equal(oldJSON, newJSON), nil
}

func aclIPEPrepareCreation(ctx context.Context, s *Scenario, model *armcompute.VirtualMachineScaleSet) error {
	if config.Config.KeepVMSS || s.Runtime == nil || s.Runtime.Cluster == nil ||
		s.Runtime.Cluster.Model == nil || s.Runtime.Cluster.Model.Properties == nil ||
		s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil ||
		config.Config.SubscriptionID == "" || s.Runtime.VMSSName == "" ||
		strings.HasPrefix(strings.ToLower(s.Runtime.VMSSName), "aks-") ||
		model.Tags == nil || model.Tags["owner"] == nil ||
		*model.Tags["owner"] == "" || *model.Tags["owner"] == "unknown" ||
		model.Tags[aclIPECreationTag] != nil {
		return fmt.Errorf("ACL IPE creation requires a new scenario VMSS, valid owner and KEEP_VMSS=false")
	}
	_, err := config.Azure.VMSS.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, nil)
	if err == nil {
		return fmt.Errorf("refuse ACL IPE create-or-update: VMSS name %q already exists", s.Runtime.VMSSName)
	}
	var responseErr *azcore.ResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusNotFound {
		return fmt.Errorf("cannot establish absent ACL IPE VMSS before creation: %w", err)
	}
	token, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate ACL IPE VMSS creation tag: %w", err)
	}
	model.Tags[aclIPECreationTag] = to.Ptr(token.String())
	return nil
}

func aclIPEVerifyCreationReceipt(s *Scenario, model *armcompute.VirtualMachineScaleSet, tags map[string]*string, receipt *aclIPECreationReceipt) error {
	if model == nil || receipt == nil || model.ID == nil || model.Name == nil ||
		model.Tags == nil || model.Tags["owner"] == nil || tags["owner"] == nil ||
		model.Tags[aclIPECreationTag] == nil || tags[aclIPECreationTag] == nil ||
		*model.Name != s.Runtime.VMSSName ||
		!strings.EqualFold(*model.ID, receipt.resourceID) ||
		*model.Tags[aclIPECreationTag] != receipt.token ||
		*tags[aclIPECreationTag] != receipt.token ||
		*model.Tags["owner"] != *tags["owner"] {
		return fmt.Errorf("VMSS ID, name, owner or unique creation tag does not match ACL IPE creation receipt")
	}
	return nil
}

func aclIPERetainCreationReceipt(ctx context.Context, s *Scenario, vm *ScenarioVM, model armcompute.VirtualMachineScaleSet) error {
	rg := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup
	receipt := &aclIPECreationReceipt{
		resourceID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s",
			config.Config.SubscriptionID, rg, s.Runtime.VMSSName),
		token: *model.Tags[aclIPECreationTag],
	}
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		response, err := config.Azure.VMSS.Get(readCtx, rg, s.Runtime.VMSSName, nil)
		if err == nil {
			if err := aclIPEVerifyCreationReceipt(s, &response.VirtualMachineScaleSet, model.Tags, receipt); err != nil {
				return fmt.Errorf("refuse ownership of ambiguous ACL IPE VMSS: %w", err)
			}
			vm.ipeCreationReceipt = receipt
			vm.VMSS = &response.VirtualMachineScaleSet
			return nil
		}
		var responseErr *azcore.ResponseError
		if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusNotFound {
			return fmt.Errorf("cannot verify ACL IPE VMSS creation ownership; no deletion permitted: %w", err)
		}
		select {
		case <-readCtx.Done():
			return fmt.Errorf("ACL IPE VMSS creation ownership not visible within 90s; no deletion permitted: %w", readCtx.Err())
		case <-ticker.C:
		}
	}
}

func aclIPEGetModel(ctx context.Context, s *Scenario) (armcompute.VirtualMachineScaleSet, error) {
	if s == nil || s.Runtime == nil || s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		s.Runtime.Cluster.Model.Properties == nil || s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil {
		return armcompute.VirtualMachineScaleSet{}, fmt.Errorf("ACL IPE transition has no scenario node resource group")
	}
	resp, err := config.Azure.VMSS.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, nil)
	if err != nil {
		return armcompute.VirtualMachineScaleSet{}, fmt.Errorf("get owned ACL VMSS: %w", err)
	}
	if err := aclIPEOwnedModel(s, &resp.VirtualMachineScaleSet); err != nil {
		return armcompute.VirtualMachineScaleSet{}, err
	}
	return resp.VirtualMachineScaleSet, nil
}

func aclIPEPatch(ctx context.Context, s *Scenario, model armcompute.VirtualMachineScaleSet, tags map[string]*string) error {
	poller, err := config.Azure.VMSS.BeginUpdate(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName,
		armcompute.VirtualMachineScaleSetUpdate{Tags: aclIPECopyTags(tags)},
		&armcompute.VirtualMachineScaleSetsClientBeginUpdateOptions{IfMatch: model.Etag})
	if err != nil {
		return fmt.Errorf("ETag-conditional tag-only VMSS PATCH: %w", err)
	}
	if _, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions()); err != nil {
		return fmt.Errorf("wait for VMSS tag PATCH: %w", err)
	}
	return nil
}

func aclIPEVerifyModel(ctx context.Context, s *Scenario, before armcompute.VirtualMachineScaleSet, tags map[string]*string) error {
	current, err := aclIPEGetModel(ctx, s)
	if err != nil {
		return err
	}
	same, err := aclIPEModelSameExceptTags(before, current)
	if err != nil {
		return err
	}
	if !same || !reflect.DeepEqual(current.Tags, tags) {
		return fmt.Errorf("scenario VMSS model changed beyond requested IPE profile tag")
	}
	return nil
}

func aclIPERestoreModel(ctx context.Context, s *Scenario, before armcompute.VirtualMachineScaleSet, tagged map[string]*string) error {
	current, err := aclIPEGetModel(ctx, s)
	if err != nil {
		return err
	}
	same, err := aclIPEModelSameExceptTags(before, current)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("refuse ACL IPE tag rollback: owned VMSS model changed")
	}
	if reflect.DeepEqual(current.Tags, before.Tags) {
		return nil
	}
	if !reflect.DeepEqual(current.Tags, tagged) {
		return fmt.Errorf("refuse ACL IPE tag rollback: concurrent tag changes")
	}
	if err := aclIPEPatch(ctx, s, current, before.Tags); err != nil {
		return fmt.Errorf("restore original VMSS tags: %w", err)
	}
	return aclIPEVerifyModel(ctx, s, before, before.Tags)
}

func deleteOwnedIPEVMSSAndWait(ctx context.Context, s *Scenario, vm *ScenarioVM) error {
	if config.Config.KeepVMSS || vm == nil || vm.ipeCreationReceipt == nil || vm.VMSS == nil || vm.VMSS.ID == nil {
		return fmt.Errorf("refuse ACL IPE deletion without verified creation receipt or with KEEP_VMSS")
	}
	rg := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup
	current, err := config.Azure.VMSS.Get(ctx, rg, s.Runtime.VMSSName, nil)
	if err != nil {
		var responseErr *azcore.ResponseError
		if errors.As(err, &responseErr) && responseErr.StatusCode == 404 {
			return nil
		}
		return fmt.Errorf("read VMSS before owned ACL IPE deletion: %w", err)
	}
	if err := aclIPEVerifyCreationReceipt(s, &current.VirtualMachineScaleSet, vm.VMSS.Tags, vm.ipeCreationReceipt); err != nil {
		return fmt.Errorf("refuse to delete VMSS without matching creation receipt: %w", err)
	}
	if !strings.EqualFold(*vm.VMSS.ID, vm.ipeCreationReceipt.resourceID) {
		return fmt.Errorf("refuse to delete VMSS whose initial ID differs from its creation receipt")
	}
	for key := range current.Tags {
		if strings.EqualFold(key, "aks-managed-poolName") {
			return fmt.Errorf("refuse to delete an AKS-managed pool VMSS")
		}
	}
	poller, err := config.Azure.VMSS.BeginDelete(ctx, rg, s.Runtime.VMSSName,
		&armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{ForceDeletion: to.Ptr(true)})
	if err != nil {
		return fmt.Errorf("begin deleting owned ACL VMSS: %w", err)
	}
	if _, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions()); err != nil {
		return fmt.Errorf("wait for owned ACL VMSS deletion: %w", err)
	}
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := config.Azure.VMSS.Get(ctx, rg, s.Runtime.VMSSName, nil)
		var responseErr *azcore.ResponseError
		if errors.As(err, &responseErr) && responseErr.StatusCode == 404 {
			return true, nil
		}
		return false, err
	})
}

func aclIPEFinishCleanup(ctx context.Context, s *Scenario, vm *ScenarioVM) error {
	restoreCtx, cancelRestore := context.WithTimeout(ctx, 3*time.Minute)
	restoreErr := runWithPanicRecovery(restoreCtx, vm.ipeTransitionCleanup)
	cancelRestore()
	deleteCtx, cancelDelete := context.WithTimeout(ctx, 7*time.Minute)
	defer cancelDelete()
	deleteErr := deleteOwnedIPEVMSSAndWait(deleteCtx, s, vm)
	return errors.Join(restoreErr, deleteErr)
}

func aclIPEArmOwnedCleanup(s *Scenario, vm *ScenarioVM) {
	s.cleanup.timeout = aclIPETransitionCleanTime
	vm.ipeTransitionCleanup = func(context.Context) error { return nil }
}

func aclIPEArmTagRestore(s *Scenario, before armcompute.VirtualMachineScaleSet, tagged map[string]*string) error {
	if s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.ipeTransitionCleanup == nil {
		return fmt.Errorf("ACL IPE transition cannot PATCH without armed owned-VMSS cleanup")
	}
	s.Runtime.VM.ipeTransitionCleanup = func(cleanCtx context.Context) error {
		return aclIPERestoreModel(cleanCtx, s, before, tagged)
	}
	return nil
}

func aclIPEVM(ctx context.Context, s *Scenario, model armcompute.VirtualMachineScaleSet) (aclIPEIdentity, error) {
	var id aclIPEIdentity
	rg := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup
	pager := config.Azure.VMSSVM.NewListPager(rg, s.Runtime.VMSSName, nil)
	count := 0
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return id, fmt.Errorf("list owned VMSS instances: %w", err)
		}
		count += len(page.Value)
	}
	if count != 1 {
		return id, fmt.Errorf("ACL IPE transition requires exactly one owned VMSS instance, got %d", count)
	}
	initial := s.Runtime.VM.VM
	if initial == nil || initial.ID == nil || initial.InstanceID == nil ||
		initial.Properties == nil || initial.Properties.VMID == nil {
		return id, fmt.Errorf("missing provisioned scenario instance identity")
	}
	response, err := config.Azure.VMSSVM.Get(ctx, rg, s.Runtime.VMSSName, *initial.InstanceID, nil)
	if err != nil {
		return id, fmt.Errorf("get owned VMSS instance: %w", err)
	}
	vm := response.VirtualMachineScaleSetVM
	image := *model.Properties.VirtualMachineProfile.StorageProfile.ImageReference.ID
	if vm.ID == nil || vm.InstanceID == nil || vm.Properties == nil || vm.Properties.VMID == nil ||
		vm.Properties.StorageProfile == nil || vm.Properties.StorageProfile.ImageReference == nil ||
		vm.Properties.StorageProfile.ImageReference.ID == nil ||
		!strings.EqualFold(*vm.Properties.StorageProfile.ImageReference.ID, image) ||
		vm.Properties.StorageProfile.OSDisk == nil ||
		!strings.EqualFold(*vm.ID, *initial.ID) || *vm.InstanceID != *initial.InstanceID ||
		!strings.EqualFold(*vm.Properties.VMID, *initial.Properties.VMID) {
		return id, fmt.Errorf("scenario VM instance/image differs from provisioned pinned image")
	}
	if _, err := uuid.Parse(*vm.Properties.VMID); err != nil {
		return id, fmt.Errorf("invalid scenario VM vmId: %w", err)
	}
	disk, err := json.Marshal(vm.Properties.StorageProfile.OSDisk)
	if err != nil || len(disk) <= 2 {
		return id, fmt.Errorf("missing effective ephemeral OS disk configuration: %v", err)
	}
	id.resourceID, id.instanceID, id.vmID, id.imageID, id.diskConfig =
		*vm.ID, *vm.InstanceID, *vm.Properties.VMID, image, disk
	return id, nil
}

func aclIPEReadUntaggedInstance(ctx context.Context, s *Scenario, instanceID string) error {
	vm, err := config.Azure.VMSSVM.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
		s.Runtime.VMSSName, instanceID, nil)
	if err != nil {
		return fmt.Errorf("read fresh first-boot VMSS instance tags: %w", err)
	}
	return validateACLIPEVMSSNoProfileTag(vm.Tags)
}

func aclIPEInstanceUnchanged(before, after aclIPEIdentity) error {
	if !strings.EqualFold(before.resourceID, after.resourceID) ||
		before.instanceID != after.instanceID || !strings.EqualFold(before.vmID, after.vmID) ||
		!strings.EqualFold(before.imageID, after.imageID) ||
		!bytes.Equal(before.diskConfig, after.diskConfig) ||
		before.nonce == "" || before.nonce != after.nonce ||
		before.nodeUID == "" || before.nodeUID != after.nodeUID ||
		!strings.EqualFold(before.providerID, after.providerID) {
		return fmt.Errorf("ACL IPE transition changed instance, image, OS disk configuration, guest nonce, or Kubernetes Node identity")
	}
	if before.bootID == "" || before.bootID == after.bootID {
		return fmt.Errorf("ACL IPE transition did not observe a new boot ID")
	}
	if before.leaseRenew.IsZero() || !after.leaseRenew.After(before.leaseRenew) {
		return fmt.Errorf("ACL IPE transition did not observe a fresh Kubernetes Node Lease")
	}
	return nil
}

var errACLIPEAwaitingNode = errors.New("awaiting scenario node recovery")

func aclIPENode(ctx context.Context, s *Scenario, id *aclIPEIdentity) error {
	nodeName := s.Runtime.VM.KubeName
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%w: node not registered: %v", errACLIPEAwaitingNode, err)
		}
		return fmt.Errorf("read exact scenario node: %w", err)
	}
	id.nodeUID, id.providerID = node.UID, node.Spec.ProviderID
	if id.nodeUID == "" || !strings.HasSuffix(strings.ToLower(id.providerID), strings.ToLower(id.resourceID)) {
		return fmt.Errorf("Kubernetes Node UID/providerID do not identify the owned VMSS instance")
	}
	ready := false
	for _, condition := range node.Status.Conditions {
		ready = ready || condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue
	}
	if !ready {
		return fmt.Errorf("%w: exact scenario Kubernetes Node is not Ready", errACLIPEAwaitingNode)
	}
	tainted := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == aclIPETransitionTaintKey && taint.Value == "true" && taint.Effect == corev1.TaintEffectNoSchedule {
			tainted = true
		}
	}
	if !tainted {
		return fmt.Errorf("scenario Node lost its pre-registration IPE isolation taint")
	}
	lease, err := s.Runtime.Kube.Typed.CoordinationV1().Leases("kube-node-lease").Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%w: Node Lease not present: %v", errACLIPEAwaitingNode, err)
		}
		return fmt.Errorf("read exact scenario Node Lease: %w", err)
	}
	if lease.Spec.HolderIdentity == nil || lease.Spec.RenewTime == nil {
		return fmt.Errorf("%w: missing holder or renewTime on scenario Node Lease", errACLIPEAwaitingNode)
	}
	if *lease.Spec.HolderIdentity != nodeName {
		return fmt.Errorf("scenario Node Lease holder does not match the owned node")
	}
	id.leaseRenew = lease.Spec.RenewTime.Time
	if id.leaseRenew.IsZero() || time.Since(id.leaseRenew) > 2*time.Minute ||
		time.Until(id.leaseRenew) > 30*time.Second {
		return fmt.Errorf("%w: scenario Node Lease is not fresh", errACLIPEAwaitingNode)
	}
	return nil
}

func aclIPEWaitForNodeAfterBoot(ctx context.Context, s *Scenario, before aclIPEIdentity, bootObservedAt time.Time, after *aclIPEIdentity) error {
	return aclIPEPollNodeAfterBoot(ctx, s, before, bootObservedAt, after, 5*time.Second, 5*time.Minute)
}

func aclIPEPollNodeAfterBoot(ctx context.Context, s *Scenario, before aclIPEIdentity, bootObservedAt time.Time, after *aclIPEIdentity, interval, deadline time.Duration) error {
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, interval, deadline, true, func(ctx context.Context) (bool, error) {
		current := *after
		if err := aclIPENode(ctx, s, &current); err != nil {
			if errors.Is(err, errACLIPEAwaitingNode) {
				lastErr = err
				return false, nil
			}
			return false, err
		}
		if current.nodeUID != before.nodeUID || !strings.EqualFold(current.providerID, before.providerID) {
			return false, fmt.Errorf("recovered ACL node UID/providerID differs from original node")
		}
		if !current.leaseRenew.After(bootObservedAt) || !current.leaseRenew.After(before.leaseRenew) {
			lastErr = fmt.Errorf("Node Lease renewTime %s must be later than observed new boot %s",
				current.leaseRenew, bootObservedAt)
			return false, nil
		}
		*after = current
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("ACL IPE same-node recovery and post-boot Lease failed (last observation: %v): %w", lastErr, err)
	}
	return nil
}

func aclIPEAllowedWorkloads(pods []corev1.Pod, nodeName string, allowed map[string]bool) error {
	for _, pod := range pods {
		if pod.Spec.NodeName != nodeName {
			return fmt.Errorf("pod inventory returned unexpected node %q", pod.Spec.NodeName)
		}
		systemDaemon := false
		for _, owner := range pod.OwnerReferences {
			if pod.Namespace == metav1.NamespaceSystem && allowed[owner.Name] && owner.Kind == "DaemonSet" &&
				owner.Controller != nil && *owner.Controller {
				systemDaemon = true
			}
		}
		if !systemDaemon {
			return fmt.Errorf("unexpected workload on isolated ACL node before restart: %s/%s (%s)", pod.Namespace, pod.Name, pod.Status.Phase)
		}
	}
	return nil
}

func aclIPECheckWorkloads(ctx context.Context, s *Scenario) error {
	nodeName := s.Runtime.VM.KubeName
	allowed, err := aclIPEAllowedDaemonsets()
	if err != nil {
		return err
	}
	pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(corev1.NamespaceAll).List(ctx,
		metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String()})
	if err != nil {
		return fmt.Errorf("list all workloads on isolated ACL node: %w", err)
	}
	return aclIPEAllowedWorkloads(pods.Items, nodeName, allowed)
}

func aclIPEProbePod(ctx context.Context, s *Scenario, after time.Time) (types.UID, error) {
	created, running, err := startPodAndCheckItRunsDetailed(ctx, s, podHTTPServerLinux(s))
	if err != nil {
		return "", err
	}
	return aclIPEProbePodEvidence(created, running, s.Runtime.VM.KubeName, after)
}

func aclIPEProbePodEvidence(created, pod *corev1.Pod, nodeName string, after time.Time) (types.UID, error) {
	ready := false
	if pod != nil {
		for _, condition := range pod.Status.Conditions {
			ready = ready || condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
		}
	}
	if created == nil || created.UID == "" || pod == nil || pod.UID != created.UID ||
		pod.Name != created.Name || pod.Namespace != created.Namespace ||
		!ready || pod.Status.Phase != corev1.PodRunning || pod.Spec.NodeName != nodeName ||
		created.CreationTimestamp.IsZero() || pod.CreationTimestamp.IsZero() ||
		!pod.CreationTimestamp.Equal(&created.CreationTimestamp) ||
		created.CreationTimestamp.Time.Before(after.Truncate(time.Second)) {
		return "", fmt.Errorf("fresh targeted ACL probe pod UID, creation time, or exact node is missing or changed")
	}
	return pod.UID, nil
}

func aclIPEReadBoot(ctx context.Context, s *Scenario) (aclIPEBootEvidence, error) {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		aclIPEPrivilegedProbe(aclIPEBootEvidenceProbe), 0, "read ACL IPE current-boot evidence")
	if err != nil {
		return aclIPEBootEvidence{}, err
	}
	return parseACLIPEBootEvidence(result.stdout)
}

const aclIPENonceCreateProbe = `set -euo pipefail
fstype=$(findmnt -n -o FSTYPE -T /var/lib)
[[ -n "$fstype" && "$fstype" != "tmpfs" && "$fstype" != "ramfs" && "$fstype" != "overlay" ]] ||
  { echo "ACL IPE nonce requires a verified disk-backed /var/lib mount" >&2; exit 1; }
install -d -m 0700 /var/lib/acl-ipe-e2e
path=/var/lib/acl-ipe-e2e/transition-nonce
[[ ! -e "$path" ]] || { echo "ACL IPE nonce already exists" >&2; exit 1; }
nonce=$(cat /proc/sys/kernel/random/uuid)
umask 077
printf '%s\n' "$nonce" > "$path"
sync -f "$path"
printf '%s\n' "$nonce"`

func aclIPECreateNonce(ctx context.Context, s *Scenario) (string, error) {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		aclIPEPrivilegedProbe(aclIPENonceCreateProbe), 0, "create disk-backed ACL IPE transition nonce")
	if err != nil {
		return "", err
	}
	nonce := strings.TrimSpace(result.stdout)
	if _, err := uuid.Parse(nonce); err != nil {
		return "", fmt.Errorf("invalid guest transition nonce: %w", err)
	}
	return nonce, nil
}

func aclIPEReadNonce(ctx context.Context, s *Scenario) (string, error) {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		aclIPEPrivilegedProbe("set -euo pipefail\ncat "+aclIPENoncePath), 0, "read disk-backed ACL IPE transition nonce")
	if err != nil {
		return "", err
	}
	nonce := strings.TrimSpace(result.stdout)
	if _, err := uuid.Parse(nonce); err != nil {
		return "", fmt.Errorf("invalid persisted guest nonce: %w", err)
	}
	return nonce, nil
}

func aclIPEReadIMDS(ctx context.Context, s *Scenario, id aclIPEIdentity, audit bool) error {
	for _, item := range []struct{ field, want string }{
		{"vmId", id.vmID}, {"vmScaleSetName", s.Runtime.VMSSName}, {"resourceId", id.resourceID},
	} {
		endpoint := fmt.Sprintf("http://169.254.169.254/metadata/instance/compute/%s?api-version=2021-02-01&format=text", item.field)
		result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			fmt.Sprintf("curl --noproxy '*' --connect-timeout 5 --max-time 10 --retry 2 -fsS -H 'Metadata: true' '%s'", endpoint),
			0, "read scenario IMDS "+item.field)
		if err != nil {
			return err
		}
		if item.field == "resourceId" {
			if !strings.EqualFold(strings.TrimSpace(result.stdout), item.want) {
				return fmt.Errorf("IMDS resourceId differs from owned VMSS instance")
			}
		} else if err := validateACLIPEIMDSField(item.field, result.stdout, item.want); err != nil {
			return err
		}
	}
	tags, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"curl --noproxy '*' --connect-timeout 5 --max-time 10 --retry 2 -fsS -H 'Metadata: true' 'http://169.254.169.254/metadata/instance/compute/tagsList?api-version=2021-02-01'",
		0, "read live ACL IMDS tagsList")
	if err != nil {
		return err
	}
	if !audit {
		return validateACLIPEIMDSTags(tags.stdout)
	}
	var entries []struct {
		Name, Value *string
	}
	if err := json.Unmarshal([]byte(tags.stdout), &entries); err != nil || entries == nil {
		return fmt.Errorf("invalid live ACL IMDS tag list after restart: %v", err)
	}
	count := 0
	for _, tag := range entries {
		if tag.Name == nil || tag.Value == nil {
			return fmt.Errorf("malformed live ACL IMDS tag")
		}
		if strings.EqualFold(*tag.Name, aclIPESecurityProfileTag) {
			count++
			if *tag.Value != aclIPEAuditTagValue {
				return fmt.Errorf("IMDS IPE profile is %q, expected %q", *tag.Value, aclIPEAuditTagValue)
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("existing VMSS instance did not receive %s=%s after restart; manualUpgrade not attempted",
			aclIPESecurityProfileTag, aclIPEAuditTagValue)
	}
	return nil
}

func aclIPEReadAuditInstance(ctx context.Context, s *Scenario) error {
	vm, err := config.Azure.VMSSVM.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
		s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, nil)
	if err != nil {
		return fmt.Errorf("read VMSS instance tags after restart: %w", err)
	}
	count := 0
	for key, value := range vm.Tags {
		if strings.EqualFold(key, aclIPESecurityProfileTag) {
			count++
			if value == nil || *value != aclIPEAuditTagValue {
				return fmt.Errorf("VMSS instance security profile tag differs from ipe=audit")
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("VMSS instance tags did not propagate %s=%s after restart; no instance upgrade attempted",
			aclIPESecurityProfileTag, aclIPEAuditTagValue)
	}
	return nil
}

func aclIPEAuditReboot(before, after aclIPEBootEvidence) error {
	if _, err := uuid.Parse(after.bootID); err != nil || after.bootID == before.bootID {
		return fmt.Errorf("ACL IPE audit transition needs a new boot ID")
	}
	if after.cache != aclIPEAuditTagValue {
		return fmt.Errorf("initrd IMDS profile cache is %q, expected %q", after.cache, aclIPEAuditTagValue)
	}
	extractHash := func(cmdline string) string {
		const prefix = "acl.ipe.policy_sha256="
		var value string
		for _, token := range strings.Fields(cmdline) {
			if strings.HasPrefix(token, prefix) {
				if value != "" {
					return ""
				}
				value = strings.TrimPrefix(token, prefix)
			}
		}
		return value
	}
	hash := extractHash(before.cmdline)
	if !aclIPEHashToken.MatchString(hash) || extractHash(after.cmdline) != hash {
		return fmt.Errorf("UKI policy credential hash missing or changed across restart")
	}
	for _, line := range []string{
		"Credential SHA-256 verified: " + hash,
		"Loaded policy " + aclIPEPolicyName + " into kernel IPE.",
		"Using IPE mode 'audit'.",
		"Activated policy " + aclIPEPolicyName + ".",
	} {
		if !strings.Contains(after.journal, "acl-ipe-load: "+line) {
			return fmt.Errorf("current-boot ACL IPE audit loader journal lacks %q", line)
		}
	}
	return nil
}

func aclIPERestart(ctx context.Context, s *Scenario, before aclIPEIdentity) error {
	rg := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup
	poller, err := config.Azure.VMSSVM.BeginRestart(ctx, rg, s.Runtime.VMSSName, before.instanceID, nil)
	if err != nil {
		return fmt.Errorf("restart only scenario-owned VMSS instance %s: %w", before.instanceID, err)
	}
	cleanupBastionTunnel(s.Runtime.VM.SSHClient)
	s.Runtime.VM.SSHClient = nil
	if _, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions()); err != nil {
		return fmt.Errorf("await single VMSS instance restart: %w", err)
	}
	return wait.PollUntilContextTimeout(ctx, 15*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		sshClient, err := DialSSHOverBastion(ctx, s.Runtime.Cluster.Bastion, s.Runtime.VM.PrivateIP, config.VMSSHPrivateKey)
		if err != nil {
			logging.Logf(ctx, "waiting for scenario SSH after restart: %v", err)
			return false, nil
		}
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		boot, checkErr := runSSHCommand(checkCtx, sshClient, "cat /proc/sys/kernel/random/boot_id", false)
		cancel()
		if checkErr != nil || boot.exitCode != "0" || strings.TrimSpace(boot.stdout) == before.bootID {
			cleanupBastionTunnel(sshClient)
			return false, nil
		}
		if _, err := uuid.Parse(strings.TrimSpace(boot.stdout)); err != nil {
			cleanupBastionTunnel(sshClient)
			return false, fmt.Errorf("invalid guest boot ID after restart: %w", err)
		}
		s.Runtime.VM.SSHClient = sshClient
		return true, nil
	})
}

func ValidateACLIPETransition(ctx context.Context, s *Scenario) (err error) {
	if !aclIPETransitionEnabled() {
		return aclIPETransitionGate(s)
	}
	start := time.Now()
	defer func() {
		s.recordADOTestCase("ACL_IPE_OffToAudit", "e2e.acl.ipe", time.Since(start), err)
	}()
	if err := aclIPETransitionGate(s); err != nil {
		return err
	}
	model, err := aclIPEGetModel(ctx, s)
	if err != nil {
		return err
	}
	if err := aclIPEPreflight(ctx, s, &model); err != nil {
		return err
	}
	before, err := aclIPEVM(ctx, s, model)
	if err != nil {
		return err
	}
	if err := validateACLIPEVMSSNoProfileTag(model.Tags); err != nil {
		return err
	}
	if err := aclIPEReadUntaggedInstance(ctx, s, before.instanceID); err != nil {
		return err
	}
	if err := aclIPENode(ctx, s, &before); err != nil {
		return err
	}
	if err := aclIPEReadIMDS(ctx, s, before, false); err != nil {
		return err
	}
	firstBoot, err := aclIPEReadBoot(ctx, s)
	if err != nil {
		return err
	}
	if err := validateACLIPEBootEvidence(firstBoot, "off"); err != nil {
		return err
	}
	before.bootID = firstBoot.bootID
	initialProbeUID, err := aclIPEProbePod(ctx, s, time.Now())
	if err != nil {
		return fmt.Errorf("initial untagged ACL node targeted workload: %w", err)
	}
	if err := aclIPECheckWorkloads(ctx, s); err != nil {
		return err
	}
	before.nonce, err = aclIPECreateNonce(ctx, s)
	if err != nil {
		return err
	}
	tags, err := aclIPETaggedModel(model.Tags)
	if err != nil {
		return err
	}
	if err := aclIPEArmTagRestore(s, model, tags); err != nil {
		return err
	}
	if err := aclIPEPatch(ctx, s, model, tags); err != nil {
		return err
	}
	if err := aclIPEVerifyModel(ctx, s, model, tags); err != nil {
		return err
	}
	if err := aclIPECheckWorkloads(ctx, s); err != nil {
		return err
	}
	if err := aclIPERestart(ctx, s, before); err != nil {
		return err
	}
	restartFinishedAt := time.Now()
	if err := aclIPEReadAuditInstance(ctx, s); err != nil {
		return err
	}
	afterModel, err := aclIPEGetModel(ctx, s)
	if err != nil {
		return err
	}
	after, err := aclIPEVM(ctx, s, afterModel)
	if err != nil {
		return err
	}
	afterBoot, err := aclIPEReadBoot(ctx, s)
	if err != nil {
		return err
	}
	after.bootID = afterBoot.bootID
	if after.bootID == before.bootID {
		return fmt.Errorf("ACL IPE transition did not observe a new guest boot ID")
	}
	if err := aclIPEWaitForNodeAfterBoot(ctx, s, before, restartFinishedAt, &after); err != nil {
		return err
	}
	if err := aclIPECheckWorkloads(ctx, s); err != nil {
		return err
	}
	after.nonce, err = aclIPEReadNonce(ctx, s)
	if err != nil {
		return err
	}
	if err := aclIPEInstanceUnchanged(before, after); err != nil {
		return err
	}
	if err := aclIPEVerifyModel(ctx, s, model, tags); err != nil {
		return err
	}
	if err := aclIPEReadIMDS(ctx, s, after, true); err != nil {
		return err
	}
	if err := aclIPEAuditReboot(firstBoot, afterBoot); err != nil {
		return err
	}
	stateOutput, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		aclIPEPrivilegedProbe(aclIPEPolicyStateProbe), 0, "read ACL IPE policy after restart")
	if err != nil {
		return err
	}
	state, err := parseACLIPEPolicyState(stateOutput.stdout)
	if err != nil {
		return err
	}
	if err := validateACLIPEPolicyState(state, "audit"); err != nil {
		return err
	}
	for _, unit := range []string{"kubelet", "containerd"} {
		if err := ValidateSystemdUnitIsRunning(ctx, s, unit); err != nil {
			return fmt.Errorf("ACL IPE audit reboot %s health: %w", unit, err)
		}
	}
	if err := validateACLIPEAuditDeny(ctx, s); err != nil {
		return err
	}
	uid, err := aclIPEProbePod(ctx, s, restartFinishedAt)
	if err != nil {
		return fmt.Errorf("post-restart fresh targeted ACL workload: %w", err)
	}
	if uid == initialProbeUID {
		return fmt.Errorf("post-restart ACL probe reused the first-boot pod UID")
	}
	logging.Logf(ctx, "ACL IPE off-to-audit validated on owned AKS-registered VMSS node: vmId=%s nodeUID=%s probeUID=%s", after.vmID, after.nodeUID, uid)
	return nil
}
