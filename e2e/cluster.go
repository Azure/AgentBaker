package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

var (
	clusterKubenet       *Cluster
	clusterKubenetAirgap *Cluster
	clusterAzureNetwork  *Cluster

	clusterKubenetError       error
	clusterKubenetAirgapError error
	clusterAzureNetworkError  error

	clusterKubenetOnce       sync.Once
	clusterKubenetAirgapOnce sync.Once
	clusterAzureNetworkOnce  sync.Once
)

type Cluster struct {
	Model                          *armcontainerservice.ManagedCluster
	Kube                           *Kubeclient
	SubnetID                       string
	NodeBootstrappingConfiguration *datamodel.NodeBootstrappingConfiguration
	Maintenance                    *armcontainerservice.MaintenanceConfiguration
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

// Same cluster can be attempted to be created concurrently by different tests
// sync.Once is used to ensure that only one cluster for the set of tests is created
func ClusterKubenet(ctx context.Context, t *testing.T) (*Cluster, error) {
	clusterKubenetOnce.Do(func() {
		clusterKubenet, clusterKubenetError = prepareCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet"))
	})
	return clusterKubenet, clusterKubenetError
}

func ClusterKubenetAirgap(ctx context.Context, t *testing.T) (*Cluster, error) {
	clusterKubenetAirgapOnce.Do(func() {
		cluster, err := prepareCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet-airgap"))
		if err == nil {
			err = addAirgapNetworkSettings(ctx, t, cluster)
		}
		clusterKubenetAirgap, clusterKubenetAirgapError = cluster, err
	})
	return clusterKubenetAirgap, clusterKubenetAirgapError
}

func ClusterAzureNetwork(ctx context.Context, t *testing.T) (*Cluster, error) {
	clusterAzureNetworkOnce.Do(func() {
		clusterAzureNetwork, clusterAzureNetworkError = prepareCluster(ctx, t, getAzureNetworkClusterModel("abe2e-azure-network"))
	})
	return clusterAzureNetwork, clusterAzureNetworkError
}

func nodeBootsrappingConfig(ctx context.Context, t *testing.T, kube *Kubeclient) (*datamodel.NodeBootstrappingConfiguration, error) {
	clusterParams, err := extractClusterParameters(ctx, t, kube)
	if err != nil {
		return nil, fmt.Errorf("extract cluster parameters: %w", err)
	}

	baseNodeBootstrappingConfig, err := getBaseNodeBootstrappingConfiguration(clusterParams)
	if err != nil {
		return nil, fmt.Errorf("get base node bootstrapping configuration: %w", err)
	}

	return baseNodeBootstrappingConfig, nil
}

func prepareCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*Cluster, error) {
	cluster.Name = to.Ptr(fmt.Sprintf("%s-%s", *cluster.Name, hash(cluster)))

	cluster, err := getOrCreateCluster(ctx, t, cluster)
	if err != nil {
		return nil, err
	}

	maintenance, err := getOrCreateMaintenanceConfiguration(ctx, t, cluster)
	if err != nil {
		return nil, fmt.Errorf("get or create maintenance configuration: %w", err)
	}

	// sometimes tests can be interrupted and vmss are left behind
	// don't waste resource and delete them
	if err := collectGarbageVMSS(ctx, t, cluster); err != nil {
		return nil, fmt.Errorf("collect garbage vmss: %w", err)
	}

	kube, err := getClusterKubeClient(ctx, config.ResourceGroupName, *cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("get kube client using cluster %q: %w", *cluster.Name, err)
	}

	if err := ensureDebugDaemonsets(ctx, kube); err != nil {
		return nil, fmt.Errorf("ensure debug daemonsets for %q: %w", *cluster.Name, err)
	}

	subnetID, err := getClusterSubnetID(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return nil, fmt.Errorf("get cluster subnet: %w", err)
	}

	nbc, err := nodeBootsrappingConfig(ctx, t, kube)
	if err != nil {
		return nil, fmt.Errorf("get node bootstrapping configuration: %w", err)
	}

	return &Cluster{Model: cluster, Kube: kube, SubnetID: subnetID, NodeBootstrappingConfiguration: nbc, Maintenance: maintenance}, nil
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
	existingCluster, err := config.Azure.AKS.Get(ctx, config.ResourceGroupName, *cluster.Name, nil)
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == 404 {
		return createNewAKSClusterWithRetry(ctx, t, cluster)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %q: %w", *cluster.Name, err)
	}
	switch *existingCluster.Properties.ProvisioningState {
	case "Succeeded":
		return &existingCluster.ManagedCluster, nil
	case "Creating", "Updating":
		return waitUntilClusterReady(ctx, config.ResourceGroupName, *cluster.Name)
	default:
		// this operation will try to update the cluster if it's in a failed state
		return createNewAKSClusterWithRetry(ctx, t, cluster)
	}
}

func createNewAKSCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	t.Logf("creating or updating cluster %s in rg %s\n", *cluster.Name, *cluster.Location)
	// Note, it seems like the operation still can start a trigger a new operation even if nothing has changes
	pollerResp, err := config.Azure.AKS.BeginCreateOrUpdate(
		ctx,
		config.ResourceGroupName,
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
		t.Logf("Attempt %d: creating or updating cluster %s in rg %s\n", attempt+1, *cluster.Name, *cluster.Location)

		createdCluster, err := createNewAKSCluster(ctx, t, cluster)
		if err == nil {
			return createdCluster, nil
		}

		// Check if the error is a 409 Conflict
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			lastErr = err
			t.Logf("Attempt %d failed with 409 Conflict: %v. Retrying in %v...\n", attempt+1, err, retryInterval)

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
	existingMaintenance, err := config.Azure.Maintenance.Get(ctx, config.ResourceGroupName, *cluster.Name, "default", nil)
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
	t.Logf("creating maintenance configuration for cluster %s in rg %s\n", *cluster.Name, config.ResourceGroupName)
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
					RelativeMonthly: &armcontainerservice.RelativeMonthlySchedule{
						DayOfWeek:      to.Ptr(armcontainerservice.WeekDayMonday),
						IntervalMonths: to.Ptr[int32](3),
						WeekIndex:      to.Ptr(armcontainerservice.TypeFirst),
					},
				},
			},
		},
	}

	_, err := config.Azure.Maintenance.CreateOrUpdate(ctx, config.ResourceGroupName, *cluster.Name, "default", maintenance, nil)
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

func isExistingResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	rgExistence, err := config.Azure.ResourceGroup.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func ensureResourceGroup(ctx context.Context) error {
	rgExists, err := isExistingResourceGroup(ctx, config.ResourceGroupName)
	if err != nil {
		return err
	}

	if !rgExists {
		_, err = config.Azure.ResourceGroup.CreateOrUpdate(
			ctx,
			config.ResourceGroupName,
			armresources.ResourceGroup{
				Location: to.Ptr(config.Config.Location),
				Name:     to.Ptr(config.ResourceGroupName),
			},
			nil)

		if err != nil {
			return fmt.Errorf("failed to create RG %q: %w", config.ResourceGroupName, err)
		}
	}

	return nil
}
