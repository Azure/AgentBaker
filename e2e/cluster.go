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
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
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

func prepareCluster(ctx context.Context, cluster *armcontainerservice.ManagedCluster, isAirgap, isNonAnonymousPull bool) (*Cluster, error) {
	ctx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutCluster)
	defer cancel()
	cluster.Name = to.Ptr(fmt.Sprintf("%s-%s", *cluster.Name, hash(cluster)))
	cluster, err := getOrCreateCluster(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create cluster: %w", err)
	}

	maintenance, err := getOrCreateMaintenanceConfiguration(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create maintenance configuration: %w", err)
	}

	logf(ctx, "node resource group: %s", *cluster.Properties.NodeResourceGroup)
	subnetID, err := getClusterSubnetID(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return nil, fmt.Errorf("get cluster subnet: %w", err)
	}

	resourceGroupName := config.ResourceGroupName(*cluster.Location)

	kube, err := getClusterKubeClient(ctx, resourceGroupName, *cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("get kube client using cluster %q: %w", *cluster.Name, err)
	}

	logf(ctx, "using private acr %q isAnonyomusPull %v", config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), isNonAnonymousPull)
	if isAirgap {
		// private acr must be created before we add the debug daemonsets
		if err := createPrivateAzureContainerRegistry(ctx, cluster, resourceGroupName, isNonAnonymousPull); err != nil {
			return nil, fmt.Errorf("failed to create private acr: %w", err)
		}

		if err := createPrivateAzureContainerRegistryPullSecret(ctx, cluster, kube, resourceGroupName, isNonAnonymousPull); err != nil {
			return nil, fmt.Errorf("create private acr pull secret: %w", err)
		}

		if err := addAirgapNetworkSettings(ctx, cluster, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *cluster.Location); err != nil {
			return nil, fmt.Errorf("add airgap network settings: %w", err)
		}
	}

	if isNonAnonymousPull {
		identity, err := config.Azure.UserAssignedIdentities.Get(ctx, resourceGroupName, config.VMIdentityName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM identity: %w", err)
		}
		if err := assignACRPullToIdentity(ctx, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *identity.Properties.PrincipalID, *cluster.Location); err != nil {
			return nil, fmt.Errorf("assign acr pull to the managed identity: %w", err)
		}
	}

	if err := kube.EnsureDebugDaemonsets(ctx, isAirgap, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location)); err != nil {
		return nil, fmt.Errorf("ensure debug daemonsets for %q: %w", *cluster.Name, err)
	}

	// sometimes tests can be interrupted and vmss are left behind
	// don't waste resource and delete them
	if err := collectGarbageVMSS(ctx, cluster); err != nil {
		return nil, fmt.Errorf("collect garbage vmss: %w", err)
	}

	clusterParams, err := extractClusterParameters(ctx, kube, cluster)
	if err != nil {
		return nil, fmt.Errorf("extracting cluster parameters: %w", err)
	}

	hostPod, err := kube.GetHostNetworkDebugPod(ctx)
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

func extractClusterParameters(ctx context.Context, kube *Kubeclient, cluster *armcontainerservice.ManagedCluster) (*ClusterParams, error) {
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
	logf(ctx, "assigning ACR-Pull role to %s", principalID)
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
		logf(ctx, "failed to assign ACR-Pull role to identity %s, error: %v", config.VMIdentityName, err)
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
		nodeRGExists, err := isExistingResourceGroup(ctx, *existingCluster.Properties.NodeResourceGroup)

		if err != nil {
			return nil, err
		}
		// ensure MC_rg as well --> functioning. during cluster provisioning, the node resource group may not exist yet and we can wait
		if nodeRGExists {
			return &existingCluster.ManagedCluster, nil
		}
		fallthrough
	case "Failed":
		logf(ctx, "echo \"##vso[task.logissue type=warning;]Cluster %s in Failed state\"", clusterName)
		derr := deleteCluster(ctx, clusterName, resourceGroupName)
		if derr != nil {
			return nil, derr
		}
		return nil, nil
	default:
		// other provisioning state,  deleting, , stopping,,cancaled,cancelling,"Creating", "Updating", "Scaling", "Migrating", "Upgrading", "Starting", "Restoring": .. plus many others.
		logf(ctx, "echo \"##vso[task.logissue type=warning;] Unexpected cluster provisioning state.\" %s: %s", clusterName, *existingCluster.Properties.ProvisioningState)
		return waitUntilClusterReady(ctx, clusterName, location)
	}
}

func deleteCluster(ctx context.Context, clusterName, resourceGroupName string) error {
	logf(ctx, "deleting cluster %s in rg %s", clusterName, resourceGroupName)
	// beileih: why do we do this?
	_, err := config.Azure.AKS.Get(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == 404 {
			logf(ctx, "cluster %s does not exist in rg %s", clusterName, resourceGroupName)
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
	logf(ctx, "deleted cluster %s in rg %s", clusterName, resourceGroupName)
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
		return nil, fmt.Errorf("failed to wait for cluster %s to be ready: %w", name, err)
	}
	return &cluster.ManagedCluster, nil
}

func isExistingResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	rgExistence, err := config.Azure.ResourceGroup.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func createNewAKSCluster(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	logf(ctx, "creating or updating cluster %s in rg %s", *cluster.Name, *cluster.Location)
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
		logf(ctx, "Attempt %d: creating or updating cluster %s in region %s and rg %s", attempt+1, *cluster.Name, *cluster.Location, config.ResourceGroupName(*cluster.Location))

		createdCluster, err := createNewAKSCluster(ctx, cluster)
		if err == nil {
			return createdCluster, nil
		}

		// Check if the error is a 409 Conflict
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			lastErr = err
			logf(ctx, "Attempt %d failed with 409 Conflict: %v. Retrying in %v...", attempt+1, err, retryInterval)

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

func getOrCreateMaintenanceConfiguration(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.MaintenanceConfiguration, error) {
	existingMaintenance, err := config.Azure.Maintenance.Get(ctx, config.ResourceGroupName(*cluster.Location), *cluster.Name, "default", nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == 404 {
		return createNewMaintenanceConfiguration(ctx, cluster)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get maintenance configuration 'default' for cluster %q: %w", *cluster.Name, err)
	}
	return &existingMaintenance.MaintenanceConfiguration, nil
}

func createNewMaintenanceConfiguration(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.MaintenanceConfiguration, error) {
	logf(ctx, "creating maintenance configuration for cluster %s in rg %s", *cluster.Name, config.ResourceGroupName(*cluster.Location))
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

func collectGarbageVMSS(ctx context.Context, cluster *armcontainerservice.ManagedCluster) error {
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
				logf(ctx, "failed to delete vmss %q: %s", *vmss.Name, err)
				continue
			}
			logf(ctx, "deleted vmss %q (age: %v)", *vmss.ID, time.Since(*vmss.Properties.TimeCreated))
		}
	}

	return nil
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
