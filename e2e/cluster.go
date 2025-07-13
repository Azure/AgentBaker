package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterSingleton struct {
	cluster *Cluster
	once    *sync.Once
	err     error
}

type ClusterCollection struct {
	latestKubernetesVersion      *ClusterSingleton
	kubenet                      *ClusterSingleton
	kubenetAirgap                *ClusterSingleton
	kubenetNonAnonAirgap         *ClusterSingleton
	azureNetwork                 *ClusterSingleton
	azureOverlayNetwork          *ClusterSingleton
	azureOverlayNetworkDualStack *ClusterSingleton
	ciliumNetwork                *ClusterSingleton
}

var (
	clusterCache = make(map[string]*ClusterCollection)
	cacheMutex   sync.RWMutex
)

type ClusterParams struct {
	CACert         []byte
	BootstrapToken string
	FQDN           string
}

type Cluster struct {
	Model         *armcontainerservice.ManagedCluster
	Kube          *Kubeclient
	SubnetID      string
	ClusterParams *ClusterParams
	Maintenance   *armcontainerservice.MaintenanceConfiguration
	DebugPod      *corev1.Pod
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

func getOrInitializeClusterCache(location string) *ClusterCollection {
	// First, try with read lock for common case
	cacheMutex.RLock()
	if collection, exists := clusterCache[location]; exists {
		cacheMutex.RUnlock()
		return collection
	}
	cacheMutex.RUnlock()

	// Need to initialize - acquire write lock
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if collection, exists := clusterCache[location]; exists {
		return collection
	}

	// Initialize new collection
	collection := &ClusterCollection{
		latestKubernetesVersion:      &ClusterSingleton{once: &sync.Once{}},
		kubenet:                      &ClusterSingleton{once: &sync.Once{}},
		kubenetAirgap:                &ClusterSingleton{once: &sync.Once{}},
		kubenetNonAnonAirgap:         &ClusterSingleton{once: &sync.Once{}},
		azureNetwork:                 &ClusterSingleton{once: &sync.Once{}},
		azureOverlayNetwork:          &ClusterSingleton{once: &sync.Once{}},
		azureOverlayNetworkDualStack: &ClusterSingleton{once: &sync.Once{}},
		ciliumNetwork:                &ClusterSingleton{once: &sync.Once{}},
	}
	clusterCache[location] = collection
	return collection
}

// Same cluster can be attempted to be created concurrently by different tests
// sync.Once is used to ensure that only one cluster for the set of tests is created

func ClusterLatestKubernetesVersion(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.latestKubernetesVersion.once.Do(func() {
		model, error := getLatestKubernetesVersionClusterModel("abe2e-latest-kubernetes-version", location, t)
		if error != nil {
			t.Fatalf("failed to get latest kubernetes version cluster model: %v", error)
		}
		collection.latestKubernetesVersion.cluster, collection.latestKubernetesVersion.err = prepareCluster(ctx, t, model, false, false)
	})
	return collection.latestKubernetesVersion.cluster, collection.latestKubernetesVersion.err
}

func ClusterKubenet(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.kubenet.once.Do(func() {
		collection.kubenet.cluster, collection.kubenet.err = prepareCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet", location), false, false)
	})
	return collection.kubenet.cluster, collection.kubenet.err
}

func ClusterKubenetAirgap(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.kubenetAirgap.once.Do(func() {
		collection.kubenetAirgap.cluster, collection.kubenetAirgap.err = prepareCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet-airgap", location), true, false)
	})
	return collection.kubenetAirgap.cluster, collection.kubenetAirgap.err
}

func ClusterKubenetAirgapNonAnon(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.kubenetNonAnonAirgap.once.Do(func() {
		collection.kubenetNonAnonAirgap.cluster, collection.kubenetNonAnonAirgap.err = prepareCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet-nonanonpull-airgap", location), true, true)
	})
	return collection.kubenetNonAnonAirgap.cluster, collection.kubenetNonAnonAirgap.err
}

func ClusterAzureNetwork(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.azureNetwork.once.Do(func() {
		collection.azureNetwork.cluster, collection.azureNetwork.err = prepareCluster(ctx, t, getAzureNetworkClusterModel("abe2e-azure-network", location), false, false)
	})
	return collection.azureNetwork.cluster, collection.azureNetwork.err
}

func ClusterAzureOverlayNetwork(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.azureOverlayNetwork.once.Do(func() {
		collection.azureOverlayNetwork.cluster, collection.azureOverlayNetwork.err = prepareCluster(ctx, t, getAzureOverlayNetworkClusterModel("abe2e-azure-overlay-network", location), false, false)
	})
	return collection.azureOverlayNetwork.cluster, collection.azureOverlayNetwork.err
}

func ClusterAzureOverlayNetworkDualStack(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.azureOverlayNetworkDualStack.once.Do(func() {
		collection.azureOverlayNetworkDualStack.cluster, collection.azureOverlayNetworkDualStack.err = prepareCluster(ctx, t, getAzureOverlayNetworkDualStackClusterModel("abe2e-azure-overlay-dualstack", location), false, false)
	})
	return collection.azureOverlayNetworkDualStack.cluster, collection.azureOverlayNetworkDualStack.err
}

func ClusterCiliumNetwork(ctx context.Context, location string, t *testing.T) (*Cluster, error) {
	collection := getOrInitializeClusterCache(location)
	collection.ciliumNetwork.once.Do(func() {
		collection.ciliumNetwork.cluster, collection.ciliumNetwork.err = prepareCluster(ctx, t, getCiliumNetworkClusterModel("abe2e-cilium-network", location), false, false)
	})
	return collection.ciliumNetwork.cluster, collection.ciliumNetwork.err
}

func prepareCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster, isAirgap, isNonAnonymousPull bool) (*Cluster, error) {
	ctx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutCluster)
	defer cancel()
	cluster.Name = to.Ptr(fmt.Sprintf("%s-%s", *cluster.Name, hash(cluster)))
	cluster, err := getOrCreateCluster(ctx, t, cluster)
	if err != nil {
		return nil, err
	}

	maintenance, err := getOrCreateMaintenanceConfiguration(ctx, t, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create maintenance configuration: %w", err)
	}

	t.Logf("node resource group: %s", *cluster.Properties.NodeResourceGroup)
	subnetID, err := getClusterSubnetID(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return nil, fmt.Errorf("get cluster subnet: %w", err)
	}

	resourceGroupName := config.ResourceGroupName(*cluster.Location)

	kube, err := getClusterKubeClient(ctx, resourceGroupName, *cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("get kube client using cluster %q: %w", *cluster.Name, err)
	}

	t.Logf("using private acr %q isAnonyomusPull %v", config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), isNonAnonymousPull)
	if isAirgap {
		// private acr must be created before we add the debug daemonsets
		if err := createPrivateAzureContainerRegistry(ctx, t, cluster, kube, resourceGroupName, isNonAnonymousPull); err != nil {
			return nil, fmt.Errorf("failed to create private acr: %w", err)
		}

		if err := createPrivateAzureContainerRegistryPullSecret(ctx, t, cluster, kube, resourceGroupName, isNonAnonymousPull); err != nil {
			return nil, fmt.Errorf("create private acr pull secret: %w", err)
		}

		if err := addAirgapNetworkSettings(ctx, t, cluster, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *cluster.Location); err != nil {
			return nil, fmt.Errorf("add airgap network settings: %w", err)
		}
	}

	if isNonAnonymousPull {
		identity, err := config.Azure.UserAssignedIdentities.Get(ctx, resourceGroupName, config.VMIdentityName, nil)
		if err != nil {
			t.Fatalf("failed to get VM identity: %v", err)
		}
		if err := assignACRPullToIdentity(ctx, t, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *identity.Properties.PrincipalID, *cluster.Location); err != nil {
			return nil, fmt.Errorf("assign acr pull to the managed identity: %w", err)
		}
	}

	if err := kube.EnsureDebugDaemonsets(ctx, t, isAirgap, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location)); err != nil {
		return nil, fmt.Errorf("ensure debug daemonsets for %q: %w", *cluster.Name, err)
	}

	// sometimes tests can be interrupted and vmss are left behind
	// don't waste resource and delete them
	if err := collectGarbageVMSS(ctx, t, cluster); err != nil {
		return nil, fmt.Errorf("collect garbage vmss: %w", err)
	}

	clusterParams, err := extractClusterParameters(ctx, t, kube, cluster)
	if err != nil {
		return nil, fmt.Errorf("extracting cluster parameters: %w", err)
	}

	hostPod, err := kube.GetHostNetworkDebugPod(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("get host network debug pod: %w", err)
	}

	return &Cluster{
		Model:         cluster,
		Kube:          kube,
		SubnetID:      subnetID,
		Maintenance:   maintenance,
		ClusterParams: clusterParams,
		DebugPod:      hostPod,
	}, nil
}

func extractClusterParameters(ctx context.Context, t *testing.T, kube *Kubeclient, cluster *armcontainerservice.ManagedCluster) (*ClusterParams, error) {
	kubeconfig, err := clientcmd.Load(kube.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("loading cluster kubeconfig: %w", err)
	}
	clusterConfig := kubeconfig.Clusters[*cluster.Name]
	if clusterConfig == nil {
		return nil, fmt.Errorf("cluster kubeconfig missing configuration for %s", *cluster.Name)
	}
	return &ClusterParams{
		CACert:         clusterConfig.CertificateAuthorityData,
		BootstrapToken: getBootstrapToken(ctx, t, kube),
		FQDN:           *cluster.Properties.Fqdn,
	}, nil
}

func assignACRPullToIdentity(ctx context.Context, t *testing.T, privateACRName, principalID string, location string) error {
	t.Logf("assigning ACR-Pull role to %s", principalID)
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
		t.Logf("failed to assign ACR-Pull role to identity %s, error: %v", config.VMIdentityName, err)
		return err
	}
	return nil
}

func getBootstrapToken(ctx context.Context, t *testing.T, kube *Kubeclient) string {
	secrets, err := kube.Typed.CoreV1().Secrets("kube-system").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	secret := func() *corev1.Secret {
		for _, secret := range secrets.Items {
			if strings.HasPrefix(secret.Name, "bootstrap-token-") {
				return &secret
			}
		}
		t.Fatal("could not find secret with bootstrap-token- prefix")
		return nil
	}()
	id := secret.Data["token-id"]
	token := secret.Data["token-secret"]
	return fmt.Sprintf("%s.%s", id, token)
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

func getOrCreateCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	resourceGroupName := config.ResourceGroupName(*cluster.Location)

	existingCluster, err := config.Azure.AKS.Get(ctx, resourceGroupName, *cluster.Name, nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == 404 {
		return createNewAKSClusterWithRetry(ctx, t, cluster)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %q: %w", *cluster.Name, err)
	}
	t.Logf("cluster %s already exists in rg %s", *cluster.Name, resourceGroupName)
	switch *existingCluster.Properties.ProvisioningState {
	case "Succeeded":
		nodeRGExists, err := isExistingResourceGroup(ctx, *existingCluster.Properties.NodeResourceGroup)
		if err != nil {
			return nil, fmt.Errorf("checking node resource group existence of cluster %s: %w", *cluster.Name, err)
		}
		if !nodeRGExists {
			// we need to recreate in the case where the cluster is in the "Succeeded" provisioning state,
			// though it's corresponding node resource group has been garbage collected
			t.Logf("node resource group of cluster %s does not exist, will attempt to recreate", *cluster.Name)
			return createNewAKSClusterWithRetry(ctx, t, cluster)
		}
		return &existingCluster.ManagedCluster, nil
	case "Creating", "Updating":
		return waitUntilClusterReady(ctx, *cluster.Name, *cluster.Location)
	default:
		// this operation will try to update the cluster if it's in a failed state
		return createNewAKSClusterWithRetry(ctx, t, cluster)
	}
}

func deleteCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) error {
	resourceGroupName := config.ResourceGroupName(*cluster.Location)
	t.Logf("deleting cluster %s in rg %s", *cluster.Name, resourceGroupName)
	_, err := config.Azure.AKS.Get(ctx, resourceGroupName, *cluster.Name, nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == 404 {
			t.Logf("cluster %s does not exist in rg %s", *cluster.Name, resourceGroupName)
			return nil
		}
		return fmt.Errorf("failed to get cluster %q: %w", *cluster.Name, err)
	}

	pollerResp, err := config.Azure.AKS.BeginDelete(ctx, resourceGroupName, *cluster.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete cluster %q: %w", *cluster.Name, err)
	}
	_, err = pollerResp.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster deletion %w", err)
	}
	t.Logf("deleted cluster %s in rg %s", *cluster.Name, resourceGroupName)
	return nil
}

func waitUntilClusterReady(ctx context.Context, name, location string) (*armcontainerservice.ManagedCluster, error) {
	var cluster armcontainerservice.ManagedClustersClientGetResponse
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		var err error
		cluster, err = config.Azure.AKS.Get(ctx, config.ResourceGroupName(location), name, nil)
		if err != nil {
			return false, err
		}
		switch *cluster.ManagedCluster.Properties.ProvisioningState {
		case "Succeeded":
			return true, nil
		case "Updating", "Assigned", "Creating":
			return false, nil
		default:
			return false, fmt.Errorf("cluster %s is in state %s", name, *cluster.ManagedCluster.Properties.ProvisioningState)
		}
	})
	if err != nil {
		return nil, err
	}
	return &cluster.ManagedCluster, err
}

func isExistingResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	rgExistence, err := config.Azure.ResourceGroup.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func createNewAKSCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	t.Logf("creating or updating cluster %s in rg %s", *cluster.Name, *cluster.Location)
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
func createNewAKSClusterWithRetry(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	maxRetries := 10
	retryInterval := 30 * time.Second
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		t.Logf("Attempt %d: creating or updating cluster %s in region %s and rg %s", attempt+1, *cluster.Name, *cluster.Location, config.ResourceGroupName(*cluster.Location))

		createdCluster, err := createNewAKSCluster(ctx, t, cluster)
		if err == nil {
			return createdCluster, nil
		}

		// Check if the error is a 409 Conflict
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			lastErr = err
			t.Logf("Attempt %d failed with 409 Conflict: %v. Retrying in %v...", attempt+1, err, retryInterval)

			select {
			case <-time.After(retryInterval):
				// Continue to next iteration
			case <-ctx.Done():
				return nil, fmt.Errorf("context canceled while retrying cluster creation: %w", ctx.Err())
			}
		} else {
			// If it's not a 409 error, return immediately
			return nil, fmt.Errorf("failed to create cluster: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to create cluster after %d attempts due to persistent 409 Conflict: %w", maxRetries, lastErr)
}

func getOrCreateMaintenanceConfiguration(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.MaintenanceConfiguration, error) {
	existingMaintenance, err := config.Azure.Maintenance.Get(ctx, config.ResourceGroupName(*cluster.Location), *cluster.Name, "default", nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == 404 {
		return createNewMaintenanceConfiguration(ctx, t, cluster)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get maintenance configuration 'default' for cluster %q: %w", *cluster.Name, err)
	}
	return &existingMaintenance.MaintenanceConfiguration, nil
}

func createNewMaintenanceConfiguration(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.MaintenanceConfiguration, error) {
	t.Logf("creating maintenance configuration for cluster %s in rg %s", *cluster.Name, config.ResourceGroupName(*cluster.Location))
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
						IntervalWeeks: to.Ptr[int32](4),
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

type VNet struct {
	name     string
	subnetId string
}

func getClusterVNet(ctx context.Context, mcResourceGroupName string) (VNet, error) {
	pager := config.Azure.VNet.NewListPager(mcResourceGroupName, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return VNet{}, fmt.Errorf("failed to advance page: %w", err)
		}
		for _, v := range nextResult.Value {
			if v == nil {
				return VNet{}, fmt.Errorf("aks vnet was empty")
			}
			return VNet{name: *v.Name, subnetId: fmt.Sprintf("%s/subnets/%s", *v.ID, "aks-subnet")}, nil
		}
	}
	return VNet{}, fmt.Errorf("failed to find aks vnet")
}

func collectGarbageVMSS(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) error {
	rg := *cluster.Properties.NodeResourceGroup
	pager := config.Azure.VMSS.NewListPager(rg, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page of VMSS: %w", err)
		}
		for _, vmss := range page.Value {
			if _, ok := vmss.Tags["KEEP_VMSS"]; ok {
				continue
			}
			// don't delete managed pools
			if _, ok := vmss.Tags["aks-managed-poolName"]; ok {
				continue
			}

			// don't delete VMSS created in the last hour. They might be currently used in tests
			// extra 10 minutes is a buffer for test cleanup, clock drift and timeout adjustments
			if config.Config.TestTimeout == 0 || time.Since(*vmss.Properties.TimeCreated) < config.Config.TestTimeout+10*time.Minute {
				continue
			}

			_, err := config.Azure.VMSS.BeginDelete(ctx, rg, *vmss.Name, &armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{
				ForceDeletion: to.Ptr(true),
			})
			if err != nil {
				t.Logf("failed to delete vmss %q: %s", *vmss.Name, err)
				continue
			}
			t.Logf("deleted garbage vmss %q", *vmss.ID)
		}
	}

	return nil
}

func ensureResourceGroup(ctx context.Context, location string) error {
	resourceGroupName := config.ResourceGroupName(location)
	_, err := config.Azure.ResourceGroup.CreateOrUpdate(
		ctx,
		resourceGroupName,
		armresources.ResourceGroup{
			Location: to.Ptr(location),
			Name:     to.Ptr(resourceGroupName),
		},
		nil)

	if err != nil {
		return fmt.Errorf("failed to create RG %q: %w", resourceGroupName, err)
	}
	return nil
}
