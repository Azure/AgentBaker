package e2e

import (
	"context"
	"fmt"
	"log"
	mrand "math/rand"
	"strings"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"k8s.io/apimachinery/pkg/util/errors"
)

const (
	managedClusterResourceType = "Microsoft.ContainerService/managedClusters"

	vmSizeStandardDS2v2 = "Standard_DS2_v2"
)

type clusterParameters map[string]string

type clusterConfig struct {
	cluster         *armcontainerservice.ManagedCluster
	kube            *kubeclient
	parameters      clusterParameters
	subnetId        string
	isNewCluster    bool
	isAirgapCluster bool
}

type VNet struct {
	name     string
	subnetId string
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

func validateExistingClusterState(ctx context.Context, resourceGroupName, clusterName string) (bool, error) {
	var needRecreate bool
	clusterResp, err := config.Azure.AKS.Get(ctx, resourceGroupName, clusterName, nil)
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
			cl, err := waitForClusterCreation(ctx, resourceGroupName, clusterName)
			if err != nil {
				return false, err
			}
			cluster = cl
		}

		// We only need to check the MC resource group + cluster properties if the cluster resource itself exists
		rgExists, err := config.Azure.IsExistingResourceGroup(ctx, *cluster.Properties.NodeResourceGroup)
		if err != nil {
			return false, err
		}

		if !rgExists || cluster.Properties == nil || cluster.Properties.ProvisioningState == nil || *cluster.Properties.ProvisioningState == "Failed" {
			log.Printf("deleting test cluster in bad state: %q", clusterName)

			needRecreate = true
			if err := config.Azure.DeleteCluster(ctx, resourceGroupName, clusterName); err != nil {
				return false, fmt.Errorf("failed to delete cluster in bad state: %w", err)
			}
		}
	}

	return needRecreate, nil
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

func getInitialClusterConfigs(ctx context.Context, resourceGroupName string) ([]clusterConfig, error) {
	var configs []clusterConfig
	pager := config.Azure.Resource.NewListByResourceGroupPager(resourceGroupName, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to advance page: %w", err)
		}
		for _, resource := range page.Value {
			if strings.EqualFold(*resource.Type, managedClusterResourceType) {
				cluster, err := config.Azure.AKS.Get(ctx, resourceGroupName, *resource.Name, nil)
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

				clusterConfig := clusterConfig{cluster: &cluster.ManagedCluster}
				isAirgap, err := isNetworkSecurityGroupAirgap(*clusterConfig.cluster.Properties.NodeResourceGroup)
				if err != nil {
					return nil, fmt.Errorf("failed to verify if aks subnet is for an airgap cluster: %w", err)
				}

				clusterConfig.isAirgapCluster = isAirgap
				log.Printf("found agentbaker e2e cluster %q in provisioning state %q is Airgap %v", *resource.Name, *cluster.Properties.ProvisioningState, clusterConfig.isAirgapCluster)
				configs = append(configs, clusterConfig)
			}
		}
	}
	return configs, nil
}

func hasViableConfig(scenario *scenario.Scenario, clusterConfigs []clusterConfig) bool {
	for _, config := range clusterConfigs {
		if scenario.Airgap && !config.isAirgapCluster {
			continue
		}
		if scenario.Config.ClusterSelector(config.cluster) {
			return true
		}
	}
	return false
}

func createMissingClusters(ctx context.Context, r *mrand.Rand,
	scenarios scenario.Table, clusterConfigs *[]clusterConfig) error {
	var newConfigs []clusterConfig
	for _, scenario := range scenarios {
		if !hasViableConfig(scenario, *clusterConfigs) && !hasViableConfig(scenario, newConfigs) {
			newClusterModel := getNewClusterModelForScenario(generateClusterName(r), config.Location, scenario)
			newConfigs = append(newConfigs, clusterConfig{cluster: &newClusterModel, isNewCluster: true, isAirgapCluster: scenario.Airgap})
		}
	}

	var createFuncs []func() error
	for i, c := range newConfigs {
		cConfig := c
		idx := i
		createFunc := func() error {
			clusterName := *cConfig.cluster.Name

			log.Printf("creating cluster %q...", clusterName)
			liveCluster, err := config.Azure.CreateCluster(ctx, config.ResourceGroupName, cConfig.cluster)
			if err != nil {
				return fmt.Errorf("unable to create new cluster: %w", err)
			}

			if liveCluster.Properties == nil {
				return fmt.Errorf("newly created cluster model has nil properties:\n%+v", liveCluster)
			}

			log.Printf("preparing cluster %q for testing...", clusterName)
			kube, subnetId, clusterParams, err := prepareClusterForTests(ctx, liveCluster)
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
	scenario *scenario.Scenario,
	clusterConfigs []clusterConfig) (clusterConfig, error) {

	var chosenConfig clusterConfig
	for i := range clusterConfigs {
		config := &clusterConfigs[i]
		if (scenario.Airgap && !config.isAirgapCluster) || (config.isAirgapCluster && !scenario.Airgap) {
			continue
		}

		if scenario.Config.ClusterSelector(config.cluster) {
			// only validate + prep the cluster for testing if we didn't just create it and it hasn't already been prepared
			if !config.isNewCluster && config.needsPreparation() {
				if err := validateAndPrepareCluster(ctx, r, config); err != nil {
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

	if chosenConfig.isAirgapCluster {
		hasAirgapNSG, err := isNetworkSecurityGroupAirgap(*chosenConfig.cluster.Properties.NodeResourceGroup)
		if err != nil {
			return clusterConfig{}, fmt.Errorf("failed to check if airgap settings are present: %w", err)
		}

		if !hasAirgapNSG {
			log.Printf("adding airgap network settings to cluster %q...", *chosenConfig.cluster.Name)
			err = addAirgapNetworkSettings(ctx, chosenConfig)
			if err != nil {
				return clusterConfig{}, fmt.Errorf("failed to add airgap network settings: %w", err)
			}
		}
	}

	return chosenConfig, nil
}

func validateAndPrepareCluster(ctx context.Context, r *mrand.Rand, clusterConfig *clusterConfig) error {
	needRecreate, err := validateExistingClusterState(ctx, config.ResourceGroupName, *clusterConfig.cluster.Name)
	if err != nil {
		return err
	}

	if needRecreate {
		log.Printf("cluster %q is in a bad state, creating a replacement...", *clusterConfig.cluster.Name)
		newModel, err := prepareClusterModelForRecreate(r, clusterConfig.cluster)
		if err != nil {
			return err
		}
		newCluster, err := config.Azure.CreateCluster(ctx, config.ResourceGroupName, clusterConfig.cluster)
		if err != nil {
			return err
		}
		log.Printf("replaced bad cluster %q with new cluster %q", *clusterConfig.cluster.Name, *newModel.Name)
		clusterConfig.cluster = newCluster
	}

	kube, subnetId, clusterParams, err := prepareClusterForTests(ctx, clusterConfig.cluster)
	if err != nil {
		return err
	}

	clusterConfig.kube = kube
	clusterConfig.parameters = clusterParams
	clusterConfig.subnetId = subnetId
	return nil
}

func prepareClusterForTests(
	ctx context.Context,
	cluster *armcontainerservice.ManagedCluster) (*kubeclient, string, clusterParameters, error) {
	clusterName := *cluster.Name

	vnet, err := getClusterVNet(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable get subnet ID of cluster %q: %w", clusterName, err)
	}

	kube, err := getClusterKubeClient(ctx, config.ResourceGroupName, clusterName)
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
	return kube, vnet.subnetId, clusterParams, nil
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
