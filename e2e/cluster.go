package e2e_test

import (
	"context"
	"fmt"
	"log"
	mrand "math/rand"
	"strings"

	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/agentbakere2e/suite"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"k8s.io/apimachinery/pkg/util/errors"
)

const (
	managedClusterResourceType = "Microsoft.ContainerService/managedClusters"

	vmSizeStandardDS2v2 = "Standard_DS2_v2"
)

type clusterParameters map[string]string

type clusterConfig struct {
	cluster      *armcontainerservice.ManagedCluster
	kube         *kubeclient
	parameters   clusterParameters
	subnetId     string
	isNewCluster bool
}

// Returns true if the cluster is configured with Azure CNI
func (c clusterConfig) isAzureCNI() (bool, error) {
	if c.cluster.Properties.NetworkProfile != nil {
		return *c.cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginAzure, nil
	}
	return false, fmt.Errorf("cluster network profile was nil: %+v", c.cluster)
}

// Returns the maximum number of pods per node of the cluster's agentpool
func (c clusterConfig) maxPodsPerNode() (int, error) {
	if len(c.cluster.Properties.AgentPoolProfiles) > 0 {
		return int(*c.cluster.Properties.AgentPoolProfiles[0].MaxPods), nil
	}
	return 0, fmt.Errorf("cluster agentpool profiles were nil or empty: %+v", c.cluster)
}

func (c clusterConfig) needsPreparation() bool {
	return c.kube == nil || c.parameters == nil || c.subnetId == ""
}

// This map is used during cluster creation to check what VM size should
// be used to create the single default agentpool used for running cluster essentials
// and the jumpbox pods/resources. This is mainly need for regions where we can't use
// the default Standard_DS2_v2 VM size due to quota/capacity.
var locationToDefaultClusterAgentPoolVMSize = map[string]string{
	// TODO: add mapping for southcentralus to perform h100 testing
}

func getDefaultAgentPoolVMSize(location string) string {
	defaultAgentPoolVMSize, hasDefaultAgentPoolVMSizeForLocation := locationToDefaultClusterAgentPoolVMSize[location]
	if !hasDefaultAgentPoolVMSizeForLocation {
		defaultAgentPoolVMSize = vmSizeStandardDS2v2
	}
	return defaultAgentPoolVMSize
}

func isExistingResourceGroup(ctx context.Context, cloud *azureClient, resourceGroupName string) (bool, error) {
	rgExistence, err := cloud.resourceGroupClient.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func ensureResourceGroup(ctx context.Context, cloud *azureClient, suiteConfig *suite.Config) error {
	log.Printf("ensuring resource group %q...", suiteConfig.ResourceGroupName)

	rgExists, err := isExistingResourceGroup(ctx, cloud, suiteConfig.ResourceGroupName)
	if err != nil {
		return err
	}

	if !rgExists {
		_, err = cloud.resourceGroupClient.CreateOrUpdate(
			ctx,
			suiteConfig.ResourceGroupName,
			armresources.ResourceGroup{
				Location: to.Ptr(suiteConfig.Location),
				Name:     to.Ptr(suiteConfig.ResourceGroupName),
			},
			nil)

		if err != nil {
			return fmt.Errorf("failed to create RG %q: %w", suiteConfig.ResourceGroupName, err)
		}
	}

	return nil
}

func validateExistingClusterState(ctx context.Context, cloud *azureClient, resourceGroupName, clusterName string) (bool, error) {
	var needRecreate bool
	clusterResp, err := cloud.aksClient.Get(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		if isResourceNotFoundError(err) {
			log.Printf("received ResourceNotFound error when trying to GET test cluster %q", clusterName)
			needRecreate = true
		} else {
			return false, fmt.Errorf("failed to get aks cluster %q: %w", clusterName, err)
		}
	} else {
		cluster := &clusterResp.ManagedCluster
		if *cluster.Properties.ProvisioningState == "Creating" {
			cl, err := waitForClusterCreation(ctx, cloud, resourceGroupName, clusterName)
			if err != nil {
				return false, err
			}
			cluster = cl
		}

		// We only need to check the MC resource group + cluster properties if the cluster resource itself exists
		rgExists, err := isExistingResourceGroup(ctx, cloud, *cluster.Properties.NodeResourceGroup)
		if err != nil {
			return false, err
		}

		if !rgExists || cluster.Properties == nil || cluster.Properties.ProvisioningState == nil || *cluster.Properties.ProvisioningState == "Failed" {
			log.Printf("deleting test cluster in bad state: %q", clusterName)

			needRecreate = true
			if err := deleteExistingCluster(ctx, cloud, resourceGroupName, clusterName); err != nil {
				return false, fmt.Errorf("failed to delete cluster in bad state: %w", err)
			}
		}
	}

	return needRecreate, nil
}

func createNewCluster(
	ctx context.Context,
	cloud *azureClient,
	resourceGroupName string,
	clusterModel *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	pollerResp, err := cloud.aksClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		*clusterModel.Name,
		*clusterModel,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin aks cluster creation: %w", err)
	}

	clusterResp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for aks cluster creation %w", err)
	}

	return &clusterResp.ManagedCluster, nil
}

func deleteExistingCluster(ctx context.Context, cloud *azureClient, resourceGroupName, clusterName string) error {
	poller, err := cloud.aksClient.BeginDelete(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to start aks cluster %q deletion: %w", clusterName, err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for aks cluster %q deletion: %w", clusterName, err)
	}

	return nil
}

func getClusterSubnetID(ctx context.Context, cloud *azureClient, location, mcResourceGroupName, clusterName string) (string, error) {
	pager := cloud.vnetClient.NewListPager(mcResourceGroupName, nil)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to advance page: %w", err)
		}
		for _, v := range nextResult.Value {
			if v == nil {
				return "", fmt.Errorf("aks vnet id was empty")
			}
			return fmt.Sprintf("%s/subnets/%s", *v.ID, "aks-subnet"), nil
		}
	}

	return "", fmt.Errorf("failed to find aks vnet")
}

func getInitialClusterConfigs(ctx context.Context, cloud *azureClient, resourceGroupName string) ([]clusterConfig, error) {
	var configs []clusterConfig
	pager := cloud.resourceClient.NewListByResourceGroupPager(resourceGroupName, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to advance page: %w", err)
		}
		for _, resource := range page.Value {
			if strings.EqualFold(*resource.Type, managedClusterResourceType) {
				cluster, err := cloud.aksClient.Get(ctx, resourceGroupName, *resource.Name, nil)
				if err != nil {
					if isNotFoundError(err) {
						log.Printf("get aks cluster %q returned 404 Not Found, continuing to list clusters...", *resource.Name)
						continue
					} else {
						return nil, fmt.Errorf("failed to get aks cluster: %w", err)
					}
				}
				if cluster.Properties == nil || cluster.Properties.ProvisioningState == nil {
					return nil, fmt.Errorf("aks cluster %q properties/provisioning state were nil", *resource.Name)
				}

				if *cluster.Properties.ProvisioningState == "Deleting" {
					continue
				}

				log.Printf("found agentbaker e2e cluster %q in provisioning state %q", *resource.Name, *cluster.Properties.ProvisioningState)
				configs = append(configs, clusterConfig{cluster: &cluster.ManagedCluster})
			}
		}
	}

	return configs, nil
}

func hasViableConfig(scenario *scenario.Scenario, clusterConfigs []clusterConfig) bool {
	for _, config := range clusterConfigs {
		if scenario.Config.ClusterSelector(config.cluster) {
			return true
		}
	}
	return false
}

func createMissingClusters(
	ctx context.Context,
	r *mrand.Rand,
	cloud *azureClient,
	suiteConfig *suite.Config,
	scenarios scenario.Table,
	clusterConfigs *[]clusterConfig) error {
	var newConfigs []clusterConfig
	for _, scenario := range scenarios {
		if !hasViableConfig(scenario, *clusterConfigs) && !hasViableConfig(scenario, newConfigs) {
			newClusterModel := getNewClusterModelForScenario(generateClusterName(r), suiteConfig.Location, scenario)
			newConfigs = append(newConfigs, clusterConfig{cluster: &newClusterModel, isNewCluster: true})
		}
	}

	var createFuncs []func() error
	for i, c := range newConfigs {
		config := c
		idx := i
		createFunc := func() error {
			clusterName := *config.cluster.Name

			log.Printf("creating cluster %q...", clusterName)
			liveCluster, err := createNewCluster(ctx, cloud, suiteConfig.ResourceGroupName, config.cluster)
			if err != nil {
				return fmt.Errorf("unable to create new cluster: %w", err)
			}

			if liveCluster.Properties == nil {
				return fmt.Errorf("newly created cluster model has nil properties:\n%+v", liveCluster)
			}

			log.Printf("preparing cluster %q for testing...", clusterName)
			kube, subnetId, clusterParams, err := prepareClusterForTests(ctx, cloud, suiteConfig, liveCluster)
			if err != nil {
				return fmt.Errorf("unable to prepare viable cluster for testing: %s", err)
			}

			newConfigs[idx].cluster = liveCluster
			newConfigs[idx].kube = kube
			newConfigs[idx].parameters = clusterParams
			newConfigs[idx].subnetId = subnetId
			return nil
		}

		createFuncs = append(createFuncs, createFunc)
	}

	if err := errors.AggregateGoroutines(createFuncs...); err != nil {
		return fmt.Errorf("at least one cluster creation routine returned an error:\n%w", err)
	}

	*clusterConfigs = append(*clusterConfigs, newConfigs...)
	return nil
}

func chooseCluster(
	ctx context.Context,
	r *mrand.Rand,
	cloud *azureClient,
	suiteConfig *suite.Config,
	scenario *scenario.Scenario,
	clusterConfigs []clusterConfig) (clusterConfig, error) {
	var chosenConfig clusterConfig
	for i := range clusterConfigs {
		config := &clusterConfigs[i]
		if scenario.Config.ClusterSelector(config.cluster) {
			// only validate + prep the cluster for testing if we didn't just create it and it hasn't already been prepared
			if !config.isNewCluster && config.needsPreparation() {
				if err := validateAndPrepareCluster(ctx, r, cloud, suiteConfig, config); err != nil {
					log.Printf("unable to validate and preprare cluster %q: %s", *config.cluster.Name, err)
					continue
				}
			}
			chosenConfig = *config
			break
		}
	}

	if chosenConfig.cluster == nil || chosenConfig.needsPreparation() {
		return clusterConfig{}, fmt.Errorf("unable to successfully choose a cluster for scenario %q", scenario.Name)
	}

	if chosenConfig.cluster.Properties.NodeResourceGroup == nil {
		return clusterConfig{}, fmt.Errorf("tried to chose a cluster without a node resource group: %+v", *chosenConfig.cluster)
	}

	return chosenConfig, nil
}

func validateAndPrepareCluster(ctx context.Context, r *mrand.Rand, cloud *azureClient, suiteConfig *suite.Config, config *clusterConfig) error {
	needRecreate, err := validateExistingClusterState(ctx, cloud, suiteConfig.ResourceGroupName, *config.cluster.Name)
	if err != nil {
		return err
	}

	if needRecreate {
		log.Printf("cluster %q is in a bad state, creating a replacement...", *config.cluster.Name)
		newModel, err := prepareClusterModelForRecreate(r, config.cluster)
		if err != nil {
			return err
		}
		newCluster, err := createNewCluster(ctx, cloud, suiteConfig.ResourceGroupName, newModel)
		if err != nil {
			return err
		}
		log.Printf("replaced bad cluster %q with new cluster %q", *config.cluster.Name, *newModel.Name)
		config.cluster = newCluster
	}

	kube, subnetId, clusterParams, err := prepareClusterForTests(ctx, cloud, suiteConfig, config.cluster)
	if err != nil {
		return err
	}

	config.kube = kube
	config.parameters = clusterParams
	config.subnetId = subnetId
	return nil
}

func prepareClusterForTests(
	ctx context.Context,
	cloud *azureClient,
	suiteConfig *suite.Config,
	cluster *armcontainerservice.ManagedCluster) (*kubeclient, string, clusterParameters, error) {
	clusterName := *cluster.Name

	subnetId, err := getClusterSubnetID(ctx, cloud, suiteConfig.Location, *cluster.Properties.NodeResourceGroup, clusterName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable get subnet ID of cluster %q: %w", clusterName, err)
	}

	kube, err := getClusterKubeClient(ctx, cloud, suiteConfig.ResourceGroupName, clusterName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable get kube client using cluster %q: %w", clusterName, err)
	}

	if err := ensureDebugDaemonset(ctx, kube); err != nil {
		return nil, "", nil, fmt.Errorf("unable to ensure debug damonset of viable cluster %q: %w", clusterName, err)
	}

	clusterParams, err := pollExtractClusterParameters(ctx, kube)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable to extract cluster parameters from %q: %w", clusterName, err)
	}

	return kube, subnetId, clusterParams, nil
}

// TODO(cameissner): figure out a better way to reconcile server-side and client-side properties,
// for now we simply regenerate a new base model and manually patch its properties according to the original model
func prepareClusterModelForRecreate(r *mrand.Rand, clusterModel *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	if clusterModel == nil || clusterModel.Properties == nil {
		return nil, fmt.Errorf("unable to prepare cluster model for recreate, got nil cluster model/properties")
	}
	if clusterModel.Properties.NetworkProfile == nil || clusterModel.Properties.NetworkProfile.NetworkPlugin == nil {
		return nil, fmt.Errorf("unable to prepare cluster model for recreate, got nil network profile/plugin")
	}

	newModel := getBaseClusterModel(generateClusterName(r), *clusterModel.Location)

	// patch new model according to original model properties
	newModel.Properties.NetworkProfile = &armcontainerservice.NetworkProfile{
		NetworkPlugin: to.Ptr(*clusterModel.Properties.NetworkProfile.NetworkPlugin),
	}

	return &newModel, nil
}

func getNewClusterModelForScenario(clusterName, location string, scenario *scenario.Scenario) armcontainerservice.ManagedCluster {
	baseModel := getBaseClusterModel(clusterName, location)
	if scenario.ClusterMutator != nil {
		scenario.ClusterMutator(&baseModel)
	}
	return baseModel
}

func generateClusterName(r *mrand.Rand) string {
	return fmt.Sprintf(testClusterNameTemplate, randomLowercaseString(r, 5))
}

func getBaseClusterModel(clusterName, location string) armcontainerservice.ManagedCluster {
	defaultAgentPoolVMSize := getDefaultAgentPoolVMSize(location)
	log.Printf("will attempt to use VM size %q for default agentpool of cluster %q", defaultAgentPoolVMSize, clusterName)

	return armcontainerservice.ManagedCluster{
		Name:     to.Ptr(clusterName),
		Location: to.Ptr(location),
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: to.Ptr(clusterName),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         to.Ptr("nodepool1"),
					Count:        to.Ptr[int32](2),
					VMSize:       to.Ptr(defaultAgentPoolVMSize),
					MaxPods:      to.Ptr[int32](110),
					OSType:       to.Ptr(armcontainerservice.OSTypeLinux),
					Type:         to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					Mode:         to.Ptr(armcontainerservice.AgentPoolModeSystem),
					OSDiskSizeGB: to.Ptr[int32](512),
				},
			},
			NetworkProfile: &armcontainerservice.NetworkProfile{
				NetworkPlugin: to.Ptr(armcontainerservice.NetworkPluginKubenet),
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}
}
