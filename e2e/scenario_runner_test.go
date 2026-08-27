package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// This file is the compatibility adapter between the `go test` runner and the
// test-control-free scenario implementation. Everything that reads or writes
// test state - skipping, failing, sub-tests, parallelism, test cleanup - lives
// here. Implementation code only returns errors and records checks.

// RunScenario runs s as a Go test. Scenario definitions keep calling it, so the
// existing Test_* functions, their names and their sub-test layout are unchanged.
func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	// Special case for testing VHD caching. Not used by default.
	if config.Config.TestPreProvision || s.VHDCaching {
		t.Run("VHDCreation", func(t *testing.T) {
			t.Parallel()
			runScenarioWithPreProvision(t, s)
		})
		return
	}
	if config.Config.DisableScriptless || scriptlessUnsupported(s) {
		runScenarioForTest(t, s)
		return
	}

	if s.Runtime == nil {
		s.Runtime = &ScenarioRuntime{}
	}
	s.Runtime.EnableScriptlessNBCCSECmd = true
	runScenarioForTest(t, s)
}

// runScenarioForTest attaches a test-backed logger and cleanup stack to s, runs
// the scenario, mirrors the checks it recorded into sub-tests, and reports the
// outcome to t. It returns the scenario error so staged runs can decide whether
// to continue. It does not return when the scenario is skipped, because
// testing.T.Skip ends the goroutine.
func runScenarioForTest(t *testing.T, s *Scenario) error {
	t.Helper()
	tb := toolkit.WithFailureFormatting(t)
	logger := toolkit.NewTestLogger(tb)
	ctx := newTestCtx(tb, logger)
	if s.cleanup != nil {
		panic("Scenario.Cleanup called outside of a scenario run")
	}
	cleanup := &scenarioCleanup{}
	s.cleanup = cleanup
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scenarioCleanupTimeout)
		defer cancel()
		if err := cleanup.runCleanups(cleanupCtx); err != nil {
			tb.Errorf("scenario cleanup failed: %v", err)
		}
	})

	err := runScenario(ctx, t.Name(), logger, s)
	reportScenarioChecks(t, s)

	var skip *skipError
	if errors.As(err, &skip) {
		t.Skip(skip.Error())
	}
	if err != nil {
		tb.Error(err)
	}
	return err
}

// reportScenarioChecks turns the checks a scenario recorded into child test
// results, so ADO keeps per-check history for CSE timings.
func reportScenarioChecks(t *testing.T, s *Scenario) {
	t.Helper()
	for _, check := range s.checks {
		t.Run(check.Name, func(t *testing.T) {
			t.Logf("%s: %s", check.Name, check.Duration)
			if check.Message != "" {
				t.Error(check.Message)
			}
		})
	}
}

func newTestCtx(t testing.TB, logger toolkit.Logger) context.Context {
	if testCtx.Err() != nil {
		t.Skip("test suite is shutting down")
	}
	ctx, cancel := context.WithTimeout(testCtx, config.Config.TestTimeout)
	t.Cleanup(cancel)
	// only a logger is put in the context, implementation code must not be able
	// to control the test through it
	ctx = toolkit.ContextWithLogger(ctx, logger)
	return ctx
}

func runScenarioWithPreProvision(t *testing.T, original *Scenario) {
	// This is hard to understand. Some functional magic is used to run the original scenario in two stages.
	// 1. Stage 1: Run the original scenario with pre-provisioning enabled, but skip the main validation and validate only pre-provisioning.
	// 2. Create a new Image from the VMSS created in Stage 1
	// 3. Stage 2: Run the original scenario again, but this time using the custom VHD created in a previous step, with validators,
	// The goal here is to test pre-provisioning logic on the variety of existing scenarios
	firstStage := copyScenario(original)
	var customVHD *config.Image

	// Mutate the copy for pre-provisioning
	firstStage.Config.SkipDefaultValidation = true
	firstStage.Config.Validator = func(ctx context.Context, stage1 *Scenario) error {
		var validationErr error
		if stage1.IsWindows() {
			validationErr = errors.Join(
				ValidateFileExists(ctx, stage1, "C:\\AzureData\\base_prep.complete"),
				ValidateFileDoesNotExist(ctx, stage1, "C:\\AzureData\\provision.complete"),
				ValidateWindowsServiceIsNotRunning(ctx, stage1, "kubelet"),
				ValidateWindowsServiceIsRunning(ctx, stage1, "containerd"),
			)
		} else {
			validationErr = errors.Join(
				ValidateFileExists(ctx, stage1, "/etc/containerd/config.toml"),
				ValidateFileExists(ctx, stage1, "/opt/azure/containers/base_prep.complete"),
				ValidateFileDoesNotExist(ctx, stage1, "/opt/azure/containers/provision.complete"),
				ValidateSystemdUnitIsRunning(ctx, stage1, "containerd"),
				ValidateSystemdUnitIsNotRunning(ctx, stage1, "kubelet"),
			)
		}
		if validationErr != nil {
			return validationErr
		}
		toolkit.Log(ctx, "=== Creating VHD Image ===")
		var err error
		customVHD, err = CreateImage(ctx, stage1)
		if err != nil {
			return err
		}
		customVHDJSON, _ := json.MarshalIndent(customVHD, "", "  ")
		toolkit.Logf(ctx, "Created custom VHD image: %s", string(customVHDJSON))
		cleanupBastionTunnel(firstStage.Runtime.VM.SSHClient)
		firstStage.Runtime.VM.SSHClient = nil
		return nil
	}
	firstStage.Config.VMConfigMutator = func(vmss *armcompute.VirtualMachineScaleSet) {
		if original.VMConfigMutator != nil {
			original.VMConfigMutator(vmss)
		}
		if vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil {
			vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = nil
		}
	}
	if original.BootstrapConfigMutator != nil || original.BootstrapConfigMutatorWithError != nil || original.PreProvisionBootstrapConfigMutator != nil {
		firstStage.BootstrapConfigMutator = nil
		firstStage.BootstrapConfigMutatorWithError = func(ctx context.Context, cluster *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
			if original.BootstrapConfigMutator != nil {
				original.BootstrapConfigMutator(cluster, nbc)
			}
			if original.BootstrapConfigMutatorWithError != nil {
				if err := original.BootstrapConfigMutatorWithError(ctx, cluster, nbc); err != nil {
					return err
				}
			}
			nbc.PreProvisionOnly = true
			nbc.EnableScriptlessNBCCSECmd = false
			// Bake-stage-only mutation: lets a scenario deliberately diverge bake-time
			// state from provision-time state (e.g. a stale sentinel bootstrap token).
			if original.PreProvisionBootstrapConfigMutator != nil {
				original.PreProvisionBootstrapConfigMutator(cluster, nbc)
			}
			return nil
		}
	}
	if original.AKSNodeConfigMutator != nil {
		firstStage.AKSNodeConfigMutator = func(cluster *Cluster, nodeconfig *aksnodeconfigv1.Configuration) {
			original.AKSNodeConfigMutator(cluster, nodeconfig)
			nodeconfig.PreProvisionOnly = true
		}
	}

	if err := runScenarioForTest(t, firstStage); err != nil {
		return
	}

	if t.Failed() {
		return
	}

	// Create a new subtest to avoid conflicts with previous steps (log output folder is based on the test name)
	t.Run("VMProvision", func(t *testing.T) {
		t.Parallel()
		secondStageScenario := copyScenario(original)
		secondStageScenario.Description = "Stage 2: Create VMSS from captured VHD via SIG"
		secondStageScenario.Config.VHD = customVHD
		secondStageScenario.Config.Validator = func(ctx context.Context, s *Scenario) error {
			// This validators are used when running all scenarios in "VHD Caching" mode, which is usually done manually
			var markerErr error
			if s.IsWindows() {
				markerErr = ValidateFileExists(ctx, s, "C:\\AzureData\\provision.complete")
			} else {
				markerErr = ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
			}
			if markerErr != nil {
				return markerErr
			}
			if original.Config.Validator != nil {
				return original.Config.Validator(ctx, s)
			}
			return nil
		}
		runScenarioForTest(t, secondStageScenario)
	})
}
