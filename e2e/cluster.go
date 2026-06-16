package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/dag"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterParams struct {
	CACert         []byte
	BootstrapToken string
	FQDN           string
}

type Cluster struct {
	Model            *armcontainerservice.ManagedCluster
	kubeconfig       []byte // raw kubeconfig for minting per-test clients
	KubeletIdentity  *armcontainerservice.UserAssignedIdentity
	SubnetID         string
	VNetResourceGUID string
	ClusterParams    *ClusterParams
	Bastion          *Bastion
	ProxyURL         string
	TenantID         string
}

// NewKubeclientForTest creates an independent Kubeclient with its own rate limiter.
// Use this in individual tests to avoid sharing rate limiter tokens with other
// parallel tests hitting the same cluster.
func (c *Cluster) NewKubeclientForTest() (*Kubeclient, error) {
	return NewKubeclient(c.kubeconfig)
}

// Returns true if the cluster is configured with Azure CNI
func (c *Cluster) IsAzureCNI() (bool, error) {
	if c.Model.Properties.NetworkProfile != nil {
		return *c.Model.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginAzure, nil
	}
	return false, fmt.Errorf("cluster network profile was nil: %+v", c.Model)
}

// Returns the maximum number of pods per node of the cluster's agentpool
func (c *Cluster) MaxPodsPerNode() (int, error) {
	if len(c.Model.Properties.AgentPoolProfiles) > 0 {
		return int(*c.Model.Properties.AgentPoolProfiles[0].MaxPods), nil
	}
	return 0, fmt.Errorf("cluster agentpool profiles were nil or empty: %+v", c.Model)
}

// prepareCluster runs all cluster preparation steps as a concurrent DAG.
// This function contains complex concurrent orchestration — keep it as
// minimal as possible and push all non-trivial logic into the individual
// task functions it calls.
func prepareCluster(ctx context.Context, clusterModel *armcontainerservice.ManagedCluster, isNetworkIsolated, attachPrivateAcr bool) (*Cluster, error) {
	defer toolkit.LogStepCtx(ctx, "preparing cluster")()
	ctx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutCluster)
	defer cancel()

	infra, err := configureSharedVNet(ctx, clusterModel, *clusterModel.Location)
	if err != nil {
		return nil, err
	}

	clusterModel.Name = to.Ptr(fmt.Sprintf("%s-%s", *clusterModel.Name, hash(clusterModel)))
	// If configureSharedVNet marked this model, create the per-cluster subnet
	// now that we have the final hashed name, with an auto-allocated CIDR.
	subnetID, err := CachedEnsureClusterSubnet(ctx, ClusterSubnetRequest{
		Location:    *clusterModel.Location,
		ClusterName: *clusterModel.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("ensuring cluster subnet: %w", err)
	}
	for _, pool := range clusterModel.Properties.AgentPoolProfiles {
		pool.VnetSubnetID = to.Ptr(subnetID)
	}

	cluster, err := getOrCreateCluster(ctx, clusterModel)
	if err != nil {
		return nil, fmt.Errorf("get or create cluster: %w", err)
	}

	g := dag.NewGroup(ctx)

	// bastion creates AzureBastionSubnet — a VNet-level mutation that must
	// finish before other subnet writes (firewall / network-isolated setup)
	// to avoid Azure VNet serialisation races.
	bastion := dag.Go(g, func(ctx context.Context) (*Bastion, error) {
		return getOrCreateBastion(ctx, cluster)
	})
	dag.Run(g, func(ctx context.Context) error { return ensureMaintenanceConfiguration(ctx, cluster) })
	subnet := dag.Go(g, func(ctx context.Context) (string, error) { return getClusterSubnetID(ctx, cluster) })
	vNet := dag.Go(g, func(ctx context.Context) (VNet, error) {
		return getClusterVNet(ctx, cluster)
	})
	// Fetch kubeconfig bytes once, then each heavy DAG task creates its own
	// Kubeclient with an independent rate limiter to prevent starvation.
	kubeconfigBytes := dag.Go(g, func(ctx context.Context) ([]byte, error) {
		resourceGroupName := config.ResourceGroupName(*cluster.Location)
		return getClusterKubeconfigBytes(ctx, resourceGroupName, *cluster.Name)
	})
	newKubeFromBytes := func(ctx context.Context, data []byte) (*Kubeclient, error) {
		return NewKubeclient(data)
	}
	kubeForGC := dag.Go1(g, kubeconfigBytes, newKubeFromBytes)
	kubeForDebug := dag.Go1(g, kubeconfigBytes, newKubeFromBytes)
	kubeForExtract := dag.Go1(g, kubeconfigBytes, newKubeFromBytes)
	kubeForACR := dag.Go1(g, kubeconfigBytes, newKubeFromBytes)
	identity := dag.Go(g, func(ctx context.Context) (*armcontainerservice.UserAssignedIdentity, error) {
		return getClusterKubeletIdentity(ctx, cluster)
	})
	if !isNetworkIsolated {
		dag.Run1(g, vNet, func(ctx context.Context, v VNet) error {
			return setupPrivateDNSForAPIServer(ctx, cluster, v)
		})
	}
	// networkSetup adds firewall routes to the existing AKS route table or
	// creates/associates a dedicated one when Azure CNI has none, or applies
	// the network-isolated NSG. It must run after bastion (both mutate the
	// VNet) and before collectGarbageVMSS (which needs network setup done).
	// collectGarbageVMSS also depends on kube to clean up stale K8s Node
	// objects whose backing VMSS no longer exist.
	var networkDeps []dag.Dep
	if !isNetworkIsolated {
		networkDeps = append(networkDeps, dag.Run(g, func(ctx context.Context) error { return addFirewallRules(ctx, infra, cluster) }, bastion))
	}
	if isNetworkIsolated {
		networkDeps = append(networkDeps, dag.Run(g, func(ctx context.Context) error { return addNetworkIsolatedSettings(ctx, cluster) }, bastion))
	}
	dag.Run1(g, kubeForGC, func(ctx context.Context, k *Kubeclient) error { return collectGarbageVMSS(ctx, cluster, k) }, networkDeps...)
	needACR := isNetworkIsolated || attachPrivateAcr

	// The private DNS zone and VNet link must exist before any PE is created.
	// Create them once as a dependency for both ACR tasks.
	var acrNonAnon, acrAnon dag.Dep
	if needACR {
		dnsReady := dag.Run1(g, vNet, func(ctx context.Context, v VNet) error {
			_, err := ensurePrivateDNSZone(ctx, v)
			return err
		}, bastion)
		acrNonAnon = dag.Run2(g, kubeForACR, identity, addACR(cluster, true), dnsReady)
		acrAnon = dag.Run2(g, kubeForACR, identity, addACR(cluster, false), dnsReady)
	}
	debugDeps := append(networkDeps[:0:0], networkDeps...)
	if acrNonAnon != nil {
		debugDeps = append(debugDeps, acrNonAnon, acrAnon)
	}
	proxyURL := dag.Go1(g, kubeForDebug, func(ctx context.Context, k *Kubeclient) (string, error) {
		if err := k.EnsureDebugDaemonsets(ctx, isNetworkIsolated, config.GetPrivateACRName(true, *cluster.Location)); err != nil {
			return "", err
		}
		if isNetworkIsolated {
			return "", nil
		}
		return k.GetProxyURL(ctx)
	}, debugDeps...)
	extract := dag.Go1(g, kubeForExtract, extractClusterParams(cluster))

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("prepare cluster tasks: %w", err)
	}
	return &Cluster{
		Model:            cluster,
		kubeconfig:       kubeconfigBytes.MustGet(),
		KubeletIdentity:  identity.MustGet(),
		SubnetID:         subnet.MustGet(),
		VNetResourceGUID: vNet.MustGet().resourceGUID,
		ClusterParams:    extract.MustGet(),
		Bastion:          bastion.MustGet(),
		ProxyURL:         proxyURL.MustGet(),
		TenantID:         infra.TenantID,
	}, nil
}

func addACR(cluster *armcontainerservice.ManagedCluster, isNonAnonymousPull bool) func(context.Context, *Kubeclient, *armcontainerservice.UserAssignedIdentity) error {
	return func(ctx context.Context, k *Kubeclient, id *armcontainerservice.UserAssignedIdentity) error {
		return addPrivateAzureContainerRegistry(ctx, cluster, k, id, isNonAnonymousPull)
	}
}

func extractClusterParams(cluster *armcontainerservice.ManagedCluster) func(context.Context, *Kubeclient) (*ClusterParams, error) {
	return func(ctx context.Context, k *Kubeclient) (*ClusterParams, error) {
		return extractClusterParameters(ctx, cluster, k)
	}
}

func getClusterKubeletIdentity(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.UserAssignedIdentity, error) {
	if cluster == nil || cluster.Properties == nil || cluster.Properties.IdentityProfile == nil {
		return nil, fmt.Errorf("cannot dereference cluster identity profile to extract kubelet identity ID")
	}
	kubeletIdentity := cluster.Properties.IdentityProfile["kubeletidentity"]
	if kubeletIdentity == nil {
		return nil, fmt.Errorf("kubelet identity is missing from cluster properties")
	}
	return kubeletIdentity, nil
}

func extractClusterParameters(ctx context.Context, cluster *armcontainerservice.ManagedCluster, kube *Kubeclient) (*ClusterParams, error) {
	kubeconfig, err := clientcmd.Load(kube.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("loading cluster kubeconfig: %w", err)
	}
	clusterConfig := kubeconfig.Clusters[*cluster.Name]
	if clusterConfig == nil {
		return nil, fmt.Errorf("cluster kubeconfig missing configuration for %s", *cluster.Name)
	}
	token, err := getBootstrapToken(ctx, kube)
	if err != nil {
		return nil, fmt.Errorf("getting bootstrap token: %w", err)
	}
	return &ClusterParams{
		CACert:         clusterConfig.CertificateAuthorityData,
		BootstrapToken: token,
		FQDN:           *cluster.Properties.Fqdn,
	}, nil
}

func assignACRPullToIdentity(ctx context.Context, privateACRName, principalID string, location string) error {
	toolkit.Logf(ctx, "assigning ACR-Pull role to %s", principalID)
	scope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerRegistry/registries/%s", config.Config.SubscriptionID, config.ResourceGroupName(location), privateACRName)

	uid := uuid.New().String()
	_, err := config.Azure.RoleAssignments.Create(ctx, scope, uid, armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID: to.Ptr(principalID),
			// ACR-Pull role definition ID
			RoleDefinitionID: to.Ptr("/providers/Microsoft.Authorization/roleDefinitions/7f951dda-4ed3-4680-a7ca-43fe172d538d"),
			PrincipalType:    to.Ptr(armauthorization.PrincipalTypeServicePrincipal),
		},
	}, nil)
	var respError *azcore.ResponseError
	if err != nil {
		// if the role assignment already exists, ignore the error
		if errors.As(err, &respError) && respError.StatusCode == http.StatusConflict {
			return nil
		}
		toolkit.Logf(ctx, "failed to assign ACR-Pull role to identity %s, error: %v", config.VMIdentityName, err)
		return err
	}
	return nil
}

func getBootstrapToken(ctx context.Context, kube *Kubeclient) (string, error) {
	secrets, err := kube.Typed.CoreV1().Secrets("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list secrets: %w", err)
	}
	secret := func() *corev1.Secret {
		for _, secret := range secrets.Items {
			if strings.HasPrefix(secret.Name, "bootstrap-token-") {
				return &secret
			}
		}
		return nil
	}()
	if secret == nil {
		return "", fmt.Errorf("no bootstrap token secret found in kube-system namespace")
	}
	id := secret.Data["token-id"]
	token := secret.Data["token-secret"]
	return fmt.Sprintf("%s.%s", id, token), nil
}

func hash(cluster *armcontainerservice.ManagedCluster) string {
	jsonData, err := json.Marshal(cluster)
	if err != nil {
		panic(err)
	}
	hasher := sha256.New()
	_, err = hasher.Write(jsonData)
	if err != nil {
		panic(err)
	}
	hexHash := hex.EncodeToString(hasher.Sum(nil))
	return hexHash[:5]
}

func getOrCreateCluster(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	defer toolkit.LogStepCtxf(ctx, "get or create cluster %s", *cluster.Name)()
	existingCluster, err := getExistingCluster(ctx, *cluster.Location, *cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing cluster %q: %w, and wont retry", *cluster.Name, err)
	}

	if existingCluster != nil {
		// create new cluster;
		return existingCluster, nil
	}

	return createNewAKSClusterWithRetry(ctx, cluster)
}

// isExistingCluster checks if an AKS cluster exists. return the cluster only if its provisioning state is Succeeded and can be used. non-nil error if not retriable
func getExistingCluster(ctx context.Context, location, clusterName string) (*armcontainerservice.ManagedCluster, error) {
	resourceGroupName := config.ResourceGroupName(location)
	existingCluster, err := config.Azure.AKS.Get(ctx, resourceGroupName, clusterName, nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) {
		if azErr.StatusCode == 404 {
			return nil, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s: %s", clusterName, err)
	}

	switch *existingCluster.Properties.ProvisioningState {
	case "Succeeded":
		nodeRGExists, err := isUsableNodeResourceGroup(ctx, location, clusterName, *existingCluster.Properties.NodeResourceGroup)

		if err != nil {
			return nil, err
		}
		// ensure MC_rg as well --> functioning. during cluster provisioning, the node resource group may not exist yet and we can wait
		if nodeRGExists {
			return &existingCluster.ManagedCluster, nil
		}
		toolkit.Logf(ctx, "##vso[task.logissue type=warning;]Cluster %s has deleting or missing node resource group %s, deleting cluster", clusterName, *existingCluster.Properties.NodeResourceGroup)
		if err := deleteCluster(ctx, clusterName, resourceGroupName); err != nil {
			return nil, err
		}
		// Wait for Azure to confirm cluster is fully deleted before allowing recreation.
		// This prevents "Reconcile managed identity credential failed" errors where Azure's
		// backend still has stale references to the old cluster during the new cluster's
		// identity reconciliation process.
		if err := waitForClusterDeletion(ctx, clusterName, resourceGroupName); err != nil {
			return nil, fmt.Errorf("failed waiting for cluster deletion: %w", err)
		}
		return nil, nil
	case "Failed":
		toolkit.Logf(ctx, "##vso[task.logissue type=warning;]Cluster %s in Failed state, deleting", clusterName)
		if err := deleteCluster(ctx, clusterName, resourceGroupName); err != nil {
			return nil, err
		}
		// Wait for Azure to confirm cluster is fully deleted before allowing recreation.
		// This prevents "Reconcile managed identity credential failed" errors where Azure's
		// backend still has stale references to the old cluster during the new cluster's
		// identity reconciliation process.
		if err := waitForClusterDeletion(ctx, clusterName, resourceGroupName); err != nil {
			return nil, fmt.Errorf("failed waiting for cluster deletion: %w", err)
		}
		return nil, nil
	default:
		// other provisioning state,  deleting, , stopping,,cancaled,cancelling,"Creating", "Updating", "Scaling", "Migrating", "Upgrading", "Starting", "Restoring": .. plus many others.
		toolkit.Logf(ctx, "##vso[task.logissue type=warning;]Unexpected cluster provisioning state %s: %s", clusterName, *existingCluster.Properties.ProvisioningState)
		return waitUntilClusterReady(ctx, clusterName, location)
	}
}

func deleteCluster(ctx context.Context, clusterName, resourceGroupName string) error {
	defer toolkit.LogStepCtxf(ctx, "deleting cluster %s", clusterName)()
	// beileih: why do we do this?
	_, err := config.Azure.AKS.Get(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == 404 {
			toolkit.Logf(ctx, "cluster %s does not exist, skipping deletion", clusterName)
			return nil
		}
		return fmt.Errorf("failed to retrieve cluster while trying to delete it %q: %w", clusterName, err)
	}

	pollerResp, err := config.Azure.AKS.BeginDelete(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete cluster %q: %w", clusterName, err)
	}
	_, err = pollerResp.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster deletion %w", err)
	}
	return nil
}

func waitForClusterDeletion(ctx context.Context, clusterName, resourceGroupName string) error {
	return wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := config.Azure.AKS.Get(ctx, resourceGroupName, clusterName, nil)
		if err != nil {
			var azErr *azcore.ResponseError
			if errors.As(err, &azErr) && azErr.StatusCode == 404 {
				return true, nil // Cluster is gone
			}
			return false, fmt.Errorf("unexpected error checking cluster: %w", err)
		}
		return false, nil // Still exists, keep polling
	})
}

func waitUntilClusterReady(ctx context.Context, name, location string) (*armcontainerservice.ManagedCluster, error) {
	var cluster armcontainerservice.ManagedClustersClientGetResponse
	var clusterDeleted bool
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		var err error
		cluster, err = config.Azure.AKS.Get(ctx, config.ResourceGroupName(location), name, nil)
		if err != nil {
			var azErr *azcore.ResponseError
			if errors.As(err, &azErr) && azErr.StatusCode == 404 {
				clusterDeleted = true
				return true, nil
			}
			return false, err
		}
		switch *cluster.ManagedCluster.Properties.ProvisioningState {
		case "Succeeded":
			return true, nil
		case "Updating", "Assigned", "Creating", "Deleting", "Canceling":
			return false, nil
		case "Canceled":
			return false, fmt.Errorf("cluster %s is in state %s, won't retry", name, *cluster.ManagedCluster.Properties.ProvisioningState)
		default:
			return false, fmt.Errorf("cluster %s is in state %s, won't retry", name, *cluster.ManagedCluster.Properties.ProvisioningState)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for cluster %s to be ready: %w", name, err)
	}
	if clusterDeleted {
		return nil, nil
	}
	if cluster.ManagedCluster.Properties != nil && cluster.ManagedCluster.Properties.NodeResourceGroup != nil {
		nodeRGExists, err := isUsableNodeResourceGroup(ctx, location, name, *cluster.ManagedCluster.Properties.NodeResourceGroup)
		if err != nil {
			return nil, err
		}
		if !nodeRGExists {
			resourceGroupName := config.ResourceGroupName(location)
			toolkit.Logf(ctx, "##vso[task.logissue type=warning;]Cluster %s became ready with deleting or missing node resource group %s, deleting cluster", name, *cluster.ManagedCluster.Properties.NodeResourceGroup)
			if err := deleteCluster(ctx, name, resourceGroupName); err != nil {
				return nil, err
			}
			if err := waitForClusterDeletion(ctx, name, resourceGroupName); err != nil {
				return nil, fmt.Errorf("failed waiting for cluster deletion: %w", err)
			}
			return nil, nil
		}
	}
	return &cluster.ManagedCluster, nil
}

func isUsableNodeResourceGroup(ctx context.Context, location, clusterName, resourceGroupName string) (bool, error) {
	rg, err := config.Azure.ResourceGroup.Get(ctx, resourceGroupName, nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	if rg.Properties != nil && rg.Properties.ProvisioningState != nil && strings.EqualFold(*rg.Properties.ProvisioningState, "Deleting") {
		if err := detachNodeResourceGroupReferencesFromClusterSubnet(ctx, location, clusterName, resourceGroupName); err != nil {
			toolkit.Logf(ctx, "warning: failed to detach subnet references for deleting node resource group %q: %v", resourceGroupName, err)
		}
		toolkit.Logf(ctx, "node resource group %q is deleting; recreating cluster %q", resourceGroupName, clusterName)
		return false, nil
	}

	hasVMSS, err := hasVMSSInResourceGroup(ctx, resourceGroupName)
	if err != nil {
		return false, err
	}
	if !hasVMSS {
		toolkit.Logf(ctx, "node resource group %q has no VMSS; recreating cluster %q", resourceGroupName, clusterName)
		return false, nil
	}

	return true, nil
}

func hasVMSSInResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	pager := config.Azure.VMSS.NewListPager(resourceGroupName, nil)
	if !pager.More() {
		return false, nil
	}
	page, err := pager.NextPage(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list VMSS in node resource group %q: %w", resourceGroupName, err)
	}
	return len(page.Value) > 0, nil
}

func createNewAKSCluster(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	// Note, it seems like the operation still can start a trigger a new operation even if nothing has changes
	pollerResp, err := config.Azure.AKS.BeginCreateOrUpdate(
		ctx,
		config.ResourceGroupName(*cluster.Location),
		*cluster.Name,
		*cluster,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin aks cluster creation: %w", err)
	}

	clusterResp, err := pollerResp.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for aks cluster creation %w", err)
	}

	return &clusterResp.ManagedCluster, nil
}

// createNewAKSClusterWithRetry is a wrapper around createNewAKSCluster
// that retries creating a cluster if it fails with a 409 Conflict error
// clusters are reused, and sometimes a cluster can be in UPDATING or DELETING state
// simple retry should be sufficient to avoid such conflicts
func createNewAKSClusterWithRetry(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	maxRetries := 10
	retryInterval := 30 * time.Second
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			toolkit.Logf(ctx, "Attempt %d: creating or updating cluster %s in region %s and rg %s", attempt+1, *cluster.Name, *cluster.Location, config.ResourceGroupName(*cluster.Location))
		}

		createdCluster, err := createNewAKSCluster(ctx, cluster)
		if err == nil {
			return createdCluster, nil
		}

		if isRetryableClusterError(err) {
			lastErr = err
			if isClusterCreateOperationInProgressError(err) {
				toolkit.Logf(ctx, "cluster %s has an in-progress create operation; waiting for it to finish", *cluster.Name)
				createdCluster, waitErr := waitUntilClusterReady(ctx, *cluster.Name, *cluster.Location)
				if waitErr != nil {
					return nil, fmt.Errorf("waiting for in-progress cluster creation: %w", waitErr)
				}
				if createdCluster != nil {
					return createdCluster, nil
				}
				continue
			}
			if isResourceGroupBeingDeletedError(err) {
				nodeResourceGroup := expectedNodeResourceGroupName(*cluster.Location, *cluster.Name)
				if cluster.Properties != nil && cluster.Properties.NodeResourceGroup != nil {
					nodeResourceGroup = *cluster.Properties.NodeResourceGroup
				}
				if cleanupErr := detachNodeResourceGroupReferencesFromClusterSubnet(ctx, *cluster.Location, *cluster.Name, nodeResourceGroup); cleanupErr != nil {
					toolkit.Logf(ctx, "warning: failed to detach subnet references for deleting node resource group %q: %v", nodeResourceGroup, cleanupErr)
				}
				if deleteErr := deleteCluster(ctx, *cluster.Name, config.ResourceGroupName(*cluster.Location)); deleteErr != nil {
					return nil, fmt.Errorf("deleting cluster with deleting node resource group %q: %w", nodeResourceGroup, deleteErr)
				}
				if deleteErr := waitForClusterDeletion(ctx, *cluster.Name, config.ResourceGroupName(*cluster.Location)); deleteErr != nil {
					return nil, fmt.Errorf("failed waiting for cluster deletion: %w", deleteErr)
				}
			}
			toolkit.Logf(ctx, "Attempt %d failed with retryable error: %v. Retrying in %v...", attempt+1, err, retryInterval)

			select {
			case <-time.After(retryInterval):
			case <-ctx.Done():
				return nil, fmt.Errorf("context canceled while retrying cluster creation: %w", ctx.Err())
			}
		} else {
			return nil, fmt.Errorf("failed to create cluster: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to create cluster after %d attempts: %w", maxRetries, lastErr)
}

func expectedNodeResourceGroupName(location, clusterName string) string {
	return fmt.Sprintf("MC_%s_%s_%s", config.ResourceGroupName(location), clusterName, location)
}

// isRetryableClusterError returns true for transient cluster creation errors
// that can be resolved by retrying, such as 409 Conflict (concurrent operations)
// and NotFound during managed identity reconciliation (stale references after cluster deletion).
func isRetryableClusterError(err error) bool {
	var respErr *azcore.ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	if respErr.StatusCode == 409 {
		return true
	}
	return respErr.ErrorCode == "NotFound" && strings.Contains(err.Error(), "Reconcile managed identity credential failed")
}

func isResourceGroupBeingDeletedError(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusConflict && respErr.ErrorCode == "ResourceGroupBeingDeleted"
}

func isClusterCreateOperationInProgressError(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) &&
		respErr.StatusCode == http.StatusConflict &&
		respErr.ErrorCode == "OperationNotAllowed" &&
		strings.Contains(err.Error(), "in progress create managed cluster operation")
}

func ensureMaintenanceConfiguration(ctx context.Context, cluster *armcontainerservice.ManagedCluster) error {
	_, err := config.Azure.Maintenance.Get(ctx, config.ResourceGroupName(*cluster.Location), *cluster.Name, "default", nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == 404 {
		_, err = createNewMaintenanceConfiguration(ctx, cluster)
		if err != nil {
			return fmt.Errorf("creating maintenance configuration for cluster %q: %w", *cluster.Name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get maintenance configuration 'default' for cluster %q: %w", *cluster.Name, err)
	}
	return nil
}

func createNewMaintenanceConfiguration(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.MaintenanceConfiguration, error) {
	toolkit.Logf(ctx, "creating maintenance configuration for cluster %s in rg %s", *cluster.Name, config.ResourceGroupName(*cluster.Location))
	maintenance := armcontainerservice.MaintenanceConfiguration{
		Properties: &armcontainerservice.MaintenanceConfigurationProperties{
			MaintenanceWindow: &armcontainerservice.MaintenanceWindow{
				NotAllowedDates: []*armcontainerservice.DateSpan{ // no maintenance till 2100
					{
						End:   to.Ptr(func() time.Time { t, _ := time.Parse("2006-01-02", "2100-01-01"); return t }()),
						Start: to.Ptr(func() time.Time { t, _ := time.Parse("2006-01-02", "2000-01-01"); return t }()),
					}},
				DurationHours: to.Ptr[int32](4),
				StartTime:     to.Ptr("00:00"),  //PST
				UTCOffset:     to.Ptr("+08:00"), //PST
				Schedule: &armcontainerservice.Schedule{
					Weekly: &armcontainerservice.WeeklySchedule{
						DayOfWeek:     to.Ptr(armcontainerservice.WeekDayMonday),
						IntervalWeeks: to.Ptr[int32](1),
					},
				},
			},
		},
	}

	_, err := config.Azure.Maintenance.CreateOrUpdate(ctx, config.ResourceGroupName(*cluster.Location), *cluster.Name, "default", maintenance, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create maintenance configuration: %w", err)
	}

	return &maintenance, nil
}

func getOrCreateBastion(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*Bastion, error) {
	location := *cluster.Location
	sharedRG := config.ResourceGroupName(location)
	sharedBastion, err := config.Azure.BastionHosts.Get(ctx, sharedRG, SharedBastionName, nil)
	if err != nil {
		if !isNotFoundError(err) {
			return nil, fmt.Errorf("checking shared bastion %s in %s: %w", SharedBastionName, sharedRG, err)
		}
		toolkit.Logf(ctx, "shared bastion not found, recreating")
		dnsName, createErr := ensureSharedBastion(ctx, sharedRG, location)
		if createErr != nil {
			return nil, fmt.Errorf("recreating shared bastion: %w", createErr)
		}
		return NewBastion(config.Azure.Credential, config.Config.SubscriptionID, sharedRG, dnsName), nil
	}
	toolkit.Logf(ctx, "using shared bastion %s in %s", SharedBastionName, sharedRG)
	return NewBastion(config.Azure.Credential, config.Config.SubscriptionID, sharedRG, *sharedBastion.Properties.DNSName), nil
}

type VNet struct {
	name          string
	resourceGroup string
	subnetName    string
	subnetId      string
	resourceGUID  string
	addressPrefix string
}

// getClusterVNet returns VNet info for the cluster by parsing the VnetSubnetID from the agent pool.
func getClusterVNet(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (VNet, error) {
	for _, pool := range cluster.Properties.AgentPoolProfiles {
		if pool.VnetSubnetID != nil && *pool.VnetSubnetID != "" {
			return vnetFromSubnetID(ctx, *pool.VnetSubnetID)
		}
	}
	return VNet{}, fmt.Errorf("no VnetSubnetID found on any agent pool profile")
}

func collectGarbageVMSS(ctx context.Context, cluster *armcontainerservice.ManagedCluster, kube *Kubeclient) error {
	defer toolkit.LogStepCtx(ctx, "collecting garbage VMSS")()
	rg := *cluster.Properties.NodeResourceGroup

	// Build a set of VMSS name prefixes belonging to the cluster's managed pools.
	// AKS names managed pool VMSS as "aks-<poolname>-<hash>-vmss". We use the
	// prefix "aks-<poolname>-" to protect these even if the aks-managed-poolName
	// tag is missing (defense-in-depth against tag propagation races).
	managedPoolPrefixes := make([]string, 0, len(cluster.Properties.AgentPoolProfiles))
	for _, pool := range cluster.Properties.AgentPoolProfiles {
		if pool.Name != nil {
			managedPoolPrefixes = append(managedPoolPrefixes, "aks-"+*pool.Name+"-")
		}
	}

	// Build a set of VMSS names that should be kept — exclude VMSS that are
	// being deleted so their stale K8s nodes can be cleaned up in the same pass.
	keptVMSS := map[string]struct{}{}
	pager := config.Azure.VMSS.NewListPager(rg, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page of VMSS: %w", err)
		}
		for _, vmss := range page.Value {
			if _, ok := vmss.Tags["KEEP_VMSS"]; ok {
				keptVMSS[*vmss.Name] = struct{}{}
				continue
			}
			// don't delete managed pools (tag-based check)
			if _, ok := vmss.Tags["aks-managed-poolName"]; ok {
				keptVMSS[*vmss.Name] = struct{}{}
				continue
			}
			// don't delete VMSS whose name matches a known managed pool prefix
			// (protects against missing tags during cluster reconciliation)
			if isManagedPoolVMSS(*vmss.Name, managedPoolPrefixes) {
				keptVMSS[*vmss.Name] = struct{}{}
				continue
			}

			// don't delete VMSS created in the last hour. They might be currently used in tests
			// extra 10 minutes is a buffer for test cleanup, clock drift and timeout adjustments
			if config.Config.TestTimeout == 0 || time.Since(*vmss.Properties.TimeCreated) < config.Config.TestTimeout+10*time.Minute {
				keptVMSS[*vmss.Name] = struct{}{}
				continue
			}

			_, err := config.Azure.VMSS.BeginDelete(ctx, rg, *vmss.Name, &armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{
				ForceDeletion: to.Ptr(true),
			})
			if err != nil {
				toolkit.Logf(ctx, "failed to delete vmss %q: %s", *vmss.Name, err)
				// Keep in map so we don't try to delete its nodes while VMSS is still around
				keptVMSS[*vmss.Name] = struct{}{}
				continue
			}
			toolkit.Logf(ctx, "deleted vmss %q (age: %v)", *vmss.ID, time.Since(*vmss.Properties.TimeCreated))
			// Don't add to keptVMSS — nodes from this VMSS should be cleaned up
		}
	}

	if err := collectGarbageNodes(ctx, kube, keptVMSS); err != nil {
		return fmt.Errorf("failed to collect garbage K8s nodes: %w", err)
	}
	return nil
}

// collectGarbageNodes deletes Kubernetes Node objects whose backing VMSS no
// longer exists. This prevents stale nodes from accumulating in the cluster
// and overwhelming the cloud-provider-azure route controller with perpetual
// "instance not found" failures.
func collectGarbageNodes(ctx context.Context, kube *Kubeclient, keptVMSS map[string]struct{}) error {
	defer toolkit.LogStepCtx(ctx, "collecting garbage K8s nodes")()

	nodes, err := kube.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing K8s nodes for garbage collection: %w", err)
	}

	var deleted, failed int
	for _, node := range nodes.Items {
		// skip managed pool nodes (system nodepool)
		if strings.HasPrefix(node.Name, "aks-") {
			continue
		}

		// VMSS instance names are the VMSS name + 6-digit instance ID suffix
		if len(node.Name) < 7 {
			continue
		}
		vmssName := node.Name[:len(node.Name)-6]

		if _, exists := keptVMSS[vmssName]; exists {
			continue
		}

		if err := kube.Typed.CoreV1().Nodes().Delete(ctx, node.Name, metav1.DeleteOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				toolkit.Logf(ctx, "stale K8s node %q already gone", node.Name)
				deleted++
				continue
			}
			toolkit.Logf(ctx, "warning: failed to delete stale K8s node %q: %v", node.Name, err)
			failed++
			continue
		}
		toolkit.Logf(ctx, "deleted stale K8s node %q (VMSS %q not found)", node.Name, vmssName)
		deleted++
	}

	if failed > 0 && deleted == 0 {
		return fmt.Errorf("failed to delete any of %d stale nodes", failed)
	}
	return nil
}

func isManagedPoolVMSS(vmssName string, managedPoolPrefixes []string) bool {
	for _, prefix := range managedPoolPrefixes {
		if strings.HasPrefix(vmssName, prefix) {
			return true
		}
	}
	return false
}

func ensureResourceGroup(ctx context.Context, location string) (armresources.ResourceGroup, error) {
	resourceGroupName := config.ResourceGroupName(location)
	rg, err := config.Azure.ResourceGroup.CreateOrUpdate(
		ctx,
		resourceGroupName,
		armresources.ResourceGroup{
			Location: to.Ptr(location),
			Name:     to.Ptr(resourceGroupName),
		},
		nil)

	if err != nil {
		return armresources.ResourceGroup{}, fmt.Errorf("creating or updating RG %q: %w", resourceGroupName, err)
	}
	return rg.ResourceGroup, nil
}

// setupPrivateDNSForAPIServer adds an A record for the cluster's API server FQDN
// to the shared private DNS zone. The zone and VNet link are created once by ensureSharedInfra.
func setupPrivateDNSForAPIServer(ctx context.Context, cluster *armcontainerservice.ManagedCluster, vnet VNet) error {
	defer toolkit.LogStepCtx(ctx, "setting up private DNS for API server")()

	fqdn := *cluster.Properties.Fqdn
	nodeRG := *cluster.Properties.NodeResourceGroup

	ips, err := net.LookupHost(fqdn)
	if err != nil {
		return fmt.Errorf("resolving API server FQDN %q: %w", fqdn, err)
	}

	var aRecords []*armprivatedns.ARecord
	for _, ip := range ips {
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
			aRecords = append(aRecords, &armprivatedns.ARecord{IPv4Address: to.Ptr(ip)})
		}
	}
	if len(aRecords) == 0 {
		return fmt.Errorf("no IPv4 addresses for %q", fqdn)
	}

	// Check if zone + record already exist and are up to date
	existing, err := config.Azure.RecordSetClient.Get(ctx, nodeRG, fqdn, armprivatedns.RecordTypeA, "@", nil)
	if err == nil && existing.Properties != nil && existing.Properties.ARecords != nil {
		existingIPs := map[string]bool{}
		for _, r := range existing.Properties.ARecords {
			if r.IPv4Address != nil {
				existingIPs[*r.IPv4Address] = true
			}
		}
		allMatch := len(existingIPs) == len(aRecords)
		if allMatch {
			for _, r := range aRecords {
				if !existingIPs[*r.IPv4Address] {
					allMatch = false
					break
				}
			}
		}
		if allMatch {
			toolkit.Logf(ctx, "private DNS zone %q already up to date", fqdn)
			return nil
		}
	}

	// Per-FQDN zone: zone name = full FQDN, record name = "@"
	if _, err := createPrivateZone(ctx, nodeRG, fqdn); err != nil {
		return fmt.Errorf("creating private zone %q: %w", fqdn, err)
	}
	if err := createPrivateDNSLink(ctx, vnet, nodeRG, fqdn); err != nil {
		return fmt.Errorf("linking private zone to VNet: %w", err)
	}

	_, err = config.Azure.RecordSetClient.CreateOrUpdate(ctx, nodeRG, fqdn, armprivatedns.RecordTypeA, "@",
		armprivatedns.RecordSet{Properties: &armprivatedns.RecordSetProperties{TTL: to.Ptr[int64](300), ARecords: aRecords}}, nil)
	if err != nil {
		return fmt.Errorf("creating A record in zone %q: %w", fqdn, err)
	}

	toolkit.Logf(ctx, "private DNS zone %q → %v", fqdn, ips)
	return nil
}
