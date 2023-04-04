package e2e_test

import (
	"context"
	mrand "math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"k8s.io/apimachinery/pkg/util/wait"
)

func pollChooseCluster(
	ctx context.Context,
	t *testing.T,
	r *mrand.Rand,
	cloud *azureClient,
	suiteConfig *suiteConfig,
	scenario *scenario.Scenario,
	clusters *[]*armcontainerservice.ManagedCluster) (*kubeclient, *armcontainerservice.ManagedCluster, map[string]string, string, error) {
	var (
		kube          *kubeclient
		cluster       *armcontainerservice.ManagedCluster
		clusterParams map[string]string
		subnetID      string
	)
	err := wait.PollImmediateWithContext(ctx, 30*time.Second, 7*time.Minute, func(ctx context.Context) (bool, error) {
		k, c, cp, s, err := chooseCluster(ctx, t, r, cloud, suiteConfig, scenario, clusters)
		if err != nil {
			if strings.Contains(err.Error(), conflictErrorMessageSubstring) {
				return false, nil
			} else {
				return false, err
			}
		} else {
			kube = k
			cluster = c
			clusterParams = cp
			subnetID = s
			return true, nil
		}
	})

	if err != nil {
		return nil, nil, nil, "", err
	}

	return kube, cluster, clusterParams, subnetID, nil
}

// Wraps extractClusterParameters in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractClusterParameters(ctx context.Context, t *testing.T, kube *kubeclient) (map[string]string, error) {
	var clusterParams map[string]string
	err := wait.PollImmediateWithContext(ctx, 15*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, t, kube)
		if err != nil {
			t.Logf("error extracting cluster parameters: %q", err)
			return false, nil
		}
		clusterParams = params
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return clusterParams, nil
}

// Wraps exctracLogsFromVM and dumpFileMapToDir in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractVMLogs(
	ctx context.Context,
	t *testing.T,
	cloud *azureClient,
	kube *kubeclient,
	suiteConfig *suiteConfig,
	mcResourceGroupName, vmssName, caseLogsDir string,
	privateKeyBytes []byte) error {
	err := wait.PollImmediateWithContext(ctx, 15*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		t.Log("attempting to extract VM logs")

		logFiles, err := extractLogsFromVM(ctx, t, cloud, kube, suiteConfig.subscription, mcResourceGroupName, vmssName, string(privateKeyBytes))
		if err != nil {
			t.Logf("error extracting VM logs: %q", err)
			return false, nil
		}

		t.Logf("dumping VM logs to local directory: %s", caseLogsDir)
		if err = dumpFileMapToDir(caseLogsDir, logFiles); err != nil {
			t.Logf("error extracting VM logs: %q", err)
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return err
	}

	return nil
}
