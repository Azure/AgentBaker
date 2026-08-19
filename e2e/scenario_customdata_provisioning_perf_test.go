package e2e

import (
	"context"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// This file benchmarks node provisioning time, capturing two complementary durations,
// comparing the two aks-node-controller provisioning modes:
//
//   - Test_Ubuntu2204_ProvisioningPerf_CSE: the Microsoft.Azure.Extensions CustomScript
//     extension is attached and its commandToExecute is "aks-node-controller provision-wait",
//     which blocks on provision.complete written by the aks-node-controller.service unit
//     started from CustomData.
//   - Test_Ubuntu2204_ProvisioningPerf_CustomDataOnly: UseCustomDataOnlyProvisioning=true, so
//     no CustomScript extension is attached at all; aks-node-controller.service runs
//     "aks-node-controller provision" directly from the CustomData boothook, and
//     report_ready.py reports Ready/NotReady straight to the wireserver.
//
// The two durations captured (both logged via a grep-able "BENCHMARK ..." line):
//
//  1. vmss_creation_seconds (s.Runtime.VMSSCreationDuration, set in prepareAKSNode in
//     test_helpers.go): client-side wall-clock time from the VMSS BeginCreateOrUpdate call
//     until ARM's PollUntilDone returns - i.e. until ARM considers the VM provisioned. In CSE
//     mode this is gated by the CustomScript extension's exit-code/status report; in
//     CustomData-only mode there's no extension, so it's gated purely by WALinuxAgent's
//     ordinary report_ready to the wireserver. This is the actual "extension exit-code report
//     time" vs. "VMSS create reports ready" comparison.
//  2. total_cse_duration_seconds (report.TotalCSEDuration(), from ExtractCSETimings in
//     cse_timing.go): the provisioning script's own internal ExecDuration, read via SSH from
//     /var/log/azure/aks/provision.json after the VM is up. Both modes run the same underlying
//     provisioning scripts (cse_main.sh's basePrep/nodePrep) and produce this file in the same
//     format, so it's comparable between them too, but it only covers script exec time, not VM
//     boot time or the ARM round-trip captured by (1).
//
// To pin both runs to the same VHD build under test (rather than the latest main-branch
// image), set before running `go test`:
//
//	SIG_VERSION_TAG_NAME=buildId SIG_VERSION_TAG_VALUE=175942544
//
// Run both tests (serially, so they don't contend for the same cluster/VMSS naming) and
// compare the "BENCHMARK provisioning_mode=..." log lines, or use
// e2e/scripts/run_provisioning_perf_benchmark.sh to run multiple iterations and average:
//
//	cd e2e && SIG_VERSION_TAG_NAME=buildId SIG_VERSION_TAG_VALUE=175942544 \
//	  go test ./... -run 'Test_Ubuntu2204_ProvisioningPerf_(CSE|CustomDataOnly)' -v -count=1 -timeout 60m
func Test_Ubuntu2204_ProvisioningPerf_CSE(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Benchmarks CSE start-to-completion duration for aks-node-controller provisioning " +
			"via the Custom Script Extension (UseCustomDataOnlyProvisioning=false, the baseline).",
		Config: Config{
			Cluster:                  ClusterKubenet,
			VHD:                      config.VHDUbuntu2204Gen2Containerd,
			EagerCSETimingExtraction: true,
			SkipDefaultValidation:    true,
			BootstrapConfigMutator: func(_ *Cluster, _ *datamodel.NodeBootstrappingConfiguration) {},
			AKSNodeConfigMutator:     func(_ *Cluster, _ *aksnodeconfigv1.Configuration) {},
			Validator: func(ctx context.Context, s *Scenario) {
				logProvisioningPerfBenchmark(ctx, s, "cse")
			},
		},
	})
}

func Test_Ubuntu2204_ProvisioningPerf_CustomDataOnly(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Benchmarks CSE start-to-completion duration for aks-node-controller provisioning " +
			"via CustomData only, with no Custom Script Extension attached (UseCustomDataOnlyProvisioning=true).",
		Config: Config{
			Cluster:                       ClusterKubenet,
			VHD:                           config.VHDUbuntu2204Gen2Containerd,
			EagerCSETimingExtraction:      true,
			SkipDefaultValidation:         true,
			UseCustomDataOnlyProvisioning: true,
			BootstrapConfigMutator: func(_ *Cluster, _ *datamodel.NodeBootstrappingConfiguration) {},
			AKSNodeConfigMutator:          func(_ *Cluster, _ *aksnodeconfigv1.Configuration) {},
			Validator: func(ctx context.Context, s *Scenario) {
				logProvisioningPerfBenchmark(ctx, s, "customdata_only")
			},
		},
	})
}

// logProvisioningPerfBenchmark validates that provisioning completed successfully, then logs
// the CSE task timing report along with a single grep-able summary line so the two modes'
// results can be diffed across separate test runs/logs.
func logProvisioningPerfBenchmark(ctx context.Context, s *Scenario, mode string) {
	ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")

	report := s.Runtime.CSETimingReport
	if report == nil {
		var err error
		report, err = ExtractCSETimings(ctx, s)
		require.NoError(s.T, err, "failed to extract CSE timings")
	}
	report.LogReport(ctx, s.T)

	total := report.TotalCSEDuration()
	s.T.Logf("BENCHMARK provisioning_mode=%s vmss_creation_seconds=%.2f total_cse_duration_seconds=%.2f",
		mode, s.Runtime.VMSSCreationDuration.Seconds(), total.Seconds())
}
