package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// it's important to share context between tests to allow graceful shutdown
// cancellation signal can be sent before a test starts, without shared context such test will miss the signal
var testCtx = setupSignalHandler()

// setupSignalHandler handles OS signals to gracefully shutdown the test suite
func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		// block until signal is received
		<-ch
		fmt.Println("Received cancellation signal, gracefully shutting down the test suite. Cancel again to force exit.")
		cancel()

		// block until second signal is received
		<-ch
		fmt.Println("Received second cancellation signal, forcing exit.")
		os.Exit(1)
	}()
	return ctx
}

func newTestCtx(t *testing.T) context.Context {
	if testCtx.Err() != nil {
		t.Skip("test suite is shutting down")
	}
	ctx, cancel := context.WithTimeout(testCtx, config.Config.TestTimeout)
	t.Cleanup(cancel)
	return ctx
}

var scenarioOnce sync.Once

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	ctx := newTestCtx(t)
	cleanTestDir(t)
	scenarioOnce.Do(func() {
		err := ensureResourceGroup(ctx)
		if err != nil {
			panic(err)
		}
	})
	maybeSkipScenario(ctx, t, s)

	model, err := s.Cluster(ctx, t)
	require.NoError(t, err, "creating AKS cluster")

	nbc, err := s.PrepareNodeBootstrappingConfiguration(model.NodeBootstrappingConfiguration)
	require.NoError(t, err)

	executeScenario(ctx, t, &scenarioRunOpts{
		clusterConfig: model,
		scenario:      s,
		nbc:           nbc,
	})
}

func maybeSkipScenario(ctx context.Context, t *testing.T, s *Scenario) {
	s.Tags.Name = t.Name()
	s.Tags.OS = s.VHD.OS
	s.Tags.Arch = s.VHD.Arch
	s.Tags.ImageName = s.VHD.Name
	if config.Config.TagsToRun != "" {
		matches, err := s.Tags.MatchesFilters(config.Config.TagsToRun)
		if err != nil {
			t.Fatalf("could not match tags for %q: %s", t.Name(), err)
		}
		if !matches {
			t.Skipf("skipping scenario %q: scenario tags %+v does not match filter %q", t.Name(), s.Tags, config.Config.TagsToRun)
		}
	}

	if config.Config.TagsToSkip != "" {
		matches, err := s.Tags.MatchesAnyFilter(config.Config.TagsToSkip)
		if err != nil {
			t.Fatalf("could not match tags for %q: %s", t.Name(), err)
		}
		if matches {
			t.Skipf("skipping scenario %q: scenario tags %+v matches filter %q", t.Name(), s.Tags, config.Config.TagsToSkip)
		}
	}

	vhd, err := s.VHD.VHDResourceID(ctx, t)
	if err != nil {
		if config.Config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
			t.Skipf("skipping scenario %q: could not find image", t.Name())
		} else {
			t.Fatalf("could not find image for %q: %s", t.Name(), err)
		}
	}
	t.Logf("running scenario %q with vhd: %q, tags %+v", t.Name(), vhd, s.Tags)
}

func executeScenario(ctx context.Context, t *testing.T, opts *scenarioRunOpts) {
	rid, _ := opts.scenario.VHD.VHDResourceID(ctx, t)
	t.Logf("running scenario %q with image %q in aks cluster %q", t.Name(), rid, *opts.clusterConfig.Model.ID)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)

	vmssName := getVmssName(t)
	createVMSS(ctx, t, vmssName, opts, privateKeyBytes, publicKeyBytes)

	t.Logf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", vmssName)
	nodeName := validateNodeHealth(ctx, t, opts.clusterConfig.Kube, vmssName, opts.scenario.Tags.Airgap)

	// skip when outbound type is block as the wasm will create pod from gcr, however, network isolated cluster scenario will block egress traffic of gcr.
	// TODO(xinhl): add another way to validate
	if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi && (opts.nbc.OutboundType != datamodel.OutboundTypeBlock && opts.nbc.OutboundType != datamodel.OutboundTypeNone) {
		validateWasm(ctx, t, opts.clusterConfig.Kube, nodeName)
	}

	t.Logf("node %s is ready, proceeding with validation commands...", vmssName)

	vmPrivateIP, err := getVMPrivateIPAddress(ctx, *opts.clusterConfig.Model.Properties.NodeResourceGroup, vmssName)
	require.NoError(t, err)

	require.NoError(t, err, "get vm private IP %v", vmssName)
	err = runLiveVMValidators(ctx, t, vmssName, vmPrivateIP, string(privateKeyBytes), opts)
	require.NoError(t, err)

	err = getCustomScriptExtensionStatus(ctx, t, *opts.clusterConfig.Model.Properties.NodeResourceGroup, vmssName)

	t.Logf("node %s bootstrapping succeeded!", vmssName)
}

func getExpectedPackageVersions(packageName, distro, release string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here
	jsonBytes, _ := os.ReadFile("../parts/linux/cloud-init/artifacts/components.json")
	packages := gjson.GetBytes(jsonBytes, fmt.Sprintf("Packages.#(name=%s).downloadURIs", packageName))

	for _, packageItem := range packages.Array() {
		// check if versionsV2 exists
		if packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release)).Exists() {
			versions := packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release))
			for _, version := range versions.Array() {
				// get versions.latestVersion and append to expectedVersions
				expectedVersions = append(expectedVersions, version.Get("latestVersion").String())
				// get versions.previousLatestVersion (if exists) and append to expectedVersions
				if version.Get("previousLatestVersion").Exists() {
					expectedVersions = append(expectedVersions, version.Get("previousLatestVersion").String())
				}
			}
		}
	}
	return expectedVersions
}

func getCustomScriptExtensionStatus(ctx context.Context, t *testing.T, resourceGroupName, vmssName string) error {
	// List all VM instances within the VM Scale Set
	pager := config.Azure.VMSSVM.NewListPager(resourceGroupName, vmssName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get VMSS instances: %v", err)
		}

		// Iterate through each instance in the VM Scale Set
		for _, vmInstance := range page.Value {
			// Get the instance view for each VM instance to access extensions data
			instanceViewResp, err := config.Azure.VMSSVM.GetInstanceView(ctx, resourceGroupName, vmssName, *vmInstance.InstanceID, nil)
			if err != nil {
				t.Logf("failed to get instance view for VM %s: %v", *vmInstance.InstanceID, err)
				continue
			}
			// Loop through each extension in the instance view
			for _, extension := range instanceViewResp.Extensions {
				for _, status := range extension.Statuses {
					resp, err := parseLinuxCSEMessage(t, *status)
					if err != nil {
						return fmt.Errorf("Parse CSE message with error, error code: %s, raw message: %s", err.Code, *status.Message)
					}
					if !strings.EqualFold(resp.ExitCode, "0") {
						return fmt.Errorf("vmssCSE %s, output=%s, error=%s", resp.ExitCode, resp.Output, resp.Error)
					}
				}
			}
		}
	}

	t.Logf("CSE completed successfully with exit code 0")
	return nil
}

func parseLinuxCSEMessage(t *testing.T, status armcompute.InstanceViewStatus) (*datamodel.CSEStatus, *datamodel.CSEStatusParsingError) {
	// Base Case Happy Path 1: https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAAxWMuw4CIRAAe79iQ4%2FBEyQWV1zOxs9Ylo2iES6wPi7x48VuJpnM1NZMc8nCH5lI0ivJuvnC%2B8qVoSxcUVLJ5xOMI6gQfETrvDaETlt7PGg0znTydh%2FY%2BoGNAswRHu0C1K%2BYcgPVBOXZtnOJrPp8qeXGJP%2Bom%2BCdYfcD%2BmnPWogAAAA%3D
	// status.Code: ProvisioningState/succeeded
	// status.Message
	/*
		Enable succeeded:
		[stdout]
		{ \"ExitCode\": \"0\", \"Output\": \"Tue Dec 28" , "ExecDuration": "53", "KernelStartTime": "Tue 2024-02-27 21:18:05 UTC" } }

		[stderr]
		Bootup is not yet finished. Please try again later.
	*/

	// Base Case Happy Path 2: https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAAz2Muw7CMAwAd77Cyh5kGkq6dKjKwme4roGASKrEPCrx8RQGxpPuritz5D5FlZd2rOERdF694XmWLJAmyaQhxcMe2haMr3fOyREtctXYLY7eDoOvLTE655B8I2L%2B9a2cgJczhVjAFCW9l3Wfxp8y5XQR1q%2B0kNJVYPMBUMXHZowAAAA%3D
	// status.Code: ProvisioningState/failed/0
	// status.Message
	/*
		Enable failed:
		[stdout]
		{ \"ExitCode\": \"53\", \"Output\": \"Tue Dec 28" , "ExecDuration": "53", "KernelStartTime": "Tue 2024-02-27 21:18:05 UTC" } }

		[stderr]
		Bootup is not yet finished. Please try again later.
	*/

	// Corner Case 1: https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAA22OOwrDMBBE%2B5xiUa9goWDhwoWxmxxjkdaJEiyZ1eYjyOGj9IFpHsMbZio1%2BTknobdMXuIzSj184HUlJsg7MUrM6bzAOIJydrCh7wdt3Gr1yTirEWnVaMyAXYt3nQJMAbZyAd9WMaYCqgjKoxznHOhfPTOhUAChbc%2BMXCFEJi%2BZq2pfds63Rj%2BpkeCdwHwBehIYpLcAAAA%3D
	// status.Code: ProvisioningState/succeeded
	// status.Message
	/*
		Enable succeeded:
		[stdout]
		{ \"ExitCode\": \"0\", \"Output\": \"Tue Dec 28" , "ExecDuration": "53", "KernelStartTime": "Tue 2024-02-27 21:18:05 UTC" } }
		Created temporary directory: /tmp/tmp.jnPaiNfqEn
		Collecting system information...

		[stderr]
		Bootup is not yet finished. Please try again later.
	*/

	// Corner Case 2: https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAAz2MOw7CMBAFe06xcm8pRsFOkyIiDcfwbjZgPnZkL59IHB6noRnpSfNmKGukY4rCHxlIwivIuvvC%2B8KZIS2cvYQUTyP0PSj0nTVuRt0gGd3ifNBda7HCccN7h4as%2Br8f5QxUyz7EAmoMxeOdoTyJmCeeNnHJ6cokm1qX%2BBuD%2BQHGxYrxkgAAAA%3D%3D
	// status.Code: ProvisioningState/succeeded
	// status.Message
	/*
		Disable succeeded
	*/

	// Failed Case 1:  https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAA1WNzQ6CMBCE7z7FpndMSAv%2BJBwIXnyMtV2kaltCF4HEh7fEaOJtJvPNTB0Xr5vgmWauNdun5WXzgqmjgSD0NCDb4M8nqCoQSkmTt7rMioORmdoRZvs9qcyoiymoxFJKJeBXd%2FEKOk2j9RFEi%2FZBBjgAzaRHppQ5h94cvwKYBmc9csImy10CLUNk5DFWhRSwMv%2Bjn3DbBEMi%2FfZDuJHmFUqO8U6QvwH2%2FZiC4gAAAA%3D%3D
	// status.Code: ProvisioningState/failed/0
	// status.Message
	/*
		Enable failed: failed to execute command: command terminated with exit status=53
		[stdout]
		(deflated 82%)
		  adding: var/log/azure/Microsoft.Azure.Monitor.AzureMonitorLinuxAgent/CommandExecution.log (deflated 76%)
		  adding: var/log/azure/Microsoft.Azure.Monitor.AzureMonitorLinuxAgent/extension.log (deflated 79%)
		  adding: var/log/syslog (deflated 90%)
		  adding: var/log/syslog.1 (deflated 82%)
		  adding: var/log/syslog.2.gz (deflated 5%)
		[stderr]
		Bootup is not yet finished. Please try again later.
	*/

	// Failed Case 2: https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAA12MOw7CMBAFe06xcm%2BUj6VAkSJKGo6xWa%2FBQOwo3gCRODymoaB50khvpktboD4G4Zd0JP7hZdu94XnhhSHOvKD4GE4DtC2og0NDNZJmR6SNIdRYVIU%2BVo5HY8umamoFP31KZ6CcRh8SKC8wrUlgZEgzk3eeLcRw3%2FIQK8Bg%2F4wkKGva99GyytF5iVcm%2BZ4yCd4Yyg8BJ8iuvwAAAA%3D%3D
	// status.Code: ProvisioningState/failed/0
	// status.Message
	/*
		Enable failed: failed to get configuration: invalid configuration: 'commandToExecute' was specified both in public and protected settings; it must be specified only once
	*/

	// Base Case and Corner Case, we should return valid CSEStatus
	// Failed Case 1, we should return exitCode and filter out those VMs and recreate
	// Failed Case 2, we should return error and filter out those VMs and recreate
	if status.Code == nil || status.Message == nil {
		t.Logf("No valid Status code or Message provided from cse extension")
		return nil, datamodel.NewError(datamodel.InvalidCSEMessage, "No valid Status code or Message provided from cse extension")
	}

	t.Logf("CSE status.Message: %s", *status.Message)
	start := strings.Index(*status.Message, "[stdout]") + len("[stdout]")
	end := strings.Index(*status.Message, "[stderr]")

	var linuxExtensionExitCodeStrRegex = regexp.MustCompile(linuxExtensionExitCodeStr)
	var linuxExtensionErrorCodeRegex = regexp.MustCompile(extensionErrorCodeRegex)
	extensionFailed := linuxExtensionErrorCodeRegex.MatchString(*status.Code)
	if end <= start {
		t.Logf("Parse CSE failed with error cannot find [stdout] and [stderr], raw CSE Message: %s, delete vm: %t", *status.Message, extensionFailed)
		return nil, datamodel.NewError(datamodel.InvalidCSEMessage, *status.Message)
	}
	rawInstanceViewInfo := (*status.Message)[start:end]
	t.Logf("rawInstanceViewInfo: %s", rawInstanceViewInfo)
	// Parse CSE message
	var cseStatus datamodel.CSEStatus
	err := json.Unmarshal([]byte(rawInstanceViewInfo), &cseStatus)
	if err != nil {
		t.Logf("Parse CSE Json failed with error: %s, raw CSE Message: %s, delete vm: %t", err, *status.Message, extensionFailed)
		exitCodeMatch := linuxExtensionExitCodeStrRegex.FindStringSubmatch(*status.Message)
		if len(exitCodeMatch) > 1 && extensionFailed {
			// Failed but the format is not expected.
			cseStatus.ExitCode = exitCodeMatch[1]
			cseStatus.Error = *status.Message
			return &cseStatus, nil
		}
		return nil, datamodel.NewError(datamodel.CSEMessageUnmarshalError, *status.Message)
	}
	if cseStatus.ExitCode == "" {
		t.Logf("CSE Json does not contain exit code, raw CSE Message: %s", *status.Message)
		return nil, datamodel.NewError(datamodel.CSEMessageExitCodeEmptyError, *status.Message)
	}
	return &cseStatus, nil
}
