package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
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
	Model           *armcontainerservice.ManagedCluster
	Kube            *Kubeclient
	KubeletIdentity *armcontainerservice.UserAssignedIdentity
	SubnetID        string
	ClusterParams   *ClusterParams
	Bastion         *Bastion
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
	defer toolkit.LogStepCtx(ctx, "preparing cluster")()
	ctx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutCluster)
	defer cancel()
	cluster.Name = to.Ptr(fmt.Sprintf("%s-%s", *cluster.Name, hash(cluster)))
	cluster, err := getOrCreateCluster(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create cluster: %w", err)
	}

	bastion, err := getOrCreateBastion(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create bastion: %w", err)
	}

	_, err = getOrCreateMaintenanceConfiguration(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create maintenance configuration: %w", err)
	}

	subnetID, err := getClusterSubnetID(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return nil, fmt.Errorf("get cluster subnet: %w", err)
	}

	resourceGroupName := config.ResourceGroupName(*cluster.Location)

	kube, err := getClusterKubeClient(ctx, resourceGroupName, *cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("get kube client using cluster %q: %w", *cluster.Name, err)
	}

	toolkit.Logf(ctx, "using private acr %q isNonAnonymousPull %v", config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), isNonAnonymousPull)
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

	if err := addFirewallRules(ctx, cluster, *cluster.Location); err != nil {
		return nil, fmt.Errorf("add firewall rules: %w", err)
	}

	kubeletIdentity, err := getClusterKubeletIdentity(cluster)
	if err != nil {
		return nil, fmt.Errorf("getting cluster kubelet identity: %w", err)
	}

	if isNonAnonymousPull {
		if err := assignACRPullToIdentity(ctx, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *kubeletIdentity.ObjectID, *cluster.Location); err != nil {
			return nil, fmt.Errorf("assigning acr pull permissions to kubelet identity: %w", err)
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

	return &Cluster{
		Model:           cluster,
		Kube:            kube,
		KubeletIdentity: kubeletIdentity,
		SubnetID:        subnetID,
		ClusterParams:   clusterParams,
		Bastion:         bastion,
	}, nil
}

func getClusterKubeletIdentity(cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.UserAssignedIdentity, error) {
	if cluster == nil || cluster.Properties == nil || cluster.Properties.IdentityProfile == nil {
		return nil, fmt.Errorf("cannot dereference cluster identity profile to extract kubelet identity ID")
	}
	kubeletIdentity := cluster.Properties.IdentityProfile["kubeletidentity"]
	if kubeletIdentity == nil {
		return nil, fmt.Errorf("kubelet identity is missing from cluster properties")
	}
	return kubeletIdentity, nil
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

		// Check if the error is a 409 Conflict
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			lastErr = err
			toolkit.Logf(ctx, "Attempt %d failed with 409 Conflict: %v. Retrying in %v...", attempt+1, err, retryInterval)

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
	nodeRG := *cluster.Properties.NodeResourceGroup
	bastionName := fmt.Sprintf("%s-bastion", *cluster.Name)

	existing, err := config.Azure.BastionHosts.Get(ctx, nodeRG, bastionName, nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == http.StatusNotFound {
		return createNewBastion(ctx, cluster)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bastion %q in rg %q: %w", bastionName, nodeRG, err)
	}

	return NewBastion(config.Azure.Credential, config.Config.SubscriptionID, nodeRG, *existing.BastionHost.Properties.DNSName), nil
}

func createNewBastion(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*Bastion, error) {
	nodeRG := *cluster.Properties.NodeResourceGroup
	location := *cluster.Location
	bastionName := fmt.Sprintf("%s-bastion", *cluster.Name)
	defer toolkit.LogStepCtxf(ctx, "creating bastion %s", bastionName)()
	publicIPName := fmt.Sprintf("%s-bastion-pip", *cluster.Name)
	publicIPName = sanitizeAzureResourceName(publicIPName)

	vnet, err := getClusterVNet(ctx, nodeRG)
	if err != nil {
		return nil, fmt.Errorf("get cluster vnet in rg %q: %w", nodeRG, err)
	}

	// Azure Bastion requires a dedicated subnet named AzureBastionSubnet. Standard SKU (required for
	// native client support/tunneling) requires at least a /26.
	bastionSubnetName := "AzureBastionSubnet"
	bastionSubnetPrefix := "10.226.0.0/26"
	if _, err := netip.ParsePrefix(bastionSubnetPrefix); err != nil {
		return nil, fmt.Errorf("invalid bastion subnet prefix %q: %w", bastionSubnetPrefix, err)
	}

	var bastionSubnetID string
	bastionSubnet, subnetGetErr := config.Azure.Subnet.Get(ctx, nodeRG, vnet.name, bastionSubnetName, nil)
	if subnetGetErr != nil {
		var subnetAzErr *azcore.ResponseError
		if !errors.As(subnetGetErr, &subnetAzErr) || subnetAzErr.StatusCode != http.StatusNotFound {
			return nil, fmt.Errorf("get subnet %q in vnet %q rg %q: %w", bastionSubnetName, vnet.name, nodeRG, subnetGetErr)
		}

		toolkit.Logf(ctx, "creating subnet %s in VNet %s (rg %s)", bastionSubnetName, vnet.name, nodeRG)
		subnetParams := armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: to.Ptr(bastionSubnetPrefix),
			},
		}
		subnetPoller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, nodeRG, vnet.name, bastionSubnetName, subnetParams, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to start creating bastion subnet: %w", err)
		}
		bastionSubnet, err := subnetPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create bastion subnet: %w", err)
		}
		bastionSubnetID = *bastionSubnet.ID
	} else {
		bastionSubnetID = *bastionSubnet.ID
	}

	// Public IP for Bastion
	pipParams := armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}

	toolkit.Logf(ctx, "creating bastion public IP %s (rg %s)", publicIPName, nodeRG)
	pipPoller, err := config.Azure.PublicIPAddresses.BeginCreateOrUpdate(ctx, nodeRG, publicIPName, pipParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start creating bastion public IP: %w", err)
	}
	pipResp, err := pipPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create bastion public IP: %w", err)
	}
	if pipResp.ID == nil {
		return nil, fmt.Errorf("bastion public IP response missing ID")
	}

	bastionHost := armnetwork.BastionHost{
		Location: to.Ptr(location),
		SKU: &armnetwork.SKU{
			Name: to.Ptr(armnetwork.BastionHostSKUNameStandard),
		},
		Properties: &armnetwork.BastionHostPropertiesFormat{
			// Native client support is enabled via tunneling.
			EnableTunneling: to.Ptr(true),
			IPConfigurations: []*armnetwork.BastionHostIPConfiguration{
				{
					Name: to.Ptr("bastion-ipcfg"),
					Properties: &armnetwork.BastionHostIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.SubResource{
							ID: to.Ptr(bastionSubnetID),
						},
						PublicIPAddress: &armnetwork.SubResource{
							ID: pipResp.ID,
						},
					},
				},
			},
		},
	}

	toolkit.Logf(ctx, "creating bastion %s (native client/tunneling enabled) in rg %s", bastionName, nodeRG)
	bastionPoller, err := config.Azure.BastionHosts.BeginCreateOrUpdate(ctx, nodeRG, bastionName, bastionHost, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start creating bastion: %w", err)
	}
	resp, err := bastionPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create bastion: %w", err)
	}

	bastion := NewBastion(config.Azure.Credential, config.Config.SubscriptionID, nodeRG, *resp.BastionHost.Properties.DNSName)

	if err := verifyBastion(ctx, cluster, bastion); err != nil {
		return nil, fmt.Errorf("failed to verify bastion: %w", err)
	}
	return bastion, nil
}

func verifyBastion(ctx context.Context, cluster *armcontainerservice.ManagedCluster, bastion *Bastion) error {
	nodeRG := *cluster.Properties.NodeResourceGroup
	vmssName, err := getSystemPoolVMSSName(ctx, cluster)
	if err != nil {
		return err
	}

	var vmssVM *armcompute.VirtualMachineScaleSetVM
	pager := config.Azure.VMSSVM.NewListPager(nodeRG, vmssName, nil)
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list vmss vms for %q in rg %q: %w", vmssName, nodeRG, err)
		}
		if len(page.Value) > 0 {
			vmssVM = page.Value[0]
		}
	}

	vmPrivateIP, err := getPrivateIPFromVMSSVM(ctx, nodeRG, vmssName, *vmssVM.InstanceID)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sshClient, err := DialSSHOverBastion(ctx, bastion, vmPrivateIP, config.SysSSHPrivateKey)
	if err != nil {
		return err
	}

	defer sshClient.Close()

	result, err := runSSHCommandWithPrivateKeyFile(ctx, sshClient, "uname -a", false)
	if err != nil {
		return err
	}
	if strings.Contains(result.stdout, vmssName) {
		return nil
	}
	return fmt.Errorf("Executed ssh on wrong VM, Expected %s: %s", vmssName, result.stdout)
}

func getSystemPoolVMSSName(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (string, error) {
	nodeRG := *cluster.Properties.NodeResourceGroup
	var systemPoolName string
	for _, pool := range cluster.Properties.AgentPoolProfiles {
		if strings.EqualFold(string(*pool.Mode), "System") {
			systemPoolName = *pool.Name
		}
	}
	pager := config.Azure.VMSS.NewListPager(nodeRG, nil)
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("list vmss in rg %q: %w", nodeRG, err)
		}
		for _, vmss := range page.Value {
			if strings.Contains(strings.ToLower(*vmss.Name), strings.ToLower(systemPoolName)) {
				return *vmss.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no matching VMSS found for system pool %q in rg %q", systemPoolName, nodeRG)
}

func sanitizeAzureResourceName(name string) string {
	// Azure resource name restrictions vary by type. For our usage here (Public IP name) we just
	// keep it simple and strip problematic characters.
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "_", "-", " ", "-")
	name = replacer.Replace(name)
	name = strings.Trim(name, "-")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
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
	defer toolkit.LogStepCtx(ctx, "collecting garbage VMSS")()
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
				toolkit.Logf(ctx, "failed to delete vmss %q: %s", *vmss.Name, err)
				continue
			}
			toolkit.Logf(ctx, "deleted vmss %q (age: %v)", *vmss.ID, time.Since(*vmss.Properties.TimeCreated))
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
